# Feature configs, pilot stats rewrite, and sync-disable plan

Status: finalized repo-grounded implementation plan

Requested change summary:

- Disable the current Airtable-backed pilot sync job and the other Airtable sync jobs/workers tied to that model.
- Redo `internal/pilots/stats_service.go` around a cleaner feature-config-driven design.
- Keep Infinite Flight game API reads in place, but ensure they flow through the generated-client-backed `infra/liveapi.Client` boundary rather than legacy/common paths.
- Evaluate and plan a feature-config model for “extra stats fields” admins can expose from configured pilot and PIREP schemas.
- Reuse the active datasource UI patterns/components (chips, mappers, cards, field lists) and move repeated styling into shared design-system/template components where needed.
- Assess feasibility, latency, rate-limit risk, and caching strategy before committing to runtime Airtable aggregation behavior.

Scope and assumptions:

- This plan covers `politburo/` only.
- No implementation is performed here.
- “Disable sync jobs” is interpreted as removing scheduled registration/startup of the Airtable sync jobs and related worker paths from the active runtime, not deleting the code immediately.
- Because the request explicitly asks for feasibility review, the plan treats some admin-configurable aggregation ideas as open questions to validate before implementation.

## Context

Files/packages inspected:

- `politburo/TECHNICAL_STANDARDS.md`
- `politburo/CLAUDE.md`
- `politburo/plans/2026-05-29-holding-config-current-state-analysis.md`
- `politburo/internal/routes/router.go`
- `politburo/internal/routes/jobs.go`
- `politburo/internal/app/app.go`
- `politburo/internal/platform/va/model.go`
- `politburo/internal/platform/va/config_dtos.go`
- `politburo/internal/platform/va/service.go`
- `politburo/internal/platform/va/repo.go`
- `politburo/internal/services/data_provider_config_service.go`
- `politburo/internal/pilots/stats_service.go`
- `politburo/internal/pilots/sync_job.go`
- `politburo/internal/va_routes/sync_job.go`
- `politburo/internal/pireps/sync_job.go`
- `politburo/internal/pireps/service.go`
- `politburo/internal/pireps/tour_service.go`
- `politburo/internal/datasource/handler.go`
- `politburo/templates/partials/datasource-field-mapper.html`
- `politburo/static/css/design-system.css`
- `politburo/infra/providers/data_provider.go`
- `politburo/infra/providers/airtable_provider.go`
- `politburo/infra/db/migrations/011_refactor_data_provider_configs.sql`

Existing behavior and architecture summary:

- Active holding config is split between `va_configs` and `va_data_provider_configs`, as documented in `plans/2026-05-29-holding-config-current-state-analysis.md`.
- Active route/job registration still enables Airtable-backed sync paths via `internal/routes/jobs.go`:
  - pilot sync job
  - route sync job
  - PIREP sync job
  - pilot sync worker
  - PIREP queue worker
- `internal/app/app.go` still wires legacy cache/config/service dependencies into `pilots.NewStatsService(...)` and `pireps.NewService(...)`.
- Current `internal/pilots/stats_service.go` is a large legacy-sensitive service that manually fetches Airtable configs, rebuilds credentials, constructs Airtable filter formulas, and directly instantiates `providers.NewAirtableProvider(cache)`.
- `infra/providers/data_provider.go` defines a provider interface, but the current config DTOs and runtime usage are still Airtable-centric (`AirtableName`, `BaseID`, `FilterFormula`, Airtable field metadata fetch, etc.).
- Datasource UI already has schema-mapping patterns that can be reused, but `templates/partials/datasource-field-mapper.html` currently uses inline styles rather than shared component classes.

Relevant repo guidance discovered:

- Backend runtime/bootstrap must remain in `internal/runtime` / `internal/app/app.go`, and route/job registration must remain in `internal/routes/router.go` and `internal/routes/jobs.go`.
- New JSON work should use `internal/platform/httpdto`.
- New LiveAPI work MUST stay behind `infra/liveapi.Client`; do not re-expand legacy `internal/common.LiveAPIService`.
- Vizburo styling must use `static/css/design-system.css` tokens/components only.
- Large files needing split are explicitly called out, including `internal/pilots/stats_service.go`.

