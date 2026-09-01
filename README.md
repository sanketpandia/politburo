# Politburo

Clean-slate Politburo game server rewrite. The previous application is kept
separately for reference during migration.

The rewrite is a **single Go binary** (`cmd/politburo`) that serves:

- public ops (`/health/live`, `/health/ready`, `/metrics`)
- machine JSON API under `/api/v1/...` (OpenAPI contract)
- server-rendered UI under `/dashboard` and `/static` (not OpenAPI)

There is no separate Vizburo process — the dashboard and static assets are
embedded in this binary.

The rewrite caches Infinite Flight sessions, liveries, and active flights via
scheduled jobs and exposes them at `GET /api/v1/game/sessions/active` and
`GET /api/v1/game/flights/active`. In active development.

## Local development

Primary entry is the labour-bureau launcher (sibling checkout required):

```sh
cd ../labour-bureau && ./start-dev.sh
```

That starts Compose backing services and Politburo on the host via Air (default
`:8082`). First-time database setup and the port map are documented in
`../labour-bureau/README.md`.

Air loads `politburo/.env` (see `.env.example` for `JOBS_ENABLED`, `IF_API_KEY`,
and `PG_*` overrides). The rewrite uses database `politburo_next` by default.

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
