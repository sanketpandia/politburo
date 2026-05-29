# Vizburo Dashboard Theme Clean-House Plan

> Status: Ready for implementation planning handoff  
> Date: 2026-05-21  
> Requested change: review the weird-looking `/dashboard` UI from the provided screenshot/curl, identify why the new Vizburo theme/palette/components are not visible, and plan a root-cause cleanup without implementing changes in this planner pass.

## 1. Title and status

- **Plan file:** `politburo/plans/vizburo-dashboard-theme-clean-house.md`
- **Requested change summary:** clean up Vizburo theme conflicts and make the active dashboard use the intended new design-system palette/components instead of looking like two themes are fighting.
- **Scope:** Vizburo dashboard shell, active templates, design-system CSS delivery, Tailwind/build inputs, legacy `vizburo/ui/**` quarantine/deletion verification, and dashboard component cleanup.
- **Assumptions:**
  - The screenshot shows the app rendered with a very narrow effective content column and large empty dark area. The live curl confirms current `/dashboard` HTML is using root `templates/**` and `/static/css/design-system.css`, not legacy `vizburo/ui/templates/**`.
  - The user’s cookie is valid enough to inspect `/dashboard`; this planner used it only for read-only HTML/CSS checks.
  - The desired canonical theme is the new Deep Void/design-system palette in `static/css/design-system.css`, not the old Nord palette in `vizburo/ui/**`.

## 2. Context

### Files/packages inspected

- Guidance: `AGENTS.md`, `politburo/CLAUDE.md`, prior plan `politburo/plans/vizburo-ui-architecture-implementation.md`.
- Runtime/routing/static: `internal/routes/router.go` (`/static/*`, `/dashboard`, `/dashboard/switch-va`, role groups), `internal/app/app.go` by prior plan reference.
- Active renderer/handlers: `infra/templates/renderer.go`, `infra/templates/session_helpers.go`, `internal/dashboard/handler.go`.
- Active templates/assets: `templates/layouts/base.html`, `templates/pages/dashboard.html`, `templates/partials/desktop-nav.html`, `templates/partials/mobile-header.html`, `templates/partials/mobile-drawer.html`, `templates/partials/secondary-nav.html`, `templates/partials/pilot-stats.html`, `static/css/design-system.css`, `static/css/output.css`, `package.json`, `tailwind.config.js`.
- Legacy duplicate UI: `vizburo/ui/templates/layouts/base.html`, `vizburo/ui/templates/pages/dashboard.html`, `vizburo/ui/input.css`, `vizburo/ui/tailwind.config.js`, `vizburo/ui/utils.go`, `vizburo/ui/handlers.go`, `vizburo/ui/template_helpers.go`.
- Live read-only checks: `curl http://localhost:8080/dashboard` with the provided `session_id` cookie; `curl -I http://localhost:8080/static/css/design-system.css`.

### Existing behavior and architecture summary

- `internal/routes/router.go` serves static files from root `static/` at `/static/*` and applies long-lived CDN headers through `middleware.CDNMiddleware`.
- Active `/dashboard` is rendered by `internal/dashboard.Handler.DashboardPageHandler()` through injected `infra/templates.Renderer`, using root `templates/pages/dashboard.html` and root `templates/layouts/base.html`.
- The active root layout links only `/static/css/design-system.css`; it does **not** link `/static/css/output.css`.
- The live `/dashboard` HTML confirms the root layout/components are active: `.app-shell`, `.desktop-header`, `.desktop-nav`, `.main-content`, `.dashboard-content`, `.page-header`, `.dashboard-card`.
- `static/css/design-system.css` is reachable, but the HTTP response includes `Cache-Control: public, max-age=31536000, immutable` and `Cf-Cache-Status: HIT`; stale browser/CDN cache can preserve old broken CSS even after local changes unless cache-busting/versioning is added.
- Legacy `vizburo/ui/**` still contains a complete Nord-themed duplicate layout/dashboard, inline styles, its own Tailwind input and output path, and a duplicate renderer/helper package. Prior planning found no active imports of `vizburo/ui`, but downstream agents MUST verify before deletion.

