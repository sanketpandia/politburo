# Discord Onboarding, Help, and Status MVP — Implementation Plan

## 1. Title and status
- **Status:** Proposed
- **Plan file:** `politburo/plans/discord-onboarding-help-status-plan.md`
- **Date:** 2026-05-19
- **Requested change summary:** Refine Comrade Bot help, `/register`, user `/status`, and add a separate bot status command while keeping `/register` as the single MVP onboarding command and preserving Politburo API contracts.
- **Scope and assumptions:**
  - Scope spans `politburo/` bot-facing registration/status APIs, `comrade-bot/` slash commands and interaction handlers, and `labour-bureau/` observability/deployment notes.
  - Existing backend endpoints already cover most primitives: `GET /api/v1/user/status`, `POST /api/v1/pilots/register`, `POST /api/v1/memberships/join`, and public `GET /healthCheck`.
  - The current status endpoint does **not** cleanly represent “global user not found” plus “current server is/is not configured as a VA” in a single 200 response; plan backend/API work before bot logic depends on that shape.
  - Do not add `/membership join` for MVP; existing user-facing bot command must be removed from deployment/help while preserving backend `POST /memberships/join` as the internal API used by `/register` VA-linking.

## 2. Context
- **Files/packages inspected:**
  - Workspace guidance: `AGENTS.md`, `politburo/CLAUDE.md`.
  - Plans/conventions: `politburo/plans/registration-generated-code-tests.md`, `watermill-login-registration-refactor.md` naming/style context.
  - Politburo routing/DI/jobs: `internal/app/app.go`, `internal/routes/router.go`, `internal/routes/jobs.go`, `internal/runtime/server.go`.
  - Auth/response/health: `internal/middleware/auth.go`, `internal/auth/claims.go`, `internal/platform/httpdto/response.go`, `internal/platform/health/handler.go`, `internal/models/entities/health.go`.
  - Registration/membership domains: `internal/pilots/{handler.go,dto.go,registration_service.go}`, `internal/memberships/{handler.go,dto.go,service.go,errors.go}`, `internal/platform/memberships/{repo.go,service.go,model.go}`, `internal/platform/users/model.go`, `internal/platform/va/{repo.go,model.go,service.go}`, `internal/platform/roles/roles.go`.
  - API contract/codegen: `api/openapi/registration.yaml`, `api/openapi/registration.cfg.yaml`, `internal/api/generated/registration/server.gen.go`, `internal/api/registration/server.go`, `Makefile`.
  - Bot commands/services: `comrade-bot/src/commands/{help.ts,register.ts,registerButtonHandler.ts,registerModalHandler.ts,status.ts,membership.ts,membershipJoinButtonHandler.ts,membershipJoinModalHandler.ts}`, `src/configs/{commandMap.ts,constants.ts}`, `src/handlers/InteractionRouter.ts`, `src/services/apiService.ts`, `src/types/Responses.ts`, `src/helpers/{messageFormatter.ts,utils.ts,commandErrorHandler.ts}`, `src/utils/commandLoader.ts`, `src/deploy-commands.ts`, `package.json`.
  - Infra/observability: `labour-bureau/docker-compose.dev.yml`, `prometheus.dev.yml`.
