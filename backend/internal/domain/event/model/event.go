package model

import (
	"time"

	"github.com/google/uuid"
)

// EventStatus is the lifecycle state of an event. Presentation states such as
// upcoming, past, sold out, and free are derived from event data.
type EventStatus string

const (
	EventStatusDraft     EventStatus = "draft"
	EventStatusPublished EventStatus = "published"
	EventStatusCancelled EventStatus = "cancelled"
)

// Event represents a public or private event in NIMNear.
// PriceLunas is the single canonical monetary representation. One NIM is
// exactly 100,000 Luna.
type Event struct {
	ID            uuid.UUID   `json:"id"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	StartsAt      time.Time   `json:"starts_at"`
	EndsAt        time.Time   `json:"ends_at"`
	Status        EventStatus `json:"status"`
	PriceLunas    int64       `json:"price_lunas"`
	Currency      string      `json:"currency"`
	Capacity      *int        `json:"capacity,omitempty"`
	AttendeeCount int         `json:"attendee_count"`
	ImageURL      string      `json:"image_url"`
	CalendarID    *uuid.UUID  `json:"calendar_id,omitempty"`
	PlaceID       *uuid.UUID  `json:"place_id,omitempty"`
	Latitude      *float64    `json:"latitude,omitempty"`
	Longitude     *float64    `json:"longitude,omitempty"`
	Address       *string     `json:"address,omitempty"`
	City          string      `json:"city"`
	OrganizerID   *uuid.UUID  `json:"organizer_id,omitempty"`
	IsPublic      bool        `json:"is_public"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// IsFree reports whether the exact stored price is zero.
func (e Event) IsFree() bool {
	return e.PriceLunas == 0
}

// IsSoldOut reports whether a capacity is configured and has been reached.
func (e Event) IsSoldOut() bool {
	return e.Capacity != nil && e.AttendeeCount >= *e.Capacity
}

// IsPast reports whether the event has ended at the supplied time.
func (e Event) IsPast(now time.Time) bool {
	return !e.EndsAt.After(now)
}
