# Cleanup & Observability — Remaining Gaps

> Audited: 2026-05-13 · Plan: dead-code-observability-cleanup.md

Phases 1 and 3 are complete. Phase 2 has thin logging coverage on three files. One pre-existing test failure also surfaced.

---

## Gap 1 — Thin logging in `internal/events/repo.go`

Plan called for error logs on all GORM errors with `event_id` and `leg_id` fields. Only 2 `logging.` call sites are present. Audit every GORM `.Error` return path and wrap with `logging.Error(...)`.

## Gap 2 — Thin logging in `internal/pireps/repository.go`

Only 1 `logging.` call site. The PIREP submission path is largely silent on DB failures. Wrap all GORM error returns with `logging.Error("...", "pirep_id", ..., "err", err)`.

## Gap 3 — Thin logging in `internal/pireps/validation_service.go`

Only 1 `logging.` call site. Plan called for an info-level log on every validation failure naming the rule that failed. Ensure all rule branches emit `logging.Info("validation failed", "rule", ruleName, "pirep_id", ...)`.

## Gap 4 — PIREP queue worker error labels incomplete

`internal/pireps/queue_worker.go` plan (§3.1) specified four `QueueErrorsTotal` labels: `transient`, `validation`, `airtable_4xx`, `airtable_5xx`. Only `transient` was confirmed present. Verify and add the remaining three where the corresponding error paths are handled.

## Gap 5 — Missing `internal/testutil` package (pre-existing)

`go vet` fails on `internal/pilots/handler_test.go` which imports `internal/testutil` — the package does not exist. Pre-existing gap, not introduced by this plan. Options:
- (A) Create `internal/testutil/` with `CreateTestClaims`, `MakeRequest`, `ExecuteRequest` helpers (derived from usage in `handler_test.go` lines 38, 39, 42, 70, 73, 107–110, 130).
- (B) Remove or `//go:build ignore` the test file if the helpers are not recoverable.

## Gap 6 — CLAUDE.md doc update

The "Not yet implemented / test errors" section still references `internal/api/user_registration_v2_test.go` (deleted with the package). Should be replaced with the `internal/pilots/handler_test.go` `testutil` failure (Gap 5 above).
