package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type videoAccountRepoStub struct {
	AccountRepository
	accounts []Account
}

func (r *videoAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			account := r.accounts[i]
			return &account, nil
		}
	}
	return nil, errors.New("account not found")
}

func (r *videoAccountRepoStub) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, platforms []string, _ bool) ([]Account, error) {
	result := make([]Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if len(platforms) > 0 && account.Platform != platforms[0] {
			continue
		}
		if !openAIStickyAccountMatchesGroup(&account, groupID) {
			continue
		}
		result = append(result, account)
	}
	return result, nil
}

func (r *videoAccountRepoStub) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	return r.ListSchedulableByGroupIDAndPlatforms(ctx, groupID, []string{platform})
}

func (r *videoAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	candidates, err := r.ListModelAvailabilityCandidates(ctx, &groupID, platforms, true)
	if err != nil {
		return nil, err
	}
	result := make([]Account, 0, len(candidates))
	for _, account := range candidates {
		if account.IsSchedulable() {
			result = append(result, account)
		}
	}
	return result, nil
}

type videoHTTPUpstreamStub struct {
	do func(*http.Request, string, int64, int) (*http.Response, error)
}

func (s *videoHTTPUpstreamStub) Do(req *http.Request, proxyURL string, accountID int64, concurrency int) (*http.Response, error) {
	return s.do(req, proxyURL, accountID, concurrency)
}

func (s *videoHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, concurrency)
}

type videoRepositoryStub struct {
	media       map[string]*VideoMedia
	jobs        map[string]*VideoJob
	idempotency map[string]*VideoJob
	balance     float64
	frozen      float64
}

func newVideoRepositoryStub() *videoRepositoryStub {
	return &videoRepositoryStub{
		media:       make(map[string]*VideoMedia),
		jobs:        make(map[string]*VideoJob),
		idempotency: make(map[string]*VideoJob),
		balance:     100,
	}
}

func (r *videoRepositoryStub) CreateMedia(_ context.Context, media *VideoMedia) error {
	copy := *media
	r.media[media.MediaID] = &copy
	return nil
}

func (r *videoRepositoryStub) GetMediaForOwner(_ context.Context, mediaID string, apiKeyID int64) (*VideoMedia, error) {
	media := r.media[mediaID]
	if media == nil || media.APIKeyID != apiKeyID || !media.ExpiresAt.After(time.Now()) {
		return nil, ErrVideoResourceNotFound
	}
	copy := *media
	return &copy, nil
}

