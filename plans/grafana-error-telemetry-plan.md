# Vizburo Frontend Exception Capture — Implementation Plan

## 1. Title and status

- **Status:** Revised plan after clarification.
- **Plan file:** `politburo/plans/grafana-error-telemetry-plan.md`
- **Date:** 2026-05-21
- **Requested change summary:** Add lightweight, open-source frontend exception capture for Vizburo browser JavaScript, similar in intent to New Relic Browser/Sentry error capture, but **only for exceptions/failures**, not broad user telemetry.
- **Scope and assumptions:**
  - Capture uncaught browser exceptions, unhandled promise rejections, HTMX response errors, and map/Leaflet initialization/runtime failures.
  - Do **not** track user behavior, click analytics, heatmaps, session replay, full RUM, web vitals, traces, or frontend API-performance metrics. Backend API metrics already cover request volume/latency.
  - Must not add noticeable page latency. Client capture should load after critical CSS/HTMX where possible, sample/error-send asynchronously, and never block map rendering or page interaction.
  - Open-source-only options are allowed. Recommended MVP is Grafana-native Faro Web SDK + Grafana Alloy `faro.receiver` forwarding exception logs to Loki, because the repo already runs Grafana/Loki/Prometheus.

## 2. Context

### Files/packages inspected

- Guidance: `AGENTS.md`, `TECHNICAL_STANDARDS.md`, `politburo/CLAUDE.md`.
- Existing UI plans: `politburo/plans/vizburo-ui-architecture.md`, `vizburo-ui-architecture-implementation.md`, `error-auth-pages-redesign.md`.
- Active Vizburo layout/templates: `politburo/templates/layouts/base.html`, `templates/pages/live.html`, selected `templates/partials/**` via grep.
- Static JS/assets: `politburo/static/js/htmx.min.js`, third-party `static/js/gleo/**`; `templates/pages/live.html` references `/static/js/live-flights.mjs` and Leaflet CDN.
- Runtime/observability context: `politburo/internal/runtime/server.go`, `internal/middleware/metrics.go`, `infra/metrics/metrics.go`, `labour-bureau/prometheus.dev.yml`, `promtail-config.yml`, `prod/promtail-config.yml`, Grafana dashboard directories.
- External OSS references reviewed: Grafana Faro Web SDK frontend docs, Grafana Alloy `faro.receiver`, Sentry JavaScript SDK, GlitchTip install docs.

### Existing behavior and architecture summary

- Vizburo is server-rendered HTML inside Politburo. The active layout is `templates/layouts/base.html`, which loads `design-system.css` and `htmx.min.js`, then includes inline shell interaction JavaScript.
- The live flights page uses Leaflet from CDN and a module script `/static/js/live-flights.mjs`; maps and richer client-side logic make browser-only exceptions plausible.
- Several templates include inline scripts and HTMX error handlers; e.g. `flight-mode-edit-form.html` currently logs HTMX response errors with `console.error`.
- Politburo API server exposes `/metrics`; the separate Vizburo runtime disables metrics. This plan does not change that because browser exceptions should flow to logs/Loki, not Prometheus request counters.
- Labour Bureau already runs Grafana, Loki, Promtail, and Prometheus in dev/prod. It does **not** currently run Grafana Alloy or a Faro receiver.

### Relevant repo guidance discovered

- Keep Vizburo handlers thin and server-rendered; use design-system tokens and HTMX, not a separate frontend app.
- Do not introduce tracing unless explicitly planned. This plan explicitly rejects tracing for MVP.
- Loki labels must remain low-cardinality. Do not label by user, Discord ID, guild, session, raw path, request ID, error message, or stack trace.
- Backend API hits/latency already suffice for usage telemetry; frontend work should be exception-only.

## 3. Existing reuse

- Reuse Grafana/Loki as the operator UI and storage path for frontend exception logs.
- Reuse Labour Bureau local/prod infra as the place to add any receiver/proxy service.
- Reuse `templates/layouts/base.html` as the single bootstrap point for active Vizburo pages.
- Reuse existing page names/route patterns as low-cardinality context only if sanitized to stable values.
- Reuse existing backend HTTP metrics for API hit/performance visibility; do not duplicate them in the browser SDK.

## 4. Architecture decisions