## Existing reuse

- Reuse modern provider-config storage in `va_data_provider_configs` instead of inventing a third unrelated config system unless feasibility forces it.
- Reuse `internal/platform/va/service.go` and `repo.go` as the modern persistence/cache boundary for provider configs.
- Reuse existing schema DTO shape in `internal/platform/va/config_dtos.go`, especially richer field metadata already supported there:
  - `display_name`
  - `is_user_visible`
  - `display_format`
  - `bot_metadata`
- Reuse active datasource UI flow in `internal/datasource/handler.go` and templates under `templates/partials/datasource-*.html`.
- Reuse `infra/liveapi.Client` / `infra/providers.LiveAPIProvider` for game/live data rather than legacy common services.
- Reuse shared design-system tokens in `static/css/design-system.css`; move datasource-field-mapper inline patterns into reusable classes/components rather than copying styles.

## Architecture decisions

1. **Disable sync registration at runtime first; do not delete implementation code in the same slice.**
   - Grounding: `internal/routes/jobs.go` is the canonical place where scheduled jobs and workers are activated.
   - Decision: remove or guard registration/startup of pilot sync, route sync, PIREP sync, pilot sync worker, and PIREP queue worker there before broader cleanup.

2. **Do not add a brand-new standalone `feature_configs` table first.**
   - Grounding: `va_data_provider_configs` already supports typed rows with `config_type`, versioning, validation state, and JSONB payloads (`internal/platform/va/model.go`, migration `011_refactor_data_provider_configs.sql`).
   - Decision: preferred first approach is a new `config_type` under `va_data_provider_configs` using `feature_pilot_stats` instead of a separate table.
   - Reason: this preserves one structured-config system rather than adding a third alongside `va_configs` and provider configs.

3. **Treat admin-configurable “extra pilot stats fields” as a feature-specific config type with a constrained schema, not arbitrary Airtable formula execution.**
   - Grounding: current provider abstraction is not strong enough for unrestricted cross-provider formula engines, and current `FilterFormula` support is Airtable-specific.
   - Decision: support a bounded set of feature cards/metrics/field extracts first, with explicit allowed sources and aggregation modes.

4. **Pilot stats rewrite MUST separate live-game data from provider-backed enrichment.**
   - Grounding: user requested that Infinite Flight fetch remains and uses generated client; current stats service mixes too many responsibilities.
   - Decision: split pilot stats into:
     - LiveAPI-backed game stats path via `infra/liveapi.Client`
     - provider-backed enrichment path driven by configured feature cards
     - cache/orchestration layer deciding freshness and fallback behavior

5. **Pilot stats SHOULD fetch live-game data and provider-backed enrichment concurrently, and MAY reuse live-flight cache for recent-flight cards.**
   - Grounding: current stats path mixes concerns, and the request explicitly asks for multithreaded live/provider calls plus reuse of cached live-flight data.
   - Decision:
     - run LiveAPI-backed stats and provider-backed feature-card fetches concurrently where both are enabled
     - evaluate maintaining a bounded per-user queue of the most recent 5 flight IDs from the existing live-flight cache/job path
     - use that bounded queue to populate a `last flights` card from cached flight objects by selecting non-null route/origin/destination data where available
   - Constraint: this bounded queue must remain cache-backed and cheap to update; it should not introduce a new polling loop or high-cardinality labels.


6. **Do not make pilot-stats UI fetch arbitrary Airtable rows on every page render without caching and strict limits.**
   - Feasibility finding: direct per-request fetches from both pilot and PIREP schemas can add latency and scale risk, especially if each dashboard open causes multiple provider reads and any aggregation requires scanning/filtering PIREP tables.
   - Decision: default design should favor cached per-user/per-VA stat cards with bounded refresh rules, and should reject or defer high-cardinality free-form aggregations.

