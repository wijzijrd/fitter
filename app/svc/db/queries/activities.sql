-- name: UpsertActivity :one
INSERT INTO activities (
    strava_activity_id, user_id, type, sport_type, name, start_date,
    distance_m, elevation_m, moving_time_s, elapsed_time_s, average_speed_ms, max_speed_ms,
    average_heartrate, max_heartrate, average_watts, weighted_average_watts, max_watts,
    device_watts, kilojoules, calories, average_cadence, suffer_score,
    created_at, updated_at
)
VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?
)
ON CONFLICT (strava_activity_id) DO UPDATE SET
    user_id                = excluded.user_id,
    type                   = excluded.type,
    sport_type             = excluded.sport_type,
    name                   = excluded.name,
    start_date             = excluded.start_date,
    distance_m             = excluded.distance_m,
    elevation_m            = excluded.elevation_m,
    moving_time_s          = excluded.moving_time_s,
    elapsed_time_s         = excluded.elapsed_time_s,
    average_speed_ms       = excluded.average_speed_ms,
    max_speed_ms           = excluded.max_speed_ms,
    average_heartrate      = COALESCE(excluded.average_heartrate, average_heartrate),
    max_heartrate          = COALESCE(excluded.max_heartrate, max_heartrate),
    average_watts          = COALESCE(excluded.average_watts, average_watts),
    weighted_average_watts = COALESCE(excluded.weighted_average_watts, weighted_average_watts),
    max_watts              = COALESCE(excluded.max_watts, max_watts),
    device_watts           = excluded.device_watts,
    kilojoules             = COALESCE(excluded.kilojoules, kilojoules),
    calories               = COALESCE(excluded.calories, calories),
    average_cadence        = COALESCE(excluded.average_cadence, average_cadence),
    suffer_score           = COALESCE(excluded.suffer_score, suffer_score),
    updated_at             = excluded.updated_at
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

-- name: GetLatestActivityStartDateForUser :one
SELECT CAST(COALESCE(MAX(start_date), 0) AS INTEGER) FROM activities WHERE user_id = ?;
