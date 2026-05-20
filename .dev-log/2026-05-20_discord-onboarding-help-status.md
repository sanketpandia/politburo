# Discord onboarding/help/status implementation log

## Backend status API unit
- Logical unit / commit intent: enrich `GET /api/v1/user/status` so normal onboarding states return explicit status data, including unregistered users and current Discord server VA context.
- Changed files: `api/openapi/registration.yaml`, `internal/memberships/dto.go`, `internal/memberships/handler.go`, `internal/memberships/service.go`, `internal/memberships/handler_test.go`, `internal/api/registration/server_test.go`.
- Reused code / patterns / components: existing `auth.UserClaims`, platform `UsersSvc`, `VASvc`, `MembershipsSvc.GetUserStatusByUserID`, `httpdto.WriteSuccess/WriteError`, existing membership handler test fakes.
- Logging added or affected: expanded status request logs with low-cardinality `discord_server_id` and resolved unregistered state; no API keys, callsigns, IF routes, or health internals added.
- Metrics added or affected: none; observability follow-up can add low-cardinality onboarding status counters through `infra/metrics.MetricsRegistry` if desired.
- Test surface touched or still needed: added handler coverage for unregistered explicit 200, registered linked current-VA state, and unauthorized status request; updated generated-adapter test stub for the enriched status DTO. Service/repo composition integration coverage remains useful follow-up.
- Build/test command(s) run and status: `gofmt -w ...` succeeded; `make generate-api` failed because `oapi-codegen` is not installed on PATH; `go test ./internal/memberships` passed; `go test ./internal/api/... ./internal/pilots ./internal/memberships ./internal/servers ./internal/auth ./internal/platform/httpdto ./internal/platform/validation` passed after adapter test update.
- Deviations from plan, if any: OpenAPI codegen could not be completed locally due missing `oapi-codegen`; generated files were not hand-edited.
- Blast-radius notes / dependent surfaces checked: inspected auth claims/header propagation, router registration, app DI wiring, platform membership/user/VA services and repos, registration adapter, existing generated registration types, and current bot status/register/service consumers.
- Live API compliance notes: no changes to `POST /api/v1/pilots/register`; account creation remains IF Community ID + last-flight only. No callsign storage added to account creation. `POST /api/v1/memberships/join` unchanged and VA-scoped callsign validation remains in place.
- Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents: Swagger/OpenAPI agent must run `make generate-api` in an environment with `oapi-codegen` and review generated strict server diffs; Observability can add counters/log review; Unit Testing should add deeper service/repo cases for server configured/not configured, registered unlinked, and multiple affiliations.

## Comrade Bot onboarding/status/help unit
- Logical unit / commit intent: update bot command behavior to consume enriched status, split user `/status` from operational `/botstatus`, catalog-drive `/help`, and remove user-facing `/membership` routing/deployment.
- Changed files: `comrade-bot/src/types/Responses.ts`, `src/services/apiService.ts`, `src/commands/{register.ts,registerModalHandler.ts,status.ts,botstatus.ts,help.ts,helpCatalog.ts,registry.ts}`, `src/handlers/InteractionRouter.ts`, `src/helpers/messageFormatter.ts`.
- Reused code / patterns / components: existing `ApiService` central HTTP boundary, Discord interaction wrapper/meta headers, existing register button/modal IDs, membership join API service method for internal VA linking, command registry/loader pattern, existing logger/errorFields sanitization.
- Logging added or affected: sanitized `getHealth` failure logging through logger/errorFields; registration modal logging no longer records last-flight route. No raw dependency health details displayed to users.
- Metrics added or affected: command registry now includes `/botstatus` and excludes `/membership`, so existing command metrics will naturally track botstatus and stop tracking user-facing membership slash usage after redeploy.
- Test surface touched or still needed: no TS unit test added; existing `npm test` suite run. Follow-up should mock `ApiService` and cover register decision tree and status rendering helpers.
- Build/test command(s) run and status: `npm run build` passed; `npm test` passed (5 tests).
- Deviations from plan, if any: retained internal membership join files/constants as reusable/backward-compatible source but removed slash command registry and interaction routing.
- Blast-radius notes / dependent surfaces checked: inspected command registry/loader, InteractionRouter, register buttons/modals, response types, status/help commands, API service, message formatter, and bot package scripts.
- Live API compliance notes: bot account creation modal still collects only IF Community username and last-flight route; callsign is only requested through VA-link modal after enriched status indicates current server is a configured VA.
- Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents: redeploy slash commands with environment-specific deploy script; Unit Testing should add mocked command-flow cases; Observability can review whether command-level metrics need dashboards/alerts.
