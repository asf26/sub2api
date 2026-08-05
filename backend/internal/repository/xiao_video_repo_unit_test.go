package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestVideoRepository_CreateMediaPersistsOwnershipAndAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	media := &service.VideoMedia{
		MediaID:         "vidmedia_test",
		UpstreamMediaID: "up-media",
		AccountID:       42,
		UserID:          11,
		APIKeyID:        22,
		UpstreamURL:     "https://upstream.example/media",
		MediaType:       "UPLOADED",
		ExpiresAt:       time.Now().Add(time.Hour),
	}
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO video_media (media_id, upstream_media_id, account_id, user_id, api_key_id, upstream_url, media_type, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`)).
		WithArgs(media.MediaID, media.UpstreamMediaID, media.AccountID, media.UserID, media.APIKeyID, media.UpstreamURL, media.MediaType, media.ExpiresAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewVideoRepository(db)
	require.NoError(t, repo.CreateMedia(context.Background(), media))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVideoRepository_ReserveJobHoldsBeforeReturning(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	groupID := int64(7)
	reservation := service.VideoJobReservation{
		JobID:                  "vidjob_reserve",
		AccountID:              42,
		Owner:                  service.VideoOwner{UserID: 11, APIKeyID: 22, GroupID: &groupID},
		IdempotencyKey:         "reserve-key",
		RequestHash:            "hash",
		Model:                  "video-public",
		Resolution:             "480p",
		Duration:               4,
		AspectRatio:            "16:9",
		PreauthorizationAmount: 10,
	}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO video_jobs").
		WithArgs(reservation.JobID, "creating:"+reservation.JobID, reservation.AccountID, reservation.Owner.UserID,
			reservation.Owner.APIKeyID, reservation.Owner.GroupID, reservation.IdempotencyKey, reservation.RequestHash,
			reservation.Model, reservation.Resolution, reservation.Duration, reservation.AspectRatio, reservation.PreauthorizationAmount).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("UPDATE users").
		WithArgs(reservation.PreauthorizationAmount, reservation.Owner.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(10.0))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(videoJobSelect + ` WHERE job_id=$1`)).
		WithArgs(reservation.JobID).
		WillReturnRows(videoJobRows(reservation.JobID, "creating:"+reservation.JobID, "pending", 10, "held", now))

	repo := NewVideoRepository(db)
	job, created, err := repo.ReserveJob(context.Background(), reservation)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, 10.0, job.Amount)
	require.Equal(t, "held", job.SettlementStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVideoRepository_ReserveJobInsufficientBalanceRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	reservation := service.VideoJobReservation{
		JobID:                  "vidjob_no_balance",
		AccountID:              42,
		Owner:                  service.VideoOwner{UserID: 11, APIKeyID: 22},
		RequestHash:            "hash",
		Model:                  "video-public",
		Resolution:             "480p",
		Duration:               4,
		AspectRatio:            "16:9",
		PreauthorizationAmount: 10,
	}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO video_jobs").
		WithArgs(reservation.JobID, "creating:"+reservation.JobID, reservation.AccountID, reservation.Owner.UserID,
			reservation.Owner.APIKeyID, reservation.Owner.GroupID, nil, reservation.RequestHash, reservation.Model,
			reservation.Resolution, reservation.Duration, reservation.AspectRatio, reservation.PreauthorizationAmount).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("UPDATE users").
		WithArgs(reservation.PreauthorizationAmount, reservation.Owner.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}))
	mock.ExpectRollback()

	repo := NewVideoRepository(db)
	_, created, err := repo.ReserveJob(context.Background(), reservation)
	require.ErrorIs(t, err, service.ErrVideoInsufficientBalance)
	require.False(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVideoRepository_FinalizeJobPreservesSellingPriceAndStoresUpstreamCost(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT user_id,upstream_job_id,amount,settlement_status FROM video_jobs WHERE job_id=$1 FOR UPDATE`)).
		WithArgs("vidjob_reconcile").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "upstream_job_id", "amount", "settlement_status"}).
			AddRow(int64(11), "creating:vidjob_reconcile", 10.0, "held"))
	mock.ExpectExec("UPDATE video_jobs SET").
		WithArgs("vidjob_reconcile", "up-job", "running", 2.0, "USD", "held", []byte(`{"status":"running"}`), nil, nil, "720p", 8, "16:9").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(videoJobSelect + ` WHERE job_id=$1`)).
		WithArgs("vidjob_reconcile").
		WillReturnRows(videoJobRows("vidjob_reconcile", "up-job", "running", 10, "held", now, 2.0, "USD"))

	repo := NewVideoRepository(db)
	job, err := repo.FinalizeJobAndReconcileHold(context.Background(), service.VideoJobFinalization{
		JobID: "vidjob_reconcile", UpstreamJobID: "up-job", Status: "running", UpstreamAmount: 2, UpstreamCurrency: "USD",
		Resolution: "720p", Duration: 8, AspectRatio: "16:9", UpstreamResponse: []byte(`{"status":"running"}`),
	})
	require.NoError(t, err)
	require.Equal(t, 10.0, job.Amount)
	require.NotNil(t, job.UpstreamAmount)
	require.Equal(t, 2.0, *job.UpstreamAmount)
	require.Equal(t, "USD", job.UpstreamCurrency)
	require.Equal(t, "held", job.SettlementStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVideoRepository_FinalizeCompletedJobCapturesAndRecordsUsage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT user_id,upstream_job_id,amount,settlement_status FROM video_jobs WHERE job_id=$1 FOR UPDATE`)).
		WithArgs("vidjob_direct_complete").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "upstream_job_id", "amount", "settlement_status"}).
			AddRow(int64(11), "creating:vidjob_direct_complete", 10.0, "held"))
	mock.ExpectExec("UPDATE users").
		WithArgs(0.0, 10.0, int64(11), 10.0).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE video_jobs SET").
		WithArgs("vidjob_direct_complete", "up-job", "completed", 2.0, "USD", "captured", []byte(`{"status":"completed"}`), sqlmock.AnyArg(), sqlmock.AnyArg(), "", 0, "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO usage_logs").
		WithArgs("vidjob_direct_complete", service.BillingTypeBalance, service.RequestTypeSync).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(videoJobSelect + ` WHERE job_id=$1`)).
		WithArgs("vidjob_direct_complete").
		WillReturnRows(videoJobRows("vidjob_direct_complete", "up-job", "completed", 10, "captured", now, 2.0, "USD"))

	repo := NewVideoRepository(db)
	job, err := repo.FinalizeJobAndReconcileHold(context.Background(), service.VideoJobFinalization{
		JobID: "vidjob_direct_complete", UpstreamJobID: "up-job", Status: "completed",
		UpstreamAmount: 2, UpstreamCurrency: "USD", UpstreamResponse: []byte(`{"status":"completed"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "completed", job.Status)
	require.Equal(t, "captured", job.SettlementStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVideoRepository_ReleaseJobReservationRefundsAndDeletes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id,amount,upstream_job_id,settlement_status").
		WithArgs("vidjob_release").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "amount", "upstream_job_id", "settlement_status"}).
			AddRow(int64(11), 10.0, "creating:vidjob_release", "held"))
	mock.ExpectExec("UPDATE users").
		WithArgs(10.0, int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM video_jobs WHERE job_id=$1`)).
		WithArgs("vidjob_release").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewVideoRepository(db)
	require.NoError(t, repo.ReleaseJobReservation(context.Background(), "vidjob_release"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVideoRepository_UpdateJobAndSettleDoesNotRegressTerminalState(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(videoJobSelect + ` WHERE job_id=$1 FOR UPDATE`)).
		WithArgs("vidjob_terminal").
		WillReturnRows(videoJobRows("vidjob_terminal", "up-job", "completed", 2, "captured", now))
	mock.ExpectRollback()

	repo := NewVideoRepository(db)
	job, err := repo.UpdateJobAndSettle(context.Background(), service.VideoJobUpdate{
		JobID: "vidjob_terminal", Status: "running", UpstreamResponse: []byte(`{"status":"running"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "completed", job.Status)
	require.Equal(t, "captured", job.SettlementStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVideoRepository_UpdateJobAndSettleCapturesHeldAmount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(videoJobSelect + ` WHERE job_id=$1 FOR UPDATE`)).
		WithArgs("vidjob_capture").
		WillReturnRows(videoJobRows("vidjob_capture", "up-job", "running", 2, "held", now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET frozen_balance=COALESCE(frozen_balance,0)-$1,updated_at=NOW() WHERE id=$2 AND COALESCE(frozen_balance,0)>=$1`)).
		WithArgs(2.0, int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE video_jobs SET").
		WithArgs("vidjob_capture", "completed", []byte(`{"status":"completed"}`), sqlmock.AnyArg(), "captured", sqlmock.AnyArg(), "1080p", 12, "21:9").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO usage_logs").
		WithArgs("vidjob_capture", service.BillingTypeBalance, service.RequestTypeSync).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(videoJobSelect + ` WHERE job_id=$1`)).
		WithArgs("vidjob_capture").
		WillReturnRows(videoJobRows("vidjob_capture", "up-job", "completed", 2, "captured", now))

	repo := NewVideoRepository(db)
	finished := now
	job, err := repo.UpdateJobAndSettle(context.Background(), service.VideoJobUpdate{
		JobID: "vidjob_capture", Status: "completed", Resolution: "1080p", Duration: 12, AspectRatio: "21:9",
		UpstreamResponse: []byte(`{"status":"completed"}`), FinishedAt: &finished,
	})
	require.NoError(t, err)
	require.Equal(t, "captured", job.SettlementStatus)
	require.NoError(t, mock.ExpectationsWereMet())
}

func videoJobRows(jobID, upstreamID, status string, amount float64, settlement string, now time.Time, upstream ...any) *sqlmock.Rows {
	var upstreamAmount any
	var upstreamCurrency any
	if len(upstream) > 0 {
		upstreamAmount = upstream[0]
	}
	if len(upstream) > 1 {
		upstreamCurrency = upstream[1]
	}
	return sqlmock.NewRows([]string{
		"job_id", "upstream_job_id", "account_id", "user_id", "api_key_id", "group_id", "idempotency_key", "request_hash",
		"model", "resolution", "duration", "aspect_ratio", "status", "amount", "currency", "upstream_amount", "upstream_currency", "settlement_status", "upstream_response",
		"created_at", "updated_at", "finished_at", "settled_at",
	}).AddRow(jobID, upstreamID, int64(42), int64(11), int64(22), int64(7), nil, "hash", "video-public", "480p", 4, "16:9", status, amount, "USD", upstreamAmount, upstreamCurrency, settlement, []byte(`{}`), now, now, nil, nil)
}
