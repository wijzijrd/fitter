-- name: CreateCampaign :one
INSERT INTO campaigns (id, name, description, mode, metric, target_value, start_date, end_date, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetCampaign :one
SELECT * FROM campaigns WHERE id = $1;

-- name: ListCampaigns :many
SELECT * FROM campaigns ORDER BY created_at DESC;

-- name: JoinCampaign :exec
INSERT INTO campaign_members (campaign_id, user_id, joined_at, per_member_target_value)
VALUES ($1, $2, $3, $4)
ON CONFLICT (campaign_id, user_id) DO UPDATE SET per_member_target_value = excluded.per_member_target_value;

-- name: LeaveCampaign :exec
DELETE FROM campaign_members WHERE campaign_id = $1 AND user_id = $2;

-- name: IsCampaignMember :one
SELECT EXISTS(SELECT 1 FROM campaign_members WHERE campaign_id = $1 AND user_id = $2) AS is_member;

-- name: CountCampaignMembers :one
SELECT COUNT(*) AS member_count FROM campaign_members WHERE campaign_id = $1;

-- name: GetCampaignMember :one
SELECT * FROM campaign_members WHERE campaign_id = $1 AND user_id = $2;

-- name: ListCampaignMembers :many
SELECT cm.campaign_id, cm.user_id, cm.joined_at, cm.per_member_target_value,
       u.name AS user_name, u.avatar_url AS user_avatar_url
FROM campaign_members cm
JOIN users u ON u.id = cm.user_id
WHERE cm.campaign_id = $1;

-- name: ListJoinedCampaignIDsForUser :many
SELECT campaign_id FROM campaign_members WHERE user_id = $1;
