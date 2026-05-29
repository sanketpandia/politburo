# Discord Bot Required Headers Middleware — Implementation Plan

## 1. Title and status
- **Status:** Proposed
- **Plan file:** `politburo/plans/discord-bot-required-headers-middleware.md`
- **Date:** 2026-05-19
- **Requested change summary:** Add a Politburo middleware for bot-facing API-key routes that mandatorily requires renamed Discord context headers `X-Discord-User-Id` and `X-Discord-Server-Id`; if either header is missing/blank, return a JSON `403 Forbidden`. Apply this migration only to the Discord bot command endpoints represented by `api/openapi/registration.yaml`.
- **Scope:** Backend routing/middleware and contract/test updates for bot-facing registration/onboarding endpoints: `GET /api/v1/user/status`, `POST /api/v1/pilots/register`, `POST /api/v1/server/init`, `POST /api/v1/memberships/join`, and `POST /api/v1/signed-link`.
- **Assumptions:** The API key itself remains authenticated by `AuthMiddleware`; the new middleware is an authorization/context guard after valid auth, so the requested missing Discord context response is `403`, not `401`. For this change, only the five targeted registration/onboarding endpoints migrate to `X-Discord-User-Id` and `X-Discord-Server-Id`; other endpoints that still depend on the old `X-Discord-Id` / `X-Server-Id` behavior are intentionally left to fail until a later migration.

## 2. Context
- **Files/packages inspected:**
  - `politburo/CLAUDE.md`
  - `politburo/internal/middleware/auth.go`
  - `politburo/internal/middleware/{is_registered.go,is_member.go,is_staff.go,is_admin.go,is_god.go,metrics.go,ratelimit.go}`
  - `politburo/internal/auth/{claims.go,request_context.go,handler.go,service.go}`
  - `politburo/internal/platform/httpdto/response.go`
  - `politburo/internal/routes/{router.go,router_test.go,jobs.go}`
  - `politburo/api/openapi/{registration.yaml,registration.cfg.yaml}`
  - `politburo/docs/bruno/Politburo.yml`
  - `politburo/internal/api/registration/{server.go,server_test.go}`
  - `politburo/internal/api/generated/registration/server.gen.go`
  - `politburo/internal/pilots/handler.go`
  - `politburo/internal/servers/handler.go`
  - `politburo/internal/memberships/handler.go`
  - `comrade-bot/src/helpers/utils.ts`
  - `comrade-bot/src/services/apiService.ts`
- **Existing behavior and architecture summary:**
  - `internal/routes/router.go` mounts `/api/v1` with `middleware.AuthMiddleware(...)`, then registers bot-facing registration routes directly at lines 116-140.
  - `AuthMiddleware` accepts either a UI session cookie or an API key. For API-key auth, `tryAuthFromAPIKey` validates `X-API-Key` and then calls `auth.MakeClaimsFromApi(r.Context(), claimsRepo, serverID, userID)` with `serverID := r.Header.Get("X-Server-Id")` and `userID := r.Header.Get("X-Discord-Id")` even when those values are blank. The targeted plan changes this API-key auth-context extraction to use the new header names for the migrated bot flow.
  - The registration handlers assume Discord context exists after auth: `pilots.RegisterPilot`, `servers.InitServer`, `memberships.JoinVA`, `memberships.GetUserStatus`, and `auth.GenerateSignedLink` all read claims from context and then use `claims.DiscordUserID()`, `claims.DiscordServerID()`, and/or `claims.ServerID()`.
  - `api/openapi/registration.yaml` already documents all endpoints as bot-facing and declares `ApiKey`, `ServerId`, and `DiscordId` security schemes using old header names (`X-Server-Id`, `X-Discord-Id`), but the response lists currently describe missing auth headers as `401`, not the requested `403` for missing Discord context. The spec must be updated to the new names for this registration contract only.
  - `docs/bruno/Politburo.yml` currently defines collection-level inherited headers `X-Discord-Id`, `X-Server-Id`, and `X-API-Key`, with environment variables `D_User_id` and `D_Server_Id`. The registration folder docs also mention old header names. The collection must be updated for the targeted registration/onboarding calls so manual API testing uses the new names.
  - `internal/api/registration/server.go` is a generated strict-server adapter that forwards requests into the active handlers; it currently maps only statuses listed in the spec. Missing `403` responses in the spec/generated types will matter if tests exercise this path.
  - `comrade-bot/src/helpers/utils.ts` currently sends `X-Discord-Id`, `X-Server-Id`, and `X-API-Key` via `generateMetaHeaders`; the targeted registration command calls must move to `X-Discord-User-Id` and `X-Discord-Server-Id` for the five migrated endpoints. Broader bot/API calls should not be migrated in this plan.
