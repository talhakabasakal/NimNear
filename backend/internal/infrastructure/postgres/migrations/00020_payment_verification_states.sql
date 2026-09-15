-- +goose Up
ALTER TABLE event_purchases
    DROP CONSTRAINT IF EXISTS event_purchases_status_check;

ALTER TABLE event_purchases
    ADD CONSTRAINT event_purchases_status_check
    CHECK (status IN ('pending', 'submitted', 'verifying', 'confirmed', 'failed', 'expired', 'cancelled'));

DROP INDEX IF EXISTS idx_event_purchases_active_user_event;

CREATE UNIQUE INDEX idx_event_purchases_active_user_event
    ON event_purchases(event_id, user_id)
    WHERE status IN ('pending', 'submitted', 'verifying', 'confirmed');

-- +goose Down
DROP INDEX IF EXISTS idx_event_purchases_active_user_event;

CREATE UNIQUE INDEX idx_event_purchases_active_user_event
    ON event_purchases(event_id, user_id)
    WHERE status IN ('pending', 'confirmed');

ALTER TABLE event_purchases
    DROP CONSTRAINT IF EXISTS event_purchases_status_check;

ALTER TABLE event_purchases
    ADD CONSTRAINT event_purchases_status_check
    CHECK (status IN ('pending', 'confirmed', 'failed', 'expired', 'cancelled'));