- **Recommended MVP:** self-host Grafana Alloy with `faro.receiver` and configure the Faro Web SDK in exception-only mode to forward browser exceptions/logs to Loki.
- **Why Faro first:** it integrates naturally with the existing Grafana/Loki stack and avoids operating a separate error-tracking product just to see frontend exceptions.
- **No latency decision:** load the browser capture script asynchronously/deferred, initialize after the main page can render, and send exception payloads in the background using the SDK transport. If Faro fails to load or the receiver is down, Vizburo must continue normally.
- **Exception-only decision:** disable/omit Faro instrumentation for session tracking, user actions, web vitals, performance measurements, console collection, and tracing. Capture only:
  - `window.onerror` / uncaught exceptions;
  - `unhandledrejection`;
  - explicit captures from map/Leaflet init and HTMX response-error hooks.
- **Privacy decision:** send no user identity by default. Include only app/environment/release, stable page area, exception type/message, stack, browser metadata normally included with an exception, and maybe a hashed/sampled anonymous client ID only if later explicitly approved.
- **Sourcemaps:** optional but useful. If JS remains mostly unbundled/minimal, sourcemaps can wait. If `live-flights.mjs` or future JS is bundled/minified, configure Alloy Faro sourcemap lookup from local static build artifacts without making sourcemaps public unless already intended.
- **Alternatives considered:**
  - Self-hosted GlitchTip: good New Relic/Sentry-style issue grouping, Docker-friendly, but adds Postgres/worker/retention/admin overhead outside the existing Grafana path.
  - Self-hosted Sentry: rich but operationally heavy for the stated “not too much telemetry” goal.
  - Custom `/client-errors` endpoint in Politburo: lowest dependency, but creates custom ingestion, grouping, rate limiting, dashboards, and security burden that Faro/Alloy already solves.

## 5. Repo-by-repo implementation plan

### politburo/

- Add a tiny Vizburo frontend error bootstrap, likely under `static/js/vizburo-errors.mjs` or equivalent.
- Load it from `templates/layouts/base.html` with `defer`/`type="module"` and failure-safe behavior.
- Recommended bootstrap responsibilities:
  - initialize Faro only when a public config says frontend error capture is enabled;
  - set app name `vizburo`, environment from server-rendered config, and release/build identifier if available;
  - disable broad instrumentations; keep exception capture only;
  - register `window.addEventListener('error', ...)` and `window.addEventListener('unhandledrejection', ...)` if Faro does not already do this in the chosen minimal setup;
  - register a single delegated `document.body` listener for `htmx:responseError` and `htmx:sendError`, capturing method/status/stable target/page area but not request bodies or full URLs with IDs;
  - expose a small `window.VizburoErrors.capture(error, context)` helper for map code to call around Leaflet/live-map initialization.
- Update map/client scripts only where they already contain meaningful try/catch boundaries:
  - `templates/pages/live.html` / `/static/js/live-flights.mjs` should capture map initialization failures, missing DOM/data JSON errors, Leaflet load failures, and marker/render exceptions.
  - Do not wrap every function; capture at page/module boundaries to avoid noisy telemetry.
- Add server-rendered public config carefully:
  - either a small inline JSON script in `base.html` or a static config endpoint/partial;
  - include only non-secret values: enabled flag, Faro endpoint URL, app environment, release.
- Do not add new database tables or backend business logic.
- Do not add browser performance spans, fetch/XHR instrumentation, page-view tracking, or user-action tracking.

### comrade-bot/

- Not applicable. Discord bot observability is separate and already covered by existing backend/bot metrics/logging plans.

### Vizburo UI

- Active UI integration point is `templates/layouts/base.html` so the capture script covers all server-rendered dashboard pages.
- Page-specific capture is only needed for richer client code:
  - live map / Leaflet page;
  - datasource setup scripts if they perform fetch/HTMX logic;
  - flight mode editor HTMX error hook currently logging to console.
- Mobile behavior:
  - capture exceptions equally on mobile and desktop;
  - do not add mobile usage analytics;
  - if admin pages are mobile-guarded, do not report expected guard behavior as errors.

### labour-bureau/

- Add Grafana Alloy to dev and prod observability stacks if not already present.
- Configure Alloy `faro.receiver`:
  - listen on an internal container port, e.g. `12347`;
  - set CORS only for Vizburo/Politburo origins used in dev/prod;
  - enable rate limiting with conservative defaults;
  - forward logs to Loki only;
  - do not wire traces output for MVP.
