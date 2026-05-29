# Registration OpenAPI Generated-Code Tests — Implementation Plan

## 1. Title and status
- **Status:** Approved
- **Plan file:** `politburo/plans/registration-generated-code-tests.md`
- **Requested change summary:** Verify whether the registration/onboarding work from `politburo/plans/watermill-login-registration-refactor.md` is missing viable unit tests, and prepare a repo-grounded plan to add tests for the generated registration OpenAPI code path plus post-generate Makefile calls.
- **Scope:** Registration/onboarding API testability in `politburo/`, specifically the spec-driven registration flow (`/pilots/register`, `/server/init`, `/memberships/join`, `/user/status`, `/signed-link`), the generated server artifact path, and the Makefile/test workflow needed after generation.
- **Assumptions:** The next implementation change may need to both finish the generated registration server path and add tests, because the generated Go output and adapter layer are not currently present in the repo.

## 2. Context
- **Files/packages inspected:**
  - `politburo/CLAUDE.md`
  - `politburo/plans/watermill-login-registration-refactor.md`
  - `politburo/Makefile`, `politburo/test.sh`, `politburo/go.mod`
  - `politburo/api/openapi/registration.yaml`, `politburo/api/openapi/registration.cfg.yaml`
  - `politburo/internal/routes/router.go`, `politburo/internal/routes/jobs.go`, `politburo/internal/routes/router_test.go`
  - `politburo/internal/pilots/{handler.go,dto.go,registration_service.go,handler_test.go}`
  - `politburo/internal/memberships/{handler.go,dto.go,service.go}`
  - `politburo/internal/servers/{handler.go,dto.go,service.go}`
  - `politburo/internal/auth/{handler.go,service.go,handler_test.go}`
  - `politburo/internal/platform/{httpdto/response.go,validation/decoder.go,validation/errors.go}`
  - `politburo/internal/middleware/{auth.go,ratelimit.go}`
  - `politburo/internal/middleware/metrics.go`
  - `politburo/internal/services/{registration_service_v2.go,registration_service_v2_test.go}`
  - `politburo/internal/runtime/server.go`
  - `politburo/infra/{logging/logger.go,metrics/metrics.go}`
  - `politburo/infra/messaging/{router.go,middleware.go}`
  - `politburo/internal/models/gorm/user.go`
  - `/.claude/commands/{architecture.md,oapi-codegen.md}` and `/.claude/agents/swagger.md`
- **Existing behavior and architecture summary:**
  - The registration endpoints are live in `internal/routes/router.go` and currently mount the existing feature handlers directly under `/api/v1` with `AuthMiddleware`.
  - The spec files exist (`api/openapi/registration.yaml` and `registration.cfg.yaml`), but the planned generated output `internal/api/generated/registration/server.gen.go` is absent, and there is no `internal/api/registration/server.go` adapter package yet.
  - `Makefile` currently defines `generate-api`, but it only shells out to `oapi-codegen`; it does not run any post-generation tests.
  - Observability is partially wired today:
    - Prometheus `/metrics` is exposed in `internal/runtime/server.go`.
    - The metrics registry is initialized in `internal/app/app.go`.
    - HTTP metrics middleware exists in `internal/middleware/metrics.go`, but it is not visibly mounted in `internal/routes/router.go`.
    - Watermill metrics middleware is mounted in `infra/messaging/router.go`.
    - Structured JSON logging is provided by `infra/logging/logger.go` and is already used throughout the registration handlers/services.
    - Rate-limit metrics are defined in `infra/metrics/metrics.go`, but the registration routes are not currently wrapped with `RateLimitMiddleware`, and the middleware itself does not increment those counters.
  - `go test ./...` currently fails in two registration-related areas:
    - `internal/pilots/handler_test.go` imports a non-existent `internal/testutil` package.
    - `internal/services/registration_service_v2_test.go` fails SQLite `AutoMigrate` because `internal/models/gorm/user.go` uses PostgreSQL-only `gen_random_uuid()` defaults.
- **Relevant repo guidance discovered:**
  - `CLAUDE.md` requires DI through `internal/app/app.go`, canonical routing via `internal/routes/router.go`, and API responses through `internal/platform/httpdto`.
  - `/.claude/commands/architecture.md` says new JSON REST endpoints are spec-driven.
  - `/.claude/commands/oapi-codegen.md` and `/.claude/agents/swagger.md` require `strict-server: true`, committed generated output, and Chi mounting through generated handlers rather than hand-editing generated files.