func (r *videoRepositoryStub) ReserveJob(_ context.Context, reservation VideoJobReservation) (*VideoJob, bool, error) {
	key := reservation.IdempotencyKey
	scopedKey := strconv.FormatInt(reservation.Owner.APIKeyID, 10) + ":" + key
	if key != "" {
		if existing := r.idempotency[scopedKey]; existing != nil {
			copy := *existing
			return &copy, false, nil
		}
	}
	if r.balance < reservation.PreauthorizationAmount {
		return nil, false, ErrVideoInsufficientBalance
	}
	r.balance -= reservation.PreauthorizationAmount
	r.frozen += reservation.PreauthorizationAmount
	job := &VideoJob{
		JobID:            reservation.JobID,
		UpstreamJobID:    videoCreatingUpstreamPrefix + reservation.JobID,
		AccountID:        reservation.AccountID,
		UserID:           reservation.Owner.UserID,
		APIKeyID:         reservation.Owner.APIKeyID,
		GroupID:          reservation.Owner.GroupID,
		RequestHash:      reservation.RequestHash,
		Model:            reservation.Model,
		Resolution:       reservation.Resolution,
		Duration:         reservation.Duration,
		AspectRatio:      reservation.AspectRatio,
		Status:           "pending",
		Amount:           reservation.PreauthorizationAmount,
		Currency:         "USD",
		SettlementStatus: "held",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if key != "" {
		job.IdempotencyKey = &key
		r.idempotency[scopedKey] = job
	}
	r.jobs[job.JobID] = job
	copy := *job
	return &copy, true, nil
}

func (r *videoRepositoryStub) ReleaseJobReservation(_ context.Context, jobID string) error {
	job := r.jobs[jobID]
	if job != nil && job.SettlementStatus == "held" &&
		(strings.HasPrefix(job.UpstreamJobID, videoCreatingUpstreamPrefix) || strings.HasPrefix(job.UpstreamJobID, videoRetryableUpstreamPrefix)) {
		r.balance += job.Amount
		r.frozen -= job.Amount
		delete(r.jobs, jobID)
		if job.IdempotencyKey != nil {
			delete(r.idempotency, strconv.FormatInt(job.APIKeyID, 10)+":"+*job.IdempotencyKey)
		}
	}
	return nil
}

func (r *videoRepositoryStub) MarkJobReservationRetryable(_ context.Context, jobID string) error {
	job := r.jobs[jobID]
	if job != nil && strings.HasPrefix(job.UpstreamJobID, videoCreatingUpstreamPrefix) {
		job.UpstreamJobID = videoRetryableUpstreamPrefix + jobID
		job.UpdatedAt = time.Now()
	}
	return nil
}

func (r *videoRepositoryStub) ClaimJobReservationRetry(_ context.Context, jobID string, staleBefore time.Time) (bool, error) {
	job := r.jobs[jobID]
	if job == nil {
		return false, nil
	}
	if strings.HasPrefix(job.UpstreamJobID, videoRetryableUpstreamPrefix) ||
		(strings.HasPrefix(job.UpstreamJobID, videoCreatingUpstreamPrefix) && job.UpdatedAt.Before(staleBefore)) {
		job.UpstreamJobID = videoCreatingUpstreamPrefix + jobID
		job.UpdatedAt = time.Now()
		return true, nil
	}
	return false, nil
}

func (r *videoRepositoryStub) FinalizeJobAndReconcileHold(_ context.Context, finalization VideoJobFinalization) (*VideoJob, error) {
	job := r.jobs[finalization.JobID]
	if job == nil {
		return nil, ErrVideoResourceNotFound
	}
	job.UpstreamJobID = finalization.UpstreamJobID
	job.Status = finalization.Status
	upstreamAmount := finalization.UpstreamAmount
	job.UpstreamAmount = &upstreamAmount
	job.UpstreamCurrency = finalization.UpstreamCurrency
	if finalization.Resolution != "" {
		job.Resolution = finalization.Resolution
	}
	if finalization.Duration > 0 {
		job.Duration = finalization.Duration
	}
	if finalization.AspectRatio != "" {
		job.AspectRatio = finalization.AspectRatio
	}
	job.UpstreamResponse = append([]byte(nil), finalization.UpstreamResponse...)
	job.SettlementStatus = "held"
	if finalization.Status == "completed" {
		r.frozen -= job.Amount
		job.SettlementStatus = "captured"
	} else if finalization.Status == "failed" || finalization.Status == "canceled" {
		r.balance += job.Amount
		r.frozen -= job.Amount
		job.SettlementStatus = "released"
	}
	job.UpdatedAt = time.Now()
	copy := *job
	return &copy, nil
}

func (r *videoRepositoryStub) GetJobForOwner(_ context.Context, jobID string, apiKeyID int64) (*VideoJob, error) {
	job := r.jobs[jobID]
	if job == nil || job.APIKeyID != apiKeyID {
		return nil, ErrVideoResourceNotFound
	}
	copy := *job
	return &copy, nil
}

func (r *videoRepositoryStub) ListJobsForOwner(_ context.Context, apiKeyID int64, limit int) ([]*VideoJob, error) {
	result := make([]*VideoJob, 0)
	for _, job := range r.jobs {
		if job.APIKeyID == apiKeyID && len(result) < limit {
			copy := *job
			result = append(result, &copy)
		}
	}
	return result, nil
}

func (r *videoRepositoryStub) ListActiveJobs(_ context.Context, _ int) ([]*VideoJob, error) {
	return nil, nil
}

func (r *videoRepositoryStub) UpdateJobAndSettle(_ context.Context, update VideoJobUpdate) (*VideoJob, error) {
	job := r.jobs[update.JobID]
	if job == nil {
		return nil, ErrVideoResourceNotFound
	}
	job.Status = update.Status
	if update.Resolution != "" {
		job.Resolution = update.Resolution
	}
	if update.Duration > 0 {
		job.Duration = update.Duration
	}
	if update.AspectRatio != "" {
		job.AspectRatio = update.AspectRatio
	}
	job.UpstreamResponse = append([]byte(nil), update.UpstreamResponse...)
	job.FinishedAt = update.FinishedAt
	job.UpdatedAt = time.Now()
	if job.SettlementStatus == "held" && update.Status == "completed" {
		job.SettlementStatus = "captured"
	} else if job.SettlementStatus == "held" && (update.Status == "failed" || update.Status == "canceled") {
		job.SettlementStatus = "released"
	}
	copy := *job
	return &copy, nil
}

func videoTestAccount(id, groupID int64, baseURL string) Account {
	return Account{
		ID:          id,
		Name:        "video-upstream",
		Platform:    PlatformXiaoAPI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 2,
		GroupIDs:    []int64{groupID},
		Credentials: map[string]any{
			"base_url":      baseURL,
			"api_key":       "upstream-secret",
			"model_mapping": map[string]any{"video-public": "video-upstream-v2"},
			XiaoVideoPricingCredentialKey: []any{
				map[string]any{
					"model":                  "video-public",
					"resolution":             "720p",
					"price_per_second":       0.75,
					"audio_price_per_second": 0.25,
					"default_resolution":     true,
					"default_duration":       20,
				},
			},
		},
	}
}

func seedanceVideoTestAccount(id, groupID int64, model string) Account {
	account := videoTestAccount(id, groupID, "https://upstream.example.test/v1")
	account.Credentials["model_mapping"] = map[string]any{model: model}
	account.Credentials[XiaoVideoPricingCredentialKey] = []any{
		map[string]any{
			"model":                  model,
			"resolution":             "480p",
			"price_per_second":       0.75,
			"audio_price_per_second": 0.25,
			"default_resolution":     true,
			"default_duration":       4,
		},
	}
	return account
}

func newVideoServiceForTest(repo VideoRepository, accountRepo *videoAccountRepoStub, upstream HTTPUpstream) *XiaoVideoService {
	cfg := &config.Config{VideoAPI: config.VideoAPIConfig{Enabled: true, PublicBaseURL: "https://video.example.test"}}
	gateway := &OpenAIGatewayService{accountRepo: accountRepo}
	return NewXiaoVideoService(repo, accountRepo, gateway, upstream, nil, nil, cfg)
}

func TestXiaoVideoAccountUsesIndependentPlatform(t *testing.T) {
	xiao := videoTestAccount(42, 7, "https://upstream.example/v1")
	require.True(t, xiao.IsXiaoAPI())

	xiao.Platform = PlatformOpenAI
	require.False(t, xiao.IsXiaoAPI())
}

func TestXiaoVideoSeedanceAudioCapabilityMatrix(t *testing.T) {
	for _, model := range []string{"seedance-2.0", "seedance-2.0-fast", "seedance-2.0-mini"} {
		require.True(t, seedanceSupportsGeneratedAudio(model), model)
	}
	require.False(t, seedanceSupportsGeneratedAudio("other-video-model"))
}

func TestXiaoVideoNormalizesAndValidatesAudio(t *testing.T) {
	svc := newVideoServiceForTest(newVideoRepositoryStub(), &videoAccountRepoStub{}, nil)
	owner := VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(7)}

	decode := func(body string) (map[string]any, string, error) {
		rewritten, _, hash, _, err := svc.rewriteGenerationRequest(context.Background(), owner, []byte(body))
		if err != nil {
			return nil, "", err
		}
		var request map[string]any
		require.NoError(t, json.Unmarshal(rewritten, &request))
		return request, hash, nil
	}

	omitted, omittedHash, err := decode(`{"model":"seedance-2.0","prompt":"waves"}`)
	require.NoError(t, err)
	require.Equal(t, false, omitted["audio"])

	explicitFalse, falseHash, err := decode(`{"model":"seedance-2.0","prompt":"waves","audio":false}`)
	require.NoError(t, err)
	require.Equal(t, false, explicitFalse["audio"])
	require.Equal(t, omittedHash, falseHash)

	explicitTrue, trueHash, err := decode(`{"model":"seedance-2.0","prompt":"waves","audio":true}`)
	require.NoError(t, err)
	require.Equal(t, true, explicitTrue["audio"])
	require.NotEqual(t, falseHash, trueHash)

	for _, invalid := range []string{
		`{"model":"seedance-2.0","prompt":"waves","audio":"true"}`,
		`{"model":"seedance-2.0","prompt":"waves","audio":1}`,
		`{"model":"seedance-2.0","prompt":"waves","audio":null}`,
	} {
		_, _, err := decode(invalid)
		require.ErrorIs(t, err, ErrVideoRequestInvalid)
	}
}

