# Error & Auth Pages Redesign — Implementation Plan

> Date: 2026-05-13
> Scope: Standalone error pages (401, 404, 405) and auth login page. Sets reference pattern for all future Vizburo pages.

## Context

The standalone error pages (401/404) and the token login path in Vizburo are currently a thin, blue-accented variant of the dashboard styling. We are redesigning them with a mobile-first **sunset/warning LED palette**, fixing the latent bug where the 405 handler renders the 404 template, and giving the token login flow a proper HTML page for the expired/invalid token case (it currently calls `http.Error`, returning plain text).

## Existing Reuse

- `templates/layouts/error.html` — standalone wrapper already used by `RenderStandalone`. Has `{{block "styles" .}}` and `{{block "content" .}}` slots.
- `infra/templates/renderer.go:Renderer.RenderStandalone` — render path for all four pages.
- `internal/routes/router.go:334-345` `handleNotFound` — already thin, registers `r.NotFound(...)`.
- `internal/routes/router.go:347-359` `handleMethodNotAllowed` — already thin, but renders the wrong template (`pages/404.html` instead of `pages/405.html`).
- `internal/routes/router.go:411-420` `render401` — render path for the 401 page; already passes `PageTitle`.
- `internal/auth/handler.go:60-117` `Handler.TokenLogin` — handles `GET /auth/login?token=...`. Currently returns plain text on empty token (400) and on invalid/expired token (401).

## Architecture Decisions

- **One template, two states for auth login.** Both the "validating" and "expired/invalid" states share `pages/auth-login.html` and are switched by a `TokenExpired` boolean in template data. Handler stays thin.
- **No JS-driven validation flow.** The server processes the token synchronously and either redirects (success) or renders the page with `TokenExpired=true`. The CSS pulse animation exists only as a visual artefact during the brief render window on slow links.
- **Scoped `<style>` blocks per page.** Page-specific CSS lives in `{{define "styles"}}` inside each template, consistent with current 401/404 convention. New shared tokens go in `design-system.css`. No new global classes.
- **Mobile-first responsive CSS only.** No separate mobile partial. Touch targets ≥ 44px, command boxes stack at ≤ 640px. These pages must work on phones.

## Files Changed

| File | Action |
|---|---|
| `static/css/design-system.css` | Add `--err-*` sunset palette tokens |
| `templates/layouts/error.html` | Add Inter + JetBrains Mono Google Fonts; set body font/bg/color |
| `templates/pages/401.html` | Rewrite with sunset palette + Discord command boxes |
| `templates/pages/404.html` | Rewrite with star display + two action buttons |
| `templates/pages/405.html` | **New** — proper 405 page (fixes bug: currently renders 404 template) |
| `templates/pages/auth-login.html` | **New** — two-state template (validating / expired) |
| `internal/auth/handler.go` | Replace bare `http.Error` calls with `renderExpired()` helper |
| `internal/app/app.go` | Pass `TemplateRenderer` into `auth.NewHandler` constructor |
| `internal/routes/router.go` | Fix `handleMethodNotAllowed` to use `pages/405.html` |

## Routing & Scopes

| Route | Type | Template | Status |
|---|---|---|---|
| `GET /auth/login` | Standalone page | `pages/auth-login.html` | Public |
| `404 fallback` | Standalone page | `pages/404.html` | Public |
| `405 fallback` | Standalone page | `pages/405.html` | Public |
| `401` (from `uiAuthMiddleware`) | Standalone page | `pages/401.html` | No session |

No new routes. No middleware changes.

---

## Phase 1 — CSS Token Extension

**File:** `static/css/design-system.css`
**Location:** Append inside `:root { ... }` block, after `--fab-item-size: 45px;`, before the closing `}`.

```css
/* Sunset / Warning LED palette — standalone error & auth pages */
--err-bg: #0d0f13;
--err-surface: #14161b;
--err-ink: #e7e4dc;
--err-ink-soft: #94a3b8;
--err-ink-mute: #64748b;
--err-accent: #f0c674;
--err-warn: #d9b261;
--err-danger: #d97766;
--err-ok: #8fc28a;
--err-border: #2a2d35;
--err-line-soft: #1e293b;
```

No other changes to `design-system.css`. The Deep Void blue palette is preserved verbatim.

