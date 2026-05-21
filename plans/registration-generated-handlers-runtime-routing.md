# Registration Generated Handlers Runtime Routing — Implementation Plan

## 1. Title and status
- **Status:** Implemented
- **Plan file:** `politburo/plans/registration-generated-handlers-runtime-routing.md`
- **Date:** 2026-05-21
- **Requested change summary:** The bot-facing registration/onboarding endpoints were expected to run through the OpenAPI-generated Chi/strict-server handlers, but `internal/routes/router.go` currently mounts handwritten feature handlers directly. Migrate the active `/api/v1` registration/onboarding routes to the generated handler path and rename pilot registration from `/pilots/register` to `/user/register`.
- **Scope:** Runtime routing for the existing registration OpenAPI domain: `POST /user/register`, `POST /server/init`, `POST /memberships/join`, `GET /user/status`, and `POST /signed-link` under `/api/v1`.
- **Assumptions:** The existing feature handlers remain the business-logic source of truth during this migration; the immediate goal is generated runtime routing, not a full rewrite of handler internals into generated request/response methods.

## 2. Context
- **Files/packages inspected:**
  - Workspace guidance: `AGENTS.md`, `politburo/CLAUDE.md`
  - Router and middleware: `internal/routes/router.go`, `internal/middleware/auth.go`, `internal/middleware/discord_context.go`, `internal/middleware/metrics.go`
  - DI: `internal/app/app.go`
  - Response helpers: `internal/platform/httpdto/response.go`
  - Active handlers: `internal/pilots/handler.go`, `internal/memberships/handler.go`, `internal/servers/handler.go`, `internal/auth/handler.go`
  - Generated contract and adapter: `api/openapi/registration.yaml`, `api/openapi/registration.cfg.yaml`, `internal/api/generated/registration/server.gen.go`, `internal/api/registration/server.go`, `internal/api/registration/server_test.go`
  - Bot callers: `comrade-bot/src/services/apiService.ts`, `comrade-bot/src/helpers/utils.ts`
  - Related prior plan: `politburo/plans/registration-generated-code-tests.md`
- **Existing behavior and architecture summary:**
  - `internal/routes/router.go` applies `AuthMiddleware` to `/api/v1`, then applies `RequireDiscordBotContextMiddleware()` to a bot subgroup.
  - Lines observed in `router.go` directly mount handwritten handlers: `bot.Get("/user/status", application.Features.MembershipsHandler.GetUserStatus())`, `bot.Post("/pilots/register", application.Features.PilotsHandler.RegisterPilot())`, `bot.Post("/server/init", application.Features.ServersHandler.InitServer())`, `bot.Post("/memberships/join", application.Features.MembershipsHandler.JoinVA())`, and `bot.Post("/signed-link", application.Features.AuthHandler.GenerateSignedLink())`.
  - `internal/api/generated/registration/server.gen.go` currently contains generated Chi route registration for `/pilots/register`, `/server/init`, `/memberships/join`, `/user/status`, and `/signed-link`, plus `NewStrictHandler` and `StrictServerInterface`. This must be regenerated from an OpenAPI path change so registration becomes `/user/register`.
  - `internal/api/registration/server.go` already implements `registrationgen.StrictServerInterface`, but it delegates to the handwritten handlers by synthesizing an in-memory request via `httptest.NewRecorder()`.
  - `internal/api/registration/server_test.go` exercises the generated strict server path, but production routing does not currently use it.
  - Global `MetricsMiddleware` is already mounted in `router.go`, and `/metrics` is exposed by `internal/runtime/server.go`.
- **Relevant repo guidance discovered:**
  - DI must flow through `internal/app/app.go`; route registration belongs in `internal/routes/router.go`.
  - API JSON responses use `internal/platform/httpdto` envelopes.
  - Generated output under `internal/api/generated/**` must not be hand-edited; `make generate-api` regenerates from `api/openapi/registration.yaml` using `registration.cfg.yaml`.

