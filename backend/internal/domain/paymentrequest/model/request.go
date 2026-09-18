package model

import (
	"time"

	"github.com/google/uuid"
)

// Status is the server-owned lifecycle of a payment request.
type Status string

const (
	StatusPending   Status = "pending"
	StatusSubmitted Status = "submitted"
	StatusVerifying Status = "verifying"
	StatusPaid      Status = "paid"
	StatusFailed    Status = "failed"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

// Request is a shareable NIM payment request. Amount, recipient, and status are never client-owned.
//
// pending is the only payable state. submitted and verifying mean a transaction hash was
// accepted and is waiting for RPC finality; they are not paid. paid, failed, expired, and
// cancelled are terminal. Public reads treat pending rows with now >= expires_at as expired
// even if a worker has not persisted that status yet.
type Request struct {
	ID                        uuid.UUID
	PublicID                  uuid.UUID
	CreatorUserID             uuid.UUID
	RecipientAddress          string
	AmountLunas               int64
	Note                      *string
	Status                    Status
	ExpiresAt                 time.Time
	CancelledAt               *time.Time
	PaidAt                    *time.Time
	PayerUserID               *uuid.UUID
	TransactionHash           *string
	ReconciliationDeadlineAt  *time.Time
	LastVerificationAttemptAt *time.Time
	VerificationClaimedUntil  *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// EffectiveStatus applies the expiry rule without requiring a persisted status update.
func (r *Request) EffectiveStatus(now time.Time) Status {
	if r == nil {
		return ""
	}
	if r.Status == StatusPending && !now.Before(r.ExpiresAt) {
		return StatusExpired
	}
	return r.Status
}

// IsTerminal reports whether the persisted status can no longer change except via expiry persistence.
func (r *Request) IsTerminal() bool {
	if r == nil {
		return false
	}
	switch r.Status {
	case StatusPaid, StatusFailed, StatusExpired, StatusCancelled:
		return true
	default:
		return false
	}
}
