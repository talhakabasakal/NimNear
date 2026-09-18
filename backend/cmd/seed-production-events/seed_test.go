package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	eventUC "github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	eventmodel "github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	eventrepo "github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
	iammodel "github.com/masterfabric-go/masterfabric/internal/domain/iam/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/validator"
)

type fakeUsers struct {
	user *iammodel.User
	err  error
}

func (f *fakeUsers) GetByID(context.Context, uuid.UUID) (*iammodel.User, error) {
	return f.user, f.err
}

type fakeEvents struct {
	created []*dto.CreateEventRequest
	err     error
}

func (f *fakeEvents) Create(_ context.Context, organizerID uuid.UUID, req dto.CreateEventRequest) (*dto.EventResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	copyReq := req
	f.created = append(f.created, &copyReq)
	id := uuid.New()
	return &dto.EventResponse{Data: dto.EventInfo{ID: id, Title: req.Title, OrganizerID: &organizerID}}, nil
}

type fakeExisting struct {
	events []*eventmodel.Event
	err    error
}

func (f *fakeExisting) ListPublicByOrganizer(context.Context, uuid.UUID) ([]*eventmodel.Event, error) {
	return f.events, f.err
}

type recordingRepo struct {
	created []*eventmodel.Event
}

func (r *recordingRepo) ListPublic(context.Context, eventrepo.ListFilter) ([]*eventmodel.Event, error) {
	return nil, nil
}
func (r *recordingRepo) ListPublicByOrganizer(context.Context, uuid.UUID) ([]*eventmodel.Event, error) {
	return nil, nil
}
func (r *recordingRepo) ListPublicByAttendee(context.Context, uuid.UUID) ([]*eventmodel.Event, error) {
	return nil, nil
}
func (r *recordingRepo) ListPublicByCalendar(context.Context, uuid.UUID) ([]*eventmodel.Event, error) {
	return nil, nil
}
func (r *recordingRepo) GetPublicByID(context.Context, uuid.UUID) (*eventmodel.Event, error) {
	return nil, nil
}
func (r *recordingRepo) Create(_ context.Context, event *eventmodel.Event) error {
	r.created = append(r.created, event)
	return nil
}

func activeOrganizer() *iammodel.User {
	return &iammodel.User{ID: uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"), Status: iammodel.UserStatusActive}
}

func testSeeder(users *fakeUsers, events *fakeEvents, existing *fakeExisting, production bool) (*seeder, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return &seeder{
		isProduction: production,
		now:          time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC),
		users:        users,
		events:       events,
		existing:     existing,
		verifyImage:  func(context.Context, string) error { return nil },
		stdout:       out,
	}, out
}

func TestCatalogHasTenDistinctFutureEvents(t *testing.T) {
	catalog := catalogEvents()
	if len(catalog) != 10 {
		t.Fatalf("catalog length=%d want 10", len(catalog))
	}
	loc, err := time.LoadLocation(catalogTZ)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	titles := map[string]struct{}{}
	images := map[string]struct{}{}
	days := map[int]struct{}{}
	free, paid, smoke := 0, 0, 0
	for _, item := range catalog {
		if _, exists := titles[normalizeTitle(item.Title)]; exists {
			t.Fatalf("duplicate title %q", item.Title)
		}
		titles[normalizeTitle(item.Title)] = struct{}{}
		if _, exists := images[item.ImageURL]; exists {
			t.Fatalf("duplicate image for %q", item.Title)
		}
		images[item.ImageURL] = struct{}{}
		days[item.DayOffset] = struct{}{}
		if item.PriceNIM == "" || item.PriceNIM == "0" {
			free++
		} else {
			paid++
		}
		if item.SmokeTest {
			smoke++
			if item.Title != "Indie Music Night" || item.PriceNIM != lunaSmoke {
				t.Fatalf("smoke-test event=%q price=%q", item.Title, item.PriceNIM)
			}
		}
		starts, ends := item.schedule(now, loc)
		if !starts.After(now) || !ends.After(starts) {
			t.Fatalf("%q schedule invalid starts=%s ends=%s", item.Title, starts, ends)
		}
		if item.Latitude < 40.9 || item.Latitude > 41.2 || item.Longitude < 28.8 || item.Longitude > 29.2 {
			t.Fatalf("%q coordinates outside Istanbul bounds", item.Title)
		}
		if err := validateCatalogItem(item); err != nil {
			t.Fatal(err)
		}
	}
	if free != 3 || paid != 7 || smoke != 1 {
		t.Fatalf("free=%d paid=%d smoke=%d", free, paid, smoke)
	}
	if len(days) != 10 {
		t.Fatalf("events must be on distinct day offsets, got %d", len(days))
	}
}

func TestCatalogPassesEventUseCaseValidation(t *testing.T) {
	loc, err := time.LoadLocation(catalogTZ)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	repo := &recordingRepo{}
	uc := eventUC.NewEventUseCase(repo)
	organizerID := uuid.New()
	for _, item := range catalogEvents() {
		starts, ends := item.schedule(now, loc)
		if _, err := uc.Create(context.Background(), organizerID, item.request(starts, ends)); err != nil {
			t.Fatalf("Create rejected %q: %v", item.Title, err)
		}
	}
	if len(repo.created) != 10 {
		t.Fatalf("created=%d want 10", len(repo.created))
	}
	for _, event := range repo.created {
		if event.Status != eventmodel.EventStatusPublished || !event.IsPublic {
			t.Fatalf("%q status=%s public=%v", event.Title, event.Status, event.IsPublic)
		}
		if event.OrganizerID == nil || *event.OrganizerID != organizerID {
			t.Fatalf("%q missing organizer", event.Title)
		}
		if event.Title == "Indie Music Night" && event.PriceLunas != 1 {
			t.Fatalf("Indie Music Night price_lunas=%d want 1", event.PriceLunas)
		}
	}
}

