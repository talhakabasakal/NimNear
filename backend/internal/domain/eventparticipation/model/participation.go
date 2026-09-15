package model

import (
	"time"

	"github.com/google/uuid"
)

// Participation records one user's attendance relationship to an event.
type Participation struct {
	ID        uuid.UUID
	EventID   uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
}

// State is the user-specific participation view returned by RSVP operations.
type State struct {
	EventID       uuid.UUID
	Attending     bool
	AttendeeCount int
	Capacity      *int
}

// IsSoldOut reports whether a configured capacity has been reached.
func (s State) IsSoldOut() bool {
	return s.Capacity != nil && s.AttendeeCount >= *s.Capacity
}
