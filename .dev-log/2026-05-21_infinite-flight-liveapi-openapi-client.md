# 2026-05-21 — Infinite Flight LiveAPI OpenAPI Client

## Logical unit 1: Upstream LiveAPI spec, generated client, and wrapper migration

- **Logical unit / commit intent:** Add repo-owned OpenAPI artifacts for the upstream Infinite Flight Live API, generate an infra-owned Go client/models package, and route the existing `infra/liveapi.Client` wrapper through generated methods for the actively used sessions, session flights, flight route, flight plan, aircraft liveries, user stats, user flights, and user grade calls while preserving the existing downstream wrapper API.
- **Changed files:**
  - `api/openapi/liveapi.yaml`
  - `api/openapi/liveapi.cfg.yaml`
  - `Makefile`
  - `go.mod`
  - `go.sum`
  - `infra/liveapi/generated/client.gen.go`
  - `infra/liveapi/client.go`
- **Reused code / patterns / components:** Reused the existing `oapi-codegen` toolchain and Makefile pattern, kept generated code behind `infra/liveapi.Client`, preserved `IF_API_BASE_URL` / `IF_API_KEY` bearer-token auth, reused existing `http.Client` timeout behavior, compatibility DTOs in `infra/liveapi/dtos.go`, existing `APITime` parsing layouts, existing job/worker consumers, and existing metrics/logging surfaces without adding a new registry.
- **Logging added or affected:** No new high-cardinality or secret-bearing logs added. Existing wrapper/session logs remain. Generated client does not log requests by default. Server-start validation showed existing startup logs and an expected `unexpected status 401` from the aircraft livery startup sync when no valid local `IF_API_KEY` is available.
- **Metrics added or affected:** No metrics added in this slice. Existing job/queue/cache metrics continue to be used by `internal/sessions`, `internal/flights`, and `internal/platform/aircraft`.
- **Test surface touched or still needed:** Build and package tests passed. Dedicated `infra/liveapi` httptest coverage is still needed for bearer auth injection, generated wrapper mapping, `errorCode` handling, `429`, nullable fields, and custom date strings as assigned to the Unit Testing agent in the plan.
- **Build/test command(s) run and status:**
  - `make generate-liveapi-client` — passed; generated `infra/liveapi/generated/client.gen.go`
  - `go mod tidy` — passed; added missing generated-client runtime transitive sums
  - `go test ./infra/liveapi` — passed (`[no test files]`)
  - `go test ./internal/sessions ./internal/flights ./internal/platform/aircraft ./internal/pilots` — passed
  - `go build -buildvcs=false -o .air_tmp/main ./cmd/server` — passed
  - `timeout 20s .air_tmp/main` — started application, scheduler, flight plan worker, flight plan queue monitor, pilot sync worker, PIREP workers, aircraft livery sync worker, Watermill router, and HTTP server; terminated by timeout with graceful shutdown. LiveAPI startup call returned `401` because the local run did not have a valid IF API key, not because worker startup failed.
  - `go test ./...` — passed
- **Deviations from plan, if any:** The OpenAPI agent found upstream-doc paths for ATC, ATIS, and world status differ from existing no-argument wrapper methods (`/atc`, `/atis`, `/world/status` vs session/airport-scoped docs paths). Those legacy wrapper methods remain on handwritten `doGET` paths in this slice instead of inventing new method signatures or product behavior. `GetWorldStatus` also remains legacy because the doc-modeled response shape differs from the current DTO and there are no active direct callers discovered.
- **Blast-radius notes / dependent surfaces checked:** Checked `infra/liveapi`, generated config/spec, Makefile generation path, Go module dependencies, `internal/sessions/cache_job.go`, `internal/flights/cache_job.go`, `internal/flights/flight_plan_worker.go`, `internal/platform/aircraft/cache_job.go`, `internal/platform/aircraft/worker.go`, `internal/app/app.go`, and `internal/routes/jobs.go`. No public Politburo API routes, Vizburo handlers/templates, comrade-bot code, database migrations, Docker/infra files, cache keys, or job cadences were changed.
- **Live API compliance notes:** Bearer auth is used exclusively; query-string API key auth is not modeled or used. Existing operational Redis cache/job cadence and TTL behavior were preserved. The 7-day complete-flight/flight-plan-derived cache review remains a follow-up compliance decision from the plan.
- **Follow-up notes:**
  - **Swagger/OpenAPI:** Review whether ATC/ATIS/world-status legacy wrapper compatibility should get new explicit session/airport-aware methods in a separate approved slice. Keep `registration.yaml` and `internal/api/generated/registration/server.gen.go` untouched for this external upstream client work.
  - **Observability:** Add wrapper-level low-cardinality metrics/log review for upstream endpoint group, status class, error code, duration, and repeated `429`/auth failures through `infra/metrics.MetricsRegistry` if assigned. Do not log API keys or full payloads.
  - **Unit Testing:** Add httptest contract tests for generated-wrapper mappings and decode samples for sessions, flights, flight plan, aircraft liveries, user stats/history/grade, plus explicit `429`, nonzero `errorCode`, nullable field, UUID validation, and custom time parsing tests.

