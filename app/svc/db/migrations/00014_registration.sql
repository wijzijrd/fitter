-- +goose Up
-- Who may create an account on sign-in. 'open' (default): an unknown athlete gets a
-- pending account for approval. 'invite_only': an unknown athlete is refused unless
-- on the allowlist. Single row, so the mode is one value.
CREATE TABLE registration_settings (
    id   BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    mode TEXT NOT NULL DEFAULT 'open' CHECK (mode IN ('open','invite_only'))
);
INSERT INTO registration_settings (id, mode) VALUES (TRUE, 'open');

-- Strava athlete ids allowed to sign up in invite_only mode. Allowed athletes are
-- approved on first sign-in, so the list itself is the approval.
CREATE TABLE registration_allowlist (
    strava_athlete_id BIGINT PRIMARY KEY,
    added_by          TEXT NOT NULL REFERENCES users(id),
    added_at          BIGINT NOT NULL
);

-- +goose Down
DROP TABLE registration_allowlist;
DROP TABLE registration_settings;
