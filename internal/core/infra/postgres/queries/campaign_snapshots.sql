-- name: CreateCampaignSnapshot :one
INSERT INTO ad_campaign_snapshots (
    campaign_id,
    snapshot_at,
    total_views,
    remaining_views,
    daily_growth,
    estimated_close_date,
    created_at
) VALUES ($1, $2, $3, $4, $5, $6, NOW())
RETURNING id;

-- name: GetLatestSnapshot :one
SELECT *
FROM ad_campaign_snapshots
WHERE campaign_id = $1
ORDER BY snapshot_at DESC, id DESC
LIMIT 1;

-- name: GetPreviousSnapshot :one
SELECT *
FROM ad_campaign_snapshots
WHERE campaign_id = $1 AND snapshot_at < $2
ORDER BY snapshot_at DESC, id DESC
LIMIT 1;

-- name: CreateCampaignSnapshotVideo :exec
INSERT INTO ad_campaign_snapshot_videos (
    snapshot_id,
    position,
    video_id,
    name,
    publish_date,
    views,
    url,
    views_updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListCampaignSnapshotVideos :many
SELECT video_id, name, publish_date, views, url, views_updated_at
FROM ad_campaign_snapshot_videos
WHERE snapshot_id = $1
ORDER BY publish_date ASC, position ASC, id ASC;