---

## Phase 2 — Layout Update

**File:** `templates/layouts/error.html`

Two edits:

1. Inside `<head>`, after `<meta name="viewport">`, before the existing `<link rel="stylesheet">`:
   ```html
   <link rel="preconnect" href="https://fonts.googleapis.com">
   <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
   <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
   ```

2. Update `<body>` tag:
   ```html
   <body style="font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif; background-color: var(--err-bg); color: var(--err-ink);">
   ```
   The inline style is a permitted exception — it applies to the entire standalone layout and cannot be scoped per-page otherwise.

---

## Phase 3 — Template Rewrites

### 3a. `templates/pages/401.html` (rewrite)

CSS vocabulary in `{{define "styles"}}`:

- `.err-page` — flex column, `min-height: 100vh`, `justify-content: space-between`, `padding: 2rem 1.5rem`
- `.err-stack` — flex column centered, `gap: 1.25rem`, `max-width: 720px`, `margin: auto`, `text-align: center`
- `.err-star` — JetBrains Mono, `font-size: 1.875rem`, `color: var(--err-accent)`
- `.err-mono` — JetBrains Mono, `font-size: 0.8125rem`, `letter-spacing: 0.05em`
- `.err-warn` — `color: var(--err-warn)`
- `.err-mute` — `color: var(--err-ink-mute)`
- `.err-h1` — Inter 600, `font-size: 1.875rem`, `color: var(--err-ink)`
- `.err-body` — Inter 400, `font-size: 1rem`, `line-height: 1.6`, `color: var(--err-ink-soft)`, `max-width: 520px`
- `.err-cmd-row` — flex row, `gap: 1rem`, `flex-wrap: wrap`, `justify-content: center`
- `.err-cmd-box` — `background: var(--err-surface)`, `border: 1px solid var(--err-border)`, `border-radius: 8px`, `padding: 1rem 1.25rem`, flex column, `min-width: 180px`, `text-align: left`
- `.err-cmd-name` — JetBrains Mono 500, `color: var(--err-accent)`, `font-size: 0.95rem`
- `.err-cmd-desc` — Inter 400, `color: var(--err-ink-mute)`, `font-size: 0.8125rem`, `margin-top: 0.25rem`
- `.err-footer` — Inter 400, `color: var(--err-ink-mute)`, `font-size: 0.75rem`, `text-align: center`
- `.err-footer a` — `color: inherit`, `text-decoration: none`; hover `color: var(--err-accent)`
- `@media (max-width: 640px)`: `.err-cmd-row { flex-direction: column; align-items: stretch; }`, `.err-h1 { font-size: 1.5rem; }`, `.err-cmd-box { min-width: 0; }`

HTML structure in `{{define "content"}}`:

```html
<main class="err-page" id="error-401-page">
  <div class="err-stack">
    <span class="err-star">★</span>
    <span class="err-mono err-warn">401 · session missing or expired</span>
    <h1 class="err-h1">Sign-in lives in Discord</h1>
    <p class="err-body">
      Vizburo doesn't hold passwords. To get back in, open the bot and run one of these.
    </p>
    <div class="err-cmd-row">
      <div class="err-cmd-box">
        <span class="err-cmd-name">/link</span>
        <span class="err-cmd-desc">first time</span>
      </div>
      <div class="err-cmd-box">
        <span class="err-cmd-name">/dashboard</span>
        <span class="err-cmd-desc">open dashboard</span>
      </div>
      <div class="err-cmd-box">
        <span class="err-cmd-name">/live</span>
        <span class="err-cmd-desc">live radar</span>
      </div>
    </div>
  </div>
  <footer class="err-footer">
    ★ <a href="https://github.com/infinite-experiment">github.com/infinite-experiment</a>
  </footer>
</main>
```

### 3b. `templates/pages/404.html` (rewrite)

Additional CSS classes:

