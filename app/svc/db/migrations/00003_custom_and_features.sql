-- +goose Up
-- Task 3: admin role. No role/permission concept has existed anywhere in
-- this codebase until now — this column is the foundation for all
-- admin-gated features later in this migration file (custom sports/metrics,
-- an admin panel) added by later tasks in this same release.
--
-- A plain ADD COLUMN (not a table rebuild) is sufficient here: unlike
-- 00002's Strava columns, every existing row has an unambiguous, correct
-- value for this column — 'member' — so there's no silent data-quality lie
-- to worry about. SQLite allows ADD COLUMN with a CHECK and a DEFAULT as
-- long as the default is a constant, which 'member' is.
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('member','admin'));

-- +goose Down
ALTER TABLE users DROP COLUMN role;
