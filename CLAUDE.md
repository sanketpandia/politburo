# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Politburo** is a Go backend for the Infinite Experiment Discord bot and web client. It provides REST APIs for a virtual airline system, integrating with Infinite Flight Live API, Airtable, and PostgreSQL. It also serves the **Vizburo** dashboard UI (HTMX + Tailwind, rendered server-side).

## Development Commands

### Local Development

```bash
# Start with Air (hot reload)
air

# Manual build
go build -buildvcs=false -o .air_tmp/main ./cmd/server

# Vizburo UI (separate binary, shares DB/Redis)
go build -o .air_tmp/vizburo ./cmd/vizburo
```

### Utilities

```bash
go mod tidy
go test ./...

# Focused registration/OpenAPI coverage
go test ./internal/api/... ./internal/pilots ./internal/memberships ./internal/servers ./internal/auth ./internal/platform/httpdto ./internal/platform/validation
```

## Architecture

### Entry Points

- **`cmd/server/main.go`**: Main HTTP server. Loads config → initializes `app.App` → builds router → registers jobs/workers → starts HTTP server with graceful shutdown.
- **`cmd/vizburo/main.go`**: Vizburo UI binary. **Currently broken** — calls `routes.RegisterRoutes` which doesn't exist. Needs to be updated to use `app.New` + `routes.NewRouter`.

### Application Initialization (`internal/app/app.go`)

The `App` struct is the single DI container. All dependencies flow through it. Three initialization tiers:

1. **`InfraDeps`** — database, Redis client/cache/queue, session service, URL signer, Live API client, scheduler, template renderer, metrics registry.
2. **`PlatformDeps`** — cross-cutting repositories and services: users, VA, memberships, aircraft, API keys, claims. These are domain-neutral.
3. **`FeatureDeps`** — feature-specific handlers, services, jobs, workers: pilots, servers, memberships, events, dashboard, PIREP, vaadmin, webhooks, etc.

### Routing (`internal/routes/router.go`)

`NewRouter(application *app.App) http.Handler` — pure routing, no initialization. Uses **Chi router**.

Generated registration contract coverage lives in:
- `internal/api/generated/registration/server.gen.go` — committed generated strict Chi server
- `internal/api/registration/server.go` — handwritten adapter that maps generated strict handlers onto the active registration feature handlers

Route groups:
- `/static/*` — static file serving
- `/auth/login` — token login (public)
- `/healthCheck` — health check (public)
- `/api/v1` — all require `AuthMiddleware`; sub-groups use role middleware
- `/dashboard` — requires UI session auth; sub-groups by role

Jobs and workers registration is in `internal/routes/jobs.go`:
- `RegisterScheduledJobs(application)` — cron jobs via `infra/scheduler`
- `RegisterWorkers(application)` — background goroutines

### Infrastructure Layer (`infra/`)

Horizontal infrastructure packages — no business logic:

| Package | Purpose |
|---|---|
| `infra/db` | GORM PostgreSQL connection + migrations |
| `infra/cache` | `RedisCacheService` + legacy in-memory `CacheService` |
| `infra/redis` | Redis client factory |
| `infra/queue` | Redis-backed queue service |
| `infra/liveapi` | Infinite Flight Live API HTTP client |
| `infra/logging` | Structured logger (Zap-based) |
| `infra/metrics` | Prometheus metrics registry |
| `infra/session` | Session service (Redis-backed) |
| `infra/security` | URL signer (JWT-based presigned links) |
| `infra/scheduler` | Cron job scheduler |
| `infra/templates` | Go HTML template renderer (layouts + partials) |
| `infra/providers` | Live API and Airtable provider wrappers |
| `infra/messaging` | Watermill router, publisher, subscriber wrappers; topic constants; zap logger adapter |

### Platform Layer (`internal/platform/`)

Cross-cutting domain services that multiple features depend on:

| Package | Purpose |
|---|---|
| `internal/platform/users` | User model, repo, service |
| `internal/platform/va` | VA model, repo, service, config service, webhook repo |
| `internal/platform/memberships` | Membership model, repo, service, handler |
| `internal/platform/aircraft` | Aircraft/livery model, repo, service, cache job, worker |
| `internal/platform/apikeys` | API key model and repo |
| `internal/platform/claims` | Auth claims repo |
| `internal/platform/airports` | Airport model, repo, loader |
| `internal/platform/roles` | VARole type with DB adapter |
| `internal/platform/httpdto` | Shared HTTP response helpers |

