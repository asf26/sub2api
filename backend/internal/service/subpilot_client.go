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
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

const (
	subPilotSelectPath        = "/v1/dispatch/select"
	subPilotReportSuccessPath = "/v1/dispatch/report-success"
	subPilotReportFailurePath = "/v1/dispatch/report-failure"
	subPilotDefaultTimeoutMS  = 80
)

type subPilotClient struct {
	cfg    config.SubPilotConfig
	client *http.Client
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

func newSubPilotClient(cfg *config.Config) subPilotClient {
	sp := config.SubPilotConfig{}
	if cfg != nil {
		sp = cfg.Gateway.SubPilot
	}
	sp.BaseURL = strings.TrimRight(strings.TrimSpace(sp.BaseURL), "/")
	if sp.TimeoutMS <= 0 {
		sp.TimeoutMS = subPilotDefaultTimeoutMS
	}
	return subPilotClient{
		cfg: sp,
		client: &http.Client{
			Timeout: time.Duration(sp.TimeoutMS) * time.Millisecond,
		},
	}
}

func (c subPilotClient) enabled() bool {
	return c.cfg.Enabled && c.cfg.BaseURL != ""
}

func (c subPilotClient) selectAccount(ctx context.Context, req subPilotSelectRequest) (*subPilotRecommendation, error) {
	if !c.enabled() {
		return nil, nil
	}
	if strings.TrimSpace(req.RequestID) == "" || strings.TrimSpace(req.Platform) == "" || strings.TrimSpace(req.Model) == "" {
		return nil, nil
	}
	var resp subPilotSelectResponse
	if err := c.postJSON(ctx, subPilotSelectPath, req, &resp); err != nil {
		return nil, c.fail(err)
	}
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

func (c subPilotClient) reportSuccess(ctx context.Context, req subPilotReportSuccessRequest) {
	if !c.enabled() {
		return
	}
	if err := c.postJSON(ctx, subPilotReportSuccessPath, req, nil); err != nil {
		slog.Debug("subpilot report success failed", "error", err)
	}
}

func (c subPilotClient) reportFailure(ctx context.Context, req subPilotReportFailureRequest) {
	if !c.enabled() {
		return
	}
	if err := c.postJSON(ctx, subPilotReportFailurePath, req, nil); err != nil {
		slog.Debug("subpilot report failure failed", "error", err)
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
	raw, err := io.ReadAll(resp.Body)
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
	subPilotLeases.Store(subPilotLeaseKey(requestID, accountID), subPilotLeaseRecord{
		LeaseID:   leaseID,
		CreatedAt: time.Now(),
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
