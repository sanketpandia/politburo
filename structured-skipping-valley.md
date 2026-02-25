# Politburo Refactoring Plan

## Executive Summary

Reorganize the Politburo codebase from a technical layer-based structure to a domain-driven feature-based structure while consolidating infrastructure concerns.

**Goals:**
1. Consolidate infrastructure (logging, metrics, redis, database, airtable, liveAPI) into `infra/`
2. Reorganize `internal/` by domain/feature (flights/, pilots/, va/, configs/, etc.)
3. Adopt Google Go style guide for naming conventions (future code only)
4. Maintain zero circular dependencies
5. Keep all existing code running throughout migration

**Status:** Already started - `infra/logging/` and `infra/metrics/` exist but imports not updated

---

## Current State vs Target State

### Current Structure (Before)
```
internal/
├── api/                    # 26 handler files (mixed domains)
├── services/               # 12+ service files
├── db/repositories/        # 16 repository files
├── common/                 # 2,336 lines (infrastructure + business logic mixed)
├── models/                 # entities/, gorm/, dtos/
├── jobs/                   # 4 scheduled jobs
├── workers/                # 5 background workers
└── [auth, middleware, routes, constants, providers]
```

### Target Structure (After)
```
infra/                      # Infrastructure (horizontal layer)
├── db/                     # Database connections & migrations
├── cache/                  # Cache interface + implementations
├── redis/                  # Redis client + queue
├── airtable/               # Airtable HTTP client
├── liveapi/                # Live API HTTP client
├── logging/                # ✅ Already moved
└── metrics/                # ✅ Already moved

internal/
├── app/                    # Composition root (NEW - replaces dependencies.go)
│   ├── app.go              # Main app constructor with DI
│   └── config.go           # App configuration
│
├── platform/               # Cross-cutting concerns (NEW)
│   ├── users/              # User management (used by all features)
│   ├── airports/           # Airport reference data
│   └── roles/              # Role constants
│
├── flights/                # Feature: Flight tracking (NEW)
│   ├── handler.go          # HTTP handlers
│   ├── service.go          # Business logic
│   ├── model.go            # GORM models
│   ├── dtos.go             # Request/response DTOs
│   ├── worker.go           # Logbook worker
│   └── cache_job.go        # Live flights cache job
│
├── pireps/                 # Feature: PIREP filing (NEW)
│   ├── handler.go
│   ├── service.go
│   ├── repo.go
│   ├── model.go
│   ├── dtos.go
│   ├── queue_worker.go     # 5 concurrent workers
│   ├── monitor.go          # Queue monitor
│   └── sync_job.go         # Airtable sync
│
├── pilots/                 # Feature: Pilot management (NEW)
│   ├── handler.go
│   ├── service.go
│   ├── repo.go
│   └── sync_job.go
│
├── pilotstats/             # Feature: Pilot statistics (NEW)
│   ├── handler.go
│   ├── service.go
│   └── repo.go             # Cross-table queries
│
├── va/                     # Feature: Virtual airline management (NEW)
│   ├── handler.go
│   ├── service.go
│   ├── config_service.go
│   ├── repo.go
│   └── ui/                 # Vizburo VA pages
│
├── events/                 # Feature: VA events (NEW)
├── worldtour/              # Feature: World tours (NEW)
├── registration/           # Feature: User/server registration (NEW)
├── aircraft/               # Feature: Aircraft & liveries (NEW)
├── sync/                   # Feature: Airtable sync (NEW)
├── dataproviders/          # Feature: Data provider config (NEW)
│
├── auth/                   # ✅ Keep as-is
├── middleware/             # ✅ Keep as-is (auth, metrics, logging)
├── routes/                 # ✅ Keep, update to use app/
└── constants/              # ✅ Keep minimal
```

---

## Migration Strategy - Phased Approach

### Phase 0: Fix Broken Imports (Week 0 - URGENT)
**Status:** Already in progress
**Issue:** logging and metrics moved to `infra/` but imports not updated

**Files with broken imports (from diagnostics):**
- `api_routes.go:7` - importing `internal/metrics`
- `main.go:12` - importing `internal/logging`
- `router.go:13` - importing `internal/logging`
- `dependencies.go:7` - importing `internal/metrics`
- `init.go:7` - importing `internal/logging`
- `metrics.go:11,12` - importing `internal/logging`, `internal/metrics`

