# Documentation

This directory documents the rewritten Politburo application. Documentation for
the previous implementation is kept separately during migration.

## Rewrite status (snapshot)

| Area | State |
|---|---|
| Binary | Single `cmd/politburo` — API, jobs, SSR UI (`/dashboard`, `/static`) |
| Contract | `api/openapi/politburo.yaml` drives Go generation and Swagger UI |
| Database | `politburo_next` by default; baseline in `migrations/000_infinite_schema.sql` |
| Cache-backed API | Active sessions and live flights exposed under `/api/v1/game/...` |
| Auth | API keys on `/api/v1`; Redis-backed browser sessions for UI |
| Comrade Bot | Host `npm run dev` via `start-dev.sh`; set `API_URL` to `:8082` for rewrite |
| Local infra | `start-dev.sh`: Compose backing services + host Politburo + host comrade-bot |

- `architecture/overview.md`: application boundaries, HTTP surfaces, startup.
- `conventions.md`: cache responses, timestamps, Redis keys, metrics policy,
  auth/service boundary, and the checklist for new cache-backed features.
- `development/getting-started.md`: `start-dev.sh`, Air `.env`, health checks.
- `development/openapi.md`: JSON-only contract ownership, Go generation,
  Swagger UI, and Comrade Bot TypeScript generation.
- `development/containers.md`: development/production images, CI, and runtime
  secret injection.
- `observability/metrics.md`: performance-oriented Prometheus metrics.
- `future/`: planned work not yet implemented (see `future/README.md`).

The authoritative machine API contract is `api/openapi/politburo.yaml`. It
drives the Go server generator, local Swagger viewer, and Comrade Bot
TypeScript types. SSR UI is not part of OpenAPI.
