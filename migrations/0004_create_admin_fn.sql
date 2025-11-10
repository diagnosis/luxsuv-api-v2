-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION create_admin(p_email text, p_hash text)
RETURNS uuid AS $$
DECLARE v_id uuid;
BEGIN
INSERT INTO users (email, password_hash, is_active, role)
VALUES (lower(btrim(p_email)), p_hash, true, 'admin')
    RETURNING id INTO v_id;
RETURN v_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- lock it down
REVOKE ALL ON FUNCTION create_admin(text,text) FROM PUBLIC;
-- Will grant to app_user after you create roles (next step)
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS create_admin(text,text);
-- +goose StatementEnd