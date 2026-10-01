-- name: CreateGoal :one
INSERT INTO goals (id, user_id, metric, target_value, period_start, period_end, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListGoalsForUser :many
SELECT * FROM goals WHERE user_id = $1 ORDER BY period_start DESC;

-- name: GetGoal :one
SELECT * FROM goals WHERE id = $1;

-- name: UpdateGoal :one
UPDATE goals SET target_value = $1, period_start = $2, period_end = $3
WHERE id = $4 AND user_id = $5
RETURNING *;

-- name: DeleteGoal :exec
DELETE FROM goals WHERE id = $1 AND user_id = $2;
