# Vizburo UI Layer — Architectural Analysis

> Pure architectural review — no code changes. Focus: handler structure, theming, responsiveness, performance.
> Date: 2026-05-13

## TL;DR

The Vizburo UI package is the only major part of Politburo that has been left behind by the platform-wide refactor. It still uses package-level helpers, free-function handlers with parameter-list DI, parallel template renderer code, and a Nord palette pinned via inline styles, while the rest of the codebase has moved to struct-based handlers, `app.App` DI, `infra/templates.Renderer`, and a token-driven design system. The fix is mostly mechanical — adopt patterns that already exist in the repo — not invention. The single real product decision is which **theme** wins (Nord vs Deep Void) and whether mobile is a real target.

---

## 1. Handler Structure

### Hits

- `PrepareTemplateData` exists and is genuinely useful — the *shape* of the data abstraction is right.
- Auth boundary is right: dashboard handlers sit behind UI session middleware that populates `auth.GetSessionData(ctx)`, so handlers never re-authenticate.
- Page vs partial split is conceptually present (`RenderTemplate` / `RenderPartial`), even if implemented poorly.

### Misses

1. **Free-function handlers with parameter-list DI.** `LogbookFlightsHandler(w, r, flightSvc)`, `FlightMapHandler(w, r, cache, liveAPI, flightSvc)` — every new dependency forces a signature change at both call site (router closure) and definition. `FlightMapHandler` already takes five params; adding airport data lookup makes it six. This is the canonical "primitive obsession in DI" smell.
2. **Two `PrepareTemplateData` implementations.** One lives at `politburo/vizburo/ui/template_helpers.go:11` (no `MenuItems`, uses `log.Printf`), the other at `politburo/infra/templates/session_helpers.go:13` (has `MenuItems`, uses `infra/logging`). The vizburo copy is the **older, inferior fork**.
3. **Two renderer implementations.** `politburo/vizburo/ui/utils.go:44-155` re-parses templates and partials from disk on every request via `ParseGlob` + `ParseFiles`. `politburo/infra/templates/renderer.go` is the canonical, struct-based, project-root-aware renderer that the rest of the app uses. Vizburo carries dead-weight duplicate code.
4. **Boilerplate session extraction.** ~8 lines × 20+ handlers (`event_handlers.go:23-35`, `pirep_config_handlers.go:46-64`, repeated again in `Update`, `Toggle`, `Edit`, `Delete`, `Create`, etc.). Every handler can subtly differ on whether it returns 401, 500, or "Invalid session data" — the worst kind of inconsistency.
5. **Imports the dead packages CLAUDE.md tells us not to use.** `event_handlers.go:14-15` imports `internal/db/repositories` and `internal/services` — both flagged as legacy. `pirep_config_handlers.go:12` does the same. New vizburo code is being layered on top of code we are trying to delete.
6. **`handlers.go` is 823 lines.** It mixes flight UI, pilot UI, map handlers, and a Ramer–Douglas–Peucker downsampling algorithm. Domain bleed, file-size bleed, and unrelated algorithmic code all in one place.
7. **Mixed response idioms.** Event handlers POST and return raw JSON (`event_handlers.go:281-286`) instead of either swapping a partial (the HTMX way) or going through `httpdto`. Client-side JS must then parse this and trigger a refetch — extra round trip, inconsistent contract.

### Recommended Target Pattern

Adopt the same struct-based handler shape that domain packages already use, applied **per page-area sub-package** inside `vizburo/ui/`. One monolithic `ui.Handler` would be too wide; per-domain handlers keep file sizes bounded.

Proposed package layout:

```
vizburo/ui/
  ui.go                     — package doc, shared types
  context.go                — UIContext{ Session, ActiveVA, IsAdmin, IsStaff, Renderer }; ResolveUIContext(r) helper
  middleware.go             — UIContextMiddleware: extracts session + VA once, stuffs UIContext on request context
  pages/
    dashboard/handler.go    — type Handler struct { renderer; dashboardSvc }; (h *Handler) Index() http.HandlerFunc
    logbook/handler.go
    flightmap/
      handler.go
      downsample.go         — the RDP algorithm gets its own file
    events/handler.go       — page + partial handlers for /dashboard/events
    pireps/handler.go
    auth/handler.go
  routes.go                 — RegisterUIRoutes(r chi.Router, deps *app.App) wires each handler
```

