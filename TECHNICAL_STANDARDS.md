# Infinite Experiment Technical Standards

Status: living reference. Last reviewed: 2026-05-21.

This document describes standards for new work across the Infinite Experiment workspace. It is intentionally concise; source code wins when this file and implementation disagree.

## Workspace Boundaries

- Treat `politburo/`, `comrade-bot/`, and `labour-bureau/` as sibling projects, not one root package.
- Run commands from the project that owns the change; there is no root manifest or root CI contract.
- Use `politburo/` for the Go backend, JSON APIs, background jobs, integrations, and Vizburo server-rendered UI.
- Use `comrade-bot/` for Discord commands, interactions, notifications, and links into deeper web flows.
- Use `labour-bureau/` for local/prod runtime wiring, Docker/Podman compose, deploy scripts, env examples, and observability assets.
- Verify older architecture notes against source before relying on them.

## Product And Workflow Shape

- Keep shared truth and business rules in Politburo; Discord and Vizburo should behave like clients of the same backend workflows.
- Use Discord for quick pilot actions: registration, status, live flights, logbook lookup, simple PIREP triggers, event discovery, and signed links.
- Use Vizburo for multi-step admin work: Airtable setup, mappings, events, tours, livery mappings, pilot management, reporting, and troubleshooting.
- Preserve VA configurability; do not hard-code one VA's schema, roles, callsign policy, route policy, or event format.
- Prefer small configurable MVPs over broad automation until the VA workflow is validated.
- Do not leak Airtable schema details, Infinite Flight API mechanics, queue names, cache keys, or internal error structures into pilot-facing UX.

## Politburo Backend

- Runtime bootstrap belongs in `internal/runtime`; `NewAPIServer` owns API runtime concerns and `NewVizburoServer` owns UI runtime concerns.
- Dependencies are added through `internal/app/app.go`; avoid global service instances and ad hoc initialization in route code.
- API routes are registered in `internal/routes/router.go`; keep routing pure and dependency initialization outside the router.
- Scheduled jobs and workers are registered in `internal/routes/jobs.go`; avoid untracked goroutine startup outside runtime/job registration.
- Feature packages under `internal/<feature>/` should own their handlers, services, repositories, DTOs, and models where practical.
- Cross-cutting domain services belong under `internal/platform/`; horizontal wrappers belong under `infra/` and should not contain business logic.
- Configuration is read from environment variables in `internal/app/config.go`; host-run Politburo does not load `.env` automatically.
- Handlers and jobs should return structured errors/statuses and log sanitized context; do not panic for expected runtime failures.

## External Integrations

- Keep third-party provider boundaries under `politburo/infra/`; generated or provider-specific details should not leak into feature packages.
- Infinite Flight Live API calls should flow through `infra/liveapi.Client`. Do not import `infra/liveapi/generated` from domain, platform, Vizburo, or route code.
- Feature-facing Live API provider adapters may live in `infra/providers`, but they should delegate to `infra/liveapi.Client` rather than create a second HTTP client.
- `internal/common.LiveAPIService` is retired; do not add new behavior there.
- The upstream Infinite Flight spec is `politburo/api/openapi/liveapi.yaml` with generation config `liveapi.cfg.yaml`; generated output belongs under `politburo/infra/liveapi/generated/`.
- Use bearer auth for Infinite Flight Live API (`Authorization: Bearer <IF_API_KEY>`). Do not use query-string API keys.
- Treat Live API response data as temporary operational cache data. Do not warehouse raw Live API responses or use them for AI/ML training, evaluation, or grounding.
- Do not present Infinite Flight Live API data as real-world flight data.
- Keep Live API cache durations in named constants such as `infra/cache/ttl.go`; do not hardcode TTL values at cache call sites.
- Preserve existing job/cache cadence unless a plan explicitly changes it; future interactive Live API features must avoid polling loops and should stop refresh behavior when idle.
- When upstream docs and existing wrapper behavior disagree, keep compatibility behind `infra/liveapi.Client` and require an explicit follow-up decision before changing public wrapper method shape.

## API Contracts

