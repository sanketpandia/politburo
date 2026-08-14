# Politburo

Clean-slate Politburo game server. The previous application is preserved in the
sibling `politburo-legacy` directory for reference.

The first milestone provides one Go binary, PostgreSQL wiring, graceful
shutdown, central job registration, Prometheus metrics, OpenAPI generation,
embedded UI primitives, and liveness/readiness endpoints. It intentionally has
no game endpoints or feature UI.

## Commands

```sh
make generate
make test
make build
go tool air -c .air.toml
```

The development server defaults to `http://localhost:8082`.

