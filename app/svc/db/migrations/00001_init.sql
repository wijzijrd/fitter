-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    strava_athlete_id INTEGER NOT NULL UNIQUE,
    name TEXT NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE activities (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    strava_activity_id INTEGER NOT NULL UNIQUE,
    type TEXT NOT NULL,
    distance_m REAL NOT NULL DEFAULT 0,
    elevation_m REAL NOT NULL DEFAULT 0,
    moving_time_s INTEGER NOT NULL DEFAULT 0,
    start_date INTEGER NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE INDEX idx_activities_user_start_date ON activities(user_id, start_date);

CREATE TABLE campaigns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL CHECK (mode IN ('pooled', 'per_member')),
    metric TEXT NOT NULL CHECK (metric IN ('distance', 'elevation', 'time', 'count')),
    target_value REAL NOT NULL,
    start_date INTEGER NOT NULL,
    end_date INTEGER NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id),
    created_at INTEGER NOT NULL
);

CREATE TABLE campaign_members (
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at INTEGER NOT NULL,
    per_member_target_value REAL,
    PRIMARY KEY (campaign_id, user_id)
);

CREATE INDEX idx_campaign_members_user ON campaign_members(user_id);

CREATE TABLE goals (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    metric TEXT NOT NULL CHECK (metric IN ('distance', 'elevation', 'time', 'count')),
    target_value REAL NOT NULL,
    period_start INTEGER NOT NULL,
    period_end INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX idx_goals_user ON goals(user_id);

-- +goose Down
DROP TABLE goals;
DROP TABLE campaign_members;
DROP TABLE campaigns;
DROP TABLE activities;
DROP TABLE users;
