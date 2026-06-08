-- name: CreateStatsRequest :one
INSERT INTO stats_requests (
    channel_id,
    ad_word,
    start_date,
    end_date,
    hidden_videos,
    fetched_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING id;

-- name: CreateStatsVideo :exec
INSERT INTO stats_videos (
    request_id,
    position,
    name,
    publish_date,
    views,
    url
) VALUES (
    $1, $2, $3, $4, $5, $6
);
