-- name: UpsertActivity :one
INSERT INTO activities (id, user_id, strava_activity_id, type, distance_m, elevation_m, moving_time_s, start_date, name, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (strava_activity_id) DO UPDATE SET
    type = excluded.type,
    distance_m = excluded.distance_m,
    elevation_m = excluded.elevation_m,
    moving_time_s = excluded.moving_time_s,
    start_date = excluded.start_date,
    name = excluded.name
RETURNING *;

-- name: DeleteActivityByStravaID :exec
DELETE FROM activities WHERE strava_activity_id = ?;

-- name: ListActivitiesForUserInRange :many
SELECT * FROM activities
WHERE user_id = ? AND start_date >= ? AND start_date <= ?
ORDER BY start_date ASC;

-- name: ListActivitiesForUsersInRange :many
SELECT * FROM activities
WHERE user_id IN (sqlc.slice('user_ids')) AND start_date >= ? AND start_date <= ?
ORDER BY start_date ASC;

-- name: ListCampaignFeedActivities :many
SELECT a.*, u.name AS user_name, u.avatar_url AS user_avatar_url
FROM activities a
JOIN campaign_members cm ON cm.user_id = a.user_id
JOIN users u ON u.id = a.user_id
WHERE cm.campaign_id = ? AND a.start_date >= ? AND a.start_date <= ?
ORDER BY a.start_date DESC
LIMIT 100;