- **Existing behavior and architecture summary:**
  - Politburo registers bot-facing routes directly in `internal/routes/router.go` under `/api/v1` with `AuthMiddleware`; generated registration code exists and should stay generated-only.
  - `AuthMiddleware` builds claims from `X-API-Key`, `X-Server-Id`, and `X-Discord-Id`; bot headers are centralized in `comrade-bot/src/helpers/utils.ts`.
  - `GET /api/v1/user/status` currently uses `claims.UserID()` and `claims.ServerID()`; if `UserID()` is empty it returns `404 USER_NOT_FOUND`, so bot must infer unregistered via errors today.
  - Current `UserDetailResponse` includes user fields, `affiliations`, and `current_va`, but lacks explicit `is_registered`, `current_server_is_va`, current VA identity when unlinked, and other-membership summary fields.
  - `JoinVA` already validates callsign against VA-scoped Airtable-synced pilots via `pilots.Repository.FindByCallsign(ctx, va.ID, callsign)` and checks duplicate callsign within the same VA through `UsersSvc.GetUserByCallsignAndVA`; it creates role `pilot`.
  - `RegisterPilot` stores Discord ID, IF Community ID, IF API ID, and active flag through `UsersSvc.RegisterUser`; no password/email/IF credential storage was observed in this active flow.
  - `/healthCheck` returns raw service names and details (`postgres`, `cache`, error details); bot status command must sanitize client-facing output.
  - Bot `/status` currently mixes user status and backend health, uses non-ephemeral `deferReply()`, and imports unused `node-fetch`; product direction requires user `/status` to be read-only account/current-server state only.
  - Bot `/help` already supports `/help command:<command>` but duplicates prose in `help.ts`, hardcodes choices, and contains stale copy (“registration is per-server”, optional callsign during registration).
  - Bot `/register` already calls `ApiService.getUserDetails()` first and branches, but cannot reliably distinguish unconfigured server from registered-unlinked VA because backend status lacks explicit server context.
  - Bot `/membership join` is deployed via `src/utils/commandLoader.ts` and routed via `commandMap.ts`/`InteractionRouter.ts`; MVP should hide/remove it from user-facing deployment while retaining join modal reuse where safe.
  - Local Prometheus scrapes Politburo `/metrics`; Politburo metrics are exposed by `internal/runtime/server.go` with the default Prometheus handler.
- **Relevant repo guidance discovered:**
  - DI dependencies must be initialized in `internal/app/app.go`; routes via `internal/routes/router.go`; jobs via `internal/routes/jobs.go`.
  - JSON responses should use `internal/platform/httpdto` envelopes.
  - New/changed bot-facing registration APIs must update `politburo/api/openapi/registration.yaml`; generated output under `internal/api/generated/**` must not be hand-edited and is regenerated with `make generate-api`.
  - Comrade Bot HTTP calls must remain centralized in `src/services/apiService.ts`; commands should not call `fetch` directly.

## 3. Existing reuse
- Reuse Politburo endpoints and services instead of creating ad hoc layers:
  - `internal/pilots.RegistrationService` for IF Community ID + last-flight validation.
  - `internal/memberships.Service.JoinVA` and `POST /api/v1/memberships/join` for VA callsign linking from `/register`.
  - `internal/platform/memberships.Repository.GetUserStatusByUserID` as the base for enriched registered-user status.
  - `internal/platform/va.Service/GetByDiscordServerID` to resolve whether current Discord server is a configured VA.
  - `internal/platform/roles.RolePilot` semantics; current join path already defaults to `pilot`.
- Reuse bot interaction patterns:
  - `register.ts` → status-first decision tree.
  - `registerButtonHandler.ts` → separate account-creation and VA-link modals.
  - `registerModalHandler.ts` → account creation then optional VA-link button.
  - `InteractionRouter.ts` centralized button/modal routing.
  - `ApiService` for all Politburo calls.
- Reuse response/error conventions:
  - `httpdto.WriteSuccess`, `WriteError`, `WriteValidationError`.
  - Existing `NotFoundError`, `UnauthorizedError`, `PermissionDeniedError` mapping in bot service layer.
- Reuse observability boundaries:
  - `infra/logging` structured logs in backend handlers/services.
  - Existing `/metrics` scrape path and `infra/metrics.MetricsRegistry` if new counters are warranted.

