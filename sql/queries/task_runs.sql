-- name: CreateTaskRun :one
INSERT INTO task_runs (task_id, trigger_type, trigger_id, triggered_by, parent_run_id, status, started_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: FinishTaskRun :exec
UPDATE task_runs
SET finished_at = $2, status = $3, error_msg = $4, duration_ms = $5, output = $6
WHERE id = $1;

-- name: ListTaskRunsByTaskID :many
SELECT * FROM task_runs
WHERE task_id = $1
ORDER BY created_at DESC
LIMIT 50;

-- name: GetTaskRunByID :one
SELECT * FROM task_runs WHERE id = $1;

-- name: GetTaskRunDetail :one
SELECT
    tr.id, tr.task_id, tr.trigger_type, tr.trigger_id, tr.triggered_by,
    tr.parent_run_id, tr.status, tr.started_at, tr.finished_at, tr.duration_ms, tr.error_msg, tr.output, tr.created_at,
    t.name AS task_name, t.label AS task_label, t.kind AS task_kind
FROM task_runs tr
JOIN tasks t ON tr.task_id = t.id
WHERE tr.id = $1;

-- name: ListChildRuns :many
SELECT
    tr.id, tr.task_id, tr.trigger_type, tr.trigger_id, tr.triggered_by,
    tr.parent_run_id, tr.status, tr.started_at, tr.finished_at, tr.duration_ms, tr.error_msg, tr.created_at,
    t.name AS task_name, t.label AS task_label, t.kind AS task_kind
FROM task_runs tr
JOIN tasks t ON tr.task_id = t.id
WHERE tr.parent_run_id = $1
ORDER BY tr.started_at ASC;

-- name: CountTaskRuns :one
SELECT COUNT(*) FROM task_runs tr
JOIN tasks t ON tr.task_id = t.id
WHERE (sqlc.narg('task_name')::text IS NULL OR t.name ILIKE '%' || sqlc.narg('task_name')::text || '%')
  AND (sqlc.narg('task_label')::text IS NULL OR t.label ILIKE '%' || sqlc.narg('task_label')::text || '%')
  AND (sqlc.narg('run_id')::bigint IS NULL OR tr.id = sqlc.narg('run_id')::bigint);

-- name: ListTaskRunsPaged :many
SELECT
    tr.id, tr.task_id, tr.trigger_type, tr.trigger_id, tr.triggered_by,
    tr.status, tr.started_at, tr.finished_at, tr.duration_ms, tr.error_msg, tr.created_at,
    t.name AS task_name, t.label AS task_label, t.kind AS task_kind
FROM task_runs tr
JOIN tasks t ON tr.task_id = t.id
WHERE (sqlc.narg('task_name')::text IS NULL OR t.name ILIKE '%' || sqlc.narg('task_name')::text || '%')
  AND (sqlc.narg('task_label')::text IS NULL OR t.label ILIKE '%' || sqlc.narg('task_label')::text || '%')
  AND (sqlc.narg('run_id')::bigint IS NULL OR tr.id = sqlc.narg('run_id')::bigint)
ORDER BY tr.created_at DESC
LIMIT $1 OFFSET $2;
