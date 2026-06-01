# Flight Mode Input Authoring vs Airtable Mapping Constraint Decision

Status: proposed analysis/implementation plan (2026-06-01)  
Plan file: `politburo/plans/2026-06-01-flight-mode-airtable-mapping-scope-decision-plan.md`

## 1. Title and status

- **Requested change summary:** Decide whether `pilot_inputs` authoring in flight modes MUST be restricted to fields that are explicitly tied to Airtable mapping, based on current repo behavior and the approved 2026-05-30 revamp plan.
- **Scope:** Planning analysis only; no source changes. Covers expected model relationships (`pilot_inputs`, `computed_fields`, `airtable_mapping`), decision options, and follow-on deltas if strict mapping-bound creation is enforced.
- **Assumptions:**
  - Active reference plan is `politburo/plans/2026-05-30-flight-modes-pirep-revamp-plan.md`.
  - “Recent implementation” refers to the current VaAdmin mode edit form and handler now accepting dynamic `pilot_input_*[]` rows.

## 2. Context

- **Files/packages inspected:**
  - Plan baseline: `politburo/plans/2026-05-30-flight-modes-pirep-revamp-plan.md` (Phase 1A and component-level sections).
  - Runtime mode DTO/validation: `politburo/internal/models/dtos/flight_mode_runtime_v2.go`.
  - VaAdmin mode authoring: `politburo/internal/vaadmin/handler.go`, `politburo/templates/partials/flight-mode-edit-form.html`, `politburo/internal/vaadmin/flight_modes_view_test.go`.
  - PIREP config + submit flow: `politburo/internal/pireps/handler.go`, `politburo/internal/pireps/service.go`.
  - API contract and route wiring: `politburo/api/openapi/pireps.yaml`, `politburo/internal/api/pireps/server.go`, `politburo/internal/routes/router.go`.
  - Repo guidance: `AGENTS.md`, `politburo/CLAUDE.md`.
- **Observed current behavior (facts):**
  - Current v2 DTO has `PilotInputs` and `AirtableMapping map[string]string`; there is no `ComputedFields` model in runtime DTO yet.
  - VaAdmin edit UI currently supports adding/removing `pilot_inputs` rows, but has no computed/mapping authoring sections.
  - Save validation (`ParseModeRuntimeEnvelope`/`ValidateModeRuntimeEnvelope`) enforces mode basics and modal field count, but does **not** enforce mapping linkage between each pilot input and Airtable mapping.
  - Submit path (`buildPirepObject`) maps pilot inputs to Airtable fields via provider schema internal-name mapping (`GetFieldMapping(field.Key)`), not via `modeConfig.AirtableMapping`.
- **Relevant guidance discovered:**
  - Route registration and feature wiring must remain via `application.Features.*` / strict server wrapper (`internal/routes/router.go`).
  - OpenAPI artifacts are spec-first via `politburo/api/openapi/*.yaml` and generated server code.
  - API response conventions should align with shared envelope conventions.

## 3. Existing reuse

- Reuse `dtos.ParseModeRuntimeEnvelope` + `ValidateModeRuntimeEnvelope` as the authoritative save-time gate for any strict mapping policy.
- Reuse VaAdmin `UpdateFlightModeHandler` as the only existing UI write path for mode edits.
- Reuse `pireps.Service.buildPirepObject` as submit-time data-shaping point; this is where “unused/unmapped pilot input” effects currently surface implicitly.
- Reuse strict PIREP OpenAPI surface (`/api/v1/admin/flight-modes/config`) instead of adding new endpoints.

## 4. Architecture decisions

- **Decision (recommended): do NOT immediately enforce “pilot input must already be Airtable-mapped” as a hard create-time block.**
  - Reason: current implementation is Phase-1A-incomplete versus approved plan; strict blocking now would gate authoring before the repo even exposes computed/mapping authoring in VaAdmin.
- **Decision (recommended target state): enforce strict mapping completeness at config save once computed_fields + airtable_mapping authoring exists in the same UI/API slice.**
  - This aligns with the approved plan’s save-time validation intent and avoids creating dead/unusable pilot input keys.
