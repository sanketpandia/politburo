# Application conventions

## Metrics policy

Prometheus metrics are **performance-oriented**: HTTP request counts/latency,
cache operations/latency/payload size, and scheduled job runs/duration/freshness.
Do not add feature-level business gauges (session counts, user totals, and
similar) unless explicitly requested. Job freshness is already visible via
`politburo_jobs_last_success_timestamp_seconds{job=...}` and cache miss rates.

## Cache-backed API responses

Endpoints whose primary result is precomputed in cache return a top-level
`data` object with these fields:

- `availableFilters`: every supported query filter, including its name, type,
  description, effective `current` value, and `default` value.
- `result`: the current cached result. Collections are encoded as `[]`, never
  `null`.
- `history`: previous cached results when supported. Collections are encoded as
  `[]` when history is disabled or not yet implemented.
- `meta`: endpoint-specific cache freshness metadata.

Cache-backed handlers do not call an upstream provider or database on a cache
miss. They return an unavailable response and leave cache population to the
owning scheduled job.

## Adding a cache-backed feature

1. Domain package under `internal/game/<feature>` (or another domain name).
2. Job under `internal/jobs/<feature>`; register once in `jobs.Register`.
3. OpenAPI path under `/api/v1/...` in `api/openapi/politburo.yaml`.
4. Handler under `internal/transport/http/api/...` implementing the generated method.
5. Cache keys only via `internal/cache/keys.go`.
6. No feature Prometheus gauges unless requested.
7. Local: configure via `politburo/.env` (see `.env.example`); Air loads it —
   `JOBS_ENABLED` + `IF_API_KEY` for sync jobs.

## Timestamps

All API timestamps, including `meta.lastCached`, use RFC 3339 / ISO 8601 with
an explicit timezone. Newly produced timestamps are normalized to UTC and
therefore end in `Z`; fractional seconds may be present when needed. Go code
should keep timestamps as `time.Time` until JSON encoding rather than formatting
them ad hoc.

## Redis keys

Redis key constants live in `internal/cache/keys.go`. Keys are lowercase,
colon-delimited, and begin with a bounded domain prefix such as `game:`. Callers
must use the shared constants or key builders rather than repeating string
literals.
