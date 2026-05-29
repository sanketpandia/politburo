# Comrade Bot Command Initialization and Deployment Refactor — Implementation Plan

## 1. Title and status
- **Status:** Proposed
- **Plan file:** `politburo/plans/comrade-bot-command-init-refactor.md`
- **Date:** 2026-05-20
- **Requested change summary:** Simplify Comrade Bot startup/init abstractions and slash-command deployment flow, especially how command definitions are registered, routed, and pushed to Discord. Evaluate whether command deployment should become part of the build step and recommend a safer workflow.
- **Scope and assumptions:**
  - Primary scope is `comrade-bot/` command registry, interaction routing, bot lifecycle, deployment scripts, and package scripts.
  - Secondary scope is `labour-bureau/` dev/prod deployment wiring where command push is currently invoked.
  - Politburo backend behavior is not expected to change, except for verification that `/initserver` continues to call the existing `POST /api/v1/server/init` route.
  - Recommendation: **do not push commands as part of plain `npm run build`** because Discord command deployment is a networked, credentialed, environment-specific side effect. Instead add an explicit `build:deploy-commands`/`release` script and keep production deploy scripts responsible for invoking it.

## 2. Context
- **Files/packages inspected:**
  - Workspace guidance: `AGENTS.md`, `politburo/CLAUDE.md`.
  - Existing plan style/context: `politburo/plans/discord-onboarding-help-status-plan.md`.
  - Comrade Bot startup/lifecycle: `comrade-bot/src/index.ts`, `src/bot/BotClient.ts`, `package.json`, `tsconfig.json`.
  - Command registration/deployment: `src/utils/commandLoader.ts`, `src/configs/commandMap.ts`, `src/deploy-commands.ts`, `src/services/deploymentService.ts`, `src/commands/rollout.ts`.
  - Interaction routing/init flow: `src/handlers/InteractionRouter.ts`, `src/commands/initServer.ts`, `src/commands/initServerButtonHandler.ts`, `src/commands/initServerModalHandler.ts`, `src/configs/constants.ts`, `src/services/apiService.ts`, `src/types/DiscordInteraction.ts`, `src/helpers/utils.ts`.
  - Backend endpoint validation for initserver: `politburo/internal/routes/router.go`, `internal/app/app.go`, `internal/servers/{handler.go,service.go,dto.go,errors.go}`.
  - Infra/deploy: `comrade-bot/Dockerfile`, `Dockerfile.dev`, `labour-bureau/docker-compose.dev.yml`, `labour-bureau/prod/docker-compose.prod.yml`, `prod/scripts/deploy-comrade-bot.sh`, `prod/scripts/start-comrade-bot.sh`, `prod/env/comrade-bot.env.example`.
- **Existing behavior and architecture summary:**
  - `src/index.ts` loads dotenv, validates `DISCORD_BOT_TOKEN` and `DISCORD_BOT_CLIENT_ID`, constructs `BotClient`, registers process signal/error handlers, then calls `bot.start()`.
  - `BotClient` owns the Discord.js `Client`, configures `ClientReady`, `InteractionCreate`, error/warn/debug handlers, and calls `DeploymentService.initialize(clientId, token)` in its constructor.
  - Command definitions are duplicated across two registries: `src/utils/commandLoader.ts` imports each command `data` for deployment, while `src/configs/commandMap.ts` imports each command module for runtime execution.
  - `InteractionRouter.ts` routes commands through `commandMap`, but modal/button/select routing remains a large hardcoded switch/if chain with raw IDs mixed with `CUSTOM_IDS` constants.
  - `src/services/deploymentService.ts` is static/global, stores credentials after initialization, and is used by in-bot `/rollout` plus indirectly mirrors CLI deployment behavior.
  - `src/deploy-commands.ts` independently reads env, validates command definitions, computes scope from argv/`GUILD_ID`, and pushes commands to Discord with REST `put`.
  - `package.json` has `build: tsc`, deployment scripts for compiled (`deploy`, `deploy:local`, `deploy:global`) and source (`deploy:dev*`) execution, but no explicit combined build-and-deploy command.
  - `labour-bureau/prod/scripts/deploy-comrade-bot.sh` already builds and starts the container, then runs `npm run deploy:local` or `npm run deploy:global` inside the container; command deployment failure is currently downgraded to a warning.
  - `comrade-bot/Dockerfile` compiles `dist/` at image build time and includes `dist/deploy-commands.js`; runtime `CMD` starts `node dist/index.js`.
  - `labour-bureau/docker-compose.dev.yml` runs Comrade Bot in dev with `npm install && exec ts-node-dev --respawn --transpile-only src/index.ts`; it does not deploy commands automatically.
  - `commandLoader.ts` imports `../commands/pilot`, but no `comrade-bot/src/commands/pilot.ts` was found by glob. This likely breaks `npm run build` until resolved or may reflect untracked/user-owned local state.
