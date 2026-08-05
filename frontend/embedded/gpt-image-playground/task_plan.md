# Task Plan

## Goal
Add and harden a Redis-backed Go async image service that can replace the current Node async service while keeping the frontend API contract unchanged.

## Phases
- [complete] Inspect current async service and frontend contract.
- [complete] Implement Go HTTP service with Redis queue and task state.
- [complete] Add local run scripts and documentation.
- [complete] Verify with Go tests, frontend tests/build, and a mock upstream smoke test.
- [complete] Store completed image bytes on the async server for one hour and return URLs instead of keeping base64 image payloads in Redis.
- [complete] Switch the upstream bridge to non-streaming by default and support non-streaming Responses-to-Images conversion.
- [complete] Re-run backend/frontend tests and smoke-test local task polling plus temp-file serving.
- [complete] Add upstream retry and streaming fallback for transient TLS/EOF/5xx failures.

## Decisions
- Keep the existing Node async script as a fallback.
- Preserve the current frontend polling contract: `POST /v1/images/generations`, `POST /v1/images/edits`, `POST /v1/responses`, and `GET /v1/images/tasks/{task_id}`.
- Use Redis for queued jobs and task state with TTL.
- Run the local Go service on the existing `8789` port so the current Vite proxy configuration continues to work.
- Keep Redis for task metadata/status only; completed image files should live in a server temp directory and be cleaned up after the configured TTL.
- Default the forced `/responses` upstream bridge to non-streaming to avoid the SSE EOF issue seen through the current upstream.
- If the non-streaming bridge hits transport-level failures such as TLS handshake timeout or unexpected EOF, retry with a fresh request and allow an internal streaming fallback.

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| Real upstream `/responses` returned HTTP 200 then `unexpected EOF` while reading SSE | Streaming bridge | Switch default upstream bridge to non-streaming, while keeping streaming parser as optional fallback |
| Non-streaming `/responses` failed with `unexpected EOF` after ~60s, then another task hit `net/http: TLS handshake timeout` after 30s | Non-streaming bridge | Add retry/fresh-connection handling and streaming fallback for retryable upstream failures |
