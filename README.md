# Politburo

Clean-slate Politburo game server. The previous application is preserved in the
sibling `politburo-legacy` directory for reference.

The rewrite is a single Go binary that serves:

- public ops (`/health/live`, `/health/ready`, `/metrics`)
- machine JSON API under `/api/v1/...` (OpenAPI contract)
- browser UI stubs under `/dashboard` and `/static` (not OpenAPI)

The rewrite caches Infinite Flight sessions, liveries, and active flights via
scheduled jobs and exposes them at `GET /api/v1/game/sessions/active` and
`GET /api/v1/game/flights/active`. In active development.

## Local development

Primary entry is the labour-bureau launcher (sibling checkout required):

```sh
cd ../labour-bureau && ./start-dev.sh
```

That starts Compose backing services, legacy Politburo on `:8080`, and this
rewrite on `:8082` via Air. Air loads `politburo/.env` (see `.env.example` for
`JOBS_ENABLED`, `IF_API_KEY`, and optional `PG_*` overrides).

## Commands

```sh
cp .env.example .env   # then set IF_API_KEY when enabling jobs
make generate
make openapi-view
make test
make build
go tool -modfile=tools/go.mod air -c .air.toml
```

Defaults: `http://localhost:8082`. Swagger UI: `http://localhost:8081` after
`make openapi-view` (or via `start-dev.sh` Compose).