- **Relevant repo guidance discovered:**
  - Work from `comrade-bot/`; package manager is npm with `package-lock.json`.
  - Build/typecheck is `npm run build`; `npm test` is a placeholder that exits 1.
  - Slash commands are added under `src/commands/<name>.ts`, registered in `src/configs/commandMap.ts`, and deployed with `npm run deploy:dev:local` or `npm run deploy:dev:global`.
  - Politburo HTTP calls must remain centralized in `src/services/apiService.ts`; commands should not call `fetch` directly.
  - Bot-facing auth headers are generated in `comrade-bot/src/helpers/utils.ts` and backend auth expects `X-API-Key`, `X-Server-Id`, and `X-Discord-Id`.

## 3. Existing reuse
- Reuse existing command module convention where each slash command exports `data` and `execute`; do not introduce a new framework.
- Reuse `src/deploy-commands.ts` and `src/services/deploymentService.ts` REST logic, but consolidate around one non-static deployment helper to remove drift.
- Reuse `validateCommands()`, duplicate-name checks, and command names from `commandLoader.ts`, but move them behind a single registry that serves both deployment and runtime routing.
- Reuse `CUSTOM_IDS` for modal/button IDs and expand it instead of adding more raw string IDs.
- Reuse `InteractionRouter.route()` as the single Discord interaction entrypoint, but split registrations into small tables/maps.
- Reuse existing `/rollout` god-mode check through `ApiService.verifyGodMode()` if the command remains.
- Reuse production deploy hook in `labour-bureau/prod/scripts/deploy-comrade-bot.sh`; it is already the right environment-specific place to push commands after a build.

## 4. Architecture decisions
- **Decision:** Create one canonical bot command registry module that contains command `data` and `execute` together, then derive both runtime command lookup and deployable command JSON from it. This replaces the current `commandLoader.ts`/`commandMap.ts` duplication.
- **Decision:** Keep command deployment out of plain `npm run build`. Builds should remain deterministic and offline-friendly. Discord command push needs bot token/client ID, optional guild ID, global/local choice, rate-limit handling, and a network call.
- **Decision:** Add explicit scripts such as `commands:deploy`, `commands:deploy:local`, `commands:deploy:global`, and `build:commands:deploy` (or `release:commands`) so operators can run one command when desired without surprising every build.
- **Decision:** Make production deployment fail or explicitly opt into non-blocking behavior. Current `deploy-comrade-bot.sh` warns and continues on command deployment failure; implementation should choose a policy and encode it via env/flag, e.g. default fail in production, allow `ALLOW_COMMAND_DEPLOY_FAILURE=true` for emergency bot restarts.
- **Decision:** Do not auto-deploy commands when the bot process starts. Auto-pushing on every restart risks global-command churn/rate limits and makes runtime credentials do release work.
- **Decision:** Keep `/rollout` only if there is a strong operator need. If retained, it MUST call the same shared deployment helper as the CLI and should probably deploy guild-local by default. If removed, remove it from the registry and help/deployment output.
- **Decision:** Refactor `InteractionRouter.ts` into data-driven handler maps for commands, modals, buttons, and select menus while preserving current behavior.
- **Decision:** Keep `/initserver` as a thin command workflow over existing backend `POST /api/v1/server/init`; cleanup should focus on constants, handler grouping, validation/copy, and route organization, not backend endpoint redesign.
- **Open questions/risks:**
  - Confirm whether `src/commands/pilot.ts` is intentionally missing. Downstream implementation MUST run `npm run build` early and either restore/remove the command from the registry based on product intent.
  - Confirm whether operators want `/rollout` exposed in Discord at all. It currently allows god-mode users to redeploy commands from the bot process.
  - `prod/env/comrade-bot.env.example` does not include `DISCORD_BOT_CLIENT_ID`, `GUILD_ID`, or `API_KEY`, although the bot/deploy scripts need them in practice. Implementation should verify real env files before changing examples.

