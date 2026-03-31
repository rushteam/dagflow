CREATE TABLE IF NOT EXISTS task_runs (
    id            BIGSERIAL    PRIMARY KEY,
    task_id       BIGINT       NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    trigger_type  VARCHAR(16)  NOT NULL DEFAULT 'manual',  -- manual | schedule | dag
    trigger_id    BIGINT,                                  -- schedule_id（调度触发时填写）
    triggered_by  BIGINT       REFERENCES users(id),       -- 手动触发时的用户 ID
    parent_run_id BIGINT       REFERENCES task_runs(id),   -- DAG 父运行 ID（DAG 子任务时填写）
    status        VARCHAR(16)  NOT NULL DEFAULT 'running',  -- running | success | failed | cancelled
    started_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    finished_at   TIMESTAMPTZ,
    duration_ms   BIGINT,
    error_msg     TEXT,
    output        TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

ALTER TABLE task_runs ADD COLUMN IF NOT EXISTS output TEXT;

CREATE INDEX IF NOT EXISTS idx_task_runs_task_id ON task_runs (task_id);
CREATE INDEX IF NOT EXISTS idx_task_runs_status ON task_runs (status) WHERE status = 'running';
CREATE INDEX IF NOT EXISTS idx_task_runs_parent ON task_runs (parent_run_id) WHERE parent_run_id IS NOT NULL;
