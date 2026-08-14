# Getting started

Start PostgreSQL, create the isolated rewrite database, then run Politburo:

```sh
createdb -h localhost -U ieuser politburo_next
go run ./cmd/politburo
```

The rewrite defaults to port `8082`; the reference application remains on
`8080` through `labour-bureau/start-dev.sh`.

```sh
curl http://localhost:8082/health/live
curl http://localhost:8082/health/ready
curl http://localhost:8082/metrics
```

Configuration is read directly from environment variables. Set `DATABASE_URL`
or the `PG_HOST`, `PG_PORT`, `PG_USER`, `PG_PASSWORD`, and `PG_DB` variables.
Scheduled jobs are disabled unless `JOBS_ENABLED=true`.

