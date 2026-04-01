-- 调度执行记录统一使用 task_runs（trigger_type=schedule），不再维护 schedule_logs。
DROP INDEX IF EXISTS idx_schedule_logs_schedule_id;
DROP TABLE IF EXISTS schedule_logs;