## 3. Existing reuse
- **Good test patterns to reuse:**
  - `internal/auth/handler_test.go` — uses `httptest`, package-local stubs, and no dead shared testutil package.
  - `internal/routes/router_test.go` — simple HTTP assertions against real handlers.
- **Runtime code to reuse as the source of truth:**
  - `internal/pilots/handler.go` and `internal/pilots/registration_service.go`
  - `internal/memberships/handler.go` and `internal/memberships/service.go`
  - `internal/servers/handler.go` and `internal/servers/service.go`
  - `internal/auth/handler.go` for signed-link and god-mode response behavior
  - `internal/platform/httpdto/response.go` and `internal/platform/validation/decoder.go`
- **Spec/codegen reuse already present:**
  - `api/openapi/registration.yaml`
  - `api/openapi/registration.cfg.yaml`
  - `Makefile` `generate-api` target as the existing entry point
- **Anti-patterns to avoid reusing:**
  - `internal/pilots/handler_test.go` in its current form (broken import, template-only assertions)
  - `internal/services/registration_service_v2_test.go` as the primary registration test base (targets legacy package and uses incompatible SQLite migration assumptions)

## 4. Architecture decisions
- **Decision:** Treat the active registration flow as the feature-layer code in `internal/pilots`, `internal/memberships`, `internal/servers`, and `internal/auth`; do not extend `internal/services/registration_service_v2*` as the long-term test surface.
- **Decision:** Generated output under `internal/api/generated/registration/` MUST remain generated-only. Tests SHOULD live alongside the handwritten adapter and router code, not inside the generated file.
- **Decision:** Because the current repo uses concrete `http.HandlerFunc` handlers and not service interfaces for these endpoints, generated-code tests SHOULD exercise the generated Chi/strict-server HTTP path rather than attempting deep unit mocking of the generated package itself.
- **Decision:** The post-generation Make flow SHOULD use a focused registration/OpenAPI test target first, because `go test ./...` is already red for existing registration-related reasons.
- **Decision:** The implementation change MUST either rewrite or retire the two broken registration test paths (`internal/pilots/handler_test.go` and `internal/services/registration_service_v2_test.go`) so that the new post-generate test target can actually pass.
- **Alternatives considered:**
  - Reusing the legacy `internal/services/registration_service_v2.go` tests was rejected because the active handlers now live elsewhere and the current test DB setup is not compatible with repo models.
  - Running full `go test ./...` immediately after generation was rejected as the only gate because the current repo already has unrelated red tests in the registration area.
- **Open questions / risks:**
  - `internal/api/generated/registration/server.gen.go` and `internal/api/registration/server.go` are both missing today; confirm whether they are added in the same implementation change or already generated in another branch before finalizing test package names.
  - `Makefile` assumes `oapi-codegen` is on `PATH`; `go.mod` currently pins `air` as a tool but does not pin `oapi-codegen`. The implementation agent must choose a repo-native pinning approach.

## 5. Repo-by-repo implementation plan

### politburo/
- **Generated code prerequisites**
  - Generate and commit `internal/api/generated/registration/server.gen.go` from `api/openapi/registration.yaml`.
  - Add the handwritten registration adapter package if still absent, expected at `internal/api/registration/server.go`, with a compile-time `var _ registrationgen.StrictServerInterface = ...` check.
- **Test additions / repairs**
  - Replace the broken `internal/pilots/handler_test.go` with real package-local tests that use `httptest` and context claims instead of the missing `internal/testutil` package.
  - Rewrite the active registration service coverage into `internal/pilots/registration_service_test.go` and stop relying on `internal/services/registration_service_v2_test.go` as the path that keeps `make test` green.
  - Add handwritten adapter/contract tests under `internal/api/registration/` for the generated registration server path.
  - Add route-level tests in `internal/routes/` if the generated registration routes are mounted in the production router.
  - Add focused tests for `internal/platform/httpdto` and `internal/platform/validation` because the OpenAPI spec is explicitly derived from those envelopes.
- **Build/test workflow**
  - Extend `Makefile` with a focused post-generation target, e.g. `generate-api-test` or `test-registration-generated`, and add the new targets to `.PHONY`.
  - Keep `generate-api` deterministic and pinned; do not depend on a manually installed ambient generator.

