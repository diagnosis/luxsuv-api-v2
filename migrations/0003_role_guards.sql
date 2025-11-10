-- +goose Up
-- +goose StatementBegin
-- Guard on INSERT/UPDATE of users.role
CREATE OR REPLACE FUNCTION guard_assign_roles()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    -- app_user can only create rider/driver
    IF session_user = 'app_user' AND NEW.role NOT IN ('rider','driver') THEN
      RAISE EXCEPTION 'app cannot assign role %', NEW.role USING ERRCODE='42501';
END IF;

    -- privileged roles only by db_owner (or a member of it)
    IF NEW.role IN ('admin','super_admin')
       AND NOT pg_has_role(current_user, 'db_owner', 'MEMBER') THEN
      RAISE EXCEPTION 'only db_owner may assign privileged roles' USING ERRCODE='42501';
END IF;

RETURN NEW;
END IF;

  IF TG_OP = 'UPDATE' THEN
    -- app_user cannot elevate to admin/super_admin or demote super_admin
    IF session_user = 'app_user' AND (
         (NEW.role IN ('admin','super_admin') AND NEW.role <> OLD.role)
      OR (OLD.role = 'super_admin' AND NEW.role <> 'super_admin')
    ) THEN
      RAISE EXCEPTION 'app cannot change privileged roles' USING ERRCODE='42501';
END IF;

    -- privileged role changes require db_owner
    IF NEW.role IN ('admin','super_admin') AND NEW.role <> OLD.role
       AND NOT pg_has_role(current_user, 'db_owner', 'MEMBER') THEN
      RAISE EXCEPTION 'only db_owner may set privileged roles' USING ERRCODE='42501';
END IF;

    -- cannot demote the last super_admin
    IF OLD.role = 'super_admin' AND NEW.role <> 'super_admin' THEN
      IF NOT pg_has_role(current_user, 'db_owner', 'MEMBER') THEN
        RAISE EXCEPTION 'only db_owner may demote super_admin' USING ERRCODE='42501';
END IF;
      PERFORM 1 FROM users WHERE role = 'super_admin' AND id <> OLD.id;
      IF NOT FOUND THEN
        RAISE EXCEPTION 'cannot demote the last super_admin' USING ERRCODE='42501';
END IF;
END IF;

RETURN NEW;
END IF;

RETURN NEW;
END;
$$;

-- Recreate trigger to be safe
DROP TRIGGER IF EXISTS trg_users_guard_assign ON users;
CREATE TRIGGER trg_users_guard_assign
    BEFORE INSERT OR UPDATE ON users
                         FOR EACH ROW EXECUTE FUNCTION guard_assign_roles();

-- Guard on DELETE of super_admin
CREATE OR REPLACE FUNCTION guard_delete_super_admin()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
  IF OLD.role = 'super_admin' THEN
    IF NOT pg_has_role(current_user, 'db_owner', 'MEMBER') THEN
      RAISE EXCEPTION 'only db_owner may delete super_admin' USING ERRCODE='42501';
END IF;
    PERFORM 1 FROM users WHERE role = 'super_admin' AND id <> OLD.id;
    IF NOT FOUND THEN
      RAISE EXCEPTION 'cannot delete the last super_admin' USING ERRCODE='42501';
END IF;
END IF;
RETURN OLD;
END;
$$;

DROP TRIGGER IF EXISTS trg_users_guard_delete ON users;
CREATE TRIGGER trg_users_guard_delete
    BEFORE DELETE ON users
    FOR EACH ROW EXECUTE FUNCTION guard_delete_super_admin();

-- At most one super_admin
CREATE UNIQUE INDEX IF NOT EXISTS one_super_admin_only
    ON users(role) WHERE role = 'super_admin';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_users_guard_delete ON users;
DROP FUNCTION IF EXISTS guard_delete_super_admin();
DROP TRIGGER IF EXISTS trg_users_guard_assign ON users;
DROP FUNCTION IF EXISTS guard_assign_roles();
DROP INDEX IF EXISTS one_super_admin_only;
-- +goose StatementEnd