Construction wiring lives in `app.New` under `FeatureDeps` (e.g. `application.Features.UI.Dashboard`, `application.Features.UI.Logbook`). The router calls:

```go
r.Get("/dashboard", application.Features.UI.Dashboard.Index())
```

This is identical in shape to how `internal/events` and `internal/vaadmin` already register.

### Eliminating the Boilerplate

Three layered fixes:

1. **A single `UIContextMiddleware`** runs once per `/dashboard/*` request, extracts `*session.SessionData`, resolves `ActiveVA`, builds role flags, and stores a typed `*UIContext` on `r.Context()`. Missing session → 302 to `/auth/login` (pages) or 401 (partials with `HX-Request` header) — single consistent contract.
2. **A typed accessor** `ui.MustContext(r) *UIContext`. Replaces every 8-line block with one line.
3. **`PrepareTemplateData` becomes a method on `*UIContext`** — `uctx.TemplateData("PIREP Configuration")`. Kill the vizburo-local copy in favour of the `infra/templates` version.

What a handler looks like end-to-end:

```go
func (h *Handler) PirepConfig() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        uctx := ui.MustContext(r)
        data := uctx.TemplateData("PIREP Configuration")
        if err := h.renderer.RenderTemplate(w, "pages/pirep-config.html", data); err != nil {
            uctx.Log.Error("render failed", "err", err)
        }
    }
}
```

That replaces 30+ lines per handler.

### File Split Policy

- `*_page_handlers.go` — full pages (return `RenderTemplate`)
- `*_partial_handlers.go` — HTMX fragments (return `RenderPartial`)
- One package per page-area (above)
- Algorithms (RDP, route enrichment) live in their own file, not handler files

---

## 2. Theming and Styling

### Hits

- `static/css/design-system.css` is genuinely good. It is mobile-first, token-driven, well-named, has a full component vocabulary (`.card`, `.btn-primary`, `.nav-link`, `.app-shell`, `.page-header`), and proper touch-target sizing.
- Tailwind is already in the build pipeline both at root and inside vizburo — no new tooling needed.

### Misses

1. **Two themes coexist.** Deep Void (`--bg-app: #0f111a`, `--accent-primary: #3b82f6`) in `design-system.css` vs Nord (`--nord0: #2E3440`) embedded in `vizburo/ui/templates/layouts/base.html`. Different palettes, different visual languages.
2. **Nord is inlined.** Lives in a `<style>` block inside `base.html`, applied via `style=""` attributes throughout every page/partial. Changing the theme requires touching every template — the opposite of why design tokens exist.
3. **Two Tailwind configs, both empty.** `politburo/tailwind.config.js` and `vizburo/ui/tailwind.config.js` are identical, both have `theme.extend: {}`. Two output CSS files. The token system in `design-system.css` is *not* exposed to Tailwind — so utility classes like `bg-accent-primary` don't exist.
4. **No component class layer in vizburo.** Templates can't say `class="card"`; they must say `class="rounded-lg shadow-sm" style="background-color: var(--nord1); border: 1px solid var(--nord3);"`. Verbose and brittle.

### Recommendation — Single Token System, Tailwind-Backed

**Adopt Deep Void / `design-system.css` as the single source of truth.** Reasons:

- It already exists, is feature-complete, and has the mobile story attached.
- The Nord setup is younger, less complete (no FAB, no drawer), and lives only in one template.
- Migrating *to* design-system.css means deleting code. Migrating *to* Nord means writing it from scratch.
- Deep Void's higher contrast is better for an admin/data dashboard than Nord's soft palette.

If product prefers the Nord *look*, port the Nord palette into `design-system.css` as a theme variant — but keep the architecture (tokens + component classes + mobile shell). Do not preserve the Nord *implementation*.

Concrete unification path:

1. **Single Tailwind config at root.** Delete `vizburo/ui/tailwind.config.js`. Set `content` glob to both `templates/**/*.html` and `vizburo/ui/templates/**/*.html`. Single CSS output served from `/static/css/output.css`.
2. **Expose tokens to Tailwind.** Move palette into `tailwind.config.js`'s `theme.extend.colors` referencing CSS custom properties (`'bg-app': 'var(--bg-app)'`). Now both component classes (`.card`) and utilities (`bg-bg-app`, `text-text-primary`) work and stay in sync.
3. **Vizburo `base.html` swaps to design-system shell.** Use `.app-shell` grid, `.desktop-tenant-sidebar`, `.desktop-header`, `.main-content`. Delete the inline `<style>` block.
4. **Template hygiene rule.** Lint/grep CI for `style="` inside `vizburo/ui/templates/` — block by default. Only allow where a dynamic computed value (e.g. progress bar width from data) is unavoidable.
5. **Vizburo input.css** keeps `@tailwind` directives + `@import` of `design-system.css`. One pipeline, one output.

