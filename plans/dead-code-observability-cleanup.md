# Politburo Cleanup & Observability Plan

> Authored: 2026-05-13 · Scope: dead code deletion, structured logging gaps, Prometheus metrics gaps

## Executive Summary

- **Phase 1 (Dead Code):** Six deletions are safe with a tiny prerequisite — `internal/api/health.go` must be relocated (still wired in `internal/routes/router.go:85`) before `internal/api/` is removed.
- **Memory correction:** `internal/workers/` is **not** fully dead. Active code in `internal/flights/service.go:349–351` writes to `workers.LogbookQueue`. However, `LogbookWorker` is never started (only `workers.InitWorkers` starts it, and nothing calls `InitWorkers`). Treat the queue sends as dead — they should be removed from `internal/flights/service.go` before the `internal/workers/` package can be deleted.
- **Memory correction:** `internal/jobs/` has a single remaining external caller — `internal/api/debug.go:17`. Deleting `internal/api/` first makes `internal/jobs/` fully orphaned.
- **Phase 2 (Logging):** Zap is already initialized in `infra/logging` and used by ~38 files. Worst offenders still on `log.Printf`: `internal/pilots/stats_service.go` (81 sites), `internal/pilots/sync_job.go` (74), `internal/pireps/sync_job.go` (27), `internal/flights/service.go` (22), `internal/pireps/queue_worker.go` (15), `internal/pireps/service.go` (15). Adopt `infra/logging` everywhere — no new logger.
- **Phase 3 (Metrics):** `MetricsRegistry` is comprehensive. Sync/queue metrics are good for `pilots` and `flights`, but PIREP queue worker (`internal/pireps/queue_worker.go`) is **not** wired to metrics at all — it does not even accept a `MetricsRegistry`. Same for `pireps/sync_job.go`, `sessions/cache_job.go`, `webhooks/live_flights_webhook_job.go`, `platform/aircraft/cache_job.go`, `platform/aircraft/worker.go`. `FlightsProcessedTotal` and `UsersActive` business counters are defined but never incremented.

---

## Phase 1: Dead Code Deletion

All paths absolute under `/home/eklavya/projects/infinite-experiment/politburo/`.

### Step 1.0 — Verification snapshot

Confirmed by grep:

| Item | External references | Notes |
|---|---|---|
| `internal/api/*.go` except `health.go` | none outside `internal/api/` | only `internal/pilots/handler_test.go:14` mentions it in a comment |
| `internal/api/health.go` | `internal/routes/router.go:85` (`api.HealthCheckHandler`) | LIVE — must be relocated |
| `internal/api/debug.go` | imports `internal/jobs` | dies with parent |
| `internal/jobs/*` | only `internal/api/debug.go:17` | orphans after step 1.2 |
| `internal/workers/*` | `internal/flights/service.go:349,351` and `internal/services/flights_service.go:266,268` | queue sends — receiver never started |
| `internal/workers/InitWorkers` | none | confirmed dead |
| `internal/services/*.go.old` | none | safe |
| `structured-skipping-valley.md` | none | safe |
| `cmd/vizburo/main.go` | calls undefined `routes.RegisterRoutes` (line 38) | does not compile |

### Step 1.1 — Relocate the health handler

Goal: free `internal/api/` for deletion.

- NEW FILE: `internal/platform/health/handler.go` (package `health`)
  - Move `HealthCheckHandler` verbatim from `internal/api/health.go`.
  - Same signature: `func HealthCheckHandler(db *gorm.DB, cache *cache.RedisCacheService, upSince time.Time) http.HandlerFunc`.
- MODIFY `internal/routes/router.go`:
  - Replace import `"infinite-experiment/politburo/internal/api"` with `"infinite-experiment/politburo/internal/platform/health"`.
  - Replace `api.HealthCheckHandler(...)` on line 85 with `health.HealthCheckHandler(...)`.

**Checkpoint:** `go build ./...` — must pass.

