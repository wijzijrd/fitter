-- name: CreateCustomMetricSport :exec
INSERT INTO custom_metric_sports (metric_id, sport_ref) VALUES (?, ?);

-- name: ListCustomMetricSportsForMetric :many
SELECT sport_ref FROM custom_metric_sports WHERE metric_id = ? ORDER BY sport_ref;

-- name: ListCustomMetricSportsForMetrics :many
-- Batch form used by CatalogService.ListMetrics: one query for every custom
-- metric's applicability rows, instead of one query per metric.
SELECT * FROM custom_metric_sports WHERE metric_id IN (sqlc.slice('metric_ids')) ORDER BY metric_id, sport_ref;
