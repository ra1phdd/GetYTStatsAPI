-- +goose Up
CREATE TABLE IF NOT EXISTS telegram_user_channels (
    id BIGSERIAL PRIMARY KEY,
    telegram_user_id BIGINT NOT NULL,
    channel_id TEXT NOT NULL,
    channel_title TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_telegram_user_channel UNIQUE (telegram_user_id, channel_id)
);

CREATE INDEX IF NOT EXISTS idx_telegram_user_channels_user_id
    ON telegram_user_channels (telegram_user_id);

CREATE TABLE IF NOT EXISTS telegram_user_settings (
    telegram_user_id BIGINT PRIMARY KEY,
    notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    notification_time TEXT NOT NULL DEFAULT '12:00',
    timezone TEXT NOT NULL DEFAULT 'Europe/Moscow',
    last_notification_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ad_campaigns (
    id BIGSERIAL PRIMARY KEY,
    telegram_user_id BIGINT NOT NULL,
    channel_id TEXT NOT NULL,
    channel_title TEXT NOT NULL,
    keyword TEXT NOT NULL,
    start_date DATE NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'Europe/Moscow',
    target_views BIGINT,
    status TEXT NOT NULL,
    export_jwt TEXT NOT NULL,
    closed_at TIMESTAMPTZ,
    close_reason TEXT,
    last_snapshot_at TIMESTAMPTZ,
    last_total_views BIGINT NOT NULL DEFAULT 0,
    last_daily_growth BIGINT,
    estimated_close_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_ad_campaign_identity UNIQUE (telegram_user_id, channel_id, keyword, start_date),
    CONSTRAINT uq_ad_campaign_export_jwt UNIQUE (export_jwt)
);

CREATE INDEX IF NOT EXISTS idx_ad_campaigns_user_status_created_at
    ON ad_campaigns (telegram_user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ad_campaigns_status_start_date
    ON ad_campaigns (status, start_date);

CREATE TABLE IF NOT EXISTS ad_campaign_snapshots (
    id BIGSERIAL PRIMARY KEY,
    campaign_id BIGINT NOT NULL REFERENCES ad_campaigns(id) ON DELETE CASCADE,
    snapshot_at TIMESTAMPTZ NOT NULL,
    total_views BIGINT NOT NULL,
    remaining_views BIGINT,
    daily_growth BIGINT,
    estimated_close_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ad_campaign_snapshots_campaign_snapshot_at
    ON ad_campaign_snapshots (campaign_id, snapshot_at DESC);

CREATE TABLE IF NOT EXISTS ad_campaign_snapshot_videos (
    id BIGSERIAL PRIMARY KEY,
    snapshot_id BIGINT NOT NULL REFERENCES ad_campaign_snapshots(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    video_id TEXT NOT NULL,
    name TEXT NOT NULL,
    publish_date TIMESTAMPTZ NOT NULL,
    views BIGINT NOT NULL,
    url TEXT NOT NULL,
    views_updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ad_campaign_snapshot_videos_snapshot_id
    ON ad_campaign_snapshot_videos (snapshot_id, position);

CREATE TABLE IF NOT EXISTS telegram_input_sessions (
    telegram_user_id BIGINT PRIMARY KEY,
    flow TEXT NOT NULL,
    step TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS public_user_sessions (
    id TEXT PRIMARY KEY,
    telegram_user_id BIGINT NOT NULL,
    refresh_token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_public_user_sessions_user_id
    ON public_user_sessions (telegram_user_id, expires_at DESC);

-- +goose Down
DROP TABLE IF EXISTS public_user_sessions;
DROP TABLE IF EXISTS telegram_input_sessions;
DROP TABLE IF EXISTS ad_campaign_snapshot_videos;
DROP TABLE IF EXISTS ad_campaign_snapshots;
DROP TABLE IF EXISTS ad_campaigns;
DROP TABLE IF EXISTS telegram_user_settings;
DROP TABLE IF EXISTS telegram_user_channels;
