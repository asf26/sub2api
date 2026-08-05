package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXiaoVideoUpstreamPricingUsesForwardMigration(t *testing.T) {
	original, err := FS.ReadFile("210_xiao_video_api.sql")
	require.NoError(t, err)
	require.NotContains(t, string(original), "upstream_amount")
	require.NotContains(t, string(original), "upstream_currency")

	followup, err := FS.ReadFile("211_xiao_video_upstream_pricing.sql")
	require.NoError(t, err)
	sql := string(followup)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS upstream_amount NUMERIC(20, 8)")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS upstream_currency VARCHAR(16)")
	require.Contains(t, sql, "video_jobs_upstream_amount_nonnegative")
}

func TestXiaoVideoUsageLogBackfillOnlyIncludesCapturedCompletedJobs(t *testing.T) {
	contents, err := FS.ReadFile("212_xiao_video_usage_logs.sql")
	require.NoError(t, err)

	sql := string(contents)
	require.Contains(t, sql, "INSERT INTO usage_logs")
	require.Contains(t, sql, "status = 'completed'")
	require.Contains(t, sql, "settlement_status = 'captured'")
	require.Contains(t, sql, "ON CONFLICT (request_id, api_key_id) DO NOTHING")
	for _, field := range []string{"billing_mode", "video_count", "video_resolution", "video_duration_seconds"} {
		require.Contains(t, sql, field)
	}
}
