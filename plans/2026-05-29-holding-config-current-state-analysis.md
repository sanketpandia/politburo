# Holding Config Current-State Analysis

## Scope

Analysis of the current configuration setup for:

1. Basic VA/holding config such as callsign prefix/suffix and enabled live servers.
2. Airtable-backed datasource/provider config for pilots, routes, PIREPs, and career mode.
3. Save/update/cache/retrieval flows.
4. Existing UI support and whether a reusable quick-fetch provider already exists.

## Files inspected

- `politburo/CLAUDE.md`
- `politburo/internal/platform/va/config_service.go`
- `politburo/internal/platform/va/readiness.go`
- `politburo/internal/platform/va/config_dtos.go`
- `politburo/internal/platform/va/service.go`
- `politburo/internal/platform/va/repo.go`
- `politburo/internal/platform/va/handler.go`
- `politburo/internal/platform/va/model.go`
- `politburo/internal/common/va_config_service.go`
- `politburo/internal/common/airtable_service.go`
- `politburo/internal/services/data_provider_config_service.go`
- `politburo/internal/db/repositories/data_provider_repo.go`
- `politburo/internal/datasource/handler.go`
- `politburo/internal/datasource/dto.go`
- `politburo/internal/vaadmin/handler.go`
- `politburo/internal/routes/router.go`
- `politburo/internal/routes/jobs.go`
- `politburo/internal/app/app.go`
- `politburo/internal/pilots/sync_job.go`
- `politburo/internal/pilots/stats_service.go`
- `politburo/internal/pilots/linking_job.go`
- `politburo/internal/va_routes/sync_job.go`
- `politburo/internal/pireps/service.go`
- `politburo/internal/pireps/sync_job.go`
- `politburo/internal/pireps/tour_service.go`
- `politburo/internal/sync/route_sync.go`
- `politburo/templates/pages/datasource.html`
- `politburo/templates/partials/basic-setup-form.html`
- `politburo/templates/partials/datasource-status.html`
- `politburo/templates/partials/datasource-schema-selector.html`
- `politburo/templates/partials/datasource-schema-config.html`
- `politburo/templates/partials/datasource-field-mapper.html`
- `politburo/templates/partials/datasource-credentials-form.html`
- `politburo/infra/providers/airtable_provider.go`
- `politburo/infra/db/migrations/000_schema.sql`
- `politburo/infra/db/migrations/011_refactor_data_provider_configs.sql`

## High-level finding

There are currently **two separate configuration systems**:

### 1. Basic VA config in `va_configs`

Used for small per-VA key/value settings such as:

- `callsign_prefix`
- `callsign_suffix`
- `enabled_server_ids`
- legacy Airtable keys
- default aircraft/airline values

Primary code:

- `internal/platform/va/config_service.go`
- `internal/platform/va/readiness.go`
- `internal/vaadmin/handler.go`

### 2. Datasource/provider config in `va_data_provider_configs`

Used for Airtable credentials and entity-specific schema definitions.

Primary code:

- `internal/platform/va/model.go`
- `internal/platform/va/repo.go`
- `internal/platform/va/service.go`
- `internal/datasource/handler.go`

This means “holding configuration” is already split between:

- a generic key/value config store
- a structured provider config store

## Basic VA config: current state

### Storage model

`internal/platform/va/config_service.go` manages key/value configs backed by `va_configs`.

Observed keys include:

- `if_server_id`
- `enabled_server_ids`
- `callsign_prefix`
- `callsign_suffix`
- `tour_flight_mode`
- `default_aircraft`
- `default_airline`
- several legacy Airtable keys such as `airtable_api_key`, `airtable_va_base`, `at_table_*`, and `at_field_*`

### Cache behavior

- Per-VA configs are cached under a VA config cache key for 10 minutes.
- `SetConfigValue` and `SetVaConfig` both invalidate the per-VA cache entry after upsert.
- `GetAllCallsigns` also caches the aggregated callsign config list for 10 minutes.

### Save/update flow

Basic setup UI goes through `internal/vaadmin/handler.go`:

- `SaveBasicSetupHandler()` saves:
  - VA display name via `vaSvc.Update`
  - `callsign_prefix`
  - `callsign_suffix`
  - `enabled_server_ids`

The form is rendered by `templates/partials/basic-setup-form.html`.

### Retrieval/use

Basic config is actively used by:

- `internal/flights/service.go` for live-flight matching and IF server selection
- `internal/platform/va/readiness.go` for setup readiness
- `internal/pireps/service.go` for callsign handling
- `internal/liverymappings/handler.go` for defaults
- `internal/memberships/handler.go` for legacy Airtable base link generation

