package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/iam/repository"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// DeleteAccountUseCase anonymizes the authenticated user's account.
type DeleteAccountUseCase struct {
	repo repository.AccountDeletionRepository
	now  func() time.Time
}

// NewDeleteAccountUseCase creates a DeleteAccountUseCase.
func NewDeleteAccountUseCase(repo repository.AccountDeletionRepository) *DeleteAccountUseCase {
	return &DeleteAccountUseCase{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

// Execute deactivates and anonymizes the current user. The target is always the
// authenticated user ID from session context; callers must not accept a user_id
// from the request body.
func (uc *DeleteAccountUseCase) Execute(ctx context.Context, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return domainErr.New(domainErr.ErrUnauthorized, "user is not authenticated", nil)
	}
	if uc == nil || uc.repo == nil {
		return domainErr.New(domainErr.ErrInternal, "account deletion is not configured", nil)
	}
	return uc.repo.DeleteAccount(ctx, userID, uc.now())
}
