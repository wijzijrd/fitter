-- name: CreateCustomMetricSport :exec
INSERT INTO custom_metric_sports (metric_id, sport_ref) VALUES ($1, $2);

-- name: ListCustomMetricSportsForMetric :many
SELECT sport_ref FROM custom_metric_sports WHERE metric_id = $1 ORDER BY sport_ref;

-- name: ListCustomMetricSportsForMetrics :many
-- Batch form used by CatalogService.ListMetrics: one query for every custom
-- metric's applicability rows, instead of one query per metric.
SELECT * FROM custom_metric_sports WHERE metric_id = ANY(sqlc.arg(metric_ids)::text[]) ORDER BY metric_id, sport_ref;
