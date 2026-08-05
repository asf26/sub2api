//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestVideoRepository_BindsAccountAndSettlesHoldExactlyOnce(t *testing.T) {
	ctx := context.Background()
	suffix := uniqueTestValue(t, "video")

	var groupID, userID, apiKeyID, accountOneID, accountTwoID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`INSERT INTO groups (name) VALUES ($1) RETURNING id`, suffix+"-group").Scan(&groupID))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`INSERT INTO users (email,password_hash,balance,frozen_balance) VALUES ($1,'test',20,0) RETURNING id`, suffix+"@example.test").Scan(&userID))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`INSERT INTO api_keys (user_id,key,name,group_id) VALUES ($1,$2,$3,$4) RETURNING id`, userID, "sk-"+suffix, suffix+"-key", groupID).Scan(&apiKeyID))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`INSERT INTO accounts (name,platform,type,credentials,extra) VALUES ($1,'xiaoapi','apikey','{}','{}') RETURNING id`, suffix+"-account-1").Scan(&accountOneID))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`INSERT INTO accounts (name,platform,type,credentials,extra) VALUES ($1,'xiaoapi','apikey','{}','{}') RETURNING id`, suffix+"-account-2").Scan(&accountTwoID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM video_jobs WHERE api_key_id=$1`, apiKeyID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM video_media WHERE api_key_id=$1`, apiKeyID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM api_keys WHERE id=$1`, apiKeyID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id IN ($1,$2)`, accountOneID, accountTwoID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, groupID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	repo := NewVideoRepository(integrationDB)
	media := &service.VideoMedia{
		MediaID:         "vidmedia_repo_bound",
		UpstreamMediaID: "upstream-media",
		AccountID:       accountOneID,
		UserID:          userID,
		APIKeyID:        apiKeyID,
		UpstreamURL:     "https://upstream.example/media",
		MediaType:       "UPLOADED",
		ExpiresAt:       time.Now().Add(time.Hour),
	}
	require.NoError(t, repo.CreateMedia(ctx, media))
	storedMedia, err := repo.GetMediaForOwner(ctx, media.MediaID, apiKeyID)
	require.NoError(t, err)
	require.Equal(t, accountOneID, storedMedia.AccountID)
	_, err = repo.GetMediaForOwner(ctx, media.MediaID, apiKeyID+1)
	require.ErrorIs(t, err, service.ErrVideoResourceNotFound)

	owner := service.VideoOwner{UserID: userID, APIKeyID: apiKeyID, GroupID: &groupID}
	reserve := func(jobID string, accountID int64, idempotencyKey string) *service.VideoJob {
		t.Helper()
		job, created, reserveErr := repo.ReserveJob(ctx, service.VideoJobReservation{
			JobID:                  jobID,
			AccountID:              accountID,
			Owner:                  owner,
			IdempotencyKey:         idempotencyKey,
			RequestHash:            strings.Repeat("a", 64),
			Model:                  "video-public",
			Resolution:             "480p",
			Duration:               4,
			AspectRatio:            "16:9",
			PreauthorizationAmount: 10,
		})
		require.NoError(t, reserveErr)
		require.True(t, created)
		return job
	}

	first := reserve("vidjob_repo_one", accountOneID, "repo-one")
	second := reserve("vidjob_repo_two", accountTwoID, "repo-two")
	require.Equal(t, accountOneID, first.AccountID)
	require.Equal(t, accountTwoID, second.AccountID)
	assertVideoBalance(t, ctx, userID, 0, 20)

	first, err = repo.FinalizeJobAndReconcileHold(ctx, service.VideoJobFinalization{
		JobID: first.JobID, UpstreamJobID: "same-upstream-id", Status: "running", UpstreamAmount: 2, UpstreamCurrency: "USD",
		Resolution: "720p", Duration: 8, AspectRatio: "9:16", UpstreamResponse: []byte(`{"status":"running"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "720p", first.Resolution)
	require.Equal(t, 8, first.Duration)
	require.Equal(t, "9:16", first.AspectRatio)
	require.Equal(t, 10.0, first.Amount)
	require.NotNil(t, first.UpstreamAmount)
	require.Equal(t, 2.0, *first.UpstreamAmount)
	second, err = repo.FinalizeJobAndReconcileHold(ctx, service.VideoJobFinalization{
		JobID: second.JobID, UpstreamJobID: "same-upstream-id", Status: "running", UpstreamAmount: 3, UpstreamCurrency: "USD", UpstreamResponse: []byte(`{"status":"running"}`),
	})
	require.NoError(t, err, "upstream IDs are unique within an account, not globally")

	assertVideoBalance(t, ctx, userID, 0, 20)
	now := time.Now()
	first, err = repo.UpdateJobAndSettle(ctx, service.VideoJobUpdate{
		JobID: first.JobID, Status: "completed", Resolution: "1080p", Duration: 12, AspectRatio: "21:9",
		UpstreamResponse: []byte(`{"status":"completed"}`), FinishedAt: &now,
	})
	require.NoError(t, err)
	require.Equal(t, "captured", first.SettlementStatus)
	require.Equal(t, "1080p", first.Resolution)
	require.Equal(t, 12, first.Duration)
	require.Equal(t, "21:9", first.AspectRatio)
	assertVideoBalance(t, ctx, userID, 0, 10)
	assertCapturedVideoUsage(t, ctx, first, groupID)

	first, err = repo.UpdateJobAndSettle(ctx, service.VideoJobUpdate{JobID: first.JobID, Status: "running", UpstreamResponse: []byte(`{"status":"running"}`)})
	require.NoError(t, err)
	require.Equal(t, "completed", first.Status, "a stale poll must not regress a terminal job")
	assertVideoBalance(t, ctx, userID, 0, 10)
	assertCapturedVideoUsage(t, ctx, first, groupID)

	second, err = repo.UpdateJobAndSettle(ctx, service.VideoJobUpdate{JobID: second.JobID, Status: "failed", UpstreamResponse: []byte(`{"status":"failed"}`), FinishedAt: &now})
	require.NoError(t, err)
	require.Equal(t, "released", second.SettlementStatus)
	assertVideoBalance(t, ctx, userID, 10, 0)
	assertVideoUsageCount(t, ctx, second.JobID, apiKeyID, 0)

	second, err = repo.UpdateJobAndSettle(ctx, service.VideoJobUpdate{JobID: second.JobID, Status: "failed", UpstreamResponse: []byte(`{"status":"failed"}`), FinishedAt: &now})
	require.NoError(t, err)
	require.Equal(t, "released", second.SettlementStatus)
	assertVideoBalance(t, ctx, userID, 10, 0)
	assertVideoUsageCount(t, ctx, second.JobID, apiKeyID, 0)

	third := reserve("vidjob_repo_direct_complete", accountOneID, "repo-direct-complete")
	third, err = repo.FinalizeJobAndReconcileHold(ctx, service.VideoJobFinalization{
		JobID: third.JobID, UpstreamJobID: "direct-complete-upstream", Status: "completed",
		UpstreamAmount: 2.5, UpstreamCurrency: "USD", Resolution: "720p", Duration: 6,
		AspectRatio: "16:9", UpstreamResponse: []byte(`{"status":"completed"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "captured", third.SettlementStatus)
	assertVideoBalance(t, ctx, userID, 0, 0)
	assertCapturedVideoUsage(t, ctx, third, groupID)

	_, err = repo.UpdateJobAndSettle(ctx, service.VideoJobUpdate{JobID: second.JobID, Status: "unknown"})
	require.EqualError(t, err, "invalid video job status")
}

func assertVideoBalance(t *testing.T, ctx context.Context, userID int64, wantBalance, wantFrozen float64) {
	t.Helper()
	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,frozen_balance FROM users WHERE id=$1`, userID).Scan(&balance, &frozen))
	require.InDelta(t, wantBalance, balance, 0.00000001)
	require.InDelta(t, wantFrozen, frozen, 0.00000001)
}

func assertVideoUsageCount(t *testing.T, ctx context.Context, jobID string, apiKeyID int64, want int) {
	t.Helper()
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, jobID, apiKeyID).Scan(&count))
	require.Equal(t, want, count)
}

func assertCapturedVideoUsage(t *testing.T, ctx context.Context, job *service.VideoJob, groupID int64) {
	t.Helper()
	var userID, apiKeyID, accountID, storedGroupID int64
	var model, requestedModel, billingMode, resolution, inboundEndpoint, upstreamEndpoint string
	var videoCount, duration, billingType, requestType int
	var totalCost, actualCost, rateMultiplier float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT user_id,api_key_id,account_id,group_id,model,requested_model,billing_mode,
		       video_count,video_resolution,video_duration_seconds,total_cost,actual_cost,
		       rate_multiplier,billing_type,request_type,inbound_endpoint,upstream_endpoint
		FROM usage_logs WHERE request_id=$1 AND api_key_id=$2
	`, job.JobID, job.APIKeyID).Scan(
		&userID, &apiKeyID, &accountID, &storedGroupID, &model, &requestedModel, &billingMode,
		&videoCount, &resolution, &duration, &totalCost, &actualCost, &rateMultiplier,
		&billingType, &requestType, &inboundEndpoint, &upstreamEndpoint,
	))
	require.Equal(t, job.UserID, userID)
	require.Equal(t, job.APIKeyID, apiKeyID)
	require.Equal(t, job.AccountID, accountID)
	require.Equal(t, groupID, storedGroupID)
	require.Equal(t, job.Model, model)
	require.Equal(t, job.Model, requestedModel)
	require.Equal(t, "video", billingMode)
	require.Equal(t, 1, videoCount)
	require.Equal(t, job.Resolution, resolution)
	require.Equal(t, job.Duration, duration)
	require.InDelta(t, job.Amount, totalCost, 0.00000001)
	require.InDelta(t, job.Amount, actualCost, 0.00000001)
	require.InDelta(t, 1, rateMultiplier, 0.00000001)
	require.Equal(t, int(service.BillingTypeBalance), billingType)
	require.Equal(t, int(service.RequestTypeSync), requestType)
	require.Equal(t, "/v1/videos/generations", inboundEndpoint)
	require.Equal(t, "/v1/videos/generations", upstreamEndpoint)
}
