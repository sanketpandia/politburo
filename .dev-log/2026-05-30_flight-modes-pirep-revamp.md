# Flight Modes + PIREP Revamp

Date: 2026-05-30
Branch: `feat/flight-modes-pirep-revamp`

## Logical unit / commit intent

- Phase 4 UTC normalization slice: enforce UTC timestamps in identified persistence/metadata hotspots.

## Changed files

- `internal/platform/aircraft/repo.go`
- `internal/pilots/stats_service.go`
- `internal/app/config.go`

## Reused code / patterns / components

- Reused existing repository write paths (`UpsertBatch`, `MarkInactive`) and existing stats metadata envelope fields.
- Reused current Postgres DSN builder in `internal/app/config.go`; only appended explicit timezone hint.

## Logging added or affected

- No new log families; existing logs unchanged.

## Metrics added or affected

- No metrics changes in this logical unit.

## Test surface touched or still needed

- Touched UTC serialization/persistence behavior in aircraft repo and pilot stats metadata.
- Still needed (out-of-scope follow-up): dedicated unit tests asserting UTC `Z` formatting and UTC DB write timestamps.

## Build/test command(s) run and status

- `go test ./internal/platform/aircraft ./internal/pilots ./internal/flights`
  - status: passed

## Deviations from plan, if any

- None for this slice.

## Blast-radius notes / dependent surfaces checked

- Checked `internal/flights` package as a guardrail to ensure no regression in flight timestamp handling.
- Limited change to plan-defined UTC hotspots only.

## Live API compliance notes when relevant

- No Live API boundary changes.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: timestamp examples should continue to show RFC3339 UTC (`Z`) values where documented.
- Observability: no new metrics required for this UTC-only slice.
- Unit Testing: add focused UTC regression tests for `aircraft.Repository` and `StatsService` metadata fields.

---

Date: 2026-05-30
Branch: `feat/flight-modes-pirep-revamp`

## Logical unit / commit intent

- Phase 1/2 backend contract hardening: enforce strict v2 flight-mode config parsing/validation/constants, align PIREP handlers to `httpdto` envelopes, clean route-source semantics in submission, and mount admin flight-mode config API route.

## Changed files

- `internal/models/dtos/flight_mode_runtime_v2.go`
- `internal/pireps/handler.go`
- `internal/pireps/service.go`
- `internal/platform/va/service.go`
- `internal/platform/va/handler.go`
- `internal/services/flight_modes_config_service.go`
- `internal/routes/router.go`

## Reused code / patterns / components

- Reused `httpdto.WriteSuccess/WriteError` envelope helpers for PIREP API.
- Reused existing `/api/v1/admin` authorization boundary and VA platform service save path.
- Reused existing PIREP submission pipeline (`buildPirepObject`, Airtable provider submit).

## Logging added or affected

- Added warning logs when v2 mode config parsing fails in PIREP config response building.

## Metrics added or affected

- No new metrics in this unit.

## Test surface touched or still needed

- Touched PIREP handler/service, VA config validation, and route mounting.
- Still needed: unit tests for `ParseModeRuntimeEnvelope` validation failures and route-source behavior permutations.

## Build/test command(s) run and status

- `go test ./internal/pireps ./internal/platform/va ./internal/routes ./internal/platform/httpdto`
  - status: passed

## Deviations from plan, if any

- Kept existing fixed submit DTO shape (mode/route_id/flight_time + known optional fields); fully generic dynamic pilot-input submission contract remains follow-up.

## Blast-radius notes / dependent surfaces checked

- Checked router admin-group auth; mounted `POST /api/v1/admin/flight-modes/config` under existing admin middleware.
- Checked bot-facing PIREP endpoints still mounted at existing paths and now emit `httpdto` envelope on both success and error.

## Live API compliance notes when relevant

- No direct LiveAPI boundary changes introduced.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: update/add PIREP and admin flight-mode endpoint schemas to reflect `httpdto` envelope and strict v2 `config_version` requirement.
- Observability: add counters for invalid mode-config parse rejects and submit error-code classes.
- Unit Testing: add table-driven tests for v2 parser enums (`detection_mode`, `route_source`) and handler envelope error mappings.

---

Date: 2026-05-30
Branch: `feat/flight-modes-pirep-revamp`

## Logical unit / commit intent

- Phase 2 VaAdmin flow completion: migrate flight-mode admin list/edit/toggle/update to strict v2 config model and ship sectioned editor UI with section-status chips; add PIREP/flight-mode OpenAPI source spec file.

## Changed files

