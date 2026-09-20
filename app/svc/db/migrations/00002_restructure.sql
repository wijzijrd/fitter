-- +goose Up
-- Rebuild activities/campaigns/campaign_members/goals and add goal_clauses.
-- users is untouched: existing JWTs and Strava refresh tokens survive, so
-- re-sync after this migration is a background job, not a forced re-login.
--
-- This is a destructive rebuild, not a column-preserving ALTER, because:
--   - SQLite can't drop a column or relax a CHECK without a full table
--     rebuild anyway, so a "careful" migration would be *more* SQL than a
--     destructive one for the same end state.
--   - The new Strava columns (average_heartrate, average_watts, etc.) have
--     no data to backfill from existing rows: an ALTER TABLE ADD COLUMN
--     would leave every pre-migration row's average_heartrate NULL, which
--     is indistinguishable from "no HR monitor" — a silent, permanent
--     data-quality lie.
--   - campaigns/goals change shape too fundamentally (clause-based goals
--     replace flat metric/target_value) for a translating migration to be
--     worth writing against dev-only data.
DROP TABLE goals;
DROP TABLE campaign_members;
DROP TABLE campaigns;
DROP TABLE activities;

-- strava_activity_id is now the primary key (the old `id TEXT` UUID column
-- is gone). This permanently fixes the pre-existing bug where
-- upsertStravaActivity minted a fresh UUID on every upsert despite matching
-- rows on strava_activity_id — there's no separate id left to churn. Cost:
-- anything referencing activities.id on the wire must change to
-- int64 strava_activity_id (Task 6's feed.proto change).
CREATE TABLE activities (
    strava_activity_id INTEGER PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT NOT NULL,
    sport_type TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL DEFAULT '',
    start_date INTEGER NOT NULL,

    distance_m       REAL    NOT NULL DEFAULT 0,
    elevation_m      REAL    NOT NULL DEFAULT 0,
    moving_time_s    INTEGER NOT NULL DEFAULT 0,
    elapsed_time_s   INTEGER NOT NULL DEFAULT 0,
    average_speed_ms REAL    NOT NULL DEFAULT 0,
    max_speed_ms     REAL    NOT NULL DEFAULT 0,

    average_heartrate      REAL,
    max_heartrate          REAL,
    average_watts          REAL,
    weighted_average_watts REAL,
    max_watts              REAL,
    device_watts           INTEGER NOT NULL DEFAULT 0,
    kilojoules             REAL,
    calories               REAL,
    average_cadence        REAL,
    suffer_score           REAL,

    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX idx_activities_user_start_date ON activities(user_id, start_date);
CREATE INDEX idx_activities_user_sport_start ON activities(user_id, sport_type, start_date);

CREATE TABLE campaigns (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    metric       TEXT,   -- NULL together with target_value: no pooled target
    target_value REAL,
    start_date  INTEGER NOT NULL,
    end_date    INTEGER NOT NULL,
    created_by  TEXT NOT NULL REFERENCES users(id),
    created_at  INTEGER NOT NULL,
    CHECK ((metric IS NULL) = (target_value IS NULL))
);

CREATE TABLE campaign_members (
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at   INTEGER NOT NULL,
    PRIMARY KEY (campaign_id, user_id)
);
CREATE INDEX idx_campaign_members_user ON campaign_members(user_id);

-- goals.campaign_id cascades on delete; leaving a campaign (as opposed to
-- the campaign being deleted) is handled explicitly in LeaveCampaign
-- (Task 7) since SQLite won't cascade through campaign_members.
CREATE TABLE goals (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    campaign_id  TEXT REFERENCES campaigns(id) ON DELETE CASCADE, -- NULL = personal goal
    name         TEXT NOT NULL DEFAULT '',
    sport_group  TEXT, -- curated group key, e.g. 'run'; NULL if sport_type is set or selector is "all"
    sport_type   TEXT, -- exact Strava sport_type, e.g. 'TrailRun'; NULL if sport_group is set or selector is "all"
    period_start INTEGER NOT NULL,
    period_end   INTEGER NOT NULL,
    created_at   INTEGER NOT NULL,
    CHECK (sport_group IS NULL OR sport_type IS NULL)
);
CREATE INDEX idx_goals_user ON goals(user_id);
CREATE INDEX idx_goals_campaign ON goals(campaign_id);
CREATE INDEX idx_goals_campaign_user ON goals(campaign_id, user_id);

-- No CHECK on goal_clauses.metric. Seventeen values in a CHECK means every
-- future eighteenth metric needs a full SQLite table rebuild.
-- internal/metrics is the single source of truth and validates at the
-- service boundary (Task 7) instead. scope and comparator keep CHECKs
-- because those two-value sets are not expected to grow.
CREATE TABLE goal_clauses (
    id         TEXT PRIMARY KEY,
    goal_id    TEXT NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
    position   INTEGER NOT NULL,
    scope      TEXT NOT NULL CHECK (scope IN ('aggregate','single')),
    metric     TEXT NOT NULL,
    comparator TEXT NOT NULL CHECK (comparator IN ('gte','lte')),
    value      REAL NOT NULL
);
CREATE UNIQUE INDEX idx_goal_clauses_goal_position ON goal_clauses(goal_id, position);

-- +goose Down
-- Non-reversible: this migration drops pre-restructure tables outright.
-- Recreating the 00001 shape would require re-deriving it from that file by hand.
DROP TABLE goal_clauses;
DROP TABLE goals;
DROP TABLE campaign_members;
DROP TABLE campaigns;
DROP TABLE activities;
