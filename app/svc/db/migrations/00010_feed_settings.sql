-- +goose Up
-- What each member sees in the feed (a viewer preference: it doesn't change what others see of
-- them), and when each member last looked at each campaign's feed. Additive, so the previous
-- release keeps working during a rollout.
ALTER TABLE users
    ADD COLUMN feed_show_pace   BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN feed_show_power  BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN feed_show_effort BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN feed_show_lifts  BOOLEAN NOT NULL DEFAULT TRUE;

-- NULL = never looked, so a first visit has nothing to call "new".
ALTER TABLE campaign_members ADD COLUMN feed_seen_at BIGINT;

-- +goose Down
ALTER TABLE campaign_members DROP COLUMN feed_seen_at;
ALTER TABLE users
    DROP COLUMN feed_show_lifts,
    DROP COLUMN feed_show_effort,
    DROP COLUMN feed_show_power,
    DROP COLUMN feed_show_pace;
