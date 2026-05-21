# Infinite Flight LiveAPI OpenAPI Client

Status: implemented baseline as of 2026-05-21; remaining items below are decision-gated follow-ups.

This document explains the external Infinite Flight Live API client structure added to Politburo. It describes the generated-client boundary, generation commands, and follow-up rules for future migrations.

## What was added

- `api/openapi/liveapi.yaml` models the upstream Infinite Flight Live API used by Politburo.
- `api/openapi/liveapi.cfg.yaml` configures `oapi-codegen` for Go client/models generation.
- `infra/liveapi/generated/client.gen.go` is generated code. Do not hand-edit it.
- `infra/liveapi.Client` remains the stable handwritten wrapper used by jobs, services, and legacy callers.

This spec is for an upstream third-party API. It is separate from Politburo's public registration API in `api/openapi/registration.yaml`.

## Generation commands

Run from `politburo/`:

```bash
make generate-liveapi-client
go test ./infra/liveapi ./internal/sessions ./internal/flights ./internal/platform/aircraft ./internal/pilots
go build -buildvcs=false -o .air_tmp/main ./cmd/server
go test ./...
```

`make generate-api` now runs both registration server generation and LiveAPI client generation. Use the focused target when only the upstream LiveAPI contract changed.

The generator is invoked through `go tool oapi-codegen`, using the module-pinned tool entry in `go.mod`. Do not rely on a globally installed `oapi-codegen` binary.

## Ownership boundary

Feature and platform packages should continue depending on `infra/liveapi.Client`, not `infra/liveapi/generated`.

The handwritten wrapper owns:

- `IF_API_BASE_URL` defaulting to `https://api.infiniteflight.com/public/v2`
- `IF_API_KEY` bearer-token injection
- HTTP timeout behavior
- context propagation when method signatures are expanded
- status and `errorCode` normalization
- conversion from generated models into existing compatibility DTOs
- sanitized logging and low-cardinality metrics through `infra/metrics.MetricsRegistry`

Generated models are allowed to mirror upstream docs closely. Internal callers should not be forced to absorb generated type churn directly.

## Runtime observability

The server runtime wires `infra/liveapi.Client` with the shared `infra/metrics.MetricsRegistry` in `internal/app/app.go`, so upstream wrapper metrics are exposed on the existing Politburo `/metrics` endpoint. No separate Prometheus scrape target is required.

Current LiveAPI wrapper metrics:

- `politburo_liveapi_requests_total`
- `politburo_liveapi_request_duration_seconds`

Both metrics use the bounded labels `provider`, `endpoint_group`, `status_class`, and `error_type`. Current endpoint groups are `sessions`, `flights`, `flight_plan`, `aircraft`, and `users`; keep any future labels low-cardinality and do not use request IDs, Discord IDs, user IDs, flight IDs, session IDs, raw paths, payload values, or free-form error strings as labels.

Wrapper completion logs use the same low-cardinality fields and do not include API keys or raw payloads. Successful calls log at debug level; failed or non-2xx calls log at warn level.

## Endpoints covered by the initial spec

The initial spec covers endpoints already used or already represented by the existing `infra/liveapi` boundary:

- sessions
- session flights
- flight route
- flight plan
- aircraft liveries
- user stats
- user flights
- user grade
- ATC
- ATIS
- world status

The first wrapper migration uses the generated client for sessions, session flights, flight route, flight plan, aircraft liveries, user stats, user flights, and user grade.

ATC, ATIS, and world status remain legacy wrapper methods for now because upstream raw docs define session/airport-scoped paths that do not match the existing no-argument wrapper methods. Do not invent compatibility behavior without a follow-up plan.

## Auth and secrets

Use bearer auth only:

```http
Authorization: Bearer <IF_API_KEY>
```

Do not use or model query-string API-key auth. Query-string keys can leak through URLs, logs, metrics, proxies, and traces.

Never log API keys, full upstream payloads, or free-form upstream request values. Prefer endpoint group, status class, upstream error code, and bounded error type when adding logs or metrics.

## LiveAPI compliance constraints

Future LiveAPI work must preserve these constraints unless a specific policy decision changes them:

- Treat LiveAPI data as simulated-flight data only; do not present it as real-world flight data.
- Do not use LiveAPI data for AI/ML training, evaluation, or grounding.
- Keep caches operational and temporary; do not warehouse raw upstream responses in PostgreSQL.
- Respect upstream polling guidance. Existing job cadences were preserved in this baseline, but any new interactive UI must avoid polling loops and stop refresh behavior when idle.
- Keep LiveAPI-derived TTLs centralized in `infra/cache/ttl.go`; do not hardcode TTL durations at call sites.
- Complete-flight and flight-plan-derived cache data use a standardized 48-hour operational TTL.

## Validation expectations

After spec or wrapper changes, run the relevant subset from `politburo/`:

```bash
make generate-liveapi-client
go test ./infra/liveapi ./infra/metrics ./internal/sessions ./internal/flights ./internal/platform/aircraft ./internal/pilots
```

For runtime confidence, start the server or dev stack long enough to confirm scheduled jobs and workers register/start. A `401` from LiveAPI during local startup usually indicates a missing or invalid local `IF_API_KEY`; it is not by itself a worker-start failure.

## Completed follow-up work

- Added focused `infra/liveapi` `httptest` coverage for bearer auth, status handling, `429`, nonzero `errorCode`, nullable fields, UUID validation, and custom date strings.
- Added wrapper-level sanitized logs and low-cardinality request count/duration metrics through the existing `infra/metrics.MetricsRegistry`.
- Converted `infra/providers.LiveAPIProvider` into a feature-facing adapter over `infra/liveapi.Client` for registration-style services.
- Retired `internal/common.LiveAPIService` into a not-implemented compatibility stub so no new code depends on the old direct HTTP client.

## Remaining decision-gated follow-up work

- **ATC, ATIS, and world status:** no active callers were found outside `infra/liveapi.Client` and legacy `internal/common.LiveAPIService` method definitions. The generated upstream paths are session/airport-scoped, while the legacy wrapper methods are no-argument compatibility methods using `/atc`, `/atis`, and `/world/status`. Keep those methods unchanged until a follow-up plan defines the desired public wrapper signatures and response compatibility.
- **Real upstream API verification:** generated-wrapper behavior has local `httptest` coverage only. Real Infinite Flight API verification was deferred by user request; do not add CI or routine local checks that consume the external rate limit without an explicit decision and non-secret operator setup.
- **LiveAPI-derived TTLs:** complete-flight and flight-plan-derived data now use centralized 48-hour TTL constants. Future TTL changes should update `infra/cache/ttl.go` and avoid hardcoded durations at cache call sites.
- **Legacy common consumers:** `internal/common.LiveAPIService` now returns `ErrLiveAPIServiceNotImplemented`. Any runtime path that still reaches the stub should be migrated to a feature service using `infra/liveapi.Client` or a small provider adapter with explicit tests.
