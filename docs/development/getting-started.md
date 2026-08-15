# Getting started

## Preferred: labour-bureau launcher

From the sibling `labour-bureau/` checkout:

```sh
./start-dev.sh
```

This starts Compose (Postgres, Redis, Swagger UI, comrade-bot, observability),
legacy Politburo on `:8080`, and the rewrite on `:8082` via Air. Before starting
the rewrite it runs `ensure-rewrite-db.sh` to create `politburo_next`.

Air loads `politburo/.env` (`env_files` in `.air.toml`). Copy the sample:

```sh
cp .env.example .env
```

`.env.example` enables jobs (`JOBS_ENABLED=true`) and expects `IF_API_KEY`. Empty
`PG_HOST` / `PG_DB` fall through to code defaults (`localhost`, `politburo_next`).
The launcher unsets a Compose-network `DATABASE_URL` so the host process does
not try to reach hostname `db`.

| Service | Address |
|---|---|
| Politburo legacy | `http://localhost:8080` |
| Politburo rewrite | `http://localhost:8082` |
| Swagger UI | `http://localhost:8081` |
| Dashboard stub | `http://localhost:8082/dashboard` |

Compose comrade-bot still targets legacy `:8080` by default. To exercise the
rewrite API from the bot on the host:

```sh
cd ../comrade-bot && npm run dev:next
```

## Manual rewrite only

With Postgres and Redis already running:

```sh
createdb -h localhost -U ieuser politburo_next   # if needed
cp .env.example .env
go run ./cmd/politburo
# or: go tool -modfile=tools/go.mod air -c .air.toml
```

```sh
curl http://localhost:8082/health/live
curl http://localhost:8082/health/ready
curl http://localhost:8082/metrics
curl -H "X-API-Key: $API_KEY" \
  'http://localhost:8082/api/v1/game/sessions/active?history=false'
```

`/api/v1` routes require an active row in `api_keys` (same table as the baseline
schema). Key status is cached in Redis for one minute.

Generate bindings and open Swagger:

```sh
make openapi-view
```

Configuration is read from the process environment (and Air-injected `.env`).
Sensitive values can use `*_FILE` mounts; see `containers.md`.
Scheduled jobs require `JOBS_ENABLED=true` and `IF_API_KEY`.

Rewrite logs under `start-dev.sh` are at `/tmp/politburo-next.log`; legacy logs
remain at `/tmp/politburo.log`.
