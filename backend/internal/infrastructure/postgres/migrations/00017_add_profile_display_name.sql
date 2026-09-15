-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(100);

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS display_name;
