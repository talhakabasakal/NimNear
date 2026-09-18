package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/model"
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

// OptionalInt and OptionalUUID preserve explicit null in event PATCH requests.
type OptionalInt struct {
	Set   bool
	Value *int
}

func (v *OptionalInt) UnmarshalJSON(data []byte) error {
	v.Set = true
	if string(data) == "null" {
		v.Value = nil
		return nil
	}
	var value int
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	v.Value = &value
	return nil
}

type OptionalUUID struct {
	Set   bool
	Value *uuid.UUID
}

func (v *OptionalUUID) UnmarshalJSON(data []byte) error {
	v.Set = true
	if string(data) == "null" {
		v.Value = nil
		return nil
	}
	var value uuid.UUID
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	v.Value = &value
	return nil
}

// UpdateEventRequest contains only organizer-editable, non-financial fields.
type UpdateEventRequest struct {
	Title       *string      `json:"title"`
	Description *string      `json:"description"`
	StartsAt    *time.Time   `json:"starts_at"`
	EndsAt      *time.Time   `json:"ends_at"`
	ImageURL    *string      `json:"image_url"`
	Capacity    OptionalInt  `json:"capacity"`
	PlaceID     OptionalUUID `json:"place_id"`
}

func (r UpdateEventRequest) Empty() bool {
	return r.Title == nil && r.Description == nil && r.StartsAt == nil && r.EndsAt == nil && r.ImageURL == nil && !r.Capacity.Set && !r.PlaceID.Set
}

func (r UpdateEventRequest) Patch() model.Patch {
	patch := model.Patch{Capacity: r.Capacity.Value, CapacitySet: r.Capacity.Set, PlaceID: r.PlaceID.Value, PlaceIDSet: r.PlaceID.Set}
	if r.Title != nil {
		patch.Title, patch.TitleSet = *r.Title, true
	}
	if r.Description != nil {
		patch.Description, patch.DescriptionSet = *r.Description, true
	}
	if r.StartsAt != nil {
		patch.StartsAt, patch.StartsAtSet = r.StartsAt.UTC(), true
	}
	if r.EndsAt != nil {
		patch.EndsAt, patch.EndsAtSet = r.EndsAt.UTC(), true
	}
	if r.ImageURL != nil {
		patch.ImageURL, patch.ImageURLSet = *r.ImageURL, true
	}
	return patch
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
