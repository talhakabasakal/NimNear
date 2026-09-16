-- +goose Up
ALTER TABLE event_purchases
    ADD COLUMN reconciliation_deadline_at TIMESTAMPTZ,
    ADD COLUMN last_verification_attempt_at TIMESTAMPTZ,
    ADD COLUMN verification_claimed_until TIMESTAMPTZ;

CREATE INDEX idx_event_purchases_reconciliation
    ON event_purchases(status, last_verification_attempt_at, verification_claimed_until, reconciliation_deadline_at, id)
    WHERE status IN ('submitted', 'verifying');

-- +goose Down
DROP INDEX IF EXISTS idx_event_purchases_reconciliation;

ALTER TABLE event_purchases
    DROP COLUMN IF EXISTS verification_claimed_until,
    DROP COLUMN IF EXISTS last_verification_attempt_at,
    DROP COLUMN IF EXISTS reconciliation_deadline_at;
