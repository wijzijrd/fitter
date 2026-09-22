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

-- Task 4: custom sports & metrics. These let an admin extend the taxonomy
-- goal_clauses.metric and goals.sport_group/sport_type already draw from,
-- without touching internal/metrics or internal/sports — both tables use
-- the same plain-string-key mechanism those packages already rely on.
CREATE TABLE custom_sports (
    id         TEXT PRIMARY KEY,
    key        TEXT NOT NULL UNIQUE,  -- validated: must not collide with any real Strava sport_type
    label      TEXT NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at INTEGER NOT NULL
);

CREATE TABLE custom_metrics (
    id                    TEXT PRIMARY KEY,
    key                   TEXT NOT NULL UNIQUE,  -- same string-key mechanism goal_clauses.metric already uses for built-ins
    label                 TEXT NOT NULL,
    unit                  TEXT NOT NULL,          -- free-text display unit, e.g. "kg", "reps" — custom metrics aren't constrained to the built-in MetricUnit enum
    aggregation           TEXT NOT NULL CHECK (aggregation IN ('sum','average','max','count')),
    nullable              INTEGER NOT NULL DEFAULT 1,  -- custom metrics are always optionally-present by nature
    lower_is_better       INTEGER NOT NULL DEFAULT 0,
    supports_single_scope INTEGER NOT NULL DEFAULT 1,
    supports_aggregate_scope INTEGER NOT NULL DEFAULT 1,
    created_by            TEXT NOT NULL REFERENCES users(id),
    created_at            INTEGER NOT NULL
);

-- sport_ref is a plain text key: either a built-in sports.Group value (e.g. "strength")
-- or a custom_sports.key — no FK, validated at the service boundary, mirroring how
-- goal_clauses.metric already has no CHECK by deliberate design (Task 4 of the prior plan).
CREATE TABLE custom_metric_sports (
    metric_id TEXT NOT NULL REFERENCES custom_metrics(id) ON DELETE CASCADE,
    sport_ref TEXT NOT NULL,
    PRIMARY KEY (metric_id, sport_ref)
);

-- +goose Down
DROP TABLE custom_metric_sports;
DROP TABLE custom_metrics;
DROP TABLE custom_sports;
ALTER TABLE users DROP COLUMN role;
