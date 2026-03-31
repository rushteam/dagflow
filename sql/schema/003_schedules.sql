CREATE TABLE IF NOT EXISTS schedules (
    id                  BIGSERIAL    PRIMARY KEY,
    name                VARCHAR(128) NOT NULL,
    task_id             BIGINT       NOT NULL REFERENCES tasks(id),
    schedule_type       VARCHAR(16)  NOT NULL DEFAULT 'cron',
    cron_expr           VARCHAR(128),
    run_at              TIMESTAMPTZ,
    variable_overrides  JSONB        NOT NULL DEFAULT '[]',
    enabled             BOOLEAN      NOT NULL DEFAULT true,
    status              VARCHAR(16)  NOT NULL DEFAULT 'idle',
    last_run_at         TIMESTAMPTZ,
    next_run_at         TIMESTAMPTZ,
    created_by          BIGINT       REFERENCES users(id),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

ALTER TABLE schedules ADD COLUMN IF NOT EXISTS variable_overrides JSONB NOT NULL DEFAULT '[]';

CREATE TABLE IF NOT EXISTS schedule_logs (
    id            BIGSERIAL    PRIMARY KEY,
    schedule_id   BIGINT       NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    started_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    finished_at   TIMESTAMPTZ,
    status        VARCHAR(16)  NOT NULL DEFAULT 'running',
    error_msg     TEXT,
    duration_ms   BIGINT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_schedules_enabled ON schedules (enabled) WHERE enabled = true;
CREATE INDEX IF NOT EXISTS idx_schedule_logs_schedule_id ON schedule_logs (schedule_id);
