//go:build unit

package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLocalImageStorageSavesPersistentFileAndReturnsSameOriginURL(t *testing.T) {
	root := t.TempDir()
	storage, err := NewLocalImageStorage(root, "/v1/images/files", 48*time.Hour, time.Hour)
	require.NoError(t, err)

	url, err := storage.Save(context.Background(), "images/task one.png", "image/png", []byte("png-data"))
	require.NoError(t, err)
	require.Equal(t, "/v1/images/files/images/task%20one.png", url)

	data, err := os.ReadFile(filepath.Join(root, "images", "task one.png"))
	require.NoError(t, err)
	require.Equal(t, []byte("png-data"), data)
}

func TestLocalImageStorageRejectsUnsafeKeys(t *testing.T) {
	storage, err := NewLocalImageStorage(t.TempDir(), "", time.Hour, time.Hour)
	require.NoError(t, err)

	for _, key := range []string{"", "/absolute.png", "../escape.png", "images/../escape.png", `images\\escape.png`} {
		_, err := storage.Save(context.Background(), key, "image/png", []byte("x"))
		require.Error(t, err, "key=%q", key)
	}
}

func TestLocalImageStorageCleansExpiredFiles(t *testing.T) {
	root := t.TempDir()
	storage, err := NewLocalImageStorage(root, "", time.Hour, time.Hour)
	require.NoError(t, err)

	oldPath := filepath.Join(root, "images", "old.png")
	freshPath := filepath.Join(root, "images", "fresh.png")
	require.NoError(t, os.MkdirAll(filepath.Dir(oldPath), 0o750))
	require.NoError(t, os.WriteFile(oldPath, []byte("old"), 0o640))
	require.NoError(t, os.WriteFile(freshPath, []byte("fresh"), 0o640))
	now := time.Now()
	require.NoError(t, os.Chtimes(oldPath, now.Add(-2*time.Hour), now.Add(-2*time.Hour)))

	require.NoError(t, storage.CleanupExpired(context.Background(), now))
	require.NoFileExists(t, oldPath)
	require.FileExists(t, freshPath)
}

func TestImageStorageFactoryFallsBackToLocalWhenS3IsOff(t *testing.T) {
	root := t.TempDir()
	factory := ProvideImageStorageFactory(&config.Config{ImageStorage: config.ImageStorageConfig{
		LocalDirectory: root,
		LocalBaseURL:   "/v1/images/files",
		LocalRetention: 48,
		LocalCleanup:   60,
	}})

	storage, err := factory(context.Background(), &config.ImageStorageConfig{Enabled: false})
	require.NoError(t, err)
	_, ok := storage.(*LocalImageStorage)
	require.True(t, ok)
}

func TestImageStorageFactoryUsesS3WhenEnabledAndConfigured(t *testing.T) {
	factory := ProvideImageStorageFactory(&config.Config{})
	storage, err := factory(context.Background(), &config.ImageStorageConfig{
		Enabled: true, Bucket: "images", AccessKeyID: "key", SecretAccessKey: "secret", Region: "auto",
	})
	require.NoError(t, err)
	_, ok := storage.(*S3ImageStorage)
	require.True(t, ok)
}
