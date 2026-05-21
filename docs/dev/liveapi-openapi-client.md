# Infinite Flight LiveAPI OpenAPI Client

Status: implemented baseline as of 2026-05-21.

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
- sanitized logging and future low-cardinality metrics

Generated models are allowed to mirror upstream docs closely. Internal callers should not be forced to absorb generated type churn directly.

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
- Review long TTLs before extending cached LiveAPI-derived data.

## Validation expectations

After spec or wrapper changes, run the relevant subset from `politburo/`:

```bash
make generate-liveapi-client
```

For runtime confidence, start the server or dev stack long enough to confirm scheduled jobs and workers register/start. A `401` from LiveAPI during local startup usually indicates a missing or invalid local `IF_API_KEY`; it is not by itself a worker-start failure.

## Follow-up work

- Add `infra/liveapi` `httptest` coverage for bearer auth, status handling, `429`, nonzero `errorCode`, nullable fields, UUID validation, and custom date strings.
- Decide whether to add session/airport-aware ATC, ATIS, and world-status wrapper methods.
- Add wrapper-level metrics/log review through `infra/metrics.MetricsRegistry` if observability work is assigned.
- Migrate `infra/providers.LiveAPIProvider` and `internal/common.LiveAPIService` only after behavior-parity tests exist.
