-- name: CreateSchedule :one
INSERT INTO schedules (name, task_id, schedule_type, cron_expr, run_at, variable_overrides, enabled, next_run_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetScheduleByID :one
SELECT * FROM schedules WHERE id = $1;

-- name: ListSchedules :many
SELECT * FROM schedules ORDER BY created_at DESC;

-- name: ListSchedulesByTaskID :many
SELECT * FROM schedules WHERE task_id = $1 ORDER BY created_at DESC;

-- name: GetEnabledSchedules :many
SELECT * FROM schedules WHERE enabled = true ORDER BY id;

-- name: UpdateSchedule :one
UPDATE schedules
SET name = $2, task_id = $3, schedule_type = $4, cron_expr = $5,
    run_at = $6, variable_overrides = $7, enabled = $8, next_run_at = $9, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteSchedule :exec
DELETE FROM schedules WHERE id = $1;

-- name: UpdateScheduleExecution :exec
UPDATE schedules
SET status = $2, last_run_at = $3, next_run_at = $4, updated_at = NOW()
WHERE id = $1;

-- name: SetScheduleEnabled :exec
UPDATE schedules SET enabled = $2, updated_at = NOW() WHERE id = $1;