## 3. Existing reuse
- Reuse `internal/api/registration.NewServer(...)` as the handwritten adapter from existing feature handlers to the generated strict interface.
- Reuse `registrationgen.NewStrictHandler(...)` plus `registrationgen.HandlerFromMux(...)` or `HandlerWithOptions(...)` from `internal/api/generated/registration/server.gen.go` for runtime route mounting.
- Reuse the existing `/api/v1` auth stack in `internal/routes/router.go`: `AuthMiddleware` followed by `RequireDiscordBotContextMiddleware()` for bot-facing routes.
- Reuse `httpdto.WriteSuccess`, `WriteError`, and `WriteValidationError` semantics; generated runtime routing must not change the envelope contract.
- Reuse `internal/api/registration/server_test.go` patterns, then add route-level coverage that proves the production router traverses the generated path.

## 4. Architecture decisions
- **Decision:** The production router MUST mount the registration/onboarding endpoints through the generated Chi server path, not five direct `bot.Get/Post` calls to feature handlers.
- **Decision:** The canonical registration endpoint MUST be `POST /api/v1/user/register`; `/api/v1/pilots/register` is the currently observed legacy path and should not remain the canonical generated path.
- **Decision:** Keep the generated-code boundary under `internal/api/generated/registration/` and the handwritten adapter under `internal/api/registration/`; do not move generated code into feature packages.
- **Decision:** The first migration SHOULD preserve the current adapter pattern, where generated strict methods delegate to existing feature handlers, because those handlers already own validation, claims extraction, logging, and `httpdto` responses.
- **Decision:** The generated registration route mounting MUST stay inside the existing `/api/v1` authenticated and Discord-bot-context middleware stack so `auth.GetUserClaims` and header requirements remain unchanged.
- **Decision:** Do not bypass DI. `routes.NewRouter(application *app.App)` should construct the adapter from `application.Features.*Handler` values or receive a DI-initialized adapter added to `app.FeatureDeps`; choose the smaller change only after implementation confirms import-cycle safety.
- **Alternative considered:** Rewriting all registration handlers as native strict-server implementations immediately. Rejected for this slice because it would duplicate or move business logic across `internal/pilots`, `internal/memberships`, `internal/servers`, and `internal/auth` instead of fixing the routing discrepancy.
- **Open questions / risks:**
  - `internal/api/registration/server.go` currently uses `httptest` in production adapter code. That may be acceptable as a bridge, but implementation should evaluate replacing it with a small `ResponseRecorder`-style internal capture or moving logic into explicit adapter methods later.
  - Generated request decode errors currently use generated default handlers unless custom strict options are supplied; route-level tests must verify bad JSON/status behavior remains acceptable.

## 5. Repo-by-repo implementation plan

### politburo/
- Update `api/openapi/registration.yaml` so the pilot registration operation path is `/user/register` while retaining the existing `registerPilot` operation ID unless the Swagger/OpenAPI agent finds a generated naming conflict.
- Regenerate `internal/api/generated/registration/server.gen.go` with `make generate-api` after the spec path rename; do not hand-edit generated output.
- Update `internal/api/registration/server.go` adapter request path from `/api/v1/pilots/register` to `/api/v1/user/register` if it still synthesizes an internal HTTP request for the delegated handler.
- Update `internal/routes/router.go` so the bot subgroup mounts generated registration routes instead of direct handwritten route registrations for the five OpenAPI-covered endpoints.
- Import the handwritten adapter package (`internal/api/registration`) and generated package (`internal/api/generated/registration`) only in routing/adapter layers; avoid importing generated code into domain feature packages.
- Construct the strict server from existing DI handlers:
  - `registration.NewServer(application.Features.PilotsHandler, application.Features.MembershipsHandler, application.Features.ServersHandler, application.Features.AuthHandler)`
  - wrap with `registrationgen.NewStrictHandler(...)`
  - mount into the `bot` Chi router with generated `BaseURL` appropriate for the current router nesting.
- Remove only the direct route calls that are replaced by generated mounting; leave unrelated `/api/v1` routes untouched.
- Add/adjust tests:
  - route-level test proving `routes.NewRouter` reaches generated registration handlers for all five endpoints;
  - regression test that generated route paths remain under `/api/v1`, not duplicated as `/api/v1/api/v1` or stripped incorrectly;
  - regression test that `POST /api/v1/user/register` is the working registration route;
  - tests for auth and Discord context behavior through the production router, not only the adapter package.

