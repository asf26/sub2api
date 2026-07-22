package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type subPilotSoftCooldownAccountRepo struct {
	AccountRepository
	account *Account
}

func (r subPilotSoftCooldownAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, errors.New("account not found")
}

func (r subPilotSoftCooldownAccountRepo) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	return nil, nil
}

func (r subPilotSoftCooldownAccountRepo) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]Account, error) {
	return nil, nil
}

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

func TestOpenAIGatewayService_SubPilotAcceptsExplicitLastResortExcludedAccount(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

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
			require.Equal(t, []string{"37131"}, req.ExcludedAccountIDs)
			_, _ = w.Write([]byte(`{"decision":"selected","reason":"last_resort","account":{"id":"37131"},"lease":{"id":"lease-37131"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	groupID := int64(10220)
	accounts := []Account{{
		ID: 37131, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
	}}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		context.Background(), &groupID, "", "", "gpt-5.4",
		map[int64]struct{}{37131: {}}, OpenAIUpstreamTransportAny, false,
	)
	require.NoError(t, err)
	require.Equal(t, int64(37131), selection.Account.ID)
	require.Equal(t, "subpilot", decision.Layer)
}

func TestGatewaySubPilotAcceptsExplicitLastResortExcludedAccount(t *testing.T) {
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
			require.Equal(t, []string{"37201"}, req.ExcludedAccountIDs)
			_, _ = w.Write([]byte(`{"decision":"selected","reason":"last_resort","account":{"id":"37201"},"lease":{"id":"lease-37201"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	groupID := int64(10221)
	account := Account{
		ID: 37201, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
	}
	cfg := &config.Config{}
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	svc := &GatewayService{
		accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cfg:         cfg,
	}

	selection, handled, err := svc.trySubPilotRecommendForGateway(
		context.Background(), &groupID, "", "claude-opus-4-6",
		map[int64]struct{}{account.ID: {}}, PlatformAnthropic, true,
	)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, account.ID, selection.Account.ID)
}

func TestGatewaySubPilotLastResortReloadsSoftCooldownAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case subPilotRuntimeConfigPath:
			_ = json.NewEncoder(w).Encode(subPilotRuntimeConfig{
				DispatchEnabled: true, DispatchFailOpen: true, DispatchSelectTimeoutMS: 200,
				DispatchAutoBypassFailures: 3, DispatchAutoRecover: true,
			})
		case subPilotSelectPath:
			_, _ = w.Write([]byte(`{"decision":"selected","reason":"last_resort","account":{"id":"37202"},"lease":{"id":"lease-37202"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	groupID := int64(10222)
	cooldownUntil := time.Now().Add(time.Minute)
	account := &Account{
		ID: 37202, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		TempUnschedulableUntil: &cooldownUntil,
		AccountGroups:          []AccountGroup{{GroupID: groupID}},
	}
	cfg := &config.Config{}
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	svc := &GatewayService{accountRepo: subPilotSoftCooldownAccountRepo{account: account}, cfg: cfg}

	selection, handled, err := svc.trySubPilotRecommendForGateway(
		context.Background(), &groupID, "", "claude-opus-4-6", nil,
		PlatformAnthropic, false,
	)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, account.ID, selection.Account.ID)
}

func TestOpenAISubPilotLastResortRunsWhenSchedulableProjectionIsEmpty(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case subPilotRuntimeConfigPath:
			_ = json.NewEncoder(w).Encode(subPilotRuntimeConfig{
				DispatchEnabled: true, DispatchFailOpen: true, DispatchSelectTimeoutMS: 200,
				DispatchAutoBypassFailures: 3, DispatchAutoRecover: true,
			})
		case subPilotSelectPath:
			_, _ = w.Write([]byte(`{"decision":"selected","reason":"last_resort","account":{"id":"37203"},"lease":{"id":"lease-37203"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	groupID := int64(10223)
	cooldownUntil := time.Now().Add(time.Minute)
	account := &Account{
		ID: 37203, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		TempUnschedulableUntil: &cooldownUntil,
		AccountGroups:          []AccountGroup{{GroupID: groupID}},
	}
	cfg := &config.Config{}
	cfg.Gateway.SubPilot = config.SubPilotConfig{Enabled: true, BaseURL: server.URL, TimeoutMS: 200, FailOpen: true}
	svc := &OpenAIGatewayService{
		accountRepo:        subPilotSoftCooldownAccountRepo{account: account},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	selection, decision, err := svc.SelectAccountWithScheduler(
		context.Background(), &groupID, "", "", "gpt-5.4", nil,
		OpenAIUpstreamTransportAny, false,
	)
	require.NoError(t, err)
	require.Equal(t, account.ID, selection.Account.ID)
	require.Equal(t, "subpilot", decision.Layer)
}

func TestSubPilotLastResortStillRejectsManuallyDisabledAccount(t *testing.T) {
	groupID := int64(10224)
	account := &Account{
		ID: 37204, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: false,
		AccountGroups: []AccountGroup{{GroupID: groupID}},
	}
	svc := &GatewayService{accountRepo: subPilotSoftCooldownAccountRepo{account: account}, cfg: &config.Config{}}

	require.False(t, svc.isSubPilotGatewayAccountEligibleForRecommendation(
		context.Background(), account, "claude-opus-4-6", true,
	))
}
