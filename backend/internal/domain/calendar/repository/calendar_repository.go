package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/calendar/model"
)

// CalendarRepository defines public discovery and authenticated calendar ownership/follow operations.
type CalendarRepository interface {
	ListPublic(ctx context.Context, limit int) ([]*model.Calendar, error)
	GetPublicByID(ctx context.Context, id uuid.UUID) (*model.Calendar, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Calendar, error)
	ListOwned(ctx context.Context, ownerID uuid.UUID) ([]*model.Calendar, error)
	ListFollowed(ctx context.Context, userID uuid.UUID) ([]*model.Calendar, error)
	Create(ctx context.Context, calendar *model.Calendar) error
	Follow(ctx context.Context, calendarID, userID uuid.UUID) error
	Unfollow(ctx context.Context, calendarID, userID uuid.UUID) error
}