- JSON API responses should use `internal/platform/httpdto` envelopes: `{status,result|error,responseTimeMs}`.
- Use `httpdto.WriteSuccess`, `httpdto.WriteError`, and `httpdto.WriteValidationError` for new JSON handlers.
- OpenAPI schemas must model the response envelope, not only raw payloads.
- OpenAPI source files live under `politburo/api/openapi/`; the current registration spec is `registration.yaml` with config `registration.cfg.yaml`.
- Never hand-edit generated OpenAPI output, including `politburo/internal/api/generated/**` and `politburo/infra/liveapi/generated/**`.
- Run OpenAPI generation from `politburo/`. `make generate-api` regenerates all configured artifacts; use focused targets such as `make generate-registration-api` or `make generate-liveapi-client` when only one contract changed.
- OpenAPI generation uses the module-pinned `go tool oapi-codegen`, not a global binary.
- Handwritten generated-server adapters belong under `politburo/internal/api/<domain>/server.go` and should delegate to feature handlers, not reimplement business logic.
- OpenAPI-covered JSON route domains should mount through generated handlers where available. Registration/onboarding routes are mounted through the generated strict Chi server under `/api/v1`; `POST /api/v1/user/register` is canonical and legacy `POST /api/v1/pilots/register` is not mounted.
- New JSON routes should align router grouping, auth middleware, operation IDs, envelope schemas, and `/api/v1` path conventions.
- Do not put Vizburo HTML/template routes in OpenAPI specs.
- Do not mix external upstream API specs with Politburo public API specs; keep third-party client specs separate from bot-facing/server-facing contracts.
- Bot-context API requests should use `X-API-Key`, `X-Discord-User-Id`, and `X-Discord-Server-Id`. Registration/onboarding endpoints reject missing Discord context with `403` after API-key auth.
- Legacy Comrade Bot helper paths may still reference `X-Discord-Id` and `X-Server-Id`; do not expand that pattern without an explicit compatibility decision.
- API handlers should read claims from request context after middleware, not reparse headers.

## Vizburo UI

- Vizburo is server-rendered UI inside Politburo, served by `cmd/vizburo` through the shared runtime and DI graph.
- UI handlers should render templates or partials and delegate domain work to services.
- Do not access infrastructure directly from templates or UI handlers when a domain/platform service exists.
- Prefer reusable partials under the active `politburo/templates/partials/` tree.
- Follow shared styling tokens and components from `static/css/design-system.css`; avoid inline styles and parallel theme systems.
- Prefer HTMX/server-rendered partial updates over adding a separate frontend state system.
- Classify mobile behavior for each UI change; admin screens may be desktop-first when that tradeoff is explicit.
- `infra/templates.Renderer` should cache parsed templates outside local development and preserve local reload behavior when `APP_ENV=local`.
- Readiness/setup pages should use active VA/session context and existing platform services; do not add direct infra calls, polling, or one-off JavaScript when HTMX partials and services are sufficient.

## Comrade Bot

- Commands live under `src/commands/` and export `data` plus `execute`.
- Command registration should flow through `src/commands/registry.ts`; `src/configs/commandMap.ts` delegates to the registry.
- Route slash commands, modals, buttons, and select menus through `src/handlers/InteractionRouter.ts`.
- Keep Discord custom IDs and repeated literals in `src/configs/constants.ts` where practical.
- Centralize Politburo HTTP calls in `src/services/apiService.ts`; command modules should not call `fetch` directly.
- Future Comrade Bot API client work should replace handwritten `fetch` wrappers with a TypeScript client generated from the same OpenAPI specs only after a generator/output/auth-header convention is defined.
- Generate API auth/meta headers through `src/helpers/utils.ts`.
- Parse Politburo response envelopes in `ApiService` and map common 401/403/404/409/422 behavior there, not in every command.
- Keep commands thin and pilot-safe; do not implement complex admin setup workflows entirely in Discord.
- `/initserver` is intentionally a minimal Discord bootstrap: collect only a VA Code / ID, then direct admins to Vizburo Basic Setup for display name and callsign matching.
- Admin-only Discord commands, including `/initserver`, should use Discord default member permissions where possible in addition to backend authorization checks.
- Use existing structured logging and metrics helpers; keep labels bounded and avoid secrets, request bodies, tokens, Discord IDs, and free-form errors as metric labels.
- `npm run build` is the meaningful typecheck/build command; `npm test` is currently a placeholder.

