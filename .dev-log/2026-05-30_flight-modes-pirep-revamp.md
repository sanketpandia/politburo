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
