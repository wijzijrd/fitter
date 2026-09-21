-- name: CreateGoalClause :one
INSERT INTO goal_clauses (id, goal_id, position, scope, metric, comparator, value)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListGoalClausesForGoal :many
SELECT * FROM goal_clauses WHERE goal_id = ? ORDER BY position;

-- name: ListGoalClausesForGoals :many
SELECT * FROM goal_clauses WHERE goal_id IN (sqlc.slice('goal_ids')) ORDER BY goal_id, position;

-- name: DeleteGoalClausesForGoal :exec
DELETE FROM goal_clauses WHERE goal_id = ?;