7. **Stats delivery MUST be cache-first, support bounded manual refresh, and use one shared JSON route for bot and UI.**
   - Decision:
     - read from cache first
     - initial cache key target is `pilot:stats:profile:{user_id}` or a VA-scoped equivalent if implementation confirms that multi-VA collisions are possible; this MUST be resolved explicitly during implementation
     - initial TTL target is 20 minutes via a named constant
     - add a manual refresh action, but reject refresh if the previous refresh was within 1 minute and surface a clear toast/message in the UI
     - expose one proper OpenAPI-documented JSON response contract for pilot stats that serves both Comrade Bot and Vizburo UI through the same route

8. **Reusability work MUST consolidate datasource/admin UI pieces into shared components/classes if the current chips/cards are not already reusable.**
   - Grounding: datasource field mapper currently uses inline styles; repo standards prefer design-system CSS tokens and reusable partials.

Alternatives considered:

- **Separate `feature_configs` table**: possible, but not preferred because `va_data_provider_configs` already provides versioned typed config rows. --> Not required 
- **Store feature config in `va_configs`**: rejected for this slice because the desired structure is richer than key/value and will likely need JSON schema, validation state, and UI editing.
- **Allow arbitrary Airtable formulas/aggregators configured by admins**: high risk for rate limits, inconsistent provider portability, and hard-to-debug runtime failures. Keep as a later guarded extension only if a constrained safe subset is proven necessary.

Open questions / risks:

- Whether one `feature_pilot_stats` config type is sufficient, or whether broader feature-config namespacing is needed from day one.
- Whether any existing consumers still require synced local pilot/route/PIREP tables for user-visible functionality after sync disable.
- Whether `internal/pireps/service.go` and `tour_service.go` must be partially refactored in the same slice because they still depend on stale config retrieval patterns.
- Whether bootstrap schema `000_schema.sql` must be updated in the same slice to match the active `config_type` model.

## Repo-by-repo implementation plan

### politburo/

#### A. Disable sync jobs and workers

- Update `internal/routes/jobs.go` so the following are no longer registered/launched in active runtime:
  - `application.Features.PilotSyncJob`
  - route sync job created via `vaRoutes.NewSyncJob(...)`
  - `application.Features.PirepSyncJob`
  - `application.Features.PilotSyncWorker`
  - `application.Features.PirepQueueWorker`
- Keep:
  - session cache job
  - aircraft cache job
  - live flights cache job
  - flight plan worker/monitor
  - live flights webhook job
  - aircraft worker
- Add clear startup logging documenting that Airtable sync/runtime ingestion is intentionally disabled.
- Follow-up cleanup MAY remove dead DI wiring in `internal/app/app.go`, but only after runtime dependencies are verified.

#### B. Rewrite pilot stats service around a config accessor and feature cards

- Replace the legacy-heavy design in `internal/pilots/stats_service.go` with smaller responsibilities, likely split into new files under `internal/pilots/` such as:
  - live stats fetch/orchestration
  - provider config/accessor integration
  - feature-card evaluation
  - caching
- New stats path SHOULD:
  - keep current live Infinite Flight stats functionality
  - route all live game API calls through `infra/liveapi.Client` / `infra/providers.LiveAPIProvider`
  - execute live and provider-backed fetches concurrently where enabled
  - stop constructing Airtable config/credentials ad hoc in stats code
  - stop depending on legacy `common.VAConfigService` for provider retrieval except where basic callsign prefix/suffix remain in `va_configs`
  - expose a single JSON response contract suitable for both bot and Vizburo consumers

#### B1. Decompose `internal/pilots/stats_service.go` before feature-config UI work

- Observed current issue after the first backend-foundation slice:
  - `internal/pilots/stats_service.go` still mixes multiple responsibilities that belong in separate feature/platform/repository areas.
  - The file still owns membership lookup, provider orchestration, LiveAPI orchestration, PIREP route fallback reads, caching decisions, standardized field mapping, and career-mode-specific enrichment.
