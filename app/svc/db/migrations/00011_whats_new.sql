-- +goose Up
-- The release whose what's-new splash a member last dismissed (the UI names releases). Empty =
-- never seen one. Additive, so the previous release keeps working during a rollout.
ALTER TABLE users ADD COLUMN whats_new_seen TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN whats_new_seen;
