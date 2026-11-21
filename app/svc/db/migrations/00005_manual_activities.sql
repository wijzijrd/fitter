-- +goose Up
-- Manual entries live in activities beside Strava ones. Their ids come from a
-- descending sequence, so they are negative and can never collide with a Strava id.
-- shortcut: negative ids mark manual rows; rename the column to id if a third source arrives.
ALTER TABLE activities ADD COLUMN source TEXT NOT NULL DEFAULT 'strava' CHECK (source IN ('strava','manual'));
CREATE SEQUENCE manual_activity_id START -1 INCREMENT -1 MINVALUE -9223372036854775808 MAXVALUE -1;
GRANT USAGE, SELECT ON SEQUENCE manual_activity_id TO fitter_app;

CREATE TABLE activity_lifts (
    id          BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES activities(strava_activity_id) ON DELETE CASCADE,
    exercise    TEXT NOT NULL,
    weight_kg   DOUBLE PRECISION NOT NULL CHECK (weight_kg > 0),
    reps        INT NOT NULL CHECK (reps > 0)
);
CREATE INDEX idx_activity_lifts_activity ON activity_lifts(activity_id);

-- Set only on max_weight_kg clauses: which exercise the clause tracks.
ALTER TABLE goal_clauses ADD COLUMN exercise TEXT;

-- +goose Down
ALTER TABLE goal_clauses DROP COLUMN exercise;
DROP TABLE activity_lifts;
DROP SEQUENCE manual_activity_id;
ALTER TABLE activities DROP COLUMN source;
