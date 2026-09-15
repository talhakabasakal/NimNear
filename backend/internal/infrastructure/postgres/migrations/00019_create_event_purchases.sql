-- +goose Up
CREATE TABLE IF NOT EXISTS event_purchases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_lunas BIGINT NOT NULL CHECK (amount_lunas > 0),
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'confirmed', 'failed', 'expired', 'cancelled')),
    capacity_hold_expires_at TIMESTAMPTZ,
    transaction_hash TEXT,
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_event_purchases_active_user_event
    ON event_purchases(event_id, user_id)
    WHERE status IN ('pending', 'confirmed');

CREATE UNIQUE INDEX IF NOT EXISTS idx_event_purchases_transaction_hash
    ON event_purchases(transaction_hash)
    WHERE transaction_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_event_purchases_event_capacity
    ON event_purchases(event_id, status, capacity_hold_expires_at);

-- +goose Down
DROP TABLE IF EXISTS event_purchases;
