# Live Command and Infinite Flight Live API Ecosystem

Status: discovery / solution-shaping plan, 2026-05-21  
Requested change summary: map the current `/live` Discord command and Infinite Flight Live API flow before deciding on a future solution.  
Scope and assumptions: this is a repo-grounded ecosystem plan only. No source changes are proposed for immediate implementation. The observed active path is the cache-first `GET /api/v1/flights/va` + Vizburo `/dashboard/live` flow, not the older direct `flights.Service.GetVALiveFlightsFromCache` helper.

## 1. Title and status

- Plan file: `politburo/plans/live-command-flight-ecosystem.md`.
- Status: ready for discussion; downstream work should start with an explicit solution goal.
- Scope: sessions, session flights, per-flight route/flight-plan lookup, Redis cache keys/TTLs, phase/waypoint calculations, Discord command rendering, Vizburo map, webhook snapshot reuse, and observability/spec/testing implications.

## 2. Context

Files/packages inspected:
- Guidance: `AGENTS.md`, `politburo/CLAUDE.md`, `politburo/TECHNICAL_STANDARDS.md`.
- DI/runtime/routes/jobs: `internal/app/app.go`, `internal/routes/router.go`, `internal/routes/jobs.go`.
- Live API wrapper/spec: `infra/liveapi/client.go`, `api/openapi/liveapi.yaml`, `api/openapi/liveapi.cfg.yaml`.
- Cache/constants: `infra/cache/keys.go`, `infra/cache/ttl.go`.
- Live data jobs/services/DTOs: `internal/sessions/cache_job.go`, `internal/flights/cache_job.go`, `internal/flights/model.go`, `internal/flights/dto.go`, `internal/flights/service.go`, `internal/flights/handler.go`, `internal/flights/flight_plan_worker.go`, `internal/flights/flight_plan_queue_monitor.go`.
- Consumers: `comrade-bot/src/commands/live.ts`, `comrade-bot/src/commands/liveHandler.ts`, `comrade-bot/src/services/apiService.ts`, `comrade-bot/src/helpers/LiveTableRenderer.ts`, `comrade-bot/src/handlers/InteractionRouter.ts`, `politburo/templates/pages/live.html`, `politburo/static/js/live-flights.mjs`, `internal/webhooks/live_flights_webhook_job.go`.

Existing behavior summary:
- `internal/sessions.CacheJob` runs every 5 minutes and calls `infra/liveapi.Client.GetSessions()`, caching `game:sessions`, `game:session:<session_id>`, and `game:session:name:<session_id>` for `cache.SessionTTL` (24h).
- `internal/flights.CacheJob` runs every minute, reads `game:sessions`, calls `GetFlights(sessionID)` for each session, filters callsigns against active VA prefix/suffix configs from `platform/va.Repository.GetAllActiveVACallsignConfigs`, builds `CompleteFlight`, appends waypoints, writes `game:live:flight:<flight_id>` and `game:live:vaflights:<va_id>`.
- Route/origin/destination enrichment is asynchronous: `FlightsCacheJob` enqueues `flight_plan_queue`; `FlightPlanWorker` calls `GetFlightPlan(sessionID, flightID)`, caches `game:flightplan:<flight_id>` plus legacy `FPL` key, and updates `CompleteFlight.Origin/Destination/LastFlightPlanFetch`.
- `/api/v1/flights/va` and `/dashboard/live` both use `flights.GetVALiveFlightsDTOs(redisCache, vaID)`, so the Discord image, signed map link, Vizburo map, and live-flights webhook share the same cached `CompleteFlight` data.

## 3. Existing reuse

- Reuse `infra/liveapi.Client`; do not import generated `infra/liveapi/generated` outside infra.
- Reuse cache constants in `infra/cache/keys.go` and TTLs in `infra/cache/ttl.go`; do not hardcode cache keys/TTL values at new call sites.
- Reuse `CompleteFlight`, `WaypointSnapshot`, `VALiveFlightDTO`, and `GetVALiveFlightsDTOs` for read-side consumers.
- Reuse `auth.Service.GenerateSignedLink` for Discord "See Map" links.
- Reuse `infra/metrics.MetricsRegistry` for LiveAPI wrapper, jobs, queue, webhook, and HTTP metrics.
- Reuse existing bot `ApiService.getLiveFlights`, `liveHandler.ts` pagination, and `LiveTableRenderer.ts` if Discord remains a table/image surface.

## 4. Architecture decisions

