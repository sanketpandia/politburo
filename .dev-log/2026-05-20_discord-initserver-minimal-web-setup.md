# 2026-05-20 — Discord Initserver Minimal Web Setup

## Logical unit / commit intent
- Implement Phase 1 minimal `/server/init` backend/API contract and Phase 1 comrade-bot one-field `/initserver` UX from `plans/discord-initserver-minimal-web-setup-plan.md`.
- No commit was created in this session because the tool policy requires explicit user approval before committing.

## Changed files
- Politburo:
  - `api/openapi/registration.yaml`
  - `internal/api/generated/registration/server.gen.go` (generated via `go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 ...` after local `oapi-codegen` binary was unavailable)
  - `internal/api/registration/server.go`
  - `internal/api/registration/server_test.go`
  - `internal/servers/dto.go`
  - `internal/servers/errors.go`
  - `internal/servers/handler.go`
  - `internal/servers/handler_test.go`
  - `internal/servers/service.go`
- Comrade Bot:
  - `src/commands/initServer.ts`
  - `src/commands/initServerButtonHandler.ts`
  - `src/commands/initServerModalHandler.ts`
  - `src/commands/helpCatalog.ts`
  - `src/services/apiService.ts`
  - `src/types/Responses.ts`

## Reused code / patterns / components
- Reused existing `POST /api/v1/server/init` route through `application.Features.ServersHandler.InitServer()`.
- Reused scoped registration/onboarding Discord context headers via `generateRegistrationMetaHeaders()` and `RequireDiscordBotContextMiddleware()`.
- Reused `httpdto` response envelopes, existing `servers` handler/service shape, `va.Service.GetByDiscordServerID`, `va.Service.GetByCode`, `users.Repository.GetUserByDiscordID`, and admin membership creation with `roles.RoleAdmin`.
- Reused existing Discord command/modal patterns and centralized `ApiService`; no command-level fetch was added.

## Logging added or affected
- Backend success logging no longer logs internal VA UUID for init success; it logs `va_code` and `discord_server_id` consistent with the previous handler context.
- Bot execution logging for initserver now logs only `vaCode`; removed VA name/callsign pattern logging from this flow.
- No provider secrets, signed URLs, webhook URLs, or Airtable configuration are logged by these changes.

## Metrics added or affected
- No new metrics added in this slice. Follow-up Observability agent should decide whether to add low-cardinality initserver/setup metrics through `infra/metrics.MetricsRegistry`.

## Test surface touched or still needed
- Updated handler and generated-contract tests for minimal `{ va_code }` init request and safe response fields.
- Service-level persistence tests for duplicate VA code, no callsign config rows, and admin membership creation are still desirable if a DB-backed test harness is added/available.
- Vizburo readiness/basic setup tests remain future Phase 2/3 work.

## Build/test commands run and status
- `make generate-api` from `politburo/`: failed because `oapi-codegen` was not on `PATH` (`/bin/sh: line 1: oapi-codegen: command not found`).
- `go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 -config registration.cfg.yaml registration.yaml` from `politburo/api/openapi/`: passed and regenerated `internal/api/generated/registration/server.gen.go`.
- `go test ./internal/api/... ./internal/servers ./internal/memberships ./internal/auth ./internal/platform/httpdto ./internal/platform/validation` from `politburo/`: passed after removing stale generated-adapter 404 mapping for `/user/status`, which is not in the current OpenAPI response set.
- `npm run build` from `comrade-bot/`: passed.

## Deviations from plan, if any
- Backend does not generate `dashboard_url`/`setup_url` during init; those fields remain optional per plan, and bot still instructs admins to use `/dashboard`.
- `make generate-api` could not be used directly because the generator binary is missing; equivalent generation was performed with `go run`.
- Vizburo Phase 2/3 readiness and Basic Setup UI were not implemented in this logical unit.

