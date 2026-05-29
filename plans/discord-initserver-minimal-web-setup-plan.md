# Discord Initserver Minimal Web Setup — Implementation Plan

## 1. Title and status
- **Status:** Proposed
- **Plan file:** `politburo/plans/discord-initserver-minimal-web-setup-plan.md`
- **Date:** 2026-05-20
- **Requested change summary:** Simplify Comrade Bot `/initserver` so Discord only bootstraps the current server with a simple VA Code / ID, then move VA name, callsign matching, Airtable/provider, schema, PIREP, event, livery, webhook, and other admin configuration into Vizburo/web UI. Admin copy must recommend a desktop browser or desktop view for setup.
- **Scope and assumptions:**
  - Scope spans `comrade-bot/` command UX, `politburo/` bot-facing `/api/v1/server/init`, OpenAPI/codegen, existing Vizburo/admin configuration surfaces, and deployment/testing/observability notes.
  - “Simple ID” means existing VA code (`virtual_airlines.code`) unless product explicitly renames it. It is already unique, human-readable, in `InitServerRequest`, and exposed in bot status/help copy.
  - The Discord server ID remains implicit via bot headers (`X-Discord-Server-Id` / generated header helper), and the backend VA UUID remains internal/support-facing.
  - `/initserver` remains admin-only Discord setup, separate from pilot `/register` and `/status`.
  - This plan does not implement the deeper Vizburo setup UI in full unless a minimal link/copy bridge is required; it identifies web setup surfaces and gaps for downstream work.

## 2. Context
- **Files/packages inspected:**
  - Workspace guidance: `AGENTS.md`, `politburo/CLAUDE.md`.
  - Product-pattern reference: `politburo/plans/discord-onboarding-help-status-plan.md`.
  - Bot init flow: `comrade-bot/src/commands/initServer.ts`, `initServerButtonHandler.ts`, `initServerModalHandler.ts`.
  - Bot registry/routing/types/API: `src/commands/registry.ts`, `src/configs/commandMap.ts`, `src/configs/constants.ts`, `src/handlers/InteractionRouter.ts`, `src/types/DiscordInteraction.ts`, `src/types/Responses.ts`, `src/services/apiService.ts`.
  - Bot help/dashboard patterns: `src/commands/helpCatalog.ts`, `src/commands/dashboard.ts`.
  - Backend server init: `politburo/internal/servers/{dto.go,handler.go,service.go,errors.go,handler_test.go}`.
  - Backend routing/DI: `internal/routes/router.go`, `internal/app/app.go`.
  - VA/platform setup surfaces: `internal/platform/va/{model.go,service.go}`, routes under `/dashboard/vaadmin`, `/dashboard/settings/datasource`, `/api/v1/admin/airtable`, `/api/v1/admin/livery-mappings`.
  - API contract: `politburo/api/openapi/registration.yaml`.
- **Existing behavior and architecture summary:**
  - `/initserver` is registered in `comrade-bot/src/commands/registry.ts` and deployable. Its slash command metadata uses `PermissionFlagsBits.Administrator`.
  - Current bot flow shows a long embed, then a modal that collects `vaCode`, `vaName`, `callsignPrefix`, and `callsignSuffix`.
  - `initServerModalHandler.ts` validates VA code/name plus at least one callsign pattern, calls `ApiService.initiateServerRegistration`, and displays raw `VA ID` on success.
  - `ApiService.initiateServerRegistration` posts to `POST /api/v1/server/init` with `va_code`, `va_name`, `callsign_prefix`, and `callsign_suffix` using centralized bot headers.
  - `internal/routes/router.go` registers `/api/v1/server/init` inside bot context middleware via `application.Features.ServersHandler.InitServer()`.
  - `internal/servers` requires a registered invoking user, rejects an already registered Discord server, creates `virtual_airlines`, writes callsign prefix/suffix into `va_configs`, and creates an admin membership for the invoking user.
  - `virtual_airlines` has `code` with a unique index, `discord_server_id` with a unique index, `name`, `is_active`, and `is_airtable_enabled` fields.
  - Existing Vizburo/admin routes already include `/dashboard/vaadmin`, `/dashboard/vaadmin/flight-modes`, `/dashboard/vaadmin/webhooks`, `/dashboard/settings/datasource`, and `/dashboard/settings/livery-mappings`; dashboard links can be generated through `POST /api/v1/signed-link` and `ApiService.generateSignedLink`.
