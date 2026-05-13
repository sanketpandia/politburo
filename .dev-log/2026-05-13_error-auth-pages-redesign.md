# Error & Auth Pages Redesign

> Feature branch: `chore/pending-changes`
> Date: 2026-05-13

---

### redesign error/auth standalone pages with sunset palette (`3ccbfd1`)

**Changed**
- `static/css/design-system.css` — appended 11 `--err-*` CSS custom properties (sunset/warning LED palette) inside `:root` after `--fab-item-size`
- `templates/layouts/error.html` — added Google Fonts preconnect + Inter/JetBrains Mono stylesheet links; updated `<body>` inline style to use `font-family: 'Inter'...` and `background-color: var(--err-bg); color: var(--err-ink)`
- `templates/pages/401.html` — full rewrite: `.err-*` CSS vocabulary, star display, three Discord command boxes, mobile breakpoint at 640px
- `templates/pages/404.html` — full rewrite: `.err-*` CSS vocabulary, star trio display, two action buttons, mobile breakpoint at 480px
- `templates/pages/405.html` — new file: "Wrong door" page with `history.back()` JS attached via `addEventListener`
- `templates/pages/auth-login.html` — new file: two-state template (validating / expired) keyed on `.TokenExpired`, pulse animation, `/dashboard` command box in expired state

**Reused**
- `templates/layouts/error.html` — existing standalone wrapper with `{{block "styles" .}}` and `{{block "content" .}}` slots; extended rather than replaced
- `infra/templates/renderer.go:RenderStandalone` — existing render path used by all four pages; no changes needed

**Metrics added**
None.

**Logging added**
None in this commit.

**Test surface**
- `templates/pages/401.html` — visual: three command boxes stack at ≤640px
- `templates/pages/404.html` — visual: buttons stack at ≤480px
- `templates/pages/405.html` — unit: POST to GET-only route → 405 + body contains `id="error-405-page"`
- `templates/pages/auth-login.html` — unit: TokenExpired=true → expired state rendered; TokenExpired=false → validating state

**Live API compliance**
N/A — no Live API calls in this commit.

**Build status**
`go build ./...` passed.

**Notes**
- Inline `style=` on `<body>` is a permitted exception per plan: it applies to the entire standalone layout and cannot be scoped per-page in a `{{block "styles"}}` that runs inside `<head>`.
- All new CSS uses `var(--err-*)` tokens only — no hardcoded hex, no `--nord*` references.

---

### wire auth handler to render HTML expired page; fix 405 template (`13eac4b`)

**Changed**
- `internal/auth/handler.go:Handler` — added `renderer *templates.Renderer` field
- `internal/auth/handler.go:NewHandler` — added `renderer *templates.Renderer` parameter
- `internal/auth/handler.go:TokenLogin` — replaced `http.Error(w, "Token required", 400)` with `h.renderExpired(w, "")` and `http.Error(w, fmt.Sprintf("Invalid or expired token: %v", err), 401)` with `logging.Warn(...)` + `h.renderExpired(w, shortToken(token))`
- `internal/auth/handler.go:renderExpired` — new method: sets 200 OK, renders `pages/auth-login.html` with `TokenExpired=true`
- `internal/auth/handler.go:shortToken` — new pure helper: first 6 chars of token or full token if shorter
- `internal/app/app.go:initFeatures` — changed `auth.NewHandler(authSvc)` to `auth.NewHandler(authSvc, a.Infra.TemplateRenderer)`
- `internal/routes/router.go:handleMethodNotAllowed` — changed template path from `"pages/404.html"` to `"pages/405.html"`

**Reused**
- `infra/templates.Renderer` — injected via existing DI pattern; `a.Infra.TemplateRenderer` already wired to every other handler that needs it
- `infra/logging.Error` / `infra/logging.Warn` — existing structured logger; same call pattern as rest of `handler.go`

**Metrics added**
None — plan explicitly states log line is sufficient for expired-link renders.

**Logging added**
| File:function | Level | Fields logged | Trigger condition |
|---|---|---|---|
| `internal/auth/handler.go:TokenLogin` | Warn | `token_prefix` | invalid/expired token before rendering expired page |
| `internal/auth/handler.go:renderExpired` | Error | `error` | template render failure (fallback to plain http.Error) |

**Test surface**
- `internal/auth/handler_test.go` — empty token: assert `renderExpired` called with `TokenShort=""`, `TokenExpired=true`, status 200
- `internal/auth/handler_test.go` — invalid token (stub `CreateSessionFromToken` to error): assert `TokenShort` is first 6 chars of input, status 200
- `internal/auth/handler_test.go` — valid token: assert cookie set, 303 redirect, renderer NOT called
- `internal/routes/router_test.go` — POST to GET-only route: assert 405 status, body contains `id="error-405-page"`

**Live API compliance**
N/A.

**Build status**
`go build -buildvcs=false -o /tmp/pb-build-check ./cmd/server` passed.

**Notes**
- `renderExpired` writes 200 OK rather than 401/400. This is per plan spec: the expired page is a user-facing help screen, not a machine-readable error response.
- The unused `fmt` import in `handler.go` was left — it is still used by `GenerateSignedLink` and `DestroySessionsByIFCId`.