- Observed existing repo reuse that SHOULD be used before adding more stats logic:
  - `internal/platform/memberships/repo.go` and `service.go` already own platform membership/user-status reads.
  - `internal/memberships/service.go` already resolves current VA context and membership semantics for user-facing status flows.
  - `internal/platform/va/ProviderConfigAccessor` now exists as the typed Airtable config boundary and SHOULD be expanded/reused rather than bypassed.
  - `internal/pilots/repository.go` already owns pilot-domain persistence concerns and MAY host pilot-stats-specific membership/read-model queries if platform memberships is not the right owner.
- Required refactor direction before UI/config-editor work:
  - move raw membership lookup out of `StatsService.getUserMembership(...)`
  - introduce a dedicated read dependency for the pilot-stats subject context, loaded once per request, containing at minimum:
    - user ID
    - Discord ID
    - IF community ID
    - VA ID / VA name
    - role
    - callsign
    - `airtable_pilot_id`
    - `career_mode_pilot_id`
  - prefer implementing that read through one of:
    - `internal/platform/memberships` repository/service extension, or
    - a small pilot-domain read repository under `internal/pilots/`
  - avoid embedding raw SQL in `stats_service.go` once the replacement exists.
- Split `stats_service.go` into smaller collaborators/files under `internal/pilots/`, reusing existing package boundaries where possible:
  - `stats_subject_reader.go`
    - resolves the pilot stats subject once from memberships/users/VA context
  - `stats_liveapi_service.go`
    - LiveAPI user stats fetch and DTO mapping only
  - `stats_provider_service.go`
    - Airtable profile fetch by stored pilot ID or bounded lookup strategy
  - `stats_career_mode_service.go`
    - career-mode record fetch, fallback strategy, and transformation only
  - `stats_pirep_resolver.go`
    - recent PIREP and route-resolution reads from synced tables / bounded Airtable fallback
  - `stats_field_mapper.go`
    - standardized/provider-field response mapping and formatting only
  - keep `stats_service.go` as the top-level orchestration entry point only, or replace it with a thin facade.
- Specific dependency cleanup tasks:
  - remove `userRepo` from `StatsService` if still unused after decomposition.
  - remove `syncRepo` from `StatsService` if still unused after decomposition.
  - stop constructing `providers.NewAirtableProvider(cache)` inside the stats service constructor if a reusable injected provider is practical within repo conventions.
  - stop re-fetching membership multiple times inside one request path; the subject context MUST be loaded once and passed down.
  - fix or remove the currently dead cache-read branch in provider stats fetch, which reads cache but intentionally ignores the cached payload.
- Ownership rules for downstream implementation agents:
  - membership/user/VA affiliation reads SHOULD live in `internal/platform/memberships` or a justified pilot-domain read repo, not as raw SQL in the stats orchestrator.
  - direct table reads of `pirep_at_synced` SHOULD move behind a small repository/read helper rather than stay inline in the orchestrator.
  - Airtable-specific heuristics such as hard-coded fallback field names and `rec...` linked-record parsing SHOULD be isolated into provider/career/PIREP resolver helpers, not scattered through the top-level service.
  - `GetPilotStatusByCallsign` SHOULD be evaluated as a separate use case from profile/stats assembly and MAY move into its own smaller service if that reduces orchestrator bloat.
- Sequence recommendation relative to the broader plan:
  - do this decomposition immediately after the provider-config accessor slice and before adding `feature_pilot_stats` storage/editor/UI.
  - keep the external `/api/v1/pilot/stats` response contract stable during this decomposition unless a separately approved contract change is made.

#### C. Add a unified provider-config accessor on the modern platform side

- Introduce a platform-side accessor/service near `internal/platform/va/` that can answer in one place:
  - basic VA config (as needed)
  - provider credentials by provider type
  - schema by `config_type`
  - feature config by `config_type`
- This accessor should replace repeated logic currently duplicated across:
  - `internal/pilots/stats_service.go`
  - `internal/pilots/sync_job.go`
  - `internal/va_routes/sync_job.go`
  - `internal/pireps/service.go`
  - `internal/pireps/tour_service.go`
