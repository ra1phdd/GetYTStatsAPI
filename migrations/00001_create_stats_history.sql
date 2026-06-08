-- +goose Up
CREATE TABLE IF NOT EXISTS stats_requests (
    id BIGSERIAL PRIMARY KEY,
    channel_id TEXT NOT NULL,
    ad_word TEXT NOT NULL,
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ NOT NULL,
    hidden_videos JSONB NOT NULL DEFAULT '[]'::jsonb,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS stats_videos (
    id BIGSERIAL PRIMARY KEY,
    request_id BIGINT NOT NULL REFERENCES stats_requests(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    name TEXT NOT NULL,
    publish_date TIMESTAMPTZ NOT NULL,
    views BIGINT NOT NULL,
    url TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_stats_videos_request_id ON stats_videos (request_id);
CREATE INDEX IF NOT EXISTS idx_stats_requests_channel_id_fetched_at ON stats_requests (channel_id, fetched_at DESC);

-- +goose Down
DROP TABLE IF EXISTS stats_videos;
DROP TABLE IF EXISTS stats_requests;
