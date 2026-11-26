-- name: CreateGoal :one
INSERT INTO goals (id, user_id, campaign_id, name, sport_group, sport_type, period_start, period_end, created_at, recurrence)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE(NULLIF(sqlc.arg(recurrence)::text, ''), 'none'))
RETURNING *;

-- name: GetGoal :one
SELECT * FROM goals WHERE id = $1;

-- name: UpdateGoalMeta :one
UPDATE goals SET name = $1, sport_group = $2, sport_type = $3, period_start = $4, period_end = $5, campaign_id = $8,
    recurrence = COALESCE(NULLIF(sqlc.arg(recurrence)::text, ''), 'none')
WHERE id = $6 AND user_id = $7
RETURNING *;

-- name: DeleteGoal :exec
DELETE FROM goals WHERE id = $1 AND user_id = $2;

-- name: ListPersonalGoalsForUser :many
SELECT * FROM goals WHERE user_id = $1 AND campaign_id IS NULL ORDER BY period_start DESC;

-- name: ListAllGoalsForUser :many
-- Personal goals plus every campaign goal of the user, for the goal hub.
SELECT * FROM goals WHERE user_id = $1 ORDER BY campaign_id NULLS FIRST, period_start DESC, created_at ASC;

-- name: ListGoalsForUserInCampaign :many
SELECT * FROM goals WHERE user_id = $1 AND campaign_id = $2 ORDER BY created_at ASC;

-- name: ListCampaignGoalsWithUsers :many
SELECT g.*, u.name AS user_name, u.avatar_url AS user_avatar_url
FROM goals g
JOIN users u ON u.id = g.user_id
WHERE g.campaign_id = $1
-- NULLS FIRST keeps "all sports" goals ahead of grouped ones.
ORDER BY g.sport_group NULLS FIRST, g.sport_type NULLS FIRST, u.name;
