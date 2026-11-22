-- +goose Up
-- A member's verified email (replies to their developer feedback go there). '' = not verified.
-- Additive, so the previous release keeps working during a rollout.
ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT '';

-- The one code a member is waiting on; asking again replaces it. Times are unix seconds.
CREATE TABLE email_verifications (
    user_id    TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email      TEXT NOT NULL,
    code_hash  TEXT NOT NULL, -- hex HMAC-SHA256 of the code, keyed by the JWT secret
    attempts   INT NOT NULL DEFAULT 0,
    sent_at    BIGINT NOT NULL,
    expires_at BIGINT NOT NULL
);

-- +goose Down
DROP TABLE email_verifications;
ALTER TABLE users DROP COLUMN email;
