-- name: UpsertUserByStravaID :one
INSERT INTO users (id, strava_athlete_id, name, avatar_url, access_token, refresh_token, token_expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (strava_athlete_id) DO UPDATE SET
    name = excluded.name,
    avatar_url = excluded.avatar_url,
    access_token = excluded.access_token,
    refresh_token = excluded.refresh_token,
    token_expires_at = excluded.token_expires_at
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByStravaID :one
SELECT * FROM users WHERE strava_athlete_id = $1;

-- name: UpdateUserTokens :exec
UPDATE users SET access_token = $1, refresh_token = $2, token_expires_at = $3 WHERE id = $4;

-- name: ListUsersByIDs :many
SELECT * FROM users WHERE id = ANY(sqlc.arg(ids)::text[]);