- **Relevant repo guidance discovered:**
  - `CLAUDE.md` requires route registration through `internal/routes/router.go`, DI through `internal/app/app.go`, claims via `internal/auth/request_context.go`, API responses via `internal/platform/httpdto`, and focused registration validation with `go test ./internal/api/... ./internal/pilots ./internal/memberships ./internal/servers ./internal/auth ./internal/platform/httpdto ./internal/platform/validation`.

## 3. Existing reuse
- Reuse the middleware shape from `internal/middleware/is_registered.go` and sibling role middleware: `func X() func(http.Handler) http.Handler` returning an `http.HandlerFunc` wrapper.
- Reuse `internal/platform/httpdto.WriteError` for the response envelope: `{"status":"error","error":{"code":...,"message":...},"responseTimeMs":...}`.
- Reuse `logging.Warn`/`logging.Debug` patterns in `AuthMiddleware` and registration handlers, but do **not** log raw API keys.
- Reuse the route grouping pattern in `internal/routes/router.go` by wrapping only the intended route block rather than changing every `/api/v1` endpoint.
- Reuse the existing `registration.yaml` security scheme structure, but rename the header names to `X-Discord-Server-Id` and `X-Discord-User-Id` for the registration contract.
- Reuse the existing Bruno environment variables if possible (`D_User_id`, `D_Server_Id`) while changing header names, or rename variables only if the Bruno collection remains internally consistent.
- Reuse `comrade-bot/src/helpers/utils.ts` as the current central header helper evidence, but implementation must be careful not to migrate every bot endpoint if that helper is shared broadly.

## 4. Architecture decisions
- **Decision:** Add a new middleware in `internal/middleware/`, likely `RequireDiscordBotContextMiddleware()`, that checks `X-Discord-User-Id` and `X-Discord-Server-Id` request headers with `strings.TrimSpace` semantics and returns `403 Forbidden` when either is missing.
- **Decision:** Apply the middleware after `AuthMiddleware` and only to the bot command/registration route block in `internal/routes/router.go`, not globally to all `/api/v1` routes. This avoids breaking UI-session-backed API routes and routes whose auth context is not necessarily Discord-command initiated.
- **Decision:** Keep API key validation inside `AuthMiddleware`, but update the API-key auth-context extraction to populate claims from the new header names (`X-Discord-User-Id`, `X-Discord-Server-Id`) for the migrated flow. Do not add fallback to old header names for the targeted endpoints; old-header callers should fail so the remaining endpoint migrations can be handled later.
- **Decision:** The middleware should read headers directly rather than claims, because `AuthMiddleware` may populate claims with empty Discord fields today; direct header checks ensure the failure is about request context presence before handlers make domain lookups.
- **Decision:** Return one stable error code for missing context, e.g. `MISSING_DISCORD_CONTEXT`, with a message that names `X-Discord-User-Id` and `X-Discord-Server-Id`. Avoid separate high-cardinality error labels or per-user/server log values when headers are missing.
- **Decision:** Do not migrate any other Politburo endpoints in this change. Endpoints outside `registration.yaml` that still expect old header behavior may fail after the auth-context header rename; that is accepted and will be fixed in later endpoint-by-endpoint migrations.
- **Decision:** Update OpenAPI and generated adapter handling for `403` on all registration endpoints so the contract matches runtime behavior. Existing generated `server.gen.go` must remain generated-only.
- **Open question / risk:** The direct router still mounts handwritten feature handlers rather than `registrationgen.HandlerFromMux` in production. If implementation agents choose to move production mounting to generated strict handlers, that is a larger generated-route migration and should stay within existing registration generated-code plans; this change only requires compatible spec/generated handling.

