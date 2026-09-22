-- name: CreateCustomSport :one
INSERT INTO custom_sports (id, key, label, created_by, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListCustomSports :many
SELECT * FROM custom_sports ORDER BY created_at;

-- name: GetCustomSportByKey :one
SELECT * FROM custom_sports WHERE key = ?;

-- name: GetCustomSportByID :one
SELECT * FROM custom_sports WHERE id = ?;

-- name: DeleteCustomSport :exec
DELETE FROM custom_sports WHERE id = ?;