### comrade-bot/
- **Not applicable:** No bot code changes are required to add backend unit tests. The registration spec already lives in `politburo/api/openapi/registration.yaml`.

### Vizburo UI
- **Not applicable:** No dashboard/UI handler changes are required for this test-only registration work. Signed-link responses must remain contract-compatible, but no Vizburo rendering or CSS work is involved.

### labour-bureau/
- **Not applicable:** No infra/container change is required for generation or Go unit tests.

### API contracts / generated clients / shared configuration
- `api/openapi/registration.yaml` and `registration.cfg.yaml` remain the contract source.
- `internal/api/generated/registration/server.gen.go` is the generated artifact that this plan aims to test.
- `Makefile` is the shared developer entry point that MUST chain generation and focused tests for this flow.

## 6. Developer guidelines for implementation agents
- **Boundary rules**
  - Do not hand-edit `internal/api/generated/registration/server.gen.go`.
  - Do not introduce a new dead `internal/testutil` package just to satisfy the current broken pilot test.
  - Keep tests close to the active packages (`internal/api/registration`, `internal/pilots`, `internal/memberships`, `internal/servers`, `internal/auth`, `internal/platform/httpdto`, `internal/platform/validation`).
- **Files likely to edit**
  - `Makefile`
  - `go.mod` only if tool pinning for `oapi-codegen` is required
  - `api/openapi/registration.yaml` / `registration.cfg.yaml` only if test-driven contract mismatches are discovered
  - `internal/api/registration/server.go` and new `_test.go` files under `internal/api/registration/`
  - `internal/pilots/handler_test.go` and new `internal/pilots/registration_service_test.go`
  - New `_test.go` files under `internal/memberships`, `internal/servers`, `internal/auth`, `internal/platform/httpdto`, `internal/platform/validation`, and possibly `internal/routes`
- **Files/packages to avoid**
  - Avoid adding new behavior to `internal/services/registration_service_v2.go` unless the team explicitly decides that legacy package is still supported.
  - Avoid modifying unrelated Watermill/PIREP code, jobs, or UI packages.
- **Sequencing recommendations**
  - Pin and prove generation first.
  - Then add/repair focused tests.
  - Then wire the Makefile post-generate target.
  - Only then decide whether the broader `make test` target should also call the focused registration test chain.

## 7. Auth scopes, claims, and context
- **Required scopes / middleware observed in router:**
  - `POST /api/v1/pilots/register` — authenticated
  - `POST /api/v1/server/init` — authenticated
  - `POST /api/v1/memberships/join` — authenticated in router, with membership checks handled by claims/handler logic
  - `GET /api/v1/user/status` — authenticated
  - `POST /api/v1/signed-link` — authenticated
- **Context propagation requirements for tests:**
  - Registration tests MUST preserve `auth.GetUserClaims` behavior because the handlers derive `DiscordUserID`, `DiscordServerID`, `UserID`, and `Role` from request context.
  - Generated-route tests SHOULD exercise the API-key claim path or a thin claims-injecting test middleware so VA context is realistic.
- **VA context handling:**
  - `JoinVA`, `GetUserStatus`, `InitServer`, and `GenerateSignedLink` all depend on current server/VA context via `X-Server-Id`-derived claims or database lookups.
  - Tests SHOULD include at least one assertion that the same user behaves differently across “no VA yet”, “server already a VA”, and “already a member” states.
- **Mobile classification / impact:**
  - Low / backend-only. The only end-user browser touchpoint is signed-link generation; tests SHOULD keep redirect defaults (`/dashboard`) stable for desktop and mobile browsers alike.

## 8. Migrations and data model
- **Schema migrations:** Not applicable.
- **Data-model concern discovered:** The current legacy SQLite-based registration tests fail because repo GORM models use PostgreSQL defaults such as `gen_random_uuid()` (`internal/models/gorm/user.go`).
- **Implementation implication:** New registration tests SHOULD prefer fakes/stubs around active services and handlers, or a Postgres-compatible test strategy if a DB-backed test is truly required. Do not assume SQLite `AutoMigrate` is a drop-in fit for these models.

## 9. Error handling and response conventions
- Tests MUST pin the existing JSON envelope from `internal/platform/httpdto/response.go`:
  - success: `status`, `result`, `responseTimeMs`
  - error: `status`, `error.code`, `error.message`, `responseTimeMs`
  - validation: `error.code = VALIDATION_FAILED` plus `error.fields`
