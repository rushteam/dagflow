-- name: CreateCallback :one
INSERT INTO callbacks (name, url, events, headers, body_template, match_mode, task_ids, enabled, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateCallback :one
UPDATE callbacks
SET name = $2, url = $3, events = $4, headers = $5, body_template = $6, match_mode = $7, task_ids = $8, enabled = $9, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteCallback :exec
DELETE FROM callbacks WHERE id = $1;

-- name: GetCallbackByID :one
SELECT * FROM callbacks WHERE id = $1;

-- name: ListCallbacks :many
SELECT * FROM callbacks ORDER BY created_at DESC;

-- name: ListEnabledCallbacks :many
SELECT * FROM callbacks WHERE enabled = TRUE ORDER BY id;