**Action:** Update all import statements:
```go
// OLD (broken)
"infinite-experiment/politburo/internal/logging"
"infinite-experiment/politburo/internal/metrics"

// NEW (correct)
"infinite-experiment/politburo/infra/logging"
"infinite-experiment/politburo/infra/metrics"
```

**Validation:**
```bash
# Find all broken imports
grep -r "internal/logging" --include="*.go" .
grep -r "internal/metrics" --include="*.go" .

# After fixing, verify build works
go build ./cmd/server
go test ./...
```

---

### Phase 1: Infrastructure Consolidation (Week 1-2)

**Goal:** Move all infrastructure from `internal/common/` and `internal/db/` to `infra/`

#### 1.1 Create `infra/` Directory Structure
```bash
mkdir -p infra/{db,cache,redis,airtable,liveapi,security,session,queue}
```

#### 1.2 Move Files to `infra/`

| Current Path | New Path | Package Name |
|-------------|----------|--------------|
| `internal/common/redis_client.go` | `infra/redis/client.go` | `package redis` |
| `internal/common/redis_cache_service.go` | `infra/cache/redis.go` | `package cache` |
| `internal/common/redis_queue_service.go` | `infra/queue/redis_queue.go` | `package queue` |
| `internal/common/cache_service.go` | `infra/cache/inmemory.go` | `package cache` |
| `internal/common/cache_interface.go` | `infra/cache/cache.go` | `package cache` |
| `internal/common/airtable_service.go` | `infra/airtable/client.go` | `package airtable` |
| `internal/common/live_api_service.go` | `infra/liveapi/client.go` | `package liveapi` |
| `internal/common/session_service.go` | `infra/session/session.go` | `package session` |
| `internal/common/url_signer.go` | `infra/security/url_signer.go` | `package security` |
| `internal/db/orm.go` | `infra/db/db.go` | `package db` |
| `internal/db/postgres.go` | `infra/db/db.go` | `package db` (merge) |
| `internal/db/migrations/` | `infra/db/migrations/` | N/A (SQL files) |
| `internal/providers/data_provider.go` | `infra/providers/interface.go` | `package providers` |
| `internal/providers/airtable_provider.go` | `infra/providers/airtable.go` | `package providers` |
| `internal/providers/live_api_provider.go` | `infra/providers/liveapi.go` | `package providers` |

#### 1.3 Update Package Names in Moved Files

**Example:** `infra/redis/client.go`
```go
// Before
package common

// After
package redis
```

#### 1.4 Update Import Statements (96 files affected)

**Strategy:** Use find-and-replace across codebase
```bash
# Update common imports to infra
find . -name "*.go" -type f -exec sed -i 's|infinite-experiment/politburo/internal/common|infinite-experiment/politburo/infra/cache|g' {} \;

# More granular updates needed for specific packages
# Redis client
sed -i 's|common\.NewRedisClient|redis.NewClient|g' **/*.go

# Cache service
sed -i 's|common\.CacheInterface|cache.Interface|g' **/*.go
sed -i 's|common\.CacheService|cache.Service|g' **/*.go
```

**Manual Review Required:**
- `internal/api/dependencies.go` - Central DI file, update all infra imports
- `internal/routes/router.go` - Route setup, update worker/job initialization
- All handler files in `internal/api/` - Update service references

#### 1.5 Validation
```bash
# Ensure no old imports remain
grep -r "internal/common" --include="*.go" . && echo "❌ Old imports found" || echo "✅ Clean"
grep -r "internal/providers" --include="*.go" . && echo "❌ Old imports found" || echo "✅ Clean"

# Build and test
go build ./cmd/server
go test ./...
```

**Critical Files to Review:**
- [internal/api/dependencies.go](internal/api/dependencies.go) - Lines 3-14 (imports)
- [internal/routes/router.go](internal/routes/router.go) - Lines 3-20 (imports)
- [cmd/server/main.go](cmd/server/main.go) - DB and logging initialization

---

### Phase 2: Platform Layer (Week 3)

**Goal:** Extract truly global concepts into `internal/platform/`

#### 2.1 Create Platform Structure
```bash
mkdir -p internal/platform/{users,airports,roles,httpdto}
```

#### 2.2 Move User Management to Platform

