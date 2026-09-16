package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	eventmodel "github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	eventrepo "github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
	profilemodel "github.com/masterfabric-go/masterfabric/internal/domain/profile/model"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

type fakeProfileRepository struct {
	profile      *profilemodel.PublicProfile
	err          error
	updatedPatch *profilemodel.ProfilePatch
}

func (f *fakeProfileRepository) GetPublic(context.Context, uuid.UUID) (*profilemodel.PublicProfile, error) {
	return f.profile, f.err
}

func (f *fakeProfileRepository) UpdateSelf(_ context.Context, _ uuid.UUID, patch profilemodel.ProfilePatch) (*profilemodel.PublicProfile, error) {
	f.updatedPatch = &patch
	return f.profile, f.err
}

type fakeProfileEventRepository struct {
	organized []*eventmodel.Event
	attended  []*eventmodel.Event
}

func (f fakeProfileEventRepository) ListPublic(context.Context, eventrepo.ListFilter) ([]*eventmodel.Event, error) {
	return nil, nil
}

func (f fakeProfileEventRepository) ListPublicByOrganizer(context.Context, uuid.UUID) ([]*eventmodel.Event, error) {
	return f.organized, nil
}

func (f fakeProfileEventRepository) ListPublicByAttendee(context.Context, uuid.UUID) ([]*eventmodel.Event, error) {
	return f.attended, nil
}

func (f fakeProfileEventRepository) ListPublicByCalendar(context.Context, uuid.UUID) ([]*eventmodel.Event, error) {
	return nil, nil
}

func (f fakeProfileEventRepository) GetPublicByID(context.Context, uuid.UUID) (*eventmodel.Event, error) {
	return nil, nil
}

func (f fakeProfileEventRepository) Create(context.Context, *eventmodel.Event) error {
	return nil
}

func testProfile(id uuid.UUID) *profilemodel.PublicProfile {
	return &profilemodel.PublicProfile{ID: id, DisplayName: "Integration Tester", JoinedAt: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)}
}

func TestGetPublicMapsPrivacySafeProfile(t *testing.T) {
	id := uuid.New()
	username := "tester"
	profile := testProfile(id)
	profile.Username = &username
	profile.Bio = "Local profile"
	profile.OrganizedEventCount = 2
	profile.AttendedEventCount = 1

	uc := NewProfileUseCase(&fakeProfileRepository{profile: profile}, fakeProfileEventRepository{})
	result, err := uc.GetPublic(context.Background(), id)
	if err != nil {
		t.Fatalf("GetPublic returned error: %v", err)
	}
	if result.Data.ID != id || result.Data.DisplayName != "Integration Tester" || result.Data.Username == nil || *result.Data.Username != username {
		t.Fatalf("unexpected public profile: %#v", result.Data)
	}
	if result.Data.OrganizedEventCount != 2 || result.Data.AttendedEventCount != 1 {
		t.Fatalf("unexpected profile counts: %#v", result.Data)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	if string(encoded) == "" || contains(string(encoded), "email") || contains(string(encoded), "password") {
		t.Fatalf("private fields leaked: %s", encoded)
	}
}

func TestGetPublicMapsMissingAndUnsafeMediaToNull(t *testing.T) {
	id := uuid.New()
	profile := testProfile(id)
	profile.AvatarURL = "javascript:alert(1)"

	result, err := NewProfileUseCase(&fakeProfileRepository{profile: profile}, fakeProfileEventRepository{}).GetPublic(context.Background(), id)
	if err != nil {
		t.Fatalf("GetPublic returned error: %v", err)
	}
	if result.Data.Bio != nil || result.Data.AvatarURL != nil {
		t.Fatalf("missing bio or unsafe avatar was exposed: %#v", result.Data)
	}

	profile.Bio = "Public bio"
	profile.AvatarURL = "https://media.example/avatar.png"
	result, err = NewProfileUseCase(&fakeProfileRepository{profile: profile}, fakeProfileEventRepository{}).GetPublic(context.Background(), id)
	if err != nil {
		t.Fatalf("GetPublic with valid media returned error: %v", err)
	}
	if result.Data.Bio == nil || *result.Data.Bio != profile.Bio || result.Data.AvatarURL == nil || *result.Data.AvatarURL != profile.AvatarURL {
		t.Fatalf("valid optional profile values were not preserved: %#v", result.Data)
	}
}

func TestUpdateSelfNormalizesDisplayNameUsernameAndBio(t *testing.T) {
	id := uuid.New()
	repo := &fakeProfileRepository{profile: testProfile(id)}
	uc := NewProfileUseCase(repo, fakeProfileEventRepository{})

	displayName := "  İzmir / NIMNear  "
	username := "  User_Name-1  "
	bio := "line one\r\nline two"
	result, err := uc.UpdateSelf(context.Background(), id, profilemodel.ProfilePatch{
		DisplayName: displayNamePtr(displayName), DisplayNameSet: true,
		Username: usernamePtr(username), UsernameSet: true,
		Bio: bioPtr(bio), BioSet: true,
	})
	if err != nil {
		t.Fatalf("UpdateSelf returned error: %v", err)
	}
	if repo.updatedPatch == nil || *repo.updatedPatch.DisplayName != "İzmir / NIMNear" || *repo.updatedPatch.Username != "user_name-1" || *repo.updatedPatch.Bio != "line one\nline two" {
		t.Fatalf("unexpected normalized patch: %#v", repo.updatedPatch)
	}
	if result.Data.ID != id {
		t.Fatalf("unexpected response: %#v", result.Data)
	}
}

func TestUpdateSelfRejectsInvalidAndReservedUsernames(t *testing.T) {
	tests := []struct {
		name  string
		value string
		code  string
	}{
		{"invalid format", "ab", "invalid_username"},
		{"invalid characters", "user.name", "invalid_username"},
		{"reserved", "Admin", "reserved_username"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := NewProfileUseCase(&fakeProfileRepository{profile: testProfile(uuid.New())}, fakeProfileEventRepository{})
			_, err := uc.UpdateSelf(context.Background(), uuid.New(), profilemodel.ProfilePatch{Username: usernamePtr(tt.value), UsernameSet: true})
			if err == nil {
				t.Fatal("UpdateSelf returned nil error")
			}
			var profileError *domainErr.DomainError
			if !errors.As(err, &profileError) || profileError.Code != tt.code || !errors.Is(err, domainErr.ErrValidation) {
				t.Fatalf("error = %v, want code %s", err, tt.code)
			}
		})
	}
}