- **Relevant repo guidance discovered:**
  - DI flows through `internal/app/app.go`; routes through `internal/routes/router.go`; jobs/workers through `internal/routes/jobs.go`.
  - `/api/v1` JSON responses should use `internal/platform/httpdto` envelopes.
  - Bot HTTP calls must stay centralized in `src/services/apiService.ts`.
  - OpenAPI source is `politburo/api/openapi/registration.yaml`; generated code under `internal/api/generated/**` must not be hand-edited and is regenerated with `make generate-api`.

## 3. Existing reuse
- Reuse existing `/api/v1/server/init` route and `internal/servers` feature package rather than adding a new endpoint.
- Reuse `virtual_airlines.code` as the typed VA Code / ID and `DiscordServerID` claims/header as the implicit server binding.
- Reuse `users.Repository.GetUserByDiscordID` and existing `ErrUserNotRegistered` prerequisite behavior.
- Reuse `va.Service.GetByDiscordServerID` / unique Discord server binding to detect already-initialized servers.
- Reuse `roles.RoleAdmin` admin membership creation for the initiating registered admin.
- Reuse `ApiService.generateSignedLink(meta, redirectTo)` and `/dashboard` command patterns for setup/dashboard links instead of inventing signed URL logic in the bot.
- Reuse existing Vizburo routes for follow-on setup: datasource (`/dashboard/settings/datasource`), VA admin (`/dashboard/vaadmin`), flight modes, webhooks, events, and livery mappings.
- Reuse `helpCatalog.ts` for command help copy; do not duplicate long prose in `/initserver`.

## 4. Architecture decisions
- **Decision:** `/initserver` becomes a minimal Discord bootstrap. Discord MUST collect only VA Code / ID and MUST NOT collect VA name, callsign prefix/suffix, Airtable credentials, schema mappings, PIREP config, event config, livery mappings, or webhook config.
- **Decision:** Treat “simple ID” as existing `va_code` for MVP. Product copy may say “VA Code / ID” during transition; code/schema should keep the canonical JSON field `va_code`.
- **Decision:** The backend should create a minimal VA with `Code=va_code`, `DiscordID=current Discord server`, `IsActive=true`, and a safe placeholder `Name` derived from the code until web setup captures display name. This avoids a DB migration if `name` remains non-null/expected by existing UI.
- **Decision:** Keep the prerequisite that the invoking Discord user must already be globally registered. Bot copy should preflight and guide admins to `/register` before opening the modal.
- **Decision:** On success, return/display a dashboard or setup URL when available; otherwise display the `/dashboard` command CTA. Signed dashboard links should use the existing `ApiService.generateSignedLink` / `POST /api/v1/signed-link` path.
- **Decision:** Success and help copy MUST recommend “desktop browser or desktop view” for admin setup because the remaining configuration is dense and web-admin-oriented.
- **Decision:** Do not add jobs, polling, background verification, or new infrastructure for this change.
- **Alternatives considered:**
  - Generated setup token/ID: not recommended for MVP; no existing token lifecycle was observed beyond signed dashboard links.
  - Backend VA UUID as the simple ID: rejected for product UX; it is internal and currently overexposed in success copy.
  - Discord server ID as the ID: rejected because it is already implicit and not meaningful to admins.
- **Open questions/risks:**
  - Confirm the placeholder VA name format. Recommendation: default `Name` to the submitted `va_code` until Vizburo setup supports editing it.
  - Confirm whether existing Vizburo has a VA profile/name editor. Routes inspected show datasource, VA admin, flight modes, webhooks, events, and livery mappings, but no explicit VA name/callsign-pattern setup page was verified.
  - Confirm if signed links to admin subroutes like `/dashboard/settings/datasource` work for newly-created admin memberships without requiring an already configured Airtable provider.