### comrade-bot/
- Update existing calls in `src/services/apiService.ts` so registration targets `/api/v1/user/register` instead of `/api/v1/pilots/register`; keep `/api/v1/server/init`, `/api/v1/user/status`, `/api/v1/signed-link`, and `/api/v1/memberships/join` unchanged unless spec verification finds another mismatch.
- Keep bot headers centralized in `src/helpers/utils.ts`; no command should call backend endpoints without the API key and Discord context headers.

### Vizburo UI
- Not applicable for UI rendering/styling. The covered endpoints are bot-facing JSON APIs plus signed-link generation.
- Preserve signed-link behavior because it is the bridge into Vizburo/browser flows.

### labour-bureau/
- Not applicable for this routing change. No Compose, Prometheus, Promtail, or deployment wiring changes are expected.

### API contracts / generated clients / shared configuration
- `api/openapi/registration.yaml` and `registration.cfg.yaml` already exist and generate `internal/api/generated/registration/server.gen.go`.
- If route mounting exposes a mismatch between generated models and handler DTOs, update the OpenAPI spec first, run `make generate-api`, and never hand-edit `internal/api/generated/registration/server.gen.go`.
- No shared configuration changes are expected.

## 6. Developer guidelines for implementation agents
- **Boundary rules:**
  - MUST NOT hand-edit `internal/api/generated/registration/server.gen.go`.
  - MUST keep all production routes registered via `internal/routes/router.go` and DI via `internal/app/app.go`.
  - MUST keep feature handlers thin and domain-owned; do not move business logic into routing code.
  - MUST preserve `httpdto` envelope behavior unless a spec update explicitly changes it.
- **Files likely to edit:**
  - `internal/routes/router.go`
  - `internal/api/registration/server.go` only if adapter behavior needs route-safe options or recorder cleanup
  - `internal/api/registration/server_test.go`
  - route tests under `internal/routes/` if an existing router test file is available
  - `api/openapi/registration.yaml` and generated output for the `/user/register` path rename
  - `comrade-bot/src/services/apiService.ts` for the backend path rename
- **Files/packages to avoid:**
  - Avoid unrelated packages under `internal/services`, `internal/common`, jobs, Watermill, PIREP, events, Vizburo templates, CSS, or infra.
  - Avoid changing `comrade-bot` commands unless backend compatibility tests prove a real contract change.
- **Sequencing recommendations:**
  1. Add route-level failing test proving current router does not use/generated path as expected, or at minimum asserting generated path behavior through `routes.NewRouter`.
  2. Update the OpenAPI path to `/user/register` and regenerate generated code.
  3. Change router mounting to generated strict handler.
  4. Update comrade-bot’s registration API call path.
  5. Run focused registration/OpenAPI tests.

## 7. Auth scopes, claims, and context
- **Required auth:** All five endpoints remain under `/api/v1` and require `AuthMiddleware`.
- **Discord bot context:** All five endpoints must continue to require `X-Discord-User-Id` and `X-Discord-Server-Id` via `RequireDiscordBotContextMiddleware()`.
- **Claims propagation:** Generated mounting must preserve the original request context so `auth.GetUserClaims(r.Context())` works in delegated handlers.
- **VA context handling:**
  - `RegisterPilot` uses Discord user/server IDs and rejects already-registered users from claims; its canonical route should be `/api/v1/user/register`.
  - `InitServer`, `JoinVA`, `GetUserStatus`, and `GenerateSignedLink` depend on Discord server/user context and/or VA lookup semantics through existing services.
- **Scopes/roles:** No new role or scope is introduced. Do not add staff/admin middleware to this bot onboarding group.
- **Mobile classification/impact:** Backend-only / low mobile impact. Signed links must remain browser-compatible for mobile and desktop clients; no mobile UI work is required.