## 4. Architecture decisions
- **Decision:** Keep `/register` as the only MVP onboarding command. The bot may call `POST /api/v1/memberships/join` internally, but `/membership join` MUST be removed from slash-command deployment and user-facing help.
- **Decision:** Add/enrich a bot-facing status API shape before final bot decision-tree changes. Preferred minimal path: evolve `GET /api/v1/user/status` to return a 200 envelope with explicit state for unregistered users and current-server VA context, rather than forcing bot logic to infer state from 404s.
- **Decision:** Preserve unlimited VA membership model. Status copy and schemas MUST describe memberships as an array/summary, not one global/current VA outside the current Discord server context.
- **Decision:** Callsign remains VA-membership data (`va_user_roles.callsign`) and must only be collected after the status API says current server is a configured VA and user is not linked to it.
- **Decision:** Add a dedicated bot status command (`/botstatus` recommended for minimal Discord command tree changes) separate from user `/status`; it should call existing `/healthCheck` through `ApiService.getHealth()` or a renamed wrapper and sanitize details in bot code.
- **Decision:** Help should become catalog-driven in `comrade-bot` (for example `src/commands/helpCatalog.ts` or `src/configs/helpCatalog.ts`) with command metadata, category/intent, visibility, and command-specific help. `help.ts` should render from the catalog and not duplicate command prose.
- **Decision:** Backend should not expose raw health internals for this MVP unless a sanitized health API is explicitly added. Bot-side sanitization is sufficient for `/botstatus`; if backend changes are made, route through `internal/platform/health` and register canonically.
- **Alternatives considered:**
  - Add `/membership join`: rejected by product scope and existing `/register` can reuse join API internally.
  - Keep `/status` as combined user + health: rejected because product requires user `/status` to mirror onboarding state and bot operational status to be separate.
  - Poll backend for registration progress: not needed; flows are command/modal-driven and synchronous API calls already exist.
- **Open questions/risks:**
  - Confirm whether changing `/user/status` from 404-on-unregistered to 200-with-state is acceptable for any existing consumers besides `comrade-bot`. If not, add a new spec-driven endpoint (e.g. `/onboarding/status`) and keep old semantics.
  - `auth.MakeClaimsFromApi` behavior was not fully inspected; implementation must verify it can populate `DiscordUserID`, `DiscordServerID`, optional `UserID`, optional `ServerID` for unregistered/non-VA server cases.
  - `commandLoader.ts` imports `../commands/pilot`, but `glob` did not show `src/commands/pilot.ts`; build status should be verified before command registry edits.

## 5. Repo-by-repo implementation plan
### politburo/
- **Status API shape:**
  - Update `internal/memberships/dto.go` to add explicit fields, preserving existing fields where possible for compatibility:
    - `is_registered` / `global_user_exists` boolean.
    - `current_server` object with `discord_server_id`, `is_configured_va`, optional `va_id`, `va_name`, `va_code`.
    - `current_va` object with `is_member`, optional `role`, `is_active`, `callsign` (keep existing semantics).
    - `memberships_summary` or `other_memberships_count` plus optional safe `other_memberships` summary derived from `affiliations`.
  - Update `internal/memberships/handler.go:GetUserStatus` so missing `claims.UserID()` does not automatically lose current-server VA context. It should use claims Discord IDs and the VA service/repo to return an unregistered status, or delegate to an added service method.
  - Update `internal/memberships/service.go` and/or `internal/platform/memberships` to compose:
    - global user lookup by Discord ID if `UserID` is absent;
    - current server VA lookup by Discord server ID;
    - full membership affiliations when user exists.
  - Keep route registration in `internal/routes/router.go`; if adding a new endpoint, register via `application.Features.MembershipsHandler`.
- **Registration API:**
  - Keep `POST /api/v1/pilots/register` request minimal (`ifc_id`, `last_flight`). Do not add callsign to account creation.
  - Consider enriching `RegisterPilotResponse` with current server VA identity if needed for bot copy; otherwise bot can re-call status after successful account creation.
  - Preserve IF validation in `internal/pilots/registration_service.go` and do not store extra IF data beyond current `users.User` fields.
- **Membership API:**
  - Keep `POST /api/v1/memberships/join` as backend/internal bot call.
  - Ensure duplicate callsign validation remains VA-scoped, not global.
  - Use role constant semantics (`pilot`) and avoid Discord role-management side effects.
