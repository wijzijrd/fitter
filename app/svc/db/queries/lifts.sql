-- name: CreateLift :exec
INSERT INTO activity_lifts (activity_id, exercise, weight_kg, reps) VALUES ($1, $2, $3, $4);

-- name: DeleteLiftsForActivity :exec
DELETE FROM activity_lifts WHERE activity_id = $1;

-- name: ListLiftsForActivities :many
SELECT activity_id, exercise, weight_kg, reps FROM activity_lifts
WHERE activity_id = ANY(sqlc.arg(activity_ids)::bigint[]) ORDER BY activity_id, id;