| Current | New | Notes |
|---------|-----|-------|
| `internal/models/gorm/user.go` | `platform/users/model.go` | User GORM model |
| `internal/db/repositories/user_repository_gorm.go` | `platform/users/repo.go` | User repository |
| `internal/services/user_service.go` | `platform/users/service.go` | User business logic |

**Why Platform?** Users are referenced by every feature (flights, pireps, pilots, VA, etc.)

#### 2.3 Move Airport Reference Data

| Current | New |
|---------|-----|
| `internal/models/gorm/airport.go` | `platform/airports/model.go` |
| `internal/db/repositories/airport_repository.go` | `platform/airports/repo.go` |
| `internal/common/airport_loader.go` | `platform/airports/loader.go` |

#### 2.4 Move Role Constants

| Current | New |
|---------|-----|
| `internal/constants/roles.go` | `platform/roles/roles.go` |

#### 2.5 Create Standard HTTP Response Format

**Create:** `platform/httpdto/response.go`
```go
package httpdto

import (
    "encoding/json"
    "net/http"
    "time"
)

type Response[T any] struct {
    Status       string `json:"status"`        // "ok" or "error"
    Result       T      `json:"result,omitempty"`
    Error        *Error `json:"error,omitempty"`
    ResponseTime int64  `json:"responseTimeMs"`
}

type Error struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

func WriteSuccess(w http.ResponseWriter, start time.Time, result interface{}, statusCode int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(Response[interface{}]{
        Status:       "ok",
        Result:       result,
        ResponseTime: time.Since(start).Milliseconds(),
    })
}

func WriteError(w http.ResponseWriter, start time.Time, code, message string, statusCode int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(Response[interface{}]{
        Status: "error",
        Error: &Error{
            Code:    code,
            Message: message,
        },
        ResponseTime: time.Since(start).Milliseconds(),
    })
}
```

---

### Phase 3: Feature Extraction - Pilot Program (Week 4-5)

**Goal:** Validate feature-based pattern with one complex domain

**Choose:** `flights/` - Complex feature with handlers, services, workers, jobs

#### 3.1 Create `internal/flights/` Structure
```bash
mkdir -p internal/flights
```

#### 3.2 Consolidate Flight Files

| Current | New | Notes |
|---------|-----|-------|
| `internal/api/flight_handlers.go` | `flights/handler.go` | Main handlers |
| `internal/api/flights.go` | `flights/handler.go` | Merge into handler.go |
| `internal/services/flights_service.go` | `flights/service.go` | Business logic |
| `internal/workers/logbook_worker.go` | `flights/worker.go` | Logbook processing |
| `internal/jobs/flights_cache_job.go` | `flights/cache_job.go` | Cache job |
| `internal/common/flight_data.go` | `flights/model.go` | Flight models |

#### 3.3 Create Standardized Handler Pattern

**File:** `internal/flights/handler.go`
```go
package flights

import (
    "net/http"
    "infinite-experiment/politburo/internal/platform/httpdto"
    "infinite-experiment/politburo/internal/va"
    "infinite-experiment/politburo/internal/aircraft"
)

type Handler struct {
    svc           *Service
    vaConfig      *va.ConfigService
    liveryService *aircraft.Service
}

func NewHandler(svc *Service, vaConfig *va.ConfigService, livery *aircraft.Service) *Handler {
    return &Handler{
        svc:           svc,
        vaConfig:      vaConfig,
        liveryService: livery,
    }
}

func (h *Handler) GetVALive() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Implementation
    }
}

func (h *Handler) GetUserFlights() http.HandlerFunc { /* ... */ }
func (h *Handler) GetServers() http.HandlerFunc { /* ... */ }
```

#### 3.4 Update Router Registration

**File:** `internal/routes/api_routes.go`
```go
// OLD
vaFlightsHandler := api.VaFlightsHandler(flightSvc, cfgSvc)
member.Get("/va/live", vaFlightsHandler)

// NEW
flightsHandler := flights.NewHandler(flightsSvc, vaCfgSvc, aircraftSvc)
member.Get("/va/live", flightsHandler.GetVALive())
member.Get("/user/flights", flightsHandler.GetUserFlights())
```

#### 3.5 Validation
- [ ] All flight endpoints work correctly
- [ ] Logbook worker processes flights
- [ ] Cache job runs on schedule
- [ ] Tests pass for flights package
- [ ] No circular dependencies introduced

**Document Lessons Learned** - This will inform remaining feature migrations

---

### Phase 4: Remaining Feature Migrations (Week 6-9)

