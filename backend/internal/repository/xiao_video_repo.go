package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type videoRepository struct{ db *sql.DB }

func NewVideoRepository(db *sql.DB) service.VideoRepository { return &videoRepository{db: db} }

func (r *videoRepository) CreateMedia(ctx context.Context, media *service.VideoMedia) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO video_media (media_id, upstream_media_id, account_id, user_id, api_key_id, upstream_url, media_type, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, media.MediaID, media.UpstreamMediaID, media.AccountID, media.UserID, media.APIKeyID, media.UpstreamURL, media.MediaType, media.ExpiresAt)
	return err
}

func (r *videoRepository) GetMediaForOwner(ctx context.Context, mediaID string, apiKeyID int64) (*service.VideoMedia, error) {
	var media service.VideoMedia
	err := r.db.QueryRowContext(ctx, `
		SELECT media_id, upstream_media_id, account_id, user_id, api_key_id, upstream_url, media_type, expires_at, created_at
		FROM video_media WHERE media_id=$1 AND api_key_id=$2 AND expires_at > NOW()
	`, mediaID, apiKeyID).Scan(&media.MediaID, &media.UpstreamMediaID, &media.AccountID, &media.UserID, &media.APIKeyID, &media.UpstreamURL, &media.MediaType, &media.ExpiresAt, &media.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrVideoResourceNotFound
	}
	return &media, err
}

