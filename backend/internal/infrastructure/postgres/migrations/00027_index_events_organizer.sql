-- +goose Up
-- ListPublicByOrganizer, organizer updates/cancels, and profile hosted-event
-- counts all filter events by organizer_id.
CREATE INDEX IF NOT EXISTS idx_events_organizer_id ON events (organizer_id);

-- +goose Down
DROP INDEX IF EXISTS idx_events_organizer_id;