## 5. Repo-by-repo implementation plan
### politburo/
- No backend implementation is required for this refactor.
- Keep `/api/v1/server/init` registered in `internal/routes/router.go` via `application.Features.ServersHandler.InitServer()`.
- Preserve `internal/servers` request/response/error behavior: `InitServerRequest`, `InitServerResponse`, `httpdto.WriteSuccess`, `httpdto.WriteError`, and validation of at least one callsign prefix/suffix.
- If `/rollout` remains dependent on `GET /api/v1/admin/verify-god`, leave auth and route registration unchanged.

### comrade-bot/
- **Canonical command registry:**
  - Add a single registry module, e.g. `src/commands/registry.ts` or `src/configs/commandRegistry.ts`.
  - Registry entries SHOULD include at minimum: `name`, `data`, `execute`, `deploy`/`visible` boolean if needed, and optional metadata such as `adminOnly`, `ownerOnly`, or category.
  - Derive deploy JSON (`registry.map(entry => entry.data.toJSON())`), command names, duplicate-name validation, and runtime lookup map from this registry.
  - Replace `src/utils/commandLoader.ts` with wrappers around the registry or delete it after all imports move.
  - Replace `src/configs/commandMap.ts` with derived lookup from the registry or keep it as a compatibility export generated from the registry.
- **Deployment helper consolidation:**
  - Replace static `DeploymentService.initialize()` state with either:
    - an instantiated `CommandDeploymentService` created with `{ clientId, token }`, or
    - pure functions accepting `{ clientId, token, scope }`.
  - Make `src/deploy-commands.ts` parse env/argv, call the shared helper, and own CLI exit codes.
  - Make `/rollout`, if retained, call the same helper with bot credentials sourced from a small config object rather than static mutable global state.
  - Validate commands before every deploy path.
- **Package scripts:**
  - Keep `"build": "tsc"` side-effect free.
  - Add explicit scripts such as:
    - `"commands:deploy": "node dist/deploy-commands.js"`
    - `"commands:deploy:local": "node dist/deploy-commands.js local"`
    - `"commands:deploy:global": "node dist/deploy-commands.js global"`
    - `"build:commands:deploy:local": "npm run build && npm run commands:deploy:local"`
    - `"build:commands:deploy:global": "npm run build && npm run commands:deploy:global"`
  - Keep or rename current `deploy:*` scripts for backward compatibility, but avoid ambiguous names if they only deploy Discord commands and do not deploy the bot container.
- **Bot startup/config:**
  - Move env validation into a small config module, e.g. `src/configs/env.ts`, returning typed `BotConfig` with `discordBotToken`, `discordClientId`, `apiUrl`, `apiKey`, optional `guildId`.
  - `src/index.ts` should load dotenv, call config validation once, construct `BotClient` with config, register shutdown/error handlers, and start.
  - `BotClient` should only own Discord lifecycle and event wiring; remove deployment-service initialization from its constructor unless `/rollout` explicitly needs an injected deployment helper.