func TestXiaoVideoSeedanceAudioReachesUpstream(t *testing.T) {
	const groupID int64 = 7
	models := []string{"seedance-2.0", "seedance-2.0-fast", "seedance-2.0-mini"}
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			repo := newVideoRepositoryStub()
			accounts := &videoAccountRepoStub{accounts: []Account{seedanceVideoTestAccount(42, groupID, model)}}
			var captured map[string]any
			upstream := &videoHTTPUpstreamStub{do: func(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
				require.NoError(t, json.NewDecoder(req.Body).Decode(&captured))
				return &http.Response{
					StatusCode: http.StatusAccepted,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"job_id":"up-audio-job","status":"pending","amount":"1","currency":"USD"}`)),
				}, nil
			}}
			svc := newVideoServiceForTest(repo, accounts, upstream)

			job, err := svc.Create(
				context.Background(),
				VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)},
				[]byte(`{"model":"`+model+`","prompt":"Ocean waves","resolution":"480p","duration":4,"aspect_ratio":"16:9","audio":true}`),
				"seedance-audio-"+model,
			)
			require.NoError(t, err)
			require.Equal(t, model, captured["model"])
			require.Equal(t, true, captured["audio"])
			require.InDelta(t, 4.0, job.Amount, 0.00000001)
			require.NotEmpty(t, job.RequestHash)
		})
	}
}

func TestXiaoVideoCreateBindsMediaAccountAndMapsModel(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	repo.media["vidmedia_local"] = &VideoMedia{
		MediaID:         "vidmedia_local",
		UpstreamMediaID: "up-media",
		AccountID:       42,
		UserID:          11,
		APIKeyID:        22,
		UpstreamURL:     "https://upstream.example.test/media/up-media/content",
		ExpiresAt:       time.Now().Add(time.Hour),
	}
	accounts := &videoAccountRepoStub{accounts: []Account{videoTestAccount(42, groupID, "https://upstream.example.test/v1")}}
	var captured map[string]any
	var capturedAccountID int64
	upstream := &videoHTTPUpstreamStub{do: func(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
		capturedAccountID = accountID
		require.Equal(t, "/v1/videos/generations", req.URL.Path)
		require.Equal(t, "Bearer upstream-secret", req.Header.Get("Authorization"))
		require.Equal(t, "respond-async", req.Header.Get("Prefer"))
		require.NotEmpty(t, req.Header.Get("Idempotency-Key"))
		require.NoError(t, json.NewDecoder(req.Body).Decode(&captured))
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"job_id":"up-job-secret","status":"pending","amount":"2.00000000","currency":"USD"}`)),
		}, nil
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)
	job, err := svc.Create(context.Background(), VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)}, []byte(`{"model":"video-public","prompt":"make a video","duration":4,"start_frame_url":"https://video.example.test/v1/videos/uploads/vidmedia_local/content"}`), "order-1")
	require.NoError(t, err)
	require.Equal(t, int64(42), capturedAccountID)
	require.Equal(t, "video-upstream-v2", captured["model"])
	require.Equal(t, "https://upstream.example.test/media/up-media/content", captured["start_frame_url"])
	require.Equal(t, int64(42), job.AccountID)
	require.Equal(t, 3.0, job.Amount)
	require.NotNil(t, job.UpstreamAmount)
	require.Equal(t, 2.0, *job.UpstreamAmount)
	require.InDelta(t, 97.0, repo.balance, 0.00000001)
	require.InDelta(t, 3.0, repo.frozen, 0.00000001)
	require.NotContains(t, string(job.UpstreamResponse), "video.example.test")
}

