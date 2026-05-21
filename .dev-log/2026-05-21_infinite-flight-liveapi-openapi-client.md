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

## Logical unit 5: Remaining LiveAPI follow-up triage and documentation refresh

- **Logical unit / commit intent:** Evaluate the remaining approved-plan follow-ups for ATC/ATIS/world-status mismatch, 7-day cache TTL compliance, and duplicate-client consolidation; update developer documentation with completed vs remaining work and record blockers instead of inventing product/API behavior.
- **Changed files:**
  - `docs/dev/liveapi-openapi-client.md`
  - `.dev-log/2026-05-21_infinite-flight-liveapi-openapi-client.md`
- **Reused code / patterns / components:** Reused existing grep-based caller inspection, existing developer-docs surface, and established dev-log structure. No generated files were edited, and the generated package remains hidden behind `infra/liveapi.Client`.
- **Logging added or affected:** None; documentation-only logical unit. Existing wrapper logs from logical unit 4 remain the active logging surface.
- **Metrics added or affected:** None; documentation-only logical unit. Existing `politburo_liveapi_requests_total` and `politburo_liveapi_request_duration_seconds` remain the active wrapper metrics.
- **Test surface touched or still needed:** No runtime test surface changed. Follow-up testing is still needed before any migration of `infra/providers.LiveAPIProvider` or `internal/common.LiveAPIService`, specifically parity coverage for context cancellation, provider error codes/details, non-UUID legacy DTO fixtures, generated-wrapper UUID validation, and registration/PIREP/Vizburo consumers.
- **Build/test command(s) run and status:** Runtime tests not run because this unit changed docs/dev-log only. Diff inspection was performed before commit.
- **Deviations from plan, if any:** None. The plan authorized review of these follow-ups but did not clearly authorize ATC/ATIS/world-status API behavior changes, 7-day TTL semantic changes, or broad legacy-client refactors.
- **Blast-radius notes / dependent surfaces checked:** Checked `infra/liveapi/client.go`, generated method presence in `infra/liveapi/generated/client.gen.go` without editing it, `api/openapi/liveapi.yaml` path shape, `internal/common/live_api_service.go`, `infra/providers/live_api_provider.go`, `infra/providers/live_api_provider_test.go`, `internal/models/dtos/responses.go`, `internal/flights/cache_job.go`, `internal/flights/flight_plan_worker.go`, `infra/cache/keys.go`, `internal/app/app.go`, `internal/pilots/registration_service.go`, `internal/pilots/stats_service.go`, and `internal/common/flight_data.go`.
- **Live API compliance notes:** No real Infinite Flight API calls were made. The 7-day `CompleteFlight` and flight-plan TTLs remain unchanged because the approved plan called for review and an explicit decision is still needed on whether 7 days qualifies as temporary operational caching. No new public Politburo API, Vizburo, bot, infra, or database behavior was introduced.
- **Follow-up notes:**
  - **Swagger/OpenAPI:** If ATC/ATIS/world-status compatibility is revisited, define explicit session/airport-aware wrapper signatures and response conversion expectations before changing `infra/liveapi.Client`; do not retrofit behavior into no-arg methods without a product decision.
  - **Observability:** No new observability work remains from this triage. If duplicate-client migration is later approved, ensure calls delegated through `infra/liveapi.Client` keep low-cardinality wrapper metrics and do not duplicate provider-level metrics.
  - **Unit Testing:** Add behavior-parity tests before consolidating `infra/providers.LiveAPIProvider` or `internal/common.LiveAPIService`; include context/error semantics and active registration/PIREP/Vizburo consumer expectations.

## Logical unit 6: Documentation reconciliation after LiveAPI follow-up passes

- **Logical unit / commit intent:** Reconcile developer documentation after generated-client implementation, tests, observability, and follow-up-decision passes so the docs describe shipped wrapper metrics and only decision-gated remaining work.
- **Changed files:**
  - `docs/dev/liveapi-openapi-client.md`
  - `.dev-log/2026-05-21_infinite-flight-liveapi-openapi-client.md`
- **Reused code / patterns / components:** Reused the existing developer-doc surface and dev-log format. No generated files, OpenAPI specs, application code, dashboards, or tests were edited.
- **Logging added or affected:** None; documentation-only logical unit. The documented runtime log behavior remains the wrapper-level completion logging from logical unit 4.
- **Metrics added or affected:** None; documentation-only logical unit. Documented the shipped `politburo_liveapi_requests_total` and `politburo_liveapi_request_duration_seconds` metrics and their bounded labels.
- **Test surface touched or still needed:** No runtime test surface changed. Docs-only diff inspection is sufficient for this unit.
- **Build/test command(s) run and status:** Runtime tests not run because this unit changed docs/dev-log only. Diff inspection was performed before commit.
- **Deviations from plan, if any:** None. User-facing docs remain intentionally unchanged because the feature is an internal upstream-client refactor.
- **Blast-radius notes / dependent surfaces checked:** Rechecked `docs/dev/liveapi-openapi-client.md`, this dev log, `api/openapi/liveapi.yaml`, `api/openapi/liveapi.cfg.yaml`, `Makefile`, `infra/liveapi/client.go`, `infra/metrics/metrics.go`, `internal/app/app.go`, `README.md`, `docs/dev/commands_cheat_sheet.md`, `docs/dev/implementation.md`, `internal/platform/aircraft/README.md`, and workspace-level `TECHNICAL_STANDARDS.md` for stale or missing generated-client references.
- **Live API compliance notes:** Real upstream API verification remains deferred by user request. The ATC/ATIS/world-status signature decision, 7-day TTL compliance decision, and provider/common consolidation parity decision remain explicit follow-ups.

