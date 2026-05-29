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
