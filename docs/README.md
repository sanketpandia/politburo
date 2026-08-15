# Documentation

This directory documents the rewritten Politburo application. Documentation for
the previous implementation remains available in `../politburo-legacy/docs`.

- `architecture/overview.md`: application boundaries, HTTP surfaces, startup.
- `conventions.md`: cache responses, timestamps, Redis keys, metrics policy,
  auth/service boundary, and the checklist for new cache-backed features.
- `development/getting-started.md`: `start-dev.sh`, Air `.env`, health checks.
- `development/openapi.md`: JSON-only contract ownership, Go generation,
  Swagger UI, and Comrade Bot TypeScript generation.
- `development/containers.md`: development/production images, CI, and runtime
  secret injection.
- `observability/metrics.md`: performance-oriented Prometheus metrics.

The authoritative machine API contract is `api/openapi/politburo.yaml`. It
drives the Go server generator, local Swagger viewer, and Comrade Bot
TypeScript types. SSR UI is not part of OpenAPI.