func TestXiaoVideoCreatePersistsResolvedUpstreamDefaults(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	accounts := &videoAccountRepoStub{accounts: []Account{videoTestAccount(42, groupID, "https://upstream.example.test/v1")}}
	upstream := &videoHTTPUpstreamStub{do: func(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
		require.Equal(t, int64(42), accountID)
		require.Equal(t, http.MethodPost, req.Method)
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"job_id":"up-job-defaults",
				"status":"pending",
				"amount":"2.00000000",
				"currency":"USD",
				"resolution":"720p",
				"duration":8,
				"aspect_ratio":"16:9"
			}`)),
		}, nil
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)

	job, err := svc.Create(
		context.Background(),
		VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)},
		[]byte(`{"model":"video-public","prompt":"use provider defaults"}`),
		"defaults-1",
	)
	require.NoError(t, err)
	require.Equal(t, "video-public", job.Model)
	require.Equal(t, "720p", job.Resolution)
	require.Equal(t, 8, job.Duration)
	require.Equal(t, "16:9", job.AspectRatio)
}

func TestXiaoVideoGetRefreshesResolvedUpstreamMetadata(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	repo.jobs["vidjob_defaults"] = &VideoJob{
		JobID:            "vidjob_defaults",
		UpstreamJobID:    "up-job-defaults",
		AccountID:        42,
		UserID:           11,
		APIKeyID:         22,
		Model:            "video-public",
		Status:           "pending",
		Amount:           2,
		Currency:         "USD",
		SettlementStatus: "held",
	}
	accounts := &videoAccountRepoStub{accounts: []Account{videoTestAccount(42, groupID, "https://upstream.example.test/v1")}}
	upstream := &videoHTTPUpstreamStub{do: func(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
		require.Equal(t, int64(42), accountID)
		require.Equal(t, http.MethodGet, req.Method)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"job_id":"up-job-defaults",
				"status":"running",
				"resolution":"480p",
				"duration":4,
				"aspect_ratio":"9:16"
			}`)),
		}, nil
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)

	job, err := svc.Get(
		context.Background(),
		VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)},
		"vidjob_defaults",
	)
	require.NoError(t, err)
	require.Equal(t, "video-public", job.Model)
	require.Equal(t, "480p", job.Resolution)
	require.Equal(t, 4, job.Duration)
	require.Equal(t, "9:16", job.AspectRatio)
}