func (r *videoRepository) ReserveJob(ctx context.Context, p service.VideoJobReservation) (*service.VideoJob, bool, error) {
	if p.PreauthorizationAmount < 0 {
		return nil, false, errors.New("video preauthorization amount must be non-negative")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	upstreamPlaceholder := "creating:" + p.JobID
	var idempotency any
	if p.IdempotencyKey != "" {
		idempotency = p.IdempotencyKey
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO video_jobs (
			job_id, upstream_job_id, account_id, user_id, api_key_id, group_id, idempotency_key,
			request_hash, model, resolution, duration, aspect_ratio, status, amount,
			currency, settlement_status, upstream_response
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'pending',$13,'USD','held','{}'::jsonb)
		ON CONFLICT DO NOTHING
	`, p.JobID, upstreamPlaceholder, p.AccountID, p.Owner.UserID, p.Owner.APIKeyID, p.Owner.GroupID, idempotency,
		p.RequestHash, p.Model, p.Resolution, p.Duration, p.AspectRatio, p.PreauthorizationAmount)
	if err != nil {
		return nil, false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, false, err
	}
	if rows == 1 {
		var newBalance float64
		err = tx.QueryRowContext(ctx, `
			UPDATE users
			SET balance=balance-$1,frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW()
			WHERE id=$2 AND deleted_at IS NULL AND balance >= $1
			RETURNING balance
		`, p.PreauthorizationAmount, p.Owner.UserID).Scan(&newBalance)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, service.ErrVideoInsufficientBalance
		}
		if err != nil {
			return nil, false, err
		}
		if err = tx.Commit(); err != nil {
			return nil, false, err
		}
		job, err := r.getJobByID(ctx, p.JobID)
		return job, true, err
	}
	if err = tx.Rollback(); err != nil {
		return nil, false, err
	}
	if idempotency == nil {
		return nil, false, errors.New("video job reservation conflict")
	}
	job, err := r.getJobByIdempotency(ctx, p.Owner.APIKeyID, p.IdempotencyKey)
	return job, false, err
}

func (r *videoRepository) MarkJobReservationRetryable(ctx context.Context, jobID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE video_jobs SET upstream_job_id=$2,updated_at=NOW()
		WHERE job_id=$1 AND upstream_job_id=$3 AND settlement_status='held'
	`, jobID, "retry:"+jobID, "creating:"+jobID)
	return err
}

func (r *videoRepository) ClaimJobReservationRetry(ctx context.Context, jobID string, staleBefore time.Time) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE video_jobs SET upstream_job_id=$2,updated_at=NOW()
		WHERE job_id=$1 AND settlement_status='held'
		  AND (upstream_job_id=$3 OR (upstream_job_id=$2 AND updated_at < $4))
	`, jobID, "creating:"+jobID, "retry:"+jobID, staleBefore)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (r *videoRepository) ReleaseJobReservation(ctx context.Context, jobID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var userID int64
	var amount float64
	var upstreamID, settlement string
	err = tx.QueryRowContext(ctx, `
		SELECT user_id,amount,upstream_job_id,settlement_status
		FROM video_jobs WHERE job_id=$1 FOR UPDATE
	`, jobID).Scan(&userID, &amount, &upstreamID, &settlement)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if settlement != "held" || (upstreamID != "creating:"+jobID && upstreamID != "retry:"+jobID) {
		return nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET balance=balance+$1,frozen_balance=COALESCE(frozen_balance,0)-$1,updated_at=NOW()
		WHERE id=$2 AND COALESCE(frozen_balance,0)>=$1
	`, amount, userID)
	if err != nil {
		return err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil {
		return rowsErr
	} else if rows != 1 {
		return errors.New("video frozen balance is insufficient")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM video_jobs WHERE job_id=$1`, jobID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *videoRepository) FinalizeJobAndReconcileHold(ctx context.Context, p service.VideoJobFinalization) (_ *service.VideoJob, err error) {
	if videoJobStatusRank(p.Status) < 0 {
		return nil, errors.New("invalid video job status")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var userID int64
	var currentUpstream string
	var preauthorizationAmount float64
	var currentSettlement string
	err = tx.QueryRowContext(ctx, `SELECT user_id,upstream_job_id,amount,settlement_status FROM video_jobs WHERE job_id=$1 FOR UPDATE`, p.JobID).
		Scan(&userID, &currentUpstream, &preauthorizationAmount, &currentSettlement)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrVideoResourceNotFound
	}
	if err != nil {
		return nil, err
	}
	if currentUpstream != "creating:"+p.JobID {
		return nil, errors.New("video job already finalized")
	}
	if currentSettlement != "held" {
		return nil, errors.New("video job preauthorization is not held")
	}
	if p.UpstreamAmount < 0 {
		return nil, errors.New("video upstream amount must be non-negative")
	}
	settlement := "held"
	var settledAt any
	var finishedAt any
	var balanceRefund float64
	var frozenReduction float64
	if p.Status == "completed" {
		frozenReduction = preauthorizationAmount
		settlement = "captured"
		settledAt = time.Now()
		finishedAt = settledAt
	} else if p.Status == "failed" || p.Status == "canceled" {
		balanceRefund = preauthorizationAmount
		frozenReduction = preauthorizationAmount
		settlement = "released"
		settledAt = time.Now()
		finishedAt = settledAt
	}
	if frozenReduction > 0 {
		result, execErr := tx.ExecContext(ctx, `
			UPDATE users
			SET balance=balance+$1,frozen_balance=COALESCE(frozen_balance,0)-$2,updated_at=NOW()
			WHERE id=$3 AND COALESCE(frozen_balance,0)>=$4
		`, balanceRefund, frozenReduction, userID, preauthorizationAmount)
		if execErr != nil {
			return nil, execErr
		}
		if rows, rowsErr := result.RowsAffected(); rowsErr != nil {
			return nil, rowsErr
		} else if rows != 1 {
			return nil, errors.New("video frozen balance is insufficient")
		}
	}
	raw := json.RawMessage(p.UpstreamResponse)
	if !json.Valid(raw) {
		raw = json.RawMessage(`{}`)
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE video_jobs SET
			upstream_job_id=$2,status=$3,upstream_amount=$4,upstream_currency=$5,settlement_status=$6,
			upstream_response=$7,updated_at=NOW(),settled_at=$8,finished_at=$9,
			resolution=COALESCE(NULLIF($10,''),resolution),
			duration=CASE WHEN $11 > 0 THEN $11 ELSE duration END,
			aspect_ratio=COALESCE(NULLIF($12,''),aspect_ratio)
		WHERE job_id=$1
	`, p.JobID, p.UpstreamJobID, p.Status, p.UpstreamAmount, p.UpstreamCurrency, settlement, raw, settledAt, finishedAt,
		p.Resolution, p.Duration, p.AspectRatio)
	if err != nil {
		return nil, err
	}
	if settlement == "captured" {
		if err = insertCapturedVideoUsageLog(ctx, tx, p.JobID); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.getJobByID(ctx, p.JobID)
}

func (r *videoRepository) GetJobForOwner(ctx context.Context, jobID string, apiKeyID int64) (*service.VideoJob, error) {
	return scanVideoJob(r.db.QueryRowContext(ctx, videoJobSelect+` WHERE job_id=$1 AND api_key_id=$2`, jobID, apiKeyID))
}

func (r *videoRepository) ListJobsForOwner(ctx context.Context, apiKeyID int64, limit int) ([]*service.VideoJob, error) {
	rows, err := r.db.QueryContext(ctx, videoJobSelect+` WHERE api_key_id=$1 ORDER BY created_at DESC LIMIT $2`, apiKeyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVideoJobs(rows)
}

func (r *videoRepository) ListActiveJobs(ctx context.Context, limit int) ([]*service.VideoJob, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, videoJobSelect+` WHERE status IN ('pending','running','settling') AND upstream_job_id NOT LIKE 'creating:%' AND upstream_job_id NOT LIKE 'retry:%' ORDER BY updated_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVideoJobs(rows)
}

func (r *videoRepository) UpdateJobAndSettle(ctx context.Context, p service.VideoJobUpdate) (_ *service.VideoJob, err error) {
	nextRank := videoJobStatusRank(p.Status)
	if nextRank < 0 {
		return nil, errors.New("invalid video job status")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	job, err := scanVideoJob(tx.QueryRowContext(ctx, videoJobSelect+` WHERE job_id=$1 FOR UPDATE`, p.JobID))
	if err != nil {
		return nil, err
	}
	currentRank := videoJobStatusRank(job.Status)
	if currentRank < 0 {
		return nil, errors.New("stored video job has invalid status")
	}
	// Polls and cancel requests can race. Once a terminal state wins, or a newer
	// non-terminal phase is stored, a stale response must not move the job back.
	if currentRank >= videoJobStatusRank("completed") || nextRank < currentRank {
		if err = tx.Rollback(); err != nil {
			return nil, err
		}
		return job, nil
	}
	settlement := job.SettlementStatus
	captured := false
	var settledAt any = job.SettledAt
	if settlement == "held" && p.Status == "completed" {
		res, execErr := tx.ExecContext(ctx, `UPDATE users SET frozen_balance=COALESCE(frozen_balance,0)-$1,updated_at=NOW() WHERE id=$2 AND COALESCE(frozen_balance,0)>=$1`, job.Amount, job.UserID)
		if execErr != nil {
			return nil, execErr
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nil, errors.New("video frozen balance is insufficient")
		}
		settlement = "captured"
		captured = true
		settledAt = time.Now()
	} else if settlement == "held" && (p.Status == "failed" || p.Status == "canceled") {
		res, execErr := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=COALESCE(frozen_balance,0)-$1,updated_at=NOW() WHERE id=$2 AND COALESCE(frozen_balance,0)>=$1`, job.Amount, job.UserID)
		if execErr != nil {
			return nil, execErr
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nil, errors.New("video frozen balance is insufficient")
		}
		settlement = "released"
		settledAt = time.Now()
	}
	raw := json.RawMessage(p.UpstreamResponse)
	if !json.Valid(raw) {
		raw = json.RawMessage(`{}`)
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE video_jobs SET
			status=$2,upstream_response=$3,updated_at=NOW(),finished_at=COALESCE($4,finished_at),
			settlement_status=$5,settled_at=$6,
			resolution=COALESCE(NULLIF($7,''),resolution),
			duration=CASE WHEN $8 > 0 THEN $8 ELSE duration END,
			aspect_ratio=COALESCE(NULLIF($9,''),aspect_ratio)
		WHERE job_id=$1
	`, p.JobID, p.Status, raw, p.FinishedAt, settlement, settledAt, p.Resolution, p.Duration, p.AspectRatio)
	if err != nil {
		return nil, err
	}
	if captured {
		if err = insertCapturedVideoUsageLog(ctx, tx, p.JobID); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.getJobByID(ctx, p.JobID)
}

func insertCapturedVideoUsageLog(ctx context.Context, tx *sql.Tx, jobID string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO usage_logs (
			user_id, api_key_id, account_id, request_id, model, requested_model, group_id,
			total_cost, actual_cost, rate_multiplier, billing_type, request_type,
			video_count, video_resolution, video_duration_seconds, billing_mode,
			inbound_endpoint, upstream_endpoint, created_at
		)
		SELECT
			user_id, api_key_id, account_id, job_id, model, model, group_id,
			amount, amount, 1, $2, $3,
			1, resolution, duration, 'video',
			'/v1/videos/generations', '/v1/videos/generations', created_at
		FROM video_jobs
		WHERE job_id=$1 AND status='completed' AND settlement_status='captured'
		ON CONFLICT (request_id, api_key_id) DO NOTHING
	`, jobID, service.BillingTypeBalance, service.RequestTypeSync)
	return err
}