- **Open question requiring explicit product decision:**
  - Should strictness be “all pilot inputs must map to Airtable” or “all required pilot inputs must map, optional may be unmapped with warning”? Current plan language is stronger for required fields; it does not explicitly require every optional field to map.
- **Risk if enforcing strict now:** high admin friction + likely false failures because mapping UX is not shipped in current VaAdmin edit form.

## 5. Repo-by-repo implementation plan

### politburo/

- **Must-do now (scope correction + safety):**
  - Add explicit validation/warning visibility for unmapped pilot inputs in VaAdmin listing/edit status (non-blocking), grounded in current DTO capabilities.
  - Document/encode in service-level validation strategy that strict blocking is deferred until computed/mapping authoring is present.
- **If strict mapping-bound creation is chosen (future delta):**
  - Extend `ModeRuntimeConfig` in `internal/models/dtos/flight_mode_runtime_v2.go` to match approved Phase 1A structures (`computed_fields[]`, `airtable_mapping[]` with source descriptors).
  - Move mapping validation into `ValidateModeRuntimeEnvelope`:
    - reject unknown mapping source keys,
    - reject ambiguous source descriptors,
    - enforce mapping completeness policy chosen by product.
  - Update `internal/vaadmin/handler.go` and `templates/partials/flight-mode-edit-form.html` to author computed/mapping rows in same save payload.
  - Update `internal/pireps/service.go` to resolve field submission from mode mapping graph (pilot/computed/fixed), not only schema internal-name lookups.
  - Preserve route mount through existing strict handler in `internal/routes/router.go`.

### comrade-bot/

- No immediate code change required for this decision-only slice.
- If strict enforcement is added, bot SHOULD surface backend validation codes/messages from `/pireps/config` and `/pireps/submit` more clearly in `/log` flow when a mode is invalid due to mapping errors.

### Vizburo UI

- Applicable inside Politburo server-rendered templates only.
- VaAdmin editor SHOULD add explicit section statuses for `computed_fields` and `airtable_mapping` (currently absent), using existing design-system token patterns.
- Mobile impact: admin editing on narrow screens needs row controls to remain usable; no polling needed.

### labour-bureau/

- Not applicable for core behavior; no infra change required for this policy decision.

### API contracts/generated clients/shared config

- If strict mapping is enforced, update `politburo/api/openapi/pireps.yaml` schemas for full v2 mode payload shape and stable validation errors; regenerate generated artifacts via standard process.

## 6. Developer guidelines for implementation agents

- **Boundary rules:**
  - MUST keep flight mode config validation centralized in DTO/service validation path; do not spread rule logic ad hoc across handlers.
  - MUST keep handlers thin; VaAdmin handler collects form data, service/DTO layer decides validity.
- **Likely files to edit (future strict rollout):**
  - `internal/models/dtos/flight_mode_runtime_v2.go`
  - `internal/vaadmin/handler.go`
  - `templates/partials/flight-mode-edit-form.html`
  - `internal/pireps/service.go`
  - `api/openapi/pireps.yaml` + generated `internal/api/generated/pireps/*`
- **Files/packages to avoid:**
  - Do not add new legacy service layers under `internal/services` or `internal/common`.
- **Sequencing:**
  1) schema/validation model, 2) VaAdmin authoring UI, 3) submit mapping engine, 4) API spec/gen updates, 5) tests.

## 7. Auth scopes, claims, and context

- Existing constraints remain:
  - Admin-only save route: `POST /api/v1/admin/flight-modes/config` under `IsAdminMiddleware`.
  - Dashboard VaAdmin UI is already admin-gated via session middleware + role middleware.
- VA context remains claim-derived and required for saving VA-specific config.
- Mobile classification: **admin UX impact only**; no pilot mobile runtime protocol impact from decision itself.

## 8. Migrations and data model

- No DB migration required for current decision.
- Future strict rollout may require compatibility handling for existing v2 blobs lacking computed/mapping arrays.
- Rollback consideration: stricter validation can make previously savable configs unsavable; provide transitional warnings before hard block.