- `.err-stars` — Inter 600, `font-size: 4rem`, `color: var(--err-accent)`, `letter-spacing: 0.5rem`, `line-height: 1`
- `.err-btn-row` — flex row, `gap: 0.75rem`, `flex-wrap: wrap`, `justify-content: center`
- `.err-btn` — `padding: 0.75rem 1.25rem`, `border-radius: 6px`, Inter 500, `min-height: 44px`, inline-flex center, `text-decoration: none`, `font-size: 0.95rem`
- `.err-btn-primary` — `background: var(--err-accent)`, `color: var(--err-bg)`; hover `filter: brightness(1.1)`
- `.err-btn-ghost` — `background: transparent`, `border: 1px solid var(--err-border)`, `color: var(--err-ink-soft)`; hover `border-color: var(--err-ink-soft)`, `color: var(--err-ink)`
- `@media (max-width: 480px)`: `.err-btn-row { flex-direction: column; align-items: stretch; }`, `.err-stars { font-size: 2.75rem; }`

HTML structure:

```html
<main class="err-page" id="error-404-page">
  <div class="err-stack">
    <span class="err-mono err-mute">404 · not found</span>
    <h1 class="err-stars">★ ★ ★</h1>
    <p class="err-body">
      This page hasn't been built. Or it was — and the comrades retired it.
    </p>
    <div class="err-btn-row">
      <a href="/dashboard" class="err-btn err-btn-primary">← dashboard</a>
      <a href="https://github.com/infinite-experiment/issues" class="err-btn err-btn-ghost">open an issue on github</a>
    </div>
  </div>
  <footer class="err-footer">
    ★ <a href="https://github.com/infinite-experiment">github.com/infinite-experiment</a>
  </footer>
</main>
```

### 3c. `templates/pages/405.html` (new file)

Reuses the same CSS vocabulary as 401/404 (duplicated per-template per project convention).

HTML structure:

```html
{{define "styles"}}
<style>
  /* err-* vocabulary — same as 401/404 */
  ...
</style>
{{end}}

{{define "content"}}
<main class="err-page" id="error-405-page">
  <div class="err-stack">
    <span class="err-mono err-warn">405 · method not allowed</span>
    <h1 class="err-h1">Wrong door</h1>
    <p class="err-body">That request method isn't supported here.</p>
    <div class="err-btn-row">
      <button type="button" id="err-back-btn" class="err-btn err-btn-primary">← back</button>
    </div>
  </div>
  <footer class="err-footer">
    ★ <a href="https://github.com/infinite-experiment">github.com/infinite-experiment</a>
  </footer>
</main>
<script>
  document.getElementById('err-back-btn').addEventListener('click', () => history.back());
</script>
{{end}}
```

---

## Phase 4 — Auth Login Page

### Current state (`internal/auth/handler.go`)

- Line 67: empty token → `http.Error(w, "Token required", 400)` — plain text
- Line 75: invalid/expired → `http.Error(w, "Invalid or expired token: ...", 401)` — plain text, leaks error string
- Lines 100-115: success → set cookie + `http.Redirect(303)` — unchanged

No `templates/pages/auth-login.html` exists today.

### New template: `templates/pages/auth-login.html`

CSS additions:

- `.err-pulse` — inline-flex, `gap: 0.375rem`, `margin-top: 0.5rem`
- `.err-pulse span` — 8×8px circle, `background: var(--err-accent)`, `animation: errPulse 1.2s ease-in-out infinite`; `:nth-child(2)` delay 0.2s; `:nth-child(3)` delay 0.4s
- `@keyframes errPulse` — `0%,100%` opacity 0.25, scale 0.85 → `50%` opacity 1, scale 1
- `.err-big-star` — JetBrains Mono, `font-size: 3.5rem`, `color: var(--err-accent)`
- `.err-danger` — `color: var(--err-danger)`

HTML structure:

```html
{{define "content"}}
<main class="err-page" id="auth-login-page" data-state="{{if .TokenExpired}}expired{{else}}validating{{end}}">
  {{if .TokenExpired}}
    <div class="err-stack">
      <span class="err-mono err-danger">EXPIRED · token {{.TokenShort}}</span>
      <h1 class="err-h1">That link's already cold</h1>
      <p class="err-body">
        Single-use links expire after 10 minutes for safety. Open Discord and ask the bot for a fresh one.
      </p>
      <div class="err-cmd-box">
        <span class="err-cmd-name">/dashboard</span>
        <span class="err-cmd-desc">the bot will DM you a new sign-in link</span>
      </div>
    </div>
    <footer class="err-footer">
      ★ free + open source ·
      <a href="https://github.com/infinite-experiment">github.com/infinite-experiment</a>
    </footer>
  {{else}}
    <div class="err-stack">
      <span class="err-big-star">★</span>
      <p class="err-body">Validating your one-time link…</p>
      <div class="err-pulse"><span></span><span></span><span></span></div>
      <span class="err-mono err-mute">links expire after 10 minutes</span>
    </div>
  {{end}}
</main>
{{end}}
```