- It SHOULD sit in modern platform code, not in `internal/common/*` or legacy `internal/services/data_provider_config_service.go`.

#### D. Add feature-config storage model under provider configs

- Extend `va_data_provider_configs` usage with the new `config_type` `feature_pilot_stats`.
- Proposed payload scope for first version:
  - enabled cards/sections
  - source schema selection (`pilot`, `pirep`)
  - selected mapped fields to expose
  - allowed display formatting
  - optional bounded aggregation mode from a predefined enum
  - explicit card mode so runtime can distinguish direct field cards, latest-row cards, recent-flight-cache cards, and bounded aggregate cards
  - caching hint / freshness policy if needed
- DO NOT start with arbitrary provider-side formulas as free text.

#### E. Feasibility-constrained stat card design

- First version SHOULD support only bounded patterns such as:
  - direct pilot-row fields by configured field mapping
  - latest PIREP row lookup by pilot link/callsign using a bounded query
  - recent-flight cards backed by local flight cache / bounded user-flight queue
  - fixed aggregations such as count / sum / latest-value over a constrained date window when the provider capability and caching strategy support it
- First version SHOULD NOT support:
  - arbitrary formula entry
  - unrestricted cross-table joins
  - unbounded scans across all PIREPs on each user request
  - admin-authored provider-specific scripting

#### F. Caching strategy analysis and recommendation

- Current issue: direct on-demand provider calls for each dashboard or bot stats request may create unacceptable latency and rate-limit pressure.
- Recommended first-pass cache layers:
  - per-VA provider credentials/schema cache remains under modern platform service
  - per-user pilot stats profile cache, ideally VA-scoped if required after implementation review
  - bounded recent-flight cache/queue populated from existing live-flight job outputs, max length 5
  - 20-minute stats TTL via named constants
  - 1-minute manual-refresh cooldown tracked via cache key/timestamp
  - explicit cache invalidation when feature config or schema config changes
- Live game data freshness SHOULD remain governed separately via existing live/session caches and current LiveAPI patterns.
- If aggregation cards are approved, implementation MUST benchmark worst-case provider call counts and define rate-limit protections before enabling them broadly.

#### G. Datasource / feature-config UI

- Extend datasource/admin UI with a new feature-config editor for pilot stats.
- Reuse existing datasource mapping UX patterns where possible:
  - schema chips / labels / field cards / dropdown mapping structure
  - active status cards / selector layout
- If shared UI pieces are currently copy-paste or inline-styled, extract them into:
  - reusable partials under `templates/partials/`
  - reusable classes in `static/css/design-system.css`
- Specifically review `templates/partials/datasource-field-mapper.html`, which currently uses inline styles and is a good candidate for componentization.
- Additional UI direction from the current review:
  - make the left-hand Airtable field list single-row, fully clickable items
  - right-align the displayed field type on the left side
  - keep the right-side internal field list read-only with dual-tone styling to show mapped vs unmapped
  - clicking a left-side row should open a popout/chooser for the mapping target plus supported special characteristics, then refresh the partial via HTMX
  - this should be built as reusable partial/component work rather than one-off inline styles

#### H. Cleanup / deprecation follow-on

- After stats rewrite and sync disable land, follow-up slices SHOULD evaluate:
  - retiring `internal/services/data_provider_config_service.go`
  - retiring `internal/common/airtable_service.go`
  - shrinking or deleting now-unused sync jobs/workers
  - reducing old Airtable keys in `va_configs`

### comrade-bot/

Not applicable.

- No direct bot feature was requested.
- If pilot stats response shape changes materially, downstream bot-facing consumers may later need updates, but that is outside this slice unless the API contract changes.

### Vizburo UI if present/applicable

Applicable inside `politburo/` server-rendered UI.

- Add admin UI for feature-config editing under the active datasource/admin flow rather than inventing a parallel UI subsystem.
- Reuse existing server-rendered partial patterns and active design-system tokens only.
- Keep handlers thin; parsing, validation, and persistence logic belong in services/platform code.
- Mobile impact: likely low-priority admin surface; desktop-first is acceptable if explicitly documented, but forms should still remain usable on smaller widths.

