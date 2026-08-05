-- Backfill usage records for XiaoAPI video jobs captured before usage logging
-- was added to the settlement transaction.
INSERT INTO usage_logs (
    user_id,
    api_key_id,
    account_id,
    request_id,
    model,
    requested_model,
    group_id,
    total_cost,
    actual_cost,
    rate_multiplier,
    billing_type,
    request_type,
    video_count,
    video_resolution,
    video_duration_seconds,
    billing_mode,
    inbound_endpoint,
    upstream_endpoint,
    created_at
)
SELECT
    user_id,
    api_key_id,
    account_id,
    job_id,
    model,
    model,
    group_id,
    amount,
    amount,
    1,
    0,
    1,
    1,
    resolution,
    duration,
    'video',
    '/v1/videos/generations',
    '/v1/videos/generations',
    created_at
FROM video_jobs
WHERE status = 'completed'
  AND settlement_status = 'captured'
ON CONFLICT (request_id, api_key_id) DO NOTHING;