### Effort vs Gain

- **High gain, medium effort:** templates need updating to use component classes, but page-by-page without breaking anything.
- **Low gain, high effort:** keeping both themes "for now" — the inline styles compound every time a new page is added. Do this migration before adding more pages.

---

## 3. Responsiveness

### Hits

- `design-system.css` already contains a full mobile strategy: drawer, mobile header, overlay, 44px touch targets. None of it needs to be designed; it needs to be wired up for pages that warrant it.

### Misses

- Vizburo currently has no mobile strategy at all — just `body { flex-direction: column }` at 768px. Pages that community members will open on their phones (live flights, rankings) look broken. Pages that are admin-only desktop workflows don't need fixing, but they currently don't signal that they're incompatible either.
- The two heaviest templates (`live.html` at 1632 lines, `logbook.html` at 1441 lines) interleave desktop and mobile markup throughout and toggle visibility with CSS + JS. The desktop and mobile interaction models are genuinely different (split-pane + click vs full-map + bottom-sheet + two-tap) — they should be separate templates, not one tangled one.

### Recommendation — Server-Side Viewport Awareness + Per-Partial Divergence

**The handler layer knows whether the request is mobile or desktop, and serves different partials accordingly.** This replaces CSS `display:none` toggling of unused markup with clean separate templates per device class. The primary unit of divergence is the **partial** (HTMX fragment), but component-level divergence within a partial is equally possible using the same mechanism.

#### How viewport reaches the server

Two-layer approach:

1. **User-Agent sniff** — for the very first full-page HTML response. Coarse (device class, not pixel width) but zero round-trip cost. Sets a safe default so the initial paint is not broken.
2. **JS-set cookie** — a small blocking `<script>` in `<head>` writes a `vp=mobile|desktop` cookie based on `window.innerWidth` before any HTMX request fires. An `htmx:configRequest` hook refreshes it on resize. This cookie is accurate, browser-maintained, and survives page navigation without a network round trip.

Viewport state lives in the **`vp` cookie, not in the Redis session**. Cookies are per-browser-instance; the session is per-login. A user on phone + laptop has two independent `session_id` cookies and two Redis entries already — the `vp` cookie follows the same scope naturally, with no multi-device coordination needed in the handler layer.

#### Middleware wiring

`UIContextMiddleware` (from §1) gains a `IsMobile bool` field on `*UIContext`, resolved at middleware time:

```go
// precedence: JS cookie > UA sniff
func resolveViewport(r *http.Request) bool {
    if c, err := r.Cookie("vp"); err == nil {
        return c.Value == "mobile"
    }
    return isMobileUA(r.Header.Get("User-Agent"))
}
```

Handlers never read the cookie directly — they read `uctx.IsMobile`.

#### Partial-level divergence (the main pattern)

Page shells remain shared. Only the **HTMX-loaded content partial** diverges:

```go
func (h *Handler) FlightsPartial() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        uctx := ui.MustContext(r)
        tmpl := "partials/live-flights-desktop.html"
        if uctx.IsMobile {
            tmpl = "partials/live-flights-mobile.html"
        }
        h.renderer.RenderPartial(w, tmpl, data)
    }
}
```

The desktop partial renders the split-pane list + map. The mobile partial renders the full-map + bottom-sheet flow. Each is a clean, focused file with no dead markup. The HTMX endpoints for data (flight selection, waypoints) are shared — only the presentation partial diverges.

**First-visit edge case:** the `vp` cookie is absent on the very first page load. The UA sniff handles this; the JS cookie is then set and subsequent HTMX partial requests use it. Default to mobile (smaller, simpler) when UA is ambiguous.

#### Per-page classification (unchanged rule)

**Mobile-incompatible pages** (admin/config workflows):
- Datasource configuration, flight mode editor, event leg management, webhook management, PIREP config, pilot role management.
- When opened on ≤ 768px, render a full-screen guard component: *"This page is designed for desktop."* Nothing else renders.
- Implementation: `IsMobile` check in the handler — return the guard partial instead of the page partial. No CSS gymnastics.

