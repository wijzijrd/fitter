-- +goose Up
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','active','banned'));
UPDATE users SET status = 'active'; -- grandfather existing users
ALTER TABLE users ADD COLUMN ban_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN banned_by TEXT REFERENCES users(id);
ALTER TABLE users ADD COLUMN banned_at BIGINT;

CREATE TABLE admin_actions (
    id         TEXT PRIMARY KEY,
    actor_id   TEXT NOT NULL REFERENCES users(id),
    action     TEXT NOT NULL, -- approve|ban|unban|promote|demote|create_sport|delete_sport|create_metric|delete_metric
    target_id  TEXT NOT NULL, -- user id or custom sport/metric id
    detail     TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL
);
CREATE INDEX admin_actions_created_at ON admin_actions (created_at DESC);

-- +goose Down
DROP TABLE admin_actions;
ALTER TABLE users DROP COLUMN banned_at;
ALTER TABLE users DROP COLUMN banned_by;
ALTER TABLE users DROP COLUMN ban_reason;
ALTER TABLE users DROP COLUMN status;
