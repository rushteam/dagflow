CREATE TABLE IF NOT EXISTS tasks (
    id          BIGSERIAL    PRIMARY KEY,
    name        VARCHAR(128) UNIQUE NOT NULL,
    label       VARCHAR(256) NOT NULL DEFAULT '',
    kind        VARCHAR(64)  NOT NULL,
    payload     JSONB        NOT NULL DEFAULT '{}',
    variables   JSONB        NOT NULL DEFAULT '[]',
    enabled     BOOLEAN      NOT NULL DEFAULT true,
    created_by  BIGINT       REFERENCES users(id),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS variables JSONB NOT NULL DEFAULT '[]';
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS callback JSONB NOT NULL DEFAULT 'null';

CREATE INDEX IF NOT EXISTS idx_tasks_kind ON tasks (kind);