- Expose the Faro receiver to browsers through the existing reverse proxy path, preferably same-origin to avoid CORS/ad-blocker problems, e.g. `/faro` or `/collect/frontend-errors` proxied to Alloy.
- Add/extend Grafana dashboard panels for frontend exceptions:
  - recent Vizburo browser exceptions;
  - exception count by page area and error type;
  - top exception messages with limits;
  - map/live-flight exception panel;
  - receiver health/rate-limit/error panels using Alloy/Faro receiver metrics if scraped.
- Add Prometheus scrape for Alloy only if its metrics endpoint is enabled and useful; keep it infra-local.

### API contracts/generated clients/shared configuration

- Not applicable. No public JSON API contract or generated client change is expected.
- If implementation chooses a custom Politburo `/client-errors` endpoint instead of Faro/Alloy, stop and create a separate OpenAPI/auth/rate-limit plan first.

## 6. Developer guidelines for implementation agents

- MUST keep frontend capture exception-only.
- MUST ensure Vizburo renders and maps initialize even if the capture SDK, Alloy, Loki, or network is unavailable.
- MUST not add user/session/guild/Discord identifiers to payloads or labels.
- MUST avoid synchronous/blocking SDK loads in the critical render path.
- SHOULD implement in this order: infra receiver/proxy, minimal client bootstrap disabled by default, enable in local/dev, dashboard, then production.
- Files likely to edit:
  - `politburo/templates/layouts/base.html`
  - new `politburo/static/js/vizburo-errors.mjs` or similar
  - `politburo/templates/pages/live.html` and/or `politburo/static/js/live-flights.mjs` if present
  - selected inline script templates with known `console.error`/HTMX error hooks
  - `politburo/internal/app/config.go` only if adding public frontend-observability config values
  - `labour-bureau/docker-compose.dev.yml`, reverse proxy/dev wiring, Grafana/Prometheus/Loki config as needed
  - `labour-bureau/prod/**` compose/proxy/env example/provisioning files
- Files/packages to avoid:
  - `politburo/internal/api/generated/**`
  - DB migrations
  - `comrade-bot/` unless docs mention the distinction
  - legacy `vizburo/ui/**` unless proven active
  - broad backend metrics changes.

## 7. Auth scopes, claims, and context

- Faro browser ingestion should not require user auth. If Alloy `api_key` is used, it is a public ingestion key and not a secret authorization boundary.
- Use reverse-proxy and receiver rate limiting as abuse protection.
- VA context:
  - do not include VA ID by default;
  - if operators later need VA-specific triage, add only a coarse, non-identifying tenant class or explicitly approved hashed VA value as a parsed log field, never a Loki label.
- Middleware/context propagation: no Politburo auth middleware changes are required.
- Mobile classification: capture exceptions from mobile browsers but do not track mobile usage or interactions.

## 8. Migrations and data model

- Not applicable. No schema changes or backfills.
- Loki retention governs exception log retention unless GlitchTip/Sentry alternative is chosen later.
- Rollback:
  - disable frontend capture config;
  - remove/proxy-block Faro endpoint;
  - keep Vizburo pages functional without SDK.

## 9. Error handling and response conventions

- Browser-side capture must never show telemetry errors to users.
- HTMX failures should continue to use existing UI error behavior; telemetry capture is a side effect only.
- Map failures should still render a user-friendly fallback if existing UI supports it; capture should not replace UI error handling.
- If a custom endpoint is ever chosen, it must rate-limit, validate payload size, return `204`/`202`, and avoid exposing internal errors to browsers.

## 10. Constants and configuration

- Proposed non-secret config:
  - `FRONTEND_ERRORS_ENABLED=false|true`
  - `FRONTEND_ERRORS_ENDPOINT=/faro` or configured public URL
  - `FRONTEND_ERRORS_APP_NAME=vizburo`
  - `FRONTEND_ERRORS_RELEASE=<git sha/build version>` if available
  - `FRONTEND_ERRORS_SAMPLE_RATE=1.0` for exceptions only; use lower sampling only if volume is noisy
- Do not add config for page views, web vitals, traces, session replay, or user actions in MVP.
- CSP consideration: allow the Faro endpoint in `connect-src` if CSP is active. Prefer same-origin endpoint to minimize CSP/CORS complexity.

## 11. Logging and monitoring

Observability agent tasks:

- Configure Alloy `faro.receiver` with `log_format = "json"`, logs output to Loki, and no traces output.
- Recommended Loki labels for frontend exception streams:
  - `service="vizburo-frontend"` or `app="vizburo"`
  - `env`
  - `level="error"`
  - optionally `source="faro"`
