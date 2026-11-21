-- +goose Up
-- start_date_local is the activity's wall-clock start encoded as unix seconds
-- as if it were UTC (Strava's start_date_local, or start_date + the browser's
-- UTC offset for manual entries). Day/week/month buckets read it so a 23:30
-- run lands on the right local day. NULL until `cmd/resync -full` backfills
-- Strava rows; evaluation falls back to UTC start_date meanwhile.
ALTER TABLE activities ADD COLUMN start_date_local BIGINT;

ALTER TABLE goals ADD COLUMN recurrence TEXT NOT NULL DEFAULT 'none'
    CHECK (recurrence IN ('none','daily','weekly','monthly'));

-- +goose Down
ALTER TABLE goals DROP COLUMN recurrence;
ALTER TABLE activities DROP COLUMN start_date_local;
