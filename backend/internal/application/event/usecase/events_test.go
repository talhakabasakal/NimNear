package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
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
