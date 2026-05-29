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

---

Date: 2026-05-29
Branch: `feat/feature-config-pilot-stats-next`

## Logical unit / commit intent

- Decompose `internal/pilots/stats_service.go` so subject resolution and liveapi mapping are extracted into focused collaborators while keeping `/api/v1/pilot/stats` contract behavior stable.

## Changed files

- `internal/pilots/stats_service.go`
- `internal/pilots/stats_subject_reader.go`
- `internal/pilots/stats_liveapi_service.go`
- `internal/pilots/stats_field_mapper.go`
- `internal/platform/memberships/stats_subject.go`
- `internal/app/app.go`
- `plans/2026-05-29-feature-configs-stats-and-sync-disable-plan.md`

## Reused code / patterns / components

- Reused `internal/platform/memberships` as the subject-context ownership boundary.
- Reused existing `infra/providers.LiveAPIProvider` path for live stats retrieval.
- Reused existing pilot stats response mapping conventions and route contract.

## Logging added or affected

- No new log family added; existing pilot stats logs remain in the orchestrator path.

## Metrics added or affected

- No new metric families in this decomposition slice.

## Test surface touched or still needed

- Touched `internal/pilots`, `internal/platform/memberships`, and DI wiring in `internal/app`.
- Still needed later: targeted tests for subject-reader failure modes, cache behavior, and provider-enrichment fallback behavior.

## Build/test command(s) run and status

- `go test ./internal/pilots ./internal/platform/memberships ./internal/dashboard`
  - status: passed
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - status: passed

## Deviations from plan, if any

- Scope intentionally remained decomposition-only; no feature-config editor, no new stats-card schema persistence, and no additional observability instrumentation in this unit.

## Blast-radius notes / dependent surfaces checked

- Checked `internal/dashboard/service.go` consumer path to confirm it still delegates through stats service.
- Checked `internal/app/app.go` feature wiring to ensure new collaborators are injected via existing DI container.
- Confirmed no bot or labour-bureau surface required updates for this backend-only decomposition.

## Live API compliance notes when relevant

- Live stats path remains behind `infra/providers.LiveAPIProvider` and does not import generated liveapi code directly into feature handlers/services.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: no route shape changes intentionally introduced in this unit.
- Observability: add cache-hit/miss and provider-enrichment latency/error classification in follow-on slices.
- Unit Testing: add focused coverage for extracted collaborators (`stats_subject_reader`, `stats_liveapi_service`, and `stats_field_mapper`) and orchestration fallbacks.

---

Date: 2026-05-29
Branch: `feat/feature-config-pilot-stats-next`

## Logical unit / commit intent

- Implement cache-first pilot stats orchestration with bounded manual refresh cooldown, concurrent fetch execution, and typed `feature_pilot_stats` config accessor support.

## Changed files

- `internal/pilots/stats_service.go`
- `internal/pilots/stats_constants.go`
- `internal/pilots/handler.go`
- `internal/platform/va/provider_accessor.go`
- `internal/platform/va/repo.go`
- `internal/platform/va/config_dtos.go`
- `plans/2026-05-29-feature-configs-stats-and-sync-disable-plan.md`

## Reused code / patterns / components

- Reused `ProviderConfigAccessor` as the single platform boundary for typed provider config reads.
- Reused existing `cache.CacheService` Get/Set primitives for response/profile and refresh cooldown keys.
- Reused existing `/api/v1/pilot/stats` route and `httpdto` envelope behavior; added optional `refresh` query handling without adding a second endpoint.

## Logging added or affected

- Added warning-level logs for unavailable `feature_pilot_stats` reads while preserving graceful fallback.
- Preserved optional-data warning semantics for provider/career-mode/liveapi fetch failures.

## Metrics added or affected

- No new metric families in this unit.
- Cache-first behavior is now active for stats profile responses and provider-record reads; observability instrumentation for hit/miss counters remains follow-on.

## Test surface touched or still needed