### Relevant repo guidance discovered

- `politburo/CLAUDE.md` says Vizburo is server-rendered UI with HTMX + Tailwind and routes should go through `internal/routes/router.go` plus `application.Features.*`.
- `AGENTS.md` says Vizburo styling must use design-system CSS tokens only, handlers must remain thin, UI must not access infrastructure directly, and mobile behavior must be explicitly classified.
- `static/css/design-system.css` is the current design-system token source; new work should not add more Nord or ad hoc inline color systems.

## 3. Existing reuse

- Reuse `infra/templates.Renderer` and root `templates/**`; do not revive `vizburo/ui/utils.go` or legacy templates.
- Reuse `templates/partials/desktop-nav.html`, `mobile-header.html`, `mobile-drawer.html`, and `secondary-nav.html` as the navigation component surface, but consolidate duplicate nav levels where product confirms.
- Reuse design-system tokens/classes in `static/css/design-system.css`: `--bg-app`, `--bg-surface`, `--bg-sidebar`, `--text-*`, `--border-*`, `--accent-*`, `.app-shell`, `.desktop-header`, `.main-content`, `.card`, `.page-header`, `.nav-link`.
- Reuse `templates.PrepareTemplateData` and `internal/platform/ui.GetMenuItems` for role-aware navigation rather than hardcoding menu labels in page templates.
- Reuse `internal/dashboard.Service` for dashboard data; keep handlers thin.

## 4. Architecture decisions

- **Root cause A — legacy duplicate source tree:** `vizburo/ui/**` is an old Nord-themed duplicate with inline styles and its own build setup. It is not what `/dashboard` currently renders, but it still misleads build commands, Tailwind config, and future implementation work.
- **Root cause B — CSS build/source mismatch:** root `package.json` builds Tailwind from `./vizburo/ui/input.css` to `./static/css/output.css`, while active root `templates/layouts/base.html` links only `design-system.css`. `tailwind.config.js` scans only `./vizburo/ui/templates/**/*.html`, not active root `templates/**/*.html`. Therefore new Tailwind utilities/components added for active templates may never be generated or loaded.
- **Root cause C — cache-busting gap:** `/static/css/design-system.css` is served with one-year immutable cache headers and no versioned URL in the layout. Browser/CDN cache can keep a stale stylesheet even when the checked-in CSS changed.
- **Root cause D — active root dashboard still has local inline component CSS:** `templates/pages/dashboard.html` defines dashboard card/stats styles in a page-local `<style>` block and still uses many inline styles. This fragments the component system even though it uses design-system variables.
- **Root cause E — viewport/layout failure needs CSS-level verification:** the screenshot shows the rendered content clipped to a narrow left strip. The live HTML is structurally full-width, so the most likely causes are stale/wrong CSS delivery, browser zoom/devtools overlay, or a design-system layout regression. Downstream must reproduce with a hard cache refresh and inspect computed styles for `.app-shell`, `.app-main`, `.main-content`, and `.desktop-header` before patching layout blindly.
- **Decision:** clean house by making root `templates/**` + `static/css/design-system.css` the only active Vizburo UI source of truth, then remove/quarantine legacy `vizburo/ui/**` only after verification.
- **Open question:** whether the dashboard should keep both desktop top nav and secondary nav. Current root templates render both; this may be intentional for page context, but it adds visual duplication.

## 5. Repo-by-repo implementation plan

### politburo/

#### Phase 1 — Reproduce and prove active asset/template paths

- Verify with browser devtools or Playwright/manual inspection:
  - `/dashboard` uses `templates/layouts/base.html` and `templates/pages/dashboard.html`.
  - CSS request is `/static/css/design-system.css`, status 200, MIME `text/css`, and actual response contains `.app-shell`, `.desktop-header`, `.main-content`.
  - After hard refresh/disable cache, computed styles for `.app-shell`, `.app-main`, `.main-content`, `.dashboard-content`, `.desktop-header` are sane and full width.