## 9. Error handling and response conventions

- Current handlers frequently return plain `http.Error`; strict rollout SHOULD normalize to stable machine-readable validation codes on API surfaces.
- Introduce explicit error code(s) for mapping violations (e.g., unmapped required input, unknown mapping source) and keep envelopes consistent with existing API conventions.

## 10. Constants and configuration

- No new env vars required.
- Validation constants/enum strings for transform/mapping SHOULD live in DTO/model package, not duplicated in handler templates.

## 11. Logging and monitoring

- **Observability agent tasks:**
  - Add structured warn logs when invalid mode configs are detected in read path (`/pireps/config`) and when save rejects mapping errors.
  - Add bounded metric labels for flight-mode config save outcomes (`result=ok|validation_error|internal_error`) and optionally `error_code` with controlled cardinality.
  - Confirm `/metrics` scrape continuity (no infra scrape changes expected).
  - Avoid logging raw pilot free-text values in validation logs (privacy).

## 12. API spec and generated code work

- **Swagger/OpenAPI agent tasks:**
  - Update `api/openapi/pireps.yaml` `FlightModesConfigV2` schema from generic `additionalProperties` to explicit Phase 1A object shapes if strict enforcement is adopted.
  - Add/align operation-level 400 error examples for mapping-related validation failures.
  - Regenerate `internal/api/generated/pireps/server.gen.go` via existing generation workflow.
  - Verify operation IDs remain stable (`saveFlightModesConfig`, `getPirepConfig`, `submitPirep`).

## 13. Documentation

- Update the active revamp plan status notes to reflect whether strict mapping is deferred or activated.
- Add concise VaAdmin authoring guidance for admins: when unmapped fields are warnings vs blocking.

## 14. Frontend/Vizburo plan

- Keep VaAdmin handlers thin and server-rendered partial-driven.
- Add explicit section cards for computed/mapping (future strict rollout), no direct infra calls, no polling.
- Use existing design-system tokens/classes; avoid introducing alternate styling systems.

## 15. Testing plan

- **Unit Testing agent tasks:**
  - DTO validation tests for mapping completeness policy variants (strict all-fields vs strict required-only).
  - VaAdmin handler tests for form payload parsing of mapping/computed rows and error status behavior.
  - PIREP service tests ensuring submit path uses configured mapping graph (including computed and fixed sources).
  - Regression tests for `/pireps/config` to ensure invalid modes are excluded or clearly flagged per contract.
  - Manual verification: create mode with extra unmapped optional field and confirm expected warning/block behavior per final policy.

## 16. Execution order for specialized agents

1. **Architecture/backend agent:** finalize policy (`strict-required-only` vs `strict-all`) and implement DTO validation model.
2. **Frontend/Vizburo agent:** add missing authoring sections and warning/block UX.
3. **Swagger/OpenAPI agent:** update schemas + generate API artifacts.
4. **Observability agent:** add logs/metrics for save/read invalid-config paths.
5. **Unit Testing agent:** add coverage across DTO/service/handler and regressions.

## 17. Out-of-scope items

- No new endpoints, no route family redesign.
- No queue/worker/job changes.
- No Airtable provider implementation rewrite beyond mapping resolution behavior tied to this decision.
- No bot command UX redesign beyond error messaging touch-ups.

## 18. Final checklist

- Planner source modifications avoided: **Yes** (no code/test/config/docs edited outside this plan file).
- Plan file path: `politburo/plans/2026-06-01-flight-mode-airtable-mapping-scope-decision-plan.md`
- Key downstream agents/tasks:
  - Backend schema/validation alignment,
  - VaAdmin UI completion for computed/mapping,
  - Swagger/OpenAPI schema hardening + generation,
  - Observability for config validation outcomes,
  - Unit/integration regressions.

## 19. Incorporated recommendations (Repo-planning + Product strategy)

