-- name: UpsertUserByStravaID :one
-- Deliberately never mentions the role column: on INSERT it takes the
-- column's own default, and on CONFLICT it's simply absent from the SET
-- list, so an existing user's role (in particular a promoted admin's) is
-- never touched by an ordinary login. See UpsertUserByStravaIDAsAdmin for
-- the only path that ever grants admin.
INSERT INTO users (id, strava_athlete_id, name, avatar_url, access_token, refresh_token, token_expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (strava_athlete_id) DO UPDATE SET
    name = excluded.name,
    avatar_url = excluded.avatar_url,
    access_token = excluded.access_token,
    refresh_token = excluded.refresh_token,
    token_expires_at = excluded.token_expires_at
RETURNING *;

-- name: UpsertUserByStravaIDAsAdmin :one
-- Used instead of UpsertUserByStravaID only when the caller has already
-- checked the Strava athlete id against ADMIN_STRAVA_ATHLETE_IDS. Explicitly
-- grants admin on both INSERT and CONFLICT, since being in the env var list
-- should (re-)grant admin on every login. There is no path that ever
-- explicitly demotes role back to member on conflict, so removing an id
-- from the env var stops future re-grants but never demotes a standing
-- admin.
INSERT INTO users (id, strava_athlete_id, name, avatar_url, access_token, refresh_token, token_expires_at, created_at, role, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'admin', 'active')
ON CONFLICT (strava_athlete_id) DO UPDATE SET
    name = excluded.name,
    avatar_url = excluded.avatar_url,
    access_token = excluded.access_token,
    refresh_token = excluded.refresh_token,
    token_expires_at = excluded.token_expires_at,
    role = 'admin',
    -- Env admins skip the approval queue, but a ban sticks.
    status = CASE WHEN users.status = 'pending' THEN 'active' ELSE users.status END
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByStravaID :one
SELECT * FROM users WHERE strava_athlete_id = $1;

-- name: UpdateUserTokens :exec
UPDATE users SET access_token = $1, refresh_token = $2, token_expires_at = $3 WHERE id = $4;

-- name: ListUsersByIDs :many
SELECT * FROM users WHERE id = ANY(sqlc.arg(ids)::text[]);

-- name: ListAllUsers :many
SELECT * FROM users ORDER BY created_at ASC;

-- name: ListUsersByStatus :many
SELECT * FROM users WHERE status = $1 ORDER BY created_at ASC;

-- name: SetUserStatus :exec
UPDATE users SET status = $2 WHERE id = $1;

-- name: BanUser :exec
UPDATE users SET status = 'banned', ban_reason = $2, banned_by = $3, banned_at = $4 WHERE id = $1;

-- name: UnbanUser :exec
UPDATE users SET status = 'active', ban_reason = '', banned_by = NULL, banned_at = NULL WHERE id = $1;

-- name: SetUserRole :exec
UPDATE users SET role = $2 WHERE id = $1;

-- name: CountActiveAdmins :one
SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active';

-- name: SetUserFeedSettings :one
UPDATE users SET feed_show_pace = $2, feed_show_power = $3, feed_show_effort = $4, feed_show_lifts = $5
WHERE id = $1
RETURNING *;

-- name: SetUserWhatsNewSeen :one
UPDATE users SET whats_new_seen = $2 WHERE id = $1
RETURNING *;
