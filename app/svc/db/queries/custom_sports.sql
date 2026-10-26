-- name: CreateCustomSport :one
INSERT INTO custom_sports (id, key, label, created_by, created_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListCustomSports :many
SELECT * FROM custom_sports ORDER BY created_at;

-- name: GetCustomSportByKey :one
SELECT * FROM custom_sports WHERE key = $1;

-- name: GetCustomSportByID :one
SELECT * FROM custom_sports WHERE id = $1;

-- name: DeleteCustomSport :exec
DELETE FROM custom_sports WHERE id = $1;
