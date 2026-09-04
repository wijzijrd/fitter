-- name: UpsertUserByStravaID :one
INSERT INTO users (id, strava_athlete_id, name, avatar_url, access_token, refresh_token, token_expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (strava_athlete_id) DO UPDATE SET
    name = excluded.name,
    avatar_url = excluded.avatar_url,
    access_token = excluded.access_token,
    refresh_token = excluded.refresh_token,
    token_expires_at = excluded.token_expires_at
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByStravaID :one
SELECT * FROM users WHERE strava_athlete_id = ?;

-- name: UpdateUserTokens :exec
UPDATE users SET access_token = ?, refresh_token = ?, token_expires_at = ? WHERE id = ?;

-- name: ListUsersByIDs :many
SELECT * FROM users WHERE id IN (sqlc.slice('ids'));
