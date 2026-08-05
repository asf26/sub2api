package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	videoCreatingUpstreamPrefix  = "creating:"
	videoRetryableUpstreamPrefix = "retry:"
)

var (
	ErrVideoGenerationDisabled   = infraerrors.New(http.StatusForbidden, "VIDEO_GENERATION_DISABLED", "video generation is not enabled for this API key")
	ErrVideoExecutionDisabled    = infraerrors.New(http.StatusServiceUnavailable, "VIDEO_EXECUTION_DISABLED", "video execution is disabled")
	ErrVideoResourceNotFound     = infraerrors.New(http.StatusNotFound, "VIDEO_RESOURCE_NOT_FOUND", "video resource not found")
	ErrVideoRequestInvalid       = infraerrors.New(http.StatusBadRequest, "VIDEO_REQUEST_INVALID", "video request is invalid")
	ErrVideoIdempotencyInvalid   = infraerrors.New(http.StatusBadRequest, "IDEMPOTENCY_KEY_INVALID", "idempotency key must be 1 to 128 printable ASCII characters")
	ErrVideoIdempotencyConflict  = infraerrors.New(http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", "idempotency key was reused with a different request")
	ErrVideoRequestInProgress    = infraerrors.New(http.StatusServiceUnavailable, "VIDEO_REQUEST_IN_PROGRESS", "the idempotent request is still being created")
	ErrVideoInsufficientBalance  = infraerrors.New(http.StatusPaymentRequired, "INSUFFICIENT_BALANCE", "insufficient balance")
	ErrVideoJobNotCancelable     = infraerrors.New(http.StatusConflict, "VIDEO_JOB_NOT_CANCELABLE", "video job is not cancelable")
	ErrVideoCapacityExhausted    = infraerrors.New(http.StatusTooManyRequests, "VIDEO_CAPACITY_EXHAUSTED", "video capacity is temporarily exhausted")
	ErrVideoPricingUnavailable   = infraerrors.New(http.StatusServiceUnavailable, "VIDEO_PRICING_UNAVAILABLE", "video pricing is unavailable")
	ErrVideoUpstreamUnavailable  = infraerrors.New(http.StatusServiceUnavailable, "VIDEO_UPSTREAM_UNAVAILABLE", "video upstream is unavailable")
	ErrVideoMediaAccountMismatch = infraerrors.New(http.StatusUnprocessableEntity, "VIDEO_MEDIA_INVALID", "all uploaded media must belong to the same video upstream")
)

type VideoOwner struct {
	UserID   int64
	APIKeyID int64
	GroupID  *int64
}

type VideoMedia struct {
	MediaID         string
	UpstreamMediaID string
	AccountID       int64
	UserID          int64
	APIKeyID        int64
	UpstreamURL     string
	MediaType       string
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

type VideoJob struct {
	JobID            string
	UpstreamJobID    string
	AccountID        int64
	UserID           int64
	APIKeyID         int64
	GroupID          *int64
	IdempotencyKey   *string
	RequestHash      string
	Model            string
	Resolution       string
	Duration         int
	AspectRatio      string
	Status           string
	Amount           float64
	Currency         string
	UpstreamAmount   *float64
	UpstreamCurrency string
	SettlementStatus string
	UpstreamResponse []byte
	CreatedAt        time.Time
	UpdatedAt        time.Time
	FinishedAt       *time.Time
	SettledAt        *time.Time
}

type VideoJobReservation struct {
	JobID                  string
	AccountID              int64
	Owner                  VideoOwner
	IdempotencyKey         string
	RequestHash            string
	Model                  string
	Resolution             string
	Duration               int
	AspectRatio            string
	PreauthorizationAmount float64
}

type VideoJobFinalization struct {
	JobID            string
	UpstreamJobID    string
	Status           string
	UpstreamAmount   float64
	UpstreamCurrency string
	Resolution       string
	Duration         int
	AspectRatio      string
	UpstreamResponse []byte
}

type VideoJobUpdate struct {
	JobID            string
	Status           string
	Resolution       string
	Duration         int
	AspectRatio      string
	UpstreamResponse []byte
	FinishedAt       *time.Time
}

type VideoRepository interface {
	CreateMedia(context.Context, *VideoMedia) error
	GetMediaForOwner(context.Context, string, int64) (*VideoMedia, error)
	ReserveJob(context.Context, VideoJobReservation) (*VideoJob, bool, error)
	MarkJobReservationRetryable(context.Context, string) error
	ClaimJobReservationRetry(context.Context, string, time.Time) (bool, error)
	ReleaseJobReservation(context.Context, string) error
	FinalizeJobAndReconcileHold(context.Context, VideoJobFinalization) (*VideoJob, error)
	GetJobForOwner(context.Context, string, int64) (*VideoJob, error)
	ListJobsForOwner(context.Context, int64, int) ([]*VideoJob, error)
	ListActiveJobs(context.Context, int) ([]*VideoJob, error)
	UpdateJobAndSettle(context.Context, VideoJobUpdate) (*VideoJob, error)
}

type VideoUpstreamError struct {
	Status int
	Header http.Header
	Body   []byte
}

func (e *VideoUpstreamError) Error() string {
	return "video upstream returned HTTP " + strconv.Itoa(e.Status)
}

type XiaoVideoService struct {
	repo          VideoRepository
	accountRepo   AccountRepository
	openAIGateway *OpenAIGatewayService
	httpUpstream  HTTPUpstream
	cfg           *config.Config
	client        *http.Client
	authCache     APIKeyAuthCacheInvalidator
	billingCache  *BillingCacheService
}

func NewXiaoVideoService(
	repo VideoRepository,
	accountRepo AccountRepository,
	openAIGateway *OpenAIGatewayService,
	httpUpstream HTTPUpstream,
	authCache APIKeyAuthCacheInvalidator,
	billingCache *BillingCacheService,
	cfg *config.Config,
) *XiaoVideoService {
	timeout := 30 * time.Second
	if cfg != nil && cfg.VideoAPI.RequestTimeoutSeconds > 0 {
		timeout = time.Duration(cfg.VideoAPI.RequestTimeoutSeconds) * time.Second
	}
	return &XiaoVideoService{
		repo:          repo,
		accountRepo:   accountRepo,
		openAIGateway: openAIGateway,
		httpUpstream:  httpUpstream,
		cfg:           cfg,
		client:        &http.Client{Timeout: timeout},
		authCache:     authCache,
		billingCache:  billingCache,
	}
}

func (s *XiaoVideoService) ActiveForGroup(ctx context.Context, groupID *int64) bool {
	if s == nil || s.cfg == nil || !s.cfg.VideoAPI.Active() || groupID == nil || *groupID <= 0 || s.accountRepo == nil {
		return false
	}
	accounts, err := s.accountRepo.ListModelAvailabilityCandidates(ctx, groupID, []string{PlatformXiaoAPI}, true)
	if err != nil {
		return false
	}
	for i := range accounts {
		if accounts[i].Platform == PlatformXiaoAPI && accounts[i].Type == AccountTypeAPIKey {
			return true
		}
	}
	return false
}

func (s *XiaoVideoService) Enabled() bool {
	return s != nil && s.cfg != nil && s.cfg.VideoAPI.Active()
}

func (s *XiaoVideoService) PublicBaseURL() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	return s.cfg.VideoAPI.PublicBaseURL
}

// ListModels merges video models from every eligible account in the API key's group.
func (s *XiaoVideoService) ListModels(ctx context.Context, owner VideoOwner) ([]map[string]any, error) {
	accounts, err := s.videoAccounts(ctx, owner.GroupID, false)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, ErrVideoUpstreamUnavailable
	}
	models := make(map[string]map[string]any)
	var firstErr error
	for i := range accounts {
		account := &accounts[i]
		pricing, pricingErr := account.XiaoVideoPricingRules()
		if pricingErr != nil {
			if firstErr == nil {
				firstErr = ErrVideoPricingUnavailable.WithCause(pricingErr)
			}
			continue
		}
		resp, requestErr := s.upstreamWithAccount(ctx, account, http.MethodGet, "/v1/models", "", nil, "", "")
		if requestErr != nil {
			if firstErr == nil {
				firstErr = requestErr
			}
			continue
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		if readErr != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if firstErr == nil {
				if readErr != nil {
					firstErr = ErrVideoUpstreamUnavailable.WithCause(readErr)
				} else {
					firstErr = upstreamVideoError(resp, raw)
				}
			}
			continue
		}
		var envelope struct {
			Data []map[string]any `json:"data"`
		}
		if json.Unmarshal(raw, &envelope) != nil {
			if firstErr == nil {
				firstErr = ErrVideoUpstreamUnavailable
			}
			continue
		}
		accountModels := pricedVideoModelsForAccount(account, pricing, envelope.Data)
		if len(accountModels) == 0 && firstErr == nil {
			firstErr = ErrVideoPricingUnavailable
		}
		for id, model := range accountModels {
			if existing, ok := models[id]; ok {
				mergePricedVideoModel(existing, model)
				continue
			}
			models[id] = model
		}
	}
	if len(models) == 0 && firstErr != nil {
		return nil, firstErr
	}
	keys := make([]string, 0, len(models))
	for key := range models {
		keys = append(keys, key)
	}
	sortStrings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		out = append(out, models[key])
	}
	return out, nil
}