func TestXiaoVideoCreateRejectsInsufficientBalanceBeforeCallingUpstream(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	repo.balance = 14.99
	repo.media["vidmedia_balance"] = &VideoMedia{MediaID: "vidmedia_balance", AccountID: 42, UserID: 11, APIKeyID: 22, UpstreamURL: "https://upstream.example/media/balance", ExpiresAt: time.Now().Add(time.Hour)}
	accounts := &videoAccountRepoStub{accounts: []Account{videoTestAccount(42, groupID, "https://upstream.example/v1")}}
	requests := 0
	upstream := &videoHTTPUpstreamStub{do: func(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		requests++
		return nil, errors.New("upstream must not be called")
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)

	_, err := svc.Create(context.Background(), VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)}, []byte(`{"model":"video-public","prompt":"no balance","start_frame_url":"https://video.example.test/v1/videos/uploads/vidmedia_balance/content"}`), "insufficient-balance")
	require.ErrorIs(t, err, ErrVideoInsufficientBalance)
	require.Zero(t, requests)
	require.Empty(t, repo.jobs)
	require.InDelta(t, 14.99, repo.balance, 0.00000001)
	require.Zero(t, repo.frozen)
}

func TestXiaoVideoCreateRejectsMissingPreauthorizationBeforeCallingUpstream(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	repo.media["vidmedia_pricing"] = &VideoMedia{MediaID: "vidmedia_pricing", AccountID: 42, UserID: 11, APIKeyID: 22, UpstreamURL: "https://upstream.example/media/pricing", ExpiresAt: time.Now().Add(time.Hour)}
	account := videoTestAccount(42, groupID, "https://upstream.example/v1")
	delete(account.Credentials, XiaoVideoPricingCredentialKey)
	accounts := &videoAccountRepoStub{accounts: []Account{account}}
	requests := 0
	upstream := &videoHTTPUpstreamStub{do: func(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		requests++
		return nil, errors.New("upstream must not be called")
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)

	_, err := svc.Create(context.Background(), VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)}, []byte(`{"model":"video-public","prompt":"missing pricing","start_frame_url":"https://video.example.test/v1/videos/uploads/vidmedia_pricing/content"}`), "missing-pricing")
	require.ErrorIs(t, err, ErrVideoPricingUnavailable)
	require.Zero(t, requests)
	require.Empty(t, repo.jobs)
}