**Extract features in dependency order:**

#### Week 6: Simple, Low-Coupling Features
1. **`aircraft/`**
   - Move: `aircraft_livery_service.go`, `aircraft_livery_repository.go`, `meta_cache_worker.go`
   - Models: `aircraft_livery.go`, `livery_airtable_mapping.go`

2. **`sync/`**
   - Move: `at_sync_service.go`, `at_sync_job.go`
   - Repositories: `pilot_at_synced_repository.go`, `route_at_synced_repository.go`

#### Week 7: Medium Complexity Features
3. **`pireps/`**
   - Handlers: `pirep_handlers.go`
   - Service: `pirep_submission_service.go`
   - Workers: `pirep_queue_worker.go`, `pirep_queue_monitor.go`, `pirep_data_backfill.go`
   - Jobs: `pirep_sync_job.go`
   - Repo: `pirep_at_synced_repository.go`

4. **`pilots/`**
   - Service: `pilot_management_service.go`
   - Jobs: `pilot_sync_job.go`, `pilot_linking_job.go`
   - Repo: `pilot_at_synced_repository.go`

5. **`pilotstats/`** (cross-feature queries)
   - Handlers: `pilot_stats_handlers.go`, `pilot_stats.go`
   - Service: `pilot_stats_service.go`

#### Week 8: Core Domain Features
6. **`va/`**
   - Handlers: `va_handlers.go`, `va_config_handlers.go`, `va.go`
   - Services: `va_management_service.go`, VA config from `common/`
   - Repositories: `va_gorm_repository.go`, `va_user_role_repository.go`
   - UI: Extract VA-related vizburo handlers

7. **`events/`**
   - Service: `va_event_service.go`
   - Repository: `va_event_repository.go`
   - UI: Event handlers from vizburo

8. **`worldtour/`**
   - Handlers: `world_tour_handlers.go`, `world_tour_bot_handlers.go`, `world_tour_admin_handlers.go`
   - Service: `world_tour_service.go`
   - Repository: `world_tour_repository.go`

#### Week 9: Orchestration Features
9. **`registration/`**
   - Handlers: `user_registration_v2.go`, `server_registration_v2.go`, `user_handlers.go`
   - Services: `registration_service_v2.go`, `registration_service.go`

10. **`dataproviders/`**
    - Already in `infra/providers/` from Phase 1
    - Handlers: `data_provider_config.go`
    - Service: `data_provider_config_service.go`

11. **`configs/`**
    - Handlers: `flight_modes_config.go`
    - Service: `flight_modes_config_service.go`, `flight_mode_validation_service.go`

---

### Phase 5: Composition Root (Week 10)

**Goal:** Centralize dependency injection in `internal/app/`

#### 5.1 Create App Package
```bash
mkdir -p internal/app
```

#### 5.2 Create App Constructor

**File:** `internal/app/app.go`
```go
package app

import (
    "infinite-experiment/politburo/infra/db"
    "infinite-experiment/politburo/infra/cache"
    "infinite-experiment/politburo/infra/redis"
    "infinite-experiment/politburo/internal/flights"
    "infinite-experiment/politburo/internal/pireps"
    // ... all features
)

type App struct {
    // Feature handlers (what routes need)
    Flights      *flights.Handler
    Pireps       *pireps.Handler
    Pilots       *pilots.Handler
    PilotStats   *pilotstats.Handler
    VA           *va.Handler
    Events       *events.Handler
    WorldTour    *worldtour.Handler
    Registration *registration.Handler
    Aircraft     *aircraft.Handler
    DataProvider *dataproviders.Handler
    Configs      *configs.Handler

    // Platform services
    Users        *users.Service
    Airports     *airports.Service

    // Infrastructure (rarely accessed directly)
    infra        *Infrastructure
}

type Infrastructure struct {
    DB         *gorm.DB
    Cache      cache.Interface
    Redis      *redis.Client
    Metrics    *metrics.Registry
}

func New(cfg Config) (*App, error) {
    // 1. Initialize infrastructure
    database := db.New(cfg.Database)
    cacheService := cache.New(cfg.Cache)
    redisClient := redis.NewClient(cfg.Redis)
    metricsReg := metrics.NewMetricsRegistry()

    infra := &Infrastructure{
        DB:      database,
        Cache:   cacheService,
        Redis:   redisClient,
        Metrics: metricsReg,
    }

    // 2. Initialize platform
    userRepo := users.NewRepo(database)
    userSvc := users.NewService(userRepo)
    airportRepo := airports.NewRepo(database)
    airportSvc := airports.NewService(airportRepo)

    // 3. Initialize features (in dependency order)
    aircraftRepo := aircraft.NewRepo(database)
    aircraftSvc := aircraft.NewService(aircraftRepo, cacheService)

    flightsRepo := flights.NewRepo(database)
    flightsSvc := flights.NewService(flightsRepo, cacheService)
    flightsHandler := flights.NewHandler(flightsSvc)

    // ... repeat for all features

    return &App{
        Flights:  flightsHandler,
        Pireps:   pirepsHandler,
        // ...
        Users:    userSvc,
        Airports: airportSvc,
        infra:    infra,
    }, nil
}
```