## 5. Repo-by-repo implementation plan

### politburo/
- Add a new middleware file under `internal/middleware/`, e.g. `discord_context.go`:
  - Function signature should match existing middleware constructors.
  - Require both `X-Discord-User-Id` and `X-Discord-Server-Id` headers to be present and non-blank.
  - On failure, call `httpdto.WriteError(w, start, "MISSING_DISCORD_CONTEXT", "Missing required Discord context headers: X-Discord-User-Id and X-Discord-Server-Id", http.StatusForbidden)`.
  - Log a low-detail warning with method/path and booleans for which headers are present; do not log API key or raw sensitive values.
- In `internal/routes/router.go`, create a scoped group inside `/api/v1` after `AuthMiddleware`, for the registration/bot command endpoints:
  - `GET /user/status`
  - `POST /pilots/register`
  - `POST /server/init`
  - `POST /memberships/join`
  - `POST /signed-link`
  - Apply `bot.Use(middleware.RequireDiscordBotContextMiddleware())` to that group.
- Update `internal/middleware/auth.go` API-key context extraction:
  - Replace `r.Header.Get("X-Discord-Id")` with `r.Header.Get("X-Discord-User-Id")`.
  - Replace `r.Header.Get("X-Server-Id")` with `r.Header.Get("X-Discord-Server-Id")`.
  - Keep `X-API-Key` unchanged.
  - Do not add old-header fallback in this plan.
- Keep non-registration routes unchanged unless a downstream agent verifies they are bot-command-only and explicitly expands scope. Candidate future bot routes visible in `comrade-bot/src/services/apiService.ts` include PIREP, events/tours, pilot stats, and live flights, but they are **out of scope** for this requested `registration.yaml`-anchored change.
- Update route logs in `router.go` only if necessary to reflect the scoped middleware, without reformatting unrelated route declarations.

### comrade-bot/
- Add a narrowly scoped way for only the five migrated registration/onboarding API calls to send `X-Discord-User-Id`, `X-Discord-Server-Id`, and `X-API-Key`.
- Do **not** migrate every `ApiService` call just because `generateMetaHeaders` is shared. Either add a new registration-specific helper (for example `generateRegistrationMetaHeaders`) or update only the five call sites if the implementation can keep other endpoints on their current behavior.
- Optional test/verification only: add or adjust unit coverage for the new registration-specific header helper if a test suite exists, but do not change `MetaInfo` semantics.
- If bot error handling is touched, ensure `403` error envelope parsing uses `body.error.message`; current `ApiService.initiateRegistration` checks `body.message`, which may not surface `httpdto` errors correctly. This is a follow-on polish, not required for middleware correctness.

### Vizburo UI
- Not applicable for direct UI pages. The scoped middleware must not be applied to `/dashboard` or session-backed browser routes.
- Signed-link generation is bot-facing even though it creates a Vizburo login URL; require Discord headers there because `auth.GenerateSignedLink` uses Discord user/server IDs for lookup.

### labour-bureau/
- Not applicable for application code. No compose or runtime infra change is required for a new in-process middleware.

### API contracts / generated clients / shared configuration
- Update `api/openapi/registration.yaml`:
  - Add `403` response entries for all five operations with `ErrorResponse` schema.
  - Revise existing `401` descriptions to focus on missing/invalid `X-API-Key` authentication, and describe `403` as missing required Discord context headers.
  - Rename the `ServerId` security scheme header from `X-Server-Id` to `X-Discord-Server-Id`.
  - Rename the `DiscordId` security scheme header from `X-Discord-Id` to `X-Discord-User-Id`; consider renaming the scheme key itself to `DiscordUserId` only if doing so does not create unnecessary generated churn. The header name is the required behavior.
