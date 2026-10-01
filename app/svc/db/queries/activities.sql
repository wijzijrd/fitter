-- name: UpsertActivity :one
INSERT INTO activities (id, user_id, strava_activity_id, type, distance_m, elevation_m, moving_time_s, start_date, name, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (strava_activity_id) DO UPDATE SET
    type = excluded.type,
    distance_m = excluded.distance_m,
    elevation_m = excluded.elevation_m,
    moving_time_s = excluded.moving_time_s,
    start_date = excluded.start_date,
    name = excluded.name
RETURNING *;

-- name: DeleteActivityByStravaID :exec
DELETE FROM activities WHERE strava_activity_id = $1;

-- name: ListActivitiesForUserInRange :many
SELECT * FROM activities
WHERE user_id = $1 AND start_date >= $2 AND start_date <= $3
ORDER BY start_date ASC;

-- name: ListActivitiesForUsersInRange :many
SELECT * FROM activities
WHERE user_id = ANY(sqlc.arg(user_ids)::text[])
  AND start_date >= sqlc.arg(start_from) AND start_date <= sqlc.arg(start_to)
ORDER BY start_date ASC;

-- name: ListCampaignFeedActivities :many
SELECT a.*, u.name AS user_name, u.avatar_url AS user_avatar_url
FROM activities a
JOIN campaign_members cm ON cm.user_id = a.user_id
JOIN users u ON u.id = a.user_id
WHERE cm.campaign_id = $1 AND a.start_date >= $2 AND a.start_date <= $3
ORDER BY a.start_date DESC
LIMIT 100;
