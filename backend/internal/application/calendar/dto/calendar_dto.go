package dto

import (
	"time"

	"github.com/google/uuid"
	eventdto "github.com/masterfabric-go/masterfabric/internal/application/event/dto"
)

// CalendarInfo is the public, privacy-safe calendar representation.
type CalendarInfo struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	ImageURL    *string   `json:"image_url"`
	Visibility  string    `json:"visibility"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateCalendarRequest is the authenticated calendar creation contract.
type CreateCalendarRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	Visibility  string `json:"visibility"`
}

// CalendarsResponse is a public calendar collection.
type CalendarsResponse struct {
	Data []CalendarInfo `json:"data"`
}

// CalendarResponse is a public calendar detail envelope.
type CalendarResponse struct {
	Data   CalendarInfo         `json:"data"`
	Events []eventdto.EventInfo `json:"events"`
}

// MyCalendarsResponse contains the authenticated user's owned and followed calendars.
type MyCalendarsResponse struct {
	Owned    []CalendarInfo `json:"owned"`
	Followed []CalendarInfo `json:"followed"`
}