- Add no source changes until this verification is captured in the dev log.

#### Phase 2 — Fix CSS cache/versioning before style edits

- Likely files to edit:
  - `templates/layouts/base.html`
  - possibly `internal/app/config.go` or a template data helper only if a dynamic asset version is already available or can be cleanly injected through existing template data.
- Tasks:
  - Add a deterministic cache-busting query/version to `design-system.css` (for example build/version constant or app startup asset mtime) without bypassing `CDNMiddleware` globally.
  - Keep static serving in `internal/routes/router.go`; do not create a second static server.
  - Ensure local development can force refreshed CSS without asking users to clear cache manually.

#### Phase 3 — Make active root design-system CSS the single styling surface

- Likely files to edit:
  - `static/css/design-system.css`
  - `templates/pages/dashboard.html`
  - `templates/partials/pilot-stats.html`
  - `templates/partials/secondary-nav.html`
  - `templates/partials/desktop-nav.html`
- Tasks:
  - Move page-local dashboard `<style>` rules from `templates/pages/dashboard.html` into `static/css/design-system.css` as component classes.
  - Replace inline color/spacing styles in dashboard and pilot stats with design-system classes/tokens.
  - Keep only truly dynamic inline styles, and document why they remain.
  - Ensure dashboard cards, stats, leaderboard modal, and nav all use `--bg-*`, `--text-*`, `--border-*`, `--accent-*` tokens only.
  - Confirm layout width by adding/fixing canonical rules if needed: `.app-shell` must occupy full viewport width; `.app-main`/`.main-content` must flex to available width and not collapse.

#### Phase 4 — Resolve Tailwind/build ambiguity

- Likely files to edit:
  - `package.json`
  - `tailwind.config.js`
  - possibly `vizburo/ui/input.css` only if retained as the Tailwind input temporarily.
- Tasks:
  - Decide whether Tailwind is still part of active root Vizburo. If yes:
    - Change content globs to scan active `templates/**/*.html`.
    - Change input to a root-owned CSS entrypoint or explicitly import design-system CSS through a root-owned build path.
    - Link the built CSS from the root layout if generated utilities are needed.
  - If Tailwind is not needed for active root UI, remove dead/ambiguous build scripts in a dedicated cleanup slice and keep handcrafted `design-system.css` authoritative.
  - Do not continue building active CSS from `vizburo/ui/input.css` while rendering root templates.

#### Phase 5 — Legacy `vizburo/ui/**` retirement

- Verification first:
  - Run `go list ./...` and content search to confirm no active import of `infinite-experiment/politburo/vizburo/ui`.
  - Confirm no deployment/script/package command references `vizburo/ui/static/css/output.css` or `vizburo/ui/templates/**`.
  - Compare any missing active templates before deletion; do not lose needed partials.
- If verified unused, delete or quarantine the legacy `vizburo/ui/**` package/templates/build files in one dedicated cleanup commit.
- If any active entrypoint still imports it, migrate that entrypoint to `app.App` + `infra/templates.Renderer` first.

#### Phase 6 — Dashboard UX cleanup after foundation is fixed

- Likely files to edit:
  - `templates/pages/dashboard.html`
  - `templates/partials/secondary-nav.html`
  - `templates/partials/desktop-nav.html`
  - `static/css/design-system.css`
- Tasks:
  - Decide whether desktop dashboard should show both top nav and secondary nav. If redundant, remove or demote secondary nav on dashboard only while preserving it on deep admin/workflow pages if useful.
  - Make dashboard cards/components match the new palette and component density.
  - Add empty/error states for missing `ActiveVA`, missing stats, and missing leaderboard without layout collapse.
  - Keep HTMX interactions request-triggered; do not introduce polling.

### comrade-bot/

- Not directly applicable. No Discord slash command or bot API behavior should change.
- Verify signed dashboard links only if asset URLs or dashboard route paths change.