- `internal/vaadmin/handler.go`
- `templates/partials/flight-modes-list.html`
- `templates/partials/flight-mode-edit-form.html`
- `api/openapi/pireps.yaml`

## Reused code / patterns / components

- Reused existing vaadmin HTMX route pattern (`/dashboard/vaadmin/flight-modes/...`).
- Reused shared v2 parser/constants in `internal/models/dtos` for all mode list/edit/toggle/update operations.
- Reused existing `ValidateAndSaveFlightModesConfig` write path for save safety.

## Logging added or affected

- Added warning path for invalid v2 payloads during admin list rendering.

## Metrics added or affected

- No metrics added in this unit.

## Test surface touched or still needed

- Touched vaadmin handlers + templates for flight-mode admin path.
- Still needed: focused handler tests for sectioned editor form validation (`fixed_route` requires route name) and card rendering status chips.

## Build/test command(s) run and status

- `go test ./internal/vaadmin ./internal/routes ./internal/pireps ./internal/platform/va`
  - status: passed
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - status: passed

## Deviations from plan, if any

- OpenAPI file added as source contract (`api/openapi/pireps.yaml`) without generated artifact wiring because no generator config exists yet for this domain.

## Blast-radius notes / dependent surfaces checked

- Checked route ownership/middleware remained within dashboard admin + API admin scopes.
- Checked bot-facing `/api/v1/pireps/*` routes unchanged path-wise to preserve comrade-bot compatibility.

## Live API compliance notes when relevant

- No direct liveapi client boundary changes in this unit.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: decide whether to register `pireps.yaml` in make generation targets for client/server generation workflow.
- Observability: add outcome counters for vaadmin mode save validation failures.
- Unit Testing: add table tests for `buildModeSectionStatus` and `buildModeCards` conversion behavior.

---

Date: 2026-05-30
Branch: `feat/flight-modes-pirep-revamp`

## Logical unit / commit intent

- Dynamic submit-contract slice: extend PIREP submit DTO/runtime to carry config-driven dynamic inputs while preserving known typed fields.

## Changed files

- `internal/models/dtos/pirep_config.go`
- `internal/pireps/service.go`

## Reused code / patterns / components

- Reused existing PIREP submit service pipeline and schema-field mapping flow in `buildPirepObject`.
- Reused current known top-level fields (`flight_time`, `fuel_kg`, `cargo_kg`, `passengers`) with fallback to dynamic `inputs` map.

## Logging added or affected

- No new logging families.

## Metrics added or affected

- No metrics changes.

## Test surface touched or still needed

- Touched submit validation and payload-composition runtime path.
- Still needed: dedicated tests for required dynamic-field validation and numeric coercion behavior.

## Build/test command(s) run and status

- `go test ./internal/pireps ./internal/models/dtos ./internal/platform/va ./internal/vaadmin ./internal/routes`
  - status: passed
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - status: passed

## Deviations from plan, if any

- None for this logical unit.

## Blast-radius notes / dependent surfaces checked

- Checked bot request shaping compatibility remains backward-safe by keeping known fields and adding dynamic map.

## Live API compliance notes when relevant

- No liveapi boundary change.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Unit Testing: add table tests for `getInputValue` + required-key validation across dynamic and typed fields.

---

Date: 2026-05-30
Branch: `feat/flight-modes-pirep-revamp`

## Logical unit / commit intent

- OpenAPI generation wiring slice: add PIREP oapi-codegen config + Makefile targets and generate server artifact from `api/openapi/pireps.yaml`.

## Changed files

- `api/openapi/pireps.cfg.yaml`
- `api/openapi/pireps.yaml`
- `internal/api/generated/pireps/server.gen.go`
- `Makefile`

## Reused code / patterns / components

- Reused existing oapi-codegen repo convention from registration/liveapi targets.

## Logging added or affected

- None.

## Metrics added or affected

- None.

## Test surface touched or still needed

- Generation/build pipeline touched.
- Still needed: follow-on adapter/mounting tests if generated strict server is integrated into runtime routes later.

## Build/test command(s) run and status

- `make generate-pireps-api`
  - status: passed
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`
  - status: passed

## Deviations from plan, if any

- Generated strict server artifact is wired and committed, but runtime mounting through generated strict adapter is not introduced in this slice.

## Blast-radius notes / dependent surfaces checked

- Checked `make generate-api` composition now includes PIREP generation through new target.

## Live API compliance notes when relevant

- No liveapi changes.

## Follow-up notes for Swagger/OpenAPI, Observability, or Unit Testing agents when applicable

- Swagger/OpenAPI: decide if generated PIREP strict server should become canonical mounted route path (matching registration pattern) in a dedicated integration slice.
