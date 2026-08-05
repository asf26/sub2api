//go:build unit

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLocalImageRouteServesGeneratedImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	storage, err := repository.NewLocalImageStorage(root, "/v1/images/files", 48*time.Hour, time.Hour)
	require.NoError(t, err)
	imageURL, err := storage.Save(context.Background(), "images/task.png", "image/png", []byte("image-data"))
	require.NoError(t, err)

	router := gin.New()
	RegisterLocalImageRoutes(router, config.ImageStorageConfig{
		LocalDirectory: root,
		LocalBaseURL:   "/v1/images/files",
	})

	req := httptest.NewRequest(http.MethodGet, imageURL, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "image-data", response.Body.String())
	require.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
	require.Contains(t, response.Header().Get("Cache-Control"), "immutable")
}

func TestLocalImageRouteRejectsNonImagesAndTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o640))
	router := gin.New()
	RegisterLocalImageRoutes(router, config.ImageStorageConfig{LocalDirectory: root})

	for _, requestPath := range []string{
		"/v1/images/files/secret.txt",
		"/v1/images/files/%2e%2e/secret.txt",
		"/v1/images/files/images/missing.png",
	} {
		req := httptest.NewRequest(http.MethodGet, requestPath, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		require.NotEqual(t, http.StatusOK, response.Code, "path=%s", requestPath)
	}
}