### Step 1.2 — Fix or stub `cmd/vizburo/main.go`

Currently broken. Two options; pick one:

- (A) MODIFY `cmd/vizburo/main.go` to use `app.New(...)` + `routes.NewRouter(app)` (matches `cmd/server/main.go`).
- (B) DELETE `cmd/vizburo/main.go` entirely if the separate UI binary is not required for this pass.

Use (A) if vizburo is expected to ship; (B) if not. (A) requires reading `cmd/server/main.go` for the exact wiring shape.

**Checkpoint:** `go build ./...`.

### Step 1.3 — Remove dead `workers.LogbookQueue` sends

`internal/workers/` cannot be deleted until callers stop importing it.

- MODIFY `internal/flights/service.go`:
  - Remove import `"infinite-experiment/politburo/internal/workers"` (line 14).
  - Delete the queue-send block at lines ~349–360 (the `select { case workers.LogbookQueue <- ... }` and the surrounding `log.Printf`). The worker that drains this queue is never started; the sends are pure dead writes.
- MODIFY `internal/services/flights_service.go` (lines 266–268, import line 11): same removal. Note: `internal/services/` is marked out of scope for deletion, but removing the dead import here is necessary so the file compiles after `internal/workers/` goes away.

**Checkpoint:** `go build ./...`.

### Step 1.4 — Delete `internal/api/`

After Steps 1.1 and 1.3:

- DELETE entire directory `internal/api/`. This includes:
  - `airports_handler.go`, `data_provider_config.go`, `debug.go`, `dependencies.go`, `flight_handlers.go`, `flights.go`, `god_mode.go`, `handlers.go`, `health.go`, `jobs_handler.go`, `maps.go`, `response.go`, `server_registration_v2.go`, `user.go`, `user_registration_v2.go`, `user_registration_v2_test.go` (broken test, removed with package), `va_config_handlers.go`, `va.go`, `va_handlers.go`, `world_tour_admin_handlers.go`, `world_tour_bot_handlers.go`, `world_tour_handlers.go`.

**Checkpoint:** `go build ./...` and `go test ./... -count=1` — the `user_registration_v2_test.go` compile error is now gone.

### Step 1.5 — Delete `internal/jobs/`

Now fully unreferenced (its only external caller `internal/api/debug.go` is gone).

- DELETE entire directory `internal/jobs/`:
  - `at_sync_job.go`, `init.go`, `route_sync_job.go`, `session_cache_job.go`.

**Checkpoint:** `go build ./...`.

### Step 1.6 — Delete `internal/workers/`

- DELETE entire directory `internal/workers/`:
  - `init.go`, `logbook_worker.go`, `meta_cache_worker.go`, `pirep_data_backfill.go`, `pirep_queue_monitor.go`, `pirep_queue_worker.go`.

**Checkpoint:** `go build ./...`.

### Step 1.7 — Delete stale files

- DELETE `internal/services/registration_service.go.old`.
- DELETE `internal/services/va_management_service.go.old`.
- DELETE `structured-skipping-valley.md` (repo root).

**Checkpoint:** `go build ./...` and `go test ./... -count=1`.

### Step 1.8 — Update `CLAUDE.md`

- MODIFY `politburo/CLAUDE.md` "Known Technical Debt" section: remove the bullets for `internal/api/`, `internal/workers/`, `internal/jobs/`, `internal/services/*.go.old`, `structured-skipping-valley.md`, and `cmd/vizburo/main.go` (resolved). Leave the partially-migrated packages and large-file entries.

---

## Phase 2: Structured Logging Gaps

### Logger choice

`infra/logging/logger.go` already provides a Zap-based global logger with `Init/Info/Debug/Warn/Error/Fatal/WithRequest`. **Do not introduce slog.** Reuse this. It is already initialized in `cmd/server/main.go`.

### Per-package gaps (files still using `log.Printf` / `fmt.Println`)