### Vizburo UI

- Active Vizburo UI should be root `templates/**` plus `static/css/design-system.css` served by Politburo.
- `vizburo/ui/**` should be treated as legacy duplicate unless implementation verification proves otherwise.
- Thin handler rule applies: `internal/dashboard/handler.go` should assemble data and render; component styling belongs in CSS/templates, and business logic remains in `internal/dashboard.Service`.

### labour-bureau/

- Not directly applicable for visual cleanup.
- If cache-busting or static asset serving changes affect local dev/prod behavior, infra agent should verify compose/prod reverse proxy/CDN assumptions and that Politburo still serves `/static/*` correctly.

### API contracts/generated clients/shared configuration

- No JSON API contract changes are expected.
- If implementation changes `/dashboard/link` or other bot-facing JSON endpoints, update the relevant OpenAPI spec and generated clients as described below.

## 6. Developer guidelines for implementation agents

- MUST use root `templates/**` and injected `infra/templates.Renderer` for active UI work.
- MUST NOT add new styling to `vizburo/ui/**` unless that package is proven active and being migrated.
- MUST NOT add a third theme or duplicate token names; use `static/css/design-system.css` tokens.
- MUST preserve `internal/routes/router.go` route registration and `application.Features.*` handler wiring.
- SHOULD implement in small slices: cache-bust, design-system consolidation, build cleanup, legacy deletion, dashboard polish.
- SHOULD verify with hard-refresh/disabled-cache browser testing after every CSS delivery change.
- Avoid editing generated files, migrations, API specs, or infra unless the slice explicitly requires it.

## 7. Auth scopes, claims, and context

- `/dashboard` is protected by `uiAuthMiddleware` in `internal/routes/router.go`.
- Role-aware navigation and cards depend on session data from `auth.GetSessionData` and `templates.PrepareTemplateData`.
- Keep route authorization unchanged:
  - dashboard base: authenticated session
  - member routes: `IsMemberMiddleware()`
  - staff routes: `IsStaffMiddleware()`
  - admin routes: `IsAdminMiddleware()`
- VA context: dashboard content should continue using `sessionData.GetActiveVA()` and `ActiveVAID` from template data. Do not hardcode a VA or infer it from client-side state.
- Mobile classification: dashboard is mobile-compatible and should remain usable behind `mobile-header`/`mobile-drawer`; admin-heavy pages can remain mobile-compatible but not mobile-first unless separately planned.

## 8. Migrations and data model

- Not applicable. This is styling/template/build cleanup with no schema or data model change.
- No backfills or rollback migrations required.

## 9. Error handling and response conventions

- Full-page UI errors should continue rendering via `infra/templates.Renderer` where existing handlers do so; avoid raw `http.Error` regressions for normal UI flows.
- HTMX fragments such as `/dashboard/switch-va` should keep small HTML error fragments and appropriate status codes.
- API JSON endpoints must continue using `internal/platform/httpdto` if touched.
- Dashboard data fetch failures should log warnings and render graceful empty states rather than breaking the whole page when stats/leaderboard are optional.

## 10. Constants and configuration

- Add asset-version/cache-busting through existing app/template configuration if possible; avoid hardcoded random query strings that change every request.
- Do not add secrets or env vars for this visual cleanup unless a deploy-specific asset version already exists.
- If adding an `ASSET_VERSION`/build SHA later, document default local behavior and ensure production templates receive it through DI/template data.

## 11. Logging and monitoring

### Observability agent tasks

- Verify no new high-cardinality metrics are needed for a visual cleanup.
- If adding asset-version handling, optionally add low-cardinality structured logs at startup showing active asset version and template/static roots.
- Confirm `/static/css/design-system.css` responses keep correct MIME type and do not leak sensitive data in logs.
- Check whether `CDNMiddleware` immutable caching is appropriate for unversioned asset URLs; if not, recommend versioned URLs rather than disabling cache globally.
- No Prometheus labels, scrape target, Docker label, dashboard, or alert changes are expected unless static serving behavior changes in prod.