func TestXiaoVideoCreateKeepsSellingPriceWhenUpstreamCostIsHigher(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	repo.media["vidmedia_ceiling"] = &VideoMedia{MediaID: "vidmedia_ceiling", AccountID: 42, UserID: 11, APIKeyID: 22, UpstreamURL: "https://upstream.example/media/ceiling", ExpiresAt: time.Now().Add(time.Hour)}
	accounts := &videoAccountRepoStub{accounts: []Account{videoTestAccount(42, groupID, "https://upstream.example/v1")}}
	posts := 0
	deletes := 0
	upstream := &videoHTTPUpstreamStub{do: func(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		switch req.Method {
		case http.MethodPost:
			posts++
			return &http.Response{StatusCode: http.StatusAccepted, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"job_id":"up-too-expensive","status":"pending","amount":"11"}`))}, nil
		case http.MethodDelete:
			deletes++
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		default:
			return nil, errors.New("unexpected request")
		}
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)

	job, err := svc.Create(context.Background(), VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)}, []byte(`{"model":"video-public","prompt":"provider cost is independent","duration":4,"start_frame_url":"https://video.example.test/v1/videos/uploads/vidmedia_ceiling/content"}`), "cost-isolation")
	require.NoError(t, err)
	require.Equal(t, 1, posts)
	require.Zero(t, deletes)
	require.Equal(t, 3.0, job.Amount)
	require.NotNil(t, job.UpstreamAmount)
	require.Equal(t, 11.0, *job.UpstreamAmount)
	require.InDelta(t, 97.0, repo.balance, 0.00000001)
	require.InDelta(t, 3.0, repo.frozen, 0.00000001)
}

func TestXiaoVideoCreateRejectsCrossKeyMediaAndIdempotencyConflict(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	repo.media["vidmedia_local"] = &VideoMedia{MediaID: "vidmedia_local", AccountID: 42, UserID: 11, APIKeyID: 22, UpstreamURL: "https://upstream.example/media", ExpiresAt: time.Now().Add(time.Hour)}
	accounts := &videoAccountRepoStub{accounts: []Account{videoTestAccount(42, groupID, "https://upstream.example/v1")}}
	requests := 0
	upstream := &videoHTTPUpstreamStub{do: func(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusAccepted, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"job_id":"up-job","status":"pending","amount":"1"}`))}, nil
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)
	_, _, _, _, err := svc.rewriteGenerationRequest(context.Background(), VideoOwner{UserID: 11, APIKeyID: 99, GroupID: videoInt64Ptr(groupID)}, []byte(`{"model":"video-public","prompt":"x","start_frame_url":"https://video.example/v1/videos/uploads/vidmedia_local/content"}`))
	require.ErrorIs(t, err, ErrVideoResourceNotFound)

	owner := VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)}
	_, err = svc.Create(context.Background(), owner, []byte(`{"model":"video-public","prompt":"first","start_frame_url":"https://video.example/v1/videos/uploads/vidmedia_local/content"}`), "same-key")
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), owner, []byte(`{"model":"video-public","prompt":"different","start_frame_url":"https://video.example/v1/videos/uploads/vidmedia_local/content"}`), "same-key")
	require.ErrorIs(t, err, ErrVideoIdempotencyConflict)
	require.Equal(t, 1, requests)
}

func TestXiaoVideoCreateScopesUpstreamIdempotencyByDownstreamKey(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	accounts := &videoAccountRepoStub{accounts: []Account{videoTestAccount(42, groupID, "https://upstream.example/v1")}}
	upstreamKeys := make([]string, 0, 2)
	upstream := &videoHTTPUpstreamStub{do: func(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		upstreamKeys = append(upstreamKeys, req.Header.Get("Idempotency-Key"))
		jobID := "up-job-" + strconv.Itoa(len(upstreamKeys))
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"job_id":"` + jobID + `","status":"pending","amount":"1"}`)),
		}, nil
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)
	body := []byte(`{"model":"video-public","prompt":"same client order id"}`)

	_, err := svc.Create(context.Background(), VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)}, body, "shared-order-id")
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), VideoOwner{UserID: 11, APIKeyID: 23, GroupID: videoInt64Ptr(groupID)}, body, "shared-order-id")
	require.NoError(t, err)
	require.Len(t, upstreamKeys, 2)
	require.NotEmpty(t, upstreamKeys[0])
	require.NotEqual(t, upstreamKeys[0], upstreamKeys[1])
}

func TestXiaoVideoCreateMapsAccountExhaustionToVideoCapacityError(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	accounts := &videoAccountRepoStub{}
	svc := newVideoServiceForTest(repo, accounts, &videoHTTPUpstreamStub{do: func(*http.Request, string, int64, int) (*http.Response, error) {
		t.Fatal("upstream must not be called")
		return nil, nil
	}})

	_, err := svc.Create(
		context.Background(),
		VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)},
		[]byte(`{"model":"video-public","prompt":"no capacity"}`),
		"no-capacity",
	)
	require.ErrorIs(t, err, ErrVideoCapacityExhausted)
}

func TestXiaoVideoCreateSelectsNextConfiguredAccountForModel(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	unsupported := videoTestAccount(41, groupID, "https://first.example.test/v1")
	unsupported.Priority = 1
	unsupported.Credentials["model_mapping"] = map[string]any{"another-video-model": "upstream-a"}
	selected := videoTestAccount(42, groupID, "https://second.example.test/v1")
	selected.Priority = 2
	accounts := &videoAccountRepoStub{accounts: []Account{unsupported, selected}}

	var selectedAccountID int64
	upstream := &videoHTTPUpstreamStub{do: func(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
		selectedAccountID = accountID
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"job_id":"up-job-selected","status":"pending","amount":"1"}`)),
		}, nil
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)

	job, err := svc.Create(
		context.Background(),
		VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)},
		[]byte(`{"model":"video-public","prompt":"select configured upstream"}`),
		"select-configured-account",
	)
	require.NoError(t, err)
	require.Equal(t, int64(42), selectedAccountID)
	require.Equal(t, int64(42), job.AccountID)
}

