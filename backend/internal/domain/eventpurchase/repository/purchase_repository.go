package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
)

// PurchaseRepository owns atomic purchase creation, ownership, and payment state transitions.
type PurchaseRepository interface {
	Create(ctx context.Context, eventID, userID uuid.UUID, now time.Time, holdDuration time.Duration) (*model.Purchase, error)
	GetOwned(ctx context.Context, purchaseID, userID uuid.UUID) (*model.Purchase, error)
	GetActiveForEvent(ctx context.Context, eventID, userID uuid.UUID) (*model.Purchase, error)
	SubmitTransaction(ctx context.Context, purchaseID, userID uuid.UUID, transactionHash string, now time.Time) (*model.Purchase, error)
	SetVerificationState(ctx context.Context, purchaseID, userID uuid.UUID, status model.Status, now time.Time) (*model.Purchase, error)
}