const videoJobSelect = `SELECT job_id,upstream_job_id,account_id,user_id,api_key_id,group_id,idempotency_key,request_hash,model,resolution,duration,aspect_ratio,status,amount,currency,upstream_amount,upstream_currency,settlement_status,upstream_response,created_at,updated_at,finished_at,settled_at FROM video_jobs`

type videoRowScanner interface{ Scan(...any) error }

func scanVideoJob(row videoRowScanner) (*service.VideoJob, error) {
	var j service.VideoJob
	var group sql.NullInt64
	var idem sql.NullString
	var upstreamAmount sql.NullFloat64
	var upstreamCurrency sql.NullString
	var finished, settled sql.NullTime
	err := row.Scan(&j.JobID, &j.UpstreamJobID, &j.AccountID, &j.UserID, &j.APIKeyID, &group, &idem, &j.RequestHash, &j.Model, &j.Resolution, &j.Duration, &j.AspectRatio, &j.Status, &j.Amount, &j.Currency, &upstreamAmount, &upstreamCurrency, &j.SettlementStatus, &j.UpstreamResponse, &j.CreatedAt, &j.UpdatedAt, &finished, &settled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrVideoResourceNotFound
	}
	if err != nil {
		return nil, err
	}
	if group.Valid {
		j.GroupID = &group.Int64
	}
	if idem.Valid {
		j.IdempotencyKey = &idem.String
	}
	if upstreamAmount.Valid {
		j.UpstreamAmount = &upstreamAmount.Float64
	}
	if upstreamCurrency.Valid {
		j.UpstreamCurrency = upstreamCurrency.String
	}
	if finished.Valid {
		j.FinishedAt = &finished.Time
	}
	if settled.Valid {
		j.SettledAt = &settled.Time
	}
	return &j, nil
}
func scanVideoJobs(rows *sql.Rows) ([]*service.VideoJob, error) {
	out := []*service.VideoJob{}
	for rows.Next() {
		j, err := scanVideoJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (r *videoRepository) getJobByID(ctx context.Context, jobID string) (*service.VideoJob, error) {
	return scanVideoJob(r.db.QueryRowContext(ctx, videoJobSelect+` WHERE job_id=$1`, jobID))
}
func (r *videoRepository) getJobByIdempotency(ctx context.Context, apiKeyID int64, key string) (*service.VideoJob, error) {
	return scanVideoJob(r.db.QueryRowContext(ctx, videoJobSelect+` WHERE api_key_id=$1 AND idempotency_key=$2`, apiKeyID, key))
}

func videoJobStatusRank(status string) int {
	switch status {
	case "pending":
		return 0
	case "running":
		return 1
	case "settling":
		return 2
	case "completed", "failed", "canceled":
		return 3
	default:
		return -1
	}
}