### labour-bureau/

Not applicable unless observability follow-up requires dashboard/alert changes.

### API contracts/generated clients/shared configuration as applicable

- If the pilot stats JSON response shape changes, OpenAPI/spec work may be needed even though current pilot stats is not yet under generated registration spec.
- Shared configuration logic is applicable inside `politburo/internal/platform/va/*` and DB migrations.

## Developer guidelines for implementation agents

Boundary rules:

- MUST disable runtime sync registration through `internal/routes/jobs.go`; do not add ad hoc flags elsewhere first.
- MUST keep LiveAPI reads behind `infra/liveapi.Client` / provider adapters.
- MUST prefer modern platform/provider config services over `internal/common/*` and `internal/services/data_provider_config_service.go` for new work.
- MUST keep Vizburo handlers thin and styling in design-system CSS.
- MUST avoid adding free-form provider formula execution in the first slice.

Files likely to edit:

- `internal/routes/jobs.go`
- `internal/app/app.go`
- `internal/pilots/stats_service.go` and/or split replacement files in `internal/pilots/`
- `internal/platform/va/service.go`
- `internal/platform/va/repo.go`
- `internal/platform/va/config_dtos.go`
- `internal/datasource/handler.go`
- relevant datasource/admin templates under `templates/partials/`
- `static/css/design-system.css`
- `infra/db/migrations/` for any new `config_type` or schema fixes
- possibly `internal/pireps/service.go` if stale config retrieval must be normalized

Files/packages to avoid unless required:

- `internal/common/airtable_service.go` except for migration/retirement reference
- `internal/common.LiveAPIService`
- `infra/liveapi/generated/**`
- unrelated Discord bot code
- unrelated live-flight cache jobs unless needed for coupling verification

Implementation checklist:

1. **Disable runtime Airtable sync activity first**
   - remove scheduled registration/startup of pilot sync, route sync, PIREP sync, pilot sync worker, and PIREP queue worker in `internal/routes/jobs.go`
   - verify startup logs clearly state Airtable sync is disabled
   - confirm unrelated jobs/workers still start normally

2. **Introduce the modern provider-config accessor before rewriting consumers**
   - add one platform-side accessor for credentials, schemas, and `feature_pilot_stats`
   - keep new retrieval logic out of `internal/common/*` and out of stale `internal/services/data_provider_config_service.go`
   - prove cache invalidation behavior for credentials/schema/feature-config reads

3. **Finalize the `feature_pilot_stats` config shape and persistence**
   - add the new typed config usage under `va_data_provider_configs`
   - define the bounded payload structure for cards/query modes/formatting
   - decide and document whether a migration is needed for bootstrap/schema alignment

4. **Rewrite pilot stats service around cache-first orchestration**
   - keep live Infinite Flight reads behind generated-client-backed `infra/liveapi.Client`
   - execute live-game and provider-backed fetches concurrently where enabled
   - add cache-first profile lookup, 20-minute TTL, and 1-minute refresh cooldown
   - evaluate/add recent-flight queue reuse from live-flight cache with max length 5
   - keep one shared JSON response route for bot and Vizburo consumers

5. **Add or adjust admin/UI surfaces only after backend contract is stable**
   - extend datasource/admin UI with `feature_pilot_stats` editing
   - reuse datasource mapping patterns and make them componentized/reusable
   - move inline mapper styling into shared partials/classes/design-system CSS
   - add manual refresh UX with clear cooldown messaging/toast behavior

6. **Complete contract, testing, and observability work before considering cleanup**
   - update OpenAPI/spec coverage for the shared pilot stats route and any new admin JSON config endpoints
   - add focused tests for accessor/cache/stats/UI flows
   - add observability for config cache hits/misses, Airtable calls, base-wise activity, and refresh behavior
   - only then consider follow-on deprecation of legacy config services and dead sync code

## Auth scopes, claims, and context