func TestXiaoVideoListModelsMapsAliases(t *testing.T) {
	const groupID int64 = 7
	accounts := &videoAccountRepoStub{accounts: []Account{videoTestAccount(42, groupID, "https://upstream.example/v1")}}
	upstream := &videoHTTPUpstreamStub{do: func(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
		require.Equal(t, int64(42), accountID)
		require.Equal(t, "/v1/models", req.URL.Path)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"object":"list","data":[{"id":"video-upstream-v2","object":"model","resolutions":["720p"]}]}`))}, nil
	}}
	svc := newVideoServiceForTest(newVideoRepositoryStub(), accounts, upstream)
	models, err := svc.ListModels(context.Background(), VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "video-public", models[0]["id"])
	require.Equal(t, []string{"720p"}, models[0]["resolutions"])
}

func TestXiaoVideoListModelsEnablesSeedanceGeneratedAudio(t *testing.T) {
	const groupID int64 = 7
	tests := []struct {
		name          string
		publicModel   string
		upstreamModel string
	}{
		{name: "seedance-2.0", publicModel: "seedance-2.0", upstreamModel: "seedance-2.0"},
		{name: "seedance-2.0-fast", publicModel: "seedance-2.0-fast", upstreamModel: "seedance-2.0-fast"},
		{name: "seedance-2.0-mini", publicModel: "seedance-2.0-mini", upstreamModel: "seedance-2.0-mini"},
		{name: "dynamic public alias", publicModel: "customer-seedance", upstreamModel: "seedance-2.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := seedanceVideoTestAccount(42, groupID, tt.publicModel)
			account.Credentials["model_mapping"] = map[string]any{tt.publicModel: tt.upstreamModel}
			accounts := &videoAccountRepoStub{accounts: []Account{account}}
			upstream := &videoHTTPUpstreamStub{do: func(*http.Request, string, int64, int) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{"object":"list","data":[{"id":"` + tt.upstreamModel +
						`","object":"model","resolutions":["480p"],"supports_audio":false}]}`)),
				}, nil
			}}
			svc := newVideoServiceForTest(newVideoRepositoryStub(), accounts, upstream)

			models, err := svc.ListModels(context.Background(), VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)})
			require.NoError(t, err)
			require.Len(t, models, 1)
			require.Equal(t, tt.publicModel, models[0]["id"])
			require.Equal(t, true, models[0]["supports_audio"])
		})
	}
}

