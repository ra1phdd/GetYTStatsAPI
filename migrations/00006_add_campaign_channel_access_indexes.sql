-- +goose Up
CREATE INDEX IF NOT EXISTS idx_telegram_user_channels_channel_user_id
    ON telegram_user_channels (channel_id, telegram_user_id);

CREATE INDEX IF NOT EXISTS idx_ad_campaigns_channel_created_at
    ON ad_campaigns (channel_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_ad_campaigns_channel_created_at;
DROP INDEX IF EXISTS idx_telegram_user_channels_channel_user_id;