| File | stdlib log sites | Already on Zap | Action |
|---|---|---|---|
| `internal/pilots/stats_service.go` | 81 | 0 | Convert wholesale; this file is also slated for split, so retrofit logging during split. Log targets: Airtable round-trip errors, cache lookup misses, derived-stats failures. |
| `internal/pilots/sync_job.go` | 74 | 0 | Convert wholesale. Fields: `va_id`, `provider`, `batch_size`, `enqueued`. |
| `internal/pilots/repository.go` | 1 | 0 | One-line conversion. |
| `internal/pilots/linking_job.go` | 16 | 0 | Convert; add `linking_pass_id` and `va_id` fields. |
| `internal/pireps/sync_job.go` | 27 | 0 | Convert; add `va_id`, `since`, `count_pulled`, `enqueued`. |
| `internal/pireps/queue_worker.go` | 15 | 0 | Convert; add `worker_id`, `va_id`, `record_id`, `attempt`. |
| `internal/pireps/service.go` | 15 | 0 | Convert; add `pirep_id`, `va_id`, validation failure reasons. |
| `internal/pireps/handler.go` | 6 | 0 | Convert; HTTP request correlation IDs. |
| `internal/flights/service.go` | 22 | 0 | Convert; add `flight_id`, `session_id`, `va_id`. (After Step 1.3 trims the dead `LogbookQueue` lines.) |
| `internal/flights/handler.go` | 2 | 9 | Convert remaining two `log.Printf` for consistency. |
| `internal/events/service.go` | 0 | 0 | **No logging at all** — add error logs on DB failures in repo wrappers, info logs on event/leg create/update/delete. |
| `internal/events/repo.go` | 0 | 0 | Add error logs on GORM errors with `event_id`, `leg_id` fields. |
| `internal/pireps/repository.go` | 0 | 0 | Add error logs on GORM errors; PIREP submission path is silent on DB failures. |
| `internal/pireps/validation_service.go` | 0 | 0 | Add info-level log on validation failures (which rule failed). |
| `internal/vaadmin/*` | 0 | 29 + 7 | Already on Zap — no action. |

### Recommended pattern

Use `logging.Info/Error` with key-value pairs (Zap SugaredLogger). For request-scoped logs, propagate `auth.GetUserClaims(ctx)` and request ID via `logging.WithRequest(...)`. No new initialization needed.

---

## Phase 3: Prometheus Metrics Gaps

### Existing instrumentation (verified)

- HTTP: `internal/middleware/metrics.go` — full coverage via Chi middleware.
- DB queries: registry defined, **never incremented** anywhere — gap.
- Cache hits/misses: `infra/cache/redis.go:99,101` and `infra/cache/inmemory.go:57,59` — covered.
- Flight plan queue: `internal/flights/flight_plan_worker.go`, `flight_plan_queue_monitor.go`, `cache_job.go` — covered (depth, enqueued, dequeued, errors, duration).
- Pilot sync: `internal/pilots/sync_job.go`, `sync_worker.go` — covered.
- Route sync: `internal/va_routes/sync_job.go` — covered.

### Gaps and proposed instrumentation

#### 3.1 PIREP queue worker — entirely uninstrumented

File: `internal/pireps/queue_worker.go`
- MODIFY struct `QueueWorker`: add `metrics *metrics.MetricsRegistry`.
- MODIFY `NewQueueWorker(...)`: accept `*metrics.MetricsRegistry`.
- MODIFY `internal/app/app.go` `FeatureDeps` PIREP queue worker construction to pass `a.Infra.MetricsReg`.
- Instrument inside `Start(ctx, numWorkers)`:
  - `QueueDequeuedTotal.WithLabelValues("pirep_queue", "pirep").Inc()` per dequeue.
  - `QueueProcessingDuration.WithLabelValues("pirep_queue", "pirep").Observe(d)` around each item.
  - `QueueErrorsTotal.WithLabelValues("pirep_queue", "pirep", errorType).Inc()` on failure (`transient`, `validation`, `airtable_4xx`, `airtable_5xx`).
  - `QueueRetriesTotal` / `QueueAcknowledgedTotal` on those paths.