## 12. API spec and generated code work

### Swagger/OpenAPI agent tasks

- Not applicable for the planned styling/template cleanup.
- If implementation changes bot-facing `/api/v1` endpoints or `/dashboard/link`, update OpenAPI artifacts with operation IDs, schemas, security declarations, response envelopes, and generated clients as appropriate.
- Do not hand-edit `internal/api/generated/**`; run `make generate-api` only if a spec changes.

## 13. Documentation

- Update developer docs/README only if the CSS build pipeline changes:
  - canonical CSS file/source of truth
  - whether Tailwind is active
  - how to rebuild CSS
  - cache-busting/local hard-refresh notes
- User-facing docs are not needed for visual cleanup.

## 14. Frontend/Vizburo plan

- Keep handlers thin and server-rendered.
- Use design-system CSS tokens only; remove Nord usage from active templates and eventually remove legacy Nord files.
- Do not directly access infrastructure from templates or browser JavaScript.
- Do not add polling.
- Desktop behavior: full-width app shell, stable top nav, dashboard content not clipped, no duplicate/conflicting nav unless intentionally retained.
- Mobile behavior: dashboard remains accessible through mobile header/drawer; secondary nav hidden by existing mobile rules; cards stack cleanly.

## 15. Testing plan

### Unit Testing agent tasks

- Go tests:
  - `go test ./infra/templates ./internal/dashboard ./internal/routes`
  - Broader smoke if deleting legacy package: `go test ./...`
- Build checks:
  - `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - `go build -buildvcs=false -o .air_tmp/vizburo ./cmd/vizburo`
- CSS/build checks:
  - If Tailwind remains: `npm run css:build` from `politburo/`, then inspect that generated CSS includes classes used by root `templates/**`.
  - If Tailwind is retired: verify no active command or template relies on `static/css/output.css`.
- Manual/browser verification:
  - `/dashboard` with provided/session cookie after hard refresh/disabled cache.
  - `/dashboard/live`, `/dashboard/logbook`, `/dashboard/vaadmin`, `/dashboard/settings/datasource`, `/dashboard/events` for theme regressions.
  - Mobile viewport at <=768px and desktop viewport at >=1280px.
  - Network tab confirms loaded CSS URL/version and no stale immutable asset.

## 16. Execution order for specialized agents

1. **Developer agent:** reproduce/verify active template and CSS paths; implement asset cache-busting if needed.
2. **Developer/UI agent:** consolidate active dashboard styles into `static/css/design-system.css` and remove inline/page-local dashboard styles.
3. **Developer agent:** resolve Tailwind/build ambiguity.
4. **Developer cleanup agent:** verify and delete/quarantine legacy `vizburo/ui/**` in a dedicated slice.
5. **Unit Testing agent:** run focused Go/build/CSS/manual regression suite.
6. **Observability/infra agent:** only if static asset cache semantics or prod serving assumptions changed.
7. **Docs agent:** only if developer CSS build instructions changed.

## 17. Out-of-scope items

- No backend data model, migration, job, worker, Discord bot command, or generated API implementation.
- No new dashboard product features beyond visual/theme cleanup and empty-state polish.
- No polling or client-side SPA rewrite.
- No Grafana UI changes; Grafana cookies in the provided curl are irrelevant to Vizburo.
- No broad redesign outside active Vizburo pages unless required to remove the theme conflict.

## 18. Final checklist

- Source modifications avoided by this planner: yes; only this plan file was created.
- Plan file path: `politburo/plans/vizburo-dashboard-theme-clean-house.md`.
- Key downstream tasks:
  - prove active CSS/template path and stale-cache behavior,
  - add CSS cache-busting/versioning,
  - make `static/css/design-system.css` the single active theme surface,
  - fix or retire Tailwind build ambiguity,
  - verify and remove/quarantine legacy `vizburo/ui/**`,
  - run focused tests/builds and browser smoke checks.
