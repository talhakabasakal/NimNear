-- +goose Up
-- consumed_nimiq_transactions is the cross-domain uniqueness store for Nimiq
-- transaction hashes. Per-table unique indexes are not sufficient to stop the
-- same hash from satisfying both an event purchase and a payment request.
CREATE TABLE consumed_nimiq_transactions (
    transaction_hash TEXT PRIMARY KEY,
    domain_type TEXT NOT NULL
        CHECK (domain_type IN ('event_purchase', 'payment_request')),
    domain_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_consumed_nimiq_transactions_domain
    ON consumed_nimiq_transactions(domain_type, domain_id);

INSERT INTO consumed_nimiq_transactions (transaction_hash, domain_type, domain_id, created_at)
SELECT transaction_hash, 'event_purchase', id, COALESCE(confirmed_at, updated_at, created_at)
FROM event_purchases
WHERE transaction_hash IS NOT NULL
ON CONFLICT (transaction_hash) DO NOTHING;

-- Payment requests expire 24 hours after creation by default
-- (NIMNEAR_PAYMENT_REQUEST_TTL_HOURS). Public and creator reads treat
-- now >= expires_at as expired even if this row is still pending.
CREATE TABLE payment_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    creator_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_address TEXT NOT NULL,
    amount_lunas BIGINT NOT NULL CHECK (amount_lunas > 0),
    note TEXT,
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'submitted', 'verifying', 'paid', 'failed', 'expired', 'cancelled')),
    expires_at TIMESTAMPTZ NOT NULL,
    cancelled_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    payer_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    transaction_hash TEXT,
    reconciliation_deadline_at TIMESTAMPTZ,
    last_verification_attempt_at TIMESTAMPTZ,
    verification_claimed_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_payment_requests_transaction_hash
    ON payment_requests(transaction_hash)
    WHERE transaction_hash IS NOT NULL;

CREATE INDEX idx_payment_requests_creator_created
    ON payment_requests(creator_user_id, created_at DESC, id);

CREATE INDEX idx_payment_requests_pending_expiry
    ON payment_requests(expires_at)
    WHERE status = 'pending';

CREATE INDEX idx_payment_requests_reconciliation
    ON payment_requests(status, last_verification_attempt_at, verification_claimed_until, reconciliation_deadline_at, id)
    WHERE status IN ('submitted', 'verifying');

-- +goose Down
DROP TABLE IF EXISTS payment_requests;
DROP TABLE IF EXISTS consumed_nimiq_transactions;
