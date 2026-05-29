# Watermill Migration + Login/Registration Refactor — Implementation Plan

> Authored: 2026-05-13 · Scope: watermill infra, PIREP worker migration, bot endpoint hygiene, service layer standardisation, validation, rate limiting, OpenAPI spec

## Context

This plan introduces two parallel workstreams. Stream 1 replaces the hand-rolled Redis Streams consumer-group boilerplate (retry counting, stale-message claiming, worker goroutine pools) in `internal/pireps/queue_worker.go` with [watermill](https://github.com/ThreeDotsLabs/watermill) + [watermill-redisstream](https://github.com/ThreeDotsLabs/watermill-redisstream), using the existing Redis connection. Stream 2 audits every endpoint comrade-bot calls, fixes five broken/404 routes, deletes dead bot code, standardises the error envelope, adds a validation framework, and lays the groundwork for an OpenAPI spec covering the bot-facing API surface. The two streams converge into a spec-driven, cleanly-tested login/registration flow. The PIREP worker is targeted first because it is the simplest of the three workers (no flight-plan retry nuance) and proves the watermill migration path before `flight_plan_worker` and `sync_worker` are tackled in a follow-up plan.

## Existing Reuse

- `infra/queue/redis_queue.go` — existing Redis Streams producer helpers; kept during dual-write window, then trimmed.
- `infra/redis/client.go` — `*goredis.Client` injected into `InfraDeps`; watermill-redisstream consumes the same client.
- `infra/metrics/metrics.go` — `MetricsRegistry`; existing `QueueDequeuedTotal`, `QueueAcknowledgedTotal`, `QueueProcessingDuration`, `QueueErrorsTotal` label sets reused for the watermill middleware.
- `infra/logging/logger.go` — Zap-based global logger; wrapped as `watermill.LoggerAdapter`.
- `internal/pireps/queue_worker.go:168–380` — `upsertPirep` logic; extracted verbatim to `pireps/upsert.go` before the worker is deleted.
- `internal/pireps/tour_handler.go` — `svcErr.Code/.Message/.StatusCode` pattern; this is the domain-error style to standardise across all service layers.
- `internal/platform/httpdto/response.go` — `RespondSuccess` / `RespondError`; the single error envelope going forward. `internal/common/` envelope is dead.
- `internal/app/app.go` — `InfraDeps` / `FeatureDeps` / `PlatformDeps` three-tier DI; all new deps wired here.
- `internal/routes/jobs.go` — `RegisterWorkers`; PIREP worker block removed here after cutover.
- `internal/middleware/auth.go` — `AuthMiddleware`; cleaned up in this plan (cookie-value logging, dead JWT branch).
- `comrade-bot/src/services/apiService.ts` — all bot→politburo calls centralised here; dead methods deleted here.

## Architecture Decision

- **Single watermill topic** (`pirep.sync.v2`) replaces per-VA `pirep:sync:<va_id>` streams. Original design was for per-VA Airtable rate-limit isolation, but watermill consumer-group concurrency provides equivalent parallelism at current scale without the operational overhead of N streams.
- **Dual-write window** during PIREP cutover: `PirepSyncJob` publishes to both old and new topic for one release cycle; both consumers run side-by-side; verified via Prometheus before old stream is removed.
- **Spec-driven OpenAPI** (not Swaggo annotations): `oapi-codegen` generates Chi-compatible server interfaces from `api/openapi/bot.v1.yaml`. Generated code is committed. The spec is the documentation — no annotation scatter across handlers.
- **Domain error type** (`DomainError{Code, Message, StatusCode, Cause}`) standardised across `pilots`, `memberships`, `servers` services — matching the pattern already in `pireps.TourPirepService`. Deletes the ad-hoc `handleRegistrationError` and `handleJoinVAError` switch blocks.
- **In-process validation** via `github.com/go-playground/validator/v10`; 422 returned on validation failures (not 400). Centralised in `internal/platform/validation/`.
- **Redis-backed rate limiting** (`INCR`+`EXPIRE` on minute buckets) rather than in-memory token buckets, so limits hold across multiple server instances.
- **Bot dead-code deletion** for `syncUserToVA`, `assignUserRole` and their command handlers: server-side endpoints were never implemented and there is no plan to add them at current scale.

## Changes Required

### politburo/

**New dependency additions (`go.mod`)**
- `github.com/ThreeDotsLabs/watermill v1.4.x`
- `github.com/ThreeDotsLabs/watermill-redisstream v1.4.x`
- `github.com/go-playground/validator/v10 v10.x`
- `github.com/alicebob/miniredis/v2` (test-only)

---

#### Stream 1 — Watermill Infrastructure

`infra/messaging/` — NEW package (no business logic)

| File | Purpose | Key exported API |
|---|---|---|
| `messaging.go` | Package constants | `DefaultBlockTime = 5*time.Second`, `DefaultMaxIdle = 5*time.Minute` |
| `logger.go` | `func NewZapLogger() watermill.LoggerAdapter` | Wraps `infra/logging` |
| `publisher.go` | `func NewPublisher(client *goredis.Client) (message.Publisher, error)` | Thin `redisstream.NewPublisher` wrapper |
| `subscriber.go` | `func NewSubscriber(client *goredis.Client, group, name string) (message.Subscriber, error)` | One subscriber per consumer group |
| `router.go` | `func NewRouter(reg *metrics.MetricsRegistry) (*message.Router, error)` | Default middleware stack wired in |
| `middleware.go` | `func MetricsMiddleware(reg *metrics.MetricsRegistry, queueType string) message.HandlerMiddleware` | Feeds existing Prometheus label sets |
| `topics.go` | Topic name constants | `const TopicPirepSync = "pirep.sync.v2"` |
| `poisonqueue.go` | `func PoisonQueue(pub message.Publisher, topic string) (message.HandlerMiddleware, error)` | Routes terminal failures to `<topic>.poison` |

Default router middleware stack (applied to all handlers): `Recoverer → CorrelationID → Retry(MaxRetries:3, InitialInterval:5s, MaxInterval:1m, Multiplier:2) → MetricsMiddleware → PoisonQueue`

`infra/metrics/metrics.go` — MODIFY
- Add `WatermillHandlerDuration prometheus.HistogramVec` (labels: `topic`, `handler`)
- Add `WatermillHandlerErrors prometheus.CounterVec` (labels: `topic`, `handler`, `error_type`)
- Add `WatermillPoisonTotal prometheus.CounterVec` (labels: `topic`)

`internal/app/app.go` — MODIFY
- Add to `InfraDeps` (~line 57):
  ```go
  WatermillPublisher  message.Publisher
  WatermillRouter     *message.Router
  WatermillSubFactory func(group, name string) (message.Subscriber, error)
  ```
- In `initInfra`: after Redis client init, build publisher, router, and sub factory closure.
- In `Shutdown`: close router before Redis client (`a.Infra.WatermillRouter.Close()`).

`cmd/server/main.go` — MODIFY
- After `routes.RegisterWorkers(application)`, start router in goroutine:
  ```go
  go func() {
      if err := application.Infra.WatermillRouter.Run(ctx); err != nil {
          logging.Error("watermill router stopped", "error", err)
      }
  }()
  ```

---

#### Stream 1 — PIREP Worker Migration

`internal/pireps/upsert.go` — NEW
- Package `pireps`
- Move `upsertPirep` (queue_worker.go:168–380) verbatim. Public signature:
  ```go
  func UpsertPirepFromAirtable(ctx context.Context, db *gorm.DB, repo *Repository, vaRepo *platformVA.Repository, vaID, airtableRecordID string, fields map[string]interface{}, createdTime string) error
  ```

`internal/pireps/messaging.go` — NEW
- Package `pireps`
- `type MessagingHandler struct { db *gorm.DB; vaRepo *platformVA.Repository; pirepRepo *Repository }`
- `func (h *MessagingHandler) HandlePirepSync(msg *message.Message) error` — unmarshal `queue.PirepQueueItem`, call `UpsertPirepFromAirtable`, return nil (ACK) or error (NACK → retry/poison).
- `func RegisterPirepHandlers(router *message.Router, sub message.Subscriber, h *MessagingHandler)`
- `func NewMessagingHandler(db, vaRepo, pirepRepo) *MessagingHandler`

`internal/pireps/sync_job.go` — MODIFY (dual-write phase)
- After existing `EnqueuePirep(item)` call, also publish to new topic:
  ```go
  payload, _ := json.Marshal(item)
  _ = publisher.Publish(messaging.TopicPirepSync, message.NewMessage(watermill.NewUUID(), payload))
  ```
- `PirepSyncJob` struct gains `publisher message.Publisher` field.

`internal/app/app.go` — MODIFY (`initFeatures`)
- Remove: `pirepQueueWorker = pireps.NewQueueWorker(...)` block (~line 401–411).
- Add:
  ```go
  pirepSub, err := a.Infra.WatermillSubFactory("pirep-workers", "consumer-"+hostname)
  pirepMsgHandler := pireps.NewMessagingHandler(a.Infra.DB, a.Platform.VARepo, pirepRepo)
  pireps.RegisterPirepHandlers(a.Infra.WatermillRouter, pirepSub, pirepMsgHandler)
  ```
- Remove `PirepQueueWorker` field from `FeatureDeps`.

`internal/routes/jobs.go` — MODIFY (post-cutover)
- Remove the PIREP worker goroutine block.

`internal/pireps/queue_worker.go` — DELETE (post-cutover, after dual-write bake)

`infra/queue/redis_queue.go` — MODIFY (post-cutover)
- Delete `EnqueuePirep`, `EnqueuePirepBatch`, `DequeuePirep`, `AckPirep`, `ClaimStalePireps`, and PIREP-specific `CreateConsumerGroup`/`GetQueueLength`/`GetPendingCount`/`TrimStream` overloads.
- Keep `FlightPlan*` and `Pilot*` helpers until those workers are migrated in a follow-up plan.

---

#### Stream 2 — Validation Package

`internal/platform/validation/` — NEW package

| File | Purpose |
|---|---|
| `validator.go` | `go-playground/validator/v10` singleton init |
| `decoder.go` | `func DecodeAndValidate(r *http.Request, dst any) *ValidationError` — JSON decode + struct tag validate |
| `errors.go` | `type ValidationError struct { Fields []FieldError; StatusCode int }` |

`internal/platform/httpdto/response.go` — MODIFY
- Add `func WriteValidationError(w http.ResponseWriter, t time.Time, ve *validation.ValidationError)` — emits 422 with `error.code = "VALIDATION_FAILED"` and field-level details.

---

#### Stream 2 — Endpoint Fixes

`internal/auth/handler.go` — MODIFY
- Add `func (h *Handler) VerifyGodMode() http.HandlerFunc` — returns `{"is_god": bool}` based on god-mode claim check.

`internal/pilots/handler.go` — MODIFY
- Add `func (h *Handler) GetUserLogbookSelf() http.HandlerFunc` — self-ownership check: if `ifc_id` param ≠ `claims.DiscordID`'s linked IFC ID, require staff claim; otherwise return logbook. Delegates to existing service.

`internal/pireps/handler.go` — MODIFY
- Wire `pireps.Handler.GetConfig()` (already implemented at handler.go:61) — migrate off `common.VAConfigService` → `internal/platform/va.ConfigService`.
- Wire `pireps.Handler.Submit()` (handler.go:185) as the default submit endpoint; tour mode delegated internally by service. Migrate off `common.RespondError` → `httpdto.WriteError`.

`internal/routes/router.go` — MODIFY
- Add under `/api/v1/` authenticated group:
  - `GET /user/{ifc_id}/flights` → `application.Features.PilotsHandler.GetUserLogbookSelf()` (Registered scope)
  - `GET /admin/verify-god` → `application.Features.AuthHandler.VerifyGodMode()` (Authenticated scope)
- Replace 501 stub at router.go:153–157 → `pireps.Get("/config", application.Features.PirepHandler.GetConfig())`
- Replace `TourHandler.SubmitTourPirep` at router.go:158 → `application.Features.PirepHandler.Submit()`

`internal/app/app.go` — MODIFY
- Initialise `pireps.Handler` (the full one, distinct from `TourHandler`) in `initFeatures` and expose as `FeatureDeps.PirepHandler`.

---

#### Stream 2 — Service Layer Standardisation

`internal/pilots/errors.go` — NEW
```go
type RegistrationError struct {
    Code       string
    Message    string
    StatusCode int
    Cause      error
}
```

`internal/pilots/registration_service.go` — MODIFY
- Change `RegisterPilot` return from `(*RegisterPilotResponse, error)` → `(*RegisterPilotResponse, *RegistrationError)`.
- Map each domain error sentinel to a `RegistrationError` with `StatusCode`.

`internal/pilots/handler.go` — MODIFY
- Delete `handleRegistrationError` (lines 156–185).
- Handler call site becomes ~10 lines using `svcErr.Code/.Message/.StatusCode`.
- Wire `validation.DecodeAndValidate` on request decode.

`internal/memberships/errors.go` — MODIFY (already has domain errors)
- Add `type MembershipError struct { Code, Message string; StatusCode int; Cause error }`.

`internal/memberships/service.go` — MODIFY
- `JoinVA` returns `(*JoinVAResponse, *MembershipError)`.

`internal/memberships/handler.go` — MODIFY
- Delete `handleJoinVAError` (lines 139–205).
- Wire `validation.DecodeAndValidate`.

`internal/servers/service.go` — MODIFY
- Same pattern: introduce `ServerError`, update `InitServer` return.

`internal/servers/handler.go` — MODIFY
- Delete error-mapping block. Wire `validation.DecodeAndValidate`.

---

#### Stream 2 — Auth Middleware Cleanup

`internal/middleware/auth.go` — MODIFY
- Replace all `log.Printf` calls (lines 23–67) with `logging.Debug`. **Stop logging cookie values** — this is a security fix.
- Delete dead Bearer/JWT branch (lines 71–80).
- Extract `tryAuthFromSession(r, sessionSvc) (UserClaims, bool)` and `tryAuthFromAPIKey(r, keysRepo, claimsRepo) (UserClaims, bool)` helpers.
- Return `httpdto.WriteError(w, ..., 401)` JSON envelope instead of `http.Error(w, "...", 401)` (plain text) so the bot can parse 401s uniformly.

---

#### Stream 2 — Rate Limiting

`internal/middleware/ratelimit.go` — NEW
- `func RateLimitMiddleware(cache *cache.RedisCacheService, group string, limit int) func(http.Handler) http.Handler`
- Key: `ratelimit:{group}:{user_id}:{minute_bucket}` via `INCR`+`EXPIRE`.
- Groups and limits: `registration` = 5 req/min, `submit` = 10 req/min, `read` = 60 req/min.

`internal/routes/router.go` — MODIFY
- Apply `RateLimitMiddleware` to route groups:
  - Registration/init routes: `registration` group.
  - `/pireps/submit`, `/memberships/join`: `submit` group.
  - Read endpoints: `read` group.

---

#### Stream 2 — OpenAPI Spec

> **Spec naming convention**: one spec file per domain flow (not one monolithic bot spec).
> Shared envelope schemas (`ErrorResponse`, `ValidationErrorResponse`) are duplicated per spec until there are ≥ 3 specs that share them, at which point extract to `components.yaml`.

`api/openapi/registration.yaml` — NEW ✅ (2026-05-13)
Login/registration flow operations:
1. `POST /api/v1/pilots/register` — `operationId: registerPilot`
2. `POST /api/v1/server/init` — `operationId: initServer`
3. `POST /api/v1/memberships/join` — `operationId: joinMembership`
4. `GET /api/v1/user/status` — `operationId: getUserStatus`
5. `POST /api/v1/signed-link` — `operationId: generateSignedLink`

`api/openapi/registration.cfg.yaml` — NEW ✅ (2026-05-13) — oapi-codegen config: `package: registrationgen`, `generate: [chi-server, strict-server, models]`, output `internal/api/generated/registration/server.gen.go`.

`internal/api/generated/registration/server.gen.go` — NEW (generated, committed) — run `make generate-api` after any spec change.

`internal/api/registration/server.go` — NEW (pending)
- Implements `registrationgen.StrictServerInterface` by delegating to existing handlers.
- Mounted in router under the authenticated v1 group.

Remaining spec files to create in follow-up work:
- `api/openapi/logbook.yaml` — `getPilotLogbook`, `getUserLogbook` (self-ownership alias)
- `api/openapi/pireps.yaml` — `getPirepConfig`, `submitPirep`
- `api/openapi/admin.yaml` — `verifyGodMode`

`Makefile` — MODIFIED ✅ (2026-05-13)
```makefile
generate-api:
	cd api/openapi && oapi-codegen -config registration.cfg.yaml registration.yaml
```

---

### comrade-bot/

`src/services/apiService.ts` — MODIFY: delete dead/broken methods
- `linkUserToVA` — dead code (bot uses `joinMembership` instead; see registerModalHandler.ts:152)
- `generateDashboardLink` — replaced by `generateSignedLink`; dead caller
- `syncUserToVA` — server-side endpoint never implemented; deleted
- `assignUserRole` — server-side endpoint never implemented; deleted
- `getUserLogbook` — wrong path (`/api/v1/user/{ifcId}/flights` vs actual `/api/v1/pilots/{ifc_id}/logbook`); replaced by server-side alias (D3); update path here once alias is live

`src/commands/SyncUserHandler.ts` — DELETE (only calls `syncUserToVA`)

`src/commands/ConfigurePilotRoleHandler.ts` — DELETE (only calls `assignUserRole`)

`src/types/Responses.ts` — MODIFY (Phase G): incrementally replace hand-maintained types with generated types from openapi-typescript as each apiService method is migrated.

`package.json` — MODIFY (Phase G): add devDeps `openapi-typescript`, `openapi-fetch`. Add script `"generate:api": "openapi-typescript ../politburo/api/openapi/bot.v1.yaml -o src/types/generated.ts"`.

### labour-bureau/

No changes required. Redis (with `--appendonly`) already runs. The watermill-redisstream subscriber uses XREADGROUP + XACK — no new infra needed. No new containers.

## Developer Guidelines

### API Response Conventions

All new and modified JSON API handlers use `internal/platform/httpdto/response.go`:
- `RespondSuccess(w, data)` for 2xx responses.
- `RespondError(w, status, code, message)` for domain errors.
- `WriteValidationError(w, t, ve)` for 422 validation failures.
- **Never** use `internal/common/api_response.go` (dead package).

Status codes per endpoint:

| Endpoint | 200 | 201 | 400 | 401 | 403 | 404 | 409 | 422 | 500 |
|---|---|---|---|---|---|---|---|---|---|
| `POST /pilots/register` | — | success | bad JSON | no auth | — | IFC user not found | already registered | validation | upstream |
| `POST /server/init` | — | success | bad JSON | no auth | — | — | server exists | validation | db error |
| `POST /memberships/join` | — | success | bad JSON | no auth | — | VA not found | already member | validation | db error |
| `GET /user/status` | found | — | — | no auth | — | not found | — | — | db error |
| `POST /signed-link` | success | — | bad JSON | no auth | — | — | — | validation | — |
| `GET /user/{ifc_id}/flights` | found | — | — | no auth | wrong user (non-staff) | not found | — | — | db error |
| `GET /admin/verify-god` | `{is_god:bool}` | — | — | no auth | — | — | — | — | — |
| `GET /pireps/config` | found | — | — | no auth | — | VA not found | — | — | db error |
| `POST /pireps/submit` | — | success | bad JSON | no auth | not member | — | — | validation | upstream |

### Auth Scopes

Inherits from parent groups in `internal/routes/router.go`:

| Route | Scope | Middleware chain |
|---|---|---|
| `POST /api/v1/pilots/register` | Authenticated | `AuthMiddleware` (v1 group) |
| `POST /api/v1/server/init` | Authenticated | `AuthMiddleware` |
| `POST /api/v1/memberships/join` | Registered | `AuthMiddleware` + `IsRegisteredMiddleware()` |
| `GET /api/v1/user/status` | Authenticated | `AuthMiddleware` |
| `POST /api/v1/signed-link` | Authenticated | `AuthMiddleware` |
| `GET /api/v1/user/{ifc_id}/flights` | Registered | `AuthMiddleware` + `IsRegisteredMiddleware()` + self-check in handler |
| `GET /api/v1/admin/verify-god` | Authenticated | `AuthMiddleware` |
| `GET /api/v1/pireps/config` | Member | `AuthMiddleware` + `IsMemberMiddleware()` |
| `POST /api/v1/pireps/submit` | Member | `AuthMiddleware` + `IsMemberMiddleware()` |

### Claims & Context

| Handler | Calls `GetUserClaims`? | Fields read |
|---|---|---|
| `RegisterPilot` | Yes | `DiscordID`, `ServerID` |
| `InitServer` | Yes | `ServerID` |
| `JoinVA` | Yes | `DiscordID`, `ServerID` |
| `GetUserStatus` | Yes | `DiscordID`, `ServerID` |
| `GenerateSignedLink` | Yes | `DiscordID` |
| `GetUserLogbookSelf` | Yes | `DiscordID` (for self-ownership check) |
| `VerifyGodMode` | Yes | `IsGod` (or equivalent god-mode claim field) |
| `GetConfig` | Yes | `ServerID` |
| `Submit` (pireps) | Yes | `DiscordID`, `ServerID` |

### Database Migrations

No migrations required. This plan introduces no new tables or columns.

### Error Handling Contract

- Watermill handler returns `nil` → ACK. Returns any non-nil error → NACK → retry middleware retries up to 3 times with exponential backoff → poison queue on terminal failure.
- Poison queue messages land on `pirep.sync.v2.poison` stream; they do not block the main consumer group.
- Auth middleware 401: JSON envelope (`httpdto.WriteError`) — not plain text.
- Rate limit exceeded: `429 Too Many Requests` with `Retry-After` header (seconds until next minute bucket).
- Validation errors: `422` with `error.code = "VALIDATION_FAILED"` and `error.fields` array.
- Upstream Airtable failures in `UpsertPirepFromAirtable`: return error → NACK → retry; caller does not swallow.
- No `log.Printf` on error paths in modified files — always `logging.Error("...", "error", err, ...)`.

## Constants & Configuration

All in `infra/messaging/topics.go`:
```go
const TopicPirepSync    = "pirep.sync.v2"
const TopicPirepPoison  = "pirep.sync.v2.poison"
```

Rate limit key pattern (in `internal/middleware/ratelimit.go`):
```
ratelimit:{group}:{user_id}:{unix_minute}
```

No new env vars. No new docker-compose entries.

## Logging & Monitoring

### Logging

| File | What gets logged | Level | Fields |
|---|---|---|---|
| `infra/messaging/logger.go` | All watermill internal events | routed to Zap at matching level | `topic`, `handler`, `message_uuid` |
| `internal/pireps/messaging.go` | Unmarshal failure, upsert success, permanent failure | Error / Info | `va_id`, `record_id`, `attempt`, `error` |
| `internal/middleware/auth.go` | Auth failure reason | Debug | `method`, `path` — **no cookie values** |
| `internal/middleware/ratelimit.go` | Rate limit hit | Warn | `group`, `user_id`, `limit` |
| `internal/pilots/registration_service.go` | IFC lookup failure, DB error | Error | `discord_id`, `ifc_id`, `error` |
| `internal/memberships/service.go` | Join failure | Error | `discord_id`, `va_id`, `error` |

### Prometheus Metrics

New metrics in `infra/metrics/metrics.go` (`MetricsRegistry`):

| Name | Type | Labels | Incremented in |
|---|---|---|---|
| `politburo_watermill_handler_duration_seconds` | Histogram | `topic`, `handler` | `infra/messaging/middleware.go` |
| `politburo_watermill_handler_errors_total` | Counter | `topic`, `handler`, `error_type` | `infra/messaging/middleware.go` |
| `politburo_watermill_poison_total` | Counter | `topic` | `infra/messaging/poisonqueue.go` |
| `politburo_ratelimit_rejected_total` | Counter | `group` | `internal/middleware/ratelimit.go` |

Existing `QueueDequeuedTotal`, `QueueAcknowledgedTotal`, `QueueProcessingDuration`, `QueueErrorsTotal` fed by `MetricsMiddleware` with label `queue_type="pirep"` — existing Grafana dashboards unchanged.

## API Spec (spec-driven endpoints only)

Spec file: `api/openapi/bot.v1.yaml` — new file, covers the bot-facing surface only (not dashboard UI, not admin Airtable endpoints).

Phase 1 operations: see Changes Required → `api/openapi/bot.v1.yaml`.

Codegen config: `api/openapi/generate.yaml` — package `v1bot`, output `internal/api/v1bot/generated.go`, generate `chi-server` + `strict-server` + `types`.

`make generate-api` target added to `Makefile`.

Runtime dep: `github.com/oapi-codegen/oapi-codegen/v2` (tool dep, not a runtime import). Add to `tools.go` with `//go:build tools`.

The OpenAPI spec is the documentation. No Swaggo annotations are added to any file.

## Documentation

`politburo/CLAUDE.md` — MODIFY after cutover:
- Update "Background Workers" table: remove PIREP queue worker row.
- Add "Infrastructure Layer" row: `infra/messaging` — watermill router, publisher, subscriber wrappers.
- Add note to "Key Patterns": watermill handler pattern alongside existing handler pattern.
- Update "Known Technical Debt": mark `internal/services/registration_service_v2_test.go` as deleted.
- Add `api/openapi/` and `internal/api/v1bot/` to architecture overview.

`.env.example` — no new vars required.

## Testing Plan

### Unit Tests

| File | What to test | Mock strategy |
|---|---|---|
| `internal/pireps/upsert_test.go` | Field extraction from Airtable payloads, DB upsert happy path, missing-field handling | `gorm.io/driver/sqlite` in-memory |
| `internal/pireps/messaging_test.go` | `HandlePirepSync` ACK on success, NACK on upsert error, NACK on unmarshal error | Hand-written fakes for `*Repository`, `*platformVA.Repository` |
| `internal/pilots/registration_service_test.go` | Each `RegistrationError` branch: IFC not found, already registered, flight mismatch | Fake `LiveAPIProvider` interface (already defined at registration_service.go:17) |
| `internal/memberships/service_test.go` | `JoinVA` happy path + each `MembershipError` branch | Fake repo interfaces |
| `internal/servers/service_test.go` | `InitServer` happy path + conflict error | Fake repo interfaces |
| `internal/platform/validation/decoder_test.go` | Bad JSON → 400, missing required field → 422, valid payload → nil error | No mocks needed |
| `internal/middleware/ratelimit_test.go` | Under limit passes, over limit returns 429, cross-minute bucket resets | `miniredis` |

### Integration Tests

| Scenario | Entry point | Expected outcome |
|---|---|---|
| Watermill pub → consume round-trip | `infra/messaging/router_test.go` publish to `TopicPirepSync` | Handler invoked, message ACKed; verified via channel or WaitGroup |
| Poison queue routing | Same test, handler returns permanent error | Message lands on `pirep.sync.v2.poison` after 3 retries |
| Auth middleware API key path | `internal/middleware/auth_test.go` httptest with valid `X-API-Key` | Claims populated in context |

All integration tests use `miniredis` — no real Redis instance required.

### Manual Verification (ordered)

1. `go build ./...` — passes with no errors after watermill deps added.
2. Boot server locally (`air`). Confirm watermill router starts (log line: `watermill router started`).
3. Trigger PIREP sync job manually (or wait for cron). Confirm message appears on `pirep.sync.v2` stream: `redis-cli XLEN pirep.sync.v2`.
4. Confirm new handler processes message: check `politburo_watermill_handler_duration_seconds` in `/metrics`.
5. With dual-write active, verify old `pirep:sync:*` streams also receive messages and old worker also processes them. Compare `QueueDequeuedTotal{queue_type="pirep"}` vs `WatermillHandlerDuration{topic="pirep.sync.v2"}` — counts should match.
6. After cutover: confirm `pirep:sync:*` streams no longer grow. Confirm only new topic processed.
7. Test bot flow end-to-end: registration → `POST /api/v1/pilots/register` → confirm 201 with correct envelope; confirm 422 on missing `ifc_id`.
8. Test logbook alias: bot calls `GET /api/v1/user/{ifc_id}/flights` with own IFC ID — confirm 200. With different IFC ID and non-staff role — confirm 403.
9. Confirm deleted bot commands (`/syncrole`, `/configurerole` or equivalent) are no longer registered in Discord slash command list.
10. Auth middleware: confirm 401 response is JSON (not plain text) — parse it in bot apiService and assert structured error.

## Out of Scope

- Migration of `internal/flights/flight_plan_worker.go` and `internal/pilots/sync_worker.go` to watermill — follow-up plan after PIREP proves the pattern.
- Deleting `internal/common/` and `internal/db/repositories/` legacy packages — touched only where the PIREP handler fix forces it.
- Splitting large files flagged in CLAUDE.md (`events/handler.go`, `pilots/stats_service.go`, `flights/service.go`, `vaadmin/handler.go`, `pireps/service.go`) — separate plan.
- Internal event bus (`internal/platform/events/`) for push-based cache invalidation — follow-up to this plan.
- Dashboard UI / HTMX endpoint changes.
- OpenAPI Phase 2 (PIREP endpoints) and Phase 3 (events endpoints) — gated on `internal/common/` migration in pireps.
- New Discord bot slash commands or UI changes beyond apiService hygiene.
- Database schema changes.
- GORM callback for DB metrics (deferred — no dashboard need yet).
- `cmd/vizburo/main.go` changes (already fixed in dead-code-observability-cleanup plan).

---

## Work Log

### 2026-05-13

**Stream 2 — OpenAPI Spec (partial)**

- Decided against a single monolithic `bot.v1.yaml`; adopted one-spec-per-domain-flow convention.
- Created `api/openapi/registration.yaml` covering the 5 login/registration endpoints:
  `POST /pilots/register`, `POST /server/init`, `POST /memberships/join`, `GET /user/status`, `POST /signed-link`.
  Schemas derived from live handler/DTO code; response envelope (`status`, `result`, `responseTimeMs`) modelled in full so generated types are wire-compatible with existing `httpdto.WriteSuccess/WriteError` output.
- Created `api/openapi/registration.cfg.yaml` (package `registrationgen`, strict-server + chi-server + models, output `internal/api/generated/registration/server.gen.go`).
- Added `make generate-api` target to `Makefile`.
- Updated Changes Required section to reflect actual file names and split-spec approach.
- Remaining spec files (`logbook.yaml`, `pireps.yaml`, `admin.yaml`) and the `StrictServerInterface` implementation (`internal/api/registration/server.go`) are pending.
