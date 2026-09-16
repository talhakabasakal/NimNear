-- +goose Up
ALTER TABLE events
    ADD COLUMN IF NOT EXISTS calendar_id UUID REFERENCES calendars(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_events_calendar_id ON events (calendar_id);

-- +goose Down
DROP INDEX IF EXISTS idx_events_calendar_id;
ALTER TABLE events DROP COLUMN IF EXISTS calendar_id;
