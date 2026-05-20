# Vizburo UI Architecture Implementation Plan

> Status: Ready for phased implementation  
> Date: 2026-05-20  
> Source review: `politburo/plans/vizburo-ui-architecture.md`  
> Requested change: translate the broad Vizburo UI architecture review into implementation-ready slices without changing source code.

## 1. Title and status

- **Plan file:** `politburo/plans/vizburo-ui-architecture-implementation.md`
- **Scope:** risk-bounded implementation plan for Vizburo renderer consolidation/caching, UI context middleware, handler cleanup, tokenized styling, mobile classification, and later performance polish.
- **Assumptions:**
  - The review is sufficient to authorize implementation, but some claims are stale: current routed UI handlers mostly live in `internal/*` packages and render from root `templates/`, while `vizburo/ui/*` appears unimported by Go source.
  - Downstream agents MUST verify imports before deleting `vizburo/ui/*` or `vizburo/ui/templates/*`.
  - No product decision is needed for the first implementation slice; it is a non-visible renderer/cache refactor.
- **Recommended first slice:** add template caching/reload policy to `infra/templates.Renderer` and update tests around render behavior before touching handlers, routes, or templates.

## 2. Context

### Files/packages inspected

- Guidance: `AGENTS.md`, `politburo/CLAUDE.md`, `politburo/plans/vizburo-ui-architecture.md`.
- Runtime/DI/routes/jobs: `cmd/vizburo/main.go`, `internal/runtime/server.go`, `internal/app/app.go`, `internal/app/config.go`, `internal/routes/router.go`, `internal/routes/jobs.go`.
- Renderer/template helpers: `infra/templates/renderer.go`, `infra/templates/session_helpers.go`.
- Current routed UI handlers: `internal/dashboard/handler.go`, `internal/datasource/handler.go`, `internal/events/handler.go`, `internal/vaadmin/handler.go`, `internal/vaadmin/webhooks_handlers.go`, `internal/pilots/handler.go`, `internal/flights/handler.go`, `internal/liverymappings/handler.go`.
- Legacy/unrouted Vizburo package: `vizburo/ui/utils.go`, `vizburo/ui/template_helpers.go`, `vizburo/ui/handlers.go`, `vizburo/ui/pirep_config_handlers.go`, `vizburo/ui/auth.go`, `vizburo/ui/templates/**`.
- Templates/assets: root `templates/layouts/base.html`, `templates/pages/**`, `templates/partials/**`, `static/css/design-system.css`, `static/css/output.css`, `package.json`, `tailwind.config.js`, `vizburo/ui/input.css`, `vizburo/ui/tailwind.config.js`.

### Existing behavior and architecture summary

- `cmd/vizburo/main.go` uses `runtime.Bootstrap()` and `runtime.NewVizburoServer(application)`, so Vizburo now shares `app.App` DI and `routes.NewRouter` with the API server.
- `internal/runtime.NewVizburoServer` disables metrics and jobs and uses `Config.VizburoPort`; API server enables `/metrics`, jobs, workers, and Watermill.
- `internal/app/app.go` is the DI container. `InfraDeps.TemplateRenderer` is built once with `templates.NewRenderer("templates", "templates/partials", "templates/layouts/base.html")`.
- Dashboard routes are currently registered directly in `internal/routes/router.go` under `/dashboard`, using `uiAuthMiddleware`, role middleware, and handlers from `application.Features.*` except two free functions in `internal/flights/handler.go`.
- `infra/templates.Renderer` still parses partials/page/layout files on every `RenderTemplate`, `RenderPartial`, and `RenderStandalone` call.
- `infra/templates.PrepareTemplateData` is canonical and includes `MenuItems`; `vizburo/ui/template_helpers.go` is an older duplicate using `log.Printf`.
- Root `templates/layouts/base.html` already uses `static/css/design-system.css` and mobile partials (`mobile-header`, `mobile-drawer`, `mobile-fab`), but it still has inline `style="..."` exceptions. Legacy `vizburo/ui/templates/layouts/base.html` still embeds Nord tokens and heavy inline styles.
- `grep` found no active imports of `vizburo/ui` outside the package itself; treat that package as legacy/unrouted until implementation verifies with `go list`/tests.

