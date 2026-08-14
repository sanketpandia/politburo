# Architecture overview

Politburo is a single Go application. `cmd/politburo/main.go` loads configuration,
constructs the application, and runs one HTTP server. Infrastructure is composed
in `internal/app`; HTTP transport, scheduled jobs, generated API code, and UI
assets remain separate packages.

Startup order:

1. Load and validate environment configuration.
2. Initialize structured logging and the application metrics registry.
3. Open and ping PostgreSQL.
4. Validate embedded UI templates.
5. Construct the scheduler and register jobs centrally.
6. Bind the HTTP listener.
7. Start scheduled jobs when `JOBS_ENABLED=true`.
8. Serve until SIGINT or SIGTERM, then shut down gracefully.

The rewrite uses a separate database (`politburo_next` by default) and disables
jobs by default so it cannot mutate legacy application state accidentally.