- **InteractionRouter cleanup:**
  - Keep `InteractionRouter.route()` public API.
  - Split modal handlers into a `Record<string, handler>` using `CUSTOM_IDS.REGISTER_MODAL`, `CUSTOM_IDS.INIT_SERVER_MODAL`, `register_link_modal`, and `CUSTOM_IDS.MEMBERSHIP_JOIN_MODAL` as needed.
  - Split exact button handlers into a `Record<string, handler>` for `initserver_proceed`, `register_new`, `register_link`, `CUSTOM_IDS.MEMBERSHIP_JOIN_BUTTON`, `tour_file_pirep`, and `tour_leg_file_pirep`.
  - Keep prefix/dynamic handlers for `CUSTOM_IDS.PIREP_MODAL`, `CUSTOM_IDS.PIREP_MODE_PREFIX`, `live_prev/next`, and `flights_prev/next` as small ordered checks.
  - Prefer constants for raw IDs: add `INIT_SERVER_PROCEED_BUTTON`, `REGISTER_NEW_BUTTON`, `REGISTER_LINK_BUTTON`, `TOUR_FILE_PIREP_BUTTON`, and `TOUR_LEG_FILE_PIREP_BUTTON` to `CUSTOM_IDS` or a companion constants module.
- **Initserver workflow cleanup:**
  - Keep slash command `data` in `initServer.ts` and modal submit in `initServerModalHandler.ts` unless the team wants one module per feature. If consolidating, ensure imports remain acyclic and registry stays simple.
  - Remove unused imports from `initServer.ts` (`ModalBuilder`, `TextInputBuilder`, `TextInputStyle` are not used there today).
  - Move initserver button/modal IDs to constants; no raw `"initserver_proceed"` in command/router code.
  - Keep API call in `ApiService.initiateServerRegistration()`; commands must not call `fetch` directly.
  - Preserve ephemeral replies.
- **Known build issue:**
  - Resolve `commandLoader.ts` import of missing `../commands/pilot` before or during registry work. If there is no intended pilot command, remove it from deployment. If intended, add/restore the command module in a separate justified implementation step.

### Vizburo UI
- Not applicable. This change is Discord bot deployment/runtime architecture only and should not touch Politburo server-rendered UI.

### labour-bureau/
- Update `labour-bureau/prod/scripts/deploy-comrade-bot.sh` to call the new explicit command-deploy script after container build/start, e.g. `npm run commands:deploy:global` or `npm run commands:deploy:local`.
- Decide failure policy:
  - Recommended production default: command deploy failure exits non-zero after logging a clear remediation command.
  - Optional escape hatch: allow non-blocking behavior only when an env var/flag is set.
- Update `prod/env/comrade-bot.env.example` to document required deploy/runtime env vars if confirmed missing: `DISCORD_BOT_TOKEN`, `DISCORD_BOT_CLIENT_ID`, `API_URL`, `API_KEY`, optional `GUILD_ID`.
- Update dev docs/scripts only if developers want a one-command local command push; do not add command push to `docker-compose.dev.yml` startup.
- `labour-bureau/manage.sh` references `npm run deploy-commands`, which does not exist in current `package.json`; either update it to the new script or mark it deprecated.

### API contracts/generated clients/shared configuration
- Not applicable unless implementation changes bot-facing backend API contracts, which this plan does not require.
- No OpenAPI generation is expected.
- Handwritten bot types may need local TypeScript type additions for registry/deployment config, but no generated client was observed.

## 6. Developer guidelines for implementation agents
- **Boundary rules:**
  - Do not modify Politburo backend behavior for this refactor unless verification reveals a direct compatibility bug.
  - Keep all Politburo HTTP calls centralized in `src/services/apiService.ts`.
  - Do not add command deployment to `npm run build` or bot startup as an implicit side effect.
  - Do not create a second command registry; all runtime and deploy command views must derive from one source.
  - Do not bypass `/api/v1/admin/verify-god` if retaining `/rollout`.
- **Files likely to edit:**
  - `comrade-bot/src/commands/registry.ts` or `src/configs/commandRegistry.ts` (new).
  - `comrade-bot/src/utils/commandLoader.ts`, `src/configs/commandMap.ts`, `src/handlers/InteractionRouter.ts`.
  - `comrade-bot/src/services/deploymentService.ts`, `src/deploy-commands.ts`, `src/commands/rollout.ts`, `src/bot/BotClient.ts`, `src/index.ts`.
  - `comrade-bot/src/configs/constants.ts`, possible new `src/configs/env.ts`.
  - `comrade-bot/package.json`.
  - `labour-bureau/prod/scripts/deploy-comrade-bot.sh`, `labour-bureau/prod/env/comrade-bot.env.example`, possibly `labour-bureau/manage.sh`.
