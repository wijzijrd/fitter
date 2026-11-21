-- name: InsertAdminAction :exec
INSERT INTO admin_actions (id, actor_id, action, target_id, detail, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListAdminActions :many
SELECT a.*, u.name AS actor_name
FROM admin_actions a
JOIN users u ON u.id = a.actor_id
ORDER BY a.created_at DESC
LIMIT $1;
