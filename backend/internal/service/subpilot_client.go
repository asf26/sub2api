package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

const (
	subPilotSelectPath        = "/v1/dispatch/select"
	subPilotReportSuccessPath = "/v1/dispatch/report-success"
	subPilotReportFailurePath = "/v1/dispatch/report-failure"
	subPilotRuntimeConfigPath = "/v1/dispatch/runtime-config"
	subPilotDefaultTimeoutMS  = 80
	subPilotConfigCacheTTL    = 5 * time.Second
	subPilotReportTimeout     = 500 * time.Millisecond
	subPilotReportQueueSize   = 16384
	subPilotReportWorkers     = 8
	subPilotMaxIdleConns      = 256
	subPilotLeaseMaxAge       = 10 * time.Minute
	subPilotLeaseSweepWait    = time.Minute
)

type subPilotClient struct {
	cfg    config.SubPilotConfig
	client *http.Client
	state  *subPilotCircuitState
}

type subPilotRuntimeConfig struct {
	DispatchEnabled            bool `json:"dispatchEnabled"`
	DispatchFailOpen           bool `json:"dispatchFailOpen"`
	DispatchSelectTimeoutMS    int  `json:"dispatchSelectTimeoutMs"`
	DispatchAutoBypassFailures int  `json:"dispatchAutoBypassFailures"`
	DispatchAutoRecover        bool `json:"dispatchAutoRecover"`
}

type subPilotCircuitState struct {
	mu          sync.RWMutex
	runtime     subPilotRuntimeConfig
	lastRefresh time.Time
	refreshing  bool
	failures    atomic.Int64
	bypassed    atomic.Bool
}

type subPilotReportJob struct {
	client  subPilotClient
	path    string
	payload any
}

type subPilotSelectRequest struct {
	RequestID  string `json:"request_id"`
	Platform   string `json:"platform"`
	GroupID    string `json:"group_id,omitempty"`
	Model      string `json:"model"`
	SessionKey string `json:"session_key,omitempty"`
}

type subPilotSelectResponse struct {
	Decision string `json:"decision"`
	Account  struct {
		ID string `json:"id"`
	} `json:"account"`
	Lease struct {
		ID string `json:"id"`
	} `json:"lease"`
}

type subPilotRecommendation struct {
	AccountID int64
	LeaseID   string
	RequestID string
}

type subPilotReportSuccessRequest struct {
	RequestID       string  `json:"request_id"`
	LeaseID         string  `json:"lease_id"`
	APIKeyID        string  `json:"api_key_id,omitempty"`
	AccountID       string  `json:"account_id"`
	Platform        string  `json:"platform"`
	GroupID         string  `json:"group_id,omitempty"`
	Model           string  `json:"model,omitempty"`
	LatencyMS       int     `json:"latency_ms,omitempty"`
	FirstTokenMS    int     `json:"first_token_ms,omitempty"`
	RequestType     string  `json:"request_type,omitempty"`
	Stream          *bool   `json:"stream,omitempty"`
	OfficialUSDUsed float64 `json:"official_usd_used,omitempty"`
}

