package usecase

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	eventdto "github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	eventusecase "github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	profiledto "github.com/masterfabric-go/masterfabric/internal/application/profile/dto"
	eventmodel "github.com/masterfabric-go/masterfabric/internal/domain/event/model"
	eventrepo "github.com/masterfabric-go/masterfabric/internal/domain/event/repository"
	"github.com/masterfabric-go/masterfabric/internal/domain/profile/model"
	profilerepo "github.com/masterfabric-go/masterfabric/internal/domain/profile/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const (
	OrganizedEvents = "organized"
	AttendedEvents  = "attended"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_-]{1,28}[a-z0-9])$`)

var reservedUsernames = map[string]struct{}{
	"admin": {}, "support": {}, "nimnear": {}, "api": {}, "me": {},
	"profile": {}, "profiles": {}, "events": {}, "login": {}, "register": {},
}

// ProfileUseCase serves privacy-safe public profile reads and self updates.
type ProfileUseCase struct {
	profileRepo profilerepo.ProfileRepository
	eventRepo   eventrepo.EventRepository
	now         func() time.Time
}

// NewProfileUseCase creates a profile read/update use case.
func NewProfileUseCase(profileRepo profilerepo.ProfileRepository, eventRepo eventrepo.EventRepository) *ProfileUseCase {
	return &ProfileUseCase{
		profileRepo: profileRepo,
		eventRepo:   eventRepo,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

// GetPublic returns a public profile and aggregate event counts.
func (uc *ProfileUseCase) GetPublic(ctx context.Context, id uuid.UUID) (*profiledto.PublicProfileResponse, error) {
	if id == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrBadRequest, "profile id is required", nil)
	}
	if uc.profileRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "profile repository is not configured", nil)
	}
	profile, err := uc.profileRepo.GetPublic(ctx, id)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domainErr.New(domainErr.ErrNotFound, "profile not found", nil)
	}
	return &profiledto.PublicProfileResponse{Data: mapProfile(profile)}, nil
}

// UpdateSelf validates and updates only the authenticated user's profile fields.
func (uc *ProfileUseCase) UpdateSelf(ctx context.Context, id uuid.UUID, patch model.ProfilePatch) (*profiledto.PublicProfileResponse, error) {
	if id == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil)
	}
	if patch.Empty() {
		return nil, domainErr.NewWithCode(domainErr.ErrValidation, "empty_profile_update", "profile update contains no fields", nil)
	}
	if uc.profileRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "profile repository is not configured", nil)
	}

	normalized, err := normalizePatch(patch)
	if err != nil {
		return nil, err
	}
	profile, err := uc.profileRepo.UpdateSelf(ctx, id, normalized)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, domainErr.New(domainErr.ErrNotFound, "profile not found", nil)
	}
	return &profiledto.PublicProfileResponse{Data: mapProfile(profile)}, nil
}

func normalizePatch(patch model.ProfilePatch) (model.ProfilePatch, error) {
	if patch.DisplayNameSet && patch.DisplayName != nil {
		value := strings.TrimSpace(*patch.DisplayName)
		if value == "" || utf8.RuneCountInString(value) > 100 || containsDisallowedControl(value, false) {
			return model.ProfilePatch{}, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_display_name", "display name is invalid", nil)
		}
		patch.DisplayName = &value
	}

	if patch.UsernameSet && patch.Username != nil {
		value := strings.ToLower(strings.TrimSpace(*patch.Username))
		if !usernamePattern.MatchString(value) {
			return model.ProfilePatch{}, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_username", "username is invalid", nil)
		}
		if _, reserved := reservedUsernames[value]; reserved {
			return model.ProfilePatch{}, domainErr.NewWithCode(domainErr.ErrValidation, "reserved_username", "username is reserved", nil)
		}
		patch.Username = &value
	}

	if patch.BioSet && patch.Bio != nil {
		value := strings.ReplaceAll(*patch.Bio, "\r\n", "\n")
		value = strings.ReplaceAll(value, "\r", "\n")
		if utf8.RuneCountInString(value) > 280 {
			return model.ProfilePatch{}, domainErr.NewWithCode(domainErr.ErrValidation, "bio_too_long", "bio is too long", nil)
		}
		if containsDisallowedControl(value, true) {
			return model.ProfilePatch{}, domainErr.NewWithCode(domainErr.ErrValidation, "invalid_bio", "bio is invalid", nil)
		}
		patch.Bio = &value
	}
	return patch, nil
}

func containsDisallowedControl(value string, allowNewline bool) bool {
	for _, r := range value {
		if !unicode.IsControl(r) {
			continue
		}
		if allowNewline && r == '\n' {
			continue
		}
		return true
	}
	return false
}

// ListEvents returns public published events associated with a profile.
func (uc *ProfileUseCase) ListEvents(ctx context.Context, id uuid.UUID, kind string) (*profiledto.ProfileEventsResponse, error) {
	if id == uuid.Nil {
		return nil, domainErr.New(domainErr.ErrBadRequest, "profile id is required", nil)
	}
	kind = strings.TrimSpace(kind)
	if kind != OrganizedEvents && kind != AttendedEvents {
		return nil, domainErr.New(domainErr.ErrBadRequest, "type must be organized or attended", nil)
	}
	if _, err := uc.GetPublic(ctx, id); err != nil {
		return nil, err
	}
	if uc.eventRepo == nil {
		return nil, domainErr.New(domainErr.ErrInternal, "event repository is not configured", nil)
	}

	var (
		events []*eventmodel.Event
		err    error
	)
	if kind == OrganizedEvents {
		events, err = uc.eventRepo.ListPublicByOrganizer(ctx, id)
	} else {
		events, err = uc.eventRepo.ListPublicByAttendee(ctx, id)
	}
	if err != nil {
		return nil, err
	}

	now := uc.now().UTC()
	data := make([]eventdto.EventInfo, 0, len(events))
	for _, event := range events {
		if event != nil {
			data = append(data, eventusecase.MapEvent(event, now))
		}
	}
	return &profiledto.ProfileEventsResponse{Data: data}, nil
}

func mapProfile(profile *model.PublicProfile) profiledto.PublicProfileInfo {
	return profiledto.PublicProfileInfo{
		ID:                  profile.ID,
		DisplayName:         profile.DisplayName,
		Username:            profile.Username,
		Bio:                 profile.Bio,
		AvatarURL:           profile.AvatarURL,
		JoinedAt:            profile.JoinedAt,
		OrganizedEventCount: profile.OrganizedEventCount,
		AttendedEventCount:  profile.AttendedEventCount,
	}
}