#### 5.3 Update Router

**File:** `internal/routes/router.go`
```go
func RegisterRoutes(upSince time.Time) http.Handler {
    r := chi.NewRouter()

    // Initialize app with all dependencies
    app, err := app.New(app.Config{
        Database: db.Config{/* ... */},
        Cache:    cache.Config{/* ... */},
        Redis:    redis.Config{/* ... */},
    })
    if err != nil {
        panic(err)
    }

    // Register routes with app
    RegisterAPIRoutes(r, app)
    RegisterUIRoutes(r, app)

    return r
}
```

**File:** `internal/routes/api_routes.go`
```go
func RegisterAPIRoutes(r chi.Router, app *app.App) {
    v1 := r.Group(func(r chi.Router) {
        r.Use(middleware.AuthMiddleware(app.Users))

        // Clean, simple route registration
        r.Get("/va/live", app.Flights.GetVALive())
        r.Post("/pireps/submit", app.Pireps.Submit())
        r.Get("/pilot/stats", app.PilotStats.Get())
        // ...
    })
}
```

#### 5.4 Delete Old Files
- [ ] Delete `internal/api/dependencies.go`
- [ ] Delete `internal/api/handlers.go`
- [ ] Delete empty `internal/common/` directory
- [ ] Delete empty `internal/services/` directory
- [ ] Delete empty `internal/db/repositories/` directory

---

### Phase 6: Cleanup & Documentation (Week 11)

#### 6.1 Final Cleanup
- [ ] Remove all `.old` files
- [ ] Remove empty directories
- [ ] Update `.gitignore` if needed
- [ ] Run `go mod tidy`

#### 6.2 Update Documentation

**Files to Update:**
1. **CLAUDE.md** - Update architecture section
   - New directory structure
   - Feature-based organization pattern
   - Import path patterns
   - Composition root explanation

2. **ARCHITECTURE_REVIEW.md** - Update with new patterns
   - Feature boundaries
   - Dependency rules
   - Testing patterns

3. **README.md** - Update development guide
   - New directory structure
   - How to add new features
   - Where to find things

4. **Create ARCHITECTURE.md** - Detailed architecture documentation
   - Dependency flow diagrams
   - Feature organization principles
   - Infrastructure layer explanation
   - Platform layer rationale

#### 6.3 Create Architecture Decision Records (ADRs)

**Create:** `docs/adr/` directory
```bash
mkdir -p docs/adr
```

**ADRs to write:**
1. `001-feature-based-organization.md` - Why domain-driven structure
2. `002-infrastructure-consolidation.md` - Why infra/ separation
3. `003-platform-layer.md` - Cross-cutting concerns pattern
4. `004-composition-root.md` - Centralized DI rationale

---

## File Naming Conventions (Google Go Style)

**Applies to NEW code only** - Do not refactor existing code just for naming

### Files in Feature Directories
```
handler.go       ✅ HTTP handlers (singular, not handlers.go)
service.go       ✅ Business logic (singular)
repo.go          ✅ Data access (singular, not repository.go)
model.go         ✅ GORM models (singular)
dtos.go          ✅ Request/response DTOs (plural)
worker.go        ✅ Background workers (singular or specific name)
sync_job.go      ✅ Specific job (descriptive name)
cache_job.go     ✅ Specific job (descriptive name)
```

### Package Naming
```go
package flights    ✅ Plural, lowercase, single word
package pireps     ✅ Plural, lowercase, single word
package va         ✅ Abbreviation (acceptable)
package flight     ❌ Should be plural if multiple concepts
```

