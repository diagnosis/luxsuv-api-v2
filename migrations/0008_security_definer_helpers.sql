-- +goose Up
-- +goose StatementBegin

-- Create a user with any role (rider/driver/admin/super_admin)
CREATE OR REPLACE FUNCTION create_user_with_role(p_email text, p_hash text, p_role user_role)
RETURNS uuid AS $$
DECLARE v_id uuid;
BEGIN
INSERT INTO users (email, password_hash, role, is_verified, is_active, created_at, updated_at)
VALUES (lower(btrim(p_email)), p_hash, p_role, false, false, now(), now())
    RETURNING id INTO v_id;
RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Elevate/demote role safely (respects “last super_admin” rule)
CREATE OR REPLACE FUNCTION set_user_role(p_user_id uuid, p_new_role user_role)
RETURNS void AS $$
DECLARE old_role user_role;
DECLARE others integer;
BEGIN
SELECT role INTO old_role FROM users WHERE id = p_user_id FOR UPDATE;

IF NOT FOUND THEN
    RAISE EXCEPTION 'user not found';
END IF;

  -- prevent deleting the last super_admin by demotion
  IF old_role = 'super_admin' AND p_new_role <> 'super_admin' THEN
SELECT COUNT(*) INTO others
FROM users
WHERE role = 'super_admin' AND id <> p_user_id;
IF others = 0 THEN
      RAISE EXCEPTION 'cannot demote the last super_admin';
END IF;
END IF;

UPDATE users
SET role = p_new_role, updated_at = now()
WHERE id = p_user_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Convenience toggles (optional)
CREATE OR REPLACE FUNCTION set_user_flags(p_user_id uuid, p_verified boolean, p_active boolean)
RETURNS void AS $$
BEGIN
UPDATE users
SET is_verified = COALESCE(p_verified, is_verified),
    is_active   = COALESCE(p_active,   is_active),
    updated_at  = now()
WHERE id = p_user_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Lock down: only app_user may call (via app), not PUBLIC
REVOKE ALL ON FUNCTION create_user_with_role(text,text,user_role) FROM PUBLIC;
REVOKE ALL ON FUNCTION set_user_role(uuid,user_role)            FROM PUBLIC;
REVOKE ALL ON FUNCTION set_user_flags(uuid,boolean,boolean)     FROM PUBLIC;

GRANT EXECUTE ON FUNCTION create_user_with_role(text,text,user_role) TO app_user;
GRANT EXECUTE ON FUNCTION set_user_role(uuid,user_role)            TO app_user;
GRANT EXECUTE ON FUNCTION set_user_flags(uuid,boolean,boolean)     TO app_user;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
REVOKE EXECUTE ON FUNCTION create_user_with_role(text,text,user_role) FROM app_user;
REVOKE EXECUTE ON FUNCTION set_user_role(uuid,user_role)            FROM app_user;
REVOKE EXECUTE ON FUNCTION set_user_flags(uuid,boolean,boolean)     FROM app_user;

DROP FUNCTION IF EXISTS set_user_flags(uuid,boolean,boolean);
DROP FUNCTION IF EXISTS set_user_role(uuid,user_role);
DROP FUNCTION IF EXISTS create_user_with_role(text,text,user_role);
-- +goose StatementEnd