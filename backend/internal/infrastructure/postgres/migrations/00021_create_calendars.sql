-- +goose Up
CREATE TABLE IF NOT EXISTS calendars (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    visibility VARCHAR(16) NOT NULL DEFAULT 'public'
        CHECK (visibility IN ('public', 'private')),
    status VARCHAR(16) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_calendars_public_order
    ON calendars (created_at, id)
    WHERE visibility = 'public' AND status = 'active';

CREATE INDEX IF NOT EXISTS idx_calendars_owner_order
    ON calendars (owner_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS calendar_followers (
    calendar_id UUID NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (calendar_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_calendar_followers_user_order
    ON calendar_followers (user_id, created_at DESC, calendar_id);

-- +goose Down
DROP INDEX IF EXISTS idx_calendar_followers_user_order;
DROP TABLE IF EXISTS calendar_followers;
DROP INDEX IF EXISTS idx_calendars_owner_order;
DROP INDEX IF EXISTS idx_calendars_public_order;
DROP TABLE IF EXISTS calendars;