### Type Naming (Don't Repeat Package Name)
```go
// In flights package
type Handler struct {}       ✅ flights.Handler
type Service struct {}       ✅ flights.Service
type Repo struct {}          ✅ flights.Repo

type FlightHandler struct {} ❌ Redundant
type FlightsService struct {} ❌ Plural redundant
```

### Constructor Naming
```go
func NewHandler(svc *Service) *Handler                           ✅ Primary
func NewHandlerWithCache(svc *Service, cache cache.Cache) *Handler  ✅ Alternative
func NewFlightHandler() *Handler                                 ❌ Redundant
```

**Reference:** https://google.github.io/styleguide/go/guide

---

## Testing Strategy

### Per-Phase Validation

**After Phase 0 (Fix Broken Imports):**
```bash
go build ./cmd/server
go test ./...
```

**After Phase 1 (Infrastructure):**
```bash
go test ./infra/...
go test ./internal/...
make test
```

**After Phase 3-4 (Each Feature):**
```bash
# Test new feature package
go test ./internal/flights/...

# Regression test
make test
make test-api

# Verify zero circular dependencies
go list -json ./... | jq -r 'select(.Deps != null) | .ImportPath + " -> " + (.Deps | join(", "))'
```

**After Phase 5 (Composition Root):**
```bash
# Full integration test
go build ./cmd/server
go test ./...
./run-script.sh  # If it exists
```

### Manual Testing Checklist

After each major phase:
- [ ] Health check endpoint: `GET /healthCheck`
- [ ] User registration flow works
- [ ] PIREP submission works
- [ ] Flight data retrieval works
- [ ] Vizburo dashboard loads
- [ ] Background workers start successfully
- [ ] Redis connection stable
- [ ] Metrics endpoint works: `GET /metrics`

---

## Dependency Management

### Zero Circular Dependencies

**Current State:** ✅ Zero circular dependencies (MUST MAINTAIN)

**Check after each phase:**
```bash
go list -f '{{.ImportPath}} {{join .Imports "\n"}}' ./... | grep cycle
# Should return nothing
```

### Allowed Dependency Rules

```
✅ Allowed:
- Features → Platform (e.g., flights → users)
- Features → Infrastructure (e.g., flights → cache)
- Features → Features (sparingly, with justification)
- Platform → Infrastructure
- App → Features + Platform
- Routes → App

❌ Forbidden:
- Infrastructure → Features
- Infrastructure → Platform
- Platform → Features
- Features ↔ Features (circular)
```

### Enforcement Script

**Create:** `scripts/check_dependencies.sh`
```bash
#!/bin/bash
echo "Checking for circular dependencies..."
if go list -f '{{.ImportPath}} {{join .Imports "\n"}}' ./... | grep -i cycle; then
    echo "❌ Circular dependency detected!"
    exit 1
fi

echo "✅ No circular dependencies"
```

**Run after each phase:**
```bash
chmod +x scripts/check_dependencies.sh
./scripts/check_dependencies.sh
```

---

## Critical Files for Implementation

### Must Review/Update in Each Phase

**Phase 0:**
1. [cmd/server/main.go](cmd/server/main.go) - Lines 12 (logging import)
2. [internal/routes/router.go](internal/routes/router.go) - Lines 13 (logging import)
3. [internal/api/dependencies.go](internal/api/dependencies.go) - Lines 7 (metrics import)
4. All files in diagnostics with broken imports

**Phase 1:**
1. [internal/api/dependencies.go](internal/api/dependencies.go) - Central DI, lines 3-14 (all imports), 59-182 (initialization)
2. [internal/routes/router.go](internal/routes/router.go) - Worker/job initialization, lines 80-119
3. All 26 handler files in `internal/api/` - Service references

**Phase 5:**
1. [internal/api/dependencies.go](internal/api/dependencies.go) - Will be replaced by `internal/app/app.go`
2. [internal/routes/router.go](internal/routes/router.go) - Will use app instead of deps
3. [internal/routes/api_routes.go](internal/routes/api_routes.go) - Route registration

---

## Success Criteria

### Build & Test Metrics
- [ ] Zero circular dependencies (enforced)
- [ ] All tests passing (maintain or improve coverage)
- [ ] `go build ./cmd/server` succeeds
- [ ] `go test ./...` succeeds
- [ ] No performance regression