## Logical unit 2: Developer documentation and standards alignment

- **Logical unit / commit intent:** Document the new upstream LiveAPI spec/client structure, refresh the living technical standards with the standards implied by the LiveAPI implementation and May 20 dev logs, and record the follow-up documentation pass.
- **Changed files:**
  - `docs/dev/liveapi-openapi-client.md`
  - `.dev-log/2026-05-21_infinite-flight-liveapi-openapi-client.md`
  - Workspace-level `TECHNICAL_STANDARDS.md` outside the Politburo repository
- **Reused code / patterns / components:** Reused the existing `docs/dev/` developer-docs surface, the established dev-log structure, and standards already represented in `AGENTS.md`, `politburo/CLAUDE.md`, and the May 20 logs.
- **Logging added or affected:** None; documentation only.
- **Metrics added or affected:** None; documentation only.
- **Test surface touched or still needed:** No runtime test surface changed. Documentation was reviewed for alignment with generated-client ownership, bot header standards, Vizburo setup/readiness standards, and template caching guidance.
- **Build/test command(s) run and status:** Not run for this docs-only logical unit.
- **Deviations from plan, if any:** None. This is the documentation follow-up requested after implementation.
- **Blast-radius notes / dependent surfaces checked:** Reviewed May 20 dev logs for initserver minimal web setup, Vizburo UI architecture, and Discord onboarding/help/status. The previously untracked `2026-05-20_discord-bot-required-headers-middleware.md` log was not present on this isolated branch after stashing/restoring only LiveAPI work, so standards were derived from the committed/available May 20 logs and current `TECHNICAL_STANDARDS.md` content.
- **Live API compliance notes:** Documented bearer-only auth, temporary-cache/no-real-world-flight/no-AI-training constraints, generated-client boundary, and the ATC/ATIS/world-status mismatch as a follow-up decision.
- **Follow-up notes:**
  - **Swagger/OpenAPI:** Keep `liveapi.yaml` reviewed against upstream raw docs before regeneration; do not blend upstream external API specs with Politburo public API specs.
  - **Observability:** Future LiveAPI wrapper metrics/log additions should update both the implementation log and developer docs with endpoint group/status/error label guidance.
  - **Unit Testing:** Add LiveAPI wrapper contract tests before making broader migrations into `infra/providers` or `internal/common`.

## Logical unit 3: LiveAPI generated-wrapper unit tests

- **Logical unit / commit intent:** Add focused `infra/liveapi` unit tests for the handwritten wrapper around the generated upstream LiveAPI client, without calling the real Infinite Flight API or editing generated code.
- **Changed files:**
  - `infra/liveapi/client_test.go`
  - `.dev-log/2026-05-21_infinite-flight-liveapi-openapi-client.md`
- **Reused code / patterns / components:** Used `httptest.Server` for generated-client request/response paths, a custom `RoundTripper` for pre-request UUID validation, existing compatibility DTOs, existing wrapper methods, and existing private `parseAPITime` / `APITime` parsing behavior.
- **Logging added or affected:** No production logging added. Tests initialize the existing package logger because `GetFlights` logs through the global logger.
- **Metrics added or affected:** None.
- **Test surface touched or still needed:** Added coverage for bearer auth with no query-string API key, base URL path handling, generated-wrapper mappings for sessions/session flights/flight plan/aircraft liveries/user stats/user flights/user grade, explicit `429` rate-limit errors, nonzero upstream `errorCode`, nullable generated fields mapping into compatibility DTOs, UUID validation before HTTP requests, and LiveAPI date parsing for `YYYY-MM-DD HH:mm:ssZ`, RFC3339, and RFC3339Nano. No real upstream API verification was run by request.
- **Build/test command(s) run and status:**
  - `gofmt -w infra/liveapi/client_test.go && go test ./infra/liveapi` — passed
  - `go test ./internal/sessions ./internal/flights ./internal/platform/aircraft ./internal/pilots` — passed
  - `go test ./...` — passed
