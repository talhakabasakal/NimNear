package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/calendar/dto"
	calendarModel "github.com/masterfabric-go/masterfabric/internal/domain/calendar/model"
	calendarRepo "github.com/masterfabric-go/masterfabric/internal/domain/calendar/repository"
	eventModel "github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	eventRepo "github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type fakeCalendarRepository struct {
	public   []*calendarModel.Calendar
	detail   *calendarModel.Calendar
	owned    []*calendarModel.Calendar
	followed []*calendarModel.Calendar
	created  *calendarModel.Calendar
	follows  int
}

func (f *fakeCalendarRepository) ListPublic(context.Context, int) ([]*calendarModel.Calendar, error) {
	return f.public, nil
}
func (f *fakeCalendarRepository) GetPublicByID(context.Context, uuid.UUID) (*calendarModel.Calendar, error) {
	if f.detail == nil {
		return nil, domainErr.New(domainErr.ErrNotFound, "calendar not found", nil)
	}
	return f.detail, nil
}
func (f *fakeCalendarRepository) GetByID(_ context.Context, id uuid.UUID) (*calendarModel.Calendar, error) {
	if f.detail == nil || f.detail.ID != id {
		return nil, domainErr.New(domainErr.ErrNotFound, "calendar not found", nil)
	}
	return f.detail, nil
}
func (f *fakeCalendarRepository) ListOwned(context.Context, uuid.UUID) ([]*calendarModel.Calendar, error) {
	return f.owned, nil
}
func (f *fakeCalendarRepository) ListFollowed(context.Context, uuid.UUID) ([]*calendarModel.Calendar, error) {
	return f.followed, nil
}
func (f *fakeCalendarRepository) Create(_ context.Context, calendar *calendarModel.Calendar) error {
	f.created = calendar
	return nil
}
func (f *fakeCalendarRepository) Follow(context.Context, uuid.UUID, uuid.UUID) error {
	f.follows++
	return nil
}
func (f *fakeCalendarRepository) Unfollow(context.Context, uuid.UUID, uuid.UUID) error { return nil }

var _ calendarRepo.CalendarRepository = (*fakeCalendarRepository)(nil)

type fakeCalendarEventRepository struct{ events []*eventModel.Event }

func (f *fakeCalendarEventRepository) ListPublic(context.Context, eventRepo.ListFilter) ([]*eventModel.Event, error) {
	return nil, nil
}
func (f *fakeCalendarEventRepository) ListPublicByOrganizer(context.Context, uuid.UUID) ([]*eventModel.Event, error) {
	return nil, nil
}
func (f *fakeCalendarEventRepository) ListPublicByAttendee(context.Context, uuid.UUID) ([]*eventModel.Event, error) {
	return nil, nil
}
func (f *fakeCalendarEventRepository) ListPublicByCalendar(context.Context, uuid.UUID) ([]*eventModel.Event, error) {
	return f.events, nil
}
func (f *fakeCalendarEventRepository) GetPublicByID(context.Context, uuid.UUID) (*eventModel.Event, error) {
	return nil, nil
}
func (f *fakeCalendarEventRepository) Create(context.Context, *eventModel.Event) error { return nil }

var _ eventRepo.EventRepository = (*fakeCalendarEventRepository)(nil)

func publicCalendar(id uuid.UUID) *calendarModel.Calendar {
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	return &calendarModel.Calendar{ID: id, Name: "Community Calendar", OwnerID: uuid.New(), Visibility: calendarModel.VisibilityPublic, Status: calendarModel.StatusActive, CreatedAt: now, UpdatedAt: now}
}

func TestListPublicReturnsEmptyData(t *testing.T) {
	result, err := NewCalendarUseCase(&fakeCalendarRepository{}, &fakeCalendarEventRepository{}).ListPublic(context.Background(), 0)
	if err != nil {
		t.Fatalf("ListPublic returned error: %v", err)
	}
	if result == nil || result.Data == nil || len(result.Data) != 0 {
		t.Fatalf("unexpected empty response: %#v", result)
	}
}

func TestGetPublicMapsOnlyAssociatedEvents(t *testing.T) {
	id := uuid.New()
	start := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	event := &eventModel.Event{ID: uuid.New(), Title: "Calendar event", StartsAt: start, EndsAt: start.Add(time.Hour), PriceLunas: 125000, Status: eventModel.EventStatusPublished, IsPublic: true, City: "Istanbul"}
	result, err := NewCalendarUseCase(&fakeCalendarRepository{detail: publicCalendar(id)}, &fakeCalendarEventRepository{events: []*eventModel.Event{event}}).GetPublic(context.Background(), id)
	if err != nil {
		t.Fatalf("GetPublic returned error: %v", err)
	}
	if result.Data.ID != id || len(result.Events) != 1 || result.Events[0].PriceNIM != "1.25" {
		t.Fatalf("unexpected calendar response: %#v", result)
	}
}

func TestGetPublicReturnsNotFoundForPrivateOrMissingCalendar(t *testing.T) {
	_, err := NewCalendarUseCase(&fakeCalendarRepository{}, &fakeCalendarEventRepository{}).GetPublic(context.Background(), uuid.New())
	if err == nil || !errors.Is(err, domainErr.ErrNotFound) {
		t.Fatalf("error = %v, want not found", err)
	}
}

func TestCreateUsesAuthenticatedOwnerAndValidatesFields(t *testing.T) {
	ownerID := uuid.New()
	repo := &fakeCalendarRepository{}
	result, err := NewCalendarUseCase(repo, &fakeCalendarEventRepository{}).Create(context.Background(), ownerID, dto.CreateCalendarRequest{Name: "My calendar", Visibility: "public"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.created == nil || repo.created.OwnerID != ownerID || result.Data.ID != repo.created.ID {
		t.Fatalf("owner was not persisted: %#v", repo.created)
	}
	_, err = NewCalendarUseCase(&fakeCalendarRepository{}, &fakeCalendarEventRepository{}).Create(context.Background(), ownerID, dto.CreateCalendarRequest{Name: "", Visibility: "public"})
	if err == nil || !errors.Is(err, domainErr.ErrValidation) {
		t.Fatalf("error = %v, want validation", err)
	}
}

func TestListMineSeparatesOwnedAndFollowed(t *testing.T) {
	owned, followed := publicCalendar(uuid.New()), publicCalendar(uuid.New())
	result, err := NewCalendarUseCase(&fakeCalendarRepository{owned: []*calendarModel.Calendar{owned}, followed: []*calendarModel.Calendar{followed}}, &fakeCalendarEventRepository{}).ListMine(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ListMine returned error: %v", err)
	}
	if len(result.Owned) != 1 || len(result.Followed) != 1 {
		t.Fatalf("unexpected mine response: %#v", result)
	}
}

func TestFollowDelegatesIdempotentRepositoryOperation(t *testing.T) {
	repo := &fakeCalendarRepository{}
	uc := NewCalendarUseCase(repo, &fakeCalendarEventRepository{})
	id, userID := uuid.New(), uuid.New()
	if err := uc.Follow(context.Background(), userID, id); err != nil {
		t.Fatal(err)
	}
	if err := uc.Follow(context.Background(), userID, id); err != nil {
		t.Fatal(err)
	}
	if repo.follows != 2 {
		t.Fatalf("follow calls = %d, want 2 delegated idempotent calls", repo.follows)
	}
}
