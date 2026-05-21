# Registration Generated Handlers Runtime Routing — Dev Log

## 2026-05-21 — Generated runtime routing and `/user/register` rename

- **Logical unit / commit intent:** Route the registration/onboarding OpenAPI domain through generated strict handlers at runtime and remove the legacy `/api/v1/pilots/register` route in favor of `POST /api/v1/user/register`.
- **Changed files:**
  - `api/openapi/registration.yaml`
  - `internal/api/generated/registration/server.gen.go`
  - `internal/api/registration/server.go`
  - `internal/api/registration/server_test.go`
  - `internal/routes/router.go`
  - `internal/routes/registration_routes_test.go`
  - `internal/middleware/discord_context_test.go`
  - `internal/pilots/handler.go`
  - `internal/pilots/handler_test.go`
  - `CLAUDE.md`
  - `plans/registration-generated-handlers-runtime-routing.md`
  - `../comrade-bot/src/services/apiService.ts`
- **Reused code / patterns / components:** Reused `internal/api/registration.NewServer` adapter, `registrationgen.NewStrictHandler`, `registrationgen.HandlerFromMux`, existing `AuthMiddleware`, existing `RequireDiscordBotContextMiddleware`, existing feature handlers, and existing `httpdto` response envelopes.
- **Logging added or affected:** Updated registration handler route text from `/pilots/register` to `/user/register`; no new raw IDs, request bodies, API keys, or signed tokens are logged.
- **Metrics added or affected:** No new metrics. Existing global `MetricsMiddleware` will now observe generated Chi route pattern `/user/register` for registration instead of the removed legacy route.
- **Test surface touched or still needed:** Updated generated-adapter tests, pilot handler tests, Discord-context middleware tests, and added route-level generated-mount tests. Still needed as a follow-up: generated TypeScript client migration for comrade-bot API calls instead of handwritten `fetch` calls in `src/services/apiService.ts`.
- **Build/test command(s) run and status:**
  - `make generate-registration-api` from `politburo/` — passed.
  - `go test ./internal/api/... ./internal/routes ./internal/pilots ./internal/middleware ./internal/memberships ./internal/servers ./internal/auth ./internal/platform/httpdto ./internal/platform/validation` from `politburo/` — passed.
  - `npm run build` from `comrade-bot/` — passed.
- **Deviations from plan, if any:** User clarified to remove legacy behavior, so no `/api/v1/pilots/register` compatibility alias was kept. User also clarified the long-term goal for comrade-bot is spec-generated clients; current implementation only updates the handwritten URL because no generated TypeScript client tooling currently exists in `comrade-bot/package.json` or `comrade-bot/src`.
- **Blast-radius notes / dependent surfaces checked:** Checked registration OpenAPI spec/config, generated Go server, registration adapter, runtime router, auth/Discord context middleware tests, pilot handler route text/tests, comrade-bot API service, and labour-bureau Swagger Editor compose service context. No Vizburo templates, jobs, DB migrations, Watermill, PIREP, events, or infra runtime wiring changed.
- **Live API compliance notes:** Not applicable; no Infinite Flight Live API client behavior changed. Registration still accepts `ifc_id` and `last_flight` and delegates validation to the existing pilot registration service.
- **Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents:**
  - Swagger/OpenAPI: confirm `registration.yaml` remains the canonical source for the registration domain and decide a TypeScript client generator convention for comrade-bot using the existing Swagger/OpenAPI dev tooling.
  - Observability: verify dashboards/queries do not depend on the old `/pilots/register` route label; route metrics should now use `/user/register`.
  - Unit Testing: future generated TypeScript client migration should include bot-side tests/type checks around the generated client boundary and auth header injection.

## 2026-05-21 — RegisterPilot 404 propagation fix

- **Logical unit / commit intent:** Add the missing generated-contract `404` response for `POST /api/v1/user/register` so expected IFC-user-not-found errors propagate as JSON error envelopes instead of adapter-level 500s.
- **Changed files:**
  - `api/openapi/registration.yaml`
  - `internal/api/generated/registration/server.gen.go`
  - `internal/api/registration/server.go`
  - `internal/api/registration/server_test.go`
- **Reused code / patterns / components:** Reused existing `pilots.RegistrationError` domain mapping (`IFC_USER_NOT_FOUND`, HTTP 404), existing `httpdto.WriteError` envelope, and generated strict response object mapping.
- **Logging added or affected:** No logging changes. Existing logs correctly record the LiveAPI lookup result and handler error without changing fields.
- **Metrics added or affected:** No metrics changes. HTTP metrics now record the correct `404` status for expected missing IFC user cases instead of `500`.
- **Test surface touched or still needed:** Added generated-adapter coverage asserting `IFC_USER_NOT_FOUND` propagates as `404` with the error envelope.
- **Build/test command(s) run and status:**
  - `make generate-registration-api` from `politburo/` — passed.
  - `go tool oapi-codegen -config registration.cfg.yaml registration.yaml >/tmp/registration-oapi-validate.go` from `politburo/api/openapi/` — passed.
  - `go test ./internal/api/... ./internal/routes ./internal/pilots ./internal/middleware ./internal/memberships ./internal/servers ./internal/auth ./internal/platform/httpdto ./internal/platform/validation` from `politburo/` — passed.
  - Local curl for missing IFC user — returned `HTTP/1.1 404 Not Found` with `{"status":"error","error":{"code":"IFC_USER_NOT_FOUND",...}}`.
- **Deviations from plan, if any:** None. This corrects an implementation bug discovered by manual curl testing.
- **Blast-radius notes / dependent surfaces checked:** Checked OpenAPI response coverage, generated response object presence, adapter switch mapping, and registration service domain error mapping.
- **Live API compliance notes:** LiveAPI may return 200 with an empty result for unknown IFC users; Politburo maps that expected domain outcome to `IFC_USER_NOT_FOUND` / 404.
- **Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents:** No further follow-up required for this specific 404 propagation issue.

## 2026-05-21 — Registration adapter host configuration

- **Logical unit / commit intent:** Remove the hardcoded `example.com` host from the generated registration adapter's synthetic delegated requests and derive it from runtime configuration.
- **Changed files:**
  - `internal/api/registration/server.go`
  - `internal/api/registration/server_test.go`
- **Reused code / patterns / components:** Kept the existing adapter delegation pattern and `httptest.NewRequest` synthetic request construction; added environment-based host selection using `POLITBURO_BASE_URL`, `API_BASE_URL`, `API_URL`, then `PORT`.
- **Logging added or affected:** No logging changes.
- **Metrics added or affected:** No metrics changes.
- **Test surface touched or still needed:** Added adapter tests for configured base URL host and `PORT` fallback host.
- **Build/test command(s) run and status:**
  - `go test ./internal/api/... ./internal/routes ./internal/pilots ./internal/middleware ./internal/memberships ./internal/servers ./internal/auth ./internal/platform/httpdto ./internal/platform/validation` from `politburo/` — passed.
- **Deviations from plan, if any:** None. This fixes a discovered adapter correctness issue.
- **Blast-radius notes / dependent surfaces checked:** Scoped to the internal adapter; real incoming request routing remains unchanged. This mainly affects delegated handlers such as signed-link generation when they inspect request host.
- **Live API compliance notes:** Not applicable.
- **Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents:** Document `POLITBURO_BASE_URL`/`API_BASE_URL` convention if it becomes a supported operator-facing environment variable beyond this adapter fallback.
