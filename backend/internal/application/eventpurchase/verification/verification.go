package verification

import (
	"context"

	"github.com/masterfabric-go/masterfabric/internal/application/nimiqtx/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
)

// Outcome is the server-side result of checking an on-chain transaction.
type Outcome = verification.Outcome

const (
	OutcomeNotFound  = verification.OutcomeNotFound
	OutcomeNotFinal  = verification.OutcomeNotFinal
	OutcomeConfirmed = verification.OutcomeConfirmed
	OutcomeInvalid   = verification.OutcomeInvalid
)

// SenderResolver returns the authenticated user's verified Nimiq addresses.
type SenderResolver = verification.SenderResolver

// Transfer is the shared on-chain check used by EventPurchase and PaymentRequest.
type Transfer = verification.Transfer

// Verifier is deliberately narrower than an RPC client so application tests never need a network.
type Verifier interface {
	Verify(ctx context.Context, purchase *model.Purchase) (Outcome, error)
}
