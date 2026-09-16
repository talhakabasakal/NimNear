package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/event/model"
)

// ListFilter contains SQL-applied filters for public event discovery.
type ListFilter struct {
	City    string
	PlaceID *uuid.UUID
	From    *time.Time
	To      *time.Time
	Limit   int
}

// EventRepository defines persistence operations for event discovery and creation.
type EventRepository interface {
	ListPublic(ctx context.Context, filter ListFilter) ([]*model.Event, error)
	ListPublicByOrganizer(ctx context.Context, organizerID uuid.UUID) ([]*model.Event, error)
	ListPublicByAttendee(ctx context.Context, userID uuid.UUID) ([]*model.Event, error)
	ListPublicByCalendar(ctx context.Context, calendarID uuid.UUID) ([]*model.Event, error)
	GetPublicByID(ctx context.Context, id uuid.UUID) (*model.Event, error)
	Create(ctx context.Context, event *model.Event) error
}
