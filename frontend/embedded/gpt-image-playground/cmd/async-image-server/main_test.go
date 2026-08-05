package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type unexpectedEOFReader struct {
	reader *strings.Reader
}

const tinyPngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII="

func (r *unexpectedEOFReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err == io.EOF {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}

func TestMaybeRewriteJSONBodyRemovesStreamingFields(t *testing.T) {
	body := []byte(`{"stream":true,"prompt":"x","nested":{"partial_images":2,"items":[{"stream":true,"keep":1}]}}`)
	rewritten := prepareUpstreamBody(body, "application/json; charset=utf-8", true, false, "/v1/images/generations")

	var payload map[string]any
	if err := json.Unmarshal(rewritten, &payload); err != nil {
		t.Fatalf("rewritten body is not JSON: %v", err)
	}
	if _, ok := payload["stream"]; ok {
		t.Fatal("top-level stream was not removed")
	}
	nested := payload["nested"].(map[string]any)
	if _, ok := nested["partial_images"]; ok {
		t.Fatal("nested partial_images was not removed")
	}
	item := nested["items"].([]any)[0].(map[string]any)
	if _, ok := item["stream"]; ok {
		t.Fatal("array item stream was not removed")
	}
	if item["keep"].(float64) != 1 {
		t.Fatal("non-streaming fields should be preserved")
	}
}

func TestBuildUpstreamURLPreservesV1BaseAndQuery(t *testing.T) {
	incoming, err := url.Parse("/v1/images/generations?mode=b64")
	if err != nil {
		t.Fatal(err)
	}
	target, err := buildUpstreamURL("https://api.gavinteam.online/v1", incoming)
	if err != nil {
		t.Fatal(err)
	}
	if target != "https://api.gavinteam.online/v1/images/generations?mode=b64" {
		t.Fatalf("unexpected upstream URL: %s", target)
	}
}

func TestMatchTaskPath(t *testing.T) {
	if got := matchTaskPath("/v1/images/tasks/img_123"); got != "img_123" {
		t.Fatalf("unexpected task id: %q", got)
	}
	if got := matchTaskPath("/images/tasks/img_456"); got != "img_456" {
		t.Fatalf("unexpected task id: %q", got)
	}
}

func TestPrepareUpstreamBodyEnablesImagesStreaming(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"x"}`)
	rewritten := prepareUpstreamBody(body, "application/json", true, true, "/v1/images/generations")

	var payload map[string]any
	if err := json.Unmarshal(rewritten, &payload); err != nil {
		t.Fatalf("rewritten body is not JSON: %v", err)
	}
	if payload["stream"] != true {
		t.Fatalf("stream should be true, got %#v", payload["stream"])
	}
	if payload["partial_images"].(float64) != 1 {
		t.Fatalf("partial_images should default to 1, got %#v", payload["partial_images"])
	}
}

func TestLoadConfigDefaultsToNonStreamingUpstream(t *testing.T) {
	t.Setenv("ASYNC_IMAGE_UPSTREAM_STREAM", "")
	cfg := loadConfig()
	if cfg.UpstreamStream {
		t.Fatal("upstream streaming should default to false")
	}
}

func TestResponsesJSONToImagesJSON(t *testing.T) {
	body, err := responsesJSONToImagesJSON([]byte(`{"output":[{"type":"image_generation_call","result":"ZmluYWw=","revised_prompt":"rewritten","size":"1024x1024"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	data := payload["data"].([]any)
	item := data[0].(map[string]any)
	if item["b64_json"] != "ZmluYWw=" {
		t.Fatalf("unexpected b64: %#v", item["b64_json"])
	}
	if item["revised_prompt"] != "rewritten" {
		t.Fatalf("unexpected revised prompt: %#v", item["revised_prompt"])
	}
}

func TestUpstreamRequestBodiesAddsStreamFallback(t *testing.T) {
	srv := &server{cfg: config{StreamFallback: true, UpstreamStream: false}}
	body := []byte(`{"model":"gpt-image-2","input":"prompt","tools":[{"type":"image_generation"}],"tool_choice":"required"}`)
	job := queuedJob{
		UpstreamURL: "https://api.example.com/v1/responses",
		Headers:     map[string]string{"Content-Type": "application/json"},
	}

	attempts := srv.upstreamRequestBodies(job, body)
	if len(attempts) != 2 {
		t.Fatalf("expected primary plus stream fallback, got %d", len(attempts))
	}
	if attempts[0].Mode != "primary" || attempts[1].Mode != "stream-fallback" {
		t.Fatalf("unexpected modes: %#v", attempts)
	}

	var payload map[string]any
	if err := json.Unmarshal(attempts[1].Body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["stream"] != true {
		t.Fatalf("fallback body should enable stream, got %#v", payload["stream"])
	}
	tool := payload["tools"].([]any)[0].(map[string]any)
	if tool["partial_images"].(float64) != 1 {
		t.Fatalf("fallback body should enable partial images, got %#v", tool["partial_images"])
	}
}

func TestRetryableErrorClassification(t *testing.T) {
	if !isRetryableErrorMessage(t.Context(), `Post "https://api.example.com/v1/responses": net/http: TLS handshake timeout`) {
		t.Fatal("TLS handshake timeout should be retryable")
	}
	if !isRetryableErrorMessage(t.Context(), "读取上游响应失败: unexpected EOF") {
		t.Fatal("unexpected EOF should be retryable")
	}
	if !isRetryableHTTPStatus(524) || !isRetryableHTTPStatus(http.StatusBadGateway) {
		t.Fatal("524 and 502 should be retryable")
	}
	if isRetryableHTTPStatus(http.StatusUnauthorized) || isRetryableHTTPStatus(http.StatusBadRequest) {
		t.Fatal("400/401 should not be retryable")
	}
}

func TestMaterializeImageResultStoresFiles(t *testing.T) {
	dir := t.TempDir()
	srv := &server{cfg: config{
		Host:         "127.0.0.1",
		Port:         8789,
		FileStoreDir: dir,
		FileTTL:      time.Hour,
	}}
	body := []byte(`{"created":1,"data":[{"b64_json":"` + tinyPngBase64 + `","output_format":"png","revised_prompt":"ok"}]}`)

	result, err := srv.materializeImageResult("img_test", body, "http://example.test")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatal(err)
	}
	item := payload["data"].([]any)[0].(map[string]any)
	if _, exists := item["b64_json"]; exists {
		t.Fatal("b64_json should not be stored in the task result")
	}
	if item["url"] != "http://example.test/v1/images/tasks/img_test/files/000.png" {
		t.Fatalf("unexpected file URL: %#v", item["url"])
	}
	if _, err := os.Stat(dir + "/img_test/000.png"); err != nil {
		t.Fatalf("expected image file to be written: %v", err)
	}
}

func TestMatchTaskFilePath(t *testing.T) {
	ref, ok := matchTaskFilePath("/v1/images/tasks/img_123/files/000.png")
	if !ok {
		t.Fatal("expected task file path to match")
	}
	if ref.TaskID != "img_123" || ref.Filename != "000.png" {
		t.Fatalf("unexpected file ref: %#v", ref)
	}
	if _, ok := matchTaskFilePath("/v1/images/tasks/img_123/files/../secret"); ok {
		t.Fatal("unsafe file path should not match")
	}
}

func TestHandleTaskFileServesImage(t *testing.T) {
	dir := t.TempDir()
	srv := &server{cfg: config{FileStoreDir: dir, FileTTL: time.Hour}}
	if err := os.MkdirAll(dir+"/img_test", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/img_test/000.png", []byte("png-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/images/tasks/img_test/files/000.png", nil)
	rr := httptest.NewRecorder()
	srv.handleTaskFile(rr, req, taskFileRef{TaskID: "img_test", Filename: "000.png"})

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("unexpected content-type: %s", ct)
	}
	if rr.Body.String() != "png-bytes" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestCleanupExpiredFiles(t *testing.T) {
	dir := t.TempDir()
	taskDir := dir + "/img_old"
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(taskDir, old, old); err != nil {
		t.Fatal(err)
	}

	srv := &server{cfg: config{FileStoreDir: dir, FileTTL: time.Hour}}
	srv.cleanupExpiredFiles()
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Fatalf("expected expired task dir to be removed, stat err=%v", err)
	}
}

func TestImagesStreamToJSON(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"cGFydGlhbA=="}`,
		"",
		`data: {"type":"image_generation.completed","b64_json":"ZmluYWw=","size":"1024x1024","quality":"high","output_format":"png"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	body, err := eventStreamToJSON("https://api.example.com/v1/images/generations", "images", strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	data := payload["data"].([]any)
	item := data[0].(map[string]any)
	if item["b64_json"] != "ZmluYWw=" {
		t.Fatalf("unexpected final b64: %#v", item["b64_json"])
	}
}

func TestResponsesStreamToImagesJSON(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"response.completed","response":{"output":[{"type":"image_generation_call","result":"ZmluYWw=","revised_prompt":"rewritten","size":"1024x1024"}]}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	body, err := eventStreamToJSON("https://api.example.com/v1/responses", "images", strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	data := payload["data"].([]any)
	item := data[0].(map[string]any)
	if item["b64_json"] != "ZmluYWw=" {
		t.Fatalf("unexpected final b64: %#v", item["b64_json"])
	}
	if item["revised_prompt"] != "rewritten" {
		t.Fatalf("unexpected revised prompt: %#v", item["revised_prompt"])
	}
}

func TestResponsesStreamToImagesJSONFallsBackToLastPartialImage(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"response.image_generation_call.partial_image","partial_image_index":0,"partial_image_b64":"cGFydGlhbA==","output_format":"png"}`,
		"",
		`data: {"type":"response.image_generation_call.partial_image","partial_image_index":1,"partial_image_b64":"ZmluYWw=","output_format":"png"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")

	body, err := eventStreamToJSON("https://api.example.com/v1/responses", "images", strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	data := payload["data"].([]any)
	item := data[0].(map[string]any)
	if item["b64_json"] != "ZmluYWw=" {
		t.Fatalf("expected last partial image as fallback, got %#v", item["b64_json"])
	}
}

func TestResponsesStreamToImagesJSONUsesPartialImageAfterIdle(t *testing.T) {
	reader, writer := io.Pipe()
	go func() {
		_, _ = writer.Write([]byte(strings.Join([]string{
			`data: {"type":"response.image_generation_call.partial_image","partial_image_index":0,"partial_image_b64":"ZmluYWw=","output_format":"png"}`,
			"",
		}, "\n")))
	}()
	defer reader.Close()
	defer writer.Close()

	body, err := eventStreamReadCloserToJSON("https://api.example.com/v1/responses", "images", reader, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ZmluYWw=") {
		t.Fatalf("expected partial image after idle timeout, got %s", string(body))
	}
}

func TestResponsesStreamIdleIgnoresHeartbeatLines(t *testing.T) {
	reader, writer := io.Pipe()
	go func() {
		_, _ = writer.Write([]byte(strings.Join([]string{
			`data: {"type":"response.image_generation_call.partial_image","partial_image_index":0,"partial_image_b64":"ZmluYWw=","output_format":"png"}`,
			"",
		}, "\n")))
		for i := 0; i < 5; i++ {
			time.Sleep(5 * time.Millisecond)
			_, _ = writer.Write([]byte(": keep-alive\n"))
		}
	}()
	defer reader.Close()
	defer writer.Close()

	body, err := eventStreamReadCloserToJSON("https://api.example.com/v1/responses", "images", reader, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ZmluYWw=") {
		t.Fatalf("expected partial image despite heartbeat lines, got %s", string(body))
	}
}

func TestResponsesStreamToImagesJSONToleratesUnexpectedEOF(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"response.completed","response":{"output":[{"type":"image_generation_call","result":"ZmluYWw="}]}}`,
		"",
	}, "\n")

	body, err := eventStreamToJSON("https://api.example.com/v1/responses", "images", &unexpectedEOFReader{reader: strings.NewReader(stream)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ZmluYWw=") {
		t.Fatalf("expected final image in payload: %s", string(body))
	}
}
