package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	calendarModel "github.com/masterfabric-go/masterfabric/internal/domain/calendar/model"
	calendarRepo "github.com/masterfabric-go/masterfabric/internal/domain/calendar/repository"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
	placeModel "github.com/masterfabric-go/masterfabric/internal/domain/place/model"
	placeRepo "github.com/masterfabric-go/masterfabric/internal/domain/place/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type fakeEventRepository struct {
	events  []*model.Event
	detail  *model.Event
	err     error
	filter  repository.ListFilter
	created *model.Event
}

func (f *fakeEventRepository) ListPublic(_ context.Context, filter repository.ListFilter) ([]*model.Event, error) {
	f.filter = filter
	return f.events, f.err
}

func (f *fakeEventRepository) ListPublicByOrganizer(_ context.Context, _ uuid.UUID) ([]*model.Event, error) {
	return f.events, f.err
}

func (f *fakeEventRepository) ListPublicByAttendee(_ context.Context, _ uuid.UUID) ([]*model.Event, error) {
	return f.events, f.err
}

func (f *fakeEventRepository) ListPublicByCalendar(_ context.Context, _ uuid.UUID) ([]*model.Event, error) {
	return f.events, f.err
}

func (f *fakeEventRepository) GetPublicByID(_ context.Context, _ uuid.UUID) (*model.Event, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.detail, nil
}

func (f *fakeEventRepository) Create(_ context.Context, event *model.Event) error {
	f.created = event
	return f.err
}

type fakeCreatePlaceRepository struct {
	place *placeModel.Place
	err   error
}

func (f *fakeCreatePlaceRepository) ListNearby(context.Context, float64, float64, float64, int) ([]*placeModel.NearbyPlace, error) {
	return nil, nil
}
func (f *fakeCreatePlaceRepository) GetActiveByID(context.Context, uuid.UUID) (*placeModel.Place, error) {
	return f.place, f.err
}

var _ placeRepo.PlaceRepository = (*fakeCreatePlaceRepository)(nil)

type fakeCreateCalendarRepository struct {
	calendar *calendarModel.Calendar
	err      error
}

