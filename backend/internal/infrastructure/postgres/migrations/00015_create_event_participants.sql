-- +goose Up
CREATE TABLE IF NOT EXISTS event_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT event_participants_event_user_unique UNIQUE (event_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_event_participants_event_id
    ON event_participants (event_id);

CREATE INDEX IF NOT EXISTS idx_event_participants_user_id
    ON event_participants (user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_event_participants_user_id;
DROP INDEX IF EXISTS idx_event_participants_event_id;
DROP TABLE IF EXISTS event_participants;
