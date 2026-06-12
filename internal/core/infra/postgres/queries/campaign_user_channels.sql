-- name: AddUserChannel :one
INSERT INTO telegram_user_channels (
    telegram_user_id,
    channel_id,
    channel_title,
    created_at,
    updated_at
) VALUES ($1, $2, $3, NOW(), NOW())
RETURNING *;

-- name: GetUserChannel :one
SELECT *
FROM telegram_user_channels
WHERE telegram_user_id = $1 AND channel_id = $2;

-- name: ListUserChannels :many
SELECT *
FROM telegram_user_channels
WHERE telegram_user_id = $1
ORDER BY created_at ASC, id ASC;

-- name: DeleteUserChannel :execrows
DELETE FROM telegram_user_channels
WHERE telegram_user_id = $1 AND channel_id = $2;
