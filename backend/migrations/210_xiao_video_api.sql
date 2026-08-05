-- XiaoAPI-compatible video proxy ownership, idempotency, and billing holds.
CREATE TABLE IF NOT EXISTS video_media (
    id BIGSERIAL PRIMARY KEY,
    media_id VARCHAR(64) NOT NULL UNIQUE,
    upstream_media_id VARCHAR(128) NOT NULL,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    upstream_url TEXT NOT NULL,
    media_type VARCHAR(32) NOT NULL DEFAULT 'UPLOADED',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_video_media_owner_created
    ON video_media (api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_video_media_account
    ON video_media (account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_video_media_expires_at
    ON video_media (expires_at);

CREATE TABLE IF NOT EXISTS video_jobs (
    id BIGSERIAL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL UNIQUE,
    upstream_job_id VARCHAR(128) NOT NULL,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    idempotency_key VARCHAR(128),
    request_hash VARCHAR(64) NOT NULL,
    model VARCHAR(128) NOT NULL,
    resolution VARCHAR(32) NOT NULL,
    duration INTEGER NOT NULL,
    aspect_ratio VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    amount NUMERIC(20, 8) NOT NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'USD',
    settlement_status VARCHAR(16) NOT NULL DEFAULT 'held',
    upstream_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    settled_at TIMESTAMPTZ,
    CONSTRAINT video_jobs_amount_nonnegative CHECK (amount >= 0),
    CONSTRAINT video_jobs_status_check
        CHECK (status IN ('pending', 'running', 'settling', 'completed', 'failed', 'canceled')),
    CONSTRAINT video_jobs_settlement_status_check
        CHECK (settlement_status IN ('held', 'captured', 'released'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_video_jobs_account_upstream
    ON video_jobs (account_id, upstream_job_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_video_jobs_api_key_idempotency
    ON video_jobs (api_key_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';
CREATE INDEX IF NOT EXISTS idx_video_jobs_owner_created
    ON video_jobs (api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_video_jobs_account
    ON video_jobs (account_id, updated_at ASC);
CREATE INDEX IF NOT EXISTS idx_video_jobs_active
    ON video_jobs (updated_at)
    WHERE status IN ('pending', 'running', 'settling');
