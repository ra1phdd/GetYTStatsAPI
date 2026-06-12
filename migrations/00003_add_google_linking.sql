-- +goose Up
ALTER TABLE telegram_user_settings
    ADD COLUMN IF NOT EXISTS google_email TEXT,
    ADD COLUMN IF NOT EXISTS google_refresh_token TEXT,
    ADD COLUMN IF NOT EXISTS google_connected_at TIMESTAMPTZ;

ALTER TABLE ad_campaigns
    ADD COLUMN IF NOT EXISTS spreadsheet_id TEXT,
    ADD COLUMN IF NOT EXISTS spreadsheet_url TEXT;

-- +goose Down
ALTER TABLE ad_campaigns
    DROP COLUMN IF EXISTS spreadsheet_url,
    DROP COLUMN IF EXISTS spreadsheet_id;

ALTER TABLE telegram_user_settings
    DROP COLUMN IF EXISTS google_connected_at,
    DROP COLUMN IF EXISTS google_refresh_token,
    DROP COLUMN IF EXISTS google_email;