## Blast-radius notes / dependent surfaces checked
- Checked existing header middleware/routing: `/server/init`, `/user/status`, `/pilots/register`, `/memberships/join`, and `/signed-link` are already under `RequireDiscordBotContextMiddleware()` after API-key auth.
- Checked comrade-bot header helper: registration/onboarding endpoints continue to use `X-Discord-User-Id`, `X-Discord-Server-Id`, and `X-API-Key`; unrelated bot endpoints were not migrated.
- Checked generated registration adapter and focused registration tests after OpenAPI/codegen changes.
- Existing uncommitted Politburo work was present before this slice (`.gitignore`, `CLAUDE.md`, `docs/bruno/Politburo.yml`, `internal/memberships/service.go`, untracked docs/plans/logs). Those were not intentionally modified by this slice except where already dirty in the worktree.

## Live API compliance notes
- No Live API calls were added or changed.
- Minimal init no longer writes callsign prefix/suffix; live-flight matching remains dependent on existing callsign config rows until Vizburo Basic Setup is implemented.

## Follow-up notes
- Swagger/OpenAPI: confirm generated code version choice or install/pin `oapi-codegen` so `make generate-api` works reproducibly.
- Observability: review low-cardinality initserver success/failure metrics/logs and 401 vs 403 visibility; no new metrics were added here.
- Unit Testing: add DB-backed service tests for duplicate VA code, no callsign config writes, and admin membership creation; add bot pure-flow tests if a command test harness is introduced.
- Vizburo: implement readiness computation and Basic Setup/checklist page so minimal VAs can become flight-matching ready from the web UI.
- Ops: slash command metadata changed; deployment owner should run the existing command deployment flow before release.

## Logical unit / commit intent
- Implement Phase 2/3 MVP Vizburo readiness and Basic Setup page/checklist so minimal VAs can add callsign matching from the dashboard.
- No commit was created in this session because the tool policy requires explicit user approval before committing.

## Changed files
- `internal/platform/va/readiness.go`
- `internal/platform/va/config_service.go`
- `internal/vaadmin/handler.go`
- `internal/app/app.go`
- `internal/routes/router.go`
- `templates/pages/vaadmin-index.html`
- `templates/pages/vaadmin-setup.html`
- `templates/partials/basic-setup-form.html`
- `templates/partials/setup-checklist.html`
- `templates/partials/callsign-test-result.html`

## Reused code / patterns / components
- Reused dashboard session/active-VA context through `auth.GetSessionData()` and `session.SessionData.GetActiveVA()`.
- Reused `internal/platform/va.ConfigService` and existing `va_configs` keys `callsign_prefix` / `callsign_suffix`.
- Reused `platformVA.Service` for VA display-name updates and added `ConfigService.SetConfigValue` so setup saves use the existing config repository while invalidating the VA config cache.
- Reused active dashboard layout, `secondary-nav`, HTMX `hx-post`/`hx-get` targeted swaps, `#global-spinner`, and design-system CSS variables/classes.

## Logging added or affected
- Added low-detail error logs for setup page load/save/checklist/test failures with `va_id` and error, but no provider secrets, webhook URLs, signed URLs, Discord IDs, or raw config blobs.
- Basic Setup success does not add a new structured success log yet; follow-up Observability agent can decide metric/log shape.

## Metrics added or affected
- No metrics added. Follow-up Observability agent should evaluate `setup_basic_identity_saved`, `setup_callsign_test_passed`, and `setup_callsign_test_failed` through the existing metrics registry if desired.

## Test surface touched or still needed
- Added compile coverage for new readiness/page handlers through package tests and server build.
- Still needed: unit tests for `ComputeSetupReadiness` and `CallsignMatches`; handler/form tests for no prefix/suffix validation and successful config save if UI test harness supports it.
- Manual HTMX checks still needed in a running Vizburo session.

