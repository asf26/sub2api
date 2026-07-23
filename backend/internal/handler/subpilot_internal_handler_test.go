package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSubPilotProbeRequiresSharedSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSubPilotInternalHandler(&service.AccountTestService{}, &config.Config{
		Gateway: config.GatewayConfig{SubPilot: config.SubPilotConfig{SharedSecret: "shared-secret"}},
	})
	router := gin.New()
	router.POST("/probe/:id", h.ProbeAccount)

	req := httptest.NewRequest(http.MethodPost, "/probe/not-an-id", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-SubPilot-Secret", "shared-secret")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	req = httptest.NewRequest(http.MethodPost, "/probe/not-an-id", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestSubPilotProbeRejectsEmptyConfiguredSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSubPilotInternalHandler(&service.AccountTestService{}, &config.Config{})
	router := gin.New()
	router.POST("/probe/:id", h.ProbeAccount)

	req := httptest.NewRequest(http.MethodPost, "/probe/11", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusUnauthorized, resp.Code)
}
