-- Keep the original 210 migration immutable for installations that already
-- applied the first XiaoAPI video schema.
ALTER TABLE video_jobs
    ADD COLUMN IF NOT EXISTS upstream_amount NUMERIC(20, 8),
    ADD COLUMN IF NOT EXISTS upstream_currency VARCHAR(16);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'video_jobs_upstream_amount_nonnegative'
          AND conrelid = 'video_jobs'::regclass
    ) THEN
        ALTER TABLE video_jobs
            ADD CONSTRAINT video_jobs_upstream_amount_nonnegative
            CHECK (upstream_amount IS NULL OR upstream_amount >= 0);
    END IF;
END $$;
