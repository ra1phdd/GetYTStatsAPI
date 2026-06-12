-- +goose Up
ALTER TABLE telegram_user_settings
    ADD COLUMN IF NOT EXISTS notification_interval_minutes INTEGER NOT NULL DEFAULT 1440;

-- +goose Down
ALTER TABLE telegram_user_settings
    DROP COLUMN IF EXISTS notification_interval_minutes;
