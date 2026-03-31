CREATE TABLE IF NOT EXISTS api_tokens (
    id            BIGSERIAL    PRIMARY KEY,
    name          VARCHAR(128) NOT NULL,
    token_hash    VARCHAR(64)  NOT NULL UNIQUE,
    prefix        VARCHAR(12)  NOT NULL,
    created_by    BIGINT       NOT NULL REFERENCES users(id),
    expires_at    TIMESTAMPTZ,
    last_used_at  TIMESTAMPTZ,
    enabled       BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens (token_hash) WHERE enabled = TRUE;
