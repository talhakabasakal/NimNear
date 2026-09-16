package dto

import (
	"time"

	"github.com/google/uuid"
)

// ListEventsQuery contains the supported public event discovery filters.
type ListEventsQuery struct {
	City    string
	PlaceID *uuid.UUID
	From    *time.Time
	To      *time.Time
	Limit   int
}

// CreateEventRequest is the authenticated event creation contract.
// PriceNIM is a human-readable decimal string at the API boundary. The
// backend converts it exactly to canonical Luna; an omitted price is free.
type CreateEventRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      time.Time  `json:"ends_at"`
	PriceNIM    string     `json:"price_nim"`
	Currency    string     `json:"currency"`
	Capacity    *int       `json:"capacity,omitempty"`
	ImageURL    string     `json:"image_url"`
	CalendarID  *uuid.UUID `json:"calendar_id,omitempty"`
	PlaceID     *uuid.UUID `json:"place_id,omitempty"`
	Latitude    *float64   `json:"latitude,omitempty"`
	Longitude   *float64   `json:"longitude,omitempty"`
	Address     *string    `json:"address,omitempty"`
	City        string     `json:"city"`
}

// EventInfo is the explicit public representation of an event.
type EventInfo struct {
	ID            uuid.UUID  `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	StartsAt      time.Time  `json:"starts_at"`
	EndsAt        time.Time  `json:"ends_at"`
	Status        string     `json:"status"`
	PriceNIM      string     `json:"price_nim"`
	Currency      string     `json:"currency"`
	Capacity      *int       `json:"capacity"`
	AttendeeCount int        `json:"attendee_count"`
	ImageURL      *string    `json:"image_url"`
	CalendarID    *uuid.UUID `json:"calendar_id"`
	PlaceID       *uuid.UUID `json:"place_id"`
	Latitude      *float64   `json:"latitude"`
	Longitude     *float64   `json:"longitude"`
	Address       *string    `json:"address"`
	City          string     `json:"city"`
	OrganizerID   *uuid.UUID `json:"organizer_id"`
	IsFree        bool       `json:"is_free"`
	IsSoldOut     bool       `json:"is_sold_out"`
	IsPast        bool       `json:"is_past"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// EventsResponse is the response envelope for public event discovery.
type EventsResponse struct {
	Data []EventInfo `json:"data"`
}

// EventResponse is the response envelope for an event detail or creation.
type EventResponse struct {
	Data EventInfo `json:"data"`
}
