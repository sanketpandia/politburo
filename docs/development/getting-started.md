# Getting started

## Preferred: labour-bureau launcher

From the sibling `labour-bureau/` checkout:

```sh
./start-dev.sh
```

This starts Compose backing services (Postgres, Redis, Swagger UI, observability)
and host processes for Politburo (Air) and comrade-bot (`npm run dev`) in separate
tmux windows. See `../labour-bureau/README.md` for the full port map and
first-time database setup.

Before first run:

```sh
cp .env.example .env
```

Air loads `politburo/.env` (`env_files` in `.air.toml`). Set `IF_API_KEY` when `JOBS_ENABLED=true`. Use `PG_HOST=localhost` and `PG_DB=politburo_next` for the rewrite database — do not point a host process at Compose hostname `db`.

Apply the baseline schema once (from `labour-bureau/`):

```sh
docker compose -f docker-compose.dev.yml exec -T db \
  psql -v ON_ERROR_STOP=1 -1 -U ieuser -d politburo_next \
  < ../politburo/migrations/000_infinite_schema.sql
```

| Surface | Address |
|---|---|
| Politburo API | `http://localhost:8082/api/v1/...` |
| Dashboard | `http://localhost:8082/dashboard` |
| Health / metrics | `http://localhost:8082/health/*`, `/metrics` |
| Swagger UI | `http://localhost:8081` |

Comrade Bot runs on the host via `start-dev.sh` (`npm run dev`). Set
`API_URL=http://localhost:8082` in `comrade-bot/.env` when calling the rewrite.

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
curl -H "X-API-Key: $API_KEY" \
  'http://localhost:8082/api/v1/game/flights/active?serverId=casual&pilotState=active&userName=hantder&callSign=swiss&pageNumber=1&pageLength=50'
```

`/api/v1` routes require an active row in `api_keys` (baseline schema). Key status is cached in Redis for one minute.

Generate bindings and open Swagger:

```sh
make openapi-view
```

Configuration is read from the process environment (and Air-injected `.env`). Sensitive values can use `*_FILE` mounts; see `containers.md`. Scheduled jobs require `JOBS_ENABLED=true` and `IF_API_KEY`.

Politburo logs under `start-dev.sh` are teed to `/tmp/politburo.log` for Promtail.
