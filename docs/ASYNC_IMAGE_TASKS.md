# Asynchronous Image Tasks

Asynchronous image tasks let clients submit long-running OpenAI-compatible image requests without keeping one HTTP connection open. This avoids proxy/CDN response timeouts such as Cloudflare 524 while preserving the existing image routing, billing, moderation, concurrency, and failover behavior.

## Endpoints

The authenticated gateway exposes both `/v1` paths and their existing no-prefix aliases:

```text
POST /v1/images/generations/async
POST /v1/images/edits/async
GET  /v1/images/tasks/{task_id}
```

The aliases are `/images/generations/async`, `/images/edits/async`, and `/images/tasks/{task_id}`.

Only OpenAI and Grok groups are supported. Requests use the same JSON or multipart payload as the corresponding synchronous endpoint. Streaming image requests are rejected because a polled task returns one final JSON result.

## Persistent image storage

Asynchronous image tasks use persistent storage so large `b64_json` results never accumulate in Redis. Server-local storage is the default. S3 is optional: when S3 is disabled or its configuration is incomplete, generated images are saved under the server data directory and returned through a same-origin URL. When S3 is enabled and fully configured, generated images are stored in S3 instead.

### From the admin UI (recommended)

**Admin -> Backup -> Async image storage.** The **Use S3** switch only selects the storage backend; it does not disable asynchronous image tasks. Saving the form takes effect immediately and does not require a container restart.

When S3 is enabled, the form can reuse the database backup S3 configuration or use separate credentials. Backups and images remain separated by their prefixes.

Saving requires step-up 2FA when that gate is enabled, for the same reason the backup S3 form does: changing the target redirects generated content to another account.

Turning the switch off makes new images use the server's local persistent storage. Existing tasks and previously returned URLs remain unaffected.

### From the config file

The admin setting takes precedence. When nothing has ever been saved there, the `image_storage` block in `config.yaml` is used instead, so deployments that enabled the feature before the admin UI existed keep working untouched.

The local storage defaults are suitable for Docker deployments because `/app/data` is a persistent volume:

```yaml
image_storage:
  enabled: false
  local_directory: "./data/image-storage"
  local_base_url: "/v1/images/files"
  local_retention_hours: 48
  local_cleanup_interval_minutes: 60
  max_download_bytes: 33554432
```

`local_directory` must be writable by the application process. Files older than `local_retention_hours` are removed during periodic cleanup.

To use an S3-compatible object store (AWS S3, Cloudflare R2, Aliyun OSS, or MinIO), enable it in `config.yaml` and supply the S3 fields (all keys also accept the `IMAGE_STORAGE_*` environment overrides):

```yaml
image_storage:
  enabled: true
  endpoint: "https://<account_id>.r2.cloudflarestorage.com"  # AWS 官方可留空
  region: "auto"
  bucket: "my-images"
  access_key_id: "..."
  secret_access_key: "..."
  prefix: "images/"
  force_path_style: false          # MinIO/path-style buckets set true
  public_base_url: ""              # set to return public_base_url/key直链; empty → presigned URL
  presign_expiry_hours: 24         # presigned link TTL when public_base_url is empty
  max_download_bytes: 33554432     # cap when re-hosting an upstream image URL (32MB)
```

When a task completes, each generated image is saved to the active storage backend and the result is rewritten to a compact form. With local storage, `data[].url` looks like `/v1/images/files/images/imgtask_...png`; with S3 it is a public or presigned object URL. In both cases `b64_json` is removed and only the small JSON result is stored in Redis. If storage fails, the task is marked `failed` rather than persisting raw base64.

To support another storage backend, implement the `service.ImageStorage` interface (`Save(ctx, key, contentType, data) (url, error)`) and provide it through the storage factory.

### Troubleshooting: storage is unavailable

`404 async image tasks are not enabled` now means neither local nor S3 storage could be initialized. With S3 off, verify that `local_directory` exists or can be created and is writable by the application process. In the provided Docker Compose deployment, `/app/data` is a persistent volume and the default local directory is `/app/data/image-storage`.

Check the startup log for:

```text
WARN image_storage S3 is enabled but not fully configured; using local image storage  missing_keys=[...]
```

`missing_keys` names the incomplete S3 fields. The warning confirms that requests will fall back to local storage.

Two further causes of a 404 that are unrelated to storage: the API key's group must be on the **OpenAI or Grok** platform (any other platform, or a key with no group at all, yields `Images API is not supported for this platform`), and a task may only be polled with the **same API key that submitted it** — polling with a different key of the same user returns `image task not found` by design.

## Submit a task

```bash
curl -i https://api.example.com/v1/images/generations/async \
  -H 'Authorization: Bearer sk-...' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-1",
    "prompt": "A lighthouse during a winter storm",
    "size": "1536x1024"
  }'
```

The server stores the initial task in Redis and responds with `202 Accepted`:

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "processing",
  "created_at": 1784092800,
  "expires_at": 1784179200,
  "poll_url": "/v1/images/tasks/imgtask_0123456789abcdef"
}
```

`Location` contains the polling path and `Retry-After: 3` provides the recommended polling interval.

## Poll a task

Use the same API key that submitted the task:

```bash
curl https://api.example.com/v1/images/tasks/imgtask_0123456789abcdef \
  -H 'Authorization: Bearer sk-...'
```

While work is in progress:

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "processing",
  "created_at": 1784092800,
  "expires_at": 1784179200
}
```

On success, `result` mirrors the synchronous image API body, except each image has been moved to persistent storage: `data[].url` points at the stored image and `b64_json` is stripped (so both URL and base64 upstream formats end up as compact stored links):

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "completed",
  "http_status": 200,
  "image_url": "https://...",
  "result": {
    "created": 1784092923,
    "data": [{"url": "https://..."}]
  },
  "created_at": 1784092800,
  "completed_at": 1784092923,
  "expires_at": 1784179323
}
```

For URL responses, `image_url` mirrors the first `data[].url` for simple clients. On failure, the task reaches `failed` and exposes the original OpenAI-compatible error object where available:

```json
{
  "id": "imgtask_0123456789abcdef",
  "task_id": "imgtask_0123456789abcdef",
  "object": "image.generation.task",
  "status": "failed",
  "http_status": 502,
  "error": {
    "type": "api_error",
    "message": "Upstream request failed"
  },
  "created_at": 1784092800,
  "completed_at": 1784092923,
  "expires_at": 1784179323
}
```

All submit and poll responses include `Cache-Control: no-store`, preventing a CDN from caching the `processing` state. Tasks and results expire 24 hours after their latest state update. A task executes for at most 30 minutes.

Task ownership is scoped to both user and API key. Unknown task IDs and IDs owned by another key both return `404`, avoiding task-existence disclosure. Polling remains available when the completed generation used the key's remaining balance; normal authentication, disabled-key, user, IP, and group checks still apply.
