-- name: CreatePublicSession :exec
INSERT INTO public_user_sessions (
    id,
    telegram_user_id,
    refresh_token_hash,
    expires_at,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, NOW(), NOW());

-- name: GetPublicSession :one
SELECT *
FROM public_user_sessions
WHERE id = $1;

-- name: UpdatePublicSession :exec
UPDATE public_user_sessions
SET refresh_token_hash = $2,
    expires_at = $3,
    revoked_at = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: RevokePublicSession :exec
UPDATE public_user_sessions
SET revoked_at = NOW(), updated_at = NOW()
WHERE id = $1;