- **Health:**
  - No backend change required for MVP bot status if bot sanitizes `/healthCheck`.
  - If adding a sanitized endpoint, put it under `internal/platform/health`, use `httpdto` if under `/api/v1`, and register canonically.
- **Generated/OpenAPI:** see section 12.

### comrade-bot/
- **Help command:**
  - Create a structured help catalog module with entries: command name, category/intent, summary, command-specific details, user/admin visibility, MVP visibility, examples, related commands.
  - Update `help.ts` to build general/category and command-specific embeds from the catalog. Keep `/help command:<command>` option; generate command choices from visible catalog entries so it scales beyond 10 commands.
  - Remove stale help copy that implies registration is per-server or callsign is part of global registration.
  - Exclude `/membership join` and Discord role-management commands from MVP help.
- **Register flow:**
  - `register.ts` MUST call the enriched status API first using `interaction.getMetaInfo()`.
  - Implement decision tree exactly from product direction:
    - not globally registered: ephemeral intro with privacy posture + impersonation warning; account creation modal collects only IF Community ID and last-flight route; after success, if current server is configured VA, offer VA-link button/modal.
    - registered + current server configured VA + not linked: explain global account already exists and this flow links only current VA; open callsign modal.
    - registered + linked: show read-only current setup/status, including callsign/role if present; do not re-collect.
    - server not configured VA: allow/check global registration but do not collect callsign; explain VA linking requires a configured VA server.
  - `registerButtonHandler.ts` should keep separate modal IDs for account creation and VA linking; update labels/copy and constants rather than raw string IDs where possible.
  - `registerModalHandler.ts` should re-check status after account creation or trust enriched registration response only if spec supports it; do not collect callsign in the account-creation modal.
  - Ensure all replies/deferred replies are ephemeral.
- **User `/status`:**
  - Remove backend health call from `status.ts`.
  - Use enriched status API only; render registered/not registered, current server VA configured/not configured, linked/not linked, callsign/role, and other memberships count/summary.
  - On action needed, point to `/register`.
  - Use `deferReply({ ephemeral: true })` or direct ephemeral reply.
- **Bot status command:**
  - Add `src/commands/botstatus.ts` (or equivalent agreed command) and register in `commandMap.ts` and `commandLoader.ts`.
  - Call centralized `ApiService.getHealth()` with timeout/error handling (add wrapper if needed).
  - Render safe summary only: bot online, backend/API operational/degraded/unavailable, checked timestamp. Do not expose raw service names (`postgres`, `cache`), details, hostnames, raw errors, stack traces, API keys, or dependency internals.
  - Admin-specific extra context is optional; only add if permission checks are straightforward with Discord permissions. Keep sanitized.
- **Remove MVP `/membership join`:**
  - Remove `membership` from `commandMap.ts` and `COMMANDS` in `commandLoader.ts` so it is not routed/deployed.
  - Keep `membershipJoin*` files only if reused internally or for backward compatibility; otherwise mark for deletion by implementation agent if no imports remain. Do not remove backend join endpoint.
  - Redeploy slash commands after bot build.

### Vizburo UI
- No mandatory UI change for MVP onboarding.
- Admin role/callsign changes should remain a later Vizburo/admin concern; if already safely supported in `internal/vaadmin/handler.go`/templates, documentation may point admins there. Do not add Discord role-management commands.
- If future MVP admin screens are touched, handlers must stay thin and templates must use existing design-system/Tailwind token conventions in `templates/`/`static/`.

### labour-bureau/
- No required infra change for command behavior.
- Observability follow-up: if new backend metrics are added, ensure existing Prometheus scrape (`prometheus.dev.yml` targeting `host.docker.internal:8080/metrics`) remains sufficient; do not add a second registry.
- Bot deployment requires command redeploy after `comrade-bot` build: `npm run deploy:dev:local` or `npm run deploy:dev:global` depending on environment, with normal env vars.

