package routes

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
)

const defaultLocalImageRoute = "/v1/images/files"

func RegisterLocalImageRoutes(r *gin.Engine, cfg config.ImageStorageConfig) {
	if r == nil {
		return
	}
	root := strings.TrimSpace(cfg.LocalDirectory)
	if root == "" {
		root = "./data/image-storage"
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return
	}
	routePath := localImageRoutePath(cfg.LocalBaseURL)
	handler := serveLocalImage(root)
	r.GET(routePath+"/*filepath", handler)
	r.HEAD(routePath+"/*filepath", handler)
}

func localImageRoutePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultLocalImageRoute
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Path != "" {
		raw = parsed.Path
	}
	raw = "/" + strings.Trim(strings.ReplaceAll(raw, "\\", "/"), "/")
	for _, segment := range strings.Split(raw, "/") {
		if segment == ".." {
			return defaultLocalImageRoute
		}
	}
	cleaned := path.Clean(raw)
	if cleaned == "/" {
		return defaultLocalImageRoute
	}
	return cleaned
}

func serveLocalImage(root string) gin.HandlerFunc {
	return func(c *gin.Context) {
		relative := strings.TrimPrefix(c.Param("filepath"), "/")
		if !validLocalImageRequestPath(relative) {
			c.Status(http.StatusNotFound)
			return
		}
		target := filepath.Join(root, filepath.FromSlash(relative))
		if !pathWithinRoot(root, target) {
			c.Status(http.StatusNotFound)
			return
		}
		info, err := os.Lstat(target)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			c.Status(http.StatusNotFound)
			return
		}
		resolved, err := filepath.EvalSymlinks(target)
		if err != nil || !pathWithinRoot(root, resolved) {
			c.Status(http.StatusNotFound)
			return
		}
		file, err := os.Open(resolved)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer func() { _ = file.Close() }()

		c.Header("Cache-Control", "public, max-age=3600, immutable")
		c.Header("X-Content-Type-Options", "nosniff")
		http.ServeContent(c.Writer, c.Request, filepath.Base(resolved), info.ModTime(), file)
	}
}

func validLocalImageRequestPath(relative string) bool {
	if relative == "" || strings.Contains(relative, "\\") {
		return false
	}
	cleaned := path.Clean(relative)
	if cleaned != relative || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return false
	}
	switch strings.ToLower(filepath.Ext(cleaned)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return true
	default:
		return false
	}
}

func pathWithinRoot(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