### Relevant repo guidance discovered

- Routes MUST be registered through `internal/routes/router.go` and existing `application.Features.*` wiring.
- Jobs/workers MUST remain in `internal/routes/jobs.go`; this plan introduces no jobs.
- Infrastructure belongs in `infra/`; renderer caching belongs in `infra/templates`.
- API JSON responses should use `internal/platform/httpdto`; UI/HTMX currently use HTML partials plus occasional `http.Error`.
- Add metrics through `infra/metrics.MetricsRegistry`; do not create a second Prometheus registry.

## 3. Existing reuse

- Reuse struct-based handler constructors in `internal/dashboard`, `internal/datasource`, `internal/events`, `internal/vaadmin`, `internal/liverymappings`.
- Reuse `infra/templates.Renderer` as the only renderer; do not keep `vizburo/ui/utils.go` rendering code.
- Reuse `infra/templates.PrepareTemplateData` and `internal/platform/ui.GetMenuItems` behavior for role-aware page data.
- Reuse `auth.GetSessionData`, `auth.SetSessionData`, `auth.SetUserClaims`, and role middleware from `internal/routes/router.go`/`internal/middleware`.
- Reuse `static/css/design-system.css` token/component vocabulary (`.app-shell`, `.desktop-tenant-sidebar`, `.main-content`, `.nav-link`, mobile drawer/FAB classes).
- Reuse existing validation commands from repo guidance: `go test ./...`, focused UI-adjacent packages, `npm run css:build`, and builds for `./cmd/server` / `./cmd/vizburo`.

## 4. Architecture decisions

- **Slice 1 first:** cache parsed templates in `infra/templates.Renderer` because it is low-risk, non-visible, and improves every current root-template UI path.
- **Renderer remains infrastructure:** implement caching/reload in `infra/templates`; handlers must keep using injected `*templates.Renderer`, not instantiate ad hoc renderers per request.
- **Development reload policy:** use `app.Config.AppEnv` or an explicit renderer option to re-parse in `local` only. Avoid global env reads inside render hot paths if DI can pass the choice from `internal/app/app.go`.
- **UI context is second:** add a typed UI context middleware after `uiAuthMiddleware`, not before, because `uiAuthMiddleware` currently validates session and sets auth/session data on context.
- **Feature wiring:** if new UI handler containers are added, they MUST be fields under `app.FeatureDeps` and routes MUST call `application.Features.*`. Do not build handlers inside route closures except temporary adapters during a slice.
- **Legacy package deletion is gated:** `vizburo/ui/*` can only be removed after `go list ./...`, `grep`, and builds prove no active imports and after equivalent template assets are confirmed under root `templates/`.
- **Styling source of truth:** keep Deep Void `static/css/design-system.css` as the canonical token system. Nord implementation in `vizburo/ui/*` is not the target.
- **No polling:** do not add interval polling. Existing HTMX interactions may remain; new mobile/partial behavior should be request-triggered by navigation/user action or existing server data paths.
- **Open question:** root `templates/layouts/base.html` includes `/dashboard/switch-va` HTMX controls, but no route was found in `internal/routes/router.go`; implementation should verify whether this is dead UI or a missing handler before changing VA switch behavior.

## 5. Repo-by-repo implementation plan

### politburo/

#### Phase 1 — Renderer caching and consolidation baseline (first slice)

- Likely files to edit:
  - `infra/templates/renderer.go`
  - `internal/app/app.go` only if constructor/options need `AppEnv` or debug reload policy
  - new tests near `infra/templates/*_test.go` if no suitable tests exist
- Tasks:
  - Add a cache inside `templates.Renderer` for full-page, partial, and standalone template parse results, keyed by template name plus render mode.
  - Parse shared partials once per renderer instance in non-local environments; allow local reparse for template editing.
  - Preserve existing function map behavior: `safeHTML`, `split`, `mod`, `dict`, `json`, `jsEscape`, `default`, `formatTime`, `formatFlightTime`, `formatDuration`.
  - Preserve current execution behavior: full pages execute layout, partials execute filename-derived block then `content`, standalone uses `error.html`.
  - Do not change template paths or route registration in this slice.