## 8. Migrations and data model
- **Not applicable:** This is a routing/runtime integration change. No schema migration, backfill, data compatibility, or rollback data plan is expected.
- **Rollback consideration:** Reverting the router mount to direct handlers should restore prior behavior if generated routing introduces production issues, but the implementation must decide whether the legacy `/api/v1/pilots/register` path remains as a temporary compatibility alias or is removed immediately.

## 9. Error handling and response conventions
- Preserve existing `httpdto` envelope shapes:
  - success: `status`, `result`, `responseTimeMs`
  - error: `status`, `error.code`, `error.message`, `responseTimeMs`
  - validation: `error.code = VALIDATION_FAILED`, optional `error.fields`
- Verify these statuses remain unchanged for generated runtime routes:
  - `201` for register/init/join success
  - `200` for user status and signed link success
  - `401` when authentication claims are missing/invalid
  - `403` when Discord context headers are missing
  - `409` for already registered/member/server conflicts
  - `422` for handler validation failures
  - `500` for existing service/internal failures
- If generated request decoding returns `400` before delegated handler validation, document and test the distinction between malformed JSON (`400`) and structurally valid but invalid payloads (`422`).
- If a temporary `/api/v1/pilots/register` compatibility alias is kept, it must return the same envelope/status behavior as `/api/v1/user/register` and should be explicitly marked transitional in tests/docs.

## 10. Constants and configuration
- **Env vars:** No new env vars.
- **Existing relevant env/config:** `UI_BASE_URL` may affect signed-link tests; keep tests explicit.
- **Headers/constants:** Keep using `middleware.DiscordUserIDHeader` and `middleware.DiscordServerIDHeader`; do not introduce new header names. `comrade-bot/src/helpers/utils.ts` currently emits both legacy and canonical Discord headers.
- **Secrets:** No secret handling changes.

## 11. Logging and monitoring
- **Observed logging:** Existing feature handlers log registration, membership, server init, auth, validation, and errors. Generated routing should not add sensitive request-body logging.
- **Observed metrics:** `internal/routes/router.go` already mounts `middleware.MetricsMiddleware(application.Infra.MetricsReg)` globally; `/metrics` is exposed in `internal/runtime/server.go`.
- **Observability agent tasks:**
  - Verify generated route labels in `MetricsMiddleware` remain bounded and meaningful; avoid high-cardinality labels from raw IDs.
  - Confirm no new Prometheus scrape targets, Docker labels, Promtail paths, or dashboard panels are required.
  - Compare metrics labels before/after routing migration; generated route mounting should not fragment route labels unexpectedly.
  - Confirm logs still omit API keys, signed tokens, IFC credentials beyond existing safe IDs, and raw request bodies.
- **Monitoring gaps:** No new alerting expected. If route labels change, update dashboards only if existing panels depend on route strings.

## 12. API spec and generated code work
- **Swagger/OpenAPI agent tasks:**
  - Re-verify `api/openapi/registration.yaml` paths, operation IDs, schemas, response envelopes, and security declarations against the active handlers.
  - Rename the registration path from `/pilots/register` to `/user/register` in `api/openapi/registration.yaml` and verify generated Chi route registration reflects the new path.
  - Confirm `registration.cfg.yaml` still generates strict server + Chi server to `internal/api/generated/registration/server.gen.go`.
  - If changes are required, run `make generate-api` from `politburo/` and review generated diffs; do not hand-edit generated output.
  - Ensure operation IDs remain stable where possible: `registerPilot`, `initServer`, `joinMembership`, `getUserStatus`, `generateSignedLink`.
  - Confirm generated errors and validation schemas match `httpdto` and `internal/platform/validation` behavior.
- **make generate-api implications:** Required because the canonical registration path changes from `/pilots/register` to `/user/register`.

## 13. Documentation
- Update `politburo/CLAUDE.md` after implementation if it currently implies generated coverage is test-only or if routing conventions need to state that registration/onboarding routes mount through generated handlers.
- Update developer-facing docs for the registration endpoint rename from `/pilots/register` to `/user/register`; user-facing bot behavior should remain unchanged if comrade-bot is updated in the same slice.
- Downstream docs maintainer should note the developer-facing convention: OpenAPI-covered JSON domains should be mounted via generated handlers where available.