### API contracts/generated clients/shared configuration
- `politburo/api/openapi/registration.yaml` must be updated before backend/bot implementation if `/user/status` response changes.
- Run `make generate-api` from `politburo/` after spec edits; do not hand-edit `internal/api/generated/**`.
- Bot has hand-written TS types in `src/types/Responses.ts`; update them to match status/registration response schema. No generated TS client was observed.

## 6. Developer guidelines for implementation agents
- **Boundary rules:**
  - Backend dependencies through `internal/app/app.go`; routes through `internal/routes/router.go`; jobs through `internal/routes/jobs.go` only if jobs are added (not expected).
  - Commands must not call `fetch` directly; use `ApiService`.
  - Preserve `httpdto` envelopes for `/api/v1` JSON responses.
  - Do not hand-edit generated OpenAPI code.
  - Keep account creation separate from VA linking in code and copy.
- **Files likely to edit:**
  - Politburo: `api/openapi/registration.yaml`, generated registration output after codegen, `internal/api/registration/server.go` if adapter types change, `internal/memberships/{dto.go,handler.go,service.go,handler_test.go}`, `internal/platform/memberships/{repo.go,service.go,model.go}` as needed, possibly `internal/platform/users/service.go` for Discord lookup reuse.
  - Bot: `src/commands/{help.ts,register.ts,registerButtonHandler.ts,registerModalHandler.ts,status.ts}`, new `src/commands/botstatus.ts`, new help catalog file, `src/services/apiService.ts`, `src/types/Responses.ts`, `src/helpers/messageFormatter.ts`, `src/configs/{commandMap.ts,constants.ts}`, `src/utils/commandLoader.ts`, maybe `InteractionRouter.ts` if modal/button IDs change.
  - Tests alongside touched files.
- **Files/packages to avoid:**
  - Do not add new legacy-style services under `internal/services` for this change.
  - Do not edit `internal/api/generated/**` except through codegen.
  - Do not add Discord role-management command files.
  - Do not add polling workers/jobs.
- **Sequencing recommendations:**
  1. OpenAPI/status schema design.
  2. Backend status API implementation/tests/codegen.
  3. Bot type/API-service updates.
  4. Bot command UX updates and `/membership join` deployment removal.
  5. Bot status command.
  6. Observability/test/docs/rollout.

## 7. Auth scopes, claims, and context
- **Required auth:**
  - Existing bot API auth via `X-API-Key`, `X-Server-Id`, `X-Discord-Id` generated by `comrade-bot/src/helpers/utils.ts`.
  - `/healthCheck` is currently public; bot status can call it without sensitive output.
- **Claims/context propagation:**
  - `AuthMiddleware` sets `auth.UserClaims` from API key headers. Implementation must verify claims retain raw Discord IDs even when no global user or VA exists.
  - User status should not require `IsRegisteredMiddleware`; it must support unregistered users for onboarding.
- **Roles/scopes:**
  - Account creation and status: authenticated bot request, no VA role required.
  - VA linking: registered global user and configured current VA; current endpoint rejects existing role via claims. Confirm behavior when claims role is empty but user is registered elsewhere.
  - Bot status: normal users get sanitized summary. Optional admin extra context should use Discord guild permissions, not backend internals.
- **VA context handling:**
  - Current Discord server is the only VA context for `/register` and `/status` actions.
  - Other memberships may be counted/summarized, but no global/current VA should be implied.
- **Mobile classification/impact:**
  - Discord mobile users are primary supported clients. Keep embeds concise, ephemeral, and avoid large tables for `/status`/`/botstatus`.
  - No Vizburo mobile UI change required.

## 8. Migrations and data model
- No migration expected for MVP if existing `users` and `va_user_roles` fields are sufficient:
  - `users.discord_id`, `users.if_community_id`, `users.if_api_id`, timestamps.
  - `va_user_roles.user_id`, `va_id`, `role`, `callsign`, `is_active`, timestamps.
