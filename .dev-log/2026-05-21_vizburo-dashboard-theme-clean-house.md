# 2026-05-21 — Vizburo Dashboard Theme Clean-House

## Logical unit 0: Active path and cache verification before source edits

- **Logical unit / commit intent:** Verify active dashboard/template/CSS paths and legacy/build ambiguity before making source changes, as required by the plan.
- **Changed files:** `.dev-log/2026-05-21_vizburo-dashboard-theme-clean-house.md` only.
- **Reused code / patterns / components:** Confirmed active rendering path remains `internal/dashboard.Handler.DashboardPageHandler()` → injected `infra/templates.Renderer` → root `templates/layouts/base.html` + `templates/pages/dashboard.html`; active layout links `/static/css/design-system.css`.
- **Logging added or affected:** None.
- **Metrics added or affected:** None.
- **Test surface touched or still needed:** No test code touched. Browser/computed-style verification still needs a valid authenticated session and is deferred to manual/UI smoke after changes.
- **Build/test command(s) run and status:**
  - `curl -fsS -I http://localhost:8080/static/css/design-system.css` — passed; returned `200 OK`, `Content-Type: text/css; charset=utf-8`, `Cache-Control: public, max-age=31536000, immutable`, `Cf-Cache-Status: HIT`.
  - `curl -fsS http://localhost:8080/static/css/design-system.css` — passed; response contains `.app-shell`, `.desktop-header`, `.main-content`, and `.dashboard-content`.
  - `curl -fsS -D /tmp/opencode/dashboard.headers http://localhost:8080/dashboard -o /tmp/opencode/dashboard.html` — returned `401` without a session cookie; authenticated HTML verification remains manual/session-dependent.
  - `go list ./...` — passed; `infinite-experiment/politburo/vizburo/ui` is still a package, but import checks found no active Go imports outside its own package and renderer comments.
- **Deviations from plan, if any:** Could not verify authenticated `/dashboard` HTML or browser computed styles because no valid session cookie was available in this implementation context. Proceeding only with source-level active-path proof plus static CSS HTTP proof.
- **Blast-radius notes / dependent surfaces checked:** Checked `internal/routes/router.go`, `internal/app/app.go`, `infra/templates/renderer.go`, `infra/templates/session_helpers.go`, active root `templates/**`, `static/css/design-system.css`, root `package.json`, root `tailwind.config.js`, and legacy `vizburo/ui/**` files named by the plan. Also searched for `design-system.css`, `output.css`, and `vizburo/ui` references across the workspace.
- **Live API compliance notes:** Not applicable; no Live API calls or response shapes changed.
- **Follow-up notes:**
  - **Swagger/OpenAPI:** No API contract impact expected for this UI/CSS cleanup.
  - **Observability:** Cache headers are confirmed immutable for unversioned CSS; versioned asset URL follow-up is in the next implementation slice. No metrics added.
  - **Unit Testing:** Add/retain focused renderer/template checks for asset-version helper behavior and run UI-adjacent Go tests after source changes.

## Logical unit 1: Version active design-system CSS URL

- **Logical unit / commit intent:** Add deterministic cache-busting to the active `/static/css/design-system.css` link without changing static serving or `CDNMiddleware`.
- **Changed files:**
  - `infra/templates/renderer.go`
  - `infra/templates/renderer_test.go`
  - `templates/layouts/base.html`
  - `.dev-log/2026-05-21_vizburo-dashboard-theme-clean-house.md`
- **Reused code / patterns / components:** Reused `infra/templates.Renderer` function map and project-root resolver; kept root layout as the single active CSS link surface and preserved `/static/*` routing through `internal/routes/router.go`.
- **Logging added or affected:** Added a low-cardinality warning if asset version resolution fails, keyed only by static asset path and error.
- **Metrics added or affected:** None.
- **Test surface touched or still needed:** Added renderer coverage that verifies the layout can emit a versioned `design-system.css` URL from filesystem mtime. Authenticated browser hard-refresh/computed-style verification still needs a valid session.
- **Build/test command(s) run and status:**
  - `gofmt -w infra/templates/renderer.go infra/templates/renderer_test.go` — passed.
  - `go test ./infra/templates ./internal/dashboard ./internal/routes` — passed.
  - `go build -buildvcs=false -o .air_tmp/main ./cmd/server` — passed.
- **Deviations from plan, if any:** Used an `assetVersion` template helper backed by file mtime instead of adding config/env; this matches the plan's deterministic startup/file-version option and avoids new environment wiring.
- **Blast-radius notes / dependent surfaces checked:** Static route remains unchanged; active root layout still links only `design-system.css` and not `output.css`; no API, bot, job, auth, or infra compose behavior changed.
- **Live API compliance notes:** Not applicable.
- **Follow-up notes:**
  - **Swagger/OpenAPI:** No API contract changes.
  - **Observability:** Optional follow-up can review whether the warning log should be startup-only; no metrics added.
  - **Unit Testing:** Current test covers helper output. Browser/manual smoke still needed for actual network URL and cache behavior with authenticated `/dashboard`.

## Logical unit 2: Consolidate active dashboard styles into design-system CSS

- **Logical unit / commit intent:** Move active dashboard page-local CSS and inline dashboard/pilot-stats/leaderboard/modal styles into `static/css/design-system.css` component classes.
- **Changed files:**
  - `static/css/design-system.css`
  - `templates/pages/dashboard.html`
  - `templates/partials/pilot-stats.html`
  - `templates/partials/pilot-logs.html`
  - `.dev-log/2026-05-21_vizburo-dashboard-theme-clean-house.md`
- **Reused code / patterns / components:** Reused existing `.card`, `.page-header`, `.page-title`, `.page-subtitle`, `.empty-state`, token variables (`--bg-*`, `--text-*`, `--border-*`, `--accent-*`, `--status-*`), and existing HTMX request-triggered leaderboard modal flow.
- **Logging added or affected:** None.
- **Metrics added or affected:** None.
- **Test surface touched or still needed:** No unit tests added for visual classes. Focused template/route packages still parse/build. Manual desktop/mobile dashboard smoke remains needed with authenticated session.
- **Build/test command(s) run and status:**
  - `go test ./infra/templates ./internal/dashboard ./internal/routes` — passed.
  - `go build -buildvcs=false -o .air_tmp/main ./cmd/server` — passed.
  - Grep checks for `style=`, `<style>`, hex colors, and `rgba(` in active dashboard/pilot-stats/pilot-logs templates — no matches.
- **Deviations from plan, if any:** Added graceful dashboard empty cards for missing active event and missing pilot stats as allowed by Phase 6 empty-state cleanup. Did not alter backend data fetching or route authorization.
- **Blast-radius notes / dependent surfaces checked:** Touched only active root dashboard templates/partial and design-system CSS. No `vizburo/ui/**`, bot, JSON API, jobs, auth, generated contracts, or infra files changed.
- **Live API compliance notes:** Not applicable.
- **Follow-up notes:**
  - **Swagger/OpenAPI:** No API contract changes.
  - **Observability:** No metrics/logging added; visual-only cleanup.
  - **Unit Testing:** Add optional template static tests for dashboard no-inline-style policy and authenticated browser smoke for leaderboard modal open/close behavior.
