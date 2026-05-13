# Observability Cleanup — 2026-05-13

## Phase 2: Structured Logging

### convert stdlib log calls to structured Zap logging (`d4d285d`)

**Changed**
- `internal/pilots/stats_service.go` — replaced all `log.Printf`/`fmt.Printf` with `logging.Debug/Info/Warn/Error`; removed `"log"` import; verbose debug-dump log lines (raw JSON, schema fields) removed entirely since they were only useful during development
- `internal/pilots/linking_job.go` — replaced all `log.Printf` with structured Zap calls; removed `"log"` import

**Reused**
- `infra/logging` package functions — same pattern already established in `sync_job.go`, `repository.go`, `pireps/sync_job.go`, etc.

**Metrics/Logging added**
- No new metrics. Logging converted at appropriate levels: Debug for internal steps (cache checks, formula building), Warn for optional-data failures (game stats, provider data), Error only when returning an error to caller.

**Test surface**
- Functions introduced: none (conversion only)
- Behaviour to integration-test: none changed

**Live API compliance**
- Not applicable (no Live API polling changes)

**Build status**
`go build ./...` passed (vizburo excluded per plan)

**Notes**
- Several verbose `fmt.Printf` log blocks in `stats_service.go` that dumped raw Airtable JSON and schema field lists were removed entirely rather than converted — they were development-only debug output that would be noise in production JSON logs and added no structured value.
- `fetchRouteFromAirtablePIREP` silent-returns empty string on config/parse errors rather than logging — consistent with the existing pattern in sibling functions that treat Airtable fallback as best-effort.

---

## Phase 3: Prometheus Metrics Wiring

### wire Prometheus metrics into jobs and workers (`a6798d9`)

**Changed**
- `internal/pireps/queue_worker.go` — added `*metrics.MetricsRegistry` field; `NewQueueWorker` takes `metricsReg`; `processQueue` increments `QueueDequeuedTotal`, `QueueProcessingDuration`, `QueueErrorsTotal` (label: `transient`), `QueueAcknowledgedTotal`
- `internal/pireps/sync_job.go` — added `*metrics.MetricsRegistry` field; `NewPirepSyncJob` takes `metricsReg`; `Run` defers `SyncJobDuration`; enqueue path adds `QueueEnqueuedTotal` + `SyncJobRecordsProcessed`
- `internal/sessions/cache_job.go` — added `*metrics.MetricsRegistry`; `Run` defers `SyncJobDuration`, sets `CacheSize` gauge after refresh
- `internal/platform/aircraft/cache_job.go` — added `*metrics.MetricsRegistry`; `Run` defers `SyncJobDuration`, adds `SyncJobRecordsProcessed`
- `internal/platform/aircraft/worker.go` — added `*metrics.MetricsRegistry`; `syncAircraftLiveriesTask` defers `SyncJobDuration`, adds `SyncJobRecordsProcessed`
- `internal/webhooks/live_flights_webhook_job.go` — added `*metrics.MetricsRegistry`; `Run` defers `SyncJobDuration`; per-delivery `WebhooksDeliveredTotal.WithLabelValues("discord", status).Inc()`
- `infra/metrics/metrics.go` — added `WebhooksDeliveredTotal CounterVec`; removed dead fields `DBQueriesTotal`, `DBQueryDuration`, `DBConnections`, `FlightsProcessedTotal`, `UsersActive` (nothing increments them)
- `internal/routes/jobs.go` — updated `NewCacheJob` calls (sessions, aircraft) to pass `MetricsReg`; updated `NewWorker` (aircraft) to pass `MetricsReg`
- `internal/app/app.go` — updated `NewPirepSyncJob`, `NewQueueWorker`, `NewLiveFlightsWebhookJob` to pass `a.Infra.MetricsReg`

**Reused**
- `infra/metrics.MetricsRegistry` fields — all existing metric vecs, same `WithLabelValues` pattern as `pilots/sync_job.go` and `flights/flight_plan_worker.go`

**Metrics/Logging added**
- `politburo_sync_job_duration_seconds{job_name=pirep_sync_job,provider=airtable,entity_type=pirep}` — histogram
- `politburo_sync_job_duration_seconds{job_name=session_cache_job,provider=liveapi,entity_type=session}` — histogram
- `politburo_sync_job_duration_seconds{job_name=aircraft_cache_job,provider=liveapi,entity_type=aircraft}` — histogram (both cache_job and worker)
- `politburo_sync_job_duration_seconds{job_name=live_flights_webhook_job,provider=liveapi,entity_type=flight}` — histogram
- `politburo_sync_job_records_processed_total{job=pirep_sync_job,...,status=enqueued}` — counter
- `politburo_sync_job_records_processed_total{job=aircraft_cache_job,...,status=success}` — counter
- `politburo_queue_enqueued_total{queue_name=pirep_queue,queue_type=pirep}` — counter
- `politburo_queue_dequeued_total{queue_name=pirep_queue,queue_type=pirep}` — counter
- `politburo_queue_processing_duration_seconds{queue_name=pirep_queue,queue_type=pirep}` — histogram
- `politburo_queue_errors_total{queue_name=pirep_queue,queue_type=pirep,error_type=transient}` — counter
- `politburo_queue_acknowledged_total{queue_name=pirep_queue,queue_type=pirep}` — counter
- `politburo_cache_size_bytes{cache_name=session_cache}` — gauge (session count, not bytes — note label name is generic from existing metric)
- `politburo_webhooks_delivered_total{webhook_target=discord,status=success|error}` — NEW counter

**Test surface**
- No new testable functions; behaviour changes are additive (metric increment on existing code paths)

**Live API compliance**
- Not applicable

**Build status**
`go build ./...` passed

**Notes**
- `FlightsProcessedTotal` was dropped (not wired) rather than wired per-flight — per-flight increment would fire ~thousands of times per minute (all non-VA flights are skipped, VA flights vary) without meaningful dashboard use. No dashboard or alert currently consumes it.
- `UsersActive` was dropped rather than wired — the pilot sync job processes per-VA batches and doesn't produce a global pilot count without an extra DB query; cost not justified by a gauge with no current consumer.
- DB metrics (`DBQueriesTotal`, `DBQueryDuration`, `DBConnections`) required a GORM plugin callback to be useful; absent that, they were pure maintenance debt.
- Pre-existing test failures: `internal/pilots` (missing `testutil` package) and `internal/services` (SQLite migration syntax error). Both pre-date this work and are documented in CLAUDE.md.
