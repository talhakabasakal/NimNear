package verification

import (
	"context"

	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
)

// Outcome is the server-side result of checking an on-chain transaction.
type Outcome string

const (
	OutcomeNotFound  Outcome = "not_found"
	OutcomeNotFinal  Outcome = "not_final"
	OutcomeConfirmed Outcome = "confirmed"
	OutcomeInvalid   Outcome = "invalid"
)

// Verifier is deliberately narrower than an RPC client so application tests never need a network.
type Verifier interface {
	Verify(ctx context.Context, purchase *model.Purchase) (Outcome, error)
}
