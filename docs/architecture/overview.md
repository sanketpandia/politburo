# Architecture overview

Politburo is a single Go application. `cmd/politburo/main.go` loads configuration,
constructs the application, and runs one HTTP server. Infrastructure is composed
in `internal/app`; HTTP transport, scheduled jobs, generated API code, and UI
assets remain separate packages.

Startup order:

1. Load and validate environment configuration.
2. Initialize structured logging and the application metrics registry.
3. Open and ping PostgreSQL.
4. Open and ping Redis.
5. Validate embedded UI templates.
6. Construct the scheduler and register jobs centrally.
7. Bind the HTTP listener.
8. Start scheduled jobs when `JOBS_ENABLED=true`.
9. Serve until SIGINT or SIGTERM, then shut down gracefully.

The rewrite uses a separate database (`politburo_next` by default). Code defaults
leave jobs disabled when no `.env` is present; local Air loads `.env` (see
`.env.example`) so jobs can run safely against the isolated database.

## HTTP surfaces

| Surface | Mount | Contract |
|---|---|---|
| Public ops | `/health/*`, `/metrics` | OpenAPI (health) + Prometheus |
| Machine API | `/api/v1/...` | OpenAPI only |
| Browser auth | `/auth/login`, `/auth/logout` | Templates/cookies — **not** OpenAPI |
| Browser UI | `/dashboard/...`, `/static/...` | Templates/forms — **not** OpenAPI |

Middleware lives under `internal/transport/http/middleware`: access log, CORS,
API-key auth against `api_keys` (Redis-cached for one minute on `/api/v1` only),
Discord context / UI session / role-gate helpers, and an unwired rate limiter.
Claims helpers are in `internal/auth`. Browser sessions are Redis JSON objects
keyed `session:{id}` with a `session_id` cookie; UI routes under `/dashboard`
require that cookie. Domain packages take primitive IDs from handlers rather
than session or claims objects.

JSON handlers live under `internal/transport/http/api/`. UI handlers live under
`internal/transport/http/ui/`.

## API contract boundary

`api/openapi/politburo.yaml` is the source of truth for machine HTTP operations
and schemas. Generated Go code is a transport boundary: handwritten handlers
implement generated interfaces, while domain packages should not depend on
generated request or response types. Generated package:
`internal/api/generated/politburo`.

SSR HTML routes are intentionally excluded from OpenAPI. If the UI needs JSON,
that JSON belongs under `/api/v1` and in the contract.

The same contract is consumed by Comrade Bot through a synchronized YAML copy
and generated TypeScript endpoint types. This keeps server and bot changes in
the same contract workflow without coupling their build systems.

## Local launcher

`labour-bureau/start-dev.sh` runs Compose backing services plus host Air for
Politburo and host `npm run dev` for comrade-bot (default Politburo `:8082`).
UI routes are served by the same Politburo process — there is no separate Vizburo
binary or container.

See `docs/development/getting-started.md` and `../labour-bureau/README.md`.