- Generated registration adapter tests SHOULD explicitly cover:
  - 201 success for register/init/join
  - 200 success for status/signed-link
  - 401 unauthorized when claims are absent
  - 409 conflict branches already exposed by the handlers/services
  - 422 validation branches for request structs using `validation.DecodeAndValidate`
  - signed-link 404 / 500 branches from `internal/auth/handler.go`
- If request-validator middleware from the generated spec is mounted, tests MUST document whether bad payloads are rejected at validator time (400) or handler validation time (422) and keep that split consistent.

## 10. Constants and configuration
- **Env/config currently relevant to tests:**
  - `UI_BASE_URL` influences signed-link formatting in `internal/auth/handler.go`; tests should set the env var or forwarded headers explicitly.
- **Tooling configuration gap:**
  - `Makefile` currently calls bare `oapi-codegen`; `go.mod` does not currently pin the generator tool.
  - The implementation SHOULD pin the generator in a repo-managed way before depending on post-generate tests.
- **Secrets:** Not applicable; no new secrets or env vars are needed.

## 11. Logging and monitoring
- **Current observed wiring:**
  - `internal/runtime/server.go` registers the Prometheus scrape endpoint at `/metrics` for the API server.
  - `infra/metrics/metrics.go` defines HTTP, queue, sync-job, webhook, Watermill, and rate-limit metric families.
  - `internal/middleware/metrics.go` records `politburo_http_requests_total`, `politburo_http_request_duration_seconds`, and `politburo_http_requests_in_flight`, but this middleware is not currently mounted in `internal/routes/router.go`; downstream agents should verify whether that omission is intentional.
  - `infra/messaging/middleware.go` and `infra/messaging/router.go` actively wire Watermill handler duration/error metrics.
  - `infra/logging/logger.go` emits structured JSON logs via Zap.
  - Registration/onboarding code already logs meaningful events in:
    - `internal/pilots/handler.go`, `internal/pilots/registration_service.go`
    - `internal/memberships/handler.go`, `internal/memberships/service.go`
    - `internal/servers/handler.go`
    - `internal/auth/handler.go`, `internal/auth/service.go`
- **Current observed gaps:**
  - The registration HTTP path may not currently contribute to the HTTP Prometheus counters/histograms if `MetricsMiddleware` truly is not mounted.
  - `RateLimitRejectedTotal`, `RateLimitAllowed`, and `RateLimitThrottled` exist in the registry, but I did not find active increments in `internal/middleware/ratelimit.go`.
  - The registration routes in `internal/routes/router.go` are not currently wrapped with `RateLimitMiddleware`, despite the broader refactor plan discussing registration/read/submit groups.
- **Observability agent tasks:**
  - Confirm this change introduces no new scrape targets or Docker/container labels.
  - Verify whether `internal/middleware/metrics.go` should be mounted for API routes as part of the generated registration path; if not, document that registration contract tests are compile/behavior checks only and not metrics assertions.
  - If the implementation also revives registration rate limiting, ensure the existing rate-limit metrics are actually incremented and keep label cardinality bounded.
  - Verify new tests do not require asserting unstable log timestamps or sensitive fields.
  - Confirm unauthorized and validation test cases preserve current structured logging behavior without expanding log-cardinality.
  - Add at most focused assertions around observable side effects that are already wired today (for example, `/metrics` endpoint availability or stable response behavior), not brittle log-line snapshots.
- **Monitoring impact:** Not applicable for runtime dashboards/alerts; this is a test-coverage and Makefile change.

## 12. API spec and generated code work
- **Swagger/OpenAPI agent tasks:**
  - Re-verify that `api/openapi/registration.yaml` still matches the live response envelopes in `internal/platform/httpdto/response.go` and the request DTOs in `internal/pilots/dto.go`, `internal/memberships/dto.go`, `internal/servers/dto.go`, and `internal/auth/handler.go`.
  - Generate `internal/api/generated/registration/server.gen.go` from `registration.cfg.yaml`; commit the generated file.
  - If absent, add the handwritten `internal/api/registration/server.go` that implements `registrationgen.StrictServerInterface`.
  - If request validation middleware is added for the generated path, use the generated swagger loader (`GetSwagger()`) and keep auth/security declarations aligned with the existing API-key header contract.
  - Reflect generator/tool pinning in the Make workflow so `make generate-api` is reproducible.
- **make generate-api implications:**
  - Post-generate tests must run after generation, not before.
  - Generated code should be compiled by the focused test target so spec drift is caught immediately.