## 5. Repo-by-repo implementation plan
### politburo/
- **Server init request/response:**
  - Update `internal/servers/dto.go` so `InitServerRequest` requires only `VACode` (`json:"va_code" validate:"required"`). Remove or ignore `VAName`, `CallsignPrefix`, and `CallsignSuffix` from the init request.
  - Update `InitServerResponse` to keep safe fields: `success`, `message`, `va_code`; add `setup_required bool` and optional `setup_url`/`dashboard_url` only if backend is chosen to produce a link. Prefer not returning `va_id` to the bot unless needed by support/admin tooling.
- **Handler/service contract:**
  - Update `serverRegistrationHandlerService.InitServer` in `internal/servers/handler.go` and implementation in `service.go` to accept only `discordServerID`, `discordUserID`, and `vaCode` unless a link dependency is added.
  - Remove handler-level callsign-config validation (`INVALID_CALLSIGN_CONFIG`) from `/server/init` because callsign patterns move to Vizburo.
  - Preserve `auth.GetUserClaims` and `httpdto.WriteSuccess`/`WriteError` conventions.
- **Minimal VA creation:**
  - In `internal/servers/service.go`, validate `vaCode` is present; normalize consistently with existing bot behavior (uppercase in bot, backend should still trim/normalize defensively if conventions allow).
  - Preserve server uniqueness check through `vaSvc.GetByDiscordServerID`.
  - Preserve registered-user prerequisite through `usersRepo.GetUserByDiscordID`.
  - Create `va.VA{Name: vaCode, Code: vaCode, DiscordID: discordServerID, IsActive: true}` or a similarly safe placeholder name.
  - Do not write `callsign_prefix`/`callsign_suffix` in this endpoint.
  - Preserve admin membership creation with `roles.RoleAdmin` and empty callsign.
- **Error codes:**
  - Keep `SERVER_ALREADY_REGISTERED`, `USER_NOT_REGISTERED`, and `VA_CREATION_FAILED`.
  - Remove `INVALID_CALLSIGN_CONFIG` from this endpoint or leave it unused for compatibility; do not emit it from minimal init.
  - Add a distinct conflict code if duplicate `va_code` can be detected cleanly (e.g. `VA_CODE_ALREADY_EXISTS`) rather than surfacing generic `VA_CREATION_FAILED`; otherwise document it as an implementation follow-up.
- **Routing/DI/jobs:**
  - Keep route registration in `internal/routes/router.go` via `application.Features.ServersHandler.InitServer()`.
  - No new DI dependency is expected unless server init itself generates a signed setup URL. If adding URL generation in backend, wire through `internal/app/app.go` using existing `Infra.URLSigner`/auth handler patterns instead of global state.
  - No scheduled job or worker changes expected.

### comrade-bot/
- **Slash command and preflight:**
  - Update `src/commands/initServer.ts` description/copy to “Bootstrap this Discord server with a VA Code / ID”. Keep `PermissionFlagsBits.Administrator`.
  - Block DM usage before API calls (`guildId` empty) with friendly ephemeral copy.
  - Preflight with `ApiService.getUserDetails()` before showing the setup button/modal:
    - unregistered: tell admin to run `/register`, then rerun `/initserver`;
    - current server already configured: show existing VA code/name and point to dashboard;
    - registered + unconfigured server: show short bootstrap copy and `Start setup` button.
  - Recheck eligibility in `initServerButtonHandler.ts` or rely on modal submit revalidation; preferred: lightweight recheck before showing the modal to avoid stale state.
- **Modal:**
  - Update `initServerButtonHandler.ts` to collect only `vaCode` with label “VA Code / ID”, min/max aligned with existing validation, and placeholder like `IFE`.
  - Remove `vaName`, `callsignPrefix`, and `callsignSuffix` modal fields.