## Labour Bureau And Observability

- Local runtime wiring lives in `labour-bureau/docker-compose.dev.yml`; production runtime lives under `labour-bureau/prod/`.
- Local dev expects Politburo to run on the host, with backing services and observability in compose.
- Dev Prometheus scrapes Politburo at `host.docker.internal:8080/metrics` and Comrade Bot at `host.docker.internal:9091/metrics`.
- `labour-bureau/start-dev.sh` is the tmux/dev-session launcher and should keep Politburo logs available at `/tmp/politburo.log` for Promtail.
- Use `go tool air -c .air.toml` from `politburo/`; avoid stale `air.toml` references.
- Production deployment flows through `labour-bureau/prod/deploy-services.sh politburo|comrade-bot|jobhunt|all`.
- Production env examples live in `labour-bureau/prod/env/*.env.example`; Politburo DB config uses `PG_*` variables, not `DATABASE_URL`.
- Preserve current port expectations unless intentionally changing runtime shape: Politburo `8080`, Comrade Bot metrics `9091`, Grafana `3000`, Prometheus `9090`, Loki `3100`, Promtail `9080`, Postgres `5432`, Redis `6379`, pgAdmin `5050`, Swagger Editor `8081`.
- Politburo health is `/healthCheck` on port `8080`; Comrade Bot health is `/healthz` on metrics port `9091`.
- Add Politburo metrics through `infra/metrics.MetricsRegistry`; do not create a second Prometheus registry.
- Keep Prometheus and Grafana queries aligned with verified metric names and scrape jobs.
- Keep Loki labels low-cardinality: service, container, job, source, env, and level-style fields.
- Do not promote request IDs, Discord IDs, guild IDs, session IDs, raw paths, or error messages to metric or Loki labels.
- Do not introduce tracing infrastructure unless a feature explicitly adds instrumentation and runtime support.

## Active Refactoring Direction

- Prefer current domain/platform/infra layering over expanding legacy packages.
- Treat `internal/services`, `internal/common`, `internal/db/repositories`, and shared `internal/models/*` as legacy or migration-sensitive areas.
- Do not delete or bypass legacy code without checking active imports and runtime wiring.
- When touching large legacy files, split by responsibility instead of adding unrelated behavior.
- Known split candidates include `internal/events/handler.go`, `internal/pilots/stats_service.go`, `internal/flights/service.go`, `internal/vaadmin/handler.go`, and `internal/pireps/service.go`.
- PIREP sync has Redis queue and Watermill behavior side-by-side; avoid deepening legacy queue dependence unless required.
- Vizburo styling should converge on shared design-system tokens and reusable partials.
- Comrade Bot command registration has converged on `src/commands/registry.ts`; do not revive removed/stubbed command handlers unless the active registry requires them.
- Header naming is mid-migration; prefer the `X-Discord-User-Id` and `X-Discord-Server-Id` middleware constants for new bot-context API work.

## Validation Expectations

- For Politburo changes, run focused package tests first; use `go test ./...` when the blast radius justifies it.
- For registration/OpenAPI work, run the focused API regression set documented in `AGENTS.md` and regenerate with `make generate-api` after spec edits.
- For upstream LiveAPI client work, run `make generate-liveapi-client`, focused `infra/liveapi` and LiveAPI consumer package tests, a server build, and a short worker-start validation when the wrapper behavior changes.
- For Comrade Bot changes, run `npm run build` from `comrade-bot/`.
- For Vizburo CSS changes, run `npm run css:build` from `politburo/` when stylesheet output is affected.
- For infra changes, validate compose, scrape targets, dashboards, env templates, health checks, ports, volumes, and labels from `labour-bureau/`.
- User-visible, admin-facing, config, deployment, and observability behavior changes should update the relevant docs or help text in the same feature slice.

## Planning Standard

- Start broad or cross-repo implementation with a repo-grounded plan under `politburo/plans/`.
- Author OpenAPI changes before implementation when a planned feature changes JSON API contracts.
- After a feature lands, reconcile observability and documentation so runtime behavior, dashboards, env examples, and user/admin guidance match what shipped.