#### 3.2 PIREP sync job

File: `internal/pireps/sync_job.go`
- Inject `*metrics.MetricsRegistry` like `pilots/sync_job.go` does.
- On each run, observe `SyncJobDuration.WithLabelValues("pirep_sync_job","airtable","pirep").Observe(d)`.
- On records pulled: `SyncJobRecordsProcessed.WithLabelValues("pirep_sync_job","airtable","pirep",vaID,"enqueued").Add(n)`.
- On enqueue: `QueueEnqueuedTotal.WithLabelValues("pirep_queue","pirep").Add(n)`.

#### 3.3 Sessions cache job

File: `internal/sessions/cache_job.go`
- Inject metrics registry.
- `SyncJobDuration.WithLabelValues("session_cache_job","liveapi","session").Observe(d)`.
- New labeled use of `CacheSize.WithLabelValues("session_cache").Set(...)` after each refresh.

#### 3.4 Aircraft cache job and worker

Files: `internal/platform/aircraft/cache_job.go`, `internal/platform/aircraft/worker.go`
- Inject metrics.
- `SyncJobDuration.WithLabelValues("aircraft_cache_job","liveapi","aircraft").Observe(d)`.
- `SyncJobRecordsProcessed.WithLabelValues("aircraft_cache_job","liveapi","aircraft","_","success").Add(n)`.

#### 3.5 Live flights webhook job

File: `internal/webhooks/live_flights_webhook_job.go`
- Inject metrics.
- `SyncJobDuration.WithLabelValues("live_flights_webhook_job","liveapi","flight").Observe(d)`.
- NEW counter (add to `infra/metrics/metrics.go`): `WebhooksDeliveredTotal` (CounterVec, labels: `webhook_target`, `status`). Use here on each Discord delivery attempt.

#### 3.6 Business counters currently dead

In `infra/metrics/metrics.go`:
- `FlightsProcessedTotal` — never incremented. Wire into `internal/flights/cache_job.go` per-flight loop.
- `UsersActive` — never set. Either remove or wire a `Set` in `internal/pilots/sync_job.go` or a dedicated periodic gauge.
- `DBQueriesTotal` / `DBQueryDuration` / `DBConnections` — defined but no GORM callback attached. **Recommend dropping** to avoid maintaining stubs with no dashboard.

#### 3.7 Stats service cache visibility

File: `internal/pilots/stats_service.go`
- Cache lookups for pilot stats / career mode happen without explicit metric calls beyond what `infra/cache/redis.go` records. Confirm `cache_key_pattern` labels are passed consistently. If so, no new metric needed.

### New metric to register

In `infra/metrics/metrics.go` `MetricsRegistry`:
- `WebhooksDeliveredTotal prometheus.CounterVec` — name `politburo_webhooks_delivered_total`, labels `webhook_target`, `status`. Registered in `NewMetricsRegistry()` via `promauto.NewCounterVec`.

No other new metric names required — the registry is broad enough; all other gaps are wiring, not definition.

---

## Out of Scope

- `internal/services/` — partially migrated, ~25 importers, risky. Tracked in CLAUDE.md tech-debt.
- `internal/common/` — same reason.
- `internal/db/repositories/` — domain-ownership migration is its own multi-step effort.
- `internal/services/registration_service_v2_test.go` — fix should accompany that package's migration.
- File-split refactors (`internal/events/handler.go` 1678 lines, `internal/pilots/stats_service.go` 1548 lines, `internal/flights/service.go` 954 lines, `internal/vaadmin/handler.go` 790 lines, `internal/pireps/service.go` 746 lines) — separate plan.
- GORM callback for DB metrics — defer until there is a concrete dashboard need; drop the unused fields to avoid stub maintenance.
- comrade-bot and labour-bureau repos — this pass is Politburo only.
