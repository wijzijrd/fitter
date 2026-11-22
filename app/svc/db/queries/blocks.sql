-- name: CreateBlock :one
INSERT INTO activity_blocks (activity_id, position, rounds) VALUES ($1, $2, $3) RETURNING id;

-- name: CreateStation :exec
INSERT INTO activity_stations (block_id, position, exercise_id, weight_kg, reps, distance_m, duration_s)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: DeleteBlocksForActivity :exec
DELETE FROM activity_blocks WHERE activity_id = $1;

-- name: ListBlocksForActivities :many
SELECT id, activity_id, position, rounds FROM activity_blocks
WHERE activity_id = ANY(sqlc.arg(activity_ids)::bigint[]) ORDER BY activity_id, position;

-- name: ListStationsForActivities :many
SELECT s.block_id, b.activity_id, s.exercise_id, e.name AS exercise_name, e.key AS exercise_key,
       s.weight_kg, s.reps, s.distance_m, s.duration_s
FROM activity_stations s
JOIN activity_blocks b ON b.id = s.block_id
JOIN exercises e ON e.id = s.exercise_id
WHERE b.activity_id = ANY(sqlc.arg(activity_ids)::bigint[])
ORDER BY b.activity_id, b.position, s.position;
