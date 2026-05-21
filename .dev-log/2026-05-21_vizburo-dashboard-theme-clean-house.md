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
