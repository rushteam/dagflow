-- name: GetAPITokenByHash :one
SELECT id, name, token_hash, prefix, created_by, expires_at, last_used_at, enabled, created_at
FROM api_tokens
WHERE token_hash = $1 AND enabled = TRUE;

-- name: TouchAPIToken :exec
UPDATE api_tokens SET last_used_at = NOW() WHERE id = $1;

-- name: CreateAPIToken :one
INSERT INTO api_tokens (name, token_hash, prefix, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAPITokensByUser :many
SELECT id, name, prefix, created_by, expires_at, last_used_at, enabled, created_at
FROM api_tokens
WHERE created_by = $1
ORDER BY created_at DESC;

-- name: ListAllAPITokens :many
SELECT t.id, t.name, t.prefix, t.created_by, t.expires_at, t.last_used_at, t.enabled, t.created_at,
       u.username AS creator_name
FROM api_tokens t
JOIN users u ON t.created_by = u.id
ORDER BY t.created_at DESC;

-- name: RevokeAPIToken :exec
UPDATE api_tokens SET enabled = FALSE WHERE id = $1;