- **Submit/API:**
  - Update `initServerModalHandler.ts` to validate only VA code and call `ApiService.initiateServerRegistration(meta, vaCode)`.
  - Stop logging VA name and callsign patterns from the modal handler.
  - On success, hide raw backend VA UUID. Show VA code, dashboard/setup CTA, and desktop/desktop-view recommendation.
  - Add setup-specific error copy for `USER_NOT_REGISTERED`, `SERVER_ALREADY_REGISTERED`, duplicate code conflicts, auth/permission errors, and malformed VA code.
  - Update `src/services/apiService.ts` request body and `src/types/Responses.ts` `InitServerResult` to match the minimal API response.
  - Preserve centralized API calls; commands must not call `fetch` directly.
- **Help/status/dashboard copy:**
  - Update `helpCatalog.ts` `/initserver` entry: “Bootstrap the Discord server with a VA Code / ID; complete setup in Vizburo. Desktop browser or desktop view recommended.”
  - Ensure `/register` help remains pilot/global account onboarding and current-server VA linking.
  - If `/status` displays server setup state, distinguish “server initialized” from “full VA setup complete” only if backend exposes setup completeness.
  - Reuse `/dashboard` or a generated signed setup link for admin continuation.

### Vizburo UI
- Existing admin routes already provide several setup areas, but the minimal init shift creates a product requirement for web setup ownership.
- Downstream Vizburo work SHOULD add or verify an admin setup/profile surface for:
  - VA display name;
  - callsign prefix/suffix or live flight matching pattern;
  - setup checklist linking to datasource, PIREP/flight modes, events/tours, livery mappings, and webhooks.
- Use existing thin handler pattern under relevant feature packages (`internal/vaadmin`, `internal/datasource`, `internal/liverymappings`, `internal/events`); do not put UI business logic in templates.
- Styling MUST use existing templates/static/Tailwind/design-system token conventions; no direct infrastructure access from UI.
- No polling; prefer normal request/HTMX form submission patterns already used by dashboard routes.
- Mobile classification: admin setup is desktop-first. Mobile users should see copy recommending desktop browser or desktop view rather than dense Discord modal setup.

### labour-bureau/
- No required infrastructure change for the minimal command/API flow.
- After bot command metadata/copy changes, run the existing command deployment flow from `comrade-bot/`: `npm run deploy:dev:local` for guild dev or `npm run deploy:dev:global`/production equivalent.
- If new backend metrics are added, existing dev Prometheus scrape of host Politburo `/metrics` in `labour-bureau/prometheus.dev.yml` should remain sufficient.

### API contracts/generated clients/shared configuration
- Update `politburo/api/openapi/registration.yaml` before backend implementation:
  - `/server/init` description: Discord bootstrap only; registered admin required; creates a minimal VA linked to current Discord server; full setup is completed in Vizburo.
  - `InitServerRequest.required` should be `[va_code]` only.
  - Remove `va_name`, `callsign_prefix`, and `callsign_suffix` from the init request schema or mark removed/deprecated if backward compatibility is required.
  - `InitServerData` should include `success`, `message`, `va_code`, `setup_required`, and optional safe dashboard/setup link fields. Avoid exposing `va_id` to bot UX by default.
  - Keep operation ID `initServer` unless a breaking regeneration requires a deliberate rename.
- Run `make generate-api` from `politburo/` after spec edits; do not hand-edit `internal/api/generated/**`.
- Bot TS types are handwritten in `comrade-bot/src/types/Responses.ts`; update them manually to match the spec.

## 6. Developer guidelines for implementation agents
- **Boundary rules:**
  - Backend dependencies through `internal/app/app.go`; routes through `internal/routes/router.go`.
  - No new dead-end packages; keep server init work in `internal/servers` and platform VA operations in `internal/platform/va`.
  - Bot commands must use `ApiService`; no direct command-level fetches.
  - `/api/v1` responses must use `httpdto` envelopes.
  - Do not hand-edit generated OpenAPI code.
  - Do not add polling, jobs, role provisioning, or Discord-based admin setup wizard.