- The canonical live-flight read model is Redis `CompleteFlight` objects keyed by `game:live:flight:<flight_id>` plus VA ID lists keyed by `game:live:vaflights:<va_id>`.
- Session discovery and flight discovery are scheduled jobs wired through `internal/routes/jobs.go`; no new polling loop should be added in Discord or Vizburo.
- Upstream calls must stay behind `infra/liveapi.Client`, whose wrapper currently records bounded LiveAPI request metrics and hides generated-client details.
- `GET /api/v1/flights/va` is cache-only and should remain fast; if a future feature needs refresh behavior, plan it as a job/event/explicit refresh path, not per-request fanout to Infinite Flight.
- Open question/risk: `flights.Service.GetVALiveFlightsFromCache` tries legacy keys like `if:session:callsigns:<sid>` and `if:session:<sid>:flight:<callsign>` that the observed `FlightsCacheJob` no longer writes. Downstream agents should verify active imports before deleting or changing it.
- Open question/risk: `CompleteFlight` comments still mention 7-day TTL in places, but `infra/cache/ttl.go` currently sets live flight and flight plan TTLs to 48h.

## 5. Repo-by-repo implementation plan

### politburo/
- For any future solution, start in `internal/flights` and preserve the current cache-first read path.
- If adding/changing API response fields, update `VALiveFlightDTO`, `GetVALiveFlightsDTOs`, handlers in `internal/flights/handler.go`, and relevant consumers.
- If changing upstream LiveAPI behavior, edit only `infra/liveapi` wrapper/spec/generation path and keep domain packages insulated.

### comrade-bot/
- `/live` is `src/commands/live.ts` -> `handleLiveFlights` -> `ApiService.getLiveFlights` -> `GET /api/v1/flights/va`.
- Pagination is client-side by re-fetching full live-flight list on button clicks (`live_prev_*`, `live_next_*`). Any future optimization should preserve thin command behavior and keep HTTP calls in `ApiService`.

### Vizburo UI
- `/dashboard/live` renders `templates/pages/live.html`; the browser JS reads embedded JSON and maps flights with `static/js/live-flights.mjs`/`flight-map.mjs`.
- Current UI does not poll; it renders a snapshot. Preserve this unless a specific refresh mechanism is planned.

### labour-bureau/
- No direct change required for understanding. Future new metrics/ports/services must be reflected in dev/prod observability only if runtime shape changes.

### API contracts/generated clients/shared configuration
- Upstream LiveAPI contract lives in `api/openapi/liveapi.yaml` with generated output under `infra/liveapi/generated/`.
- There is no observed public Politburo OpenAPI spec for `GET /api/v1/flights/va`; any future formalization must model the response envelope and bot headers.

## 6. Developer guidelines for implementation agents

- MUST keep jobs registered through `internal/routes/jobs.go` and dependencies through `internal/app/app.go`.
- MUST not call Infinite Flight directly from comrade-bot, Vizburo handlers, route handlers, or templates.
- SHOULD split or isolate changes in `internal/flights/service.go` if touching legacy direct-fetch paths; it is a known large/split candidate.
- Files likely to edit for a future live-flight solution: `internal/flights/*`, `infra/cache/*`, `infra/liveapi/*`, `internal/routes/jobs.go`, `internal/app/app.go`, `comrade-bot/src/services/apiService.ts`, `comrade-bot/src/commands/liveHandler.ts`, `templates/pages/live.html`, `static/js/live-flights.mjs`.
- Files/packages to avoid unless required: `internal/common.LiveAPIService`, `infra/liveapi/generated/**`, unrelated PIREP/Airtable legacy services.

## 7. Auth scopes, claims, and context

- `/api/v1/flights/va` runs behind `AuthMiddleware`; it reads `auth.GetUserClaims(r.Context())` and uses `claims.ServerID()` as VA ID.
- Comrade Bot currently calls general endpoints with legacy `X-Discord-Id` and `X-Server-Id` via `generateMetaHeaders`; registration paths use newer `X-Discord-User-Id` and `X-Discord-Server-Id`.
- Future API work should prefer new header names while preserving compatibility intentionally.
- VA context is central: all live filtering is VA callsign prefix/suffix based, not Airtable-dependent.
- Mobile impact: Vizburo `/dashboard/live` has list/map toggle and bottom-sheet details; Discord output is image/table plus link.

## 8. Migrations and data model

- Current live-flight/session/route state is Redis operational cache only; no DB migration is involved.
- VA callsign configs come from existing VA config/repository tables; future schema changes need explicit migration/backfill/rollback planning.
- Do not warehouse raw Infinite Flight responses; standards require temporary operational cache behavior.

## 9. Error handling and response conventions

- Existing live-flight API uses legacy `common.RespondSuccess/RespondError`, while standards prefer `internal/platform/httpdto` for new JSON handlers.
- Cache miss for VA flight list returns an empty flight array, not a 404.
- Flight detail/waypoint endpoints return 404 if the flight object is absent from cache.
- Future new endpoints should use `httpdto` envelopes, validation errors, and explicit 401/403/404/422 behavior.

