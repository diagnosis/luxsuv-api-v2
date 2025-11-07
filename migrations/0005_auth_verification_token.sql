-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS auth_verification_tokens (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose    TEXT        NOT NULL CHECK (purpose IN ('rider_confirm','driver_confirm')),
    token_hash TEXT        NOT NULL UNIQUE,  -- sha256 of raw token
    issued_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    user_agent TEXT,
    ip         INET,
    CHECK (expires_at > issued_at)
    );

-- Only one active (unused) token per user+purpose
CREATE UNIQUE INDEX IF NOT EXISTS uniq_avt_user_purpose_active
    ON auth_verification_tokens (user_id, purpose)
    WHERE used_at IS NULL;

-- Fast lookup by hash (also unique at column level, but keep for clarity/speed if desired)
CREATE INDEX IF NOT EXISTS idx_avt_token_hash ON auth_verification_tokens (token_hash);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uniq_avt_user_purpose_active;
DROP INDEX IF EXISTS idx_avt_token_hash;
DROP TABLE IF EXISTS auth_verification_tokens;
-- +goose StatementEnd