# Politburo endpoint inventory

Audience: developers and maintainers auditing Politburo's currently mounted HTTP surface.

## Purpose

This document consolidates the endpoints mounted by `internal/routes/router.go`, grouped feature-wise. Each endpoint also includes its return type at the end so it is easy to scan whether it returns `json`, `partial`, `page`, `static`, or a similar response shape.

Source of truth order used here:

1. `internal/routes/router.go`
2. Handler implementations behind those routes
3. `TECHNICAL_STANDARDS.md`
4. `CLAUDE.md`
5. Relevant dev logs and OpenAPI artifacts

## Classification used

- **Aligned** — matches current standards reasonably well.
- **Mixed** — active and supported, but still uses older helpers or sits in known debt-heavy code.
- **Legacy-sensitive** — active route, but implementation clearly depends on legacy or migration-sensitive packages.

## Feature: platform and infrastructure

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/static/*` | Serve static assets | Aligned | `static` |
| `GET` | `/favicon.ico` | Serve favicon | Aligned | `static` |
| `GET` | `/healthCheck` | Health check | Aligned | `json` |
| `GET` | `/auth/login` | Token login page | Aligned | `page` |

## Feature: registration and onboarding

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/user/register` | Register pilot against IFC account | Aligned | `json` |
| `POST` | `/api/v1/server/init` | Bootstrap a Discord server as a minimal VA | Aligned | `json` |
| `POST` | `/api/v1/memberships/join` | Join current Discord server's VA | Aligned | `json` |
| `GET` | `/api/v1/user/status` | Read onboarding/registration state | Aligned | `json` |
| `POST` | `/api/v1/signed-link` | Create signed Vizburo access URL | Aligned | `json` |

Notes:

- This is the cleanest standards-aligned API area.
- It is OpenAPI-covered through `api/openapi/registration.yaml` and mounted through generated handlers.
- Legacy `POST /api/v1/pilots/register` is intentionally not mounted.

## Feature: pilot stats and logbook

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/pilot/stats` | Current user's pilot stats | Aligned | `json` |
| `GET` | `/api/v1/pilots/{ifc_id}/logbook` | Staff/admin logbook lookup | Aligned | `json` |
| `GET` | `/api/v1/user/{ifc_id}/flights` | Self-service logbook lookup | Aligned | `json` |
| `GET` | `/dashboard/logbook` | Staff/admin logbook page | Aligned | `page` |
| `GET` | `/dashboard/logbook/flights` | Staff/admin logbook results | Aligned | `partial` |

## Feature: live flights

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/flights/va` | Cached live flights for active VA | Mixed | `json` |
| `GET` | `/api/v1/flights/{flight_id}` | Cached single flight | Mixed | `json` |
| `GET` | `/dashboard/live` | Live flights page | Mixed | `page` |
| `GET` | `/dashboard/flights/{flight_id}/waypoints` | Waypoint data for map | Mixed | `json` |
| `GET` | `/dashboard/flights/{flight_id}/paths` | Cached planned/flown path data | Mixed | `json` |
| `GET` | `/dashboard/link` | Signed dashboard link | Aligned | `json` |

Notes:

- Active and useful, but the JSON helper style still uses older `internal/common.Respond*` patterns in `internal/flights`.
- This area is still technical-debt heavy rather than redundant.

## Feature: PIREPs

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/pireps/config` | Current-flight PIREP configuration | Mixed | `json` |
| `POST` | `/api/v1/pireps/submit` | Submit a PIREP | Mixed | `json` |
| `POST` | `/api/v1/admin/flight-modes/config` | Save strict v2 flight-mode config | Mixed | `json` |

Notes:

- Mounted through generated OpenAPI strict handler (`internal/api/generated/pireps`) with a handwritten adapter in `internal/api/pireps/server.go`, mirroring registration runtime pattern.
- Domain logic still uses some legacy-sensitive dependencies (`internal/common`, `internal/db/repositories`) and remains a follow-up cleanup area.

## Feature: events API

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/events` | List events | Aligned | `json` |
| `POST` | `/api/v1/events` | Create event | Aligned | `json` |
| `GET` | `/api/v1/events/pirep-config` | Event PIREP config | Aligned | `json` |
| `GET` | `/api/v1/events/{id}` | Get event | Aligned | `json` |
| `PUT` | `/api/v1/events/{id}` | Update event | Aligned | `json` |
| `DELETE` | `/api/v1/events/{id}` | Delete event | Aligned | `json` |
| `PATCH` | `/api/v1/events/{id}/status` | Update event status | Aligned | `json` |
| `GET` | `/api/v1/events/{id}/summary` | Event summary | Aligned | `json` |
| `GET` | `/api/v1/events/{id}/legs` | List event legs | Aligned | `json` |
| `POST` | `/api/v1/events/{id}/legs` | Create event leg | Aligned | `json` |
| `GET` | `/api/v1/events/{id}/legs/{leg_id}` | Get event leg | Aligned | `json` |
| `PUT` | `/api/v1/events/{id}/legs/{leg_id}` | Update event leg | Aligned | `json` |
| `PATCH` | `/api/v1/events/{id}/legs/{leg_id}/additional-data` | Patch leg additional data | Aligned | `json` |
| `DELETE` | `/api/v1/events/{id}/legs/{leg_id}` | Delete event leg | Aligned | `json` |

## Feature: events UI

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/dashboard/events` | Events management page | Mixed | `page` |
| `GET` | `/dashboard/events/list` | Event list | Mixed | `partial` |
| `GET` | `/dashboard/events/form` | New-event form | Mixed | `partial` |
| `GET` | `/dashboard/events/form/{event_id}` | Edit-event form | Mixed | `partial` |
| `POST` | `/dashboard/events/create` | Create event | Mixed | `partial` |
| `GET` | `/dashboard/events/routes/search` | Search routes for events | Mixed | `partial` |
| `POST` | `/dashboard/events/{event_id}/update` | Update event | Mixed | `partial` |
| `DELETE` | `/dashboard/events/{event_id}` | Delete event | Mixed | `partial` |
| `GET` | `/dashboard/events/{event_id}/legs/form` | New leg form | Mixed | `partial` |
| `GET` | `/dashboard/events/{event_id}/legs/form/{leg_id}` | Edit leg form | Mixed | `partial` |
| `POST` | `/dashboard/events/{event_id}/legs/create` | Create leg | Mixed | `partial` |
| `POST` | `/dashboard/events/{event_id}/legs/{leg_id}/update` | Update leg | Mixed | `partial` |
| `DELETE` | `/dashboard/events/{event_id}/legs/{leg_id}` | Delete leg | Mixed | `partial` |

Notes:

- Rendering model is current, but `internal/events/handler.go` is still a very large file and a known split target.

## Feature: auth and session admin

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/admin/verify-god` | Return `is_god` flag | Aligned | `json` |
| `POST` | `/api/v1/admin/sessions/destroy/{ifc_id}` | Destroy sessions for IFC ID | Aligned | `json` |
| `POST` | `/dashboard/switch-va` | Switch active VA in session | Aligned | `page` |

## Feature: dashboard core

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/dashboard/` | Main dashboard | Aligned | `page` |
| `GET` | `/dashboard/leaderboard/pilot/logs` | Leaderboard pilot logs partial | Aligned | `partial` |
| `GET` | `/dashboard/test-click` | HTMX click test endpoint | Aligned | `partial` |

## Feature: VA admin setup and management

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/dashboard/vaadmin/` | VA admin landing page | Mixed | `page` |
| `GET` | `/dashboard/vaadmin/setup` | Setup/readiness page | Mixed | `page` |
| `GET` | `/dashboard/vaadmin/setup/basic` | Basic setup form | Mixed | `partial` |
| `POST` | `/dashboard/vaadmin/setup/basic` | Save display name and callsign matching | Mixed | `partial` |
| `GET` | `/dashboard/vaadmin/setup/checklist` | Setup checklist | Mixed | `partial` |
| `POST` | `/dashboard/vaadmin/setup/callsign-test` | Callsign-matching test | Mixed | `partial` |
| `GET` | `/dashboard/vaadmin/flight-modes` | Flight modes page | Mixed | `page` |
| `GET` | `/dashboard/vaadmin/pilots` | Pilot management page | Mixed | `page` |
| `GET` | `/dashboard/vaadmin/pilots/list` | Pilot list | Mixed | `partial` |
| `POST` | `/dashboard/vaadmin/pilots/{pilot_id}/callsign` | Update callsign | Mixed | `partial` |
| `POST` | `/dashboard/vaadmin/pilots/{pilot_id}/role` | Update role | Mixed | `partial` |
| `DELETE` | `/dashboard/vaadmin/pilots/{pilot_id}` | Remove pilot | Mixed | `partial` |
| `GET` | `/dashboard/vaadmin/flight-modes/list` | Flight mode list | Mixed | `partial` |
| `GET` | `/dashboard/vaadmin/flight-modes/{mode_id}/edit` | Edit flight mode | Mixed | `partial` |
| `POST` | `/dashboard/vaadmin/flight-modes/{mode_id}/toggle` | Toggle flight mode | Mixed | `partial` |
| `POST` | `/dashboard/vaadmin/flight-modes/{mode_id}/update` | Update flight mode | Mixed | `partial` |
| `GET` | `/dashboard/vaadmin/webhooks` | Webhooks page | Mixed | `page` |
| `GET` | `/dashboard/vaadmin/webhooks/list` | Webhooks list | Mixed | `partial` |
| `POST` | `/dashboard/vaadmin/webhooks` | Create webhook | Mixed | `partial` |
| `POST` | `/dashboard/vaadmin/webhooks/run` | Run webhook now | Mixed | `partial` |

Notes:

- These are active Vizburo routes, but `internal/vaadmin/handler.go` remains a large technical-debt hotspot.

## Feature: datasource configuration UI

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/dashboard/settings/datasource` | Datasource config page | Aligned | `page` |
| `GET` | `/dashboard/settings/datasource/status` | Datasource status | Aligned | `partial` |
| `GET` | `/dashboard/settings/datasource/type-selector` | Datasource type selector | Aligned | `partial` |
| `GET` | `/dashboard/settings/datasource/schema-selector` | Schema selector | Aligned | `partial` |
| `GET` | `/dashboard/settings/datasource/credentials-form` | Credentials form | Aligned | `partial` |
| `POST` | `/dashboard/settings/datasource/credentials` | Save credentials | Aligned | `partial` |
| `POST` | `/dashboard/settings/datasource/test-connection` | Test datasource connection | Aligned | `partial` |
| `GET` | `/dashboard/settings/datasource/schema/{schemaType}` | Schema config form | Aligned | `partial` |
| `POST` | `/dashboard/settings/datasource/schema/{schemaType}` | Save schema config | Aligned | `partial` |
| `POST` | `/dashboard/settings/datasource/schema/{schemaType}/sync` | Sync schema/table metadata | Aligned | `partial` |

## Feature: Airtable admin API

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/admin/airtable/credentials` | Save Airtable credentials | Legacy-sensitive | `json` |
| `POST` | `/api/v1/admin/airtable/schema/{schemaType}` | Save Airtable schema | Legacy-sensitive | `json` |
| `GET` | `/api/v1/admin/airtable/schema/{schemaType}` | Read Airtable schema | Legacy-sensitive | `json` |
| `GET` | `/api/v1/admin/airtable/schemas` | List Airtable schemas | Legacy-sensitive | `json` |

Notes:

- These are still mounted and active.
- But `internal/platform/va/handler.go` still relies on `internal/common`, `internal/db/repositories`, and `internal/services` patterns flagged as migration-sensitive.

## Feature: livery mappings

| Method | Path | Purpose | Status | Return type |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/admin/livery-mappings` | List mappings | Aligned | `json` |
| `POST` | `/api/v1/admin/livery-mappings` | Create mappings | Aligned | `json` |
| `DELETE` | `/api/v1/admin/livery-mappings/{id}` | Delete mapping | Aligned | `json` |
| `GET` | `/api/v1/admin/livery-mappings/liveries` | List available liveries | Aligned | `json` |
| `GET` | `/api/v1/admin/livery-mappings/unique-aircraft` | List unique aircraft | Aligned | `json` |
| `GET` | `/api/v1/admin/livery-mappings/unique-liveries` | List unique liveries | Aligned | `json` |
| `GET` | `/api/v1/admin/livery-mappings/defaults` | Read defaults | Aligned | `json` |
| `POST` | `/api/v1/admin/livery-mappings/defaults` | Save defaults | Aligned | `json` |
| `GET` | `/dashboard/settings/livery-mappings` | Livery mappings page | Aligned | `page` |

## Explicitly not mounted or intentionally superseded

| Endpoint/path | Current state | Evidence |
| --- | --- | --- |
| `POST /api/v1/pilots/register` | Not mounted; replaced by `POST /api/v1/user/register` | `TECHNICAL_STANDARDS.md`, `CLAUDE.md`, registration dev log |
| Older Vizburo/Tailwind UI trees under `vizburo/ui/**` | Not part of active endpoint serving path | Vizburo cleanup docs/dev logs |
| OpenAPI for Vizburo HTML routes | Intentionally not used | `TECHNICAL_STANDARDS.md` |

## Overall findings

- Best-aligned feature area: registration and onboarding.
- Most obviously mixed but still active feature areas: live flights, events UI, VA admin UI.
- Most legacy-sensitive mounted feature areas: PIREPs and Airtable admin API.

## Recommended follow-up work

1. Standardize active flights JSON handlers on `internal/platform/httpdto` envelopes.
2. Audit `internal/pireps` runtime dependencies, especially remaining reliance on `internal/common.LiveAPIService`.
3. Split `internal/events/handler.go` into clearer UI/API responsibilities.
4. Split `internal/vaadmin/handler.go` by feature area.
5. Migrate active Airtable admin handlers away from `internal/common`, `internal/services`, and `internal/db/repositories` where practical.
