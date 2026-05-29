# Comrade Bot monitoring and observability

Comrade Bot emits service-local JSON logs and Prometheus metrics. The bot metrics endpoint is scraped by `labour-bureau` Prometheus in both dev and production, and logs are parsed by Promtail for Loki/Grafana.

## Structured logs

Logs are one-line JSON records written to stdout/stderr. Error-level logs go to stderr; other levels go to stdout. Stable fields include:

- `timestamp`, `level`, `service`, `env`, `event`
- `command`, `interaction_type`, `guild_id`, `result`, `duration_ms`
- `error_name`, `error_message` for sanitized errors

`service` is emitted as `comrade-bot`. `env` comes from `APP_ENV`, then `NODE_ENV`, then `development`.

### Redaction policy

The logger redacts known sensitive fields recursively, including tokens, API keys, `Authorization`, `X-API-Key`, request/response headers, request/response bodies, modal values, and params. Command error logging no longer logs raw command/modal params.

Do not intentionally log raw modal values, callsigns, IFC IDs, Discord payload bodies, API keys, bot tokens, or authorization headers.

## Metrics endpoint

When enabled, the bot starts an internal HTTP server with:

- `GET /metrics` — Prometheus text exposition
- `GET /healthz` — lightweight `ok` health response

Defaults and environment controls:

| Variable | Default | Notes |
| --- | --- | --- |
| `METRICS_ENABLED` | `true` | Set to `false` to disable the metrics server. |
| `METRICS_HOST` | `0.0.0.0` | Container-friendly bind address. |
| `METRICS_PORT` | `9091` | Scrape port. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `APP_ENV` | `NODE_ENV` or `development` | Included in JSON logs. |

Implemented metric names:

- `comrade_bot_command_executions_total{command,result,interaction_type}`
- `comrade_bot_command_duration_seconds{command,result,interaction_type}` histogram, including `_bucket`, `_sum`, and `_count` series
- `comrade_bot_interactions_total{interaction_type,result}`
- `comrade_bot_discord_events_total{event,result}`
- default Node/process metrics from `prom-client`, prefixed with `comrade_bot_`

`guild_id` and `user_id` are not Prometheus labels. Command labels come from the canonical command registry; unknown commands use `command="unknown"`.

## Labour Bureau wiring

### Prometheus

- Dev: `labour-bureau/prometheus.dev.yml` scrapes job `comrade-bot` at `host.docker.internal:9091/metrics`. This matches the dev bot's host-networked container.
- Production: `labour-bureau/prod/prometheus.prod.yml` scrapes job `comrade-bot` at `comrade-bot:9091/metrics` on the internal compose network.
- Production compose exposes port `9091` only internally and adds a `/healthz` healthcheck.

### Promtail and Loki

Dev and production Promtail parse Comrade Bot JSON fields including `level`, `event`, `command`, `interaction_type`, `guild_id`, `result`, `duration_ms`, `error_name`, and `error_message`.

Label policy is intentionally low-cardinality:

- Use `service=comrade-bot` for service filtering.
- Promote only low-cardinality level fields (`level` / `log_level`).
- Keep `guild_id`, `command`, and error details as parsed fields for LogQL, not Loki stream labels.

## Grafana dashboard

The provisioned dashboard file is `comrade-bot-observability.json` in both:

- `labour-bureau/grafana/provisioning/dashboards/`
- `labour-bureau/prod/grafana/provisioning/dashboards/`

Dashboard coverage includes target health, command hits, success/failure by command, error ratio, latency quantiles, interactions by type/result, process memory, live logs, command failure logs, and log-derived guild/command activity.

## Manual verification

1. Start the bot with metrics enabled.
2. Check the endpoint: `curl http://localhost:9091/metrics` and `curl http://localhost:9091/healthz`.
3. In Prometheus, verify target `comrade-bot` is UP.
4. In Loki/Grafana Explore, query `{service="comrade-bot"}`.
5. Open the Comrade Bot observability dashboard and trigger a known command in a dev guild to confirm command counters, latency, and logs populate.
