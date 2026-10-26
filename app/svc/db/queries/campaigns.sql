-- name: CreateCampaign :one
INSERT INTO campaigns (id, name, description, metric, target_value, start_date, end_date, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetCampaign :one
SELECT * FROM campaigns WHERE id = $1;

-- name: ListCampaigns :many
SELECT * FROM campaigns ORDER BY created_at DESC;

-- name: JoinCampaign :exec
INSERT INTO campaign_members (campaign_id, user_id, joined_at)
VALUES ($1, $2, $3)
ON CONFLICT (campaign_id, user_id) DO NOTHING;

-- name: LeaveCampaign :exec
DELETE FROM campaign_members WHERE campaign_id = $1 AND user_id = $2;

-- name: IsCampaignMember :one
SELECT EXISTS(SELECT 1 FROM campaign_members WHERE campaign_id = $1 AND user_id = $2) AS is_member;

-- name: CountCampaignMembers :one
SELECT COUNT(*) AS member_count FROM campaign_members WHERE campaign_id = $1;

-- name: GetCampaignMember :one
SELECT * FROM campaign_members WHERE campaign_id = $1 AND user_id = $2;

-- name: ListCampaignMembers :many
SELECT cm.campaign_id, cm.user_id, cm.joined_at,
       u.name AS user_name, u.avatar_url AS user_avatar_url
FROM campaign_members cm
JOIN users u ON u.id = cm.user_id
WHERE cm.campaign_id = $1;

-- name: ListJoinedCampaignIDsForUser :many
SELECT campaign_id FROM campaign_members WHERE user_id = $1;

-- name: CountCampaignGoals :one
SELECT COUNT(*) FROM goals WHERE campaign_id = $1;

-- name: DeleteGoalsForUserInCampaign :exec
DELETE FROM goals WHERE campaign_id = $1 AND user_id = $2;