### Feature Layer (`internal/`)

Domain-feature packages — each owns its handler, service, repo, model:

| Package | Purpose |
|---|---|
| `internal/flights` | Live flight cache job, flight plan worker/monitor, service, handler, DTOs |
| `internal/pilots` | Registration, stats, management, logbook, sync job/worker, repository |
| `internal/pireps` | PIREP submit, tour handler, queue worker, sync job, validation, repository |
| `internal/events` | Events + legs CRUD (UI + API), service, repo, model |
| `internal/memberships` | User membership feature (join VA, get status) |
| `internal/servers` | Server (VA) initialization |
| `internal/vaadmin` | Admin UI: pilot management, flight modes, webhooks |
| `internal/dashboard` | Dashboard page handler + service |
| `internal/datasource` | Data provider config UI |
| `internal/liverymappings` | Livery mapping UI and API |
| `internal/sync` | Sync job container, events, history model/repo |
| `internal/sessions` | Session cache job |
| `internal/va_routes` | Route model, repo, sync job |
| `internal/webhooks` | Discord webhook handler, live flights webhook job |
| `internal/auth` | Auth service, handler, claims, request context |
| `internal/middleware` | Auth, role, request ID middleware |

### Authentication

`internal/middleware/auth.go` — `AuthMiddleware(claimsRepo, keysRepo, sessionSvc)` populates `UserClaims` from either a Vizburo session cookie or API-key bot context headers (`X-API-Key`, `X-Discord-Server-Id`, `X-Discord-User-Id`). Registration/onboarding routes also use `RequireDiscordBotContextMiddleware()` so missing Discord context returns `403` after API-key auth.

`internal/auth/claims.go` — `UserClaims` interface; `APIKeyClaims` struct.
`internal/auth/request_context.go` — `SetUserClaims`, `GetUserClaims`, `SetSessionData`, `GetSessionData`.

### Role Middleware

`internal/middleware/`:
- `IsRegisteredMiddleware()` — user record exists
- `IsMemberMiddleware()` — has active VA membership
- `IsStaffMiddleware()` — role ≥ staff
- `IsAdminMiddleware()` — role = admin
- `IsGodMiddlewareWithKey()` — special god-mode with header

### Scheduled Jobs (registered in `routes/jobs.go`)

| Job | Location | Cadence |
|---|---|---|
| Session cache | `internal/sessions/cache_job.go` | Every 5 min |
| Aircraft cache | `internal/platform/aircraft/cache_job.go` | Every hour |
| Flights cache | `internal/flights/cache_job.go` | Every minute |
| Pilot sync | `internal/pilots/sync_job.go` | Every minute |
| Route sync | `internal/va_routes/sync_job.go` | Every 10 min |
| PIREP sync | `internal/pireps/sync_job.go` | Every 5 min |
| Live flights webhook | `internal/webhooks/live_flights_webhook_job.go` | :10 and :40 each hour |

### Background Workers (registered in `routes/jobs.go`)

| Worker | Location | Purpose |
|---|---|---|
| Flight plan worker | `internal/flights/flight_plan_worker.go` | Fetches flight plans from queue |
| Flight plan queue monitor | `internal/flights/flight_plan_queue_monitor.go` | Monitors queue health |
| Pilot sync worker | `internal/pilots/sync_worker.go` | Processes pilot sync queue |
| PIREP queue worker | `internal/pireps/queue_worker.go` | Submits PIREPs to Airtable |
| Aircraft livery worker | `internal/platform/aircraft/worker.go` | Syncs aircraft/livery data every 6h |

### Database

GORM + PostgreSQL. Migrations in `infra/db/migrations/`. Models in domain packages (e.g., `internal/flights/model.go`, `internal/platform/va/model.go`) and `internal/models/gorm/` (shared GORM models).

## Key Patterns & Conventions

### Dependency Injection
All dependencies initialized in `internal/app/app.go` and injected via `App` struct. No global service instances. The router receives `*app.App` and constructs handlers inline.

### Context-Based Claims
Set via `auth.SetUserClaims(ctx, claims)`, retrieved via `auth.GetUserClaims(ctx)`. Available in any handler after `AuthMiddleware` runs.

