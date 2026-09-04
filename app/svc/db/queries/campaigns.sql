-- name: CreateCampaign :one
INSERT INTO campaigns (id, name, description, mode, metric, target_value, start_date, end_date, created_by, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetCampaign :one
SELECT * FROM campaigns WHERE id = ?;

-- name: ListCampaigns :many
SELECT * FROM campaigns ORDER BY created_at DESC;

-- name: JoinCampaign :exec
INSERT INTO campaign_members (campaign_id, user_id, joined_at, per_member_target_value)
VALUES (?, ?, ?, ?)
ON CONFLICT (campaign_id, user_id) DO UPDATE SET per_member_target_value = excluded.per_member_target_value;

-- name: LeaveCampaign :exec
DELETE FROM campaign_members WHERE campaign_id = ? AND user_id = ?;

-- name: IsCampaignMember :one
SELECT EXISTS(SELECT 1 FROM campaign_members WHERE campaign_id = ? AND user_id = ?) AS is_member;

-- name: CountCampaignMembers :one
SELECT COUNT(*) AS member_count FROM campaign_members WHERE campaign_id = ?;

-- name: GetCampaignMember :one
SELECT * FROM campaign_members WHERE campaign_id = ? AND user_id = ?;

-- name: ListCampaignMembers :many
SELECT cm.campaign_id, cm.user_id, cm.joined_at, cm.per_member_target_value,
       u.name AS user_name, u.avatar_url AS user_avatar_url
FROM campaign_members cm
JOIN users u ON u.id = cm.user_id
WHERE cm.campaign_id = ?;

-- name: ListJoinedCampaignIDsForUser :many
SELECT campaign_id FROM campaign_members WHERE user_id = ?;
