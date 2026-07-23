package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	openaiapi "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type priorityOpenAIOAuthClient struct{}

func (priorityOpenAIOAuthClient) ExchangeCode(context.Context, string, string, string, string, string) (*openaiapi.TokenResponse, error) {
	return &openaiapi.TokenResponse{AccessToken: "access-token", RefreshToken: "refresh-token", ExpiresIn: 3600}, nil
}

func (priorityOpenAIOAuthClient) RefreshToken(context.Context, string, string) (*openaiapi.TokenResponse, error) {
	return nil, nil
}

func (priorityOpenAIOAuthClient) RefreshTokenWithClientID(context.Context, string, string, string) (*openaiapi.TokenResponse, error) {
	return nil, nil
}

type priorityGrokOAuthClient struct{}

func (priorityGrokOAuthClient) ExchangeCode(context.Context, string, string, string, string, string) (*xai.TokenResponse, error) {
	return &xai.TokenResponse{AccessToken: "access-token", RefreshToken: "refresh-token", ExpiresIn: 3600}, nil
}

func (priorityGrokOAuthClient) RefreshToken(context.Context, string, string, string) (*xai.TokenResponse, error) {
	return nil, nil
}

func (priorityGrokOAuthClient) ConvertSSOToBuild(context.Context, string, string) (*xai.TokenResponse, error) {
	return &xai.TokenResponse{AccessToken: "access-token", RefreshToken: "refresh-token", ExpiresIn: 3600}, nil
}

func TestResolveNewAccountPriority(t *testing.T) {
	zero := 0
	tests := []struct {
		name        string
		accountType string
		priority    *int
		want        int
	}{
		{name: "OAuth omitted", accountType: service.AccountTypeOAuth, want: 10},
		{name: "OAuth explicit zero", accountType: service.AccountTypeOAuth, priority: &zero, want: 0},
		{name: "Setup token omitted", accountType: service.AccountTypeSetupToken, want: 10},
		{name: "Setup token explicit zero", accountType: service.AccountTypeSetupToken, priority: &zero, want: 0},
		{name: "API key omitted", accountType: service.AccountTypeAPIKey, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, resolveNewAccountPriority(tt.accountType, tt.priority))
		})
	}
}

func TestCreateAccountPriorityDefaultAndExplicitZero(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
		want int
	}{
		{
			name: "OAuth omitted",
			body: map[string]any{"name": "oauth", "platform": service.PlatformOpenAI, "type": service.AccountTypeOAuth, "credentials": map[string]any{"access_token": "token"}},
			want: 10,
		},
		{
			name: "OAuth explicit zero",
			body: map[string]any{"name": "oauth", "platform": service.PlatformOpenAI, "type": service.AccountTypeOAuth, "credentials": map[string]any{"access_token": "token"}, "priority": 0},
			want: 0,
		},
		{
			name: "Setup token omitted",
			body: map[string]any{"name": "setup-token", "platform": service.PlatformAnthropic, "type": service.AccountTypeSetupToken, "credentials": map[string]any{"access_token": "token"}},
			want: 10,
		},
		{
			name: "API key omitted",
			body: map[string]any{"name": "api-key", "platform": service.PlatformOpenAI, "type": service.AccountTypeAPIKey, "credentials": map[string]any{"api_key": "key"}},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adminService := newStubAdminService()
			handler := NewAccountHandler(adminService, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.POST("/accounts", handler.Create)

			postPriorityJSON(t, router, "/accounts", tt.body)
			require.Len(t, adminService.createdAccounts, 1)
			require.Equal(t, tt.want, adminService.createdAccounts[0].Priority)
		})
	}
}