- Touched pilot stats handler/service orchestration and platform provider config accessors.
- Still needed: dedicated unit tests for refresh cooldown (429 path), profile cache hit behavior, and feature config parse coverage.

## Build/test command(s) run and status

- `go test ./internal/pilots ./internal/platform/va ./internal/platform/memberships ./internal/dashboard ./internal/routes`
  - status: passed
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - status: passed
- `curl --request GET --url http://localhost:8080/api/v1/pilot/stats --header 'x-api-key: 76fc9056-c4e3-4d35-83a2-6404fd6567d0' --header 'x-discord-server-id: 790634557228711956' --header 'x-discord-user-id: 668664447950127154'`
  - status: passed (200 envelope with game+provider data)

## Deviations from plan, if any

- Added short unit-testing guide text directly into the plan document to support follow-on Unit Testing agent execution.
- Feature-config admin UI/editor slice remains pending in this branch.

## Blast-radius notes / dependent surfaces checked

- Checked `internal/dashboard/service.go` compatibility: existing `GetPilotStats` call path remains valid via default non-refresh wrapper.
- Checked handler claims/context path in `internal/pilots/handler.go`: auth claim extraction unchanged; only optional refresh query added.
- No comrade-bot or labour-bureau source modifications were required for this backend slice.

## Live API compliance notes when relevant

- Live stats remain fetched through `infra/providers.LiveAPIProvider` collaborator path.
- No direct import/use of `infra/liveapi/generated/**` introduced.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: decide whether `refresh` query param should be documented for `/api/v1/pilot/stats` in a formal OpenAPI artifact.
- Observability: add explicit counters/histograms for stats profile cache hit/miss, manual refresh accepted/rejected, and feature-config fetch failures.
- Unit Testing: add focused tests for `GetPilotStatsWithOptions` cache/cooldown behavior and `feature_pilot_stats` parsing paths.

---

Date: 2026-05-29
Branch: `feat/feature-config-pilot-stats-next`

## Logical unit / commit intent

- Implement dashboard pilot-stats UI integration as HTMX partial loading so dashboard page render is not blocked by pilot stats fetch latency.

## Changed files

- `internal/dashboard/handler.go`
- `internal/dashboard/service.go`
- `internal/routes/router.go`
- `templates/pages/dashboard.html`
- `templates/partials/pilot-stats.html`

## Reused code / patterns / components

- Reused existing dashboard/session auth flow and active-VA context resolution in dashboard handlers.
- Reused existing `StatsService.GetPilotStatsWithOptions(...)` refresh/cooldown behavior through dashboard service.
- Reused existing design-system classes (`card`, `section-card`, `btn`, `empty-state`) and HTMX route patterns already used in dashboard partials.

## Logging added or affected

- Added warning logs for pilot-stats partial fetch failures with VA/user context and refresh mode.

## Metrics added or affected

- No new metrics in this UI slice.
- Existing stats/cache behavior metrics follow-up remains pending for observability agent.

## Test surface touched or still needed

- Touched dashboard handler/service routing and pilot stats partial template rendering path.
- Still needed: focused handler tests for `/dashboard/pilot-stats` success/refresh/error fallback HTML behavior.

## Build/test command(s) run and status

- `go test ./internal/dashboard ./internal/routes ./internal/pilots`
  - status: passed
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - status: passed
- `curl --request GET --url http://localhost:8080/api/v1/pilot/stats --header 'x-api-key: 76fc9056-c4e3-4d35-83a2-6404fd6567d0' --header 'x-discord-server-id: 790634557228711956' --header 'x-discord-user-id: 668664447950127154'`
  - status: passed

## Deviations from plan, if any

- Implemented HTMX lazy-load of pilot stats card partial on dashboard page load to avoid blocking initial dashboard render.

## Blast-radius notes / dependent surfaces checked

- Checked member dashboard route group in `internal/routes/router.go` to keep auth/role protections unchanged.
- Kept `/api/v1/pilot/stats` as the single stats data contract and consumed it through existing service wiring.
- No comrade-bot or labour-bureau changes required.

