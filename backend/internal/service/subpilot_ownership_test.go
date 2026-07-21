package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestSubPilotSelectNoChannelKeepsOwnershipAndForwardsExclusions(t *testing.T) {
	requests := make(chan subPilotSelectRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case subPilotRuntimeConfigPath:
			_ = json.NewEncoder(w).Encode(subPilotRuntimeConfig{
				DispatchEnabled: true, DispatchFailOpen: true, DispatchSelectTimeoutMS: 200,
				DispatchAutoBypassFailures: 3, DispatchAutoRecover: true,
			})
		case subPilotSelectPath:
			var req subPilotSelectRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			requests <- req
			_, _ = w.Write([]byte(`{"decision":"no_channel"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	client := newSubPilotClient(cfg)
	recommendation, handled, err := client.selectAccountWithOwnership(context.Background(), subPilotSelectRequest{
		RequestID:          "req-owned",
		Platform:           PlatformOpenAI,
		GroupID:            "10",
		Model:              "gpt-test",
		ExcludedAccountIDs: []string{"791", "787"},
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.Nil(t, recommendation)

	select {
	case req := <-requests:
		require.Equal(t, []string{"791", "787"}, req.ExcludedAccountIDs)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SubPilot select request")
	}
}

func TestSubPilotSelectTransportFailureOnlyFallsBackWhenFailOpen(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case subPilotRuntimeConfigPath:
			_ = json.NewEncoder(w).Encode(subPilotRuntimeConfig{
				DispatchEnabled: true, DispatchFailOpen: true, DispatchSelectTimeoutMS: 200,
				DispatchAutoBypassFailures: 3, DispatchAutoRecover: true,
			})
		case subPilotSelectPath:
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	client := newSubPilotClient(cfg)
	recommendation, handled, err := client.selectAccountWithOwnership(context.Background(), subPilotSelectRequest{
		RequestID: "req-fail-open", Platform: PlatformOpenAI, GroupID: "10", Model: "gpt-test",
	})
	require.NoError(t, err)
	require.False(t, handled)
	require.Nil(t, recommendation)
}

func TestSubPilotFailureReportUsesOriginalRequestLease(t *testing.T) {
	reports := make(chan subPilotReportFailureRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != subPilotReportFailurePath {
			http.NotFound(w, r)
			return
		}
		var req subPilotReportFailureRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		reports <- req
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	service := &OpenAIGatewayService{cfg: cfg}
	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "client-request-1")
	rememberSubPilotLease("client-request-1", 791, "lease-original-791")
	service.reportSubPilotFailureForOpenAIWithContext(ctx, 791)

	select {
	case req := <-reports:
		require.Equal(t, "client-request-1", req.RequestID)
		require.Equal(t, "lease-original-791", req.LeaseID)
		require.Equal(t, "791", req.AccountID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SubPilot failure report")
	}
}

func TestSubPilotOpenAISuccessReportUsesOriginalRequestLease(t *testing.T) {
	reports := make(chan subPilotReportSuccessRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != subPilotReportSuccessPath {
			http.NotFound(w, r)
			return
		}
		var req subPilotReportSuccessRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		reports <- req
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	const (
		requestID = "client-request-success-1"
		accountID = int64(791)
		leaseID   = "lease-original-success-791"
	)
	leaseKey := subPilotLeaseKey(requestID, accountID)
	t.Cleanup(func() { subPilotLeases.Delete(leaseKey) })

	cfg := &config.Config{}
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	service := &OpenAIGatewayService{cfg: cfg}
	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, requestID)
	rememberSubPilotLease(requestID, accountID, leaseID)
	service.reportSubPilotSuccess(ctx, &UsageLog{
		RequestID: "upstream-response-id",
		AccountID: accountID,
		APIKeyID:  5,
		Model:     "gpt-test",
	}, &Account{ID: accountID, Platform: PlatformOpenAI})

	select {
	case req := <-reports:
		require.Equal(t, requestID, req.RequestID)
		require.Equal(t, leaseID, req.LeaseID)
		require.Equal(t, "791", req.AccountID)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SubPilot success report")
	}
}

func TestSubPilotGatewayAccountEligibilityRejectsUnsupportedModel(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-opus-4-6": "claude-opus-4-6",
			},
		},
	}

	require.False(t, svc.isSubPilotGatewayAccountEligible(context.Background(), account, "claude-fable-5"))
	require.True(t, svc.isSubPilotGatewayAccountEligible(context.Background(), account, "claude-opus-4-6"))
}

func TestOpenAIGatewayService_SubPilotNoChannelDoesNotFallBackToNativeScheduler(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case subPilotRuntimeConfigPath:
			_ = json.NewEncoder(w).Encode(subPilotRuntimeConfig{
				DispatchEnabled: true, DispatchFailOpen: true, DispatchSelectTimeoutMS: 200,
				DispatchAutoBypassFailures: 3, DispatchAutoRecover: true,
			})
		case subPilotSelectPath:
			_, _ = w.Write([]byte(`{"decision":"no_channel"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	groupID := int64(10217)
	accounts := []Account{{
		ID: 37101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
	}}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}
	require.True(t, svc.isOpenAIAdvancedSchedulerEnabled(context.Background()))

	selection, _, err := svc.SelectAccountWithScheduler(
		context.Background(), &groupID, "", "", "gpt-5.4", nil,
		OpenAIUpstreamTransportAny, false,
	)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, selection)
}

func TestOpenAIGatewayService_SubPilotReceivesFailedAccountExclusions(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	requests := make(chan subPilotSelectRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case subPilotRuntimeConfigPath:
			_ = json.NewEncoder(w).Encode(subPilotRuntimeConfig{
				DispatchEnabled: true, DispatchFailOpen: true, DispatchSelectTimeoutMS: 200,
				DispatchAutoBypassFailures: 3, DispatchAutoRecover: true,
			})
		case subPilotSelectPath:
			var req subPilotSelectRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			requests <- req
			_, _ = w.Write([]byte(`{"decision":"selected","account":{"id":"37112"},"lease":{"id":"lease-37112"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	groupID := int64(10218)
	accounts := []Account{
		{ID: 37111, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
		{ID: 37112, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, _, err := svc.SelectAccountWithScheduler(
		context.Background(), &groupID, "", "", "gpt-5.4",
		map[int64]struct{}{37111: {}}, OpenAIUpstreamTransportAny, false,
	)
	require.NoError(t, err)
	require.Equal(t, int64(37112), selection.Account.ID)

	select {
	case req := <-requests:
		require.Equal(t, []string{"37111"}, req.ExcludedAccountIDs)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SubPilot select request")
	}
}
