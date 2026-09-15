package model

import (
	"time"

	"github.com/google/uuid"
)

// PublicProfile is the privacy-safe profile read model exposed to other users.
type PublicProfile struct {
	ID                  uuid.UUID
	DisplayName         string
	Username            *string
	Bio                 string
	AvatarURL           string
	JoinedAt            time.Time
	OrganizedEventCount int
	AttendedEventCount  int
}

// ProfilePatch contains only explicitly supplied profile fields.
// A nil value with its corresponding Set flag means the field is cleared.
type ProfilePatch struct {
	DisplayName    *string
	DisplayNameSet bool
	Username       *string
	UsernameSet    bool
	Bio            *string
	BioSet         bool
}

func (p ProfilePatch) Empty() bool {
	return !p.DisplayNameSet && !p.UsernameSet && !p.BioSet
}