## Build/test commands run and status
- `go test ./internal/platform/va ./internal/vaadmin ./internal/routes ./internal/app` from `politburo/`: passed.
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server` from `politburo/`: passed.
- Re-ran `go test ./internal/platform/va ./internal/vaadmin ./internal/routes ./internal/app && go build -buildvcs=false -o .air_tmp/main ./cmd/server` after adding config-cache invalidation: passed.
- `go test ./internal/api/... ./internal/servers ./internal/memberships ./internal/auth ./internal/platform/httpdto ./internal/platform/validation` from `politburo/`: passed.
- `npm run build` from `comrade-bot/`: passed.

## Deviations from plan, if any
- Readiness MVP currently computes `bootstrap_created` and `flight_matching_ready` only. Other optional modules are displayed as friendly optional checklist rows but not fully computed from provider schemas/webhooks yet.
- Basic Setup sample tester uses saved prefix/suffix, not unsaved form values.
- Display-name save updates the VA record but does not refresh the Redis session VA name immediately; a fresh session/login may be needed for navigation copy to reflect name changes.

## Blast-radius notes / dependent surfaces checked
- Added admin-only dashboard routes under existing `/dashboard/vaadmin` group protected by `IsAdminMiddleware()`.
- No API JSON endpoint was added for readiness; scope stays Vizburo-first as the plan recommended.
- No new jobs/workers, polling, infra access, or custom JavaScript were added.
- Templates use design-system CSS variables and existing button classes; no hardcoded colors were introduced in the new setup templates.

## Live API compliance notes
- No Live API calls added. Callsign test is a string match preview only.

## Follow-up notes
- Swagger/OpenAPI: none for Vizburo HTML/HTMX routes unless a future JSON readiness endpoint is added.
- Observability: decide whether setup save/test/checklist metrics are needed and add via `infra/metrics.MetricsRegistry` only.
- Unit Testing: add readiness and handler tests; manually verify HTMX partial swaps, validation errors, and mobile readability.
- Product/UI: expand readiness computation for Airtable schemas, flight modes, webhooks, events, and livery mapping once the optional-module states are prioritized.

## Logical unit / commit intent
- Make OpenAPI generation reproducible with the Go tool-managed `oapi-codegen` binary after `oapi-codegen` was added to the module `tool` block.

## Changed files
- `Makefile`
- `go.mod`
- `go.sum`
- `internal/api/generated/registration/server.gen.go`

## Reused code / patterns / components
- Reused the existing `generate-api` Make target and existing `api/openapi/registration.cfg.yaml`.
- Reused Go 1.25 `tool` directive already present in `go.mod` for `github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen`.

## Logging added or affected
- None.

## Metrics added or affected
- None.

## Test surface touched or still needed
- Regenerated OpenAPI code with the pinned Go tool version (`v2.7.0` from `go.mod`) instead of relying on an ambient binary.
- Go module checksums were refreshed because the first post-generation test run failed on missing `go.sum` entries for tool/dependency transitive modules.

## Build/test commands run and status
- `make generate-api` from `politburo/`: passed using `go tool oapi-codegen`.
- Initial post-generation validation failed due missing `go.sum` entries for transitive modules (`golang.org/x/net/html`, `golang.org/x/crypto/sha3`, `golang.org/x/sys/unix`, etc.).
- `go mod tidy` from `politburo/`: passed and refreshed `go.mod`/`go.sum` for the tool-managed dependency graph.
- `go test ./internal/api/... ./internal/servers ./internal/memberships ./internal/auth ./internal/platform/httpdto ./internal/platform/validation`: passed.
- `go test ./internal/platform/va ./internal/vaadmin ./internal/routes ./internal/app`: passed.
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`: passed.

## Deviations from plan, if any
- None for product behavior. This was build tooling only.

## Blast-radius notes / dependent surfaces checked
- The Make target now avoids dependence on a globally installed `oapi-codegen` binary.
- Generated registration code was refreshed with the module-pinned tool version; focused registration tests and server build passed afterward.

## Live API compliance notes
- Not applicable; no runtime API behavior changed.

## Follow-up notes
- Document for developers that `make generate-api` now requires Go tool support and should be run from `politburo/`.
