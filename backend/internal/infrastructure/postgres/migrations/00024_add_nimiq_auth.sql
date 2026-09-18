-- +goose Up
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

CREATE TABLE auth_challenges (
    id UUID PRIMARY KEY,
    nonce BYTEA NOT NULL UNIQUE,
    purpose TEXT NOT NULL CHECK (purpose = 'AUTH_LOGIN'),
    transport TEXT NOT NULL CHECK (transport IN ('mini-app', 'hub')),
    environment TEXT NOT NULL,
    network TEXT NOT NULL,
    domain TEXT NOT NULL,
    audience TEXT NOT NULL,
    claimed_address TEXT NOT NULL,
    message TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_auth_challenges_expires_at ON auth_challenges(expires_at);
CREATE INDEX idx_auth_challenges_address ON auth_challenges(network, claimed_address);

CREATE TABLE user_nimiq_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    network TEXT NOT NULL,
    address TEXT NOT NULL,
    public_key BYTEA NOT NULL,
    verified_at TIMESTAMPTZ NOT NULL,
    last_verified_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (network, address),
    UNIQUE (network, public_key)
);
CREATE INDEX idx_user_nimiq_identities_user ON user_nimiq_identities(user_id);

-- +goose Down
DROP TABLE IF EXISTS user_nimiq_identities;
DROP TABLE IF EXISTS auth_challenges;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
