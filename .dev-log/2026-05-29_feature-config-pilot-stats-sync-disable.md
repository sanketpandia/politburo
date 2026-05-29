# Feature Config Pilot Stats Sync Disable

Date: 2026-05-29
Branch: `feat/feature-config-pilot-stats-sync-disable`

## Logical unit / commit intent

- Phase 1 only: disable Airtable-backed scheduled sync jobs and workers at runtime through `internal/routes/jobs.go`.

## Changed files

- `internal/routes/jobs.go`

## Reused code / patterns / components

- Reused the existing scheduler/worker registration boundary in `internal/routes/jobs.go`.
- Preserved unrelated job and worker startup flow for sessions, aircraft, live flights, webhook, and flight-plan processing.

## Logging added or affected

- Added explicit startup logs documenting that Airtable sync scheduled jobs are intentionally disabled.
- Added explicit startup logs documenting that Airtable sync workers are intentionally disabled.

## Metrics added or affected

- No new metrics added in this phase.
- Existing Airtable sync job/worker metrics will stop receiving new runtime activity because registration/startup is disabled.

## Test surface touched or still needed

- Touched runtime registration surface only.
- Still needed later: focused tests for disabled job registration behavior in `internal/routes/jobs.go`.

## Build/test command(s) run and status

- `go test ./internal/routes ./internal/flights ./internal/platform/aircraft ./internal/sessions`
  - status: passed

## Deviations from plan, if any

- None. This implements the requested phase-1-only slice.

## Blast-radius notes / dependent surfaces checked

- Checked `internal/app/app.go`: sync job/worker dependencies remain initialized but are no longer started here in phase 1.
- Checked `internal/pilots/sync_job.go`, `internal/va_routes/sync_job.go`, and `internal/pireps/sync_job.go`: code retained intentionally; runtime activation disabled only.
- Checked `internal/pireps/queue_worker` startup path via `internal/routes/jobs.go`: disabled without deleting worker implementation.

## Live API compliance notes when relevant

- Not applicable in this phase; no LiveAPI path changes.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: no API contract changes in phase 1.
- Observability: remove or adjust dashboards/alerts/assumptions tied to Airtable sync jobs and workers now that runtime registration is disabled.
- Unit Testing: add focused regression coverage that confirms disabled Airtable sync jobs/workers do not register while unrelated jobs/workers still do.

---

Date: 2026-05-29
Branch: `feat/feature-config-pilot-stats-next`

## Logical unit / commit intent

- Backend foundation only: add a unified modern Airtable provider-config accessor in `internal/platform/va` and migrate pilot stats service to consume it instead of ad hoc config parsing.

## Changed files

- `internal/platform/va/provider_accessor.go`
- `internal/app/app.go`
- `internal/pilots/stats_service.go`

## Reused code / patterns / components

- Reused existing modern `internal/platform/va.Repository` typed config methods:
  - `GetAirtableCredentials`
  - `GetAirtableSchema`
- Reused `internal/platform/va.ConfigService` for remaining basic key/value callsign-prefix access.
- Reused `infra/providers.LiveAPIProvider` injected from app wiring instead of instantiating a new provider inside stats service.
- Reused the existing `/api/v1/pilot/stats` route and response envelope shape.

## Logging added or affected

- No new log families added in this slice.
- Existing pilot stats logs now flow through the modern accessor-backed config retrieval path.

## Metrics added or affected

- No new metrics added in this slice.
- No direct observability changes beyond preserving existing LiveAPI provider metrics through injected provider reuse.

## Test surface touched or still needed

- Touched `internal/pilots`, `internal/platform/va`, and `internal/app` wiring surface.
- Still needed later: focused accessor tests for cache behavior, nil config handling, and invalidation expectations.

## Build/test command(s) run and status

- `go test ./internal/pilots ./internal/platform/va ./internal/dashboard`
  - status: passed
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - status: passed
- `curl --request GET --url http://localhost:8080/api/v1/user/status --header 'x-api-key: a5bf24a3-8ae8-49e3-9f86-2f411c19ff96' --header 'x-discord-server-id: 790634557228711956' --header 'x-discord-user-id: 668664447950127154'`
  - status: passed
- `curl --request GET --url http://localhost:8080/api/v1/pilot/stats --header 'x-api-key: a5bf24a3-8ae8-49e3-9f86-2f411c19ff96' --header 'x-discord-server-id: 790634557228711956' --header 'x-discord-user-id: 668664447950127154'`
  - status: passed

## Deviations from plan, if any

- Intentionally limited to backend-foundation only per user instruction.
- Did not add `feature_pilot_stats`, admin UI, CSS/component extraction, or broader consumer migrations in this slice.

## Blast-radius notes / dependent surfaces checked

- Checked `/api/v1/user/status` to confirm API-key claim resolution still maps Discord server ID to VA UUID correctly before using `/api/v1/pilot/stats`.
- Checked `internal/dashboard/service.go`: unchanged because it delegates to stats service.
- Checked `internal/app/app.go`: stats service now receives the platform accessor and injected LiveAPI provider, while legacy VA config remains in place for PIREP flows outside this slice.

## Live API compliance notes when relevant

- Pilot stats now uses the injected `infra/providers.LiveAPIProvider` from app wiring rather than constructing a new provider instance in `internal/pilots/stats_service.go`.
- No new direct LiveAPI client imports were introduced into feature code.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: no response-shape change was intentionally introduced in this backend-foundation slice.
- Observability: future accessor/cache metrics remain a follow-up; current accessor adds no hit/miss instrumentation yet.
- Unit Testing: add focused tests for `ProviderConfigAccessor` cache read path and for pilot stats behavior when pilot/career-mode schema or credentials are absent.