func (f *fakeCreateCalendarRepository) ListPublic(context.Context, int) ([]*calendarModel.Calendar, error) {
	return nil, nil
}
func (f *fakeCreateCalendarRepository) GetPublicByID(context.Context, uuid.UUID) (*calendarModel.Calendar, error) {
	return nil, nil
}
func (f *fakeCreateCalendarRepository) GetByID(context.Context, uuid.UUID) (*calendarModel.Calendar, error) {
	return f.calendar, f.err
}
func (f *fakeCreateCalendarRepository) ListOwned(context.Context, uuid.UUID) ([]*calendarModel.Calendar, error) {
	return nil, nil
}
func (f *fakeCreateCalendarRepository) ListFollowed(context.Context, uuid.UUID) ([]*calendarModel.Calendar, error) {
	return nil, nil
}
func (f *fakeCreateCalendarRepository) Create(context.Context, *calendarModel.Calendar) error {
	return nil
}
func (f *fakeCreateCalendarRepository) Follow(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (f *fakeCreateCalendarRepository) Unfollow(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

var _ calendarRepo.CalendarRepository = (*fakeCreateCalendarRepository)(nil)

func fixedEvent(now time.Time) *model.Event {
	capacity := 2
	return &model.Event{
		ID:            uuid.New(),
		Title:         "Istanbul Meetup",
		Description:   "A local event",
		StartsAt:      now.Add(time.Hour),
		EndsAt:        now.Add(2 * time.Hour),
		Status:        model.EventStatusPublished,
		PriceLunas:    0,
		Currency:      "NIM",
		Capacity:      &capacity,
		AttendeeCount: 2,
		City:          "Istanbul",
		IsPublic:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func TestListDefaultsToPublicUpcomingChronologicalDiscovery(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	first := fixedEvent(now)
	second := fixedEvent(now)
	second.ID = uuid.New()
	second.Title = "Later event"
	second.StartsAt = now.Add(2 * time.Hour)
	repo := &fakeEventRepository{events: []*model.Event{first, second}}
	uc := NewEventUseCase(repo)
	uc.now = func() time.Time { return now }

	result, err := uc.List(context.Background(), dto.ListEventsQuery{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result.Data) != 2 || result.Data[0].Title != "Istanbul Meetup" || result.Data[1].Title != "Later event" {
		t.Fatalf("unexpected chronological data: %#v", result.Data)
	}
	if repo.filter.Limit != DefaultEventLimit || repo.filter.From == nil || !repo.filter.From.Equal(now) {
		t.Fatalf("unexpected default filter: %#v", repo.filter)
	}
	if !result.Data[0].IsFree || !result.Data[0].IsSoldOut || result.Data[0].IsPast {
		t.Fatalf("unexpected derived event state: %#v", result.Data[0])
	}
}

func TestListPassesPlaceFilterToRepository(t *testing.T) {
	placeID := uuid.New()
	repo := &fakeEventRepository{events: []*model.Event{fixedEvent(time.Now().UTC())}}
	result, err := NewEventUseCase(repo).List(context.Background(), dto.ListEventsQuery{PlaceID: &placeID, Limit: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result == nil || repo.filter.PlaceID == nil || *repo.filter.PlaceID != placeID {
		t.Fatalf("place filter was not passed through: %#v", repo.filter)
	}
}

func TestCreateRejectsInvalidTimeRange(t *testing.T) {
	start := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	_, err := NewEventUseCase(&fakeEventRepository{}).Create(context.Background(), uuid.New(), dto.CreateEventRequest{
		Title:    "Invalid event",
		StartsAt: start,
		EndsAt:   start,
		City:     "Istanbul",
	})
	if err == nil || !domainErrIs(err, domainErr.ErrValidation) {
		t.Fatalf("error = %v, want validation error", err)
	}
}

func TestCreateRejectsInvalidCoordinates(t *testing.T) {
	latitude := 91.0
	longitude := 29.0
	_, err := NewEventUseCase(&fakeEventRepository{}).Create(context.Background(), uuid.New(), dto.CreateEventRequest{
		Title:     "Invalid coordinates",
		StartsAt:  time.Now().UTC().Add(time.Hour),
		EndsAt:    time.Now().UTC().Add(2 * time.Hour),
		City:      "Istanbul",
		Latitude:  &latitude,
		Longitude: &longitude,
	})
	if err == nil || !domainErrIs(err, domainErr.ErrValidation) {
		t.Fatalf("error = %v, want validation error", err)
	}
}

func TestCreateMapsExactFreePriceAndOrganizer(t *testing.T) {
	repo := &fakeEventRepository{}
	organizerID := uuid.New()
	result, err := NewEventUseCase(repo).Create(context.Background(), organizerID, dto.CreateEventRequest{
		Title:    "Free event",
		StartsAt: time.Now().UTC().Add(time.Hour),
		EndsAt:   time.Now().UTC().Add(2 * time.Hour),
		PriceNIM: "000.50000",
		City:     "Istanbul",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.created == nil || repo.created.OrganizerID == nil || *repo.created.OrganizerID != organizerID {
		t.Fatalf("organizer was not mapped: %#v", repo.created)
	}
	if repo.created.PriceLunas != 50000 || result.Data.PriceNIM != "0.5" || result.Data.IsFree {
		t.Fatalf("price mapping is not exact: %#v", result.Data)
	}
}

func TestGetReturnsNotFound(t *testing.T) {
	repo := &fakeEventRepository{err: domainErr.New(domainErr.ErrNotFound, "event not found", nil)}
	_, err := NewEventUseCase(repo).Get(context.Background(), uuid.New())
	if err == nil || !domainErrIs(err, domainErr.ErrNotFound) {
		t.Fatalf("error = %v, want not found", err)
	}
}

func domainErrIs(err, target error) bool {
	return err != nil && (err == target || domainErr.HTTPStatusCode(err) == domainErr.HTTPStatusCode(target))
}

func TestCreateRejectsPriceThatNeedsRounding(t *testing.T) {
	_, err := NewEventUseCase(&fakeEventRepository{}).Create(context.Background(), uuid.New(), dto.CreateEventRequest{
		Title: "Too precise", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour),
		PriceNIM: "1.000001", City: "Istanbul",
	})
	if err == nil || !domainErrIs(err, domainErr.ErrValidation) {
		t.Fatalf("error = %v, want exact-Luna validation error", err)
	}
}

func TestCreateMapsExplicitFreePriceToZeroLunas(t *testing.T) {
	repo := &fakeEventRepository{}
	result, err := NewEventUseCase(repo).Create(context.Background(), uuid.New(), dto.CreateEventRequest{
		Title: "Free event", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour),
		PriceNIM: "0", City: "Istanbul",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.created == nil || repo.created.PriceLunas != 0 || result.Data.PriceNIM != "0" || !result.Data.IsFree {
		t.Fatalf("unexpected free price: %#v", result.Data)
	}
}

func TestCreateRejectsInvalidCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		value := capacity
		_, err := NewEventUseCase(&fakeEventRepository{}).Create(context.Background(), uuid.New(), dto.CreateEventRequest{
			Title: "Invalid capacity", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour),
			City: "Istanbul", Capacity: &value,
		})
		if err == nil || !domainErrIs(err, domainErr.ErrValidation) {
			t.Fatalf("capacity %d error = %v, want validation error", capacity, err)
		}
	}
}

func TestCreateMapsPaidPriceToExactLunas(t *testing.T) {
	repo := &fakeEventRepository{}
	result, err := NewEventUseCase(repo).Create(context.Background(), uuid.New(), dto.CreateEventRequest{
		Title: "Paid event", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour),
		PriceNIM: "12.50000", City: "Istanbul",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.created.PriceLunas != 1250000 || result.Data.PriceNIM != "12.5" || result.Data.IsFree {
		t.Fatalf("unexpected exact paid price: %#v", result.Data)
	}
}

func TestCreateValidatesExternalImageURL(t *testing.T) {
	base := dto.CreateEventRequest{
		Title: "Image event", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour), City: "Istanbul",
	}
	for _, imageURL := range []string{"javascript:alert(1)", "data:image/png;base64,AAAA", "file:///tmp/image.png", "ftp://media.example/image.png", "://bad"} {
		request := base
		request.ImageURL = imageURL
		if _, err := NewEventUseCase(&fakeEventRepository{}).Create(context.Background(), uuid.New(), request); err == nil || !domainErrIs(err, domainErr.ErrValidation) {
			t.Fatalf("image_url %q error = %v, want validation error", imageURL, err)
		}
	}

	repo := &fakeEventRepository{}
	base.ImageURL = "https://media.example/event.jpg"
	result, err := NewEventUseCase(repo).Create(context.Background(), uuid.New(), base)
	if err != nil {
		t.Fatalf("valid image URL returned error: %v", err)
	}
	if result.Data.ImageURL == nil || *result.Data.ImageURL != base.ImageURL {
		t.Fatalf("valid image URL was not preserved: %#v", result.Data.ImageURL)
	}
}

func TestMapEventUsesNullForMissingOrUnsafeImage(t *testing.T) {
	event := fixedEvent(time.Now().UTC())
	event.ImageURL = "javascript:alert(1)"
	if mapped := MapEvent(event, time.Now().UTC()); mapped.ImageURL != nil {
		t.Fatalf("unsafe legacy image URL was exposed: %#v", mapped.ImageURL)
	}
	event.ImageURL = ""
	if mapped := MapEvent(event, time.Now().UTC()); mapped.ImageURL != nil {
		t.Fatalf("missing image URL = %#v, want nil", mapped.ImageURL)
	}
}

func TestCreateSupportsValidatedPlaceAndOwnedCalendarAssociations(t *testing.T) {
	organizerID := uuid.New()
	placeID, calendarID := uuid.New(), uuid.New()
	place := &placeModel.Place{ID: placeID, IsActive: true}
	calendar := &calendarModel.Calendar{ID: calendarID, OwnerID: organizerID, Status: calendarModel.StatusActive}
	repo := &fakeEventRepository{}
	result, err := NewEventUseCaseWithAssociations(repo, &fakeCreatePlaceRepository{place: place}, &fakeCreateCalendarRepository{calendar: calendar}).Create(context.Background(), organizerID, dto.CreateEventRequest{
		Title: "Associated event", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour),
		City: "Istanbul", PlaceID: &placeID, CalendarID: &calendarID,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.Data.PlaceID == nil || *result.Data.PlaceID != placeID || result.Data.CalendarID == nil || *result.Data.CalendarID != calendarID {
		t.Fatalf("associations were not persisted/mapped: %#v", result.Data)
	}
}

func TestCreateRejectsUnauthorizedOrMissingCalendar(t *testing.T) {
	organizerID := uuid.New()
	calendarID := uuid.New()
	otherOwner := &calendarModel.Calendar{ID: calendarID, OwnerID: uuid.New(), Status: calendarModel.StatusActive}
	_, err := NewEventUseCaseWithAssociations(&fakeEventRepository{}, &fakeCreatePlaceRepository{}, &fakeCreateCalendarRepository{calendar: otherOwner}).Create(context.Background(), organizerID, dto.CreateEventRequest{
		Title: "Unauthorized calendar", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour), City: "Istanbul", CalendarID: &calendarID,
	})
	if err == nil || !errors.Is(err, domainErr.ErrForbidden) {
		t.Fatalf("error = %v, want forbidden calendar association", err)
	}
	_, err = NewEventUseCaseWithAssociations(&fakeEventRepository{}, &fakeCreatePlaceRepository{}, &fakeCreateCalendarRepository{err: domainErr.New(domainErr.ErrNotFound, "calendar not found", nil)}).Create(context.Background(), organizerID, dto.CreateEventRequest{
		Title: "Missing calendar", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour), City: "Istanbul", CalendarID: &calendarID,
	})
	if err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("error = %v, want missing calendar error", err)
	}
}

func TestCreateRejectsInvalidPlaceAndContradictoryLocation(t *testing.T) {
	organizerID := uuid.New()
	placeID := uuid.New()
	address := "Custom address"
	_, err := NewEventUseCaseWithAssociations(&fakeEventRepository{}, &fakeCreatePlaceRepository{err: domainErr.New(domainErr.ErrNotFound, "place not found", nil)}, &fakeCreateCalendarRepository{}).Create(context.Background(), organizerID, dto.CreateEventRequest{
		Title: "Missing place", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour), City: "Istanbul", PlaceID: &placeID,
	})
	if err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("error = %v, want missing place error", err)
	}
	_, err = NewEventUseCaseWithAssociations(&fakeEventRepository{}, &fakeCreatePlaceRepository{place: &placeModel.Place{ID: placeID, IsActive: true}}, &fakeCreateCalendarRepository{}).Create(context.Background(), organizerID, dto.CreateEventRequest{
		Title: "Contradictory place", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour), City: "Istanbul", PlaceID: &placeID, Address: &address,
	})
	if err == nil || !errors.Is(err, domainErr.ErrValidation) {
		t.Fatalf("error = %v, want contradictory location validation", err)
	}
}