- Existing migration `012_add_unique_constraint_if_community_id.sql` supports IF Community ID uniqueness.
- If implementation discovers missing unique constraints for VA-scoped callsigns, plan a separate migration with compatibility/backfill analysis; do not introduce global callsign uniqueness.
- Rollback should be API/bot behavior rollback only unless a new migration is actually added.

## 9. Error handling and response conventions
- Backend `/api/v1` responses MUST use `httpdto` envelope: `{status,result|error,responseTimeMs}`.
- Status API should prefer explicit state over exceptional control flow for normal onboarding cases:
  - unregistered user: 200 with `is_registered=false` if compatibility allows.
  - unauthenticated/missing API key: 401.
  - malformed validation: 422 via `WriteValidationError` for body endpoints.
  - backend/DB failures: 500 with non-sensitive message.
- Bot should map backend errors to friendly ephemeral copy, especially IF validation mismatch, IF ID already registered, callsign not in VA provider, callsign taken, already linked, VA not configured.
- Bot status command must collapse health fetch failures/timeouts/non-200/malformed JSON into “backend unavailable/degraded” without raw error text.

## 10. Constants and configuration
- No new backend env vars expected.
- Bot continues to use `API_URL`, `API_KEY`, Discord token/client/guild env vars.
- Move raw modal IDs like `register_link_modal` into `CUSTOM_IDS` to reduce drift.
- Help catalog should include a visibility flag for MVP-hidden commands like `membership` if retained in source but not deployed.
- Secret handling: never log or display API keys, Discord token, health raw dependency details, or IF credentials. Account flow should explicitly state no passwords/emails/IF credentials are collected.

## 11. Logging and monitoring
- **Observability agent tasks:**
  - Add/verify structured logs around status decisions with low-cardinality fields: `discord_server_id`, `is_registered`, `server_is_va`, `is_member`; avoid logging IF API IDs, full route histories, API keys, raw health errors, or PII beyond existing Discord/IF IDs where already used.
  - If metrics are added, use `infra/metrics.MetricsRegistry`; suggested low-cardinality counters: onboarding status requests by outcome, registration attempts by result/error_code, VA link attempts by result/error_code. Avoid labels containing user IDs, guild IDs, callsigns, IF IDs, routes, or VA names.
  - Verify `/metrics` exposure remains through `internal/runtime/server.go`; no new scrape target required for Politburo.
  - Bot observability: console logs should avoid raw exception dumps in user-facing flow where they might contain response bodies; sanitize health failures.
  - Docker/container labels in `labour-bureau/docker-compose.dev.yml` already label `comrade-bot`; no required change unless dashboards depend on command-specific metrics.
  - Monitoring gaps: no current alerting targets in `prometheus.dev.yml`; consider dashboard/alert follow-up only after metrics exist.

## 12. API spec and generated code work
- **Swagger/OpenAPI agent tasks:**
  - Update `politburo/api/openapi/registration.yaml` for the refined status model.
  - Ensure operation IDs remain stable or intentionally changed:
    - `getUserStatus` if evolving existing endpoint.
    - Add new operation ID only if a new endpoint is required for compatibility.
  - Define explicit schemas for `UserStatusResponse`, `UserStatusResult`, `CurrentServerStatus`, `CurrentVAStatus`, `MembershipSummary`/`VAAffiliation` with callsign/role optionality.
  - Update descriptions to say `/register` handles account creation and VA linking; remove “bot knows whether to prompt the user to /join” language from `/pilots/register`.
  - Keep security declarations for `ApiKey`, `ServerId`, `DiscordId` on `/api/v1` endpoints.
  - Ensure error schemas remain aligned with `httpdto` and validation error schemas.
  - Run `make generate-api` from `politburo/` after spec edits; verify generated `internal/api/generated/registration/server.gen.go` and adapter compile. Do not hand-edit generated files.
  - Update tests around generated adapter if response structs change.

## 13. Documentation
- Update bot/user docs if present (none inspected beyond command help) to explain:
  - `/register` creates global account and links current VA only when current server is configured.
  - Privacy posture and impersonation warning.
  - `/status` is user/account/current-server membership only.
  - `/botstatus` is operational status.
  - `/membership join` is not MVP user-facing.
