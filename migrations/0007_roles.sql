-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'db_owner') THEN
CREATE ROLE db_owner LOGIN PASSWORD 'dev_db_owner_pwd' SUPERUSER;
END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
    -- In dev you can give LOGIN; in prod you’ll manage this outside migrations
CREATE ROLE app_user LOGIN PASSWORD 'dev_app_user_pwd' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
END IF;
END$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
    REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM app_user;
DROP ROLE app_user;
END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'db_owner') THEN
DROP ROLE db_owner;
END IF;
END$$;
-- +goose StatementEnd