package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	eventdto "github.com/masterfabric-go/masterfabric/internal/application/event/dto"
	"github.com/masterfabric-go/masterfabric/internal/domain/profile/model"
)

// PublicProfileInfo contains only fields safe for public profile views.
type PublicProfileInfo struct {
	ID                  uuid.UUID `json:"id"`
	DisplayName         string    `json:"display_name"`
	Username            *string   `json:"username"`
	Bio                 *string   `json:"bio"`
	AvatarURL           *string   `json:"avatar_url"`
	JoinedAt            time.Time `json:"joined_at"`
	OrganizedEventCount int       `json:"organized_event_count"`
	AttendedEventCount  int       `json:"attended_event_count"`
}

// PublicProfileResponse is the response envelope for a public or self profile.
type PublicProfileResponse struct {
	Data PublicProfileInfo `json:"data"`
}

// ProfileEventsResponse is the public organized or attended event response.
type ProfileEventsResponse struct {
	Data []eventdto.EventInfo `json:"data"`
}

// PatchString preserves the difference between an omitted JSON field and null.
type PatchString struct {
	Set   bool
	Value *string
}

func (p *PatchString) UnmarshalJSON(data []byte) error {
	p.Set = true
	if string(data) == "null" {
		p.Value = nil
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	p.Value = &value
	return nil
}

// UpdateProfileRequest contains only fields allowed by PATCH /api/v1/me/profile.
type UpdateProfileRequest struct {
	DisplayName PatchString `json:"display_name"`
	Username    PatchString `json:"username"`
	Bio         PatchString `json:"bio"`
}

func (r UpdateProfileRequest) Empty() bool {
	return !r.DisplayName.Set && !r.Username.Set && !r.Bio.Set
}

func (r UpdateProfileRequest) Patch() model.ProfilePatch {
	return model.ProfilePatch{
		DisplayName:    r.DisplayName.Value,
		DisplayNameSet: r.DisplayName.Set,
		Username:       r.Username.Value,
		UsernameSet:    r.Username.Set,
		Bio:            r.Bio.Value,
		BioSet:         r.Bio.Set,
	}
}
