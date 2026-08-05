package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicVideoJobDoesNotLeakUpstreamIdentifiers(t *testing.T) {
	now := time.Now().UTC()
	job := &service.VideoJob{
		JobID:            "vidjob_public",
		UpstreamJobID:    "secret-upstream-job",
		AccountID:        999,
		Model:            "video-public",
		Status:           "failed",
		Amount:           1.25,
		Currency:         "USD",
		CreatedAt:        now,
		UpdatedAt:        now,
		UpstreamResponse: []byte(`{"job_id":"secret-upstream-job","internal_job_id":"internal-secret","account_id":999,"error":{"message":"provider https://secret.example failed"}}`),
	}
	raw, err := json.Marshal(publicVideoJob(job))
	require.NoError(t, err)
	response := string(raw)
	require.NotContains(t, response, "secret-upstream-job")
	require.NotContains(t, response, "internal-secret")
	require.NotContains(t, response, "secret.example")
	require.NotContains(t, response, "account_id")
	require.Contains(t, response, "VIDEO_GENERATION_FAILED")
}

func TestVideoErrorSanitizesUpstreamBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	videoError(ctx, &service.VideoUpstreamError{
		Status: http.StatusBadRequest,
		Header: http.Header{"X-Request-Id": []string{"request-safe"}},
		Body:   []byte(`{"error":{"message":"account 999 at https://secret.example rejected upstream job abc"}}`),
	})
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "request-safe", recorder.Header().Get("X-Request-Id"))
	require.NotContains(t, recorder.Body.String(), "secret.example")
	require.NotContains(t, recorder.Body.String(), "account 999")
	require.True(t, strings.Contains(recorder.Body.String(), "VIDEO_UPSTREAM_ERROR"))
}

func TestVideoErrorPreservesDocumentedCodeWithoutLeakingMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	videoError(ctx, &service.VideoUpstreamError{
		Status: http.StatusUnprocessableEntity,
		Header: make(http.Header),
		Body:   []byte(`{"error":{"code":"VIDEO_RESOLUTION_INVALID","message":"secret provider at https://internal.example rejected account 9"}}`),
	})
	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	require.Contains(t, recorder.Body.String(), "VIDEO_RESOLUTION_INVALID")
	require.Contains(t, recorder.Body.String(), "resolution is not supported by this model")
	require.NotContains(t, recorder.Body.String(), "internal.example")
	require.NotContains(t, recorder.Body.String(), "account 9")
}

func TestVideoRequestHeaderValidation(t *testing.T) {
	require.True(t, validVideoJSONContentType("application/json; charset=utf-8"))
	require.False(t, validVideoJSONContentType("text/plain"))
	require.True(t, validVideoUploadContentType("multipart/form-data; boundary=upload-boundary"))
	require.False(t, validVideoUploadContentType("multipart/form-data"))
	require.True(t, videoPreferRespondAsync("wait=5, respond-async"))
	require.False(t, videoPreferRespondAsync("not-respond-async"))
}

func TestProxyVideoResponseForwardsRangeHeadersAndStreams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	upstream := &http.Response{
		StatusCode: http.StatusPartialContent,
		Header: http.Header{
			"Content-Type":   []string{"video/mp4"},
			"Content-Length": []string{"4"},
			"Content-Range":  []string{"bytes 0-3/10"},
			"Accept-Ranges":  []string{"bytes"},
		},
		Body: io.NopCloser(strings.NewReader("data")),
	}

	proxyVideoResponse(ctx, upstream, true)

	require.Equal(t, http.StatusPartialContent, recorder.Code)
	require.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	require.Equal(t, "4", recorder.Header().Get("Content-Length"))
	require.Equal(t, "bytes 0-3/10", recorder.Header().Get("Content-Range"))
	require.Equal(t, "bytes", recorder.Header().Get("Accept-Ranges"))
	require.Equal(t, "data", recorder.Body.String())
}
