package model

import (
	"time"

	"github.com/google/uuid"
)

// Status is the server-owned lifecycle of a paid event purchase.
type Status string

const (
	StatusPending   Status = "pending"
	StatusSubmitted Status = "submitted"
	StatusVerifying Status = "verifying"
	StatusConfirmed Status = "confirmed"
	StatusFailed    Status = "failed"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

// Purchase is a server-side payment record. Amount and status are never client-owned.
type Purchase struct {
	ID                    uuid.UUID
	EventID               uuid.UUID
	UserID                uuid.UUID
	AmountLunas           int64
	Status                Status
	CapacityHoldExpiresAt *time.Time
	TransactionHash       *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	ConfirmedAt           *time.Time
}
