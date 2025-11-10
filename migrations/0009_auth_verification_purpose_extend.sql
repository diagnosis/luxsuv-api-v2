-- +goose Up
-- +goose StatementBegin
ALTER TABLE auth_verification_tokens
DROP CONSTRAINT IF EXISTS auth_verification_tokens_purpose_check;

ALTER TABLE auth_verification_tokens
    ADD CONSTRAINT auth_verification_tokens_purpose_check
        CHECK (purpose IN ('rider_confirm','driver_confirm','password_reset'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE auth_verification_tokens
DROP CONSTRAINT IF EXISTS auth_verification_tokens_purpose_check;

ALTER TABLE auth_verification_tokens
    ADD CONSTRAINT auth_verification_tokens_purpose_check
        CHECK (purpose IN ('rider_confirm','driver_confirm'));
-- +goose StatementEnd