- Re-run the existing codegen path (`make generate-api` or repo-approved equivalent) so `internal/api/generated/registration/server.gen.go` includes `403` response object types. Do not hand-edit generated code.
- Update `internal/api/registration/server.go` to map `http.StatusForbidden` for each affected operation to the generated `403` JSON response type after regeneration.
- Update `docs/bruno/Politburo.yml` for the targeted collection behavior:
  - Replace collection-level inherited header names `X-Discord-Id` and `X-Server-Id` with `X-Discord-User-Id` and `X-Discord-Server-Id` if the collection remains focused on the registration/onboarding flow.
  - If the collection later contains non-migrated endpoints that must keep old headers, scope the new headers to the five Registration & Onboarding requests instead of changing global inherited headers.
  - Update request docs text that currently says `X-API-Key`, `X-Server-Id`, and `X-Discord-Id` to the new names.

## 6. Developer guidelines for implementation agents
- **Boundary rules:**
  - Do not bypass `AuthMiddleware`; the new middleware is additive and scoped.
  - Do not add package globals or initialize services outside `internal/app/app.go`.
  - Do not place this in `infra/`; request-context authorization middleware belongs in `internal/middleware`.
  - Do not hand-edit `internal/api/generated/registration/server.gen.go`.
  - Do not change comrade-bot header names globally; add the new names only for the targeted registration/onboarding calls.
- **Files likely to edit:**
  - `politburo/internal/middleware/discord_context.go` (new)
  - `politburo/internal/middleware/discord_context_test.go` (new)
  - `politburo/internal/routes/router.go`
  - `politburo/api/openapi/registration.yaml`
  - `politburo/internal/api/registration/server.go`
  - `politburo/internal/api/registration/server_test.go`
  - `politburo/docs/bruno/Politburo.yml`
  - `comrade-bot/src/helpers/utils.ts` or a registration-specific helper file if needed
  - `comrade-bot/src/services/apiService.ts` only for the five targeted call sites
  - Generated file via codegen only: `politburo/internal/api/generated/registration/server.gen.go`
- **Files/packages to avoid:**
  - Avoid adding old-header compatibility in `tryAuthFromAPIKey`; the desired behavior is to use the new header names and let non-migrated endpoints fail until later.
  - Avoid unrelated role middleware (`is_staff.go`, `is_member.go`, etc.).
  - Avoid Vizburo templates/CSS and dashboard handlers.
  - Avoid migrations, jobs, workers, queue code, and infra wiring.
- **Sequencing recommendations:**
  1. Add middleware and unit tests using the new header names.
  2. Scope route group in `router.go`.
  3. Update `AuthMiddleware` API-key claim extraction to read `X-Discord-User-Id` / `X-Discord-Server-Id`.
  4. Update OpenAPI header names plus `403` responses and regenerate API artifacts.
  5. Update Bruno collection headers/docs for the targeted registration calls.
  6. Update comrade-bot only for the five targeted calls.
  7. Update adapter/status tests.
  8. Run focused tests.

## 7. Auth scopes, claims, and context
- **Required auth:** `X-API-Key` remains required and validated by `AuthMiddleware` for `/api/v1`.
- **Required context headers for scoped bot routes:** `X-Discord-User-Id` and `X-Discord-Server-Id` must be non-blank.
- **Claims propagation:** `AuthMiddleware` should continue to set `auth.UserClaims` via `auth.SetUserClaims`, but the API-key branch must now derive `DiscordUIDVal` from `X-Discord-User-Id` and `DiscordServerIDVal` from `X-Discord-Server-Id`. The new middleware should not mutate claims.
- **Status codes:**
  - `401 Unauthorized`: no valid session/API key, invalid/inactive API key.
  - `403 Forbidden`: valid auth path but missing required Discord context headers for bot-command routes.
- **VA context handling:** `X-Discord-Server-Id` is the Discord guild/server ID used by `auth.MakeClaimsFromApi` and downstream VA lookups (`GetByDiscordServerID`). The middleware guards presence only; it does not verify that the server maps to a VA. Existing handlers/services keep handling unknown VA/server as `404` or domain errors.
- **Mobile classification/impact:** No native mobile client behavior observed. Discord mobile users interact through comrade-bot, which sends headers server-side; mobile Discord clients should not be impacted unless bot metadata is absent.

## 8. Migrations and data model
- Not applicable. No schema, model, migration, backfill, or rollback data work is required.
- Rollback is code/config only: remove middleware application and revert OpenAPI/generated response additions if necessary.

