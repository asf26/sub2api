package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

type config struct {
	Host                  string
	Port                  int
	UpstreamBaseURL       string
	UpstreamAPIKey        string
	ResponsesModel        string
	RequestTimeout        time.Duration
	TaskTTL               time.Duration
	FileTTL               time.Duration
	Concurrency           int
	ForceNonStream        bool
	UpstreamStream        bool
	ForceResponses        bool
	UpstreamHTTP2         bool
	NodeFetchPrimary      bool
	NodeFetchFallback     bool
	NodeCommand           string
	DisableKeepAlive      bool
	DialTimeout           time.Duration
	TLSHandshakeTimeout   time.Duration
	StreamIdleTimeout     time.Duration
	UpstreamRetryAttempts int
	StreamFallback        bool
	MaxBodyBytes          int64
	FileStoreDir          string
	PublicBaseURL         string
	RedisAddr             string
	RedisPassword         string
	RedisDB               int
	RedisKeyPrefix        string
}

type server struct {
	cfg        config
	rdb        *redis.Client
	httpClient *http.Client
	active     int64
}

type queuedJob struct {
	ID            string            `json:"id"`
	Method        string            `json:"method"`
	UpstreamURL   string            `json:"upstream_url"`
	Headers       map[string]string `json:"headers"`
	BodyBase64    string            `json:"body_base64,omitempty"`
	CreatedAt     int64             `json:"created_at"`
	ResultShape   string            `json:"result_shape,omitempty"`
	PublicBaseURL string            `json:"public_base_url,omitempty"`
}