func TestOpenAIOAuthCreationPriorityDefaultAndExplicitZero(t *testing.T) {
	testOAuthPriorityCases(t, func(t *testing.T, priority *int) int {
		oauthService := service.NewOpenAIOAuthService(nil, priorityOpenAIOAuthClient{})
		defer oauthService.Stop()
		auth, err := oauthService.GenerateAuthURL(context.Background(), nil, "", service.PlatformOpenAI)
		require.NoError(t, err)
		authURL, err := url.Parse(auth.AuthURL)
		require.NoError(t, err)

		adminService := newStubAdminService()
		handler := NewOpenAIOAuthHandler(oauthService, adminService, nil)
		router := gin.New()
		router.POST("/openai/create-from-oauth", handler.CreateAccountFromOAuth)
		body := map[string]any{"session_id": auth.SessionID, "code": "code", "state": authURL.Query().Get("state")}
		if priority != nil {
			body["priority"] = *priority
		}
		postPriorityJSON(t, router, "/openai/create-from-oauth", body)
		require.Len(t, adminService.createdAccounts, 1)
		return adminService.createdAccounts[0].Priority
	})
}

func TestGrokOAuthCreationPriorityDefaultAndExplicitZero(t *testing.T) {
	testOAuthPriorityCases(t, func(t *testing.T, priority *int) int {
		oauthService := service.NewGrokOAuthService(nil, priorityGrokOAuthClient{})
		defer oauthService.Stop()
		auth, err := oauthService.GenerateAuthURL(context.Background(), nil, "")
		require.NoError(t, err)

		adminService := newStubAdminService()
		handler := NewGrokOAuthHandler(oauthService, adminService, nil, nil)
		router := gin.New()
		router.POST("/grok/create-from-oauth", handler.CreateAccountFromOAuth)
		body := map[string]any{"session_id": auth.SessionID, "code": "code", "state": auth.State}
		if priority != nil {
			body["priority"] = *priority
		}
		postPriorityJSON(t, router, "/grok/create-from-oauth", body)
		require.Len(t, adminService.createdAccounts, 1)
		return adminService.createdAccounts[0].Priority
	})
}

func TestCodexPATPriorityRequestDistinguishesOmittedAndExplicitZero(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "omitted", body: `{"access_token":"at-token"}`, want: 10},
		{name: "explicit zero", body: `{"access_token":"at-token","priority":0}`, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req OpenAICodexPATCreateRequest
			require.NoError(t, json.Unmarshal([]byte(tt.body), &req))
			require.Equal(t, tt.want, resolveNewAccountPriority(service.AccountTypeOAuth, req.Priority))
		})
	}
}

func TestCodexSessionImportPriorityDefaultAndExplicitZero(t *testing.T) {
	testOAuthPriorityCases(t, func(t *testing.T, priority *int) int {
		adminService := newCodexImportMemoryAdminService(nil)
		handler := NewAccountHandler(adminService, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
		req := CodexSessionImportRequest{Priority: priority, SkipDefaultGroupBind: boolPtr(true)}
		entries := []codexImportEntry{{Index: 1, Value: buildCodexAccessOnlyImportValue(t, "workspace-priority", "user-priority")}}

		result, err := handler.importCodexSessions(context.Background(), req, entries)
		require.NoError(t, err)
		require.Equal(t, 1, result.Created)
		require.Len(t, adminService.createdAccounts, 1)
		return adminService.createdAccounts[0].Priority
	})
}

func TestGrokSSOImportPriorityDefaultAndExplicitZero(t *testing.T) {
	testOAuthPriorityCases(t, func(t *testing.T, priority *int) int {
		oauthService := service.NewGrokOAuthService(nil, priorityGrokOAuthClient{})
		defer oauthService.Stop()
		adminService := newStubAdminService()
		handler := NewGrokOAuthHandler(oauthService, adminService, nil, nil)
		router := gin.New()
		router.POST("/grok/sso-to-oauth", handler.CreateAccountsFromSSO)
		body := map[string]any{"sso_tokens": []string{"sso-token"}}
		if priority != nil {
			body["priority"] = *priority
		}
		postPriorityJSON(t, router, "/grok/sso-to-oauth", body)
		require.Len(t, adminService.createdAccounts, 1)
		return adminService.createdAccounts[0].Priority
	})
}

func testOAuthPriorityCases(t *testing.T, run func(*testing.T, *int) int) {
	t.Helper()
	zero := 0
	for _, tt := range []struct {
		name     string
		priority *int
		want     int
	}{
		{name: "omitted", want: 10},
		{name: "explicit zero", priority: &zero, want: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, run(t, tt.priority))
		})
	}
}

func postPriorityJSON(t *testing.T, router http.Handler, path string, body map[string]any) {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
}