## 9. Error handling and response conventions
- Use `internal/platform/httpdto.WriteError`, not `http.Error`, to preserve the OpenAPI envelope.
- Suggested error:
  - HTTP status: `403`
  - Code: `MISSING_DISCORD_CONTEXT`
  - Message: `Missing required Discord context headers: X-Discord-User-Id and X-Discord-Server-Id`
- Validation behavior remains owned by handlers and `internal/platform/validation`; missing headers are not request-body validation errors and should not return `422`.
- Do not leak API key values or sensitive tokens into logs or response messages.

## 10. Constants and configuration
- No new env vars, config structs, or secrets are required.
- Header names can be local constants in the new middleware to avoid typos (`X-Discord-User-Id`, `X-Discord-Server-Id`). If shared constants already exist by implementation time, reuse them; none were observed in the inspected files.
- Do not change `comrade-bot`'s `API_KEY` or `API_URL`. Avoid changing `generateMetaHeaders` globally unless all non-target endpoints are intentionally allowed to fail under the new names; the safer implementation is a registration-specific helper.

## 11. Logging and monitoring
- **Observability agent tasks:**
  - Ensure middleware logs one structured warning for missing context with fields like `method`, `path`, `has_discord_id`, and `has_server_id`.
  - Do not log raw Discord IDs for missing-header failures; if present, avoid adding them because middleware logs can be high-volume auth noise.
  - Existing `internal/middleware/metrics.go` records status code if mounted; verify whether HTTP metrics middleware is mounted in runtime. If it remains unmounted, note that `403` visibility depends on access logs and any reverse-proxy metrics.
  - No new Prometheus metric is required unless the observability agent identifies an existing low-cardinality auth-failure counter pattern. If added, labels must be low-cardinality only (`reason=missing_discord_context`, `route`, `method`).
  - No scrape targets, Docker labels, dashboard panels, or alerts are required for this small middleware change unless existing monitoring expects explicit 4xx dashboards.
  - Privacy: treat API keys and Discord IDs as sensitive in logs; boolean presence is enough.

## 12. API spec and generated code work
- **Swagger/OpenAPI agent tasks:**
  - Update `api/openapi/registration.yaml` to declare `403` responses for each operation under the registration spec.
  - Rename registration security header names to `X-Discord-User-Id` and `X-Discord-Server-Id`.
  - Keep operation IDs unchanged: `registerPilot`, `initServer`, `joinMembership`, `getUserStatus`, `generateSignedLink`.
  - Reuse `#/components/schemas/ErrorResponse` for the `403` response schema.
  - Clarify response descriptions: `401` for missing/invalid API key authentication; `403` for missing Discord user/server headers.
  - Keep `ApiKey` as `X-API-Key`; update the existing Discord context schemes rather than inventing unrelated schemes.
  - Run the repo-approved generation flow so `internal/api/generated/registration/server.gen.go` reflects new response types.
  - Update handwritten adapter mapping in `internal/api/registration/server.go` only after generated types exist.
  - Include `make generate-api` implications in the implementation log; generated output must not be manually edited.

## 13. Documentation
- Downstream docs agent should update any developer-facing bot/API docs that list registration endpoint auth requirements, if such docs exist outside the OpenAPI spec.
- Update the Bruno collection at `docs/bruno/Politburo.yml` so manual requests for the five registration/onboarding endpoints send `X-Discord-User-Id` and `X-Discord-Server-Id` and no longer document the old names for that flow.
- User-facing docs are likely not needed because Discord users do not set these headers manually.
- If comrade-bot command help mentions generic unauthorized errors, no change is required unless implementation also improves 403 handling.

## 14. Frontend/Vizburo plan
- Not applicable for Vizburo UI rendering or CSS.
- Preserve thin Vizburo handlers and do not introduce UI direct infra access.
- No polling or browser behavior change is involved.
- Mobile behavior: Discord mobile users continue using bot commands; no CSS/design-system work is required.