### Code Organization Metrics
- [ ] `internal/common/` deleted (empty)
- [ ] `internal/api/dependencies.go` deleted
- [ ] All infrastructure in `infra/`
- [ ] All features in domain directories
- [ ] Platform layer created with users, airports, roles

### Documentation Completeness
- [ ] CLAUDE.md updated
- [ ] ARCHITECTURE.md created
- [ ] ADRs written (4 documents)
- [ ] README.md updated

---

## Rollback Strategy

### Per-Phase Rollback

**Git Branch Strategy:**
```bash
# Create branch per phase
git checkout -b refactor/phase-0-fix-imports
# Do work, test thoroughly
git commit -m "Phase 0: Fix broken imports"

# Only merge if tests pass
git checkout main
git merge refactor/phase-0-fix-imports

# If issues arise
git revert <commit-hash>
```

**Keep Old Code Functional:**
- Don't delete files until all imports updated
- Use temporary aliases during transition
- Each phase should leave code in runnable state

---

## Timeline

**Total: ~11 weeks (55-60 working days)**

| Week | Phase | Focus | Milestone |
|------|-------|-------|-----------|
| 0 | Phase 0 | Fix broken imports | Build works, no errors |
| 1-2 | Phase 1 | Infrastructure to infra/ | All infra consolidated |
| 3 | Phase 2 | Platform layer | Users, airports in platform/ |
| 4-5 | Phase 3 | Flights pilot program | Pattern validated |
| 6 | Phase 4a | aircraft/, sync/ | Simple features done |
| 7 | Phase 4b | pireps/, pilots/, pilotstats/ | Complex features done |
| 8 | Phase 4c | va/, events/, worldtour/ | Core domains done |
| 9 | Phase 4d | registration/, providers/, configs/ | All features migrated |
| 10 | Phase 5 | Composition root | Single DI point |
| 11 | Phase 6 | Cleanup & docs | Production ready |

---

## Verification Steps (Run After Completion)

### Final Validation Checklist

```bash
# 1. Build succeeds
go build ./cmd/server

# 2. All tests pass
go test ./...

# 3. No circular dependencies
go list -f '{{.ImportPath}} {{join .Imports "\n"}}' ./... | grep cycle
# Should return nothing

# 4. No old import paths
grep -r "internal/common" --include="*.go" . && echo "❌ Found" || echo "✅ Clean"
grep -r "internal/services" --include="*.go" . && echo "❌ Found" || echo "✅ Clean"
grep -r "internal/db/repositories" --include="*.go" . && echo "❌ Found" || echo "✅ Clean"

# 5. Ensure infra doesn't import from features
grep -r "internal/flights" infra/ --include="*.go" && echo "❌ Violation" || echo "✅ Clean"
grep -r "internal/pireps" infra/ --include="*.go" && echo "❌ Violation" || echo "✅ Clean"

# 6. Run linters
go vet ./...
golangci-lint run

# 7. Check test coverage
go test -cover ./... | grep -E "coverage|FAIL"

# 8. Verify app starts
go run ./cmd/server &
sleep 5
curl http://localhost:8080/healthCheck
kill %1
```

### Manual Verification

- [ ] Server starts without errors
- [ ] Metrics endpoint accessible: http://localhost:8080/metrics
- [ ] Vizburo dashboard loads: http://localhost:8080/dashboard
- [ ] API endpoints respond correctly
- [ ] Background workers running (check logs)
- [ ] Redis connection healthy
- [ ] Database queries working

---

## Quick Reference: Where Should This File Go?

```
Is it technical infrastructure? (Redis, DB, HTTP client)
├─ YES → infra/{category}/
└─ NO ↓

Is it used by ALL features? (Users, Airports, Roles)
├─ YES → internal/platform/{domain}/
└─ NO ↓

Is it business logic for ONE feature? (Flights, PIREPs)
├─ YES → internal/{feature}/
│   ├─ handler.go
│   ├─ service.go
│   ├─ repo.go
│   ├─ model.go
│   ├─ dtos.go
│   └─ worker.go / job files
└─ NO ↓

Is it a cross-cutting concern? (Auth, Middleware)
├─ YES → internal/{concern}/ (keep as-is)
└─ NO → Ask for guidance
```

---

## End of Plan

This plan provides a systematic, phased approach to refactoring the Politburo codebase while maintaining zero downtime and backward compatibility. Each phase builds on the previous one with clear validation steps and rollback strategies.