## Logical unit 7: LiveAPI wrapper observability classification tests

- **Logical unit / commit intent:** Add focused unit coverage for the LiveAPI wrapper metrics classification paths introduced in logical unit 4.
- **Changed files:**
  - `infra/liveapi/client_test.go`
  - `go.mod`
  - `.dev-log/2026-05-21_infinite-flight-liveapi-openapi-client.md`
- **Reused code / patterns / components:** Reused `httptest.Server`, the existing wrapper methods, the existing metrics registry type, and isolated Prometheus test registries/counters instead of calling the real Infinite Flight API or inspecting generated code internals.
- **Logging added or affected:** No production logging changed. The tests assert the metric labels emitted by the same `observeLiveAPICall` path that also emits wrapper logs, without coupling to logger output formatting.
- **Metrics added or affected:** No production metrics changed. Tests verify `politburo_liveapi_requests_total` label classifications for `rate_limited`, `auth`, `error_code_6`, `empty_response`, `decode_error`, and `network`.
- **Test surface touched or still needed:** Added observability classification coverage for `429`, `401`, `403`, nonzero upstream `errorCode`, empty generated response, malformed JSON through the legacy wrapper decode path, and network failures. Generated-client malformed JSON returns before a parsed response is available, so this slice keeps decode-error assertion on a wrapper path that can observe the response status safely.
- **Build/test command(s) run and status:**
  - `gofmt -w infra/liveapi/client_test.go && go test ./infra/liveapi ./infra/metrics` — initially requested `go mod tidy` for the Prometheus test helper import.
  - `go mod tidy && go test ./infra/liveapi ./infra/metrics` — passed.
  - `go test ./...` — passed.
- **Deviations from plan, if any:** None. No generated files, provider/common migrations, cache behavior, or public API behavior were changed.
- **Blast-radius notes / dependent surfaces checked:** Limited to `infra/liveapi` tests and the existing metrics type. `infra/metrics` production code was not edited.
- **Live API compliance notes:** All coverage uses local fakes/custom transports only; no real upstream API calls were made.

## Logical unit 8: LiveAPI-derived TTL standardization

- **Logical unit / commit intent:** Apply the product/compliance decision to keep LiveAPI-derived complete-flight and flight-plan cache data for 48 hours, and centralize cache TTL durations instead of hardcoding them at call sites.
- **Changed files:**
  - `infra/cache/ttl.go`
  - `infra/cache/keys.go`
  - `internal/flights/cache_job.go`
  - `internal/flights/flight_plan_worker.go`
  - `internal/sessions/cache_job.go`
  - `internal/platform/aircraft/cache_job.go`
  - `internal/platform/aircraft/worker.go`
  - `docs/dev/liveapi-openapi-client.md`
  - `.dev-log/2026-05-21_infinite-flight-liveapi-openapi-client.md`
- **Reused code / patterns / components:** Reused the existing `infra/cache` package as the cache-key/Redis-cache boundary and moved TTL values next to cache key ownership. Existing jobs/workers still use the same cache keys and cache services.
- **Logging added or affected:** None.
- **Metrics added or affected:** None.
- **Test surface touched or still needed:** No behavior tests existed for TTL values. Focused compile/package tests covered all touched packages.
- **Build/test command(s) run and status:**
  - `gofmt -w infra/cache/ttl.go infra/cache/keys.go internal/flights/cache_job.go internal/flights/flight_plan_worker.go internal/sessions/cache_job.go internal/platform/aircraft/cache_job.go internal/platform/aircraft/worker.go` — passed
  - `go test ./infra/cache ./internal/sessions ./internal/flights ./internal/platform/aircraft` — passed
- **Deviations from plan, if any:** The original plan requested review of the 7-day TTL. The user clarified the decision in this follow-up: LiveAPI-derived TTLs should be 48 hours and standardized in constants.
- **Blast-radius notes / dependent surfaces checked:** Updated CompleteFlight, flight-plan, session, aircraft/livery, live-flight-list, and world/session-details TTL call sites that were directly using hardcoded LiveAPI-related cache durations. No cache keys, job cadence, queue behavior, API contracts, generated files, or database persistence behavior changed.
- **Live API compliance notes:** Complete-flight and flight-plan-derived data now use `cache.LiveFlightTTL` and `cache.FlightPlanTTL` set to 48 hours. Other LiveAPI-related operational TTLs are also named constants: `SessionTTL`, `AircraftTTL`, `LiveFlightListTTL`, and `WorldDetailsTTL`.
- **Follow-up notes:**
  - **Swagger/OpenAPI:** None.
  - **Observability:** Existing wrapper metrics/logging remain unchanged.
  - **Unit Testing:** If TTL behavior becomes product-critical, add tests around cache set calls or use fake cache services that capture TTLs.