## 15. Testing plan
- **Unit Testing agent tasks:**
  - Add focused middleware tests in `internal/middleware` covering:
    - both new headers present -> next handler called
    - missing `X-Discord-User-Id` -> `403` JSON error
    - missing `X-Discord-Server-Id` -> `403` JSON error
    - blank/whitespace header values -> `403`
    - old headers only (`X-Discord-Id`, `X-Server-Id`) -> `403` for the targeted middleware
    - existing request context/claims are not mutated
  - Add or adjust route-level tests so the scoped registration endpoints reject requests with valid auth context but missing Discord headers. If full `AuthMiddleware` dependencies are difficult to instantiate, test middleware composition directly plus adapter behavior.
  - Update `internal/api/registration/server_test.go`:
    - Existing `newComradeBotRequest` should continue passing headers.
    - Replace or augment `TestStrictServer_UnauthorizedWithoutBotHeaders`; after the new route/middleware path, missing bot headers should expect `403` when auth otherwise succeeds. Pure generated adapter tests without auth middleware may still produce handler-level `401` for missing claims; distinguish these cases clearly.
    - Add adapter mappings for `403` once OpenAPI/generated response types are present.
  - Add/adjust `AuthMiddleware` tests proving API-key claims are populated from `X-Discord-User-Id` and `X-Discord-Server-Id`, not the old names.
  - Regression-test that invalid/missing `X-API-Key` still returns `401` from `AuthMiddleware`, not `403`.
  - Run focused validation: `go test ./internal/api/... ./internal/pilots ./internal/memberships ./internal/servers ./internal/auth ./internal/platform/httpdto ./internal/platform/validation ./internal/middleware`.
  - If feasible, run `go test ./...` and document unrelated failures rather than masking them.
- **Manual verification:**
  - `curl` with valid API key and both new Discord headers reaches handler behavior.
  - `curl` with valid API key but missing either new Discord header returns `403` JSON envelope.
  - `curl` with valid API key and only old Discord headers returns `403` for the targeted registration routes.
  - Bruno collection registration requests use the new header names and can reproduce the same success/failure cases.
  - `curl` with missing/invalid API key returns `401`.

## 16. Execution order for specialized agents
1. **Swagger/OpenAPI agent:** Rename registration header schemes, add `403` responses to `registration.yaml`, and regenerate API artifacts, or coordinate with developer if generation is part of implementation.
2. **Plan-to-code developer:** Add middleware, update API-key auth-context extraction to new headers, route grouping, Bruno header updates, comrade-bot targeted header usage, adapter `403` mapping, and focused tests.
3. **Unit Testing agent:** Expand/validate middleware, route, generated adapter, and auth-regression tests.
4. **Observability agent:** Verify logs/metrics behavior for 403s; only add low-cardinality metrics if aligned with existing registry patterns.
5. **Feature docs maintainer:** Update developer/API docs only if documentation beyond OpenAPI exists.

## 17. Out-of-scope items
- Do not implement role/permission checks beyond requiring Discord context headers.
- Do not verify that the Discord user belongs to the Discord server; this middleware is a presence guard only.
- Do not require or migrate these new headers globally for every `/api/v1` endpoint.
- Do not preserve old header fallback for the five targeted registration endpoints.
- Do not change API-key storage, rotation, hashing, or `apikeys.Repository` behavior.
- Do not rewrite registration handlers/services or move production routing to generated strict-server handlers unless already required by another approved plan.
- Do not update migrations, jobs, workers, Vizburo UI, CSS, Docker, or labour-bureau infra.

## 18. Final checklist
- Source modifications avoided by this planner: yes; only this markdown plan was created.
- Plan file path: `politburo/plans/discord-bot-required-headers-middleware.md`
- Key downstream tasks:
  - Add `internal/middleware` Discord context guard returning `403` for missing `X-Discord-User-Id` / `X-Discord-Server-Id`.
  - Update API-key auth-context extraction to read the new headers.
  - Scope it to registration/bot command endpoints in `internal/routes/router.go`.
  - Update `api/openapi/registration.yaml` header names and `403` responses, regenerate generated server code, and update adapter mapping.
  - Update `docs/bruno/Politburo.yml` headers/docs for the targeted registration/onboarding requests.
  - Update comrade-bot only for the five targeted registration/onboarding calls.
  - Add middleware/route/adapter tests and verify `401` vs `403` behavior.