### Handler changes (`internal/auth/handler.go`)

1. Add `renderer *templates.Renderer` field to `Handler` struct (line ~18).
2. Add `renderer *templates.Renderer` parameter to `NewHandler` (line ~23).
3. Replace `http.Error(w, "Token required", 400)` at line 67 with `h.renderExpired(w, "")`.
4. Replace `http.Error(w, fmt.Sprintf("Invalid or expired token: %v", err), 401)` at line 75 with `h.renderExpired(w, shortToken(token))`.
5. Add helpers at the bottom of the file:

```go
func (h *Handler) renderExpired(w http.ResponseWriter, tokenShort string) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    data := map[string]interface{}{
        "PageTitle":    "Sign-in link expired",
        "TokenExpired": true,
        "TokenShort":   tokenShort,
    }
    if err := h.renderer.RenderStandalone(w, "pages/auth-login.html", data); err != nil {
        logging.Error("Failed to render auth-login page", "error", err)
        http.Error(w, "Sign-in unavailable", http.StatusInternalServerError)
    }
}

func shortToken(t string) string {
    if len(t) > 6 {
        return t[:6]
    }
    return t
}
```

Also add before `renderExpired` call:
```go
logging.Warn("Auth login rendered expired state", "token_prefix", shortToken(token))
```

### DI wiring (`internal/app/app.go`)

Find the `auth.NewHandler(...)` call in the `FeatureDeps` initialization block and pass `a.Infra.TemplateRenderer` as an additional argument. One-line change; no other ripple effects.

---

## Phase 5 — Router Bug Fix

**File:** `internal/routes/router.go`
**Line:** 355

Change:
```go
if err := templateRenderer.RenderStandalone(w, "pages/404.html", data); err != nil {
```
to:
```go
if err := templateRenderer.RenderStandalone(w, "pages/405.html", data); err != nil {
```

The `data` map (`PageTitle: "Method Not Allowed"`) is already correct.

---

## Testing Plan

### Unit Tests

- `internal/auth/handler_test.go` → empty token: assert `renderExpired` called with `TokenShort=""`, `TokenExpired=true`
- `internal/auth/handler_test.go` → invalid token: stub `CreateSessionFromToken` to error; assert `TokenShort` is first 6 chars of input
- `internal/auth/handler_test.go` → success: assert cookie set, 303 redirect, renderer NOT called
- `internal/routes/router_test.go` → POST to GET-only route: assert 405 status, body contains `id="error-405-page"`
- `internal/routes/router_test.go` → GET nonexistent path: assert 404, body contains `id="error-404-page"`

### Manual Verification

1. `air` running; visit `http://localhost:8080/does-not-exist` → 404 page renders with new sunset palette and Inter/JetBrains Mono fonts
2. `curl -X POST http://localhost:8080/healthCheck` → 405 page renders (not 404)
3. Visit `/dashboard` without session cookie → 401 page renders with three command boxes; check on mobile 375px: boxes stack vertically
4. Visit `/auth/login` (no token) → auth-login expired state renders
5. Visit `/auth/login?token=invalid` → expired state with `EXPIRED · token invali` mono label
6. Real signed link via bot → redirect to `/dashboard` on success
7. DevTools: confirm zero hardcoded hex in new style blocks; confirm no `--nord*` references; confirm Inter + JetBrains Mono loaded

---

## Out of Scope

- Restyling dashboard, live, logbook, events, vaadmin, datasource, or livery-mappings pages
- Adding a dedicated `/auth/validating` route or async token validation flow
- Self-hosting Google Fonts
- Replacing `--accent-primary` and the Deep Void blue palette globally
- Localization / i18n of error copy
- Prometheus metrics for expired-link renders (log line is sufficient)
- Promoting the `<body>` inline `font-family` to a CSS class
