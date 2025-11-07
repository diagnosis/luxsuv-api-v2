-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Roles as a proper enum (keeps data consistent)
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
CREATE TYPE user_role AS ENUM ('rider','driver','admin','super_admin');
END IF;
END$$;

CREATE TABLE users (
                       id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                       email         VARCHAR(255)     NOT NULL,
                       password_hash VARCHAR(255)     NOT NULL,
                       is_verified   BOOLEAN          NOT NULL DEFAULT false,
                       is_active     BOOLEAN          NOT NULL DEFAULT false,
                       role          user_role        NOT NULL DEFAULT 'rider',
                       created_at    TIMESTAMPTZ      NOT NULL DEFAULT now(),
                       updated_at    TIMESTAMPTZ      NOT NULL DEFAULT now()
);

-- Case-insensitive + trimmed uniqueness for email
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_unique
    ON users (lower(btrim(email)));

-- Quick lookup by email
CREATE INDEX IF NOT EXISTS idx_users_email_lower
    ON users (lower(btrim(email)));

-- Admin and verification filters
CREATE INDEX IF NOT EXISTS idx_users_role_verified_active
    ON users (role, is_verified, is_active);

CREATE INDEX IF NOT EXISTS idx_users_verified_role
    ON users (is_verified, role);

-- Trigger to auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $fn$
BEGIN
  NEW.updated_at := now();
RETURN NEW;
END;
$fn$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS users;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
DROP TYPE user_role;
END IF;
END$$;
-- +goose StatementEnd