## 13. Documentation
- Update `politburo/CLAUDE.md` after implementation to:
  - note the preferred registration/OpenAPI test command,
  - remove or revise the stale “Test compilation errors” note once the broken registration tests are repaired,
  - mention generated registration server code if the adapter path is added.
- No user-facing docs are otherwise required.

## 14. Frontend/Vizburo plan
- **Not applicable:** No frontend/Vizburo handler or styling work is needed.
- **Contract preservation only:** Signed-link response shape (`url`, `expires_in`, `redirect_to`) must stay stable because Vizburo/browser entry depends on it.
- **No polling / direct infra access:** Not applicable.

## 15. Testing plan
- **Unit Testing agent tasks**
  - `internal/api/registration/server_test.go`
    - Mount the generated strict handler on a Chi mux and cover each registration operation through HTTP.
    - Assert response status codes, envelope shape, and selected body fields.
    - Include at least one request-validator/schema rejection case if validator middleware is part of the generated path.
  - `internal/pilots/registration_service_test.go`
    - Cover IFC already registered, IFC not found, no recent flights, flight mismatch, registration success, and VA-already-exists happy-path flag.
    - Use a fake `LiveAPIProvider`; avoid SQLite `AutoMigrate`.
  - `internal/pilots/handler_test.go`
    - Replace the broken template tests with real handler tests for missing claims, 422 validation, 409 already registered, and 201 success.
  - `internal/memberships/handler_test.go` and/or `service_test.go`
    - Cover already-member short-circuit, 422 missing callsign, `CALLSIGN_NOT_IN_AIRTABLE` enriched message behavior, and success.
  - `internal/servers/handler_test.go` and/or `service_test.go`
    - Cover missing claims, invalid callsign config, server already registered, user-not-registered, and success.
  - `internal/auth/handler_test.go`
    - Extend existing tests to cover `GenerateSignedLink` defaults, lookup failure, generation failure, and `VerifyGodMode` true/false cases.
  - `internal/platform/httpdto/response_test.go`
    - Lock the success, error, and validation envelope JSON shape that the spec is modeled on.
  - `internal/platform/validation/decoder_test.go`
    - Cover malformed JSON, missing required fields, and valid payloads.
- **Integration / contract tests**
  - Add a focused generated-route contract test that compiles and exercises `registrationgen.NewStrictHandler(...)` plus Chi mounting.
  - If production router registration is updated to use the generated registration adapter, add a regression test in `internal/routes/` for one full authenticated registration request path.
- **Manual verification**
  - Run `make generate-api`.
  - Run the focused post-generate target (see section 16 suggestions).
  - After broken registration tests are repaired/replaced, run `go test ./...` to confirm the repo-wide suite is green again.

## 16. Execution order for specialized agents
1. **Swagger/OpenAPI / generated-code agent**
   - Pin generator tooling if needed.
   - Generate `internal/api/generated/registration/server.gen.go`.
   - Add the handwritten registration adapter if still missing.
2. **Unit Testing agent**
   - Replace broken registration-related tests.
   - Add generated-route contract tests and active handler/service tests.
3. **Build/Make agent**
   - Add focused post-generate test targets and `.PHONY` entries.
   - Ensure `make generate-api` (or `make generate-api-test`) runs generation before the focused test package set.
4. **Observability/docs agent**
   - Confirm the current metrics/logging wiring is accurately documented, including the apparent unmounted HTTP metrics middleware and inactive rate-limit counters.
   - Update `CLAUDE.md` notes once the tests are real and passing.

## 17. Out-of-scope items
- New endpoints beyond the existing registration spec.
- Bot feature changes, Vizburo UI changes, or labour-bureau infra changes.
- Watermill/PIREP migration work from the broader refactor plan.
- Database schema changes.
- Broad legacy-package cleanup outside what is necessary to stop the broken registration tests from blocking the new workflow.

## 18. Final checklist
- [x] This planner avoided modifying any source/config/test/generated file outside this single plan file.
- [x] Plan file path: `politburo/plans/registration-generated-code-tests.md`
- **Key downstream agents/tasks:**
  - Swagger/OpenAPI/generated-code: generate and pin `registration` server code
  - Unit Testing: replace broken registration tests and add generated-route coverage
  - Build/Make: add post-generate focused test target
  - Observability/docs: verify current metrics/logging wiring and document any intentional gaps
