-- name: CreateGoal :one
INSERT INTO goals (id, user_id, campaign_id, name, sport_group, sport_type, period_start, period_end, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetGoal :one
SELECT * FROM goals WHERE id = ?;

-- name: UpdateGoalMeta :one
UPDATE goals SET name = ?, sport_group = ?, sport_type = ?, period_start = ?, period_end = ?
WHERE id = ? AND user_id = ?
RETURNING *;

-- name: DeleteGoal :exec
DELETE FROM goals WHERE id = ? AND user_id = ?;

-- name: ListPersonalGoalsForUser :many
SELECT * FROM goals WHERE user_id = ? AND campaign_id IS NULL ORDER BY period_start DESC;

-- name: ListGoalsForUserInCampaign :many
SELECT * FROM goals WHERE user_id = ? AND campaign_id = ? ORDER BY created_at ASC;

-- name: ListCampaignGoalsWithUsers :many
SELECT g.*, u.name AS user_name, u.avatar_url AS user_avatar_url
FROM goals g
JOIN users u ON u.id = g.user_id
WHERE g.campaign_id = ?
ORDER BY g.sport_group, g.sport_type, u.name;
