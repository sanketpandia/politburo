# Comrade Bot Structured Logging, Metrics, and Grafana Dashboards — Implementation Plan

## 1. Title and status
- **Status:** Proposed companion plan
- **Plan file:** `politburo/plans/comrade-bot-observability-grafana-plan.md`
- **Date:** 2026-05-20
- **Requested change summary:** Add proper structured logging and Prometheus metrics for `comrade-bot`, wire those metrics/logs into existing `labour-bureau` Grafana/Loki/Prometheus stacks, and provision dashboards for command hits, success/failure, latency, server/guild activity, and runtime logs.
- **Scope and assumptions:**
  - This is a companion to the in-progress `politburo/plans/comrade-bot-command-init-refactor.md`; do not overwrite or block that plan.
  - Primary scope: `comrade-bot/` logging/metrics instrumentation and `labour-bureau/` dev/prod observability wiring.
  - Backend `politburo/` app logic is out of scope unless shared docs mention bot observability. Do not add Politburo metrics for bot commands.
  - Metrics should be emitted by `comrade-bot` itself via a dedicated `/metrics` endpoint, recommended port `9091`.
  - Logs should be JSON to stdout/stderr so existing Promtail/Loki pipelines can ingest container logs.

## 2. Context
- **Files/packages inspected:**
  - Workspace guidance: `AGENTS.md`, `politburo/CLAUDE.md`.
  - Existing related plan: `politburo/plans/comrade-bot-command-init-refactor.md`.
  - Observability agent findings from `observability-infra-maintainer` for current bot logging, Prometheus/Loki/Grafana wiring, and dashboard recommendations.
  - Comrade Bot app files: `comrade-bot/src/index.ts`, `src/bot/BotClient.ts`, `src/handlers/InteractionRouter.ts`, `src/helpers/commandErrorHandler.ts`, `src/services/apiService.ts`, `src/services/deploymentService.ts`, `src/deploy-commands.ts`, `package.json`.
  - Dev infra: `labour-bureau/docker-compose.dev.yml`, `labour-bureau/prometheus.dev.yml`, `labour-bureau/promtail-config.yml`, `labour-bureau/grafana/provisioning/dashboards/logs-errors.json` and dashboard directory.
  - Prod infra: `labour-bureau/prod/docker-compose.prod.yml`, `prod/prometheus.prod.yml`, `prod/promtail-config.yml`, `prod/scripts/podman-log-shipper.sh`, `prod/scripts/podman-logs-to-files.sh`, `prod/grafana/provisioning/dashboards/`.
- **Existing behavior and architecture summary:**
  - `comrade-bot` currently uses direct `console.log` / `console.warn` / `console.error` across startup, command routing, API calls, deployment, and command helpers.
  - `src/index.ts` currently logs `DISCORD_BOT_TOKEN` and `DISCORD_BOT_CLIENT_ID` at startup; this is a critical secret leak that MUST be removed before structured logging rollout.
  - `InteractionRouter.ts` is the central command/interaction dispatch point and already logs command start as plain text. This is the correct instrumentation boundary for command metrics and command lifecycle logs.
  - `CommandErrorHandler.logExecution()` logs user/guild/params as plain text; params may contain sensitive command input and should not be logged raw.
  - No `prom-client`, metrics registry, or bot `/metrics` HTTP endpoint was observed.
  - Dev Prometheus currently only scrapes Politburo at `host.docker.internal:8080/metrics`.
  - Prod Prometheus currently scrapes Politburo, Caddy, and node-exporter; it does not scrape `comrade-bot`.
  - Dev `comrade-bot` runs with `network_mode: "host"` and labels `service=comrade-bot`, `env=dev`; no metrics port is configured.
  - Prod `comrade-bot` runs on the internal Podman compose network with no healthcheck and no metrics port.
  - Prod log shipper explicitly includes `comrade-bot` and writes `/var/log/containers/comrade-bot.log`; prod Promtail maps container file names to `container_name` and `service` labels.
  - Dev Promtail reads journald container logs and `/tmp/politburo.log`; it does not explicitly tail a Comrade Bot file. Existing dev `logs-errors.json` has a Comrade-Bot logs panel querying `{service="comrade-bot"}`, but reliability depends on container journal visibility.
  - Grafana dashboard datasource UIDs observed in existing dashboards: Prometheus `PBFA97CFB590B2093`, Loki `P8E80F9AEF21F6940`.