- Existing pilot stats endpoint remains under `/api/v1/pilot/stats` with `AuthMiddleware` only; no new scope is implied unless implementation adds admin-only config APIs.
- The shared pilot stats JSON route SHOULD serve both bot and Vizburo consumers; implementation MUST preserve correct context/claims handling for both.
- Feature-config admin editing SHOULD remain under existing admin UI/API protections:
  - dashboard admin session auth for Vizburo editor
  - admin role middleware for any JSON config endpoints
- Claims/context propagation should continue using `auth.GetUserClaims(ctx)` rather than reparsing headers.
- VA context handling is central: all feature config must remain VA-scoped.
- Mobile classification/impact:
  - end-user stats API: applicable to bot/mobile consumers
  - admin feature-config editor: desktop-first acceptable, but not broken on mobile

## Migrations and data model

- Preferred data-model change: no new standalone table initially.
- Add support for the `feature_pilot_stats` `config_type` row in `va_data_provider_configs` rather than a new feature-config table.
- Verify whether DB migration is required only for seed/constraint/docs, or whether the app can begin using a new `config_type` without structural DB changes.
- Also verify mismatch between `000_schema.sql` and `011_refactor_data_provider_configs.sql`; if repo policy expects fresh bootstrap correctness, plan a compatibility fix.
- Backfill/compatibility:
  - existing VAs can operate without the new feature-config row by falling back to current/default pilot stats behavior
  - feature config should be optional, not mandatory for stats endpoint success
- Rollback:
  - runtime can ignore the new `config_type` row
  - sync jobs can be re-enabled in `jobs.go` if needed, but implementation should document consequences

## Error handling and response conventions

- Pilot stats endpoint MUST use `internal/platform/httpdto` envelopes.
- Config editor JSON endpoints, if added, MUST use `httpdto.WriteSuccess` / `WriteError`.
- Validation behavior for feature config SHOULD return explicit 422-style or well-structured validation errors rather than generic 500s.
- Provider fetch errors for extra stat cards SHOULD degrade gracefully where feasible:
  - live game stats may still return if provider enrichment fails
  - response may include partial enrichment or omit optional cards rather than failing the entire endpoint
- Avoid leaking provider credentials, raw Airtable error bodies, or arbitrary formulas to user-facing responses.

## Constants and configuration

- Keep feature-config type names and any cache key prefixes in shared constants rather than scattering string literals.
- `feature_pilot_stats` SHOULD be defined as a shared constant rather than repeated raw strings.
- Credentials remain secret and MUST NOT be re-exposed to UI reads.
- Consider standardizing cache keys mentioned in the analysis doc, e.g. `config:airtable_creds:{vaID}`, if implementation is already touching cache naming; otherwise treat as a follow-up unless justified.
- Any stat-card TTL defaults, refresh cooldowns, and recent-flight queue sizes SHOULD live in a named constant package rather than inline literals.

## Logging and monitoring

Observability agent tasks:

- Remove or update observability assumptions tied to disabled sync jobs/workers:
  - job duration metrics
  - processed-record counters
  - queue-worker metrics
  - alerts/dashboards that assume Airtable sync is active
- Add monitoring around the new stats path:
  - config-fetch cache hit/miss metrics for provider credentials, schemas, and `feature_pilot_stats`
  - cache hit/miss metrics for provider-enriched stats cards
  - manual refresh attempts, accepts, and cooldown rejections
  - bounded latency metrics for provider enrichment
  - Airtable call counters and latency for stats-related reads
  - low-cardinality, base-wise monitoring for Airtable activity so operators can identify which configured base is generating traffic; implementation MUST sanitize or alias base identity if treating raw base IDs as sensitive
  - provider error classification without high-cardinality labels
- Keep labels bounded; do not use Discord IDs, VA IDs at high cardinality unless already accepted in existing metric families and justified.
- Avoid logging provider credentials, raw request payloads with secrets, or free-form formula strings if later added.
- All config/cache/provider instrumentation SHOULD use the existing metrics/logging infrastructure and not introduce a second registry.

## API spec and generated code work

Swagger/OpenAPI agent tasks:

