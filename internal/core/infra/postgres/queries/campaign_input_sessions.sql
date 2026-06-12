-- name: GetInputSession :one
SELECT *
FROM telegram_input_sessions
WHERE telegram_user_id = $1;

-- name: UpsertInputSession :one
INSERT INTO telegram_input_sessions (
    telegram_user_id,
    flow,
    step,
    payload,
    expires_at,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4::jsonb, $5, NOW(), NOW())
ON CONFLICT (telegram_user_id) DO UPDATE SET
    flow = EXCLUDED.flow,
    step = EXCLUDED.step,
    payload = EXCLUDED.payload,
    expires_at = EXCLUDED.expires_at,
    updated_at = NOW()
RETURNING *;

-- name: DeleteInputSession :exec
DELETE FROM telegram_input_sessions
WHERE telegram_user_id = $1;
