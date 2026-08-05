# Findings

- The current dev proxy points `/api-proxy` to `http://127.0.0.1:8789/v1`.
- The Node async service accepts image generation, image edits, and responses endpoints, returns `202` with `data.task_id`, and exposes task polling at `/v1/images/tasks/{task_id}`.
- The service strips `stream` and `partial_images` from JSON bodies by default to avoid streaming responses through the async worker.
- Redis is available locally at `127.0.0.1:6379`.
- The Go async service health check at `http://127.0.0.1:8789/health` reports Redis `ok` and upstream `https://api.gavinteam.online/v1`.
- The Go service now defaults `ASYNC_IMAGE_UPSTREAM_STREAM=false`, so the forced `/responses + image_generation` bridge uses non-streaming upstream calls unless explicitly enabled.
- Completed Images-shaped base64 results are written to `ASYNC_IMAGE_FILE_STORE_DIR` and task results keep `data[].url`; Redis stores task metadata/result URLs rather than large base64 image payloads.
- Temporary image files use `ASYNC_IMAGE_FILE_TTL_MS`, defaulting to the task TTL of 1 hour.
- Real service logs showed one non-streaming `/responses` request failing with `unexpected EOF` after about 60 seconds and the next failing during TLS setup with `net/http: TLS handshake timeout` after 30 seconds.
- Local curl to `https://api.gavinteam.online/v1/models` completed TLS in about 0.4-0.7 seconds and returned 401 without a key, so the TLS timeout is likely transient upstream/network behavior rather than a permanently wrong domain.
- Go async worker now treats TLS handshake timeout, EOF, 502/503/504/524, and similar transport failures as retryable; non-streaming primary requests can fall back to internal streaming.
