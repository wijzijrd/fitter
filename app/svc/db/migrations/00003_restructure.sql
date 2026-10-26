-- +goose Up
-- Rebuild activities/campaigns/campaign_members/goals and add goal_clauses.
-- users is untouched: existing JWTs and Strava refresh tokens survive, so
-- re-sync after this migration (cmd/resync) is a background job, not a forced re-login.
--
-- This is a destructive rebuild, not a column-preserving ALTER: the new Strava
-- columns (average_heartrate, average_watts, ...) have nothing to backfill from,
-- and NULL would be indistinguishable from "no HR monitor"; campaigns and goals
-- change shape too fundamentally (clause-based goals replace flat
-- metric/target_value) for a translating migration over dev-only data.
DROP TABLE goals;
DROP TABLE campaign_members;
DROP TABLE campaigns;
DROP TABLE activities;

-- strava_activity_id is the primary key: there is no separate id to churn on upsert.
CREATE TABLE activities (
    strava_activity_id BIGINT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT NOT NULL,
    sport_type TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL DEFAULT '',
    start_date BIGINT NOT NULL,

    distance_m       DOUBLE PRECISION NOT NULL DEFAULT 0,
    elevation_m      DOUBLE PRECISION NOT NULL DEFAULT 0,
    moving_time_s    BIGINT NOT NULL DEFAULT 0,
    elapsed_time_s   BIGINT NOT NULL DEFAULT 0,
    average_speed_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_speed_ms     DOUBLE PRECISION NOT NULL DEFAULT 0,

    average_heartrate      DOUBLE PRECISION,
    max_heartrate          DOUBLE PRECISION,
    average_watts          DOUBLE PRECISION,
    weighted_average_watts DOUBLE PRECISION,
    max_watts              DOUBLE PRECISION,
    device_watts           BOOLEAN NOT NULL DEFAULT FALSE,
    kilojoules             DOUBLE PRECISION,
    calories               DOUBLE PRECISION,
    average_cadence        DOUBLE PRECISION,
    suffer_score           DOUBLE PRECISION,

    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);
CREATE INDEX idx_activities_user_start_date ON activities(user_id, start_date);
CREATE INDEX idx_activities_user_sport_start ON activities(user_id, sport_type, start_date);

CREATE TABLE campaigns (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    metric       TEXT,   -- NULL together with target_value: no pooled target
    target_value DOUBLE PRECISION,
    start_date   BIGINT NOT NULL,
    end_date     BIGINT NOT NULL,
    created_by   TEXT NOT NULL REFERENCES users(id),
    created_at   BIGINT NOT NULL,
    CHECK ((metric IS NULL) = (target_value IS NULL))
);

CREATE TABLE campaign_members (
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at   BIGINT NOT NULL,
    PRIMARY KEY (campaign_id, user_id)
);
CREATE INDEX idx_campaign_members_user ON campaign_members(user_id);

-- goals.campaign_id cascades on campaign delete; leaving a campaign deletes the
-- member's campaign goals explicitly in LeaveCampaign.
CREATE TABLE goals (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    campaign_id  TEXT REFERENCES campaigns(id) ON DELETE CASCADE, -- NULL = personal goal
    name         TEXT NOT NULL DEFAULT '',
    sport_group  TEXT, -- curated group key, e.g. 'run'; NULL if sport_type is set or selector is "all"
    sport_type   TEXT, -- exact Strava sport_type, e.g. 'TrailRun'; NULL if sport_group is set or selector is "all"
    period_start BIGINT NOT NULL,
    period_end   BIGINT NOT NULL,
    created_at   BIGINT NOT NULL,
    CHECK (sport_group IS NULL OR sport_type IS NULL)
);
CREATE INDEX idx_goals_user ON goals(user_id);
CREATE INDEX idx_goals_campaign ON goals(campaign_id);
CREATE INDEX idx_goals_campaign_user ON goals(campaign_id, user_id);

-- No CHECK on goal_clauses.metric: internal/metrics is the single source of truth
-- and validates at the service boundary, so a new metric needs no migration.
CREATE TABLE goal_clauses (
    id         TEXT PRIMARY KEY,
    goal_id    TEXT NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
    position   BIGINT NOT NULL,
    scope      TEXT NOT NULL CHECK (scope IN ('aggregate','single')),
    metric     TEXT NOT NULL,
    comparator TEXT NOT NULL CHECK (comparator IN ('gte','lte')),
    value      DOUBLE PRECISION NOT NULL
);
CREATE UNIQUE INDEX idx_goal_clauses_goal_position ON goal_clauses(goal_id, position);

-- +goose Down
-- Non-reversible: this migration drops pre-restructure tables outright.
DROP TABLE goal_clauses;
DROP TABLE goals;
DROP TABLE campaign_members;
DROP TABLE campaigns;
DROP TABLE activities;
