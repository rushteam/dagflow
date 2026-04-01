CREATE TABLE IF NOT EXISTS callbacks (
    id             BIGSERIAL    PRIMARY KEY,
    name           VARCHAR(128) NOT NULL,
    url            TEXT         NOT NULL,
    events         JSONB        NOT NULL DEFAULT '["success","failed","cancelled"]',
    headers        JSONB        NOT NULL DEFAULT '{}',
    body_template  TEXT         NOT NULL DEFAULT '',
    match_mode     VARCHAR(16)  NOT NULL DEFAULT 'all',  -- all | selected
    task_ids       JSONB        NOT NULL DEFAULT '[]',
    enabled        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_by     BIGINT       REFERENCES users(id),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_callbacks_enabled ON callbacks (enabled) WHERE enabled = TRUE;