- **MVP-now policy (adopted): mapping-aware authoring without immediate hard gate on all unmapped fields.**
  - Keep `pilot_inputs` authorable in VaAdmin now.
  - Add explicit warning/status for unmapped/incomplete configuration paths.
  - Keep strict hard blocks for validated safety constraints already in scope (max 5 modal inputs, duplicate keys, invalid key format, fixed-route dependency, required mode basics).
- **Target-state policy (follow-on): mapping-first strict save once Phase 1A parity is complete.**
  - Once `computed_fields` + `airtable_mapping` authoring is implemented in same slice, enforce cross-reference completeness at save-time with machine-readable validation errors.
  - Use mode mapping graph as authoritative submit projection path.

### MVP now checklist

1. Keep Pilot Inputs authoring rows in VaAdmin edit form.
2. Preserve Discord modal max-field hard limit (5) with user-visible validation.
3. Validate key uniqueness and key-format constraints at UI + backend save path.
4. Show clear admin-facing non-success reasons (validation and server errors) instead of opaque HTMX-only diagnostics.
5. Reflect configuration readiness in section status/warnings.

### Defer-later checklist

1. Full Phase 1A model parity: `computed_fields[]` + structured `airtable_mapping[]` source descriptors.
2. Strict save-time mapping completeness policy finalization (`required-only` vs `all-fields`).
3. Submit-path mapping-graph execution and related OpenAPI schema hardening.
4. Extended tests for mapping graph, transform failures, and adapter contract errors.

---

## Decision summary (requested output)

- **Continue current branch, but do not hard-block pilot input creation by Airtable mapping yet.** Current implementation lacks Phase 1A computed/mapping authoring parity, so strict blocking now is premature.
- **Expected relationship:** `pilot_inputs` define collected values, `computed_fields` derive additional values, and `airtable_mapping` should be the authoritative projection to Airtable targets.
- **Enforce strict mapping completeness only when all three sections are implemented together** (save-time validation + submit-time execution), with clear machine-readable errors.
- **Current slice is scope-mismatched with approved plan:** pilot input authoring landed, but computed_fields + airtable_mapping authoring/validation and runtime mapping execution are not yet aligned.

## Evidence from plan sections

- Phase 1A minimum schema explicitly includes `pilot_inputs`, `computed_fields`, and `airtable_mapping` together (active plan lines 70-79, 80-113).
- Phase 1A save-time validation expects mapping/source-reference checks and unknown source rejection (lines 123-133).
- Phase 1A acceptance criteria include transform+mapping execution on submit and rejection behavior (lines 147-153).
- Component G in active plan defines mapping composer and guardrails, including save/submit blocking semantics (lines 243-249).

## Required implementation deltas by repo path (if strict policy enforced)

- `politburo/internal/models/dtos/flight_mode_runtime_v2.go`: add explicit computed/mapping models and strict cross-reference validation.
- `politburo/internal/vaadmin/handler.go` + `politburo/templates/partials/flight-mode-edit-form.html`: add computed/mapping authoring rows and field-level validation display.
- `politburo/internal/pireps/service.go`: execute mapping graph (pilot/computed/fixed) rather than only schema internal-name lookup.
- `politburo/api/openapi/pireps.yaml` + `politburo/internal/api/generated/pireps/*`: tighten schema + regenerate.
- `politburo/internal/vaadmin/*_test.go`, `politburo/internal/models/dtos/*_test.go`, `politburo/internal/pireps/*_test.go`: expand coverage for strict mapping validation and runtime behavior.
- `comrade-bot/src/services/apiService.ts` and `/log` handlers: consume/display new validation codes (if API error contract changes).

## Stop/continue recommendation for current branch

- **Continue** with current branch for Phase-1A completion, but **stop short of strict mapping-bound creation enforcement** until computed/mapping sections are implemented end-to-end.
- Immediate guardrail: ship non-blocking unmapped-field warnings in VaAdmin and explicit “incomplete mode config” signaling.
- Gate strict enforcement behind a single completion PR that includes DTO schema, VaAdmin authoring, submit mapping engine, API schema updates, and tests.
