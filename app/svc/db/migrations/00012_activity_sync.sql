-- +goose Up
-- When each member's activities were last pulled from Strava on demand (unix seconds), so the
-- app can sync them on load without hammering Strava's shared rate limit. Additive.
ALTER TABLE users ADD COLUMN last_synced_at BIGINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE users DROP COLUMN last_synced_at;