func TestApplyRequiresOrganizer(t *testing.T) {
	s, _ := testSeeder(&fakeUsers{user: activeOrganizer()}, &fakeEvents{}, &fakeExisting{}, true)
	err := s.apply(context.Background(), options{production: true})
	if err == nil || !strings.Contains(err.Error(), "organizer-id is required") {
		t.Fatalf("error=%v", err)
	}
}

func TestApplyRefusesProductionWriteWithoutFlag(t *testing.T) {
	s, _ := testSeeder(&fakeUsers{user: activeOrganizer()}, &fakeEvents{}, &fakeExisting{}, true)
	err := s.apply(context.Background(), options{organizerID: activeOrganizer().ID})
	if err == nil || !strings.Contains(err.Error(), "--production") {
		t.Fatalf("error=%v", err)
	}
}

func TestApplyRejectsProductionFlagOutsideProduction(t *testing.T) {
	s, _ := testSeeder(&fakeUsers{user: activeOrganizer()}, &fakeEvents{}, &fakeExisting{}, false)
	err := s.apply(context.Background(), options{organizerID: activeOrganizer().ID, production: true})
	if err == nil || !strings.Contains(err.Error(), "APP_ENV=production") {
		t.Fatalf("error=%v", err)
	}
}

func TestApplyDryRunDoesNotCreate(t *testing.T) {
	events := &fakeEvents{}
	s, out := testSeeder(&fakeUsers{user: activeOrganizer()}, events, &fakeExisting{}, true)
	if err := s.apply(context.Background(), options{organizerID: activeOrganizer().ID, dryRun: true}); err != nil {
		t.Fatal(err)
	}
	if len(events.created) != 0 {
		t.Fatalf("dry-run created %d events", len(events.created))
	}
	if !strings.Contains(out.String(), "dry-run: no events were created") {
		t.Fatalf("missing dry-run confirmation: %s", out.String())
	}
	if !strings.Contains(out.String(), "Indie Music Night") || !strings.Contains(out.String(), "1-Luna smoke-test") {
		t.Fatalf("plan missing smoke-test event: %s", out.String())
	}
}

func TestApplyIsIdempotentByTitle(t *testing.T) {
	events := &fakeEvents{}
	existing := &fakeExisting{events: []*eventmodel.Event{
		{ID: uuid.New(), Title: "Bosphorus Sunset Meetup"},
		{ID: uuid.New(), Title: "indie music night"},
	}}
	s, out := testSeeder(&fakeUsers{user: activeOrganizer()}, events, existing, true)
	if err := s.apply(context.Background(), options{organizerID: activeOrganizer().ID, production: true}); err != nil {
		t.Fatal(err)
	}
	if len(events.created) != 8 {
		t.Fatalf("created=%d want 8", len(events.created))
	}
	if !strings.Contains(out.String(), "done created=8 skipped=2") {
		t.Fatalf("summary=%s", out.String())
	}
}

func TestApplyRejectsInactiveOrganizer(t *testing.T) {
	user := activeOrganizer()
	user.Status = iammodel.UserStatusInactive
	s, _ := testSeeder(&fakeUsers{user: user}, &fakeEvents{}, &fakeExisting{}, true)
	err := s.apply(context.Background(), options{organizerID: user.ID, dryRun: true})
	if err == nil || !strings.Contains(err.Error(), "inactive") {
		t.Fatalf("error=%v", err)
	}
}

func TestApplyRejectsMissingOrganizer(t *testing.T) {
	s, _ := testSeeder(&fakeUsers{err: domainErr.New(domainErr.ErrNotFound, "user not found", nil)}, &fakeEvents{}, &fakeExisting{}, true)
	err := s.apply(context.Background(), options{organizerID: activeOrganizer().ID, dryRun: true})
	if err == nil || !strings.Contains(err.Error(), "organizer lookup failed") {
		t.Fatalf("error=%v", err)
	}
}

func TestApplyStopsWhenImageDoesNotResolve(t *testing.T) {
	events := &fakeEvents{}
	s, _ := testSeeder(&fakeUsers{user: activeOrganizer()}, events, &fakeExisting{}, true)
	s.verifyImage = func(context.Context, string) error { return errors.New("404") }
	err := s.apply(context.Background(), options{organizerID: activeOrganizer().ID, production: true})
	if err == nil || !strings.Contains(err.Error(), "did not resolve") {
		t.Fatalf("error=%v", err)
	}
	if len(events.created) != 0 {
		t.Fatalf("created events after image failure")
	}
}

func TestCatalogImageURLsAreAbsoluteHTTPS(t *testing.T) {
	for _, item := range catalogEvents() {
		if !strings.HasPrefix(item.ImageURL, "https://images.unsplash.com/") {
			t.Fatalf("%q image host %s", item.Title, item.ImageURL)
		}
		if !validator.ValidHTTPSMediaURL(item.ImageURL) {
			t.Fatalf("%q invalid image URL", item.Title)
		}
	}
}