### Important note

`internal/platform/va/handler.go` contains generic config handlers:

- `GetConfigs()`
- `ListConfigKeys()`
- `SetConfigs()`

But these were **not observed mounted** in `internal/routes/router.go`.

## Datasource/Airtable config: current state

### Storage model

`va_data_provider_configs` is the structured config system.

Observed model in `internal/platform/va/model.go`:

- `provider_type`
- `config_type`
- `config_data`
- `config_version`
- `is_active`
- `validation_status`

Observed `config_type` usage:

- `credentials`
- `pilot`
- `route`
- `pirep`
- `career_mode`

### Does the current system support multiple Airtable configs?

Yes, **by type**.

It supports separate configs for:

- pilots
- routes
- PIREPs
- career mode

Each schema has its own:

- `table_name`
- `enabled`
- `last_modified_field`
- field mappings

So the same Airtable base can already support:

- one pilot table
- one route table
- one PIREP table

or all of those schema types can point to the **same Airtable table name** if an admin configures them that way.

### Current limit

The current system supports **one active config per schema type**, not many configs of the same type.

So today it supports:

- one `pilot`
- one `route`
- one `pirep`

per VA/provider, not multiple `pilot` configs or multiple `pirep` configs.

## Airtable schema/config DTOs

Observed in:

- `internal/platform/va/config_dtos.go`
- `internal/models/dtos/provider_config.go`

Supported schema structure includes:

- `entity_type`
- `table_name`
- `enabled`
- `fields`
- `last_modified_field`
- `career_mode_flight_mode`

Field mappings support richer metadata than the UI currently exposes:

- `internal_name`
- `airtable_name`
- `data_type`
- `required`
- `default_value`
- `display_name`
- `is_user_visible`
- `display_format`
- `bot_metadata`

## Datasource save/update/cache flows

### Credentials flow

API handlers:

- `POST /api/v1/admin/airtable/credentials`
- `GET /api/v1/admin/airtable/credentials`

UI handlers:

- `/dashboard/settings/datasource/credentials`

Implementation path:

- `internal/datasource/handler.go`
- `internal/platform/va/service.go: SaveAirtableCredentials`
- `internal/platform/va/repo.go: SaveAirtableCredentials`

Cache behavior:

- credentials cached under `airtable_creds:{vaID}`
- save invalidates that cache key

Dev comments:

- make sure apart from the save request, we are not fetching the credentials to the UI at any point to prevent token leakage.
- make sure we are using a common provider for the cache that is being used elsewhere and no specific implementation
- we can also standardise the config to `config:airtable_creds:{vaID}`

### Schema flow

API handlers:

- `POST /api/v1/admin/airtable/schema/{schemaType}`
- `GET /api/v1/admin/airtable/schema/{schemaType}`
- `GET /api/v1/admin/airtable/schemas`

UI handlers:

- `/dashboard/settings/datasource/schema/{schemaType}`
- `/dashboard/settings/datasource/schema/{schemaType}/sync`

Implementation path:

- `internal/datasource/handler.go`
- `internal/platform/va/service.go: SaveAirtableSchema`
- `internal/platform/va/repo.go: SaveAirtableSchema`

Cache behavior:

- individual schemas cached under `airtable_schema:{vaID}:{schemaType}`
- save invalidates that schema cache key
- `GetAirtableSchemas()` returns all schemas but does not cache the full schema map

## UI support currently present

### Basic setup UI

`templates/partials/basic-setup-form.html` supports:

- VA code display
- display name
- callsign prefix
- callsign suffix
- enabled live servers

### Datasource UI

`templates/pages/datasource.html` and related partials support:

- datasource status page
- Airtable credential entry
- connection test
- add/edit schema flow
- schema type selection for:
  - pilot
  - route
  - pirep
  - career_mode
- Airtable metadata field sync via `FetchTableFields`
- manual internal-to-Airtable field mapping

### UI limitation

The UI mostly supports:

- table name
- enabled toggle
- last modified field
- field mapping

But it does **not** expose all stored schema capabilities, especially:

- `display_format`
- `bot_metadata`
- more advanced user-visible field metadata
- `career_mode_flight_mode`

## Existing retrieval/provider patterns

### Modern-ish retrieval path

The cleaner path is `internal/platform/va/service.go`, which provides:

- `GetAirtableCredentials`
- `GetAirtableSchema`
- `GetAirtableSchemas`

