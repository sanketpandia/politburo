# 2026-05-20 — Vizburo UI Architecture

## Phase 1: Renderer caching baseline

- **Logical unit / commit intent:** Add non-local template caching to `infra/templates.Renderer` while preserving local template reload behavior and existing render modes.
- **Changed files:**
  - `infra/templates/renderer.go`
  - `infra/templates/renderer_test.go`
  - `internal/app/app.go`
- **Reused code / patterns / components:** Reused existing `infra/templates.Renderer`, existing template function map, existing `APP_ENV` config, and `internal/app.App` infrastructure wiring. Kept `RenderTemplate`, `RenderPartial`, and `RenderStandalone` public APIs intact.
- **Logging added or affected:** Added debug cache-hit logging and retained low-cardinality parse/load logs by render mode and template name/path. App startup now logs whether template reload is enabled.
- **Metrics added or affected:** No metrics added in this slice. Follow-up observability can add render duration/cache miss counters through `infra/metrics.MetricsRegistry` if approved.
- **Test surface touched or still needed:** Added renderer tests for production-like cache reuse, local reload behavior, partial `content` fallback, standalone error layout rendering, and missing file errors. Broader UI handler tests remain for later phases.
- **Build/test command(s) run and status:**
  - `go test ./infra/templates` — passed
  - `go test ./infra/templates ./internal/routes ./internal/dashboard ./internal/datasource ./internal/events ./internal/vaadmin ./internal/pilots ./internal/flights ./internal/liverymappings` — passed
  - `go build -buildvcs=false -o .air_tmp/main ./cmd/server` — passed
  - `go build -buildvcs=false -o .air_tmp/vizburo ./cmd/vizburo` — passed
- **Deviations from plan, if any:** None for Phase 1. The renderer keeps local reparsing when `APP_ENV=local` and caches parsed templates otherwise.
- **Blast-radius notes / dependent surfaces checked:** Checked `internal/app` renderer construction, route-adjacent packages, dashboard/datasource/events/vaadmin/pilots/flights/livery mappings packages, and both server/vizburo binaries. No route, handler, template path, API, job, bot, or infra compose changes were made.
- **Live API compliance notes:** Not applicable; no Live API calls or request/response contracts changed.
- **Follow-up notes:**
  - **Swagger/OpenAPI:** No API contract changes in this slice; verification only.
  - **Observability:** Consider renderer duration/cache miss metrics later via the existing metrics registry; avoid high-cardinality labels.
  - **Unit Testing:** Renderer baseline tests landed. Later phases still need UI context middleware and handler tests.