## Live API compliance notes when relevant

- No changes to LiveAPI boundary; dashboard partial path uses existing stats service/provider stack.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: dashboard HTML partial endpoint is UI-only and should not be added to OpenAPI.
- Observability: consider adding partial-route latency and refresh rejection counters if dashboard UX monitoring is needed.
- Unit Testing: add coverage for HTMX partial rendering and cooldown user message path.

---

Date: 2026-05-29
Branch: `feat/feature-config-pilot-stats-next`

## Logical unit / commit intent

- Polish dashboard pilot-stats presentation: remove redundant dashboard secondary nav, simplify card content, and improve flight-hours/readability formatting.

## Changed files

- `templates/pages/dashboard.html`
- `templates/partials/pilot-stats.html`
- `infra/templates/renderer.go`

## Reused code / patterns / components

- Reused existing template helper pattern in renderer func map for reusable duration formatting.
- Reused existing design-system card/field styles in pilot stats partial.

## Logging added or affected

- No logging changes in this UI-polish unit.

## Metrics added or affected

- No metrics changes.

## Test surface touched or still needed

- Touched template function map and dashboard template partial rendering.
- Still needed later: UI snapshot/functional checks for HH:MM output edge cases from provider values.

## Build/test command(s) run and status

- `go test ./infra/templates ./internal/dashboard ./internal/routes ./internal/pilots`
  - status: passed
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - status: passed

## Deviations from plan, if any

- Removed dashboard secondary nav on `/dashboard` page by request to rely on top nav only.
- Removed freshness card from user dashboard pilot-stats cards as low-value noise.

## Blast-radius notes / dependent surfaces checked

- Scoped to dashboard page/partial and template helpers only; no API contract changes.

## Live API compliance notes when relevant

- Not applicable; no liveapi boundary changes.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: no spec changes required for UI-only polish.
- Unit Testing: add template-level checks for `formatDurationHHMM` behavior when provider returns unexpected numeric types.

---

Date: 2026-05-29
Branch: `feat/feature-config-pilot-stats-next`

## Logical unit / commit intent

- Implement datasource mapping workflow refresh: clickable Airtable rows, HTMX chooser popout, HTMX partial refresh on mapping apply, and read-only dual-tone internal field targets.

## Changed files

- `internal/datasource/handler.go`
- `internal/routes/router.go`
- `templates/partials/datasource-field-mapper.html`
- `templates/partials/datasource-field-mapping-chooser.html`
- `static/css/design-system.css`

## Reused code / patterns / components

- Reused existing datasource schema sync flow and active-VA session context.
- Reused HTMX partial endpoint pattern already used throughout datasource/admin UI.
- Reused design-system tokens/classes and section-card layout conventions.

## Logging added or affected

- Existing datasource error logging paths now include mapping chooser/apply rendering failures.

## Metrics added or affected

- No metrics added in this UI workflow slice.

## Test surface touched or still needed

- Touched datasource handler route surface and template rendering for mapping workflow.
- Still needed: focused handler tests for mapping chooser/apply parameter validation and mapping conflict replacement behavior.

## Build/test command(s) run and status

- `go test ./internal/datasource ./internal/routes ./infra/templates`
  - status: passed
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - status: passed

## Deviations from plan, if any

- Implemented mapping UX with hidden `field_mapping[...]` inputs + HTMX mapping apply endpoint rather than dropdown-driven direct edits.

## Blast-radius notes / dependent surfaces checked

- Scoped to datasource admin workflow; no pilot stats API contract or dashboard behavior changed.
- Save schema flow still posts the same `field_mapping[...]` shape expected by existing backend parsing.

## Live API compliance notes when relevant

- Not applicable.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: no API spec changes needed; dashboard/admin HTML HTMX endpoints remain out of OpenAPI scope.
- Unit Testing: add regression tests ensuring mapping-apply unmaps previous target when remapping Airtable field to a new internal field.