- **Files/packages to avoid:**
  - Do not edit `politburo/internal/api/generated/**`.
  - Do not edit migrations, generated OpenAPI output, Politburo route/job wiring, or Vizburo UI.
  - Do not add a background worker/job for command deployment.
- **Sequencing recommendations:**
  1. Run `npm run build` from `comrade-bot/` to capture current baseline and confirm the missing `pilot` import failure.
  2. Introduce canonical command registry and migrate `commandLoader`/`commandMap` consumers.
  3. Consolidate deployment helper and update CLI `/rollout` use.
  4. Simplify `BotClient`/env config so startup is lifecycle-only.
  5. Refactor `InteractionRouter` handler maps and initserver constants.
  6. Update package scripts and labour-bureau deploy script.
  7. Validate build and manual command deployment paths.

## 7. Auth scopes, claims, and context
- **Required scopes/roles/claims:**
  - Discord command deployment requires `DISCORD_BOT_TOKEN` and `DISCORD_BOT_CLIENT_ID`; guild-local deployment requires `GUILD_ID` or a guild ID from the interaction.
  - Runtime bot API calls require `API_KEY` plus Discord user/server headers from `generateMetaHeaders()`/`generateRegistrationMetaHeaders()`.
  - `/rollout`, if retained, must continue requiring backend god-mode verification through `ApiService.verifyGodMode()`.
- **Middleware/context propagation:**
  - No Politburo middleware changes.
  - `DiscordInteraction.getMetaInfo()` should continue supplying `discordId` as guild ID and `userId` as Discord user ID.
- **VA context handling:**
  - `/initserver` remains VA-context creation for the current Discord server. Ensure refactor does not change the guild ID passed to `ApiService.initiateServerRegistration()`.
- **Mobile classification/impact:**
  - Discord mobile users may invoke slash commands and interact with modals/buttons. Refactor should preserve ephemeral responses and modal/button behavior; no mobile-specific UI changes.

## 8. Migrations and data model
- Not applicable. No database schema/data migration or backfill is required.
- Rollback is code/config only: restore prior package scripts/registry if deployment breaks, then redeploy commands manually with the previous script.

## 9. Error handling and response conventions
- Command deployment CLI should return non-zero on validation or Discord REST failure, with sanitized errors that do not print tokens.
- `/rollout`, if retained, should return ephemeral success/failure embeds and avoid exposing raw token, request headers, or stack traces.
- `InteractionRouter.handleError()` can remain the shared fallback, but refactoring should preserve `isRepliable()` checks and reply/follow-up behavior.
- `ApiService.initiateServerRegistration()` should keep current handling for 401/403/400/409 and `ApiResponse<InitServerResult>` envelopes.

## 10. Constants and configuration
- Add/centralize bot env parsing in a typed config module; avoid reading `process.env` throughout lifecycle/deployment code except inside config/CLI entrypoints.
- Ensure `DISCORD_BOT_CLIENT_ID` is documented wherever command deployment is documented.
- Ensure optional `GUILD_ID` is clearly scoped to local/guild command deployment.
- Keep secrets (`DISCORD_BOT_TOKEN`, `API_KEY`) out of logs.
- Move raw interaction IDs from router/commands to `CUSTOM_IDS` or a companion `INTERACTION_IDS` constant object.

## 11. Logging and monitoring
- **Observability agent tasks:**
  - Review `comrade-bot` logs after refactor for command validation/deployment/startup events.
  - Ensure logs include command deploy scope (`guild` vs `global`), command count, and success/failure without token/client secret leakage.
  - Keep cardinality low: do not add per-guild/per-command Prometheus labels unless there is an existing bot metrics stack to support it. None was observed.
  - Confirm prod Promtail/container log collection in `labour-bureau/prod/` still captures `comrade-bot` deployment and runtime logs.
  - No new Prometheus scrape target is required for this refactor.
  - If production deploy script failure policy changes, ensure operator-visible logs and exit codes are clear enough for alerting/runbooks.