- **Deviations from plan, if any:** None for the testing slice. Existing uncommitted production-file modifications were present in the worktree and were left untouched.
- **Blast-radius notes / dependent surfaces checked:** Checked `infra/liveapi` directly, then the current LiveAPI-dependent session, flight, aircraft, and pilot packages, then the full Go test suite.
- **Live API compliance notes:** All tests use local fakes only. Bearer auth remains the only auth asserted by the generated-wrapper path.
- **Follow-up notes:** Broader migrations into `infra/providers.LiveAPIProvider` or `internal/common.LiveAPIService`, observability metrics/logging, and real upstream verification remain out of scope for this unit.

## Logical unit 4: LiveAPI wrapper observability and compliance follow-up

- **Logical unit / commit intent:** Add low-cardinality wrapper-level observability for generated/legacy `infra/liveapi.Client` calls using the existing `infra/metrics.MetricsRegistry`, and document the LiveAPI cache-compliance findings without changing cache semantics.
- **Changed files:**
  - `infra/liveapi/client.go`
  - `infra/metrics/metrics.go`
  - `internal/app/app.go`
  - `.dev-log/2026-05-21_infinite-flight-liveapi-openapi-client.md`
- **Reused code / patterns / components:** Reused the existing shared MetricsRegistry and app DI path. `liveapi.NewClient()` remains backward-compatible for tests/standalone callers, while `app.initInfra` now uses `liveapi.NewClientWithMetrics(metricsReg)` so the canonical server runtime exposes wrapper metrics on the existing `/metrics` endpoint.
- **Logging added or affected:** Added wrapper-level structured completion logs with `provider=liveapi`, `endpoint_group`, `status_class`, `status_code`, `error_type`, and `duration_ms`. Successful calls log at debug level; failed/non-2xx calls log at warn level. The wrapper logs do not add API keys, payloads, request IDs, Discord IDs, user IDs, flight IDs, session IDs, raw paths, or free-form error strings. Removed the previous wrapper-level `GetFlights` info log that included a session ID, and removed the previous wrapper-level `GetUserFlights` error log that included a user ID.
- **Metrics added or affected:** Added `politburo_liveapi_requests_total` and `politburo_liveapi_request_duration_seconds` with labels `provider`, `endpoint_group`, `status_class`, and `error_type`. Endpoint groups currently used are `sessions`, `flights`, `flight_plan`, `aircraft`, and `users`. Error types are bounded strings such as `none`, `network`, `client_init`, `request_build`, `encode_error`, `read_error`, `decode_error`, `empty_response`, `rate_limited`, `auth`, `not_found`, `status_4xx`, `status_5xx`, and `error_code_<n>` for the generated LiveAPI enum values.
- **Prometheus / Loki / prod infra impact:** No labour-bureau dev/prod monitoring files were changed. Politburo is already scraped through the existing `/metrics` target, so the new metrics are exposed without a new scrape target. No new log stream or container/service was introduced; existing Politburo log routing remains sufficient.
- **Job metrics review:** Existing job/worker metrics in `internal/sessions/cache_job.go`, `internal/flights/cache_job.go`, `internal/flights/flight_plan_worker.go`, `internal/platform/aircraft/cache_job.go`, and `internal/platform/aircraft/worker.go` were reviewed. They continue to cover sync duration, cache sizes, queue activity, and aircraft record processing; wrapper metrics now add upstream-call status/duration visibility without widening every job constructor.
- **Live API compliance notes:** The plan's 7-day LiveAPI-derived cache concern is still present in `internal/flights/cache_job.go` (`CompleteFlight` cache), `internal/flights/flight_plan_worker.go` (flight plan and complete-flight refresh caches), and `infra/cache/keys.go` comments. No TTL/cache semantic change was made because the plan requested review and did not clearly authorize a behavior change. Sessions and aircraft operational cache TTLs remain as-is.
- **Build/test command(s) run and status:**
  - `go test ./infra/liveapi ./infra/metrics ./internal/sessions ./internal/flights ./internal/platform/aircraft` — passed
  - `go build -buildvcs=false -o .air_tmp/main ./cmd/server` — passed
- **Follow-up notes:**
  - **Developer:** Decide in a separate approved slice whether 7-day complete-flight and flight-plan TTLs are still acceptable under Infinite Flight's temporary-cache terms, and adjust product/cache behavior if needed.
  - **Swagger/OpenAPI:** No spec changes needed for observability; continue avoiding generated-file hand edits.
  - **Unit Testing:** Add httptest coverage for the new wrapper metrics/log classification paths, especially `429`, `401/403`, nonzero `errorCode`, empty response, decode errors, and network failures.