This is used by:

- datasource UI
- some sync jobs
- newer platform-facing code

### Legacy retrieval path still present

`internal/common/airtable_service.go` still reads Airtable config from `va_configs` using legacy keys such as:

- `airtable_api_key`
- `airtable_va_base`
- `at_table_pilots`
- `at_table_routes`
- `at_table_pireps`

This path assumes legacy key/value Airtable storage and does not align with the newer per-type provider-config model.

## Important fragmentation findings

### 1. No single canonical quick-fetch provider exists

I did **not** find a single modern provider/facade that gives downstream features a unified object like:

- basic holding config
- Airtable credentials
- pilot schema
- route schema
- PIREP schema

Instead, multiple features manually repeat:

- fetch credentials row
- fetch entity schema row
- unmarshal JSON
- rebuild `ProviderCredentials`
- convert DTOs/platform schema types
- set `provider_credentials` on context

Repeated in:

- `internal/pilots/sync_job.go`
- `internal/va_routes/sync_job.go`
- `internal/pilots/stats_service.go`
- `internal/pireps/tour_service.go`
- `internal/pireps/service.go`

This strongly suggests a reusable provider/config accessor layer would be useful.

### 2. Old and new Airtable config systems coexist

There is a real split between:

- legacy Airtable config in `va_configs`
- new Airtable config in `va_data_provider_configs`

This is likely the biggest source of confusion for future fixes.

### 3. `DataProviderConfigService` looks out of sync with the current storage model

`internal/services/data_provider_config_service.go` still behaves as if there is one provider config blob per provider:

- `GetActiveConfigCached(ctx, vaID, "airtable")`
- `SaveOrUpdateConfig(...)` checks only one active provider config row

But the actual refactored schema is **multiple rows per provider** with `config_type`.

That service does not appear aligned with the newer row-per-type model.

### 4. PIREP submit path looks inconsistent

`internal/pireps/service.go` uses `DataProviderConfigService.GetActiveConfigCached(ctx, vaID, "airtable")` and expects a full `ProviderConfigData` with embedded schemas.

But the active repository/service model elsewhere is split into:

- one credentials row
- one row per schema type

Also, `submitToAirtable()` eventually calls `AirtableProvider.SubmitRecord`, which expects `provider_credentials` in context, while the PIREP service sets `provider_config` in context.

That path appears mismatched and should be reviewed before broader config work.

### 5. Migration/schema mismatch risk exists

Observed:

- `011_refactor_data_provider_configs.sql` includes `config_type`
- `000_schema.sql` still shows an older `va_data_provider_configs` definition without `config_type` and with a unique constraint on `(va_id, provider_type)`

This suggests fresh bootstrap schema and incremental migrations may be out of sync.

## Current consumers by feature area

### Basic config consumers

- `internal/flights/service.go`
- `internal/platform/va/readiness.go`
- `internal/pireps/service.go`
- `internal/liverymappings/handler.go`
- `internal/memberships/handler.go`

### Airtable credentials/schema consumers

- `internal/datasource/handler.go`
- `internal/pilots/sync_job.go`
- `internal/va_routes/sync_job.go`
- `internal/pireps/sync_job.go`
- `internal/pilots/stats_service.go`
- `internal/pireps/tour_service.go`
- `internal/pireps/service.go`

## What already works for your stated requirement

The current repo already supports the important structural requirement that Airtable may have:

- a pilots table
- a routes table
- a filed PIREPs table

and each can be configured independently.

It also already allows all of those schema types to point at the **same Airtable table name** if desired.

## Main gaps before planning changes

1. **Unify config retrieval** so features stop hand-building credentials/schema objects.
2. **Decide what to do with legacy `va_configs` Airtable keys** and legacy services.
3. **Resolve stale provider-config service usage**, especially in PIREP submit flow.
4. **Verify DB schema truth** between bootstrap schema and migration path.
5. **Decide whether one schema per type is enough** or whether “multiple configs of same type” is a real requirement.
6. **Expand UI only if needed** for advanced schema metadata already supported in code.

## Suggested direction for next planning pass

When converting this analysis into an implementation plan, the most valuable next target appears to be a single holding/provider config accessor that can answer questions like:

- basic VA identity + callsign matching config
- datasource enabled/configured state
- Airtable credentials
- pilot/route/pirep/career_mode schema lookup by type
- quick-read checks for menus, feature gates, and submission/sync flows

That accessor should likely sit on the modern platform side rather than extending the legacy `internal/common/*` services.