- **Files likely to edit:**
  - Politburo: `api/openapi/registration.yaml`, generated `internal/api/generated/registration/server.gen.go` via codegen only, `internal/api/registration/server.go` if generated adapter signatures change, `internal/servers/{dto.go,handler.go,service.go,errors.go,handler_test.go}`, possibly `internal/platform/va` if adding explicit setup status helpers.
  - Comrade Bot: `src/commands/{initServer.ts,initServerButtonHandler.ts,initServerModalHandler.ts,helpCatalog.ts}`, `src/services/apiService.ts`, `src/types/Responses.ts`, possibly `src/configs/constants.ts` if modal IDs/labels are centralized further.
  - Vizburo follow-up: likely `internal/vaadmin` or a new setup sub-surface under existing dashboard/admin routing, plus templates/static assets.
- **Files/packages to avoid:**
  - Do not add new legacy-style services under `internal/services`.
  - Do not edit `internal/api/generated/**` except through `make generate-api`.
  - Do not add Discord role-management command files.
  - Do not place Airtable credentials or schema setup in `comrade-bot`.
- **Sequencing recommendations:**
  1. OpenAPI request/response revision and codegen.
  2. Backend minimal `/server/init` implementation and tests.
  3. Bot API types/service update.
  4. Bot UX/copy/modal simplification.
  5. Vizburo setup link/profile/checklist follow-up if scoped.
  6. Observability, docs/help, slash-command redeploy.

## 7. Auth scopes, claims, and context
- **Required scopes/roles/claims:**
  - Bot API auth continues through `X-API-Key`, `X-Discord-Id`, and `X-Server-Id`/Discord server header generation in bot helpers.
  - `/server/init` requires Discord bot context through `RequireDiscordBotContextMiddleware()` and claims from `AuthMiddleware`.
  - Invoking user must be globally registered; backend currently checks `usersRepo.GetUserByDiscordID` and returns `USER_NOT_REGISTERED`.
  - Discord command metadata requires Administrator. Bot should also block obvious non-guild/DM usage. A modal-submit permission recheck is recommended if Discord.js exposes member permissions at that stage.
- **Middleware/context propagation:**
  - Preserve `auth.GetUserClaims(r.Context())` in `internal/servers/handler.go`.
  - Do not add `IsRegisteredMiddleware()` to `/server/init` unless it preserves the current clear `USER_NOT_REGISTERED` setup error semantics.
- **VA context handling:**
  - Current Discord server is the VA context. Do not allow admins to type Discord server IDs.
  - The created VA is tied to `claims.DiscordServerID()` and the initiating registered user becomes admin for that VA.
- **Mobile classification/impact:**
  - Discord mobile is supported only for the minimal one-field bootstrap.
  - Admin configuration after bootstrap should explicitly recommend desktop browser or desktop view.

## 8. Migrations and data model
- No migration expected for minimal init if `virtual_airlines.name` can safely store the VA code as a placeholder.
- Existing data model evidence:
  - `virtual_airlines.code` is unique and suitable for VA Code / ID.
  - `virtual_airlines.discord_server_id` is unique and binds one Discord server to one VA.
  - `va_configs` currently stores callsign prefix/suffix but minimal `/initserver` should stop writing those keys.
  - `va_data_provider_configs` and admin routes support provider/datasource configuration outside Discord.
- If implementation discovers a NOT NULL/display requirement for `Name` beyond GORM model tags, use `Name=Code` as compatibility fallback rather than adding a migration.
- Rollback should be code/API behavior rollback; no backfill expected.
- Existing VAs initialized with name/callsign config remain valid; do not delete or rewrite their configs.

## 9. Error handling and response conventions
- Backend MUST keep `httpdto` envelope shape: `{status,result|error,responseTimeMs}`.
- Expected backend statuses:
  - 201: minimal VA created and admin membership created.
  - 400/422: missing or invalid `va_code`.
  - 400: `USER_NOT_REGISTERED` for registered-account prerequisite, unless changed to a more precise status by existing conventions.
  - 401: missing/invalid API key.
  - 403: missing Discord bot context headers.
  - 409: server already initialized; duplicate VA code should also be conflict if explicitly handled.
  - 500: unexpected persistence failures with non-sensitive message.
