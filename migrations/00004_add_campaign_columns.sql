-- +goose Up
ALTER TABLE ad_campaigns
    ADD COLUMN IF NOT EXISTS columns JSONB NOT NULL DEFAULT '["id","publish_date","video_url","views"]'::jsonb;

-- +goose Down
ALTER TABLE ad_campaigns
    DROP COLUMN IF EXISTS columns;
