package dto

import "github.com/google/uuid"

// ParticipationInfo is the authenticated user's participation state for an event.
type ParticipationInfo struct {
	EventID       uuid.UUID `json:"event_id"`
	Attending     bool      `json:"attending"`
	AttendeeCount int       `json:"attendee_count"`
	Capacity      *int      `json:"capacity,omitempty"`
	IsSoldOut     bool      `json:"is_sold_out"`
}

// ParticipationResponse is the response envelope for participation operations.
type ParticipationResponse struct {
	Data ParticipationInfo `json:"data"`
}