- **Relevant repo guidance discovered:**
  - Local infra is under `labour-bureau/`; production infra is under `labour-bureau/prod/`.
  - Existing Politburo metrics use `infra/metrics.MetricsRegistry`; do not create another Politburo registry. This bot-specific plan should not route metrics through Politburo.
  - Comrade Bot HTTP calls must remain centralized in `src/services/apiService.ts`; observability wrappers should not introduce direct API fetches from commands.

## 3. Existing reuse
- Reuse `InteractionRouter.route()` / `handleCommand()` as the single point for command hit counts, success/failure classification, and duration histograms.
- Reuse `BotClient` Discord lifecycle events for runtime logs and Discord event counters.
- Reuse the in-progress command registry plan once available: command names and metadata should come from the canonical registry rather than a third list.
- Reuse existing Prometheus/Grafana provisioning directories in `labour-bureau/grafana/provisioning/dashboards/` and `labour-bureau/prod/grafana/provisioning/dashboards/`.
- Reuse existing Promtail service labels `service=comrade-bot` / `container_name=comrade-bot`; normalize dashboards to prefer `{service="comrade-bot"}` where prod config supports it.
- Reuse production `podman-log-shipper.sh`, which already monitors `comrade-bot`.

## 4. Architecture decisions
- **Decision:** Add a small bot-local observability layer under `comrade-bot/src/infra/` or `src/observability/`, not inside command modules. Recommended files: `logger.ts`, `metrics.ts`, and `metricsServer.ts`.
- **Decision:** Use JSON logs to stdout/stderr with stable fields: `timestamp`, `level`, `service`, `env`, `event`, `command`, `interaction_type`, `guild_id`, `result`, `duration_ms`, and sanitized `error_name`/`error_message` where safe.
- **Decision:** Add `prom-client` and expose `/metrics` from Comrade Bot on `METRICS_PORT`, default `9091`, bound to `0.0.0.0` in containers.
- **Decision:** Track command hits per command in Prometheus. Track hits per server/guild primarily in Loki using structured log fields, not Prometheus labels, to avoid high-cardinality time series.
- **Decision:** Do not use raw `guild_id`, `user_id`, Discord interaction IDs, callsigns, IFC IDs, modal values, route params, API keys, or bot tokens as Prometheus labels or Loki labels.
- **Decision:** Allow raw `guild_id` as a JSON log field for operational analysis and Grafana LogQL top-N panels, but do not promote it to a Loki stream label.
- **Decision:** Dev and prod should both provision a dedicated Comrade Bot Grafana dashboard rather than overloading existing Politburo dashboards.
- **Decision:** Instrument at the router boundary first; replace scattered console calls incrementally, prioritizing startup, bot lifecycle, interaction routing, deployment, and API service errors.
- **Alternatives considered:**
  - Prometheus metric label `guild_id`: rejected by default due to high cardinality/privacy; only add behind explicit opt-in if product accepts the cost.
  - Poll Discord or Politburo for command usage: rejected; command execution events are already available in-process.
  - Send bot metrics through Politburo: rejected; it creates unnecessary coupling and bypasses clean service-local observability.
- **Open questions/risks:**
  - Decide whether guild IDs are acceptable in logs as raw IDs or should be hashed. Plan assumes raw guild ID in logs is acceptable for ops dashboards, but not as a label.
  - Decide whether `/rollout` command deployment events should be included in command metrics; recommended yes, as a command with `command="rollout"` and result labels.
  - Current TypeScript build may already be blocked by unrelated issues noted in the command refactor plan (`src/commands/pilot.ts` missing and stats command issues). Observability implementation should not hide those failures.

