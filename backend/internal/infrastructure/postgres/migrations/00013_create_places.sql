-- +goose Up
CREATE TABLE IF NOT EXISTS places (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    latitude DOUBLE PRECISION NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    address TEXT NOT NULL DEFAULT '',
    category VARCHAR(100) NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_places_active_coordinates
    ON places (is_active, latitude, longitude);

CREATE INDEX IF NOT EXISTS idx_places_active_category
    ON places (category)
    WHERE is_active = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_places_active_category;
DROP INDEX IF EXISTS idx_places_active_coordinates;
DROP TABLE IF EXISTS places;