- Bot should map machine error codes from `ApiResponse.error.code` rather than brittle message matching.
- Bot user-facing errors must be setup-specific and ephemeral:
  - “Run `/register` first.”
  - “This server is already initialized.”
  - “That VA Code / ID is already in use.”
  - “Use `/dashboard`/Open setup dashboard to continue configuration.”

## 10. Constants and configuration
- No new backend env vars expected.
- Bot continues to use `API_URL`, `API_KEY`, Discord token/client/guild vars.
- `CUSTOM_IDS.INIT_SERVER_MODAL` and `INIT_SERVER_PROCEED_BUTTON` can remain; update labels/copy rather than introducing new IDs unless backward-compatible cleanup is needed.
- If returning signed setup links, use existing signed-link TTL defaults unless product asks for a different admin setup TTL.
- Secret handling:
  - Never log or display API keys, Discord tokens, Airtable credentials, raw signed-token payloads, or provider schema secrets.
  - Do not include raw backend VA UUID in normal Discord success copy.

## 11. Logging and monitoring
- **Observability agent tasks:**
  - Verify structured logs around server init stay low-cardinality and non-sensitive.
  - Suggested backend log fields: `discord_server_id`, `va_code`, `result`/error code. Avoid user IDs where not already used, signed URLs, Airtable credentials, and stack traces in user-facing paths.
  - Suggested bot log/metric outcomes if using existing bot metrics: `initserver_preflight_unregistered`, `initserver_preflight_already_initialized`, `initserver_modal_submitted`, `initserver_created`, `initserver_api_error`.
  - If backend metrics are added, use `infra/metrics.MetricsRegistry`; avoid labels containing guild IDs, user IDs, VA codes, VA names, signed URLs, or raw error messages.
  - Existing Politburo `/metrics` exposure and local Prometheus scrape should remain unchanged; no new scrape target or Docker label expected.
  - Privacy: do not log Airtable/provider secrets when redirecting admins to web setup.

## 12. API spec and generated code work
- **Swagger/OpenAPI agent tasks:**
  - Update `politburo/api/openapi/registration.yaml` `/server/init` summary/description to minimal Discord bootstrap.
  - Update `InitServerRequest` required fields to `va_code` only.
  - Remove or deprecate `va_name`, `callsign_prefix`, and `callsign_suffix` from `InitServerRequest`.
  - Update `InitServerData`/`InitServerResponse` schemas for `setup_required` and optional `dashboard_url`/`setup_url` if included.
  - Ensure error schemas include/describe `USER_NOT_REGISTERED`, `SERVER_ALREADY_REGISTERED`, duplicate `va_code` conflict, validation, auth, and bot-context failures.
  - Keep security declarations for API key and Discord context headers.
  - Run `make generate-api` from `politburo/`; verify generated strict server and `internal/api/registration/server.go` adapter compile.
  - Do not hand-edit generated output.

## 13. Documentation
- Update command help (`comrade-bot/src/commands/helpCatalog.ts`) as part of bot implementation.
- Update any bot/admin docs if present to explain:
  - `/initserver` only claims/bootstrap current Discord server with VA Code / ID.
  - Admins must run `/register` first.
  - Full setup continues in Vizburo.
  - Desktop browser or desktop view is recommended for admin configuration.
  - Pilots should use `/register` only after staff complete required VA setup.
- Add rollout notes for slash-command redeploy and global propagation delay.
- If Vizburo setup pages are added, update relevant admin docs/runbooks with setup order.

## 14. Frontend/Vizburo plan
- This change intentionally moves configuration to Vizburo, but downstream implementation should be scoped:
  - MVP bot/backend slice may only link to existing `/dashboard` or `/dashboard/settings/datasource` if that route is usable immediately after init.
  - Follow-on Vizburo slice should add an admin setup checklist/profile page if missing.
- Thin handlers:
  - Use existing dashboard/admin route grouping in `internal/routes/router.go`.
  - Prefer `internal/vaadmin` for VA profile/callsign-pattern setup and `internal/datasource` for provider setup.
