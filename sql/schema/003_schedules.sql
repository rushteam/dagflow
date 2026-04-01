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

CREATE INDEX IF NOT EXISTS idx_schedules_enabled ON schedules (enabled) WHERE enabled = true;