**Mobile-first pages** (community-facing, high mobile traffic):
- Live flights, pilot rankings, leaderboard/stats — opened on phones to check a flight or a rank.
- These get genuinely different mobile partials. Desktop and mobile templates are maintained separately. Shared data structs, shared HTMX endpoints, divergent presentation.

**Architectural rule:** Mobile classification of each page is a product decision stated explicitly in the architect plan — not inferred by the developer. The architect specifies `mobile-first` or `mobile-incompatible` per page area. The `IsMobile` field and guard component are available infrastructure; the architect decides which applies and at what granularity (full page vs specific partials).

---

## 4. Performance

### Hits

- HTMX-first architecture is the right call — minimal JS, server-side rendering, partial swaps avoid full page reloads.
- `infra/cache` Redis service exists and is well-instrumented.
- Aircraft/livery and flight caching already exist at the data layer.

### Misses (priority order)

| Issue | Fix | Impact |
|---|---|---|
| Templates re-parsed from disk **on every request** (`vizburo/ui/utils.go:44` calls `ParseGlob`+`ParseFiles` per request) | Cache parsed `*template.Template` map in `infra/templates.Renderer` at startup; re-parse only in `local` env | **Highest** |
| Two renderers (vizburo's own vs `infra/templates.Renderer`) | Kill `vizburo/ui/utils.go` rendering code entirely | High |
| `FlightMapHandler` serial API calls (`liveAPI.GetUserByIfcId` then `flightSvc.GetUserFlights`) | Parallelise with `errgroup.WithContext` | Medium |
| RDP downsampling not cached — O(n²) worst case runs on every request for same flight | Cache simplified route in Redis; TTL 5min (active) / 24h (completed) | Medium |
| Global `hx-indicator` on body — every partial swap dims the whole page | Scope per-partial: `hx-indicator="#card-spinner"` targeting a spinner inside the swapped partial | Low (perception) |
| `log.Printf` everywhere in vizburo (rest of app uses `infra/logging` Zap) | Replace with `infra/logging` | Low |

### Do not do

- **Don't swap the template engine** (Templ, Pongo, etc.). `html/template` is fine once parsing is cached. Migrating ~50 templates is not justified by marginal speedup.
- **Don't add a service worker or client-side route cache.** HTMX + cached partials covers 95% of perceived speed gain at zero JS cost.

---

## How This Lands in `app.App`

No new DI tiers. Natural fit into existing structure:

- `InfraDeps.Renderer` — already exists; vizburo handlers consume it instead of `ui.RenderTemplate`.
- `PlatformDeps` — unchanged.
- `FeatureDeps.UI` — new struct with one field per page-area handler (`Dashboard`, `Logbook`, `FlightMap`, `Events`, `PirepConfig`, `Datasource`, `Auth`). Each constructed with the platform/feature services it needs and the shared renderer.

Router wiring in `routes/router.go`:

```go
r.Route("/dashboard", func(r chi.Router) {
    r.Use(application.Middleware.UISession)
    r.Use(ui.UIContextMiddleware)
    r.Get("/", application.Features.UI.Dashboard.Index())
    r.Get("/logbook", application.Features.UI.Logbook.Page())
    r.Get("/logbook/flights", application.Features.UI.Logbook.FlightsPartial())
    // ...
})
```

Identical in shape to how `internal/events` and `internal/vaadmin` already register.

---

## Migration Order (Risk-Bounded)

| Step | Work | User-visible? |
|---|---|---|
| 1 | Renderer consolidation + template caching in `infra/templates.Renderer` | No |
| 2 | `UIContextMiddleware` + `UIContext` typed accessor | No |
| 3 | Handler struct migration, one page area at a time; delete legacy imports | No |
| 4 | Theme unification — `base.html` → `.app-shell`, kill Nord `<style>` block | **Yes** |
| 5 | Mobile tier-1 — `vp` cookie + `IsMobile` on `UIContext`; split heavy partials (Live Flights, Logbook) into desktop/mobile variants; guard component for admin pages | Yes |
| 6 | Perf polish — parallelise FlightMap, cache RDP output, scope `hx-indicator` | No |

Steps 1–3 are pure refactor with no visible change — safe to ship continuously. Step 4 is the only one that ships a visible product change. Steps 5 and 6 can interleave after step 4.
