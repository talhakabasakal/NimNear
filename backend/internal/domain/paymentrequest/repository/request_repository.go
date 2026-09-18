package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/model"
)

// RequestRepository owns atomic payment-request creation and server-owned payment transitions.
type RequestRepository interface {
	Create(ctx context.Context, request *model.Request) (*model.Request, error)
	GetByPublicID(ctx context.Context, publicID uuid.UUID, now time.Time) (*model.Request, error)
	GetOwnedByPublicID(ctx context.Context, publicID, creatorID uuid.UUID, now time.Time) (*model.Request, error)
	ListByCreator(ctx context.Context, creatorID uuid.UUID, now time.Time, limit int) ([]*model.Request, error)
	Cancel(ctx context.Context, publicID, creatorID uuid.UUID, now time.Time) (*model.Request, error)
	SubmitTransaction(ctx context.Context, publicID, payerID uuid.UUID, transactionHash string, now, reconciliationDeadline time.Time) (*model.Request, error)
	ClaimVerification(ctx context.Context, id uuid.UUID, now time.Time, retryInterval, claimDuration time.Duration, force bool) (*model.Request, error)
	ListReconciliationCandidates(ctx context.Context, now time.Time, retryInterval, fallbackDeadline time.Duration, limit int) ([]*model.Request, error)
	SetVerificationState(ctx context.Context, id uuid.UUID, status model.Status, now time.Time) (*model.Request, error)
	ExpireUnresolved(ctx context.Context, id uuid.UUID, now time.Time, fallbackDeadline time.Duration) (*model.Request, error)
	ConfirmExpiredRecovery(ctx context.Context, id uuid.UUID, now time.Time) (*model.Request, error)
}
