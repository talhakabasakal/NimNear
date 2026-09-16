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