## 5. Repo-by-repo implementation plan
### politburo/
- No application code changes expected.
- Keep this plan separate from `comrade-bot-command-init-refactor.md`; downstream agents should coordinate sequencing but not merge the plans.
- Documentation-only updates may mention bot observability if a shared runbook exists, but do not modify routes, DI, jobs, migrations, OpenAPI, or generated code.

### comrade-bot/
- **Dependencies and config:**
  - Add `prom-client` as a runtime dependency.
  - Add environment/config support for:
    - `METRICS_ENABLED` default `true` in dev/prod unless explicitly disabled.
    - `METRICS_HOST` default `0.0.0.0`.
    - `METRICS_PORT` default `9091`.
    - `LOG_LEVEL` default `info`.
    - `APP_ENV` or reuse an existing env variable if introduced by the command init refactor.
  - Integrate with the typed env/config module proposed in `comrade-bot-command-init-refactor.md` if that work lands first.
- **Structured logger:**
  - Create `src/infra/logger.ts` or `src/observability/logger.ts`.
  - Emit one-line JSON logs to stdout/stderr.
  - Provide helpers like `logger.info(event, fields)`, `logger.warn(...)`, `logger.error(...)`.
  - Redact known sensitive keys: `DISCORD_BOT_TOKEN`, `BOT_TOKEN`, `API_KEY`, `Authorization`, `X-API-Key`, modal field values, and raw request/response bodies.
  - Remove the startup token/client ID log in `src/index.ts` immediately.
  - Replace direct console calls first in `src/index.ts`, `src/bot/BotClient.ts`, `src/handlers/InteractionRouter.ts`, `src/helpers/commandErrorHandler.ts`, `src/services/deploymentService.ts`, `src/deploy-commands.ts`, and high-volume `src/services/apiService.ts` paths.
- **Metrics registry and endpoint:**
  - Create `src/infra/metrics.ts` or `src/observability/metrics.ts` wrapping `prom-client`.
  - Enable default Node/process metrics with a `comrade_bot_` prefix or clear default metric names; document whichever convention is chosen.
  - Create a small HTTP metrics server (`src/infra/metricsServer.ts`) that serves `GET /metrics`; optionally `GET /healthz` for container health if lightweight.
  - Wire metrics server startup/shutdown from `src/index.ts` so it starts with the bot and closes on SIGINT/SIGTERM.
- **Core metrics:**
  - `comrade_bot_command_executions_total{command,result,interaction_type}` where `result` is one of `success`, `error`, `unknown_command`, `validation_error`, `permission_denied` if reliably known.
  - `comrade_bot_command_duration_seconds_bucket{command,result,interaction_type}` histogram.
  - `comrade_bot_interactions_total{interaction_type,result}` for modal/button/select/chat input dispatch.
  - `comrade_bot_discord_events_total{event,result}` for ready, warning, error, reconnect/resume if available from Discord.js events.
  - Optional `comrade_bot_api_requests_total{operation,result,status_class}` and `comrade_bot_api_request_duration_seconds_bucket{operation,result,status_class}` inside `ApiService`; do not label by raw path containing IDs.
- **Command instrumentation:**
  - In `InteractionRouter.handleCommand()`, capture start time, command name, guild ID, interaction type, and outcome.
  - Always log `command_started` and `command_completed`; on exceptions log `command_failed` with sanitized error fields.
  - Increment metrics in `finally` blocks so failures are counted.
  - For unknown commands, increment `command_executions_total{command="unknown",result="unknown_command"}` and log a warning with the provided command name as a field (not a label if the value is not from registry).
  - If the command registry refactor lands first, validate command names against the registry before using them as labels.
- **Server/guild hit dashboards:**
  - Emit `guild_id` as a JSON log field on command start/completion.
  - Do not include `guild_id` in Prometheus labels by default.
  - If product requires Prometheus per-server time series, gate it behind `METRICS_INCLUDE_GUILD_LABELS=false` default and document the cardinality risk.

### Vizburo UI
- Not applicable. No Vizburo UI changes should be made.

