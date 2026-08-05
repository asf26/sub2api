package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type XiaoVideoHandler struct {
	service *service.XiaoVideoService
	openAI  *OpenAIGatewayHandler
}

const xiaoVideoEnabledContextKey = "xiao_video_enabled"

func NewXiaoVideoHandler(svc *service.XiaoVideoService, openAI *OpenAIGatewayHandler) *XiaoVideoHandler {
	return &XiaoVideoHandler{service: svc, openAI: openAI}
}

func (h *XiaoVideoHandler) EnabledFor(c *gin.Context) bool {
	owner, ok := videoOwnerFromContext(c)
	enabled := ok && h != nil && h.service != nil && h.service.ActiveForGroup(c.Request.Context(), owner.GroupID)
	if enabled {
		c.Set(xiaoVideoEnabledContextKey, true)
	}
	return enabled
}

func (h *XiaoVideoHandler) Models(c *gin.Context) {
	owner, ok := h.requireEnabledOwner(c)
	if !ok {
		return
	}
	models, err := h.service.ListModels(c.Request.Context(), owner)
	if err != nil {
		videoError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": models})
}

func (h *XiaoVideoHandler) Upload(c *gin.Context) {
	owner, ok := h.requireEnabledOwner(c)
	if !ok {
		return
	}
	contentType := c.GetHeader("Content-Type")
	if !validVideoUploadContentType(contentType) {
		videoError(c, service.ErrVideoRequestInvalid)
		return
	}
	media, err := h.service.Upload(c.Request.Context(), owner, c.Request.Body, contentType)
	if err != nil {
		videoError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"media_id":   media.MediaID,
		"url":        h.service.PublicBaseURL() + "/v1/videos/uploads/" + media.MediaID + "/content",
		"type":       media.MediaType,
		"expires_at": media.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *XiaoVideoHandler) Create(c *gin.Context) {
	owner, ok := h.requireEnabledOwner(c)
	if !ok {
		return
	}
	if !validVideoJSONContentType(c.GetHeader("Content-Type")) {
		videoError(c, service.ErrVideoRequestInvalid)
		return
	}
	if !videoPreferRespondAsync(c.GetHeader("Prefer")) {
		videoError(c, infraerrors.New(http.StatusBadRequest, "ASYNC_REQUIRED", "Prefer: respond-async is required"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 2<<20))
	if err != nil || len(body) == 0 {
		videoError(c, service.ErrVideoRequestInvalid)
		return
	}
	if !h.checkSecurityAudit(c, body) {
		return
	}
	job, err := h.service.Create(c.Request.Context(), owner, body, c.GetHeader("Idempotency-Key"))
	if err != nil {
		videoError(c, err)
		return
	}
	location := "/v1/videos/jobs/" + job.JobID
	c.Header("Preference-Applied", "respond-async")
	c.Header("Location", location)
	c.JSON(http.StatusAccepted, gin.H{"job_id": job.JobID, "status": job.Status, "status_url": location})
}

func (h *XiaoVideoHandler) Get(c *gin.Context) {
	owner, ok := h.requireOwner(c)
	if !ok {
		return
	}
	job, err := h.service.Get(c.Request.Context(), owner, c.Param("job_id"))
	if err != nil {
		videoError(c, err)
		return
	}
	c.JSON(http.StatusOK, publicVideoJob(job))
}

func (h *XiaoVideoHandler) List(c *gin.Context) {
	owner, ok := h.requireOwner(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	jobs, err := h.service.List(c.Request.Context(), owner, limit)
	if err != nil {
		videoError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(jobs))
	for _, job := range jobs {
		data = append(data, publicVideoJob(job))
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

func (h *XiaoVideoHandler) Cancel(c *gin.Context) {
	owner, ok := h.requireOwner(c)
	if !ok {
		return
	}
	job, err := h.service.Cancel(c.Request.Context(), owner, c.Param("job_id"))
	if err != nil {
		videoError(c, err)
		return
	}
	c.JSON(http.StatusOK, publicVideoJob(job))
}

func (h *XiaoVideoHandler) Content(c *gin.Context) {
	owner, ok := h.requireOwner(c)
	if !ok {
		return
	}
	resp, err := h.service.OpenContent(c.Request.Context(), owner, c.Param("job_id"), c.GetHeader("Range"), c.Request.URL.RawQuery)
	if err != nil {
		videoError(c, err)
		return
	}
	proxyVideoResponse(c, resp, true)
}

func (h *XiaoVideoHandler) MediaContent(c *gin.Context) {
	owner, ok := h.requireOwner(c)
	if !ok {
		return
	}
	resp, err := h.service.OpenMedia(c.Request.Context(), owner, c.Param("media_id"), c.GetHeader("Range"), c.Request.URL.RawQuery)
	if err != nil {
		videoError(c, err)
		return
	}
	proxyVideoResponse(c, resp, true)
}

func (h *XiaoVideoHandler) requireOwner(c *gin.Context) (service.VideoOwner, bool) {
	owner, ok := videoOwnerFromContext(c)
	if !ok {
		videoError(c, infraerrors.New(http.StatusUnauthorized, "API_KEY_REQUIRED", "API key is required"))
		return service.VideoOwner{}, false
	}
	if h == nil || h.service == nil {
		videoError(c, service.ErrVideoExecutionDisabled)
		return service.VideoOwner{}, false
	}
	return owner, true
}

func (h *XiaoVideoHandler) requireEnabledOwner(c *gin.Context) (service.VideoOwner, bool) {
	owner, ok := h.requireOwner(c)
	if !ok {
		return service.VideoOwner{}, false
	}
	if !h.service.Enabled() {
		videoError(c, service.ErrVideoExecutionDisabled)
		return service.VideoOwner{}, false
	}
	enabled, _ := c.Get(xiaoVideoEnabledContextKey)
	if enabled != true && !h.service.ActiveForGroup(c.Request.Context(), owner.GroupID) {
		videoError(c, service.ErrVideoGenerationDisabled)
		return service.VideoOwner{}, false
	}
	return owner, true
}

func videoOwnerFromContext(c *gin.Context) (service.VideoOwner, bool) {
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key == nil || key.ID <= 0 || key.UserID <= 0 {
		return service.VideoOwner{}, false
	}
	return service.VideoOwner{UserID: key.UserID, APIKeyID: key.ID, GroupID: key.GroupID}, true
}

func (h *XiaoVideoHandler) checkSecurityAudit(c *gin.Context, body []byte) bool {
	if h == nil || h.openAI == nil {
		return true
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		videoError(c, infraerrors.New(http.StatusUnauthorized, "API_KEY_REQUIRED", "API key is required"))
		return false
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		videoError(c, infraerrors.New(http.StatusInternalServerError, "USER_CONTEXT_REQUIRED", "user context is required"))
		return false
	}
	var request struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(body, &request)
	reqLog := requestLogger(c, "handler.xiao_video.security_audit",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID), zap.String("model", request.Model))
	decision := h.openAI.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, request.Model, body)
	if decision != nil && !decision.AllowNextStage {
		h.openAI.openAISecurityAuditError(c, decision)
		return false
	}
	return true
}

func publicVideoJob(job *service.VideoJob) map[string]any {
	out := map[string]any{
		"job_id":       job.JobID,
		"status":       job.Status,
		"model":        job.Model,
		"resolution":   job.Resolution,
		"duration":     job.Duration,
		"aspect_ratio": job.AspectRatio,
		"amount":       strconv.FormatFloat(job.Amount, 'f', 8, 64),
		"currency":     job.Currency,
		"created_at":   job.CreatedAt.Format(time.RFC3339),
		"updated_at":   job.UpdatedAt.Format(time.RFC3339),
		"status_url":   "/v1/videos/jobs/" + job.JobID,
	}
	if job.Status == "completed" {
		out["content_url"] = "/v1/videos/jobs/" + job.JobID + "/content"
	}
	if job.Status == "failed" {
		out["error"] = map[string]any{"code": "VIDEO_GENERATION_FAILED", "message": "video generation failed"}
	}
	return out
}

func proxyVideoResponse(c *gin.Context, resp *http.Response, stream bool) {
	if resp == nil {
		videoError(c, service.ErrVideoUpstreamUnavailable)
		return
	}
	defer resp.Body.Close()
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Content-Disposition", "Cache-Control", "ETag", "Last-Modified"} {
		if value := resp.Header.Get(name); value != "" {
			c.Header(name, value)
		}
	}
	c.Status(resp.StatusCode)
	if stream {
		_, _ = io.CopyBuffer(c.Writer, resp.Body, make([]byte, 64*1024))
		return
	}
	_, _ = io.Copy(c.Writer, io.LimitReader(resp.Body, 2<<20))
}

func videoError(c *gin.Context, err error) {
	var upstream *service.VideoUpstreamError
	if errors.As(err, &upstream) {
		if requestID := upstream.Header.Get("X-Request-Id"); requestID != "" {
			c.Header("X-Request-Id", requestID)
		}
		if retryAfter := upstream.Header.Get("Retry-After"); retryAfter != "" {
			c.Header("Retry-After", retryAfter)
		}
		status := upstream.Status
		if status == http.StatusPaymentRequired || status == http.StatusUnauthorized || status == http.StatusForbidden || status >= 500 {
			status = http.StatusServiceUnavailable
		}
		if status <= 0 {
			status = http.StatusBadGateway
		}
		code, message := "VIDEO_UPSTREAM_ERROR", "video upstream request failed"
		if safeCode, safeMessage, ok := safeVideoUpstreamError(upstream.Status, upstream.Body); ok {
			code, message = safeCode, safeMessage
		}
		c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "code": code, "message": message}})
		return
	}
	status := infraerrors.Code(err)
	code := infraerrors.Reason(err)
	message := infraerrors.Message(err)
	if status == 0 {
		status = http.StatusInternalServerError
	}
	if strings.TrimSpace(code) == "" {
		code = "INTERNAL_ERROR"
	}
	if strings.TrimSpace(message) == "" {
		message = "internal error"
	}
	c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "code": code, "message": message}})
}

func validVideoJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func validVideoUploadContentType(value string) bool {
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(value))
	return err == nil && strings.EqualFold(mediaType, "multipart/form-data") && strings.TrimSpace(params["boundary"]) != ""
}

func videoPreferRespondAsync(value string) bool {
	for _, preference := range strings.Split(value, ",") {
		name := strings.TrimSpace(strings.SplitN(preference, ";", 2)[0])
		if strings.EqualFold(name, "respond-async") {
			return true
		}
	}
	return false
}

func safeVideoUpstreamError(status int, body []byte) (string, string, bool) {
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return "", "", false
	}
	type publicError struct {
		status  int
		message string
	}
	allowed := map[string]publicError{
		"VIDEO_REQUEST_INVALID":      {http.StatusBadRequest, "video request is invalid"},
		"VIDEO_RESOURCE_NOT_FOUND":   {http.StatusNotFound, "video resource not found"},
		"VIDEO_JOB_NOT_CANCELABLE":   {http.StatusConflict, "video job is not cancelable"},
		"VIDEO_MODEL_INVALID":        {http.StatusUnprocessableEntity, "model is not supported"},
		"VIDEO_PROMPT_INVALID":       {http.StatusUnprocessableEntity, "prompt is invalid"},
		"VIDEO_RESOLUTION_INVALID":   {http.StatusUnprocessableEntity, "resolution is not supported by this model"},
		"VIDEO_DURATION_INVALID":     {http.StatusUnprocessableEntity, "duration is not supported by this model"},
		"VIDEO_ASPECT_RATIO_INVALID": {http.StatusUnprocessableEntity, "aspect ratio is not supported by this model"},
		"VIDEO_MEDIA_INVALID":        {http.StatusUnprocessableEntity, "video media is invalid"},
		"VIDEO_OPTION_UNSUPPORTED":   {http.StatusUnprocessableEntity, "video option is not supported by this model"},
		"VIDEO_CAPACITY_EXHAUSTED":   {http.StatusTooManyRequests, "video capacity is temporarily exhausted"},
	}
	definition, ok := allowed[strings.TrimSpace(envelope.Error.Code)]
	if !ok || definition.status != status {
		return "", "", false
	}
	return strings.TrimSpace(envelope.Error.Code), definition.message, true
}