func (s *XiaoVideoService) Upload(ctx context.Context, owner VideoOwner, body io.Reader, contentType string) (*VideoMedia, error) {
	account, release, err := s.selectVideoAccount(ctx, owner, "", 0, nil)
	if err != nil {
		return nil, err
	}
	defer release()
	resp, err := s.upstreamWithAccount(ctx, account, http.MethodPost, "/v1/videos/uploads", contentType, body, "", "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		return nil, ErrVideoUpstreamUnavailable.WithCause(readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, upstreamVideoError(resp, raw)
	}
	var upstream struct {
		MediaID   string    `json:"media_id"`
		URL       string    `json:"url"`
		Type      string    `json:"type"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(raw, &upstream); err != nil || strings.TrimSpace(upstream.MediaID) == "" || strings.TrimSpace(upstream.URL) == "" {
		return nil, ErrVideoUpstreamUnavailable.WithCause(errors.New("invalid upload response"))
	}
	if upstream.ExpiresAt.IsZero() {
		upstream.ExpiresAt = time.Now().Add(24 * time.Hour)
	}
	mediaID, err := newVideoID("vidmedia_")
	if err != nil {
		return nil, err
	}
	mediaType := upstream.Type
	if mediaType == "" {
		mediaType = "UPLOADED"
	}
	media := &VideoMedia{
		MediaID:         mediaID,
		UpstreamMediaID: upstream.MediaID,
		AccountID:       account.ID,
		UserID:          owner.UserID,
		APIKeyID:        owner.APIKeyID,
		UpstreamURL:     upstream.URL,
		MediaType:       mediaType,
		ExpiresAt:       upstream.ExpiresAt,
		CreatedAt:       time.Now(),
	}
	if err := s.repo.CreateMedia(ctx, media); err != nil {
		return nil, err
	}
	return media, nil
}

func (s *XiaoVideoService) OpenMedia(ctx context.Context, owner VideoOwner, mediaID, rangeHeader, rawQuery string) (*http.Response, error) {
	media, err := s.repo.GetMediaForOwner(ctx, mediaID, owner.APIKeyID)
	if err != nil {
		return nil, err
	}
	account, err := s.accountByID(ctx, media.AccountID)
	if err != nil {
		return nil, err
	}
	path := "/v1/videos/uploads/" + url.PathEscape(media.UpstreamMediaID) + "/content"
	if strings.TrimSpace(rawQuery) != "" {
		path += "?" + rawQuery
	}
	return s.upstreamWithAccount(ctx, account, http.MethodGet, path, "", nil, rangeHeader, "")
}

func (s *XiaoVideoService) Create(ctx context.Context, owner VideoOwner, body []byte, idempotencyKey string) (*VideoJob, error) {
	if !s.Enabled() {
		return nil, ErrVideoExecutionDisabled
	}
	if owner.GroupID == nil {
		return nil, ErrVideoGenerationDisabled
	}
	if !validVideoIdempotencyKey(idempotencyKey) {
		return nil, ErrVideoIdempotencyInvalid
	}
	rewritten, meta, requestHash, fixedAccountID, err := s.rewriteGenerationRequest(ctx, owner, body)
	if err != nil {
		return nil, err
	}
	jobID, err := newVideoID("vidjob_")
	if err != nil {
		return nil, err
	}
	excluded := make(map[int64]struct{})
	pricingUnavailable := false
	for attempt := 0; attempt < 3; attempt++ {
		account, release, selectErr := s.selectVideoAccount(ctx, owner, meta.Model, fixedAccountID, excluded)
		if selectErr != nil {
			return nil, selectErr
		}
		hasAPIKey := strings.TrimSpace(account.GetCredential("api_key")) != ""
		hasBaseURL := strings.TrimSpace(account.GetCredential("base_url")) != ""
		if !hasAPIKey || !hasBaseURL {
			slog.WarnContext(ctx, "xiao_video.account_credentials_incomplete",
				"account_id", account.ID,
				"attempt", attempt+1,
				"has_api_key", hasAPIKey,
				"has_base_url", hasBaseURL,
			)
			release()
			excluded[account.ID] = struct{}{}
			continue
		}
		preauthorizationAmount, resolvedResolution, resolvedDuration, pricingOK := account.XiaoVideoPrice(
			meta.Model, meta.Resolution, meta.Duration, meta.Audio,
		)
		if !pricingOK {
			slog.WarnContext(ctx, "xiao_video.account_pricing_unavailable",
				"account_id", account.ID,
				"attempt", attempt+1,
			)
			release()
			pricingUnavailable = true
			if fixedAccountID != 0 {
				return nil, ErrVideoPricingUnavailable
			}
			excluded[account.ID] = struct{}{}
			continue
		}
		resolvedMeta := meta
		resolvedMeta.Resolution = resolvedResolution
		resolvedMeta.Duration = resolvedDuration
		upstreamBody, rewriteErr := rewriteVideoRequest(
			rewritten,
			account.GetMappedModel(meta.Model),
			resolvedMeta.Resolution,
			resolvedMeta.Duration,
		)
		if rewriteErr != nil {
			release()
			return nil, rewriteErr
		}
		reserved, created, reserveErr := s.repo.ReserveJob(ctx, VideoJobReservation{
			JobID:                  jobID,
			AccountID:              account.ID,
			Owner:                  owner,
			IdempotencyKey:         idempotencyKey,
			RequestHash:            requestHash,
			Model:                  meta.Model,
			Resolution:             resolvedMeta.Resolution,
			Duration:               resolvedMeta.Duration,
			AspectRatio:            meta.AspectRatio,
			PreauthorizationAmount: preauthorizationAmount,
		})
		if reserveErr != nil {
			release()
			if created {
				_ = s.repo.ReleaseJobReservation(context.Background(), jobID)
				s.invalidateBalance(ctx, owner.UserID)
			}
			return nil, reserveErr
		}
		if !created {
			release()
			if reserved.RequestHash != requestHash {
				return nil, ErrVideoIdempotencyConflict
			}
			if !strings.HasPrefix(reserved.UpstreamJobID, videoCreatingUpstreamPrefix) &&
				!strings.HasPrefix(reserved.UpstreamJobID, videoRetryableUpstreamPrefix) {
				return reserved, nil
			}
			retryAccount, accountErr := s.accountByID(ctx, reserved.AccountID)
			if accountErr != nil {
				return nil, accountErr
			}
			account, release, accountErr = s.acquireVideoSlot(ctx, retryAccount)
			if accountErr != nil {
				return nil, accountErr
			}
			claimed, claimErr := s.repo.ClaimJobReservationRetry(ctx, reserved.JobID, time.Now().Add(-s.videoReservationStaleAfter()))
			if claimErr != nil {
				release()
				return nil, claimErr
			}
			if !claimed {
				release()
				return nil, ErrVideoRequestInProgress
			}
			jobID = reserved.JobID
			resolvedMeta.Resolution = reserved.Resolution
			resolvedMeta.Duration = reserved.Duration
			resolvedMeta.AspectRatio = reserved.AspectRatio
			upstreamBody, rewriteErr = rewriteVideoRequest(
				rewritten,
				account.GetMappedModel(meta.Model),
				resolvedMeta.Resolution,
				resolvedMeta.Duration,
			)
			if rewriteErr != nil {
				release()
				s.markVideoReservationRetryable(jobID)
				return nil, rewriteErr
			}
		}
		resp, requestErr := s.upstreamWithAccount(ctx, account, http.MethodPost, "/v1/videos/generations", "application/json", bytes.NewReader(upstreamBody), "", videoUpstreamIdempotencyScope(owner.APIKeyID, idempotencyKey))
		release()
		if requestErr != nil {
			if idempotencyKey != "" {
				s.markVideoReservationRetryable(jobID)
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, requestErr
			}
			_ = s.repo.ReleaseJobReservation(context.Background(), jobID)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if fixedAccountID == 0 && isRetryableVideoTransportError(requestErr) {
				excluded[account.ID] = struct{}{}
				continue
			}
			return nil, requestErr
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			if idempotencyKey != "" {
				s.markVideoReservationRetryable(jobID)
			} else {
				_ = s.repo.ReleaseJobReservation(context.Background(), jobID)
			}
			return nil, ErrVideoUpstreamUnavailable.WithCause(readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if idempotencyKey != "" && resp.StatusCode >= 500 {
				s.markVideoReservationRetryable(jobID)
				return nil, upstreamVideoError(resp, raw)
			}
			_ = s.repo.ReleaseJobReservation(context.Background(), jobID)
			if fixedAccountID == 0 && retryableVideoStatus(resp.StatusCode) {
				excluded[account.ID] = struct{}{}
				continue
			}
			return nil, upstreamVideoError(resp, raw)
		}
		var upstream map[string]any
		if err := json.Unmarshal(raw, &upstream); err != nil {
			if idempotencyKey != "" {
				s.markVideoReservationRetryable(jobID)
			} else {
				_ = s.repo.ReleaseJobReservation(context.Background(), jobID)
			}
			return nil, ErrVideoUpstreamUnavailable.WithCause(err)
		}
		upstreamID := videoStringValue(upstream["job_id"])
		status := defaultString(videoStringValue(upstream["status"]), "pending")
		amountText := videoStringValue(upstream["amount"])
		if upstreamID == "" {
			if idempotencyKey != "" {
				s.markVideoReservationRetryable(jobID)
			} else {
				_ = s.repo.ReleaseJobReservation(context.Background(), jobID)
			}
			return nil, ErrVideoUpstreamUnavailable.WithCause(errors.New("upstream job_id missing"))
		}
		if amountText == "" {
			refreshed, refreshedRaw, refreshErr := s.fetchUpstreamJob(ctx, account, upstreamID)
			if refreshErr != nil {
				if idempotencyKey != "" {
					s.markVideoReservationRetryable(jobID)
					return nil, refreshErr
				}
				_ = s.cancelUpstream(context.Background(), account, upstreamID)
				_ = s.repo.ReleaseJobReservation(context.Background(), jobID)
				return nil, refreshErr
			}
			upstream = refreshed
			raw = refreshedRaw
			status = defaultString(videoStringValue(upstream["status"]), status)
			amountText = videoStringValue(upstream["amount"])
		}
		if !isVideoStatus(status) {
			_ = s.cancelUpstream(context.Background(), account, upstreamID)
			_ = s.repo.ReleaseJobReservation(context.Background(), jobID)
			return nil, ErrVideoUpstreamUnavailable.WithCause(errors.New("invalid upstream video status"))
		}
		amount, amountErr := strconv.ParseFloat(amountText, 64)
		if amountErr != nil || amount < 0 {
			_ = s.cancelUpstream(context.Background(), account, upstreamID)
			_ = s.repo.ReleaseJobReservation(context.Background(), jobID)
			return nil, ErrVideoUpstreamUnavailable
		}
		upstreamCurrency := defaultString(videoStringValue(upstream["currency"]), "USD")
		resolvedMeta = resolveVideoGenerationMeta(resolvedMeta, upstream)
		job, finalizeErr := s.repo.FinalizeJobAndReconcileHold(ctx, VideoJobFinalization{
			JobID:            jobID,
			UpstreamJobID:    upstreamID,
			Status:           status,
			UpstreamAmount:   amount,
			UpstreamCurrency: upstreamCurrency,
			Resolution:       resolvedMeta.Resolution,
			Duration:         resolvedMeta.Duration,
			AspectRatio:      resolvedMeta.AspectRatio,
			UpstreamResponse: raw,
		})
		if finalizeErr != nil {
			_ = s.cancelUpstream(context.Background(), account, upstreamID)
			_ = s.repo.ReleaseJobReservation(context.Background(), jobID)
			return nil, finalizeErr
		}
		s.invalidateBalance(ctx, owner.UserID)
		return job, nil
	}
	if pricingUnavailable {
		return nil, ErrVideoPricingUnavailable
	}
	slog.WarnContext(ctx, "xiao_video.account_attempts_exhausted",
		"group_id", derefGroupID(owner.GroupID),
		"model", meta.Model,
		"excluded_accounts", len(excluded),
	)
	return nil, ErrVideoCapacityExhausted
}

type videoGenerationMeta struct {
	Model       string
	Resolution  string
	AspectRatio string
	Duration    int
	Audio       bool
}

func (s *XiaoVideoService) rewriteGenerationRequest(ctx context.Context, owner VideoOwner, body []byte) ([]byte, videoGenerationMeta, string, int64, error) {
	var request map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		return nil, videoGenerationMeta{}, "", 0, ErrVideoRequestInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, videoGenerationMeta{}, "", 0, ErrVideoRequestInvalid
	}
	allowed := map[string]struct{}{"model": {}, "prompt": {}, "resolution": {}, "duration": {}, "aspect_ratio": {}, "audio": {}, "prompt_enhance": {}, "image_url": {}, "start_frame_url": {}, "end_frame_url": {}, "guidances": {}}
	for key := range request {
		if _, ok := allowed[key]; !ok {
			return nil, videoGenerationMeta{}, "", 0, ErrVideoRequestInvalid
		}
	}
	for _, key := range []string{"model", "prompt", "resolution", "aspect_ratio", "prompt_enhance", "image_url", "start_frame_url", "end_frame_url"} {
		if value, exists := request[key]; exists && value != nil {
			if _, ok := value.(string); !ok {
				return nil, videoGenerationMeta{}, "", 0, ErrVideoRequestInvalid
			}
		}
	}
	meta := videoGenerationMeta{
		Model:       videoStringValue(request["model"]),
		Resolution:  videoStringValue(request["resolution"]),
		AspectRatio: videoStringValue(request["aspect_ratio"]),
	}
	if prompt := videoStringValue(request["prompt"]); meta.Model == "" || prompt == "" {
		return nil, meta, "", 0, ErrVideoRequestInvalid
	}
	if rawDuration, exists := request["duration"]; exists && rawDuration != nil {
		n, ok := rawDuration.(json.Number)
		if !ok {
			return nil, meta, "", 0, ErrVideoRequestInvalid
		}
		meta.Duration, _ = strconv.Atoi(n.String())
		if meta.Duration <= 0 {
			return nil, meta, "", 0, ErrVideoRequestInvalid
		}
	}
	if rawAudio, exists := request["audio"]; exists {
		audio, ok := rawAudio.(bool)
		if !ok {
			return nil, meta, "", 0, ErrVideoRequestInvalid
		}
		meta.Audio = audio
	}
	// Normalize omitted audio to false so idempotency and retries use one stable
	// representation while preserving the legacy no-audio behavior.
	request["audio"] = meta.Audio
	if videoStringValue(request["image_url"]) != "" && videoStringValue(request["start_frame_url"]) != "" {
		return nil, meta, "", 0, ErrVideoRequestInvalid
	}
	if rawGuidances, exists := request["guidances"]; exists && rawGuidances != nil {
		if _, ok := rawGuidances.(map[string]any); !ok {
			return nil, meta, "", 0, ErrVideoRequestInvalid
		}
	}
	var fixedAccountID int64
	mapURL := func(value string) (string, error) {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:") {
			return "", ErrVideoRequestInvalid
		}
		mapped, accountID, err := s.mapMediaURL(ctx, owner, value)
		if err != nil {
			return "", err
		}
		if accountID > 0 {
			if fixedAccountID > 0 && fixedAccountID != accountID {
				return "", ErrVideoMediaAccountMismatch
			}
			fixedAccountID = accountID
		}
		return mapped, nil
	}
	for _, key := range []string{"image_url", "start_frame_url", "end_frame_url"} {
		if value := videoStringValue(request[key]); value != "" {
			mapped, err := mapURL(value)
			if err != nil {
				return nil, meta, "", 0, err
			}
			request[key] = mapped
		}
	}
	if guidances, ok := request["guidances"].(map[string]any); ok {
		for _, listKey := range []string{"image_reference", "video_reference_base", "audio_reference"} {
			items, _ := guidances[listKey].([]any)
			for _, rawItem := range items {
				item, _ := rawItem.(map[string]any)
				for _, mediaKey := range []string{"image", "video", "audio"} {
					media, _ := item[mediaKey].(map[string]any)
					if media == nil {
						continue
					}
					if value := videoStringValue(media["url"]); value != "" {
						mapped, err := mapURL(value)
						if err != nil {
							return nil, meta, "", 0, err
						}
						media["url"] = mapped
					}
				}
			}
		}
	}
	rewritten, err := json.Marshal(request)
	if err != nil {
		return nil, meta, "", 0, ErrVideoRequestInvalid
	}
	digest := sha256.Sum256(rewritten)
	return rewritten, meta, hex.EncodeToString(digest[:]), fixedAccountID, nil
}

func (s *XiaoVideoService) mapMediaURL(ctx context.Context, owner VideoOwner, value string) (string, int64, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Path == "" {
		return value, 0, nil
	}
	marker := "/v1/videos/uploads/"
	index := strings.Index(parsed.Path, marker)
	if index < 0 {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "http://") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "https://") {
			return value, 0, nil
		}
		return "", 0, ErrVideoRequestInvalid
	}
	rest := parsed.Path[index+len(marker):]
	mediaID := strings.SplitN(rest, "/", 2)[0]
	if !strings.HasPrefix(mediaID, "vidmedia_") {
		return value, 0, nil
	}
	media, err := s.repo.GetMediaForOwner(ctx, mediaID, owner.APIKeyID)
	if err != nil {
		return "", 0, err
	}
	return media.UpstreamURL, media.AccountID, nil
}

func (s *XiaoVideoService) Get(ctx context.Context, owner VideoOwner, jobID string) (*VideoJob, error) {
	job, err := s.repo.GetJobForOwner(ctx, jobID, owner.APIKeyID)
	if err != nil {
		return nil, err
	}
	if isVideoTerminal(job.Status) {
		return job, nil
	}
	return s.refresh(ctx, job)
}

func (s *XiaoVideoService) List(ctx context.Context, owner VideoOwner, limit int) ([]*VideoJob, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListJobsForOwner(ctx, owner.APIKeyID, limit)
}

func (s *XiaoVideoService) Cancel(ctx context.Context, owner VideoOwner, jobID string) (*VideoJob, error) {
	job, err := s.repo.GetJobForOwner(ctx, jobID, owner.APIKeyID)
	if err != nil {
		return nil, err
	}
	if job.Status != "pending" && job.Status != "running" {
		return nil, ErrVideoJobNotCancelable
	}
	account, err := s.accountByID(ctx, job.AccountID)
	if err != nil {
		return nil, err
	}
	resp, err := s.upstreamWithAccount(ctx, account, http.MethodDelete, "/v1/videos/jobs/"+url.PathEscape(job.UpstreamJobID), "", nil, "", "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, upstreamVideoError(resp, raw)
	}
	var upstream map[string]any
	if json.Unmarshal(raw, &upstream) == nil {
		status := videoStringValue(upstream["status"])
		if isVideoStatus(status) {
			var finished *time.Time
			if isVideoTerminal(status) {
				now := time.Now()
				finished = &now
			}
			meta := resolveVideoGenerationMeta(videoGenerationMeta{
				Resolution:  job.Resolution,
				Duration:    job.Duration,
				AspectRatio: job.AspectRatio,
			}, upstream)
			updated, updateErr := s.repo.UpdateJobAndSettle(ctx, VideoJobUpdate{
				JobID:            job.JobID,
				Status:           status,
				Resolution:       meta.Resolution,
				Duration:         meta.Duration,
				AspectRatio:      meta.AspectRatio,
				UpstreamResponse: raw,
				FinishedAt:       finished,
			})
			if updateErr != nil {
				return nil, updateErr
			}
			if updated.SettlementStatus != job.SettlementStatus {
				s.invalidateBalance(ctx, updated.UserID)
			}
			return updated, nil
		}
	}

	// A successful DELETE only requests cancellation. If the response does not
	// carry a documented status, try one immediate refresh and otherwise leave
	// the held amount untouched for the reconciler to settle later.
	updated, refreshErr := s.refresh(ctx, job)
	if refreshErr == nil {
		return updated, nil
	}
	return job, nil
}

func (s *XiaoVideoService) OpenContent(ctx context.Context, owner VideoOwner, jobID, rangeHeader, rawQuery string) (*http.Response, error) {
	job, err := s.Get(ctx, owner, jobID)
	if err != nil {
		return nil, err
	}
	if job.Status != "completed" {
		return nil, ErrVideoResourceNotFound
	}
	account, err := s.accountByID(ctx, job.AccountID)
	if err != nil {
		return nil, err
	}
	path := "/v1/videos/jobs/" + url.PathEscape(job.UpstreamJobID) + "/content"
	if strings.TrimSpace(rawQuery) != "" {
		path += "?" + rawQuery
	}
	return s.upstreamWithAccount(ctx, account, http.MethodGet, path, "", nil, rangeHeader, "")
}

func (s *XiaoVideoService) Reconcile(ctx context.Context, limit int) error {
	jobs, err := s.repo.ListActiveJobs(ctx, limit)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, _ = s.refresh(ctx, job)
	}
	return nil
}

func (s *XiaoVideoService) refresh(ctx context.Context, job *VideoJob) (*VideoJob, error) {
	account, err := s.accountByID(ctx, job.AccountID)
	if err != nil {
		return nil, err
	}
	resp, err := s.upstreamWithAccount(ctx, account, http.MethodGet, "/v1/videos/jobs/"+url.PathEscape(job.UpstreamJobID), "", nil, "", "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		return nil, ErrVideoUpstreamUnavailable.WithCause(readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, upstreamVideoError(resp, raw)
	}
	var upstream map[string]any
	if err := json.Unmarshal(raw, &upstream); err != nil {
		return nil, ErrVideoUpstreamUnavailable.WithCause(err)
	}
	status := defaultString(videoStringValue(upstream["status"]), job.Status)
	if !isVideoStatus(status) {
		return nil, ErrVideoUpstreamUnavailable.WithCause(errors.New("invalid upstream video status"))
	}
	var finished *time.Time
	if isVideoTerminal(status) {
		now := time.Now()
		finished = &now
	}
	meta := resolveVideoGenerationMeta(videoGenerationMeta{
		Resolution:  job.Resolution,
		Duration:    job.Duration,
		AspectRatio: job.AspectRatio,
	}, upstream)
	updated, err := s.repo.UpdateJobAndSettle(ctx, VideoJobUpdate{
		JobID:            job.JobID,
		Status:           status,
		Resolution:       meta.Resolution,
		Duration:         meta.Duration,
		AspectRatio:      meta.AspectRatio,
		UpstreamResponse: raw,
		FinishedAt:       finished,
	})
	if err == nil && updated.SettlementStatus != job.SettlementStatus {
		s.invalidateBalance(ctx, updated.UserID)
	}
	return updated, err
}

func (s *XiaoVideoService) fetchUpstreamJob(ctx context.Context, account *Account, upstreamID string) (map[string]any, []byte, error) {
	resp, err := s.upstreamWithAccount(ctx, account, http.MethodGet, "/v1/videos/jobs/"+url.PathEscape(upstreamID), "", nil, "", "")
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if readErr != nil {
		return nil, nil, ErrVideoUpstreamUnavailable.WithCause(readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, upstreamVideoError(resp, raw)
	}
	var upstream map[string]any
	if err := json.Unmarshal(raw, &upstream); err != nil {
		return nil, nil, ErrVideoUpstreamUnavailable.WithCause(err)
	}
	return upstream, raw, nil
}

func (s *XiaoVideoService) cancelUpstream(ctx context.Context, account *Account, upstreamID string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := s.upstreamWithAccount(ctx, account, http.MethodDelete, "/v1/videos/jobs/"+url.PathEscape(upstreamID), "", nil, "", "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return nil
}

func (s *XiaoVideoService) selectVideoAccount(ctx context.Context, owner VideoOwner, model string, fixedAccountID int64, excluded map[int64]struct{}) (*Account, func(), error) {
	if fixedAccountID > 0 {
		account, err := s.accountByID(ctx, fixedAccountID)
		if err != nil {
			return nil, func() {}, err
		}
		if !s.accountEligibleForVideo(owner, account, model) {
			return nil, func() {}, ErrVideoUpstreamUnavailable
		}
		return s.acquireVideoSlot(ctx, account)
	}
	accounts, err := s.videoAccounts(ctx, owner.GroupID, true)
	if err != nil {
		return nil, func() {}, ErrVideoUpstreamUnavailable.WithCause(err)
	}
	sort.SliceStable(accounts, func(i, j int) bool {
		if accounts[i].Priority != accounts[j].Priority {
			return accounts[i].Priority < accounts[j].Priority
		}
		switch {
		case accounts[i].LastUsedAt == nil && accounts[j].LastUsedAt != nil:
			return true
		case accounts[i].LastUsedAt != nil && accounts[j].LastUsedAt == nil:
			return false
		case accounts[i].LastUsedAt != nil && accounts[j].LastUsedAt != nil && !accounts[i].LastUsedAt.Equal(*accounts[j].LastUsedAt):
			return accounts[i].LastUsedAt.Before(*accounts[j].LastUsedAt)
		default:
			return accounts[i].ID < accounts[j].ID
		}
	})

	eligible := 0
	for i := range accounts {
		account := &accounts[i]
		if _, skip := excluded[account.ID]; skip || !s.accountEligibleForVideo(owner, account, model) {
			continue
		}
		eligible++
		selected, release, acquireErr := s.acquireVideoSlot(ctx, account)
		if acquireErr == nil {
			return selected, release, nil
		}
		if !errors.Is(acquireErr, ErrVideoCapacityExhausted) {
			return nil, func() {}, acquireErr
		}
	}
	slog.WarnContext(ctx, "xiao_video.account_selection_exhausted",
		"group_id", derefGroupID(owner.GroupID),
		"model", model,
		"candidate_accounts", len(accounts),
		"eligible_accounts", eligible,
	)
	return nil, func() {}, ErrVideoCapacityExhausted
}

func (s *XiaoVideoService) acquireVideoSlot(ctx context.Context, account *Account) (*Account, func(), error) {
	if account == nil {
		return nil, func() {}, ErrVideoUpstreamUnavailable
	}
	if s.openAIGateway == nil {
		return nil, func() {}, ErrVideoUpstreamUnavailable
	}
	result, err := s.openAIGateway.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if err != nil {
		return nil, func() {}, ErrVideoUpstreamUnavailable.WithCause(err)
	}
	if result == nil || !result.Acquired {
		slog.WarnContext(ctx, "xiao_video.account_slot_exhausted",
			"account_id", account.ID,
			"max_concurrency", account.Concurrency,
		)
		return nil, func() {}, ErrVideoCapacityExhausted
	}
	return account, result.ReleaseFunc, nil
}

func (s *XiaoVideoService) accountByID(ctx context.Context, id int64) (*Account, error) {
	if id <= 0 || s.accountRepo == nil {
		return nil, ErrVideoUpstreamUnavailable
	}
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil || account == nil {
		return nil, ErrVideoUpstreamUnavailable
	}
	if account.Platform != PlatformXiaoAPI || account.Type != AccountTypeAPIKey {
		return nil, ErrVideoUpstreamUnavailable
	}
	if strings.TrimSpace(account.GetCredential("api_key")) == "" || strings.TrimSpace(account.GetCredential("base_url")) == "" {
		return nil, ErrVideoUpstreamUnavailable
	}
	return account, nil
}

func (s *XiaoVideoService) accountEligibleForVideo(owner VideoOwner, account *Account, model string) bool {
	if account == nil || account.Platform != PlatformXiaoAPI || account.Type != AccountTypeAPIKey || !account.IsSchedulable() {
		return false
	}
	if model != "" && !account.IsModelSupported(model) {
		return false
	}
	if owner.GroupID == nil {
		return len(account.GroupIDs) == 0 && len(account.AccountGroups) == 0
	}
	for _, id := range account.GroupIDs {
		if id == *owner.GroupID {
			return true
		}
	}
	for _, group := range account.AccountGroups {
		if group.GroupID == *owner.GroupID {
			return true
		}
	}
	return false
}

func (s *XiaoVideoService) videoAccounts(ctx context.Context, groupID *int64, includeTransient bool) ([]Account, error) {
	if s == nil || s.accountRepo == nil || groupID == nil {
		return nil, ErrVideoUpstreamUnavailable
	}
	accounts, err := s.accountRepo.ListModelAvailabilityCandidates(ctx, groupID, []string{PlatformXiaoAPI}, true)
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Platform != PlatformXiaoAPI || account.Type != AccountTypeAPIKey {
			continue
		}
		if !includeTransient && !account.IsActive() {
			continue
		}
		out = append(out, account)
	}
	return out, nil
}

func (s *XiaoVideoService) upstreamWithAccount(ctx context.Context, account *Account, method, path, contentType string, body io.Reader, rangeHeader, idempotencyKey string) (*http.Response, error) {
	if !s.Enabled() {
		return nil, ErrVideoExecutionDisabled
	}
	if account == nil {
		return nil, ErrVideoUpstreamUnavailable
	}
	endpoint, err := accountVideoEndpoint(account.GetCredential("base_url"), path)
	if err != nil {
		return nil, ErrVideoUpstreamUnavailable.WithCause(err)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, ErrVideoUpstreamUnavailable.WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.GetCredential("api_key")))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if method == http.MethodPost && path == "/v1/videos/generations" {
		req.Header.Set("Prefer", "respond-async")
	}
	if strings.TrimSpace(rangeHeader) != "" {
		req.Header.Set("Range", rangeHeader)
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		digest := sha256.Sum256([]byte(strconv.FormatInt(account.ID, 10) + ":" + idempotencyKey))
		req.Header.Set("Idempotency-Key", "sub2api-"+hex.EncodeToString(digest[:]))
	}
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	if s.httpUpstream != nil {
		resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
		if err != nil {
			return nil, ErrVideoUpstreamUnavailable.WithCause(err)
		}
		return resp, nil
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, ErrVideoUpstreamUnavailable.WithCause(err)
	}
	return resp, nil
}

func accountVideoEndpoint(rawBase, path string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(rawBase))
	if err != nil || base.Scheme == "" || base.Host == "" || base.RawQuery != "" || base.Fragment != "" {
		return "", errors.New("invalid video account base_url")
	}
	requestURL, err := url.Parse(path)
	if err != nil || requestURL.IsAbs() || requestURL.Host != "" || requestURL.Fragment != "" {
		return "", errors.New("invalid video upstream path")
	}
	basePath := strings.TrimRight(base.Path, "/")
	requestPath := requestURL.Path
	if strings.HasSuffix(basePath, "/v1") && strings.HasPrefix(requestPath, "/v1/") {
		requestPath = strings.TrimPrefix(requestPath, "/v1")
	}
	base.Path = basePath + requestPath
	base.RawPath = ""
	base.RawQuery = requestURL.RawQuery
	return base.String(), nil
}

func rewriteVideoRequest(body []byte, model, resolution string, duration int) ([]byte, error) {
	model = strings.TrimSpace(model)
	resolution = strings.TrimSpace(resolution)
	if model == "" || resolution == "" || duration <= 0 {
		return nil, ErrVideoRequestInvalid
	}
	var request map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		return nil, ErrVideoRequestInvalid
	}
	request["model"] = model
	request["resolution"] = resolution
	request["duration"] = duration
	rewritten, err := json.Marshal(request)
	if err != nil {
		return nil, ErrVideoRequestInvalid
	}
	return rewritten, nil
}

func seedanceSupportsGeneratedAudio(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "seedance-2.0", "seedance-2.0-fast", "seedance-2.0-mini":
		return true
	default:
		return false
	}
}

func pricedVideoModelsForAccount(account *Account, pricing []XiaoVideoPricingRule, upstream []map[string]any) map[string]map[string]any {
	upstreamByID := make(map[string]map[string]any, len(upstream))
	for _, item := range upstream {
		if id := videoStringValue(item["id"]); id != "" {
			upstreamByID[id] = item
		}
	}
	rulesByModel := make(map[string][]XiaoVideoPricingRule)
	for _, rule := range pricing {
		rulesByModel[rule.Model] = append(rulesByModel[rule.Model], rule)
	}
	out := make(map[string]map[string]any, len(rulesByModel))
	for publicModel, rules := range rulesByModel {
		upstreamModel := account.GetMappedModel(publicModel)
		capability := upstreamByID[upstreamModel]
		if capability == nil {
			continue
		}
		supportedResolutions := videoStringSet(capability["resolutions"])
		resolutions := make([]string, 0, len(rules))
		defaultResolution := ""
		defaultDuration := 0
		for _, rule := range rules {
			if len(supportedResolutions) > 0 {
				if _, ok := supportedResolutions[rule.Resolution]; !ok {
					continue
				}
			}
			resolutions = append(resolutions, rule.Resolution)
			if rule.DefaultResolution {
				defaultResolution = rule.Resolution
				defaultDuration = rule.DefaultDuration
			}
		}
		if len(resolutions) == 0 {
			continue
		}
		sort.Strings(resolutions)
		if defaultResolution == "" && len(resolutions) == 1 {
			defaultResolution = resolutions[0]
			for _, rule := range rules {
				if rule.Resolution == defaultResolution {
					defaultDuration = rule.DefaultDuration
					break
				}
			}
		}
		model := map[string]any{
			"id":          publicModel,
			"object":      "model",
			"owned_by":    "video",
			"resolutions": resolutions,
		}
		if defaultResolution != "" {
			model["default_resolution"] = defaultResolution
		}
		if defaultDuration > 0 {
			model["default_duration"] = defaultDuration
		}
		for _, key := range []string{"default_aspect_ratio", "supports_guidances"} {
			if value, ok := capability[key]; ok {
				model[key] = value
			}
		}
		if seedanceSupportsGeneratedAudio(publicModel) || seedanceSupportsGeneratedAudio(upstreamModel) {
			model["supports_audio"] = true
		} else if value, ok := capability["supports_audio"]; ok {
			model["supports_audio"] = value
		}
		out[publicModel] = model
	}
	return out
}

func videoStringSet(value any) map[string]struct{} {
	out := make(map[string]struct{})
	switch values := value.(type) {
	case []any:
		for _, item := range values {
			if text := videoStringValue(item); text != "" {
				out[text] = struct{}{}
			}
		}
	case []string:
		for _, item := range values {
			if text := strings.TrimSpace(item); text != "" {
				out[text] = struct{}{}
			}
		}
	}
	return out
}

func mergePricedVideoModel(target, source map[string]any) {
	resolutions := videoStringSet(target["resolutions"])
	for resolution := range videoStringSet(source["resolutions"]) {
		resolutions[resolution] = struct{}{}
	}
	merged := make([]string, 0, len(resolutions))
	for resolution := range resolutions {
		merged = append(merged, resolution)
	}
	sort.Strings(merged)
	target["resolutions"] = merged
	for _, key := range []string{"default_resolution", "default_duration", "default_aspect_ratio", "supports_guidances"} {
		if _, exists := target[key]; exists {
			continue
		}
		if value, exists := source[key]; exists {
			target[key] = value
		}
	}
	targetAudio, targetHasAudio := target["supports_audio"]
	sourceAudio, sourceHasAudio := source["supports_audio"]
	if targetAudio == true || sourceAudio == true {
		target["supports_audio"] = true
	} else if !targetHasAudio && sourceHasAudio {
		target["supports_audio"] = sourceAudio
	}
}

func resolveVideoGenerationMeta(current videoGenerationMeta, upstream map[string]any) videoGenerationMeta {
	if resolution := videoStringValue(upstream["resolution"]); resolution != "" {
		current.Resolution = resolution
	}
	if duration, err := strconv.Atoi(videoStringValue(upstream["duration"])); err == nil && duration > 0 {
		current.Duration = duration
	}
	if aspectRatio := videoStringValue(upstream["aspect_ratio"]); aspectRatio != "" {
		current.AspectRatio = aspectRatio
	}
	return current
}

func isRetryableVideoTransportError(err error) bool {
	return err != nil && errors.Is(err, ErrVideoUpstreamUnavailable)
}

func retryableVideoStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout || status >= 500
}

func invalidateVideoBalance(authCache APIKeyAuthCacheInvalidator, billingCache *BillingCacheService, ctx context.Context, userID int64) {
	if authCache != nil {
		authCache.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if billingCache != nil {
		_ = billingCache.InvalidateUserBalance(ctx, userID)
	}
}

func (s *XiaoVideoService) invalidateBalance(ctx context.Context, userID int64) {
	invalidateVideoBalance(s.authCache, s.billingCache, ctx, userID)
}

func (s *XiaoVideoService) markVideoReservationRetryable(jobID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.repo.MarkJobReservationRetryable(ctx, jobID)
}

func (s *XiaoVideoService) videoReservationStaleAfter() time.Duration {
	delay := 40 * time.Second
	if s != nil && s.client != nil && s.client.Timeout > 0 {
		delay = s.client.Timeout + 10*time.Second
	}
	if delay < 15*time.Second {
		return 15 * time.Second
	}
	return delay
}

type XiaoVideoRuntime struct {
	svc    *XiaoVideoService
	cfg    *config.Config
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewXiaoVideoRuntime(svc *XiaoVideoService, cfg *config.Config) *XiaoVideoRuntime {
	runtime := &XiaoVideoRuntime{svc: svc, cfg: cfg}
	runtime.Start()
	return runtime
}

func (r *XiaoVideoRuntime) Start() {
	if r == nil || r.svc == nil || r.cfg == nil || !r.cfg.VideoAPI.Active() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	go func() {
		defer close(r.done)
		interval := time.Duration(r.cfg.VideoAPI.ReconcileIntervalSeconds) * time.Second
		if interval < 5*time.Second {
			interval = 30 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = r.svc.Reconcile(ctx, r.cfg.VideoAPI.ReconcileBatchSize)
			}
		}
	}()
}

func (r *XiaoVideoRuntime) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.cancel = nil
	r.done = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func validVideoIdempotencyKey(value string) bool {
	if len(value) < 1 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if char < 0x20 || char > 0x7e {
			return false
		}
	}
	return true
}

func videoUpstreamIdempotencyScope(apiKeyID int64, idempotencyKey string) string {
	if idempotencyKey == "" {
		return ""
	}
	return strconv.FormatInt(apiKeyID, 10) + ":" + idempotencyKey
}

func newVideoID(prefix string) (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(data[:]), nil
}

func isVideoTerminal(status string) bool {
	return status == "completed" || status == "failed" || status == "canceled"
}

func isVideoStatus(status string) bool {
	switch status {
	case "pending", "running", "settling", "completed", "failed", "canceled":
		return true
	default:
		return false
	}
}

func videoStringValue(value any) string {
	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return ""
	}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func upstreamVideoError(resp *http.Response, body []byte) error {
	return &VideoUpstreamError{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: body}
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
