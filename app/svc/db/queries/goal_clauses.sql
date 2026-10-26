-- name: CreateGoalClause :one
INSERT INTO goal_clauses (id, goal_id, position, scope, metric, comparator, value)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListGoalClausesForGoal :many
SELECT * FROM goal_clauses WHERE goal_id = $1 ORDER BY position;

-- name: ListGoalClausesForGoals :many
SELECT * FROM goal_clauses WHERE goal_id = ANY(sqlc.arg(goal_ids)::text[]) ORDER BY goal_id, position;

-- name: DeleteGoalClausesForGoal :exec
DELETE FROM goal_clauses WHERE goal_id = $1;
