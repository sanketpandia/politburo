# Comrade Bot startup, environment, and command deployment

This document covers the shipped Comrade Bot startup flow, required environment, and Discord slash-command deployment workflow.

## Runtime startup flow

At runtime `src/index.ts`:

1. Loads `.env` with `dotenv`.
2. Validates typed configuration with `loadBotConfig()`.
3. Constructs `BotClient` with that config.
4. Starts the metrics server when `METRICS_ENABLED` is not `false`.
5. Registers graceful `SIGINT`/`SIGTERM` handlers.
6. Starts the Discord bot.

Graceful shutdown stops the bot and then the metrics server before exiting.

`npm run build` remains side-effect free: it runs `tsc` only and does not contact Discord or deploy slash commands.

## Environment variables

Required runtime variables:

| Variable | Notes |
| --- | --- |
| `DISCORD_BOT_TOKEN` | Required for bot login and command deployment. |
| `DISCORD_BOT_CLIENT_ID` | Required by runtime config and command deployment. |

Backend/API variables:

| Variable | Default | Notes |
| --- | --- | --- |
| `API_URL` | `http://localhost:8080` | Politburo API base URL. |
| `API_KEY` | unset | Optional in config but required for real backend calls that need bot API auth. |

Command deployment variable:

| Variable | Notes |
| --- | --- |
| `GUILD_ID` | Optional normally; required for explicit local/guild command deployment. |

Observability variables:

| Variable | Default |
| --- | --- |
| `APP_ENV` | `NODE_ENV` or `development` |
| `LOG_LEVEL` | `info` |
| `METRICS_ENABLED` | `true` |
| `METRICS_HOST` | `0.0.0.0` |
| `METRICS_PORT` | `9091` |

## Command registry

`src/commands/registry.ts` is the canonical source for slash commands. It defines registry entries with command `data`, `execute`, deployment status, name, and category.

User-facing command help lives in `src/commands/helpCatalog.ts`. Keep it aligned when command behavior changes; for example, `/initserver` now documents the one-field VA Code / ID bootstrap and points admins to Vizburo Basic Setup.

Compatibility exports remain:

- `src/utils/commandLoader.ts` derives deployable command JSON/names from the registry.
- `src/configs/commandMap.ts` derives the runtime command map from the registry.

`npm run commands:validate` validates the registry without mutating Discord state.

## Command deployment scripts

Available package scripts:

- `commands:validate` — validate registry only.
- `commands:deploy` — deploy compiled commands; default scope is guild if `GUILD_ID` is set, otherwise global.
- `commands:deploy:local` — deploy compiled commands to `GUILD_ID`; fails if `GUILD_ID` is missing.
- `commands:deploy:global` — deploy compiled commands globally.
- `build:commands:deploy:local` — build, then local deploy.
- `build:commands:deploy:global` — build, then global deploy.
- `deploy`, `deploy:local`, `deploy:global` — compatibility aliases for the `commands:*` scripts.
- `deploy:dev`, `deploy:dev:local`, `deploy:dev:global` — run deployment from TypeScript source via `ts-node`.

CLI scope behavior:

- `global` argument always deploys global commands.
- `local` argument deploys guild commands and requires `GUILD_ID`.
- No argument uses guild scope when `GUILD_ID` is set; otherwise it deploys globally.

Global Discord command propagation can take time; guild-local deployment is preferred for development verification.

## `/rollout` command

`/rollout` is retained. It uses the same shared `CommandDeploymentService` as the CLI and keeps god-mode verification through `ApiService.verifyGodMode()`.

## Production deployment behavior

`labour-bureau/prod/scripts/deploy-comrade-bot.sh` builds and starts the `comrade-bot` container, then runs command deployment inside the container:

- `commands:deploy:local` when called with `local`
- `commands:deploy:global` by default or when called with `global`

Command deployment failure fails production deploy by default and prints a remediation command. Set `ALLOW_COMMAND_DEPLOY_FAILURE=true` only for emergency non-blocking restarts where commands will be deployed manually afterward.

`labour-bureau/manage.sh deploy-commands` now runs `npm run commands:deploy` inside the bot container.

## Common troubleshooting

- **Missing `DISCORD_BOT_TOKEN` or `DISCORD_BOT_CLIENT_ID`:** startup or deployment exits with a config error. Set both values.
- **Local deploy fails with missing `GUILD_ID`:** set `GUILD_ID` or use global deployment intentionally.
- **Global commands do not appear immediately:** Discord global command propagation may lag; use guild-local deploy for dev checks.
- **Backend calls fail:** verify `API_URL`, `API_KEY`, and Politburo availability.
- **Prometheus target down:** verify `METRICS_ENABLED`, `METRICS_HOST`, `METRICS_PORT`, container health, and that Prometheus is scraping `host.docker.internal:9091` in dev or `comrade-bot:9091` in production.
