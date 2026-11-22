-- +goose Up
-- Exercise library and block logging. Additive only (expand/contract): activity_lifts stays,
-- and the service keeps dual-writing it until a later release drops it, so containers from
-- the previous release keep working while old and new overlap.

-- key is the normalised name (lower-case, single spaces), the same folding the service
-- applies to goal_clauses.exercise and activity_lifts.exercise, so all three join on it.
CREATE TABLE exercises (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    key        TEXT NOT NULL UNIQUE,
    measure    TEXT NOT NULL CHECK (measure IN ('weight_reps','reps','distance','time')),
    -- A weight is optional on distance and time exercises (sled push, weighted carry hold).
    loaded     BOOLEAN NOT NULL DEFAULT FALSE CHECK (NOT loaded OR measure IN ('distance','time')),
    -- Muscle keys mirror the Muscle enum in proto/fitter/v1/exercise.proto. Empty only for
    -- names backfilled from before the library existed; the service requires one on create.
    muscles    TEXT[] NOT NULL DEFAULT '{}' CHECK (muscles <@ ARRAY['chest','back','shoulders','biceps','triceps','forearms','core','glutes','quads','hamstrings','calves','full_body','cardio']::TEXT[]),
    created_by TEXT REFERENCES users(id) ON DELETE SET NULL,  -- NULL = seeded
    created_at BIGINT NOT NULL,
    -- NULL = offered when logging and in goal suggestions. Set = retired: hidden from both, but
    -- past sessions and goals keep it. Used exercises are archived rather than deleted.
    archived_at BIGINT
);

-- A manual session is an ordered list of blocks; a block repeats its stations for `rounds`.
CREATE TABLE activity_blocks (
    id          BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES activities(strava_activity_id) ON DELETE CASCADE,
    position    INT NOT NULL,
    rounds      INT NOT NULL CHECK (rounds >= 1)
);
CREATE INDEX idx_activity_blocks_activity ON activity_blocks(activity_id);

-- Which result columns are set follows exercises.measure; the service enforces that.
CREATE TABLE activity_stations (
    id          BIGSERIAL PRIMARY KEY,
    block_id    BIGINT NOT NULL REFERENCES activity_blocks(id) ON DELETE CASCADE,
    position    INT NOT NULL,
    exercise_id TEXT NOT NULL REFERENCES exercises(id),
    weight_kg   DOUBLE PRECISION CHECK (weight_kg > 0),
    reps        INT CHECK (reps > 0),
    distance_m  DOUBLE PRECISION CHECK (distance_m > 0),
    duration_s  BIGINT CHECK (duration_s > 0)
);
CREATE INDEX idx_activity_stations_block ON activity_stations(block_id);
CREATE INDEX idx_activity_stations_exercise ON activity_stations(exercise_id);

-- Default privileges cover tables, not the sequences behind BIGSERIAL, so grant them here.
-- activity_lifts_id_seq (00005) never was; the dual-write below inserts into it.
GRANT USAGE, SELECT ON SEQUENCE activity_blocks_id_seq, activity_stations_id_seq, activity_lifts_id_seq TO fitter_app;

-- +goose Down
DROP TABLE activity_stations;
DROP TABLE activity_blocks;
DROP TABLE exercises;
