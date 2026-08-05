package repository

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	defaultLocalImageDirectory = "./data/image-storage"
	defaultLocalImageBaseURL   = "/v1/images/files"
)

type LocalImageStorage struct {
	root            string
	baseURL         string
	retention       time.Duration
	cleanupInterval time.Duration

	cleanupMu   sync.Mutex
	lastCleanup time.Time
}

var _ service.ImageStorage = (*LocalImageStorage)(nil)

func NewLocalImageStorage(directory, baseURL string, retention, cleanupInterval time.Duration) (*LocalImageStorage, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory = defaultLocalImageDirectory
	}
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve local image directory: %w", err)
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create local image directory: %w", err)
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultLocalImageBaseURL
	}
	if retention <= 0 {
		retention = 48 * time.Hour
	}
	if cleanupInterval <= 0 {
		cleanupInterval = time.Hour
	}
	return &LocalImageStorage{
		root:            root,
		baseURL:         baseURL,
		retention:       retention,
		cleanupInterval: cleanupInterval,
	}, nil
}

func (s *LocalImageStorage) Save(ctx context.Context, key, _ string, data []byte) (string, error) {
	if s == nil {
		return "", errors.New("local image storage is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	relative, err := validateLocalImageKey(key)
	if err != nil {
		return "", err
	}
	target, err := localImageTarget(s.root, relative)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return "", fmt.Errorf("create local image parent directory: %w", err)
	}

	tmp, err := os.CreateTemp(parent, ".image-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create local image temp file: %w", err)
	}
	tmpName := tmp.Name()
	removeTemp := true
	defer func() {
		_ = tmp.Close()
		if removeTemp {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o640); err != nil {
		return "", fmt.Errorf("set local image permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return "", fmt.Errorf("write local image: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("sync local image: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close local image: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, target); err != nil {
		return "", fmt.Errorf("publish local image: %w", err)
	}
	removeTemp = false

	s.maybeCleanup(ctx)
	return localImageURL(s.baseURL, relative), nil
}

func (s *LocalImageStorage) CleanupExpired(ctx context.Context, now time.Time) error {
	if s == nil || s.retention <= 0 {
		return nil
	}
	cutoff := now.Add(-s.retention)
	return filepath.WalkDir(s.root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		return nil
	})
}

func (s *LocalImageStorage) maybeCleanup(ctx context.Context) {
	if s.cleanupInterval <= 0 || !s.cleanupMu.TryLock() {
		return
	}
	defer s.cleanupMu.Unlock()
	now := time.Now()
	if !s.lastCleanup.IsZero() && now.Sub(s.lastCleanup) < s.cleanupInterval {
		return
	}
	if s.CleanupExpired(ctx, now) == nil {
		s.lastCleanup = now
	}
}

func validateLocalImageKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return "", errors.New("invalid local image key")
	}
	cleaned := path.Clean(key)
	if cleaned == "." || cleaned != key || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("invalid local image key")
	}
	return cleaned, nil
}

func localImageTarget(root, relative string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("local image path escapes storage root")
	}
	return target, nil
}

func localImageURL(baseURL, relative string) string {
	segments := strings.Split(relative, "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.Join(segments, "/")
}
