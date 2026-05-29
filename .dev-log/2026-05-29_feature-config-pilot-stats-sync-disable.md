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