- Validation:
  - `go test ./infra/templates ./internal/routes ./internal/dashboard ./internal/datasource ./internal/events ./internal/vaadmin ./internal/pilots ./internal/flights ./internal/liverymappings`
  - `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - `go build -buildvcs=false -o .air_tmp/vizburo ./cmd/vizburo`
- Blast-radius checks:
  - Verify 401/404/405 standalone rendering still works.
  - Verify `RenderPartial` still handles nested partial paths like `partials/flight-map/with-route.html` if those are kept.
  - Stop if cached templates break local template editing or any template block name collision appears.

#### Phase 2 — UI context middleware and helper

- Likely files to edit:
  - New small package/file: `internal/platform/ui/context.go` or `internal/ui/context.go` only after choosing the least cyclic location.
  - `internal/routes/router.go`
  - UI handlers in `internal/dashboard`, `internal/datasource`, `internal/events`, `internal/vaadmin`, `internal/pilots`, `internal/flights`, `internal/liverymappings`.
- Tasks:
  - Introduce `UIContext` with `Session`, `ActiveVA`, `IsAdmin`, `IsStaff`, `IsPilot`, `MenuItems`, and later `IsMobile`.
  - Add typed `FromContext`/`MustContext` accessors; prefer returning `(ctx, ok)` in handlers that can render controlled errors.
  - Middleware should run after `uiAuthMiddleware` and use session data already placed by `auth.SetSessionData`.
  - Replace repeated session extraction and `templates.PrepareTemplateData` calls incrementally, one package at a time.
  - Preserve role middleware (`IsMemberMiddleware`, `IsStaffMiddleware`, `IsAdminMiddleware`) as the authorization boundary.
- Validation:
  - Focused tests/builds from Phase 1.
  - Manual smoke: `/auth/login`, `/dashboard/`, `/dashboard/live`, `/dashboard/logbook`, `/dashboard/events`, `/dashboard/settings/datasource`, `/dashboard/vaadmin/*`.
- Stop conditions:
  - Any import cycle involving `internal/auth`, `infra/session`, `internal/platform/ui`, or feature handlers.
  - Any behavior change from redirect/page 401 to raw `http.Error` on full-page requests.

#### Phase 3 — Handler consolidation by page area

- Likely files to edit:
  - `internal/app/app.go`
  - `internal/routes/router.go`
  - `internal/flights/handler.go` and/or a new UI-focused handler file in `internal/flights`
  - `internal/pilots/handler.go`, `internal/events/handler.go`, `internal/vaadmin/handler.go`, `internal/datasource/handler.go`, `internal/dashboard/handler.go`
- Tasks:
  - Convert remaining routed UI free functions (`flights.LiveFlightsPageHandler`, `flights.GetFlightWaypoints`) to injected handler methods when dependencies are already in `app.App`.
  - Remove per-request renderer construction in `internal/flights.LiveFlightsPageHandler`; use `application.Infra.TemplateRenderer` via a handler field.
  - Split very large handlers only along existing package boundaries; do not create dead-end packages.
  - Keep business logic in services/repos (`internal/flights.Service`, `internal/dashboard.Service`, `internal/events.Service`, etc.), not in templates or route closures.
- Validation:
  - `go test ./internal/flights ./internal/pilots ./internal/events ./internal/vaadmin ./internal/datasource ./internal/dashboard ./internal/liverymappings ./internal/routes`
  - Builds for both binaries.

#### Phase 4 — Legacy `vizburo/ui` retirement or quarantine

- Verification first:
  - `grep`/`go list` must prove `vizburo/ui` is not imported by any active package.
  - Confirm root `templates/**` contains replacements for any needed `vizburo/ui/templates/**` assets.
- If verified unused, downstream implementation may delete or quarantine only in a dedicated cleanup slice with tests/builds.
- If still used by an uninspected command, adapt it to `app.App` and `infra/templates.Renderer` instead of keeping duplicate helpers.

#### Phase 5 — Design-system theme unification

- Likely files to edit:
  - `templates/layouts/base.html`
  - `templates/pages/**`, `templates/partials/**`
  - `static/css/design-system.css`
  - `tailwind.config.js`, `vizburo/ui/input.css`, maybe `package.json`
  - `vizburo/ui/tailwind.config.js` only if retiring legacy build inputs is verified safe
- Tasks:
  - Convert inline style/hardcoded color usage in active root templates to design-system classes/tokens.
  - Keep any unavoidable dynamic inline values narrowly documented (e.g., computed widths).
  - Extend root `tailwind.config.js` content globs to include active root `templates/**/*.html`; current config only scans `vizburo/ui/templates/**/*.html` while active layouts use root `templates/`.
  - Ensure `npm run css:build` remains the single CSS build path to `static/css/output.css` if Tailwind output is actually used; root layout currently links `design-system.css`, not `output.css`.
- Validation:
  - `npm run css:build`
  - `go test ./infra/templates ./internal/routes`
  - Browser smoke across dashboard, live, logbook, events, datasource, VA admin.
- Stop conditions:
  - Product rejects Deep Void as the canonical theme.
  - Build pipeline ambiguity remains between `design-system.css` and `output.css`.

#### Phase 6 — Mobile classification and viewport context

- Likely files to edit:
  - UI context middleware from Phase 2
  - `templates/layouts/base.html`
  - New/active partials under `templates/partials/**`
- Tasks:
  - Add `IsMobile` to `UIContext` from `vp=mobile|desktop` cookie, falling back to user-agent sniff; default ambiguous clients to mobile.
  - Add a tiny head script to set `vp` before HTMX requests; do not write viewport into Redis session.
  - Classify pages explicitly:
    - **Mobile-first:** `/dashboard/live`, `/dashboard/` leaderboard/stats surfaces.
    - **Mobile-compatible but not first:** `/dashboard/logbook` after template audit.
    - **Mobile-incompatible guard:** datasource configuration, events management/legs, VA admin pilots/flight modes/webhooks, livery mappings.
  - Implement guard partial/page for mobile-incompatible admin workflows.
- Stop condition: do not split heavy templates until product accepts per-page mobile classification.

#### Phase 7 — Performance polish after structural cleanup

- Candidate tasks:
  - Parallelize independent data calls only where services support context cancellation.
  - Cache route/downsample outputs through `infra/cache.RedisCacheService` with low-cardinality keys and TTLs.
  - Scope HTMX indicators to partial containers instead of a global full-screen spinner.
- Must not add polling.

### comrade-bot/

- Not directly in scope for the first renderer/UI architecture slices.
- Verify only if routes or signed-link destinations change. Bot HTTP calls should remain in `comrade-bot/src/services/apiService.ts`; header generation remains in `src/helpers/utils.ts`.
- No slash command changes are expected.

### Vizburo UI

- Active UI appears to be root `templates/**` plus `static/css/design-system.css`, served by Politburo. Legacy `vizburo/ui/**` should not receive new feature work unless proven active.
- Thin handler rule: handlers assemble context/data and call services/renderers; no business logic in templates or route closures.

### labour-bureau/

- Not applicable for early slices.
- Observability/infra agents should check `labour-bureau/prometheus.dev.yml`, `labour-bureau/promtail-config.yml`, and prod compose only if new UI metrics or container labels are added later.

### API contracts/generated clients/shared configuration

- No API contract changes expected for renderer/context/styling slices.
- If any JSON endpoint under `/api/v1` changes, Swagger/OpenAPI work must update `api/openapi/registration.yaml` only when it is in that contract scope and run `make generate-api`; do not hand-edit `internal/api/generated/**`.

## 6. Developer guidelines for implementation agents

- MUST route through existing `app.App` DI and `application.Features.*` fields.
- MUST keep jobs/workers unchanged unless a later cache-warming job is explicitly approved; then wire only through `internal/routes/jobs.go`.
- MUST keep infrastructure concerns in `infra/`, especially renderer caching and metrics.
- MUST keep Vizburo handlers thin; move data access/business rules to existing services.
- SHOULD implement one page/package slice at a time and run focused tests after each.
- SHOULD avoid touching `static/css/output.css` unless running the CSS build is part of the slice and expected.
- Avoid `internal/services`, `internal/common`, `internal/db/repositories` for new UI architecture unless existing services still require them during migration.
- Do not delete `vizburo/ui/**` until a dedicated verification slice proves it is unused.

## 7. Auth scopes, claims, and context

- Dashboard route groups already enforce:
  - `/dashboard` base: `uiAuthMiddleware` session auth.
  - Member routes: `middleware.IsMemberMiddleware()`.
  - Staff routes: `middleware.IsStaffMiddleware()`.
  - Admin routes: `middleware.IsAdminMiddleware()`.
- `uiAuthMiddleware` sets `auth.APIKeyClaims` and session data using active VA fields; preserve this behavior.
- New `UIContext` MUST derive VA context from the session active VA and carry `ActiveVAID`, role flags, and menu items consistently.
- Full-page missing session should remain a rendered 401 or redirect per existing route behavior; HTMX partials should return consistent 401 fragments/statuses.
- **VA context:** every data query must use `UIContext.ActiveVA.VAID` or existing claims `ServerID()` as appropriate; do not trust form/query VA IDs without authorization checks.
- **Mobile classification:** mobile context is request/browser scoped via cookie/UA, not user/session scoped.

## 8. Migrations and data model

- Not applicable for Phases 1–6: no schema/data model changes.
- If later RDP/route caching uses Redis only, no DB migration is needed; define cache keys/TTLs near `infra/cache` conventions.
- Stop if implementation discovers a need to persist UI preferences in Postgres; create a separate data-model plan before adding migrations.

## 9. Error handling and response conventions

- Renderer methods should return errors and log through `infra/logging`; they should not silently swallow parse/execute failures.
- API JSON endpoints must keep `httpdto` where already used; avoid introducing raw JSON contracts from UI handlers.
- HTMX partial handlers should render meaningful partials or return explicit statuses; do not hide service errors by returning empty fragments.
- Preserve `RenderStandalone` for 401/404/405 pages.
- If a render error occurs after headers are written, log it with template name and mode but do not attempt a second conflicting response.

## 10. Constants and configuration

- Existing relevant config: `APP_ENV`, `DEBUG`, `PORT`, `VIZBURO_PORT`, HTTP timeouts in `internal/app/config.go`.
- Renderer cache reload should be based on existing `Config.AppEnv` (`local` reparses; non-local caches) or an explicit option; avoid introducing a new env var unless reload behavior cannot be expressed with current config.
- Do not store secrets or tokens in template data/logs.
- If adding viewport cookie constants, keep names centralized in the UI context package (`vp`, values `mobile|desktop`).

## 11. Logging and monitoring

Observability agent tasks:

- Replace any remaining `log.Printf` in active UI code with `infra/logging`; `vizburo/ui/template_helpers.go` is legacy unless proven active.
- Add renderer logs with low-cardinality fields: `mode`, `template`, `cache_hit`, `app_env`, `duration_ms`; avoid user IDs, Discord IDs, tokens, or raw query strings.
- If metrics are added, extend `infra/metrics.MetricsRegistry` only; suggested low-cardinality candidates:
  - template render duration by `mode` and template basename/path category;
  - template parse/cache miss count by `mode`.
- Do not label metrics by VA, user, Discord ID, or full URL.
- Verify `/metrics` remains API-server-only because `NewVizburoServer` disables metrics; if Vizburo is deployed separately and needs metrics, plan a separate runtime/infra change.
- Check `labour-bureau/prometheus.dev.yml` and Docker/Podman labels only if new scrape targets are introduced.

## 12. API spec and generated code work

Swagger/OpenAPI agent tasks:

- Phases 1–6 are UI/internal-renderer work; no OpenAPI changes expected.
- Verify no `/api/v1` request/response schemas or operation IDs changed after handler consolidation.
- If any bot-facing API changes are made, update `politburo/api/openapi/registration.yaml`, keep response envelopes aligned with `httpdto`, and run `make generate-api` from `politburo/`.
- Never hand-edit `internal/api/generated/**`.

## 13. Documentation

- Implementation agents should update docs only after code lands, not in the first renderer slice.
- Candidate follow-ups:
  - README/runbook note for renderer cache behavior in local vs production.
  - Vizburo styling guidelines: design-system tokens only, inline-style exceptions.
  - Mobile page classification table for UI contributors.

## 14. Frontend/Vizburo plan

- Active templates to prioritize: root `templates/layouts/base.html`, `templates/pages/dashboard.html`, `templates/pages/live.html`, `templates/pages/logbook.html`, `templates/pages/events.html`, `templates/pages/datasource.html`, `templates/pages/vaadmin-*.html`, `templates/pages/livery-mappings.html`, and root `templates/partials/**`.
- Use design-system tokens/classes only; no hardcoded colors and no Nord variables in new active templates.
- Keep handlers thin and use injected renderer/services; UI must not access DB/Redis/Live API directly.
- Do not introduce polling. HTMX should remain user/action/server-response driven.
- Mobile behavior:
  - Community/member pages can get mobile partials.
  - Admin/config workflows should render a mobile guard until product approves mobile workflows.

## 15. Testing plan

Unit Testing agent tasks:

- Renderer tests:
  - cache hit/miss behavior in production-like mode;
  - local reload behavior;
  - partial block-name fallback to `content`;
  - standalone error layout rendering;
  - missing file/parse error propagation.
- Middleware/context tests:
  - session present/missing/expired;
  - active VA missing;
  - role flags/menu items;
  - HTMX vs full-page unauthorized behavior if implemented.
- Handler tests:
  - one package at a time for dashboard, datasource, events, vaadmin, pilots/logbook, flights/live.
  - verify handlers use `UIContext` and do not duplicate session extraction.
- UI tests/manual checks:
  - desktop smoke for all dashboard routes listed in router.
  - mobile viewport smoke for classified mobile-first and mobile-incompatible pages.
  - CSS build: `npm run css:build` only in styling phase.
- Regression commands:
  - `go test ./...` before final handoff.
  - focused registration/OpenAPI set if any `/api/v1` or generated-adapter code is touched.

## 16. Execution order for specialized agents

1. **Backend/infra agent:** Phase 1 renderer caching tests and implementation.
2. **Unit Testing agent:** strengthen renderer tests and regression coverage.
3. **Backend/UI architecture agent:** Phase 2 UI context middleware, then package-by-package handler cleanup.
4. **Backend cleanup agent:** verify and remove/quarantine legacy `vizburo/ui` only after imports/build checks.
5. **Frontend/Vizburo agent:** design-system theme cleanup and mobile guards/partials.
6. **Observability agent:** renderer logs/metrics review after caching lands.
7. **Swagger/OpenAPI agent:** verify no API spec impact; update only if API contracts changed.
8. **Docs agent:** document final cache/theme/mobile conventions.

## 17. Out-of-scope items

- No new product features, routes, jobs, queues, service workers, or polling loops.
- No migration to another template engine.
- No bot slash-command changes unless routes/signed-link destinations change later.
- No DB migrations or persistent UI preferences.
- No broad deletion of legacy packages outside a verified cleanup slice.

## 18. Final checklist

- Planner modified no source/config/test/generated files; only this markdown plan was created.
- Plan file path: `politburo/plans/vizburo-ui-architecture-implementation.md`.
- First downstream task: renderer caching in `infra/templates.Renderer` with focused tests and builds.
- Key agents/tasks: Backend/infra for renderer and context, Unit Testing for coverage, Frontend/Vizburo for design-system/mobile, Observability for low-cardinality render metrics/logs, Swagger/OpenAPI for verification-only unless APIs change.