type taskState struct {
	TaskID    string          `json:"task_id"`
	Status    string          `json:"status"`
	CreatedAt int64           `json:"created_at,omitempty"`
	UpdatedAt int64           `json:"updated_at,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *taskError      `json:"error,omitempty"`
}

type taskError struct {
	Message string `json:"message"`
}

type taskResponse struct {
	Data taskState `json:"data"`
}

type upstreamRequestBody struct {
	Body []byte
	Mode string
}

type nodeFetchResult struct {
	Status     int               `json:"status"`
	Headers    map[string]string `json:"headers"`
	BodyBase64 string            `json:"bodyBase64"`
	Error      string            `json:"error"`
}

func main() {
	cfg := loadConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Redis is not reachable yet: %v", err)
	}

	app := &server{
		cfg:        cfg,
		rdb:        rdb,
		httpClient: newHTTPClient(cfg),
	}
	app.startWorkers(ctx)
	app.startFileCleanup(ctx)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:           app,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("Async image Go server listening at http://%s:%d", cfg.Host, cfg.Port)
		log.Printf("Provider settings: http://%s:%d/provider-settings.json", cfg.Host, cfg.Port)
		log.Printf("Upstream image API: %s", cfg.UpstreamBaseURL)
		log.Printf("Redis queue: %s keyPrefix=%s concurrency=%d forceResponses=%t upstreamStream=%t streamFallback=%t retryAttempts=%d upstreamHTTP2=%t disableKeepAlive=%t nodeFetchPrimary=%t nodeFetchFallback=%t", cfg.RedisAddr, cfg.RedisKeyPrefix, cfg.Concurrency, cfg.ForceResponses, cfg.UpstreamStream, cfg.StreamFallback, cfg.UpstreamRetryAttempts, cfg.UpstreamHTTP2, cfg.DisableKeepAlive, cfg.NodeFetchPrimary, cfg.NodeFetchFallback)
		log.Printf("Image file store: %s ttl=%s publicBase=%s", cfg.FileStoreDir, cfg.FileTTL, firstNonEmpty(cfg.PublicBaseURL, "request-host"))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = rdb.Close()
}

func loadConfig() config {
	taskTTL := getenvDurationMs("ASYNC_IMAGE_TASK_TTL_MS", time.Hour)
	return config{
		Host:                  getenvString("ASYNC_IMAGE_SERVER_HOST", "127.0.0.1"),
		Port:                  getenvInt("ASYNC_IMAGE_SERVER_PORT", 8789),
		UpstreamBaseURL:       normalizeBaseURL(getenvString("ASYNC_IMAGE_UPSTREAM_URL", "https://api.gavinteam.online/v1")),
		UpstreamAPIKey:        strings.TrimSpace(os.Getenv("ASYNC_IMAGE_UPSTREAM_API_KEY")),
		ResponsesModel:        strings.TrimSpace(getenvString("ASYNC_IMAGE_RESPONSES_MODEL", "gpt-5.4")),
		RequestTimeout:        getenvDurationMs("ASYNC_IMAGE_REQUEST_TIMEOUT_MS", 60*time.Minute),
		TaskTTL:               taskTTL,
		FileTTL:               getenvDurationMs("ASYNC_IMAGE_FILE_TTL_MS", taskTTL),
		Concurrency:           max(1, getenvInt("ASYNC_IMAGE_CONCURRENCY", 3)),
		ForceNonStream:        getenvBool("ASYNC_IMAGE_FORCE_NON_STREAM", true),
		UpstreamStream:        getenvBool("ASYNC_IMAGE_UPSTREAM_STREAM", false),
		ForceResponses:        getenvBool("ASYNC_IMAGE_FORCE_RESPONSES", false),
		UpstreamHTTP2:         getenvBool("ASYNC_IMAGE_UPSTREAM_HTTP2", false),
		NodeFetchPrimary:      getenvBool("ASYNC_IMAGE_NODE_FETCH_PRIMARY", true),
		NodeFetchFallback:     getenvBool("ASYNC_IMAGE_NODE_FETCH_FALLBACK", true),
		NodeCommand:           getenvString("ASYNC_IMAGE_NODE_COMMAND", "node"),
		DisableKeepAlive:      getenvBool("ASYNC_IMAGE_UPSTREAM_DISABLE_KEEPALIVE", true),
		DialTimeout:           getenvDurationMs("ASYNC_IMAGE_DIAL_TIMEOUT_MS", 30*time.Second),
		TLSHandshakeTimeout:   getenvDurationMs("ASYNC_IMAGE_TLS_HANDSHAKE_TIMEOUT_MS", 30*time.Second),
		StreamIdleTimeout:     getenvDurationMs("ASYNC_IMAGE_STREAM_IDLE_TIMEOUT_MS", 20*time.Second),
		UpstreamRetryAttempts: max(1, getenvInt("ASYNC_IMAGE_UPSTREAM_RETRY_ATTEMPTS", 3)),
		StreamFallback:        getenvBool("ASYNC_IMAGE_STREAM_FALLBACK", true),
		MaxBodyBytes:          int64(max(1, getenvInt("ASYNC_IMAGE_MAX_BODY_MB", 60))) * 1024 * 1024,
		FileStoreDir:          getenvString("ASYNC_IMAGE_FILE_STORE_DIR", filepath.Join(os.TempDir(), "gpt-image-playground-async-images")),
		PublicBaseURL:         normalizeBaseURL(os.Getenv("ASYNC_IMAGE_PUBLIC_BASE_URL")),
		RedisAddr:             getenvString("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:         os.Getenv("REDIS_PASSWORD"),
		RedisDB:               getenvInt("REDIS_DB", 0),
		RedisKeyPrefix:        strings.Trim(getenvString("ASYNC_IMAGE_REDIS_PREFIX", "async-image"), ":"),
	}
}

func newHTTPClient(cfg config) *http.Client {
	dialer := &net.Dialer{
		Timeout:   cfg.DialTimeout,
		KeepAlive: 30 * time.Second,
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     cfg.UpstreamHTTP2,
			DisableKeepAlives:     cfg.DisableKeepAlive,
			MaxIdleConns:          max(1, cfg.Concurrency*2),
			MaxIdleConnsPerHost:   max(1, cfg.Concurrency),
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
			ResponseHeaderTimeout: cfg.RequestTimeout,
			ExpectContinueTimeout: time.Second,
		},
	}
}

func getenvString(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getenvDurationMs(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return time.Duration(value) * time.Millisecond
}

func getenvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func normalizeBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	appendCORS(w.Header())

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if req.Method == http.MethodGet && (req.URL.Path == "/" || req.URL.Path == "/health") {
		s.handleHealth(w, req)
		return
	}

	if req.Method == http.MethodGet && req.URL.Path == "/provider-settings.json" {
		writeJSON(w, http.StatusOK, s.providerSettings(req))
		return
	}

	if req.Method == http.MethodGet {
		if ref, ok := matchTaskFilePath(req.URL.Path); ok {
			s.handleTaskFile(w, req, ref)
			return
		}
		if taskID := matchTaskPath(req.URL.Path); taskID != "" {
			s.handleTaskPoll(w, req, taskID)
			return
		}
	}

	if req.Method == http.MethodPost && isSubmitPath(req.URL.Path) {
		s.handleSubmit(w, req)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "Not found"}})
}

func appendCORS(headers http.Header) {
	headers.Set("Access-Control-Allow-Origin", "*")
	headers.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key, X-Goog-API-Key")
	headers.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *server) handleHealth(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()

	queued, _ := s.rdb.LLen(ctx, s.queueKey()).Result()
	redisStatus := "ok"
	status := http.StatusOK
	ok := true
	if err := s.rdb.Ping(ctx).Err(); err != nil {
		redisStatus = err.Error()
		status = http.StatusServiceUnavailable
		ok = false
	}

	writeJSON(w, status, map[string]any{
		"ok":                  ok,
		"upstreamBaseUrl":     s.cfg.UpstreamBaseURL,
		"queued":              queued,
		"running":             atomic.LoadInt64(&s.active),
		"redis":               redisStatus,
		"providerSettingsUrl": fmt.Sprintf("http://%s/provider-settings.json", req.Host),
	})
}

func (s *server) handleTaskPoll(w http.ResponseWriter, req *http.Request, taskID string) {
	task, err := s.loadTask(req.Context(), taskID)
	if errors.Is(err, redis.Nil) {
		writeJSON(w, http.StatusNotFound, taskResponse{Data: taskState{
			TaskID: taskID,
			Status: "failed",
			Error:  &taskError{Message: "任务不存在或已过期"},
		}})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": err.Error()}})
		return
	}
	writeJSON(w, http.StatusOK, taskResponse{Data: task})
}

type taskFileRef struct {
	TaskID   string
	Filename string
}

func matchTaskFilePath(pathname string) (taskFileRef, bool) {
	for _, prefix := range []string{"/v1/images/tasks/", "/images/tasks/"} {
		if !strings.HasPrefix(pathname, prefix) {
			continue
		}
		rest := strings.TrimPrefix(pathname, prefix)
		taskID, filename, ok := strings.Cut(rest, "/files/")
		if !ok {
			continue
		}
		taskID, taskErr := url.PathUnescape(taskID)
		filename, fileErr := url.PathUnescape(filename)
		if taskErr != nil || fileErr != nil || !isSafePathSegment(taskID) || !isSafePathSegment(filename) {
			return taskFileRef{}, false
		}
		return taskFileRef{TaskID: taskID, Filename: filename}, true
	}
	return taskFileRef{}, false
}

func (s *server) handleTaskFile(w http.ResponseWriter, req *http.Request, ref taskFileRef) {
	if s.cfg.FileStoreDir == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "Image file store is disabled"}})
		return
	}

	dir := filepath.Join(s.cfg.FileStoreDir, ref.TaskID)
	if s.isExpiredTaskFileDir(dir) {
		_ = os.RemoveAll(dir)
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "图片文件已过期"}})
		return
	}

	pathname := filepath.Join(dir, ref.Filename)
	file, err := os.Open(pathname)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "图片文件不存在"}})
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "图片文件不存在"}})
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	if contentType := imageContentType(file, ref.Filename); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, req, ref.Filename, info.ModTime(), file)
}

func (s *server) handleSubmit(w http.ResponseWriter, req *http.Request) {
	rawBody, err := readRawBody(req, s.cfg.MaxBodyBytes)
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": map[string]any{"message": err.Error()}})
		return
	}

	contentType := req.Header.Get("Content-Type")
	upstreamURL, err := buildUpstreamURL(s.cfg.UpstreamBaseURL, req.URL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": err.Error()}})
		return
	}
	resultShape := resultShapeForPath(req.URL.Path)
	if s.cfg.ForceResponses && shouldForceResponses(req.URL.Path, contentType) {
		nextBody, transformErr := imagesGenerationBodyToResponsesBody(rawBody, s.cfg.UpstreamStream, s.cfg.ResponsesModel)
		if transformErr == nil {
			rawBody = nextBody
			upstreamURL, err = buildUpstreamURLForPath(s.cfg.UpstreamBaseURL, "/v1/responses", req.URL.RawQuery)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": err.Error()}})
				return
			}
			resultShape = "images"
		}
	}
	rawBody = prepareUpstreamBody(rawBody, contentType, s.cfg.ForceNonStream, s.cfg.UpstreamStream, req.URL.Path)

	now := time.Now().UnixMilli()
	task := taskState{
		TaskID:    "img_" + newUUID(),
		Status:    "queued",
		CreatedAt: now,
		UpdatedAt: now,
	}
	job := queuedJob{
		ID:            task.TaskID,
		Method:        req.Method,
		UpstreamURL:   upstreamURL,
		Headers:       s.forwardHeaders(req, contentType),
		BodyBase64:    base64.StdEncoding.EncodeToString(rawBody),
		CreatedAt:     now,
		ResultShape:   resultShape,
		PublicBaseURL: s.publicBaseURL(req),
	}

	ctx := req.Context()
	if err := s.saveTask(ctx, task); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]any{"message": "Redis 保存任务失败: " + err.Error()}})
		return
	}
	if err := s.saveJob(ctx, job); err != nil {
		_ = s.deleteTask(ctx, task.TaskID)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]any{"message": "Redis 保存队列任务失败: " + err.Error()}})
		return
	}
	if err := s.rdb.RPush(ctx, s.queueKey(), task.TaskID).Err(); err != nil {
		_ = s.deleteTask(ctx, task.TaskID)
		_ = s.deleteJob(ctx, task.TaskID)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": map[string]any{"message": "Redis 入队失败: " + err.Error()}})
		return
	}

	writeJSON(w, http.StatusAccepted, taskResponse{Data: task})
}

func readRawBody(req *http.Request, maxBytes int64) ([]byte, error) {
	defer req.Body.Close()
	body, err := io.ReadAll(io.LimitReader(req.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("请求体超过限制 %d MB", maxBytes/1024/1024)
	}
	return body, nil
}

func isSubmitPath(pathname string) bool {
	switch pathname {
	case "/v1/images/generations", "/images/generations",
		"/v1/images/edits", "/images/edits",
		"/v1/responses", "/responses":
		return true
	default:
		return false
	}
}

func matchTaskPath(pathname string) string {
	for _, prefix := range []string{"/v1/images/tasks/", "/images/tasks/"} {
		if strings.HasPrefix(pathname, prefix) {
			value, err := url.PathUnescape(strings.TrimPrefix(pathname, prefix))
			if err != nil {
				return ""
			}
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stripVersionPrefix(pathname string) string {
	return strings.TrimPrefix(pathname, "/v1")
}

func buildUpstreamURL(upstreamBaseURL string, incoming *url.URL) (string, error) {
	return buildUpstreamURLForPath(upstreamBaseURL, incoming.Path, incoming.RawQuery)
}

func buildUpstreamURLForPath(upstreamBaseURL string, pathname string, rawQuery string) (string, error) {
	target, err := url.Parse(upstreamBaseURL)
	if err != nil {
		return "", err
	}
	target.Path = strings.TrimRight(target.Path, "/") + stripVersionPrefix(pathname)
	target.RawQuery = rawQuery
	return target.String(), nil
}

func resultShapeForPath(pathname string) string {
	if strings.Contains(pathname, "/responses") {
		return "responses"
	}
	return "images"
}

func shouldForceResponses(pathname string, contentType string) bool {
	return strings.Contains(pathname, "/images/generations") &&
		strings.Contains(strings.ToLower(contentType), "application/json")
}

func imagesGenerationBodyToResponsesBody(rawBody []byte, upstreamStream bool, responsesModel string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, err
	}

	model := stringValue(payload["model"])
	targetModel := model
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-image-") && strings.TrimSpace(responsesModel) != "" {
		targetModel = strings.TrimSpace(responsesModel)
	}
	prompt := stringValue(payload["prompt"])
	if prompt == "" {
		return nil, errors.New("missing prompt")
	}

	tool := map[string]any{"type": "image_generation"}
	if model != "" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-image-") {
		tool["model"] = model
	}
	copyKnownFields(tool, payload, "size", "quality", "output_format", "output_compression", "moderation")
	if upstreamStream {
		tool["partial_images"] = firstNumberLike(payload["partial_images"], 1)
	}

	responses := map[string]any{
		"model":       targetModel,
		"input":       prompt,
		"tools":       []any{tool},
		"tool_choice": "required",
	}
	if upstreamStream {
		responses["stream"] = true
	}
	return json.Marshal(responses)
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func firstNumberLike(value any, fallback int) any {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return typed
		}
	case int:
		if typed > 0 {
			return typed
		}
	case string:
		if strings.TrimSpace(typed) != "" {
			return typed
		}
	}
	return fallback
}

func (s *server) forwardHeaders(req *http.Request, contentType string) map[string]string {
	headers := map[string]string{
		"Accept":     "*/*",
		"User-Agent": getenvString("ASYNC_IMAGE_UPSTREAM_USER_AGENT", "undici"),
	}
	if s.cfg.UpstreamAPIKey != "" {
		headers["Authorization"] = "Bearer " + s.cfg.UpstreamAPIKey
	} else if value := req.Header.Get("Authorization"); value != "" {
		headers["Authorization"] = value
	}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	for _, name := range []string{"X-API-Key", "X-Goog-API-Key"} {
		if value := req.Header.Get(name); value != "" {
			headers[name] = value
		}
	}
	return headers
}

func prepareUpstreamBody(rawBody []byte, contentType string, forceNonStream bool, upstreamStream bool, pathname string) []byte {
	if len(rawBody) == 0 || !strings.Contains(strings.ToLower(contentType), "application/json") {
		return rawBody
	}

	var payload any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return rawBody
	}

	if upstreamStream {
		enableStreamingFields(payload, pathname)
	} else if forceNonStream {
		removeStreamingFields(payload)
	}

	rewritten, err := json.Marshal(payload)
	if err != nil {
		return rawBody
	}
	return rewritten
}

func removeStreamingFields(value any) {
	switch typed := value.(type) {
	case map[string]any:
		delete(typed, "stream")
		delete(typed, "partial_images")
		for _, item := range typed {
			removeStreamingFields(item)
		}
	case []any:
		for _, item := range typed {
			removeStreamingFields(item)
		}
	}
}

func enableStreamingFields(value any, pathname string) {
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	object["stream"] = true

	if strings.Contains(pathname, "/images/generations") || strings.Contains(pathname, "/images/edits") {
		if _, exists := object["partial_images"]; !exists {
			object["partial_images"] = 1
		}
	}

	if strings.Contains(pathname, "/responses") {
		tools, ok := object["tools"].([]any)
		if !ok {
			return
		}
		for _, tool := range tools {
			toolObject, ok := tool.(map[string]any)
			if !ok || toolObject["type"] != "image_generation" {
				continue
			}
			if _, exists := toolObject["partial_images"]; !exists {
				toolObject["partial_images"] = 1
			}
		}
	}
}

func (s *server) startWorkers(ctx context.Context) {
	for i := 0; i < s.cfg.Concurrency; i++ {
		go s.worker(ctx, i+1)
	}
}

func (s *server) worker(ctx context.Context, workerID int) {
	for {
		values, err := s.rdb.BLPop(ctx, 5*time.Second, s.queueKey()).Result()
		if errors.Is(err, context.Canceled) {
			return
		}
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			log.Printf("worker %d Redis pop failed: %v", workerID, err)
			time.Sleep(time.Second)
			continue
		}
		if len(values) < 2 {
			continue
		}
		taskID := values[1]
		job, err := s.loadJob(ctx, taskID)
		if errors.Is(err, redis.Nil) {
			log.Printf("worker %d skipped missing job %s", workerID, taskID)
			continue
		}
		if err != nil {
			log.Printf("worker %d failed to load job %s: %v", workerID, taskID, err)
			continue
		}
		s.runTask(ctx, job)
	}
}

func (s *server) runTask(parent context.Context, job queuedJob) {
	atomic.AddInt64(&s.active, 1)
	defer atomic.AddInt64(&s.active, -1)
	defer func() {
		_ = s.deleteJob(context.Background(), job.ID)
	}()

	task, err := s.loadTask(parent, job.ID)
	if err != nil {
		log.Printf("task %s disappeared before running: %v", job.ID, err)
		return
	}
	task.Status = "running"
	task.UpdatedAt = time.Now().UnixMilli()
	task.Error = nil
	task.Result = nil
	_ = s.saveTask(parent, task)

	body, err := base64.StdEncoding.DecodeString(job.BodyBase64)
	if err != nil {
		s.failTask(parent, task, "队列任务体解析失败: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(parent, s.cfg.RequestTimeout)
	defer cancel()

	attemptBodies := s.upstreamRequestBodies(job, body)
	var lastMessage string
	for bodyIndex, attemptBody := range attemptBodies {
		for attempt := 1; attempt <= s.cfg.UpstreamRetryAttempts; attempt++ {
			result, retryable, message := s.performUpstreamAttempt(ctx, job, attemptBody, attempt, s.cfg.UpstreamRetryAttempts)
			if len(result) > 0 {
				s.completeTask(parent, task, job, result)
				return
			}
			lastMessage = message
			if !retryable {
				s.failTask(parent, task, message)
				return
			}
			s.httpClient.CloseIdleConnections()
			if bodyIndex == 0 && len(attemptBodies) > 1 {
				log.Printf("task %s switching to upstream %s after retryable failure: %s", job.ID, attemptBodies[1].Mode, message)
				break
			}
			if attempt < s.cfg.UpstreamRetryAttempts {
				if !sleepBeforeRetry(ctx, attempt) {
					s.failTask(parent, task, firstNonEmpty(lastMessage, "上游请求已取消"))
					return
				}
			}
		}
	}
	s.failTask(parent, task, firstNonEmpty(lastMessage, "请求上游失败"))
}

func (s *server) completeTask(ctx context.Context, task taskState, job queuedJob, result json.RawMessage) {
	materialized, err := s.materializeImageResult(task.TaskID, result, job.PublicBaseURL)
	if err != nil {
		s.failTask(ctx, task, "保存图片文件失败: "+err.Error())
		return
	}
	task.Status = "completed"
	task.UpdatedAt = time.Now().UnixMilli()
	task.Result = materialized
	task.Error = nil
	if err := s.saveTask(ctx, task); err != nil {
		log.Printf("failed to save completed task %s: %v", task.TaskID, err)
	}
}

func (s *server) upstreamRequestBodies(job queuedJob, body []byte) []upstreamRequestBody {
	attempts := []upstreamRequestBody{{Body: body, Mode: "primary"}}
	contentType := job.Headers["Content-Type"]
	if !s.cfg.StreamFallback || s.cfg.UpstreamStream || len(body) == 0 || !strings.Contains(strings.ToLower(contentType), "application/json") {
		return attempts
	}
	if jsonBodyHasStream(body) {
		return attempts
	}
	pathname := upstreamPathname(job.UpstreamURL)
	if !supportsStreamingPath(pathname) {
		return attempts
	}
	streamBody := prepareUpstreamBody(body, contentType, false, true, pathname)
	if bytes.Equal(streamBody, body) {
		return attempts
	}
	return append(attempts, upstreamRequestBody{Body: streamBody, Mode: "stream-fallback"})
}

func (s *server) performUpstreamAttempt(ctx context.Context, job queuedJob, attemptBody upstreamRequestBody, attempt int, maxAttempts int) (json.RawMessage, bool, string) {
	if s.cfg.NodeFetchPrimary {
		return s.performNodeFetchAttempt(ctx, job, attemptBody, attempt, maxAttempts)
	}

	request, err := http.NewRequestWithContext(ctx, job.Method, job.UpstreamURL, bytes.NewReader(attemptBody.Body))
	if err != nil {
		return nil, false, "创建上游请求失败: " + err.Error()
	}
	for key, value := range job.Headers {
		request.Header.Set(key, value)
	}
	if job.Method == http.MethodGet {
		request.Body = nil
		request.ContentLength = 0
	}

	startedAt := time.Now()
	bodySummary := safeBodySummary(attemptBody.Body, job.Headers["Content-Type"])
	response, err := s.httpClient.Do(request)
	if err != nil {
		log.Printf(
			"task %s upstream %s %s mode=%s attempt=%d/%d failed after %s auth=%s body=%s error=%s",
			job.ID,
			job.Method,
			job.UpstreamURL,
			attemptBody.Mode,
			attempt,
			maxAttempts,
			time.Since(startedAt).Round(time.Millisecond),
			safeAuthSummary(job.Headers),
			bodySummary,
			err.Error(),
		)
		message := formatRequestError(ctx, err)
		if s.cfg.NodeFetchFallback && isRetryableUpstreamError(ctx, err) {
			log.Printf(
				"task %s upstream %s %s mode=%s switching to node-fetch fallback after %s",
				job.ID,
				job.Method,
				job.UpstreamURL,
				attemptBody.Mode,
				err.Error(),
			)
			return s.performNodeFetchAttempt(ctx, job, attemptBody, attempt, maxAttempts)
		}
		return nil, isRetryableUpstreamError(ctx, err), message
	}
	defer response.Body.Close()
	log.Printf(
		"task %s upstream %s %s mode=%s attempt=%d/%d -> HTTP %d after %s auth=%s body=%s",
		job.ID,
		job.Method,
		job.UpstreamURL,
		attemptBody.Mode,
		attempt,
		maxAttempts,
		response.StatusCode,
		time.Since(startedAt).Round(time.Millisecond),
		safeAuthSummary(job.Headers),
		bodySummary,
	)

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		text, err := io.ReadAll(response.Body)
		if err != nil {
			message := "读取上游错误响应失败: " + err.Error()
			return nil, isRetryableReadError(ctx, err), message
		}
		message := formatUpstreamError(response.StatusCode, text)
		return nil, isRetryableHTTPStatus(response.StatusCode), message
	}

	if strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		result, err := eventStreamReadCloserToJSON(job.UpstreamURL, job.ResultShape, response.Body, s.cfg.StreamIdleTimeout)
		if err != nil {
			return nil, isRetryableErrorMessage(ctx, err.Error()), err.Error()
		}
		return result, false, ""
	}

	text, err := io.ReadAll(response.Body)
	if err != nil {
		message := "读取上游响应失败: " + err.Error()
		return nil, isRetryableReadError(ctx, err), message
	}
	if !json.Valid(text) {
		return nil, false, "上游未返回 JSON 图片结果"
	}

	result := json.RawMessage(text)
	if shouldConvertResponsesJSONToImages(job.UpstreamURL, job.ResultShape) {
		result, err = responsesJSONToImagesJSON(text)
		if err != nil {
			return nil, false, err.Error()
		}
	}
	return result, false, ""
}

func (s *server) performNodeFetchAttempt(ctx context.Context, job queuedJob, attemptBody upstreamRequestBody, attempt int, maxAttempts int) (json.RawMessage, bool, string) {
	script := `
const fs = require('node:fs');
const { setGlobalDispatcher, Agent } = require('undici');
setGlobalDispatcher(new Agent({
  connect: { timeout: 30000 },
  headersTimeout: 900000,
  bodyTimeout: 900000,
}));
function errorDetails(error) {
  const cause = error && error.cause ? error.cause : null;
  const parts = [error && error.message ? error.message : String(error)];
  if (cause) {
    if (cause.code) parts.push('cause.code=' + cause.code);
    if (cause.name) parts.push('cause.name=' + cause.name);
    if (cause.message) parts.push('cause.message=' + cause.message);
  }
  return parts.join(' | ');
}
(async () => {
  const input = JSON.parse(fs.readFileSync(0, 'utf8'));
  const options = {
    method: input.method,
    headers: input.headers || {},
  };
  if (input.method !== 'GET' && input.bodyBase64) {
    options.body = Buffer.from(input.bodyBase64, 'base64');
  }
  try {
    const response = await fetch(input.url, options);
    const headers = {};
    response.headers.forEach((value, key) => { headers[key] = value; });
    const body = Buffer.from(await response.arrayBuffer()).toString('base64');
    process.stdout.write(JSON.stringify({ status: response.status, headers, bodyBase64: body }));
  } catch (error) {
    process.stdout.write(JSON.stringify({ error: errorDetails(error) }));
  }
})();
`
	payload := map[string]any{
		"method":     job.Method,
		"url":        job.UpstreamURL,
		"headers":    job.Headers,
		"bodyBase64": base64.StdEncoding.EncodeToString(attemptBody.Body),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, false, "创建 node-fetch 兜底请求失败: " + err.Error()
	}

	startedAt := time.Now()
	cmd := exec.CommandContext(ctx, s.cfg.NodeCommand, "-e", script)
	cmd.Stdin = bytes.NewReader(payloadBytes)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, isRetryableUpstreamError(ctx, err), "node-fetch 兜底请求失败: " + err.Error() + ": " + string(exitErr.Stderr)
		}
		return nil, isRetryableUpstreamError(ctx, err), "node-fetch 兜底请求失败: " + err.Error()
	}

	var fetched nodeFetchResult
	if err := json.Unmarshal(output, &fetched); err != nil {
		return nil, false, "node-fetch 兜底响应解析失败: " + err.Error()
	}
	if fetched.Error != "" {
		return nil, isRetryableErrorMessage(ctx, fetched.Error), "node-fetch 兜底请求失败: " + fetched.Error
	}
	body, err := base64.StdEncoding.DecodeString(fetched.BodyBase64)
	if err != nil {
		return nil, false, "node-fetch 兜底响应解码失败: " + err.Error()
	}

	contentType := fetched.Headers["content-type"]
	bodySummary := safeBodySummary(attemptBody.Body, job.Headers["Content-Type"])
	log.Printf(
		"task %s upstream %s %s mode=%s/node-fetch attempt=%d/%d -> HTTP %d after %s auth=%s body=%s",
		job.ID,
		job.Method,
		job.UpstreamURL,
		attemptBody.Mode,
		attempt,
		maxAttempts,
		fetched.Status,
		time.Since(startedAt).Round(time.Millisecond),
		safeAuthSummary(job.Headers),
		bodySummary,
	)

	if fetched.Status < 200 || fetched.Status >= 300 {
		message := formatUpstreamError(fetched.Status, body)
		return nil, isRetryableHTTPStatus(fetched.Status), message
	}
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		result, err := eventStreamToJSON(job.UpstreamURL, job.ResultShape, bytes.NewReader(body))
		if err != nil {
			return nil, isRetryableErrorMessage(ctx, err.Error()), "解析上游流式响应失败: " + err.Error()
		}
		return result, false, ""
	}

	result := json.RawMessage(body)
	if shouldConvertResponsesJSONToImages(job.UpstreamURL, job.ResultShape) {
		converted, err := responsesJSONToImagesJSON(body)
		if err != nil {
			return nil, false, "解析 Responses 图片结果失败: " + err.Error()
		}
		result = converted
	}
	return result, false, ""
}

func upstreamPathname(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsedURL.Path
}

func supportsStreamingPath(pathname string) bool {
	return strings.Contains(pathname, "/responses") ||
		strings.Contains(pathname, "/images/generations") ||
		strings.Contains(pathname, "/images/edits")
}

func jsonBodyHasStream(body []byte) bool {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	stream, ok := payload["stream"].(bool)
	return ok && stream
}

func isRetryableHTTPStatus(status int) bool {
	return status == http.StatusRequestTimeout ||
		status == http.StatusTooManyRequests ||
		status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout ||
		status == 524 ||
		status >= 500 && status <= 599
}

func isRetryableUpstreamError(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return isRetryableErrorMessage(ctx, err.Error())
}

func isRetryableReadError(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	return isRetryableErrorMessage(ctx, err.Error())
}

func isRetryableErrorMessage(ctx context.Context, message string) bool {
	if ctx.Err() != nil {
		return false
	}
	text := strings.ToLower(message)
	for _, part := range []string{
		"tls handshake timeout",
		"unexpected eof",
		"connection reset",
		"connection refused",
		"broken pipe",
		"timeout",
		"temporary",
		"eof",
	} {
		if strings.Contains(text, part) {
			return true
		}
	}
	return false
}

func sleepBeforeRetry(ctx context.Context, attempt int) bool {
	delay := time.Duration(attempt) * 2 * time.Second
	if delay > 10*time.Second {
		delay = 10 * time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func eventStreamToJSON(upstreamURL string, resultShape string, reader io.Reader) (json.RawMessage, error) {
	events, err := readSSEEvents(reader)
	if err != nil {
		return nil, err
	}
	return eventsToJSON(upstreamURL, resultShape, events)
}

func eventStreamReadCloserToJSON(upstreamURL string, resultShape string, reader io.ReadCloser, idleTimeout time.Duration) (json.RawMessage, error) {
	events, err := readSSEEventsWithIdle(reader, idleTimeout)
	if err != nil {
		return nil, err
	}
	return eventsToJSON(upstreamURL, resultShape, events)
}

func eventsToJSON(upstreamURL string, resultShape string, events []map[string]any) (json.RawMessage, error) {
	parsedURL, _ := url.Parse(upstreamURL)
	pathname := ""
	if parsedURL != nil {
		pathname = parsedURL.Path
	}
	if strings.Contains(pathname, "/responses") && resultShape == "images" {
		return responsesStreamToImagesJSON(events)
	}
	if strings.Contains(pathname, "/responses") {
		return responsesStreamToJSON(events)
	}
	return imagesStreamToJSON(events)
}

func shouldConvertResponsesJSONToImages(upstreamURL string, resultShape string) bool {
	if resultShape != "images" {
		return false
	}
	parsedURL, err := url.Parse(upstreamURL)
	if err != nil {
		return false
	}
	return strings.Contains(parsedURL.Path, "/responses")
}

func responsesJSONToImagesJSON(body []byte) (json.RawMessage, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	output, ok := payload["output"].([]any)
	if !ok {
		return nil, errors.New("上游 Responses 接口未返回 output")
	}

	data := make([]map[string]any, 0, len(output))
	for _, item := range output {
		object, ok := item.(map[string]any)
		if !ok || object["type"] != "image_generation_call" || !hasImageGenerationResult(object) {
			continue
		}
		image := map[string]any{}
		if b64 := responseItemBase64(object); b64 != "" {
			image["b64_json"] = b64
		}
		copyKnownFields(image, object, "revised_prompt", "size", "quality", "output_format", "output_compression", "moderation")
		if len(image) > 0 {
			data = append(data, image)
		}
	}
	if len(data) == 0 {
		return nil, errors.New("上游 Responses 接口未返回最终图片数据")
	}

	result := map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
	}
	copyKnownFields(result, payload, "usage")
	return json.Marshal(result)
}

func readSSEEvents(reader io.Reader) ([]map[string]any, error) {
	buffered := bufio.NewReader(reader)
	var events []map[string]any
	var dataLines []string

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = nil
		if data == "" || data == "[DONE]" {
			return nil
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("上游流式响应包含无法解析的 JSON 事件: %w", err)
		}
		events = append(events, event)
		return nil
	}

	for {
		line, err := buffered.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				if flushErr := flush(); flushErr != nil {
					return nil, flushErr
				}
			} else if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			if flushErr := flush(); flushErr != nil {
				return nil, flushErr
			}
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取上游流式响应失败: %w", err)
		}
	}
	return events, nil
}

type sseLineResult struct {
	line string
	err  error
}

func readSSEEventsWithIdle(reader io.ReadCloser, idleTimeout time.Duration) ([]map[string]any, error) {
	if idleTimeout <= 0 {
		return readSSEEvents(reader)
	}

	buffered := bufio.NewReader(reader)
	lineCh := make(chan sseLineResult, 1)
	done := make(chan struct{})
	defer close(done)

	go func() {
		for {
			line, err := buffered.ReadString('\n')
			select {
			case lineCh <- sseLineResult{line: line, err: err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	var events []map[string]any
	var dataLines []string

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = nil
		if data == "" || data == "[DONE]" {
			return nil
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("上游流式响应包含无法解析的 JSON 事件: %w", err)
		}
		events = append(events, event)
		return nil
	}

	timer := time.NewTimer(idleTimeout)
	defer timer.Stop()
	resetTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(idleTimeout)
	}

	for {
		select {
		case item := <-lineCh:
			line := item.line
			err := item.err
			if len(line) > 0 {
				line = strings.TrimRight(line, "\r\n")
				if line == "" {
					if flushErr := flush(); flushErr != nil {
						return nil, flushErr
					}
				} else if strings.HasPrefix(line, "data:") {
					data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
					dataLines = append(dataLines, data)
					if data != "" && data != "[DONE]" {
						resetTimer()
					}
				}
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				if flushErr := flush(); flushErr != nil {
					return nil, flushErr
				}
				return events, nil
			}
			if err != nil {
				return nil, fmt.Errorf("读取上游流式响应失败: %w", err)
			}
		case <-timer.C:
			_ = reader.Close()
			if flushErr := flush(); flushErr != nil {
				return nil, flushErr
			}
			if len(events) > 0 {
				return events, nil
			}
			return nil, fmt.Errorf("读取上游流式响应空闲超时: %s", idleTimeout)
		}
	}
}

func imagesStreamToJSON(events []map[string]any) (json.RawMessage, error) {
	var completed []map[string]any
	var resultPayload map[string]any

	for _, event := range events {
		object, _ := event["object"].(string)
		if object == "image.generation.result" || object == "image.edit.result" {
			resultPayload = event
			continue
		}
		eventType, _ := event["type"].(string)
		if eventType == "image_generation.completed" || eventType == "image_edit.completed" {
			item := map[string]any{}
			copyKnownFields(item, event, "b64_json", "url", "revised_prompt", "size", "quality", "output_format", "output_compression", "moderation")
			completed = append(completed, item)
		}
	}

	if resultPayload != nil {
		body, err := json.Marshal(resultPayload)
		if err != nil {
			return nil, err
		}
		return body, nil
	}
	if len(completed) == 0 {
		return nil, errors.New("上游流式接口未返回最终图片数据")
	}
	body, err := json.Marshal(map[string]any{
		"created": time.Now().Unix(),
		"data":    completed,
	})
	if err != nil {
		return nil, err
	}
	return body, nil
}

func responsesStreamToJSON(events []map[string]any) (json.RawMessage, error) {
	var completedPayload map[string]any
	var outputItems []any

	for _, event := range events {
		if item, ok := event["item"].(map[string]any); ok && item["type"] == "image_generation_call" {
			if hasImageGenerationResult(item) {
				outputItems = append(outputItems, item)
			}
		}
		if response, ok := event["response"].(map[string]any); ok {
			completedPayload = response
		}
	}

	if len(outputItems) > 0 {
		body, err := json.Marshal(map[string]any{"output": outputItems})
		if err != nil {
			return nil, err
		}
		return body, nil
	}
	if partialItem := lastResponsesPartialImageItem(events); partialItem != nil {
		body, err := json.Marshal(map[string]any{"output": []any{partialItem}})
		if err != nil {
			return nil, err
		}
		return body, nil
	}
	if completedPayload != nil {
		body, err := json.Marshal(completedPayload)
		if err != nil {
			return nil, err
		}
		return body, nil
	}
	return nil, fmt.Errorf("上游流式接口未返回最终图片数据，事件类型: %s", eventTypesSummary(events))
}

func responsesStreamToImagesJSON(events []map[string]any) (json.RawMessage, error) {
	output, err := collectResponsesOutputItems(events)
	if err != nil {
		return nil, err
	}

	data := make([]map[string]any, 0, len(output))
	for _, item := range output {
		image := map[string]any{}
		if b64 := responseItemBase64(item); b64 != "" {
			image["b64_json"] = b64
		}
		copyKnownFields(image, item, "revised_prompt", "size", "quality", "output_format", "output_compression", "moderation")
		if len(image) > 0 {
			data = append(data, image)
		}
	}
	if len(data) == 0 {
		return nil, errors.New("上游 Responses 流式接口未返回最终图片数据")
	}

	body, err := json.Marshal(map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
	})
	if err != nil {
		return nil, err
	}
	return body, nil
}

func collectResponsesOutputItems(events []map[string]any) ([]map[string]any, error) {
	var completedPayload map[string]any
	var outputItems []map[string]any

	for _, event := range events {
		if item, ok := event["item"].(map[string]any); ok && item["type"] == "image_generation_call" {
			if hasImageGenerationResult(item) {
				outputItems = append(outputItems, item)
			}
		}
		if response, ok := event["response"].(map[string]any); ok {
			completedPayload = response
		}
	}

	if len(outputItems) > 0 {
		return outputItems, nil
	}
	if partialItem := lastResponsesPartialImageItem(events); partialItem != nil {
		return []map[string]any{partialItem}, nil
	}
	if completedPayload == nil {
		return nil, fmt.Errorf("上游流式接口未返回最终图片数据，事件类型: %s", eventTypesSummary(events))
	}
	output, ok := completedPayload["output"].([]any)
	if !ok {
		return nil, errors.New("上游 Responses 流式接口未返回 output")
	}
	for _, item := range output {
		object, ok := item.(map[string]any)
		if ok && object["type"] == "image_generation_call" && hasImageGenerationResult(object) {
			outputItems = append(outputItems, object)
		}
	}
	if len(outputItems) == 0 {
		return nil, fmt.Errorf("上游 Responses 流式接口未返回最终图片数据，事件类型: %s", eventTypesSummary(events))
	}
	return outputItems, nil
}

func lastResponsesPartialImageItem(events []map[string]any) map[string]any {
	var last map[string]any
	for _, event := range events {
		image := map[string]any{}
		if b64 := firstNonEmpty(
			stringValue(event["partial_image_b64"]),
			stringValue(event["b64_json"]),
			stringValue(event["image"]),
			stringValue(event["data"]),
			responseItemBase64(event),
		); b64 != "" {
			image["result"] = b64
		}
		copyKnownFields(image, event, "revised_prompt", "size", "quality", "output_format", "output_compression", "moderation")
		eventType := stringValue(event["type"])
		if _, ok := image["result"]; ok && (strings.Contains(eventType, "image_generation") || eventType == "") {
			image["type"] = "image_generation_call"
			if _, exists := image["output_format"]; !exists {
				image["output_format"] = "png"
			}
			last = image
		}
	}
	return last
}

func eventTypesSummary(events []map[string]any) string {
	if len(events) == 0 {
		return "none"
	}
	counts := map[string]int{}
	order := []string{}
	for _, event := range events {
		eventType := firstNonEmpty(stringValue(event["type"]), stringValue(event["object"]), "unknown")
		if counts[eventType] == 0 {
			order = append(order, eventType)
		}
		counts[eventType]++
	}
	parts := make([]string, 0, len(order))
	for _, eventType := range order {
		parts = append(parts, fmt.Sprintf("%s(%d)", eventType, counts[eventType]))
	}
	return strings.Join(parts, ", ")
}

func responseItemBase64(item map[string]any) string {
	result := item["result"]
	switch value := result.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		for _, key := range []string{"b64_json", "base64", "image", "data"} {
			if text, ok := value[key].(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func (s *server) materializeImageResult(taskID string, result json.RawMessage, publicBaseURL string) (json.RawMessage, error) {
	if s.cfg.FileStoreDir == "" {
		return result, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		return nil, err
	}
	data, ok := payload["data"].([]any)
	if !ok {
		return result, nil
	}

	changed := false
	fileIndex := 0
	for _, item := range data {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		b64 := stringValue(object["b64_json"])
		if b64 == "" {
			continue
		}
		filename, err := s.writeTaskImageFile(taskID, fileIndex, b64, object)
		if err != nil {
			return nil, err
		}
		object["url"] = buildTaskFileURL(firstNonEmpty(publicBaseURL, s.cfg.PublicBaseURL, fmt.Sprintf("http://%s:%d", s.cfg.Host, s.cfg.Port)), taskID, filename)
		delete(object, "b64_json")
		changed = true
		fileIndex++
	}

	if !changed {
		return result, nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s *server) writeTaskImageFile(taskID string, index int, b64 string, item map[string]any) (string, error) {
	if !isSafePathSegment(taskID) {
		return "", errors.New("invalid task id")
	}
	imageBytes, mimeType, err := decodeImageBase64(b64)
	if err != nil {
		return "", err
	}
	ext := imageFileExtension(mimeType, item)
	filename := fmt.Sprintf("%03d.%s", index, ext)
	if !isSafePathSegment(filename) {
		return "", errors.New("invalid image filename")
	}

	dir := filepath.Join(s.cfg.FileStoreDir, taskID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmpPath := filepath.Join(dir, filename+".tmp")
	finalPath := filepath.Join(dir, filename)
	if err := os.WriteFile(tmpPath, imageBytes, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	now := time.Now()
	_ = os.Chtimes(finalPath, now, now)
	_ = os.Chtimes(dir, now, now)
	return filename, nil
}

func decodeImageBase64(value string) ([]byte, string, error) {
	raw := strings.TrimSpace(value)
	mimeType := ""
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		header, data, ok := strings.Cut(raw, ",")
		if !ok {
			return nil, "", errors.New("invalid data URL image")
		}
		raw = data
		meta := header
		if len(meta) >= len("data:") {
			meta = meta[len("data:"):]
		}
		if semi := strings.Index(meta, ";"); semi >= 0 {
			mimeType = strings.TrimSpace(meta[:semi])
		} else {
			mimeType = strings.TrimSpace(meta)
		}
	}

	imageBytes, err := decodeBase64String(raw)
	if err != nil {
		return nil, "", err
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(imageBytes)
	}
	return imageBytes, mimeType, nil
}

func decodeBase64String(value string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			return -1
		}
		return r
	}, value)
	if decoded, err := base64.StdEncoding.DecodeString(cleaned); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(cleaned); err == nil {
		return decoded, nil
	}
	return nil, errors.New("invalid base64 image data")
}

func imageFileExtension(mimeType string, item map[string]any) string {
	switch strings.Trim(strings.ToLower(stringValue(item["output_format"])), ".") {
	case "jpg", "jpeg":
		return "jpg"
	case "png":
		return "png"
	case "webp":
		return "webp"
	case "gif":
		return "gif"
	}

	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	}
	return "png"
}

func buildTaskFileURL(publicBaseURL string, taskID string, filename string) string {
	base := strings.TrimRight(publicBaseURL, "/")
	return base + "/v1/images/tasks/" + url.PathEscape(taskID) + "/files/" + url.PathEscape(filename)
}

func (s *server) publicBaseURL(req *http.Request) string {
	if s.cfg.PublicBaseURL != "" {
		return s.cfg.PublicBaseURL
	}
	scheme := firstForwardedValue(req.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if req.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := firstForwardedValue(req.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = req.Host
	}
	return normalizeBaseURL(scheme + "://" + host)
}

func firstForwardedValue(value string) string {
	part, _, _ := strings.Cut(value, ",")
	return strings.TrimSpace(part)
}

func isSafePathSegment(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, `/\`)
}

func hasImageGenerationResult(item map[string]any) bool {
	result := item["result"]
	switch value := result.(type) {
	case string:
		return strings.TrimSpace(value) != ""
	case map[string]any:
		for _, key := range []string{"b64_json", "base64", "image", "data"} {
			if text, ok := value[key].(string); ok && strings.TrimSpace(text) != "" {
				return true
			}
		}
	}
	return false
}

func copyKnownFields(target map[string]any, source map[string]any, keys ...string) {
	for _, key := range keys {
		if value, exists := source[key]; exists {
			target[key] = value
		}
	}
}

func formatRequestError(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "上游请求超时"
	}
	return "请求上游失败: " + err.Error()
}

func safeAuthSummary(headers map[string]string) string {
	value := strings.TrimSpace(headers["Authorization"])
	if value == "" {
		return "missing"
	}
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return fmt.Sprintf("bearer(len=%d)", len(strings.TrimSpace(value[len("bearer "):])))
	}
	return fmt.Sprintf("present(len=%d)", len(value))
}

func safeBodySummary(body []byte, contentType string) string {
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		if strings.TrimSpace(contentType) == "" {
			return "content-type=none"
		}
		return "content-type=" + strings.Split(contentType, ";")[0]
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "json=invalid"
	}
	streamValue := payload["stream"]
	summary := fmt.Sprintf("json(stream=%v", streamValue)
	if partialImages, exists := payload["partial_images"]; exists {
		summary += fmt.Sprintf(",partial_images=%v", partialImages)
	}
	if tools, ok := payload["tools"].([]any); ok {
		for _, tool := range tools {
			toolObject, ok := tool.(map[string]any)
			if !ok || toolObject["type"] != "image_generation" {
				continue
			}
			if partialImages, exists := toolObject["partial_images"]; exists {
				summary += fmt.Sprintf(",tool_partial_images=%v", partialImages)
			}
			break
		}
	}
	return summary + ")"
}

func formatUpstreamError(status int, body []byte) string {
	if message := extractErrorMessage(body); message != "" {
		return fmt.Sprintf("上游 HTTP %d: %s", status, message)
	}
	excerpt := strings.TrimSpace(string(body))
	if len(excerpt) > 500 {
		excerpt = excerpt[:500]
	}
	if excerpt != "" {
		return fmt.Sprintf("上游 HTTP %d: %s", status, excerpt)
	}
	return fmt.Sprintf("上游 HTTP %d", status)
}

func extractErrorMessage(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if message := stringAtPath(payload, "error", "message"); message != "" {
		return message
	}
	if message := stringAtPath(payload, "message"); message != "" {
		return message
	}
	if message := stringAtPath(payload, "error"); message != "" {
		return message
	}
	return ""
}

func stringAtPath(payload map[string]any, path ...string) string {
	var current any = payload
	for _, part := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[part]
	}
	value, ok := current.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func (s *server) failTask(ctx context.Context, task taskState, message string) {
	task.Status = "failed"
	task.UpdatedAt = time.Now().UnixMilli()
	task.Result = nil
	task.Error = &taskError{Message: message}
	if err := s.saveTask(ctx, task); err != nil {
		log.Printf("failed to save failed task %s: %v", task.TaskID, err)
	}
}

func imageContentType(file *os.File, filename string) string {
	if contentType := mime.TypeByExtension(filepath.Ext(filename)); contentType != "" {
		return contentType
	}

	var sample [512]byte
	n, err := file.Read(sample[:])
	if err == nil {
		_, _ = file.Seek(0, io.SeekStart)
		return http.DetectContentType(sample[:n])
	}
	_, _ = file.Seek(0, io.SeekStart)
	return ""
}

func (s *server) startFileCleanup(ctx context.Context) {
	if s.cfg.FileStoreDir == "" {
		return
	}
	if err := os.MkdirAll(s.cfg.FileStoreDir, 0o755); err != nil {
		log.Printf("failed to create image file store %s: %v", s.cfg.FileStoreDir, err)
		return
	}

	interval := s.cfg.FileTTL / 4
	if interval < time.Minute {
		interval = time.Minute
	}
	if interval > 15*time.Minute {
		interval = 15 * time.Minute
	}

	go func() {
		s.cleanupExpiredFiles()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanupExpiredFiles()
			}
		}
	}()
}

func (s *server) cleanupExpiredFiles() {
	if s.cfg.FileStoreDir == "" || s.cfg.FileTTL <= 0 {
		return
	}
	entries, err := os.ReadDir(s.cfg.FileStoreDir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("failed to read image file store %s: %v", s.cfg.FileStoreDir, err)
		}
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(s.cfg.FileStoreDir, entry.Name())
		if s.isExpiredTaskFileDir(dir) {
			if err := os.RemoveAll(dir); err != nil {
				log.Printf("failed to remove expired image files %s: %v", dir, err)
			}
		}
	}
}

func (s *server) isExpiredTaskFileDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	if s.cfg.FileTTL <= 0 {
		return false
	}
	return time.Since(info.ModTime()) > s.cfg.FileTTL
}

func (s *server) queueKey() string {
	return s.key("queue")
}

func (s *server) taskKey(taskID string) string {
	return s.key("task", taskID)
}

func (s *server) jobKey(taskID string) string {
	return s.key("job", taskID)
}

func (s *server) key(parts ...string) string {
	prefix := s.cfg.RedisKeyPrefix
	if prefix == "" {
		prefix = "async-image"
	}
	return prefix + ":" + strings.Join(parts, ":")
}

func (s *server) saveTask(ctx context.Context, task taskState) error {
	body, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, s.taskKey(task.TaskID), body, s.cfg.TaskTTL).Err()
}

func (s *server) loadTask(ctx context.Context, taskID string) (taskState, error) {
	body, err := s.rdb.Get(ctx, s.taskKey(taskID)).Bytes()
	if err != nil {
		return taskState{}, err
	}
	var task taskState
	if err := json.Unmarshal(body, &task); err != nil {
		return taskState{}, err
	}
	return task, nil
}

func (s *server) deleteTask(ctx context.Context, taskID string) error {
	return s.rdb.Del(ctx, s.taskKey(taskID)).Err()
}

func (s *server) saveJob(ctx context.Context, job queuedJob) error {
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, s.jobKey(job.ID), body, s.cfg.TaskTTL).Err()
}

func (s *server) loadJob(ctx context.Context, taskID string) (queuedJob, error) {
	body, err := s.rdb.Get(ctx, s.jobKey(taskID)).Bytes()
	if err != nil {
		return queuedJob{}, err
	}
	var job queuedJob
	if err := json.Unmarshal(body, &job); err != nil {
		return queuedJob{}, err
	}
	return job, nil
}

func (s *server) deleteJob(ctx context.Context, taskID string) error {
	return s.rdb.Del(ctx, s.jobKey(taskID)).Err()
}

func (s *server) providerSettings(req *http.Request) map[string]any {
	baseURL := fmt.Sprintf("http://%s/v1", req.Host)
	return map[string]any{
		"customProviders": []map[string]any{
			{
				"id":   "local-async-openai",
				"name": "本地异步 OpenAI 中转",
				"submit": map[string]any{
					"path":        "images/generations",
					"method":      "POST",
					"contentType": "json",
					"body": map[string]string{
						"model":              "$profile.model",
						"prompt":             "$prompt",
						"size":               "$params.size",
						"quality":            "$params.quality",
						"output_format":      "$params.output_format",
						"moderation":         "$params.moderation",
						"output_compression": "$params.output_compression",
						"n":                  "$params.n",
					},
					"taskIdPath": "data.task_id",
				},
				"editSubmit": map[string]any{
					"path":        "images/edits",
					"method":      "POST",
					"contentType": "multipart",
					"body": map[string]string{
						"model":              "$profile.model",
						"prompt":             "$prompt",
						"size":               "$params.size",
						"quality":            "$params.quality",
						"output_format":      "$params.output_format",
						"moderation":         "$params.moderation",
						"output_compression": "$params.output_compression",
						"n":                  "$params.n",
					},
					"files": []map[string]any{
						{"field": "image[]", "source": "inputImages", "array": true},
						{"field": "mask", "source": "mask"},
					},
					"taskIdPath": "data.task_id",
				},
				"poll": map[string]any{
					"path":            "images/tasks/{task_id}",
					"method":          "GET",
					"intervalSeconds": 3,
					"statusPath":      "data.status",
					"successValues":   []string{"completed"},
					"failureValues":   []string{"failed", "cancelled"},
					"errorPath":       "data.error.message",
					"result": map[string]any{
						"imageUrlPaths": []string{"data.result.data.*.url"},
						"b64JsonPaths":  []string{"data.result.data.*.b64_json"},
					},
				},
			},
		},
		"profiles": []map[string]any{
			{
				"id":           "local-async-openai-profile",
				"name":         "本地异步中转",
				"provider":     "local-async-openai",
				"baseUrl":      baseURL,
				"apiKey":       "",
				"model":        "gpt-image-2",
				"apiMode":      "images",
				"timeout":      30,
				"apiProxy":     false,
				"streamImages": false,
			},
		},
	}
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
