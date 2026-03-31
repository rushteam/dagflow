-- name: CreateTask :one
INSERT INTO tasks (name, label, kind, payload, variables, enabled, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetTaskByID :one
SELECT * FROM tasks WHERE id = $1;

-- name: GetTaskByName :one
SELECT * FROM tasks WHERE name = $1;

-- name: ListTasks :many
SELECT * FROM tasks ORDER BY created_at DESC;

-- name: ListEnabledTasks :many
SELECT * FROM tasks WHERE enabled = true ORDER BY name;

-- name: UpdateTask :one
UPDATE tasks
SET name = $2, label = $3, kind = $4, payload = $5, variables = $6, enabled = $7, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE id = $1;