func TestUpdateSelfRejectsLongBioAndEmptyPatch(t *testing.T) {
	uc := NewProfileUseCase(&fakeProfileRepository{profile: testProfile(uuid.New())}, fakeProfileEventRepository{})
	_, err := uc.UpdateSelf(context.Background(), uuid.New(), profilemodel.ProfilePatch{Bio: bioPtr(strings.Repeat("a", 281)), BioSet: true})
	if err == nil || !errors.Is(err, domainErr.ErrValidation) {
		t.Fatalf("long bio error = %v, want validation", err)
	}
	var profileError *domainErr.DomainError
	if !errors.As(err, &profileError) || profileError.Code != "bio_too_long" {
		t.Fatalf("long bio code = %v, want bio_too_long", err)
	}

	_, err = uc.UpdateSelf(context.Background(), uuid.New(), profilemodel.ProfilePatch{})
	if err == nil || !errors.Is(err, domainErr.ErrValidation) {
		t.Fatalf("empty patch error = %v, want validation", err)
	}
	if !errors.As(err, &profileError) || profileError.Code != "empty_profile_update" {
		t.Fatalf("empty patch code = %v, want empty_profile_update", err)
	}
}

func TestUpdateSelfClearsNullableFieldsAndPreservesOmittedFields(t *testing.T) {
	id := uuid.New()
	username := "existing"
	profile := testProfile(id)
	profile.Username = &username
	profile.Bio = "existing bio"
	repo := &fakeProfileRepository{profile: profile}
	uc := NewProfileUseCase(repo, fakeProfileEventRepository{})

	_, err := uc.UpdateSelf(context.Background(), id, profilemodel.ProfilePatch{
		DisplayNameSet: true,
		UsernameSet:    true,
		BioSet:         true,
	})
	if err != nil {
		t.Fatalf("clear update returned error: %v", err)
	}
	if repo.updatedPatch == nil || repo.updatedPatch.DisplayName != nil || repo.updatedPatch.Username != nil || repo.updatedPatch.Bio != nil || !repo.updatedPatch.DisplayNameSet || !repo.updatedPatch.UsernameSet || !repo.updatedPatch.BioSet {
		t.Fatalf("clear patch was not preserved: %#v", repo.updatedPatch)
	}

	_, err = uc.UpdateSelf(context.Background(), id, profilemodel.ProfilePatch{Bio: bioPtr("new bio"), BioSet: true})
	if err != nil {
		t.Fatalf("partial update returned error: %v", err)
	}
	if repo.updatedPatch == nil || !repo.updatedPatch.BioSet || repo.updatedPatch.DisplayNameSet || repo.updatedPatch.UsernameSet {
		t.Fatalf("omitted fields were not preserved as omitted: %#v", repo.updatedPatch)
	}
}

func TestUpdateSelfMapsUsernameConflict(t *testing.T) {
	repo := &fakeProfileRepository{profile: testProfile(uuid.New()), err: domainErr.NewWithCode(domainErr.ErrAlreadyExists, "username_taken", "username is already taken", nil)}
	uc := NewProfileUseCase(repo, fakeProfileEventRepository{})
	_, err := uc.UpdateSelf(context.Background(), uuid.New(), profilemodel.ProfilePatch{Username: usernamePtr("taken"), UsernameSet: true})
	if err == nil || !errors.Is(err, domainErr.ErrAlreadyExists) || domainErr.HTTPStatusCode(err) != 409 {
		t.Fatalf("error = %v, want conflict", err)
	}
}

func TestListEventsRejectsUnknownTypeAndMapsOrganizedEvents(t *testing.T) {
	id := uuid.New()
	event := &eventmodel.Event{ID: uuid.New(), Title: "Public event", StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour), Status: eventmodel.EventStatusPublished, PriceLunas: 0, Currency: "NIM", City: "Istanbul", IsPublic: true}
	repo := fakeProfileEventRepository{organized: []*eventmodel.Event{event}}
	uc := NewProfileUseCase(&fakeProfileRepository{profile: testProfile(id)}, repo)

	if _, err := uc.ListEvents(context.Background(), id, "unknown"); err == nil || !errors.Is(err, domainErr.ErrBadRequest) {
		t.Fatalf("ListEvents error = %v, want bad request", err)
	}
	result, err := uc.ListEvents(context.Background(), id, OrganizedEvents)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(result.Data) != 1 || result.Data[0].ID != event.ID {
		t.Fatalf("unexpected organized events: %#v", result.Data)
	}
}

func displayNamePtr(value string) *string { return &value }
func usernamePtr(value string) *string    { return &value }
func bioPtr(value string) *string         { return &value }

func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