func TestXiaoVideoOpenMediaUsesBoundAccountAndForwardsRange(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	repo.media["vidmedia_local"] = &VideoMedia{MediaID: "vidmedia_local", UpstreamMediaID: "up media", AccountID: 42, UserID: 11, APIKeyID: 22, UpstreamURL: "https://upstream.example/media", ExpiresAt: time.Now().Add(time.Hour)}
	boundAccount := videoTestAccount(42, groupID, "https://upstream.example/v1")
	boundAccount.Status = StatusDisabled
	boundAccount.Schedulable = false
	accounts := &videoAccountRepoStub{accounts: []Account{boundAccount}}
	upstream := &videoHTTPUpstreamStub{do: func(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
		require.Equal(t, int64(42), accountID)
		require.Equal(t, "/v1/videos/uploads/up media/content", req.URL.Path)
		require.Equal(t, "download=1", req.URL.RawQuery)
		require.Equal(t, "bytes=0-99", req.Header.Get("Range"))
		return &http.Response{StatusCode: http.StatusPartialContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("video"))}, nil
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)
	resp, err := svc.OpenMedia(context.Background(), VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)}, "vidmedia_local", "bytes=0-99", "download=1")
	require.NoError(t, err)
	require.Equal(t, http.StatusPartialContent, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func TestXiaoVideoCancelDoesNotReleaseUntilUpstreamConfirmsTerminalStatus(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	repo.jobs["vidjob_local"] = &VideoJob{
		JobID:            "vidjob_local",
		UpstreamJobID:    "up-job",
		AccountID:        42,
		UserID:           11,
		APIKeyID:         22,
		Status:           "running",
		Amount:           2,
		Currency:         "USD",
		SettlementStatus: "held",
	}
	accounts := &videoAccountRepoStub{accounts: []Account{videoTestAccount(42, groupID, "https://upstream.example/v1")}}
	requests := 0
	upstream := &videoHTTPUpstreamStub{do: func(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
		require.Equal(t, int64(42), accountID)
		requests++
		switch req.Method {
		case http.MethodDelete:
			require.Equal(t, "/v1/videos/jobs/up-job", req.URL.Path)
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"job_id":"up-job","status":"running"}`))}, nil
		case http.MethodGet:
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"job_id":"up-job","status":"canceled"}`))}, nil
		default:
			t.Fatalf("unexpected method %s", req.Method)
			return nil, nil
		}
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)
	owner := VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)}

	job, err := svc.Cancel(context.Background(), owner, "vidjob_local")
	require.NoError(t, err)
	require.Equal(t, "running", job.Status)
	require.Equal(t, "held", job.SettlementStatus)

	job, err = svc.Get(context.Background(), owner, "vidjob_local")
	require.NoError(t, err)
	require.Equal(t, "canceled", job.Status)
	require.Equal(t, "released", job.SettlementStatus)
	require.Equal(t, 2, requests)
}

func TestXiaoVideoCreateRetriesUncertainResultOnOriginalReservation(t *testing.T) {
	const groupID int64 = 7
	repo := newVideoRepositoryStub()
	repo.media["vidmedia_retry"] = &VideoMedia{
		MediaID: "vidmedia_retry", AccountID: 42, UserID: 11, APIKeyID: 22,
		UpstreamURL: "https://upstream.example/media/retry", ExpiresAt: time.Now().Add(time.Hour),
	}
	accounts := &videoAccountRepoStub{accounts: []Account{videoTestAccount(42, groupID, "https://upstream.example/v1")}}
	requests := 0
	var upstreamIdempotencyKey string
	upstream := &videoHTTPUpstreamStub{do: func(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
		require.Equal(t, int64(42), accountID)
		requests++
		if requests == 1 {
			upstreamIdempotencyKey = req.Header.Get("Idempotency-Key")
			return nil, errors.New("connection reset after request write")
		}
		require.Equal(t, upstreamIdempotencyKey, req.Header.Get("Idempotency-Key"))
		return &http.Response{StatusCode: http.StatusAccepted, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"job_id":"up-job-stable","status":"pending","amount":"1"}`))}, nil
	}}
	svc := newVideoServiceForTest(repo, accounts, upstream)
	owner := VideoOwner{UserID: 11, APIKeyID: 22, GroupID: videoInt64Ptr(groupID)}
	body := []byte(`{"model":"video-public","prompt":"stable retry","start_frame_url":"https://video.example.test/v1/videos/uploads/vidmedia_retry/content"}`)

	_, err := svc.Create(context.Background(), owner, body, "stable-key")
	require.Error(t, err)
	require.Len(t, repo.jobs, 1)
	var reservedJobID string
	for _, job := range repo.jobs {
		reservedJobID = job.JobID
		require.Equal(t, int64(42), job.AccountID)
		require.True(t, strings.HasPrefix(job.UpstreamJobID, videoRetryableUpstreamPrefix))
	}

	job, err := svc.Create(context.Background(), owner, body, "stable-key")
	require.NoError(t, err)
	require.Equal(t, reservedJobID, job.JobID)
	require.Equal(t, "up-job-stable", job.UpstreamJobID)
	require.Equal(t, 2, requests)
}

func videoInt64Ptr(value int64) *int64 { return &value }