- Do not label by page URL, route with IDs, error message, stack, browser version, user/session, VA, guild, or request ID.
- Suggested dashboard queries should be verified after implementation because Faro log shapes depend on receiver output:
  - recent frontend exceptions by service/env/level;
  - count over time filtered to exception records;
  - top error names/messages using LogQL parsing, with panel limits.
- Scrape Alloy/Faro receiver metrics only for receiver health and drops, not user behavior.
- Alerting MVP:
  - optional alert on frontend exception burst above a threshold;
  - optional alert on Faro receiver down/exporter errors;
  - no alert on single sporadic browser errors.

## 12. API spec and generated code work

- Swagger/OpenAPI agent tasks: Not applicable for Faro/Alloy approach.
- No `make generate-api` work.
- If a custom Politburo ingestion endpoint is later selected, create a separate OpenAPI/auth/rate-limit plan before implementation.

## 13. Documentation

- Update operator/runbook docs after implementation with:
  - what frontend exceptions are captured;
  - what is intentionally not captured;
  - how to find Vizburo frontend exceptions in Grafana;
  - privacy/label policy;
  - how to disable the capture script quickly.
- No pilot/user-facing documentation is needed unless error UI changes.

## 14. Frontend/Vizburo plan

- Keep the capture bootstrap tiny and loaded from active root templates.
- Prefer one shared script plus a few explicit capture calls around map-heavy code.
- Do not introduce npm bundling solely for telemetry unless the selected SDK requires it and there is no acceptable CDN/self-hosted bundle option.
- If using a third-party SDK file, prefer vendoring or serving from `/static/js/` after license/version review to avoid CDN latency and availability risk.
- The script should initialize after page essentials and should not block rendering. Telemetry sends must be async/background.
- Mobile: no separate behavior beyond exception capture.

## 15. Testing plan

Unit Testing agent tasks:

- JavaScript/manual verification:
  - simulate an uncaught error from a test-only local page/script and verify Loki/Grafana receives it;
  - simulate `unhandledrejection`;
  - simulate an HTMX response error and verify it is captured once;
  - simulate Leaflet/map init failure without breaking the whole page.
- Latency/regression verification:
  - compare page load with receiver unavailable; Vizburo must still render and maps must still attempt to initialize;
  - confirm no synchronous blocking network request is made before first render;
  - confirm SDK failure produces no user-visible error.
- Infra verification:
  - validate compose/proxy config;
  - verify Alloy/Faro receiver accepts payloads and forwards to Loki;
  - verify CORS/same-origin behavior in dev and prod;
  - verify rate limiting rejects excessive payloads.
- Politburo verification:
  - build server/Vizburo binaries if templates/config change;
  - run template tests if available;
  - run `npm run css:build` only if stylesheet/build inputs change, which this plan should avoid.

## 16. Execution order for specialized agents

1. **Observability-infra maintainer:** add Alloy/Faro receiver, reverse-proxy path, Loki forwarding, receiver health metrics, and Grafana dashboard panels.
2. **Plan-to-code developer:** add minimal Vizburo frontend exception bootstrap and page/map capture integration, disabled by default until infra is available.
3. **Unit testing agent:** validate exception capture paths, receiver failure behavior, dashboard queries, and no blocking render impact.
4. **Feature docs maintainer:** document operator usage and privacy/disable policy.
5. **Swagger/OpenAPI agent:** not needed unless the approach changes to custom Politburo ingestion.

## 17. Out-of-scope items

- User behavior analytics, click tracking, heatmaps, session replay, user feedback widgets.
- Web vitals, frontend performance monitoring, distributed tracing, fetch/XHR tracing.
- Backend API metric changes beyond existing coverage.
- Per-user/per-VA/per-guild labels or dashboards.
- New data warehouses or DB tables.
- Self-hosted Sentry/GlitchTip in MVP; keep as later alternatives if Grafana/Faro is insufficient.

## 18. Final checklist

- Source modifications avoided by this planner: yes; only this markdown plan was updated.
- Plan file path: `politburo/plans/grafana-error-telemetry-plan.md`.
- Key downstream agents/tasks:
  - Observability agent: Alloy `faro.receiver`, Loki forwarding, Grafana panels, rate limiting, proxy/CORS.
  - Developer agent: minimal Vizburo exception-only JS bootstrap and map/HTMX capture hooks.
  - Unit testing agent: exception capture, receiver-down behavior, no page latency/blocking regression.