### labour-bureau/
- **Dev compose/Prometheus:**
  - Update `labour-bureau/docker-compose.dev.yml` `comrade-bot` env to include `METRICS_PORT=9091` and `METRICS_HOST=0.0.0.0` if not supplied by `.env`.
  - Because dev bot uses `network_mode: "host"`, add a `comrade-bot` scrape job in `labour-bureau/prometheus.dev.yml` targeting `host.docker.internal:9091` with `metrics_path: /metrics`.
- **Prod compose/Prometheus:**
  - Update `labour-bureau/prod/docker-compose.prod.yml` `comrade-bot` service with metrics env defaults and expose only internally as needed. Do not publish port 9091 to the public internet.
  - Add a `comrade-bot` scrape job in `labour-bureau/prod/prometheus.prod.yml` targeting `comrade-bot:9091`.
  - Consider a container healthcheck using a lightweight `/healthz` or `/metrics` probe if implemented.
- **Promtail/Loki:**
  - Update dev `promtail-config.yml` JSON pipeline to parse Comrade Bot fields: `level`, `timestamp`, `event`, `command`, `interaction_type`, `guild_id`, `result`, `duration_ms`, `error_name`.
  - Update prod `promtail-config.yml` similarly; preserve existing Politburo `L/T/M` parsing while adding bot JSON fields.
  - Promote only low-cardinality labels: `log_level`/`level`, `service`, `env` if already present. Do not label `guild_id`, `user_id`, `command`, or error messages unless explicitly reviewed. Command can remain a parsed field for LogQL.
  - If dev journald capture is unreliable for Docker logs, add an explicit dev file/log driver strategy for `comrade-bot`; do not break existing Politburo `/tmp/politburo.log` flow.
- **Grafana dashboards:**
  - Add new dashboard JSON in both dev and prod dashboard provisioning directories, e.g. `comrade-bot-observability.json`.
  - Use existing datasource UIDs: Prometheus `PBFA97CFB590B2093`, Loki `P8E80F9AEF21F6940`.
  - Keep dashboard JSON provisioned alongside existing dashboards; do not edit unrelated panels unless linking from `logs-errors.json` is useful.

### API contracts/generated clients/shared configuration
- Not applicable. No Politburo API contract changes are expected.
- No `make generate-api` work required.
- If `ApiService` operation-level metrics require TypeScript helper types, keep them local to `comrade-bot`; no generated client was observed.

## 6. Developer guidelines for implementation agents
- **Boundary rules:**
  - Do not add bot metrics to Politburo `infra/metrics.MetricsRegistry`.
  - Do not introduce polling jobs or background workers for usage metrics.
  - Do not log secrets, raw Discord payloads, modal values, callsigns, IFC IDs, API keys, tokens, or raw request headers.
  - Do not add `guild_id`/`user_id` as Prometheus labels by default.
  - Keep metrics endpoint internal to dev/prod monitoring networks; do not expose publicly.
  - If command registry refactor is already in progress, instrument the new canonical registry/router shape rather than reintroducing duplicated command lists.
- **Files likely to edit:**
  - `comrade-bot/package.json`, `package-lock.json` for `prom-client`.
  - New `comrade-bot/src/infra/logger.ts`, `metrics.ts`, `metricsServer.ts` or equivalent.
  - `comrade-bot/src/index.ts`, `src/bot/BotClient.ts`, `src/handlers/InteractionRouter.ts`, `src/helpers/commandErrorHandler.ts`, `src/services/apiService.ts`, `src/services/deploymentService.ts`, `src/deploy-commands.ts`.
  - `labour-bureau/docker-compose.dev.yml`, `prometheus.dev.yml`, `promtail-config.yml`.
  - `labour-bureau/prod/docker-compose.prod.yml`, `prod/prometheus.prod.yml`, `prod/promtail-config.yml`.
  - New dashboard files under `labour-bureau/grafana/provisioning/dashboards/` and `labour-bureau/prod/grafana/provisioning/dashboards/`.
- **Files/packages to avoid:**
  - `politburo/internal/api/generated/**`, Politburo migrations, Politburo DI/routes/jobs, and Vizburo templates/static assets.
  - Existing unrelated Grafana dashboards except for cross-links or logs panel alignment.
