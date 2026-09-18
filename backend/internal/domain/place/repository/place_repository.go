package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/place/model"
)

// PlaceRepository defines persistence operations for place discovery.
type PlaceRepository interface {
	ListNearby(ctx context.Context, latitude, longitude, radiusMeters float64, limit int) ([]*model.NearbyPlace, error)
	GetActiveByID(ctx context.Context, id uuid.UUID) (*model.Place, error)
}

// OperatorRepository is the operator-managed inventory surface. It is not a
// public product API. UUID primary keys are the authoritative identity.
type OperatorRepository interface {
	Create(ctx context.Context, place *model.Place) error
	Update(ctx context.Context, place *model.Place) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Place, error)
	List(ctx context.Context, limit int) ([]*model.Place, error)
	ListByName(ctx context.Context, name string, limit int) ([]*model.Place, error)
}
