-- +goose Up
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL CHECK (ends_at > starts_at),
    status VARCHAR(32) NOT NULL DEFAULT 'published'
        CHECK (status IN ('draft', 'published', 'cancelled')),
    price_nim NUMERIC(20, 8) NOT NULL DEFAULT 0 CHECK (price_nim >= 0),
    currency VARCHAR(16) NOT NULL DEFAULT 'NIM',
    capacity INTEGER CHECK (capacity IS NULL OR capacity > 0),
    attendee_count INTEGER NOT NULL DEFAULT 0 CHECK (attendee_count >= 0),
    image_url TEXT NOT NULL DEFAULT '',
    place_id UUID REFERENCES places(id) ON DELETE SET NULL,
    latitude DOUBLE PRECISION CHECK (latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION CHECK (longitude BETWEEN -180 AND 180),
    address TEXT,
    city VARCHAR(120) NOT NULL,
    organizer_id UUID REFERENCES users(id) ON DELETE SET NULL,
    is_public BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT events_coordinates_pair CHECK ((latitude IS NULL) = (longitude IS NULL)),
    CONSTRAINT events_capacity_attendees CHECK (capacity IS NULL OR attendee_count <= capacity)
);

CREATE INDEX IF NOT EXISTS idx_events_starts_at ON events (starts_at);
CREATE INDEX IF NOT EXISTS idx_events_city ON events (city);
CREATE INDEX IF NOT EXISTS idx_events_place_id ON events (place_id);
CREATE INDEX IF NOT EXISTS idx_events_public_upcoming
    ON events (starts_at, city)
    WHERE is_public = TRUE AND status = 'published';

-- +goose Down
DROP INDEX IF EXISTS idx_events_public_upcoming;
DROP INDEX IF EXISTS idx_events_place_id;
DROP INDEX IF EXISTS idx_events_city;
DROP INDEX IF EXISTS idx_events_starts_at;
DROP TABLE IF EXISTS events;