- **Sequencing recommendations:**
  1. Remove secret startup log immediately.
  2. Add logger and replace logs in startup/router lifecycle.
  3. Add metrics module and `/metrics` server.
  4. Instrument `InteractionRouter` command lifecycle.
  5. Add Prometheus scrape configs in dev/prod.
  6. Update Promtail parsing for bot JSON logs.
  7. Add Grafana dashboards.
  8. Expand lower-priority console replacements in command files/API service.

## 7. Auth scopes, claims, and context
- **Required scopes/roles/claims:** Not applicable for metrics scraping; Prometheus scrapes internal bot `/metrics` without Discord auth.
- **Middleware/context propagation:** No Politburo middleware changes. Bot command logs should use Discord interaction context available in `InteractionRouter`.
- **VA context handling:** Do not log VA-sensitive details by default. Server/guild activity should use `guild_id` only as an operational identifier unless hashing is chosen.
- **Mobile classification/impact:** Discord mobile users generate the same slash-command/button/modal interactions; no separate mobile instrumentation is required. Metrics should classify by `interaction_type`, not client platform, because platform was not observed as available in current code.

## 8. Migrations and data model
- Not applicable. No DB schema changes, backfills, or data migrations.
- Rollback: disable metrics via `METRICS_ENABLED=false`, remove Prometheus scrape target if needed, and keep JSON logs backward-compatible with Loki.

## 9. Error handling and response conventions
- Metrics server startup failure should be logged as `metrics_server_start_failed`; decide whether it is fatal. Recommendation: fatal in production when `METRICS_ENABLED=true`, warning when disabled.
- Command execution metrics must be recorded even when handlers throw and `InteractionRouter.handleError()` sends fallback replies.
- API service errors should log sanitized operation/status/error fields, not response bodies containing user data.
- Deployment logs should never print bot token/client ID/API key.

## 10. Constants and configuration
- Add documented env vars:
  - `METRICS_ENABLED=true`
  - `METRICS_HOST=0.0.0.0`
  - `METRICS_PORT=9091`
  - `LOG_LEVEL=info`
  - Optional `METRICS_INCLUDE_GUILD_LABELS=false` only if product insists on Prometheus per-server labels.
- Update `labour-bureau/prod/env/comrade-bot.env.example` and any dev `.env.example` equivalent if present.
- Keep all secret handling centralized with the env/config module planned in the command-init refactor when possible.

## 11. Logging and monitoring
- **Observability agent tasks:**
  - Implement and validate JSON logs for bot startup, ready, shutdown, Discord warnings/errors, command start/completion/failure, unknown commands, and deployment command push results.
  - Add Prometheus scrape jobs for dev/prod `comrade-bot`.
  - Extend Promtail JSON parsing for Comrade Bot fields in dev/prod.
  - Provision Comrade Bot Grafana dashboard with panels:
    - Bot target health: `up{job="comrade-bot"}`.
    - Command hits by command: `sum(rate(comrade_bot_command_executions_total[5m])) by (command)`.
    - Command success/failure by command: `sum(rate(comrade_bot_command_executions_total[5m])) by (command,result)`.
    - Command error rate: `sum(rate(comrade_bot_command_executions_total{result!="success"}[5m])) by (command) / sum(rate(comrade_bot_command_executions_total[5m])) by (command)`.
    - p50/p95/p99 command latency: `histogram_quantile(0.95, sum(rate(comrade_bot_command_duration_seconds_bucket[5m])) by (le,command))` and companion quantiles.
    - Interaction totals by type/result: `sum(rate(comrade_bot_interactions_total[5m])) by (interaction_type,result)`.
    - Node/process CPU/memory/event-loop/default metrics if enabled by `prom-client`.
    - Live logs panel: `{service="comrade-bot"}`.
    - Command failures logs: `{service="comrade-bot"} | json | event="command_completed" | result!="success"`.
    - Hits per server/guild from logs: `sum by (guild_id) (count_over_time({service="comrade-bot"} | json | event="command_completed" [$__interval]))`.
    - Top commands from logs: `sum by (command) (count_over_time({service="comrade-bot"} | json | event="command_completed" [$__interval]))`.
  - Add optional Grafana alerting recommendations/runbook notes for:
    - `up{job="comrade-bot"} == 0`.
    - elevated command error ratio.
    - p95 command latency above threshold.
    - repeated Discord client errors in Loki.
  - Confirm cardinality by inspecting Prometheus series count after dev guild testing.

