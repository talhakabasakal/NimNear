-- +goose Up
-- Canonical event prices are stored as integer Luna. One NIM is exactly 100,000 Luna.
-- The preflight check intentionally fails instead of rounding incompatible data.
-- +goose StatementBegin
DO $do$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'events' AND column_name = 'price_nim'
    ) AND EXISTS (
        SELECT 1 FROM events
        WHERE price_nim * 100000 <> trunc(price_nim * 100000)
    ) THEN
        RAISE EXCEPTION 'events contain price_nim values that cannot be represented exactly in Luna; remediate before migration';
    END IF;
END $do$;

ALTER TABLE events ADD COLUMN IF NOT EXISTS price_lunas BIGINT;

DO $do$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'events' AND column_name = 'price_nim'
    ) THEN
        UPDATE events SET price_lunas = (price_nim * 100000)::BIGINT
        WHERE price_lunas IS NULL;
    END IF;
END $do$;

ALTER TABLE events ALTER COLUMN price_lunas SET NOT NULL;

DO $do$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'events_price_lunas_nonnegative') THEN
        ALTER TABLE events ADD CONSTRAINT events_price_lunas_nonnegative CHECK (price_lunas >= 0);
    END IF;
END $do$;

ALTER TABLE events DROP COLUMN IF EXISTS price_nim;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE events ADD COLUMN price_nim NUMERIC(20,8) NOT NULL DEFAULT 0;
UPDATE events SET price_nim = price_lunas::numeric / 100000;
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_price_lunas_nonnegative;
ALTER TABLE events DROP COLUMN price_lunas;
-- +goose StatementEnd