## 10. Constants and configuration

- Required upstream config is `IF_API_BASE_URL` and `IF_API_KEY` through `infra/liveapi.Client`; `IF_API_KEY` must be bearer auth, not query string.
- Important cache constants: `KeySessionList`, `SessionKey`, `SessionNameKey`, `LiveFlightsKey`, `LiveFlightKey`, `LiveVAFlightsKey`, `FlightPlanKey`.
- Important TTLs: `SessionTTL` 24h, `LiveFlightListTTL` 5m, `LiveFlightTTL` 48h, `FlightPlanTTL` 48h.

## 11. Logging and monitoring

- Observability agent tasks for future changes:
  - Verify LiveAPI metrics labels remain bounded: provider, endpoint_group, status_class, error_type.
  - Review session/flight cache job duration and cache-size metrics coverage after any cadence/TTL/key changes.
  - Preserve queue metrics for `flight_plan_queue`: depth, pending, enqueued, dequeued, acknowledged, errors, retries, processing duration.
  - Do not log or label API keys, Discord IDs, guild IDs, raw request paths, session IDs, flight IDs, or user IDs in metric labels.
  - Update dashboards/alerts only if new metrics or changed semantics are introduced.

## 12. API spec and generated code work

- Swagger/OpenAPI agent tasks for future changes:
  - If upstream Infinite Flight contract changes, update `api/openapi/liveapi.yaml`, regenerate `infra/liveapi/generated/**` via the repo target, and update wrapper tests.
  - If Politburo exposes/changes bot-facing live endpoints, add/extend the appropriate public API spec with operation IDs, envelope schemas, auth headers/security, `VALiveFlightDTO` schemas, and error envelopes.
  - Do not mix upstream LiveAPI spec and Politburo public API spec artifacts.

## 13. Documentation

- Future shipped behavior should update bot help for `/live`, Vizburo live page docs if present, and operator notes for LiveAPI cache/queue behavior if runtime semantics change.
- Do not expose internal cache keys or Infinite Flight mechanics in pilot-facing copy.

## 14. Frontend/Vizburo plan

- Keep Vizburo handlers thin; use `GetVALiveFlightsDTOs` or a future domain service, not direct Redis/LiveAPI from templates or JS.
- Styling must continue using design-system CSS tokens/components.
- Preserve no-poll snapshot behavior unless a plan explicitly adds idle-aware refresh/push semantics.
- Mobile behavior must retain list/map toggle and details sheet or document any desktop-first tradeoff.

## 15. Testing plan

- Unit Testing agent tasks for future changes:
  - `internal/sessions`: session cache job success/error/cache-key tests.
  - `internal/flights`: VA pattern filtering, `CompleteFlight` build/preservation, phase transitions, waypoint append/pruning, normalization, DTO max speed/altitude, flight-plan enqueue intervals, route extraction.
  - `infra/liveapi`: httptest wrapper tests for sessions, flights, flight route, flight plan, error codes, rate limits, UUID validation, and metrics observations.
  - Handler tests for `/api/v1/flights/va`, `/api/v1/flights/{flight_id}`, and `/dashboard/flights/{flight_id}/waypoints` cache miss/hit behavior.
  - Bot build/typecheck: `npm run build`; add tests only if a test framework exists or is planned.
  - Manual verification: run dev stack, seed/observe cache jobs, use `/live`, click pagination, open signed map link, inspect route/waypoints.

## 16. Execution order for specialized agents

1. Product/architect discussion: decide the actual problem to solve using this ecosystem map.
2. OpenAPI agent only if public API or upstream LiveAPI spec changes are required.
3. Developer agent for backend cache/job/API changes first.
4. Comrade Bot/Vizburo developer changes after backend contract stabilizes.
5. Unit Testing agent for focused coverage.
6. Observability agent if metrics/runtime/cadence/queue behavior changes.
7. Documentation agent after implementation lands.

## 17. Out-of-scope items

- No code, tests, migrations, config, generated files, Docker files, dashboards, or docs outside this plan.
- No real Infinite Flight API calls.
- No deletion/refactor of legacy live-flight helpers without a separate approved scope.
- No new polling behavior.

## 18. Final checklist

- Source modifications avoided by planner: yes; only this markdown plan was added.
- Plan file path: `politburo/plans/live-command-flight-ecosystem.md`.
- Key downstream tasks: choose solution goal; verify legacy cache helper status; preserve cache-first live read model; keep OpenAPI/observability/testing work explicit if behavior changes.