### Handler Pattern
Handlers are structs with constructor `New*(deps...) *Handler`. Methods return `http.HandlerFunc`. Example:
```go
type Handler struct { svc *Service; renderer *templates.Renderer }
func (h *Handler) GetFoo() http.HandlerFunc { return func(w http.ResponseWriter, r *http.Request) { ... } }
```

### HTTP Responses
- API JSON: `internal/platform/httpdto/response.go` — `WriteSuccess`, `WriteError`, `WriteValidationError`
- UI HTML: `templates.Renderer.Render(w, "pages/foo.html", data)` or `RenderStandalone`

### Watermill Handler Pattern
Handlers are registered via `pireps.RegisterPirepHandlers(router, subscriber, handler)`. Each handler:
1. Sets `msg.Metadata.Set("handler_name", HandlerName)` as first line (for MetricsMiddleware).
2. Returns `nil, nil` to ACK (success or intentionally skipped).
3. Returns a non-nil error to NACK → retry → poison queue after 3 retries.
Middleware stack (outer→inner): `PoisonQueueMiddleware → MetricsMiddleware`.
Order matters: MetricsMiddleware is inner so it sees the real error before PoisonQueue suppresses it.

### Cache Keys (Redis)
- `game:live:session:{id}` — IF session data (24h)
- `game:live:sessions` — pipe-delimited session ID list (24h)
- `game:live:flight:{id}` — cached flight with waypoints (5min)
- `game:live:vaflights:{va_id}` — VA's live flight IDs (1min)
- `if:aircraft:{id}` — aircraft/livery data (1h)

### Error Handling
API errors: `http.Error()` or `httpdto.RespondError`. No panics. DB retries only at connection init.

## Known Technical Debt (as of May 2026)

**Dead code — resolved (May 2026):**
- `internal/api/` — deleted. `HealthCheckHandler` relocated to `internal/platform/health/`.
- `internal/workers/` — deleted. Dead LogbookQueue sends removed from callers first.
- `internal/jobs/` — deleted. Only caller was `internal/api/debug.go` (also deleted).
- `internal/services/*.go.old` — deleted.
- `structured-skipping-valley.md` — deleted.
- `cmd/vizburo/main.go` — fixed; now wired to `app.New` + `routes.NewRouter`.

**Partially migrated legacy packages:**
- `internal/services/` — `world_tour_service.go`, `flights_service.go`, `at_sync_service.go`, `flight_modes_config_service.go` still exist. Some are still imported. Migrate to domain packages.
- `internal/common/` — `VAConfigService`, `AirtableApiService`, `LiveAPIService` wrappers still widely imported (~25 files). Equivalents are in `infra/` and `internal/platform/va/`.
- `internal/db/repositories/` — miscellaneous repositories not yet moved to domain packages.
- `internal/models/entities/` and `internal/models/gorm/` — shared model files; domain packages should own their own models over time.

**Large files that need splitting:**
- `internal/events/handler.go` (1678 lines) → split into UI and API handlers
- `internal/pilots/stats_service.go` (1548 lines) → split stats, career mode, airtable helpers
- `internal/flights/service.go` (954 lines) → split live and cache concerns
- `internal/vaadmin/handler.go` (790 lines) → split by feature area
- `internal/pireps/service.go` (746 lines) → split core and enrichment

**Watermill dual-write:** PIREP sync job publishes to both old Redis queue (`pirep:sync:<va_id>`) and new watermill topic (`wm:pirep:sync`). Both consumers run side-by-side. After bake period, delete `internal/pireps/queue_worker.go` and remove PIREP worker from `RegisterWorkers`.

**Registration generated-code coverage:** use the focused `go test ./internal/api/... ./internal/pilots ./internal/memberships ./internal/servers ./internal/auth ./internal/platform/httpdto ./internal/platform/validation` command when validating the bot-facing registration/onboarding flow after spec or handler changes.

**comrade-bot pending deletions:** `src/commands/SyncUserHandler.ts` and `src/commands/ConfigurePilotRoleHandler.ts` are stubbed (exports only) pending physical `rm`; all router references removed.

## Environment Variables

```bash
APP_ENV=local          # local | production
DEBUG=true
PORT=8080
PG_HOST=db
PG_PORT=5432
PG_USER=ieuser
PG_DB=infinite
PG_PASSWORD=iepass
REDIS_HOST=redis
REDIS_PORT=6379
JWT_SECRET=...
```