- Add rollout notes for slash-command redeploy and global command propagation delay.
- If Vizburo/admin docs exist in a follow-up, note admin role/callsign changes happen there, not through Discord MVP commands.

## 14. Frontend/Vizburo plan
- Not applicable for required MVP UI; this change is Discord-command first.
- If later admin role/callsign management is added in Vizburo:
  - Keep handlers thin in `internal/vaadmin`.
  - Use existing templates/partials and design-system/Tailwind tokens only.
  - UI must call domain services, not direct infrastructure.
  - No polling; use existing request/HTMX patterns.
  - Mobile behavior should respect existing `mobile-*` partial conventions.

## 15. Testing plan
- **Unit Testing agent tasks — Politburo:**
  - Add/update `internal/memberships/handler_test.go` for status cases:
    - unregistered + server not VA;
    - unregistered + server is VA;
    - registered + server not VA;
    - registered + server is VA + not linked;
    - registered + linked with callsign/role;
    - multiple VA affiliations summary;
    - unauthorized request.
  - Add/update service/repo tests for VA context composition and no global callsign uniqueness assumption.
  - Preserve/extend tests for `RegisterPilot` IF validation and duplicate IF Community ID.
  - Keep focused command: `go test ./internal/api/... ./internal/pilots ./internal/memberships ./internal/servers ./internal/auth ./internal/platform/httpdto ./internal/platform/validation`.
- **Unit Testing agent tasks — Comrade Bot:**
  - Add TypeScript unit tests if harness exists; none observed (`npm test` placeholder exits 1), so at minimum add build/type coverage and lightweight pure-function tests if introducing catalog/render helpers.
  - Run `npm run build` after command/type updates.
  - Manually verify slash command JSON generation/deployment validation through `src/utils/commandLoader.ts`.
  - Test decision tree with mocked `ApiService` responses for each product scenario.
  - Verify all target commands reply ephemerally (`/help`, `/register`, `/status`, `/botstatus`).
- **Infra/manual verification:**
  - Start dev services from `labour-bureau/` and Politburo on host as usual.
  - Exercise `/healthCheck` success/failure behavior and bot `/botstatus` sanitized output.
  - Verify `/membership` no longer appears after dev command deployment.

## 16. Execution order for specialized agents
1. **Swagger/OpenAPI agent:** finalize status schema and registration descriptions; run codegen implications.
2. **Backend agent:** implement enriched status API through existing membership/platform services and DI; update tests.
3. **Unit Testing agent (backend):** add status/registration/membership coverage and focused generated-adapter tests.
4. **Bot agent:** update TS types/API service, help catalog, `/register`, `/status`, `/botstatus`, command registry, and MVP removal of `/membership` deployment.
5. **Unit Testing agent (bot):** build/typecheck and any available helper tests/manual mocks.
6. **Observability agent:** add/sanitize logs/metrics as warranted; verify Prometheus/Loki implications.
7. **Docs/rollout agent:** update docs/help/operational notes and perform slash-command redeploy steps.

## 17. Out-of-scope items
- No Discord `/membership join` MVP command.
- No Discord role-management commands.
- No self-service unlinking/callsign change unless an already safe flow is discovered and explicitly scoped later.
- No broad registration rewrite, new auth system, polling worker, or new infrastructure stack.
- No storage of passwords, emails, IF credentials, or full logbooks as part of account creation.
- No global/current VA concept beyond current Discord server context.

## 18. Final checklist
- [x] Planner inspected actual `politburo/`, `comrade-bot/`, and `labour-bureau/` context.
- [x] Source modifications avoided by this planner.
- [x] Created exactly one planning document.
- [x] Plan file path: `politburo/plans/discord-onboarding-help-status-plan.md`.
- [x] Key downstream tasks identified: OpenAPI/status schema, backend status composition, bot help/register/status/botstatus command updates, `/membership` deployment removal, tests, observability, rollout.