## 12. API spec and generated code work
- **Swagger/OpenAPI agent tasks:** Not applicable.
- If a future health endpoint is added to Politburo, handle it separately; bot `/metrics` is not part of Politburo OpenAPI.

## 13. Documentation
- Update Comrade Bot developer/operator docs with:
  - Logging format and sensitive-field policy.
  - Metrics endpoint env vars and local curl command.
  - How to view the Comrade Bot dashboard in Grafana.
  - Why guild/server hit counts are log-derived rather than Prometheus labels.
- Update labour-bureau runbooks if Prometheus targets, dashboards, or env examples change.
- Mention the critical removal of token/client ID startup logging in changelog/dev notes if this repo uses them.

## 14. Frontend/Vizburo plan
- Not applicable. No Vizburo handler/template/style work.
- Grafana dashboards are infrastructure observability assets, not Vizburo UI.

## 15. Testing plan
- **Unit Testing agent tasks:**
  - Test logger redaction for known sensitive keys.
  - Test metrics label normalization and result classification helpers.
  - Test command instrumentation with mocked successful command, thrown command, and unknown command if a test harness is added.
  - Test metrics server handler returns Prometheus text format.
- **Manual/dev verification:**
  - From `comrade-bot/`: `npm run build` after dependency and TypeScript changes.
  - Start bot in dev; verify no token/client ID appears in logs.
  - `curl http://localhost:9091/metrics` and confirm command/default metrics exist.
  - Trigger representative commands in a dev guild; verify counters and histograms increment.
  - From `labour-bureau/`, start dev stack and verify Prometheus `comrade-bot` target is UP.
  - In Grafana, verify new dashboard panels populate and LogQL queries return bot logs.
- **Prod verification:**
  - Confirm `/var/log/containers/comrade-bot.log` receives JSON log lines.
  - Confirm Loki queries `{service="comrade-bot"}` and/or `{container_name="comrade-bot"}` return logs.
  - Confirm Prometheus target `comrade-bot:9091` is UP.
  - Confirm Grafana dashboard loads from provisioning with stable datasource UIDs.

## 16. Execution order for specialized agents
1. **Plan-to-code developer:** implement Comrade Bot logger, metrics endpoint, command instrumentation, dependency/config changes, and removal of secret startup logs.
2. **Observability-infra maintainer:** wire dev/prod Prometheus, Promtail, Docker/Podman env/healthcheck, and Grafana dashboards.
3. **Unit testing agent:** add tests for logger redaction, metrics helpers, metrics endpoint, and router instrumentation where feasible.
4. **Feature docs maintainer:** update operator/developer docs and dashboard/runbook notes.
5. **Swagger/OpenAPI agent:** not needed unless unrelated backend API changes are introduced.

## 17. Out-of-scope items
- Do not modify Politburo API behavior, DI, jobs, routes, migrations, or generated OpenAPI code.
- Do not add per-user metrics or labels.
- Do not add per-guild Prometheus labels by default.
- Do not expose bot metrics publicly.
- Do not introduce polling for command usage.
- Do not redesign command registration/deployment; coordinate with `comrade-bot-command-init-refactor.md` instead.

## 18. Final checklist
- **Source modifications avoided by this planner:** Yes. Only this markdown plan file should be created by the planner.
- **Plan file path:** `politburo/plans/comrade-bot-observability-grafana-plan.md`.
- **Key downstream agents/tasks:**
  - Remove existing secret startup logs.
  - Add JSON logger and `prom-client` metrics endpoint in Comrade Bot.
  - Instrument command executions, interactions, Discord lifecycle, and optional API calls.
  - Wire Prometheus/Promtail in dev and prod.
  - Add dedicated Grafana dashboards for command hits, server/guild log-derived activity, latency, errors, and logs.