- Determine whether `/api/v1/pilot/stats` should become or remain spec-documented in an OpenAPI artifact.
- If response shape changes materially, update or author the correct spec source and model the standard envelope.
- The pilot stats route SHOULD become a proper OpenAPI-documented shared contract because it will serve both bot and UI consumers.
- Do not place Vizburo HTML routes in OpenAPI.
- If new JSON admin endpoints for feature config are added, they need operation IDs, auth declarations, request/response schemas, and error envelopes.
- No changes should hand-edit generated code; use existing generation conventions if a spec is introduced/updated.

## Documentation

- Update admin/developer docs describing datasource configuration and pilot stats behavior after implementation.
- Document that sync jobs are intentionally disabled if that becomes shipped behavior.
- Document supported feature-config capabilities and explicit non-goals (for example no arbitrary formulas in v1).
- If operator expectations change around caching/latency, add brief runbook notes.

## Frontend/Vizburo plan

- Add a pilot-stats feature-config editor within the active datasource/admin flow.
- Reuse datasource mapping concepts:
  - selected-field chips
  - internal-field cards
  - table/schema selectors
  - status badges
- Convert current inline datasource mapper styles into reusable component classes/partials if not already available.
- Use design-system CSS tokens only.
- No direct infra/provider access from templates.
- No polling for config editing; normal form/HTMX interactions are sufficient.
- Mobile: admin editor may be desktop-first, but stacked responsive layout should remain readable.

## Testing plan

Unit Testing agent tasks:

- Add focused tests for disabled job registration behavior in `internal/routes/jobs.go`.
- Add tests for the new provider-config accessor:
  - credentials fetch
  - schema fetch by type
  - feature-config fetch by type
  - cache invalidation behavior
- Add tests for rewritten pilot stats service:
  - liveapi-only success path
  - provider-enriched path
  - concurrent live + provider fetch orchestration
  - partial enrichment fallback on provider failure
  - feature-config parsing/validation
  - cache hit/miss behavior
  - manual refresh cooldown behavior
  - recent-flight cache/queue behavior for last-flight cards
- Add UI handler tests for feature-config save/render flows if feasible.
- Regression coverage should verify no new code reaches retired `internal/common.LiveAPIService`.
- Manual verification:
  - pilot stats still returns live-game data
  - optional configured extra cards render as expected
  - disabling sync jobs does not break startup
  - datasource UI still edits credentials/schemas and new feature config

## Execution order for specialized agents

1. **Developer/implementation agent** — disable sync registration/startup, add config accessor, rewrite stats service, add feature-config model and admin UI, normalize reusable components.
2. **Swagger/OpenAPI agent** — update/specify pilot stats and any new admin JSON config endpoints if response/contract changes.
3. **Unit Testing agent** — add focused backend/UI tests and regression coverage.
4. **Observability agent** — remove stale sync-job assumptions and instrument the new stats/cache path.
5. **Docs maintainer** — reconcile admin/developer docs after shipped behavior is verified.

## Out-of-scope items

- Full provider-agnostic adapter overhaul across the whole repo.
- Deleting all legacy Airtable/common services in the same slice.
- Rebuilding every PIREP/tour/provider flow to be fully provider-neutral.
- Arbitrary admin-authored formula language or unrestricted provider-side aggregations.
- Broad Discord bot feature redesign.
- Deep refactor of unrelated events/live-flight/dashboard features.

## Final checklist

- Planner modified no source/runtime/config/generated files outside this plan document.
- Plan file path: `politburo/plans/2026-05-29-feature-configs-stats-and-sync-disable-plan.md`
- Plan status is final and ready for downstream implementation work.
- Key downstream tasks:
  - disable Airtable sync jobs/workers in runtime registration
  - add modern provider-config accessor
  - store pilot-stats feature config as the `feature_pilot_stats` provider `config_type`
  - rewrite `internal/pilots/stats_service.go`
  - keep Infinite Flight reads behind generated-client-backed `infra/liveapi.Client`
  - add bounded caching and reusable admin UI components