type subPilotReportFailureRequest struct {
	RequestID    string `json:"request_id"`
	LeaseID      string `json:"lease_id"`
	APIKeyID     string `json:"api_key_id,omitempty"`
	AccountID    string `json:"account_id"`
	Platform     string `json:"platform,omitempty"`
	GroupID      string `json:"group_id,omitempty"`
	Model        string `json:"model,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	RequestType  string `json:"request_type,omitempty"`
}

type subPilotLeaseRecord struct {
	LeaseID   string
	CreatedAt time.Time
}

var subPilotLeases sync.Map
var subPilotCircuitStates sync.Map
var subPilotSharedHTTPClient = newSubPilotSharedHTTPClient()
var subPilotReportQueue = make(chan subPilotReportJob, subPilotReportQueueSize)
var subPilotReportWorkersOnce sync.Once
var subPilotLeaseSweeperOnce sync.Once
var subPilotDroppedReports atomic.Uint64

func newSubPilotSharedHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = subPilotMaxIdleConns
	transport.MaxIdleConnsPerHost = subPilotMaxIdleConns
	return &http.Client{Transport: transport}
}

func newSubPilotClient(cfg *config.Config) subPilotClient {
	sp := config.SubPilotConfig{}
	if cfg != nil {
		sp = cfg.Gateway.SubPilot
	}
	sp.BaseURL = strings.TrimRight(strings.TrimSpace(sp.BaseURL), "/")
	if sp.TimeoutMS <= 0 {
		sp.TimeoutMS = subPilotDefaultTimeoutMS
	}
	stateValue, _ := subPilotCircuitStates.LoadOrStore(sp.BaseURL, &subPilotCircuitState{
		runtime: subPilotRuntimeConfig{
			DispatchEnabled:            sp.Enabled,
			DispatchFailOpen:           sp.FailOpen,
			DispatchSelectTimeoutMS:    sp.TimeoutMS,
			DispatchAutoBypassFailures: 3,
			DispatchAutoRecover:        true,
		},
	})
	return subPilotClient{
		cfg:    sp,
		client: subPilotSharedHTTPClient,
		state:  stateValue.(*subPilotCircuitState),
	}
}

func (c subPilotClient) enabled() bool {
	return c.cfg.Enabled && c.cfg.BaseURL != ""
}

func (c subPilotClient) selectAccount(ctx context.Context, req subPilotSelectRequest) (*subPilotRecommendation, error) {
	if !c.enabled() {
		return nil, nil
	}
	runtime := c.runtimeConfig(ctx)
	if !runtime.DispatchEnabled || c.isBypassed(runtime) {
		return nil, nil
	}
	if strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.Platform) == "" || strings.TrimSpace(req.Model) == "" {
		return nil, nil
	}
	var resp subPilotSelectResponse
	timeout := runtime.DispatchSelectTimeoutMS
	if timeout <= 0 {
		timeout = subPilotDefaultTimeoutMS
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	if err := c.postJSON(requestCtx, subPilotSelectPath, req, &resp); err != nil {
		c.recordFailure(runtime)
		if runtime.DispatchFailOpen {
			return nil, nil
		}
		return nil, err
	}
	c.recordSuccess()
	if resp.Decision != "selected" {
		return nil, nil
	}
	accountID, err := strconv.ParseInt(strings.TrimSpace(resp.Account.ID), 10, 64)
	if err != nil || accountID <= 0 {
		return nil, nil
	}
	return &subPilotRecommendation{
		AccountID: accountID,
		LeaseID:   strings.TrimSpace(resp.Lease.ID),
		RequestID: req.RequestID,
	}, nil
}

func (c subPilotClient) runtimeConfig(ctx context.Context) subPilotRuntimeConfig {
	if c.state == nil {
		return subPilotRuntimeConfig{
			DispatchEnabled: true, DispatchFailOpen: c.cfg.FailOpen,
			DispatchSelectTimeoutMS: c.cfg.TimeoutMS, DispatchAutoBypassFailures: 3, DispatchAutoRecover: true,
		}
	}
	c.state.mu.RLock()
	cached := c.state.runtime
	stale := c.state.lastRefresh.IsZero() || time.Since(c.state.lastRefresh) >= subPilotConfigCacheTTL
	refreshing := c.state.refreshing
	c.state.mu.RUnlock()
	if !stale || refreshing {
		return cached
	}
	c.state.mu.Lock()
	if c.state.refreshing || (!c.state.lastRefresh.IsZero() && time.Since(c.state.lastRefresh) < subPilotConfigCacheTTL) {
		cached = c.state.runtime
		c.state.mu.Unlock()
		return cached
	}
	c.state.refreshing = true
	c.state.mu.Unlock()

	refreshCtx, cancel := context.WithTimeout(ctx, time.Duration(max(c.cfg.TimeoutMS, subPilotDefaultTimeoutMS))*time.Millisecond)
	defer cancel()
	var next subPilotRuntimeConfig
	err := c.getJSON(refreshCtx, subPilotRuntimeConfigPath, &next)
	c.state.mu.Lock()
	c.state.refreshing = false
	c.state.lastRefresh = time.Now()
	if err != nil {
		cached = c.state.runtime
		c.state.mu.Unlock()
		return cached
	}
	if next.DispatchSelectTimeoutMS <= 0 {
		next.DispatchSelectTimeoutMS = subPilotDefaultTimeoutMS
	}
	if next.DispatchAutoBypassFailures <= 0 {
		next.DispatchAutoBypassFailures = 3
	}
	c.state.runtime = next
	if c.state.bypassed.Load() && next.DispatchAutoRecover {
		c.state.bypassed.Store(false)
		c.state.failures.Store(0)
	}
	c.state.mu.Unlock()
	return next
}

func (c subPilotClient) isBypassed(runtime subPilotRuntimeConfig) bool {
	if c.state == nil {
		return false
	}
	return c.state.bypassed.Load()
}

func (c subPilotClient) recordFailure(runtime subPilotRuntimeConfig) {
	if c.state == nil {
		return
	}
	threshold := runtime.DispatchAutoBypassFailures
	if threshold <= 0 {
		threshold = 3
	}
	if c.state.failures.Add(1) >= int64(threshold) {
		c.state.bypassed.Store(true)
	}
}

func (c subPilotClient) recordSuccess() {
	if c.state == nil {
		return
	}
	if c.state.failures.Load() != 0 {
		c.state.failures.Store(0)
	}
	if c.state.bypassed.Load() {
		c.state.bypassed.Store(false)
	}
}

func (c subPilotClient) reportSuccess(ctx context.Context, req subPilotReportSuccessRequest) {
	if !c.enabled() {
		return
	}
	c.enqueueReport(subPilotReportSuccessPath, req)
}

func (c subPilotClient) reportFailure(ctx context.Context, req subPilotReportFailureRequest) {
	if !c.enabled() {
		return
	}
	c.enqueueReport(subPilotReportFailurePath, req)
}

func (c subPilotClient) enqueueReport(path string, payload any) {
	subPilotReportWorkersOnce.Do(startSubPilotReportWorkers)
	select {
	case subPilotReportQueue <- subPilotReportJob{client: c, path: path, payload: payload}:
	default:
		dropped := subPilotDroppedReports.Add(1)
		if dropped == 1 || dropped%1000 == 0 {
			slog.Warn("subpilot report queue full", "dropped", dropped)
		}
	}
}

func startSubPilotReportWorkers() {
	for index := 0; index < subPilotReportWorkers; index++ {
		go func() {
			for job := range subPilotReportQueue {
				ctx, cancel := context.WithTimeout(context.Background(), subPilotReportTimeout)
				err := job.client.postJSON(ctx, job.path, job.payload, nil)
				cancel()
				if err != nil {
					slog.Debug("subpilot async report failed", "path", job.path, "error", err)
				}
			}
		}()
	}
}

func (c subPilotClient) postJSON(ctx context.Context, path string, in any, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	if secret := strings.TrimSpace(c.cfg.SharedSecret); secret != "" {
		httpReq.Header.Set("X-SubPilot-Secret", secret)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("subpilot status %d", resp.StatusCode)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c subPilotClient) getJSON(ctx context.Context, path string, out any) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+path, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/json")
	if secret := strings.TrimSpace(c.cfg.SharedSecret); secret != "" {
		httpReq.Header.Set("X-SubPilot-Secret", secret)
	}
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("subpilot status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(out)
}

func (c subPilotClient) fail(err error) error {
	if err == nil || c.cfg.FailOpen {
		return nil
	}
	return err
}

func subPilotRequestID(ctx context.Context, fallback string) string {
	if ctx != nil {
		if id, _ := ctx.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
		if id, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "subpilot-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func rememberSubPilotLease(requestID string, accountID int64, leaseID string) {
	requestID = strings.TrimSpace(requestID)
	leaseID = strings.TrimSpace(leaseID)
	if requestID == "" || accountID <= 0 || leaseID == "" {
		return
	}
	subPilotLeaseSweeperOnce.Do(startSubPilotLeaseSweeper)
	subPilotLeases.Store(subPilotLeaseKey(requestID, accountID), subPilotLeaseRecord{
		LeaseID:   leaseID,
		CreatedAt: time.Now(),
	})
}

func startSubPilotLeaseSweeper() {
	go func() {
		ticker := time.NewTicker(subPilotLeaseSweepWait)
		defer ticker.Stop()
		for now := range ticker.C {
			cleanupSubPilotLeases(now, subPilotLeaseMaxAge)
		}
	}()
}

func cleanupSubPilotLeases(now time.Time, maxAge time.Duration) {
	cutoff := now.Add(-maxAge)
	subPilotLeases.Range(func(key, value any) bool {
		record, ok := value.(subPilotLeaseRecord)
		if !ok || record.CreatedAt.Before(cutoff) {
			subPilotLeases.Delete(key)
		}
		return true
	})
}

func takeSubPilotLease(requestID string, accountID int64) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || accountID <= 0 {
		return syntheticSubPilotLease(requestID, accountID)
	}
	key := subPilotLeaseKey(requestID, accountID)
	if raw, ok := subPilotLeases.LoadAndDelete(key); ok {
		if record, ok := raw.(subPilotLeaseRecord); ok && strings.TrimSpace(record.LeaseID) != "" {
			return strings.TrimSpace(record.LeaseID)
		}
	}
	return syntheticSubPilotLease(requestID, accountID)
}

func subPilotLeaseKey(requestID string, accountID int64) string {
	return strings.TrimSpace(requestID) + "|" + strconv.FormatInt(accountID, 10)
}

func syntheticSubPilotLease(requestID string, accountID int64) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	return "external_" + requestID + "_" + strconv.FormatInt(accountID, 10)
}

func subPilotGroupID(groupID *int64) string {
	if groupID == nil || *groupID <= 0 {
		return ""
	}
	return strconv.FormatInt(*groupID, 10)
}

func subPilotUsageGroupID(log *UsageLog) string {
	if log == nil || log.GroupID == nil || *log.GroupID <= 0 {
		return ""
	}
	return strconv.FormatInt(*log.GroupID, 10)
}

func subPilotAPIKeyID(apiKeyID int64) string {
	if apiKeyID <= 0 {
		return ""
	}
	return strconv.FormatInt(apiKeyID, 10)
}

func subPilotRequestType(stream bool) string {
	if stream {
		return "stream"
	}
	return "sync"
}

func subPilotDurationMS(duration *int) int {
	if duration == nil || *duration < 0 {
		return 0
	}
	return *duration
}

func subPilotFirstTokenMS(firstToken *int) int {
	if firstToken == nil || *firstToken < 0 {
		return 0
	}
	return *firstToken
}

func subPilotPlatformForAccount(account *Account, fallback string) string {
	if account != nil && strings.TrimSpace(account.Platform) != "" {
		return account.Platform
	}
	return fallback
}

func subPilotErrNoAccount() error {
	return errors.New("subpilot selected account is not schedulable")
}