## 12. API spec and generated code work
- **Swagger/OpenAPI agent tasks:** Not applicable for the expected refactor because no Politburo API schema changes are planned.
- If downstream implementation unexpectedly changes `/api/v1/server/init`, `/api/v1/admin/verify-god`, or other bot-facing endpoints, then an OpenAPI agent MUST update `politburo/api/openapi/registration.yaml`, operation IDs, request/response schemas, error schemas, auth declarations, and run `make generate-api` from `politburo/`.
- Do not hand-edit `politburo/internal/api/generated/**`.

## 13. Documentation
- Update any Comrade Bot README/docs that explain adding commands, command deployment, or production rollout. If no bot README exists, add concise notes where existing docs live rather than creating broad new documentation.
- Update `labour-bureau` deployment/runbook docs if command deployment script names or failure policy change.
- User-facing Discord help text only needs changes if `/rollout` is removed or command visibility changes.

## 14. Frontend/Vizburo plan
- Not applicable. No Vizburo handlers/templates/styles should be edited.
- If any future UI documentation is touched, preserve thin handlers and design-system/Tailwind token conventions, but this plan does not require UI work.

## 15. Testing plan
- **Unit Testing agent tasks:**
  - Add tests for command registry validation if the project has or adds a test harness; otherwise validate through TypeScript build and a small script invocation.
  - Test duplicate command name detection and missing description detection currently covered by `validateCommands()` behavior.
  - Test deployment scope selection for CLI args (`local`, `global`, default with/without `GUILD_ID`) using mocked Discord REST if a test framework is introduced.
  - Test `InteractionRouter` map dispatch by mocking command/modal/button handlers if feasible.
- **Build/manual verification:**
  - From `comrade-bot/`: `npm run build`.
  - From `comrade-bot/`: run a non-mutating command validation path if added, e.g. `npm run commands:validate`.
  - For actual Discord mutation, manually verify in a dev guild with `npm run build:commands:deploy:local` and `GUILD_ID` set.
  - Verify production script command with a dry-run or staging environment before global deploy.
  - Exercise `/initserver` command through button and modal in Discord to ensure constants/router refactor preserved behavior.
  - Exercise `/rollout` only if retained.
- **Regression checks:**
  - Ensure `npm run dev` still starts bot locally.
  - Ensure Docker production image still contains `dist/deploy-commands.js` or the renamed compiled CLI target.
  - Ensure `labour-bureau/prod/scripts/deploy-comrade-bot.sh` uses script names that exist in `package.json`.

## 16. Execution order for specialized agents
1. **Plan-to-code developer:** implement `comrade-bot` registry/deployment/startup/router refactor and package scripts within this plan scope.
2. **Observability/infra maintainer:** update and validate `labour-bureau` deployment script/env example/logging implications.
3. **Unit testing agent:** add feasible registry/deployment/router tests or validation scripts and run build/manual verification checklist.
4. **Feature docs maintainer:** update operator/developer docs for command deployment workflow.
5. **Swagger/OpenAPI agent:** only if backend API contracts unexpectedly change.

## 17. Out-of-scope items
- Do not redesign Politburo server initialization domain logic.
- Do not change database schema, VA membership data model, or callsign config storage.
- Do not add polling, scheduled jobs, or background workers for command deployment.
- Do not automatically push commands on every bot startup.
- Do not add command deployment to plain `npm run build`.
- Do not build a generated TypeScript API client as part of this cleanup.
- Do not refactor unrelated bot command UX beyond what is needed to centralize registry/routing.

## 18. Final checklist
- **Source modifications avoided by this planner:** Yes. Only this markdown plan file was created.
- **Plan file path:** `politburo/plans/comrade-bot-command-init-refactor.md`.
- **Key downstream agents/tasks:**
  - Canonical command registry replacing duplicate `commandLoader`/`commandMap` state.
  - Explicit build-and-command-deploy scripts, while keeping `build` side-effect free.
  - Shared deployment helper for CLI and optional `/rollout`.
  - Simplified `BotClient` startup/config and data-driven interaction router.
  - Labour-bureau production script/env documentation alignment.
