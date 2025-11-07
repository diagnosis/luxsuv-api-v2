-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'driver_app_status') THEN
CREATE TYPE driver_app_status AS ENUM ('pending','approved','rejected');
END IF;
END$$;

CREATE TABLE IF NOT EXISTS driver_applications (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    status      driver_app_status NOT NULL DEFAULT 'pending',
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
    );

-- Use your shared updater if it already exists; otherwise this defines it.
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $fn$
BEGIN
  NEW.updated_at := now();
RETURN NEW;
END;
$fn$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_driver_apps_updated_at ON driver_applications;
CREATE TRIGGER trg_driver_apps_updated_at
    BEFORE UPDATE ON driver_applications
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_driver_apps_updated_at ON driver_applications;
-- If this function is used by other tables (e.g., users), DO NOT drop it here.
-- DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS driver_applications;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'driver_app_status') THEN
DROP TYPE driver_app_status;
END IF;
END$$;
-- +goose StatementEnd