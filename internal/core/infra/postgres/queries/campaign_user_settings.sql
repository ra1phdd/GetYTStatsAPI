-- name: GetUserSettings :one
SELECT *
FROM telegram_user_settings
WHERE telegram_user_id = $1;

-- name: UpsertUserSettings :one
INSERT INTO telegram_user_settings (
    telegram_user_id,
    notifications_enabled,
    notification_time,
    notification_interval_minutes,
    timezone,
    google_email,
    google_refresh_token,
    google_connected_at,
    last_notification_sent_at,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, NULL, NULL, NULL, $6, NOW(), NOW())
ON CONFLICT (telegram_user_id) DO UPDATE SET
    notifications_enabled = EXCLUDED.notifications_enabled,
    notification_time = EXCLUDED.notification_time,
    notification_interval_minutes = EXCLUDED.notification_interval_minutes,
    timezone = EXCLUDED.timezone,
    last_notification_sent_at = COALESCE(EXCLUDED.last_notification_sent_at, telegram_user_settings.last_notification_sent_at),
    updated_at = NOW()
RETURNING *;

-- name: SaveGoogleLink :one
INSERT INTO telegram_user_settings (
    telegram_user_id,
    notifications_enabled,
    notification_time,
    notification_interval_minutes,
    timezone,
    google_email,
    google_refresh_token,
    google_connected_at,
    created_at,
    updated_at
) VALUES ($1, TRUE, $2, $3, $4, $5, $6, $7, NOW(), NOW())
ON CONFLICT (telegram_user_id) DO UPDATE SET
    google_email = EXCLUDED.google_email,
    google_refresh_token = EXCLUDED.google_refresh_token,
    google_connected_at = EXCLUDED.google_connected_at,
    updated_at = NOW()
RETURNING *;

-- name: MarkNotificationSent :exec
INSERT INTO telegram_user_settings (
    telegram_user_id,
    notifications_enabled,
    notification_time,
    notification_interval_minutes,
    timezone,
    last_notification_sent_at,
    created_at,
    updated_at
) VALUES ($1, TRUE, $2, $3, $4, $5, NOW(), NOW())
ON CONFLICT (telegram_user_id) DO UPDATE SET
    last_notification_sent_at = EXCLUDED.last_notification_sent_at,
    updated_at = NOW();

-- name: ListNotificationUsers :many
SELECT users.telegram_user_id,
    COALESCE(settings.notifications_enabled, TRUE) AS notifications_enabled,
    COALESCE(settings.notification_time, $1) AS notification_time,
    COALESCE(settings.notification_interval_minutes, $2) AS notification_interval_minutes,
    COALESCE(settings.timezone, $3) AS timezone,
    settings.last_notification_sent_at,
    COALESCE(settings.created_at, NOW()) AS created_at,
    COALESCE(settings.updated_at, NOW()) AS updated_at
FROM (
    SELECT telegram_user_id FROM telegram_user_settings
    UNION
    SELECT DISTINCT telegram_user_id FROM telegram_user_channels
    UNION
    SELECT DISTINCT telegram_user_id FROM ad_campaigns WHERE closed_at IS NULL
) AS users
LEFT JOIN telegram_user_settings AS settings ON settings.telegram_user_id = users.telegram_user_id
WHERE COALESCE(settings.notifications_enabled, TRUE) = TRUE
ORDER BY users.telegram_user_id ASC;
