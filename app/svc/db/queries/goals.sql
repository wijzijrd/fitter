-- name: CreateGoal :one
INSERT INTO goals (id, user_id, metric, target_value, period_start, period_end, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListGoalsForUser :many
SELECT * FROM goals WHERE user_id = ? ORDER BY period_start DESC;

-- name: GetGoal :one
SELECT * FROM goals WHERE id = ?;

-- name: UpdateGoal :one
UPDATE goals SET target_value = ?, period_start = ?, period_end = ?
WHERE id = ? AND user_id = ?
RETURNING *;

-- name: DeleteGoal :exec
DELETE FROM goals WHERE id = ? AND user_id = ?;
