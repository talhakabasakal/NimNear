package verification

import (
	"context"

	"github.com/google/uuid"
)

// Outcome is the server-side result of checking an on-chain transaction.
type Outcome string

const (
	OutcomeNotFound  Outcome = "not_found"
	OutcomeNotFinal  Outcome = "not_final"
	OutcomeConfirmed Outcome = "confirmed"
	OutcomeInvalid   Outcome = "invalid"
)

// SenderResolver returns the authenticated user's verified Nimiq addresses.
// Payment confirmation requires the RPC sender to match one of these addresses.
type SenderResolver interface {
	VerifiedAddresses(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// Transfer is the shared on-chain check used by EventPurchase and PaymentRequest.
// Sender binding uses PayerUserID's verified identities, never a request creator.
type Transfer struct {
	TransactionHash string
	Recipient       string
	AmountLunas     int64
	PayerUserID     uuid.UUID
}

// TransferVerifier checks one exact basic NIM transfer against Nimiq RPC.
type TransferVerifier interface {
	VerifyTransfer(ctx context.Context, transfer Transfer) (Outcome, error)
}
