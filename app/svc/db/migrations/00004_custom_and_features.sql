-- +goose Up
-- Admin role: the foundation for the admin-gated custom sports and metrics below.
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('member','admin'));

-- Custom sports & metrics let an admin extend the taxonomy goal_clauses.metric and
-- goals.sport_group/sport_type draw from, using the same plain-string keys as
-- internal/metrics and internal/sports.
CREATE TABLE custom_sports (
    id         TEXT PRIMARY KEY,
    key        TEXT NOT NULL UNIQUE,  -- validated: must not collide with any real Strava sport_type
    label      TEXT NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at BIGINT NOT NULL
);

CREATE TABLE custom_metrics (
    id                       TEXT PRIMARY KEY,
    key                      TEXT NOT NULL UNIQUE,
    label                    TEXT NOT NULL,
    unit                     TEXT NOT NULL,  -- free-text display unit, e.g. "kg", "reps"
    aggregation              TEXT NOT NULL CHECK (aggregation IN ('sum','average','max','count')),
    nullable                 BOOLEAN NOT NULL DEFAULT TRUE,
    lower_is_better          BOOLEAN NOT NULL DEFAULT FALSE,
    supports_single_scope    BOOLEAN NOT NULL DEFAULT TRUE,
    supports_aggregate_scope BOOLEAN NOT NULL DEFAULT TRUE,
    created_by               TEXT NOT NULL REFERENCES users(id),
    created_at               BIGINT NOT NULL
);

-- sport_ref is a built-in sports.Group value or a custom_sports.key; no FK,
-- validated at the service boundary.
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