## 14. Frontend/Vizburo plan
- **Not applicable:** No Vizburo handler/template/CSS change.
- **Thin handlers / no direct infra:** Existing Vizburo handlers are not touched.
- **Design-system CSS tokens:** Not applicable.
- **No polling:** Not applicable; this change only affects bot-facing JSON route mounting.
- **Mobile behavior:** No UI behavior change; signed dashboard URLs should continue to work on mobile browsers.

## 15. Testing plan
- **Unit Testing agent tasks:**
  - Add/extend `internal/api/registration` tests to cover generated strict server behavior for all five endpoints, using `/user/register` for registration.
  - Add route-level tests proving `routes.NewRouter` mounts the generated registration handler path under `/api/v1` with auth and Discord context middleware preserved.
  - Add negative tests for missing API key (`401`), missing Discord context (`403`), malformed JSON (`400` if generated decoder catches it), and validation failure (`422`).
  - Add regression tests for response envelopes using `status`, `result`/`error`, and `responseTimeMs`.
  - Add a path-rename regression test that fails if generated routes still expose only `/pilots/register`.
  - Keep fakes/stubs package-local; avoid introducing broad test-only global state.
- **Integration/manual verification:**
  - Run focused command from repo guidance: `go test ./internal/api/... ./internal/pilots ./internal/memberships ./internal/servers ./internal/auth ./internal/platform/httpdto ./internal/platform/validation`.
  - Run any existing route tests under `./internal/routes`.
  - Manual smoke: call each `/api/v1` endpoint with bot headers against local Politburo and confirm status/envelope compatibility for comrade-bot.
- **Bot/UI/infra tests:** Run `npm run build` from `comrade-bot/` if `src/services/apiService.ts` changes for the `/user/register` path; Vizburo/infra tests are not required for this migration.

## 16. Execution order for specialized agents
1. **OpenAPI/spec agent:** Rename `/pilots/register` to `/user/register` in the registration spec, verify schemas/security, and regenerate generated code.
2. **Developer agent:** Change route mounting to generated strict handler, update the registration adapter path, update comrade-bot’s registration API URL, and preserve middleware/DI conventions.
3. **Unit Testing agent:** Add route, adapter, and path-rename regression coverage; run focused tests.
4. **Observability agent:** Confirm route metric labels/log privacy remain acceptable; no infra change unless label drift impacts dashboards.
5. **Docs maintainer:** Update developer-facing guidance only after implementation lands.

## 17. Out-of-scope items
- Do not rewrite registration, membership, server initialization, or signed-link business logic.
- Do not add new onboarding endpoints.
- Do not change comrade-bot commands, interaction routing, or user copy unless a contract regression forces it.
- Do not change database schema, migrations, jobs, Watermill workers, PIREP flows, events, Vizburo pages, or CSS.
- Do not introduce polling or new infrastructure services.
- Do not remove legacy packages as part of this routing fix.

## 18. Final checklist
- [x] Planner avoided source/config/test/generated/docs modifications outside this plan.
- [x] Plan file path: `politburo/plans/registration-generated-handlers-runtime-routing.md`.
- [x] Key downstream task: route registration must use generated handlers at runtime.
- [x] Key downstream task: preserve auth, Discord context, VA context, `httpdto` envelopes, and all endpoint URLs except the intentional `/user/register` rename.
- [x] Key downstream task: add route-level tests proving production router uses the generated registration path.
- [x] Key downstream agents: OpenAPI/spec, Developer, Unit Testing, Observability, Docs Maintainer.
- [x] Implementation note: legacy `/api/v1/pilots/register` was removed rather than kept as a compatibility alias.
- [x] Follow-up planning note: comrade-bot still uses handwritten `src/services/apiService.ts`; a separate generated TypeScript client migration should use `politburo/api/openapi/registration.yaml` and the Swagger/OpenAPI tooling surfaced through `labour-bureau/docker-compose.dev.yml`'s Swagger Editor service.
