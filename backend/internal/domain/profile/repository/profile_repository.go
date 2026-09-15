package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/profile/model"
)

// ProfileRepository reads and updates privacy-safe profile data.
type ProfileRepository interface {
	GetPublic(ctx context.Context, id uuid.UUID) (*model.PublicProfile, error)
	UpdateSelf(ctx context.Context, id uuid.UUID, patch model.ProfilePatch) (*model.PublicProfile, error)
}
