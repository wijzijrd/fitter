-- name: CreateCustomMetric :one
INSERT INTO custom_metrics (
    id, key, label, unit, aggregation, nullable, lower_is_better,
    supports_single_scope, supports_aggregate_scope, created_by, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListCustomMetrics :many
SELECT * FROM custom_metrics ORDER BY created_at;

-- name: GetCustomMetricByKey :one
SELECT * FROM custom_metrics WHERE key = $1;

-- name: GetCustomMetricByID :one
SELECT * FROM custom_metrics WHERE id = $1;

-- name: DeleteCustomMetric :exec
DELETE FROM custom_metrics WHERE id = $1;
