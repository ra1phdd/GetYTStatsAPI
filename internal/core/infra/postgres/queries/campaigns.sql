-- name: CreateCampaign :one
INSERT INTO ad_campaigns (
    telegram_user_id,
    channel_id,
    channel_title,
    keyword,
    start_date,
    timezone,
    target_views,
    columns,
    status,
    export_jwt,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, NOW(), NOW())
RETURNING *;

-- name: GetCampaignByID :one
SELECT *
FROM ad_campaigns
WHERE ad_campaigns.id = $1;

-- name: GetCampaignByIDForUser :one
SELECT *
FROM ad_campaigns
WHERE ad_campaigns.id = $1 AND EXISTS (
    SELECT 1
    FROM telegram_user_channels
    WHERE telegram_user_channels.telegram_user_id = $2 AND telegram_user_channels.channel_id = ad_campaigns.channel_id
);

-- name: GetCampaignByExportJWT :one
SELECT *
FROM ad_campaigns
WHERE export_jwt = $1;

-- name: ListUserCampaigns :many
SELECT *
FROM ad_campaigns
WHERE EXISTS (
    SELECT 1
    FROM telegram_user_channels
    WHERE telegram_user_channels.telegram_user_id = $1 AND telegram_user_channels.channel_id = ad_campaigns.channel_id
)
ORDER BY ad_campaigns.created_at DESC, ad_campaigns.id DESC;

-- name: GetCampaignByChannelIdentity :one
SELECT *
FROM ad_campaigns
WHERE ad_campaigns.channel_id = $1 AND ad_campaigns.keyword = $2 AND ad_campaigns.start_date = $3
ORDER BY ad_campaigns.id ASC
LIMIT 1;

-- name: UpdateCampaignAfterSnapshot :exec
UPDATE ad_campaigns
SET status = $2,
    closed_at = $3,
    close_reason = $4,
    last_snapshot_at = $5,
    last_total_views = $6,
    last_daily_growth = $7,
    estimated_close_date = $8,
    updated_at = NOW()
WHERE id = $1;

-- name: CloseCampaign :execrows
UPDATE ad_campaigns
SET status = $2,
    closed_at = $3,
    close_reason = $4,
    updated_at = NOW()
WHERE id = $1 AND closed_at IS NULL;

-- name: UpdateCampaignColumns :exec
UPDATE ad_campaigns
SET columns = $2::jsonb,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateCampaignTarget :exec
UPDATE ad_campaigns
SET target_views = $2,
    status = $3,
    closed_at = $4,
    close_reason = $5,
    estimated_close_date = $6,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateCampaignSpreadsheet :exec
UPDATE ad_campaigns
SET spreadsheet_id = $2,
    spreadsheet_url = $3,
    updated_at = NOW()
WHERE id = $1;
