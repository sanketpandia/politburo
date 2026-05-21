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