- UI must not directly access infra packages; use platform/domain services already wired in `app.App`.
- Styling must use existing Tailwind/design-system token conventions in `templates/` and `static/`.
- No polling. Use normal form submissions/HTMX fragments where existing pages do.
- Mobile behavior:
  - Admin setup should be responsive where possible, but product copy should say desktop/desktop view is recommended.
  - Discord should not host dense setup forms beyond one VA Code / ID field.

## 15. Testing plan
- **Unit Testing agent tasks — Politburo:**
  - Update `internal/servers/handler_test.go` fake service signature and request bodies.
  - Remove/replace `TestInitServer_InvalidCallsignConfig` with tests for `va_code` required/invalid.
  - Update success test to assert service receives only VA code and response includes `setup_required`/safe fields, not raw VA ID unless intentionally retained.
  - Add service tests or repository-backed tests if harness exists for:
    - registered user + unconfigured server + unique VA code succeeds;
    - unregistered user returns `USER_NOT_REGISTERED`;
    - server already initialized returns `SERVER_ALREADY_REGISTERED`;
    - duplicate VA code returns conflict or documented error;
    - no callsign config rows are written by init.
  - Focused command: `go test ./internal/api/... ./internal/servers ./internal/memberships ./internal/auth ./internal/platform/httpdto ./internal/platform/validation`.
- **Unit Testing agent tasks — Comrade Bot:**
  - Update TypeScript types and run `npm run build`.
  - Validate command registry with `npm run commands:validate` if available in `package.json`; otherwise run existing build/deploy validation flow.
  - Add/update tests if harness exists for pure decision/render helpers. If not, manually mock `ApiService.getUserDetails`, `initiateServerRegistration`, and `generateSignedLink` for:
    - DM blocked;
    - unregistered admin CTA;
    - already initialized server;
    - eligible setup opens one-field modal;
    - success shows desktop recommendation and no raw VA UUID;
    - duplicate VA code error.
  - Verify all `/initserver` replies remain ephemeral.
- **Vizburo/manual verification:**
  - After init, verify the initiating admin can open dashboard/setup link.
  - Verify datasource/admin routes render for the new minimal VA even when Airtable is not configured.
  - Verify pilots using `/register` are not prompted for VA linking until setup prerequisites are actually satisfied by existing status logic.

## 16. Execution order for specialized agents
1. **Swagger/OpenAPI agent:** revise `/server/init` schema/description and generated-code implications.
2. **Backend agent:** implement minimal init in `internal/servers`, preserve route/DI boundaries, update focused tests.
3. **Bot agent:** simplify `/initserver` UX/modal/API/types/help and add desktop/desktop-view setup copy.
4. **Vizburo/frontend agent:** verify or add web setup/profile/checklist surface for moved fields, keeping handlers thin and styling token-based.
5. **Unit Testing agent:** complete backend/bot/Vizburo test coverage and manual verification checklist.
6. **Observability agent:** review low-cardinality logs/metrics and confirm no infra scrape changes are needed.
7. **Docs/rollout agent:** update help/docs and redeploy slash commands.

## 17. Out-of-scope items
- No full Discord admin setup wizard.
- No Discord collection of VA name, callsign prefix/suffix, Airtable credentials, schema mappings, event/PIREP/livery/webhook config.
- No Discord role creation/assignment/provisioning.
- No generated setup-token lifecycle unless separately prioritized.
- No Live API validation of callsign patterns during `/initserver`.
- No polling workers, background verification jobs, or new infra stack.
- No deletion or migration of existing VA configs created by earlier `/initserver` flows.

## 18. Final checklist
- [x] Planner inspected actual `politburo/`, `comrade-bot/`, and relevant `labour-bureau` guidance/context.
- [x] Source modifications avoided by this planner.
- [x] Created exactly one planning document.
- [x] Plan file path: `politburo/plans/discord-initserver-minimal-web-setup-plan.md`.
- [x] Key downstream tasks identified: OpenAPI `/server/init` minimal schema, backend minimal VA creation, bot `/initserver` one-field UX with desktop recommendation, Vizburo setup ownership, tests, observability, docs/rollout.
