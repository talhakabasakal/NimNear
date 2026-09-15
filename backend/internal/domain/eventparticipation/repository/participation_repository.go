package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventparticipation/model"
)

// ParticipationRepository persists user-to-event attendance relationships.
type ParticipationRepository interface {
	RSVP(ctx context.Context, eventID, userID uuid.UUID, now time.Time) (*model.State, error)
	Cancel(ctx context.Context, eventID, userID uuid.UUID) (*model.State, error)
	GetState(ctx context.Context, eventID, userID uuid.UUID) (*model.State, error)
}
