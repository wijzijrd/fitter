-- name: CreateExercise :one
INSERT INTO exercises (id, name, key, measure, loaded, muscles, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetExercise :one
SELECT * FROM exercises WHERE id = $1;

-- name: GetExerciseByKey :one
SELECT * FROM exercises WHERE key = $1;

-- name: GetExercisesByIDs :many
SELECT * FROM exercises WHERE id = ANY(sqlc.arg(ids)::text[]);

-- name: ListExercises :many
SELECT e.id, e.name, e.key, e.measure, e.loaded, e.muscles, e.created_by, e.created_at,
       (SELECT count(*) FROM activity_stations s WHERE s.exercise_id = e.id)::int AS use_count
FROM exercises e
ORDER BY e.name;

-- name: CountExerciseUse :one
SELECT count(*)::int FROM activity_stations WHERE exercise_id = $1;

-- name: UpdateExercise :one
UPDATE exercises SET name = $2, key = $3, measure = $4, loaded = $5, muscles = $6
WHERE id = $1
RETURNING *;

-- name: DeleteExercise :exec
DELETE FROM exercises WHERE id = $1;

-- The three rewrites below keep goal clauses, stations and legacy lifts pointing at the
-- same exercise when one is renamed (key changes) or merged into another.

-- name: RekeyGoalClauses :exec
UPDATE goal_clauses SET exercise = $2 WHERE exercise = $1;

-- name: RekeyLegacyLifts :exec
UPDATE activity_lifts SET exercise = $2 WHERE exercise = $1;

-- name: RepointStations :exec
UPDATE activity_stations SET exercise_id = $2 WHERE exercise_id = $1;

-- name: CountExerciseRefs :one
-- Everything that would dangle if the exercise were deleted: stations, goal clauses that
-- track it, and legacy lifts (still dual-written).
SELECT (
    (SELECT count(*) FROM activity_stations WHERE exercise_id = sqlc.arg(id)::text)
  + (SELECT count(*) FROM goal_clauses WHERE exercise = sqlc.arg(key)::text)
  + (SELECT count(*) FROM activity_lifts WHERE exercise = sqlc.arg(key)::text)
)::int;
