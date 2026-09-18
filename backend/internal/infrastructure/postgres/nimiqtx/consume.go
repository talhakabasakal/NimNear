package nimiqtx

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	DomainEventPurchase  = "event_purchase"
	DomainPaymentRequest = "payment_request"
)

// Consume records that a Nimiq transaction hash now belongs to one business object.
// Callers must invoke this inside the same database transaction as the domain update.
func Consume(ctx context.Context, tx pgx.Tx, hash, domainType string, domainID uuid.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `
        INSERT INTO consumed_nimiq_transactions (transaction_hash, domain_type, domain_id, created_at)
        VALUES ($1, $2, $3, $4)`, hash, domainType, domainID, now)
	return err
}
