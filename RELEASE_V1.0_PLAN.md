# Politburo v1.0 Release Plan

**Release Week:** January 27 - February 2, 2026
**Target Release Date:** February 2, 2026
**Current Branch:** world_tour → main
**Last Updated:** January 29, 2026 (22:00 UTC - Handler refactoring mostly complete)

---

## Current Status Summary

### Overall Progress: ~55% Complete

**Completed:**
- ✅ GORM migration (sqlx completely removed)
- ✅ Core bot commands implemented (7/7 endpoints functional)
- ✅ Vizburo UI features operational
- ✅ World Tour feature complete
- ✅ Events management system
- ✅ Intelligent cache jobs created (SessionCacheJob, FlightsCacheJob with phase detection)
- ✅ Handler refactoring (10 handlers created, FlightHandlers & VAHandlers added)
- ✅ Legacy handler files deprecated (flights.go, maps.go, va.go)
- ✅ Cache job compilation fixes (flights_cache_job.go)
- ✅ Vizburo sqlx removal fix (main.go)

**In Progress:**
- 🟡 Test suite fixes (compilation errors from API changes)

**Blocked/Not Started:**
- 🔴 Live flights endpoint validation (item #3)
- 🔴 End-to-end testing flows (items #4, #5)
- 🔴 Metrics and monitoring implementation (item #7)
- 🔴 Docker infrastructure modernization (item #8)
- 🔴 Worker refactoring (item #9)
- 🔴 Help command API (item #10)

**Critical Blockers:**
1. ⚠️ Test compilation errors must be fixed before merge
2. ⚠️ Integration testing required before production deploy

---

## Release Goals

This release focuses on **production readiness** with:
- ✅ Complete GORM migration (remove sqlx) - **DONE**
- ✅ Stable API architecture - **95% complete** (handler refactoring done)
- 🔴 Full monitoring stack - **Not started**
- 🔴 Comprehensive testing - **Tests need fixes**
- 🔴 Clean infrastructure - **Docker work needed**
- ✅ Production-ready features - **Core features functional**

---

## Immediate Action Items (Priority Order)

### Priority 1: Fix Blocking Issues ⚠️
1. **Fix test compilation errors** (2-3 hours)
   - Update mock interfaces in test files
   - Fix APIKeyClaims field references
   - Update InitUserRegistration call signatures
   - Files: `user_registration_v2_test.go`, `registration_service_v2_test.go`

2. **Fix cache job compilation errors** (30-60 minutes)  --> Done
   - Fix RedisCacheService method calls in `flights_cache_job.go`
   - Change `SetJSON/GetJSON` to use `Set/Get` with proper serialization
   - Fix context parameter usage (cache methods don't take context)
   - Fix undefined `flights` variable reference (should be `flightsResp.Flights`)
   - Files: `internal/jobs/flights_cache_job.go`

### Priority 2: Complete Handler Migration (4-6 hours)
2. **Finish handler refactoring**
   - Migrate remaining endpoints from old handlers to new pattern
   - Update router to use all new handlers
   - Deprecate/remove old handler files
   - Ensure all tests pass

### Priority 3: Bot Integration Enhancements (3-4 hours)
3. **Enhance bot command responses**
   - Add VA connectivity check to /health endpoint
   - Generate signed dashboard links for /live and /logbook commands
   - Enhance /status endpoint with more details
   - Add help text references to responses

### Priority 4: Testing & Validation (6-8 hours)
4. **End-to-end testing**
   - Test complete registration flow
   - Test PIREP submission flow
   - Validate cache filling jobs
   - Load test critical endpoints

### Priority 5: Production Readiness (10-12 hours)
5. **Infrastructure & monitoring**
   - Implement Prometheus metrics
   - Set up Grafana dashboards
   - Modernize Docker configuration
   - Refactor workers for graceful shutdown

---

## Bot Commands Implementation Status

These commands are implemented for Discord bot integration via Politburo API:

### 1. /health Command ✅ IMPLEMENTED
- **Endpoint:** `GET /healthCheck`
- **Status:** Functional - Returns API status, DB status, uptime
- **TODO for v1.0:**
  - [ ] Add VA connectivity check (validate Airtable/Live API access)
  - [ ] Include help/status command references in response message
  - [ ] Add Redis health check

### 2. /register Command ✅ IMPLEMENTED
- **Endpoints:**
  - `POST /api/v1/user/register/init` - Initialize user registration
  - `POST /api/v1/user/register/link` - Link user to VA
- **Status:** Fully functional
- **Handler:** `handlers.User` (old pattern, works but needs test fixes)

### 3. /live Command ✅ IMPLEMENTED
- **Endpoint:** `GET /api/v1/va/live` (member-level access)
- **Status:** Functional
- **Handler:** `api.VaFlightsHandler(flightSvc)`
- **TODO for v1.0:**
  - [ ] Generate signed link to vizburo live flights UI
  - [ ] Validate cache strategy (1-minute TTL for live data)

### 4. /logbook Command ✅ IMPLEMENTED
- **Endpoint:** `GET /api/v1/user/{user_id}/flights` (staff-level access)
- **Status:** Functional
- **Handler:** `api.FetchFlights(userFlightsHandler)`
- **TODO for v1.0:**
  - [ ] Update bot response message to include UI link
  - [ ] Generate signed link to pilot's logbook in vizburo

### 5. /stats Command ✅ IMPLEMENTED
- **Endpoint:** `GET /api/v1/pilot/stats` (member-level access)
- **Status:** Fully functional with new handler pattern
- **Handler:** `pilotStatsHandlers.GetPilotStats()`
- **Returns:** Comprehensive stats (IF game stats, last flight, VA flights, provider data)

### 6. /status Command 🟡 PARTIAL
- **Endpoint:** `GET /api/v1/user/details` (authenticated)
- **Status:** Exists but may need enhancement
- **TODO for v1.0:**
  - [ ] Verify it returns registration status
  - [ ] Ensure VA link status included
  - [ ] Add last activity timestamp

### 7. /pirep Command ✅ IMPLEMENTED
- **Endpoints:**
  - `GET /api/v1/pireps/config` - Get available flight modes
  - `POST /api/v1/pireps/submit` - Submit PIREP
- **Status:** Fully functional with new handler pattern
- **Handler:** `pirepHandlers` (queue-based processing)

---

## Cache Filling Jobs Validation

### ✅ Aircrafts and Liveries Parsing
- **Worker:** MetaCacheWorker (runs every 6 hours)
- **Status:** Implemented
- **Location:** `internal/workers/meta_cache_worker.go`
- **TODO:** Validate sync logic and change detection

### ✅ Session Cache Job (NEW)
- **Job:** SessionCacheJob
- **Status:** ✅ Implemented
- **Location:** `internal/jobs/session_cache_job.go`
- **Features:**
  - Fetches all Infinite Flight sessions/servers from Live API
  - Caches each session object with 24-hour TTL
  - Caches session names separately for quick lookups
  - Maintains master list of session IDs for other jobs to consume
- **Cache Keys:**
  - `if:session:{sessionID}` - Full session object (24h TTL)
  - `if:session:name:{sessionID}` - Session name (24h TTL)
  - `if:sessions` - Pipe-delimited list of all session IDs (24h TTL)

### ✅ Flights Cache Job with Intelligence (NEW)
- **Job:** FlightsCacheJob
- **Status:** ✅ Implemented (needs compilation fixes)
- **Location:** `internal/jobs/flights_cache_job.go`
- **Features:**
  - Fetches live flights for all sessions from cache
  - **Intelligent flight phase detection**: on_ground, takeoff, climb, cruise, descent, landed
  - **Phase-based polling strategy**: Different refresh intervals per flight phase
  - Enhanced flight data with session context
  - Caches callsign lists per session for quick lookups
  - Flight state tracking with next poll time calculation
- **Flight Phases Tracked:**
  - `on_ground`: Speed < 50 knots → Poll every 2 minutes
  - `takeoff`: Speed > 80 knots from ground → Poll every 5 minutes
  - `climb`: Altitude > 8000 feet → Poll every 5 minutes
  - `cruise`: Altitude > 30000 feet or speed > 300 knots → Poll every 5 minutes
  - `descent`: Altitude < 15000 feet from cruise → Poll every 5 minutes
  - `landed`: Speed < 50 after active flight → Poll every 2 minutes
- **Enhanced Data Captured:**
  - Flight ID, callsign, username
  - Session ID and session name
  - Speed, altitude, lat/long, heading
  - **Flight phase** (derived from speed/altitude analysis)
  - **Takeoff time** (captured on phase transition)
  - **Landing time** (captured on phase transition)
  - Last updated timestamp
- **Cache Keys:**
  - `if:flight:{flightID}` - Enhanced flight object (5min TTL)
  - `if:flight:state:{flightID}` - Flight state tracking (1h TTL)
  - `if:session:callsigns:{sessionID}` - Pipe-delimited callsign list (5min TTL)

### 🟡 Live Flights API Endpoint
- **Endpoint:** `GET /api/v1/va/live`
- **Status:** Functional but needs enhancement
- **TODO:**
  - [ ] Integrate with new FlightsCacheJob data
  - [ ] Add flight phase information to response
  - [ ] Include takeoff/landing timestamps
  - [ ] Add intelligent polling metadata to response
  - [ ] Verify cache strategy (1-minute TTL for API, 5-minute for flight data)
  - [ ] Validate VA regex filtering
  - [ ] Add flight phase filtering options (e.g., only active flights)

### 🟡 Routes Synchronization
- **Job:** RouteSyncJob (runs every 10 minutes)
- **Status:** Implemented but needs validation
- **Location:** `internal/jobs/route_sync_job.go`
- **TODO:**
  - [ ] Validate coordinate enrichment from airports table
  - [ ] Test sync frequency and performance

### 📋 TODO: Job Scheduling Integration
- [ ] Register SessionCacheJob with job scheduler (run every 5 minutes)
- [ ] Register FlightsCacheJob with job scheduler (run every 1 minute for live updates)
- [ ] Fix compilation errors in flights_cache_job.go (RedisCacheService method signatures)
- [ ] Add job monitoring metrics (last run time, duration, success/failure counts)
- [ ] Implement graceful shutdown for jobs
- [ ] Add configuration for job intervals

---

## Vizburo UI Features Status

### ✅ 1. Pilots Tab (Admin/Staff)
- **Location:** `vizburo/ui/templates/pages/pilots.html`
- **Features:**
  - View all pilots
  - Update roles (pilot/staff/admin)
  - Delete users
  - Update callsigns
- **Status:** Fully functional

### ✅ 2. Live Flights View
- **Location:** Vizburo dashboard
- **Features:**
  - Real-time VA flights
  - Aircraft/livery information
  - Flight progress indicators
- **Status:** Functional

### ✅ 3. Logbook with Search
- **Location:** `vizburo/ui/templates/pages/logbook.html`
- **Features:**
  - Interactive map visualization (Gleo library)
  - Search and filter by date, aircraft, status
  - Staff/Admin access
  - Search suggestions for VA users
- **Status:** Fully functional

---

## Intelligent Live Flight Caching (NEW for v1.0)

### Overview
Implemented intelligent caching system for live flight data with phase-based polling strategy to optimize API usage and provide richer flight tracking data.

### Architecture Enhancements

#### 1. Flight Phase Detection
The system now tracks each flight through its lifecycle:
- **On Ground**: Aircraft stationary or taxiing (< 50 knots)
- **Takeoff**: Acceleration phase (> 80 knots from ground)
- **Climb**: Ascending to cruise altitude (> 8000 feet)
- **Cruise**: Level flight at altitude (> 30000 feet or > 300 knots)
- **Descent**: Approaching destination (< 15000 feet from cruise)
- **Landed**: Flight completed (< 50 knots after active flight)

#### 2. Intelligent Polling Strategy
Poll frequencies adjusted based on flight phase to reduce API load:
- **Active phases** (takeoff, climb, cruise, descent): 5-minute intervals
- **Inactive phases** (on ground, landed): 2-minute intervals
- **Unknown phase**: 1-minute intervals (more frequent until classified)

#### 3. Enhanced Flight Data
Each cached flight now includes:
- Standard telemetry (speed, altitude, position, heading)
- **Session context** (session ID, session name)
- **Flight phase** (current phase of flight)
- **Timestamps** (takeoff time, landing time when applicable)
- **State tracking** (previous values for trend analysis)
- **Next poll time** (when to refresh this flight's data)

#### 4. Session Management
Session cache provides foundation for flight tracking:
- All sessions cached with 24-hour TTL
- Session names cached separately for quick lookups
- Master session list maintained for job coordination

### Benefits
1. **Reduced API Load**: Smart polling reduces unnecessary requests to Live API
2. **Richer Data**: Flight phases enable better analytics and user experience
3. **Event Detection**: Automatic capture of takeoff/landing times
4. **Better UX**: Phase information can drive UI features (e.g., "in flight" badges)
5. **Future Analytics**: Phase data enables flight pattern analysis

### Integration Points
- `/api/v1/va/live` endpoint will use enhanced cached data
- Discord bot can display flight phases to users
- Vizburo dashboard can show live flight phases on map
- PIREP validation can check if flight is actually in progress

### Next Steps
1. Fix compilation errors in FlightsCacheJob
2. Integrate jobs with scheduler
3. Update API responses to include flight phase data
4. Add job monitoring metrics
5. Test phase detection logic with real flights

---

## Future Development Plans (Post v1.0)

### Flight Mode Enhancements
- **EVENT mode** - Special flight mode for community events with unique handling:
  - Auto-linking to event records
  - Event-specific validation rules
  - Integration with events management system
  - Custom Discord notifications for event PIREPs
- Support for route-specific modes (store specific route regardless of actual route flown)
- Flight mode templates library (pre-configured common modes)
- Advanced field validation per mode (regex, ranges, dependencies)
- Conditional fields (show/hide based on other field values)

### Service Separation of Concerns
These architectural improvements are planned for v1.1+:

1. **IF Live Flights Service**
   - `getFlightDetailsWithRoute()` - Fetch single flight with route data
   - `getVAFlights()` - Filter flights by VA regex

2. **User Handling Service**
   - `addUser()` - User registration
   - `removeUser()` - User deletion with cleanup

3. **VA Handling Service**
   - `addVA()` - Create new virtual airline
   - `configureVA()` - Update VA settings

4. **VA Relationships Service**
   - `addRelationship()` - Link user to VA
   - `removeRelationship()` - Unlink user from VA
   - `changeRole()` - Update user role in VA
   - `getStatus()` - Get VA membership status

5. **Logbook Service**
   - UI screens enhancement
   - Paginated logbook API
   - Advanced filtering

6. **PIREP Service Enhancements**
   - `savePirep()` - Enhanced storage
   - `getUserPirepHistory()` - Historical view
   - Flight mode CRUD (add, remove, activate, deactivate)
   - `listFlightModes()` - Mode discovery

---

## Critical Path Items

### 1. Remove All sqlx Dependencies ⚠️ BREAKING CHANGE

**Status:** ✅ COMPLETED (with test fixes needed)
**Priority:** Critical (blocks other work)
**Completed:** January 2026

**Scope:**
- Migrate all repositories from sqlx to GORM
- Update connection initialization in main.go
- Remove sqlx from go.mod
- Update all raw SQL queries to GORM methods

**Completed Actions:**
- ✅ Migrated all repositories to GORM
- ✅ Deleted old sqlx-based repositories (user_repository.go, va_repository.go, postgres.go)
- ✅ Removed sqlx from go.mod
- ✅ Application builds and runs successfully with GORM only
- ✅ All GORM repositories functional (va_gorm_repository.go, etc.)

**Remaining Issues:**
- ⚠️ Test files have compilation errors due to:
  - Mock interfaces don't match updated signatures
  - APIKeyClaims structure changed (removed DiscordUserIDValue, DiscordServerIDValue fields)
  - InitUserRegistration signature changed (requires more parameters)
- **Files needing fixes:**
  - `internal/api/user_registration_v2_test.go`
  - `internal/services/registration_service_v2_test.go`

**Acceptance Criteria:**
- ✅ All repositories use GORM
- ✅ No imports of `github.com/jmoiron/sqlx` in codebase
- ⚠️ Tests need fixing (compilation errors)
- ✅ No `db.DB` (sqlx) references, only `db.PgDB` (GORM)
- ✅ Application builds and runs without sqlx dependency

**Testing:**
- ✅ User registration flow works (runtime)
- ✅ VA configuration works (runtime)
- ✅ API key validation works (runtime)
- ✅ All CRUD operations functional (runtime)
- ⚠️ Unit tests need update for new signatures

---

### 2. Finish Refactoring Endpoints

**Status:** 🟡 Mostly Complete (10 handlers created, legacy files deprecated)
**Priority:** High
**Estimated Time:** 1-2 hours remaining (test fixes only)

**Completed Handlers (New Pattern):**
- ✅ PirepHandlers (pirep_handlers.go) - Used in routes
- ✅ VAConfigHandlers (va_config_handlers.go) - Used in routes
- ✅ PilotStatsHandlers (pilot_stats_handlers.go) - Used in routes
- ✅ WorldTourHandlers (world_tour_handlers.go) - Used in routes (both admin and bot)
- ✅ JobsHandlers (jobs_handlers.go) - Used in routes
- ✅ UserHandlers (user_handlers.go) - Used in routes via Handlers wrapper
- ✅ WorldTourAdminHandlers (world_tour_admin_handlers.go) - Separate admin features
- ✅ WorldTourBotHandlers (world_tour_bot_handlers.go) - Separate bot features
- ✅ FlightHandlers (flight_handlers.go) - **NEW** - All flight endpoints migrated
- ✅ VAHandlers (va_handlers.go) - **NEW** - VA management endpoints migrated

**Legacy Files Status:**
- ✅ handlers.go - Used as wrapper for UserHandlers (keep)
- ✅ user.go - Deprecated (marked with notice)
- ✅ flights.go - Deprecated (marked with notice, replaced by flight_handlers.go)
- ✅ maps.go - Deprecated (marked with notice, replaced by flight_handlers.go)
- ✅ va.go - Deprecated (marked with notice, replaced by va_handlers.go)
- ⚠️ health.go - Standalone, could be refactored but not critical
- ✅ response.go - Utility file (keep)
- ✅ dependencies.go - Dependency injection container (keep)

**Completed Work (Jan 29):**
- [x] Migrate flight endpoints → FlightHandlers
  - [x] GetVALiveFlights() - GET /api/v1/va/live
  - [x] GetLiveSessions() - GET /api/v1/live/sessions
  - [x] GetUserFlights() - GET /api/v1/user/{user_id}/flights
  - [x] GetFlightFromCache() - GET /public/flight
  - [x] GetUserFlightsFromCache() - GET /public/flight/user
- [x] Migrate VA management endpoints → VAHandlers
  - [x] SyncUser() - POST /api/v1/va/userSync
  - [x] SetRole() - POST /api/v1/va/setRole
  - [x] GetConfigs() - GET /api/v1/va/configs
  - [x] ListConfigKeys() - GET /api/v1/va/configs/keys
  - [x] SetConfigs() - POST /api/v1/va/configs
- [x] Update router (api_routes.go) to use new handler instances
- [x] Deprecate old handler files (flights.go, maps.go, va.go)
- [x] Fix flights_cache_job.go compilation errors
- [x] Fix vizburo main.go db.InitPostgres error

**Remaining Work:**
- [ ] Fix broken test files (user_registration_v2_test.go, registration_service_v2_test.go)
- [ ] Clean up unused parameters from RegisterAPIRoutes function signature (optional)

**Acceptance Criteria:**
- ✅ All API endpoints use feature-based handler structs
- ✅ All handlers use `common.RespondSuccess/RespondError`
- ✅ All handlers track `initTime` for response metrics
- ✅ Router uses new handler constructors
- ✅ Old handler files deleted or marked deprecated
- [ ] All endpoint tests pass

---

### 3. Make Live Flights Endpoint Functional

**Status:** 🔴 Not Started
**Priority:** High
**Estimated Time:** 4-6 hours

**Reference:** `docs/dev/live_flights.md`

**Implementation:**
1. **Create LiveFlightsHandler** in FlightHandlers
   - Endpoint: `GET /api/v1/virtual_airlines/{va_id}/live_flights`
   - Fetch VA config from cache (1 hour TTL)
   - Query Live API for all flights (1 minute cache)
   - Filter by VA regex pattern
   - Fetch routes for each flight (10 minute cache per flight)
   - Parse aircraft/livery info (1 day cache)

2. **Cache Strategy:**
   ```
   live_flights                     → 1 minute TTL
   live_flight_route_{flight_id}    → 10 minutes TTL
   aircraft_info_{type_id}          → 1 day TTL
   va_config_{va_id}                → 1 hour TTL
   ```

3. **Response Format:**
   ```json
   {
     "flights": [
       {
         "flight_number": "AA123",
         "airline": "American Airlines",
         "departure_airport": "KJFK",
         "arrival_airport": "KLAX",
         "aircraft_type": "Boeing 777",
         "livery": "Standard",
         "estimates": {
           "departure_time": "2024-06-15T14:30:00Z",
           "arrival_time": "2024-06-15T17:45:00Z",
           "on_ground_time": 1800,
           "flight_time": 13499
         },
         "using_autopilot": true,
         "route_cached": true,
         "if_community_username": "pilot123"
       }
     ],
     "last_updated": "2024-06-15T12:00:00Z",
     "total_flights": 2
   }
   ```

**Files to Create/Modify:**
- `internal/api/flight_handlers.go` - Add `GetLiveFlights()` method
- `internal/routes/api_routes.go` - Register route
- `internal/services/flights_service.go` - Add live flight filtering logic

**Acceptance Criteria:**
- [ ] Endpoint returns live flights for VA
- [ ] Flights filtered by VA regex correctly
- [ ] Route data cached efficiently
- [ ] Response time < 2 seconds
- [ ] Cache keys use correct TTLs
- [ ] Error handling for Live API failures

---

### 4. Test Registration and VA Setup Flows

**Status:** 🔴 Not Started
**Priority:** Critical
**Estimated Time:** 3-4 hours

**Test Scenarios:**

#### 4.1. User Registration Flow
1. **Discord user runs `/register` command**
   - Bot calls `POST /api/v1/user/register/init`
   - Request: `{"discord_user_id": "123", "discord_username": "pilot"}`
   - Expected: User created in database with UUID

2. **User selects VA to join**
   - Bot calls `POST /api/v1/user/register/link`
   - Request: `{"user_uuid": "...", "discord_server_id": "456"}`
   - Expected: User linked to VA with "pilot" role

3. **Verify registration**
   - Call `GET /api/v1/user/details`
   - Expected: User details with VA affiliations

#### 4.2. VA Setup Flow
1. **Admin initializes server**
   - Call `POST /api/v1/server/init`
   - Request: `{"discord_server_id": "456", "va_name": "Test VA"}`
   - Expected: VA created, admin role assigned

2. **Admin configures VA**
   - Call `POST /api/v1/va/configs`
   - Set Airtable config, regex patterns, etc.
   - Expected: Config saved and cached

3. **Admin sets flight modes**
   - Call `POST /api/v1/va/flight-modes/config`
   - Add "classic" and "career" modes
   - Expected: Flight modes available for PIREP submission

**Test Environment:**
- Use separate test Discord server
- Test PostgreSQL database
- Redis cache instance

**Acceptance Criteria:**
- [ ] User registration creates user in database
- [ ] User can link to VA
- [ ] VA initialization creates VA record
- [ ] Admin gets admin role automatically
- [ ] VA config persists and caches
- [ ] Flight modes configuration works
- [ ] All API responses use standard format
- [ ] Error messages are clear and actionable

---

### 5. Test PIREP Submission

**Status:** 🔴 Not Started
**Priority:** High
**Estimated Time:** 3-4 hours

**Test Scenarios:**

#### 5.1. Get PIREP Configuration
1. **User requests PIREP config**
   - Call `GET /api/v1/pireps/config`
   - Headers: `X-API-Key`, `X-Server-Id`, `X-Discord-Id`
   - Expected: Flight modes for user's current flight

2. **Verify field visibility**
   - Check `show_in_discord` flags on fields
   - Ensure only visible fields returned

#### 5.2. Submit PIREP
1. **User submits PIREP**
   - Call `POST /api/v1/pireps/submit`
   - Body:
   ```json
   {
     "mode": "classic",
     "route_id": "KJFK-KLAX",
     "flight_time": "02:30",
     "pilot_remarks": "Smooth flight",
     "fuel_kg": "15000",
     "cargo_kg": "8000",
     "passengers": "180"
   }
   ```
   - Expected: PIREP queued for processing

2. **Verify PIREP processing**
   - Check Redis queue has entry
   - Wait for worker to process (< 1 minute)
   - Verify PIREP sent to Airtable
   - Check backfill worker enriches data

#### 5.3. Route Validation
1. **Test valid route**
   - Submit PIREP with route in system
   - Expected: Success response

2. **Test invalid route**
   - Submit PIREP with route NOT in system (e.g., "ZSPD-UAAA")
   - Expected: Clear error message "Route not found in system: ZSPD-UAAA"
   - **MUST FIX:** Currently shows "Unknown error occurred"

**Acceptance Criteria:**
- [ ] PIREP config endpoint returns correct modes
- [ ] Field visibility works correctly
- [ ] PIREP submission queues to Redis
- [ ] Queue workers process PIREPs
- [ ] PIREPs sent to Airtable successfully
- [ ] Route validation returns clear errors
- [ ] Backfill worker enriches PIREP data
- [ ] Error messages properly formatted for Discord

---

### 6. Flight Mode Management System

**Status:** 🔴 Not Started
**Priority:** Medium
**Estimated Time:** 6-8 hours

**Requirement:** Complete flight mode management system with UI for creating, editing, and managing flight modes

#### Business Rules
1. **EVENT mode** is reserved for special handling and is out of scope for v1.0 (future feature)
2. Maximum **4 active flight modes** allowed at any time
3. Users can enable/disable modes (already implemented in UI)
4. Each mode requires both a display label (shown to pilots) and Airtable column name mapping

#### Implementation Tasks

##### 6.1. Create "Add New Flight Mode" UI Flow
**Location:** `vizburo/ui/templates/partials/pirep-mode-create-wizard.html`

**Design:** Multi-step wizard with flowchart-style questions
- Step 1: Basic Information
  - Mode ID (e.g., "cargo", "charter", "training")
  - Display name (shown to pilots)
  - Description
  - Icon selection (optional)

- Step 2: Route Configuration
  - Question: "Does this mode require route selection?"
    - Yes → Show route selection options
    - No → Skip route fields
  - Question: "Should this mode auto-fill a specific route?"
    - Yes → Route selection dropdown
    - No → Continue

- Step 3: Custom Fields Configuration
  - Question: "Does this mode need custom fields?"
    - Yes → Show field builder
    - No → Use standard fields only

  **Field Builder Interface:**
  - Add/remove fields dynamically
  - For each field:
    - Display Label (shown to user in Discord/UI)
    - Airtable Column Name (backend mapping)
    - Field Type (text, textarea, number, date)
    - Required? (yes/no toggle)
    - Show in Discord modal? (yes/no toggle)

- Step 4: Validation Rules
  - Question: "Should this mode validate against specific routes?"
    - Yes → Route validation settings
    - No → Allow any route
  - Validation mode selection (exact_match, any)

- Step 5: Review & Create
  - Summary of all configuration
  - "Create Mode" button
  - "Back to Edit" button

**UI Components:**
- Progress indicator (1 of 5, 2 of 5, etc.)
- Next/Previous navigation buttons
- Cancel button (returns to modes list)
- Field validation on each step before advancing

##### 6.2. Create Flight Mode Handler
**Location:** `vizburo/ui/pirep_config_handlers.go`

**New Handler:** `CreatePirepModeHandler()`
```go
func CreatePirepModeHandler(
    w http.ResponseWriter,
    r *http.Request,
    vaGormRepo *repositories.VAGormRepository,
)
```

**Functionality:**
- Parse multi-step wizard form data
- Validate mode ID uniqueness
- Check active mode limit (max 4 active)
- Construct complete flight mode configuration
- Save to VA flight_modes_config JSONB
- Return success/error response

##### 6.3. Active Mode Limit Enforcement
**Location:** `internal/services/flight_modes_config_service.go`

**New Method:** `ValidateActiveModeLimit()`
- Count currently active modes
- Return error if attempting to activate 5th mode
- Update `ValidateAndSaveConfig()` to check limit

##### 6.4. Field Mapping Management
**Data Structure Enhancement:**
```json
{
  "fields": [
    {
      "name": "cargo_weight",
      "type": "number",
      "label": "Cargo Weight (kg)",
      "airtable_column": "Cargo Weight",
      "required": true,
      "show_in_discord": true
    }
  ]
}
```

**New Field Property:** `airtable_column` - Maps UI field to Airtable column name

##### 6.5. Update Existing Edit UI
**Location:** `vizburo/ui/templates/partials/pirep-mode-edit-form.html`

**Enhancements:**
- Add Airtable column name editing for fields
- Show field mapping table
- Warning if Airtable column names conflict

##### 6.6. Mode Activation/Deactivation Logic
**Enhancement to existing toggle:**
- Check active mode count before enabling
- Show error message if limit reached
- Suggest which mode to disable first

#### API Endpoints

**New Endpoint:** `GET /dashboard/settings/pirep/mode/create`
- Returns: Create wizard initial form (step 1)
- Handler: `GetPirepModeCreateHandler()`

**New Endpoint:** `POST /dashboard/settings/pirep/mode/create`
- Accepts: Complete mode configuration from wizard
- Handler: `CreatePirepModeHandler()`
- Validates: Mode ID uniqueness, active limit, field structure
- Returns: Success + updated modes list, or error with details

**Enhanced Endpoint:** `POST /dashboard/settings/pirep/mode/{mode_id}/toggle`
- Add validation: Check if activation would exceed 4 active modes
- Return error with actionable message if limit exceeded

#### Validation Rules

1. **Mode ID Validation:**
   - Must be unique within VA
   - Lowercase alphanumeric and underscores only
   - Cannot be "event" (reserved)
   - Max length: 50 characters

2. **Active Mode Limit:**
   - Maximum 4 modes can have `enabled: true`
   - Enforce on toggle and creation
   - Clear error message: "Maximum 4 active flight modes allowed. Please disable another mode first."

3. **Field Validation:**
   - Display label required (non-empty)
   - Airtable column name required (non-empty)
   - Field type must be valid (text, textarea, number, date)
   - At least one field required per mode

4. **Airtable Column Mapping:**
   - No duplicate column names within same mode
   - Warning (not error) if column name doesn't follow Airtable naming conventions

#### User Experience Flow

**Admin creates new flight mode:**
1. Navigate to `/dashboard/settings/pirep`
2. Click "Add New Flight Mode" button (new button in UI)
3. Wizard Step 1: Enter basic info → Click "Next"
4. Wizard Step 2: Configure route settings → Click "Next"
5. Wizard Step 3: Add custom fields with Airtable mappings → Click "Next"
6. Wizard Step 4: Set validation rules → Click "Next"
7. Wizard Step 5: Review configuration → Click "Create Mode"
8. See success message + new mode appears in list (disabled by default)
9. Admin can toggle mode to "enabled" (if under 4 active limit)

**Admin activates 5th mode (error case):**
1. Admin tries to enable a disabled mode
2. System checks: 4 modes already active
3. Error message displayed: "Maximum 4 active flight modes. Disable one of: [Classic, Cargo, Charter, Training]"
4. Admin disables one existing mode
5. Admin enables desired mode → Success

#### Database Schema (No changes needed)
- Uses existing `virtual_airlines.flight_modes_config` JSONB field
- New `airtable_column` property added to field objects
- Mode structure already supports all required fields

#### Acceptance Criteria

**Create Mode Flow:**
- [ ] "Add New Flight Mode" button appears on PIREP config page
- [ ] Wizard displays all 5 steps with progress indicator
- [ ] Each step validates inputs before allowing "Next"
- [ ] Can navigate back to previous steps without losing data
- [ ] Field builder allows adding/removing custom fields
- [ ] Each field captures both display label and Airtable column name
- [ ] Review step shows complete configuration summary
- [ ] "Create Mode" button creates mode and returns to list
- [ ] New mode appears in modes list (disabled by default)

**Active Mode Limit:**
- [ ] System prevents activating 5th mode
- [ ] Clear error message shown with actionable instructions
- [ ] Can activate 4 modes without issues
- [ ] Toggling modes respects limit at all times

**Field Mapping:**
- [ ] Can specify Airtable column name for each field
- [ ] Airtable mappings stored in configuration
- [ ] Edit form shows Airtable column names
- [ ] Can update Airtable mappings after creation

**Validation:**
- [ ] Mode ID must be unique
- [ ] Cannot create mode with ID "event"
- [ ] All required fields validated before creation
- [ ] Airtable column names validated (non-empty)

**Integration:**
- [ ] New mode appears in `GET /api/v1/pireps/config` response
- [ ] PIREP submission works with new mode
- [ ] Discord bot receives correct field configuration
- [ ] Airtable sync uses correct column mappings

#### Technical Notes

**Wizard State Management:**
- Use HTMX with hidden form fields to preserve state between steps
- Store wizard progress in session or browser localStorage
- Validate each step server-side before allowing progression

**Error Handling:**
- Field-level validation errors displayed inline
- Step-level errors prevent progression
- Final validation before creation with rollback on failure

**Future Enhancements (post v1.0):**
- Field templates (common field sets)
- Clone existing mode as starting point
- Bulk import modes via JSON
- Mode usage analytics
- EVENT mode implementation with special handling

---

### 7. Metrics and Grafana Dashboard

**Status:** 🔴 Not Started
**Priority:** High
**Estimated Time:** 6-8 hours

**Reference:** `MONITORING.md`

**Implementation Plan:**

#### 7.1. Add Dependencies
```bash
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
go get go.uber.org/zap
```

#### 7.2. Create Metrics Infrastructure
**File:** `internal/metrics/metrics.go`
- Define Prometheus registry
- Define metric collectors:
  - `politburo_http_requests_total` (Counter)
  - `politburo_http_request_duration_seconds` (Histogram)
  - `politburo_db_queries_total` (Counter)
  - `politburo_db_query_duration_seconds` (Histogram)
  - `politburo_cache_hits_total` (Counter)
  - `politburo_cache_misses_total` (Counter)
  - `politburo_flights_processed_total` (Counter)
  - `politburo_pirep_queue_depth` (Gauge)
  - `politburo_sync_job_duration_seconds` (Histogram)

**File:** `internal/metrics/middleware.go`
- HTTP metrics middleware
- Wraps response writer to capture status code
- Records request duration and status

#### 7.3. Update Main Entry Point
**File:** `cmd/server/main.go`
- Initialize metrics registry
- Initialize structured logger (Zap)
- Register `/metrics` endpoint
- Pass metrics to middleware chain

#### 7.4. Update Middleware
**File:** `internal/middleware/logging.go`
- Convert to structured logging (Zap)
- Add request ID generation
- Log structured fields (method, path, status, duration, user_id, server_id)

**File:** `internal/middleware/auth.go`
- Add request ID to context
- Reduce verbose logging (move to DEBUG level)

#### 7.5. Instrument Services
- `internal/common/cache_service.go` - Add cache hit/miss metrics
- `internal/common/redis_cache_service.go` - Add cache metrics
- `internal/workers/pirep_queue_worker.go` - Add queue depth gauge
- `internal/jobs/pilot_sync_job.go` - Add sync duration histogram
- `internal/jobs/route_sync_job.go` - Add sync duration histogram

#### 7.6. Grafana Dashboard Setup
**File:** `monitoring/grafana/dashboards/politburo-overview.json`
- System metrics (CPU, memory, uptime)
- HTTP metrics (requests/sec, latency percentiles, error rate)
- Database metrics (query count, query latency, slow queries)
- Cache metrics (hit rate, hit/miss ratio)
- Business metrics (flights processed, PIREPs submitted)
- Queue health (depth, processing rate)

**Docker Compose Updates:**
- Add Prometheus service
- Add Grafana service
- Add Loki service (log aggregation)
- Configure scrape targets
- Set retention policies

**Acceptance Criteria:**
- [ ] `/metrics` endpoint returns Prometheus format
- [ ] HTTP metrics collected for all endpoints
- [ ] Database query metrics recorded
- [ ] Cache hit/miss tracked
- [ ] Queue depth monitored
- [ ] Structured logging with Zap
- [ ] Request IDs in logs
- [ ] Grafana dashboard imports successfully
- [ ] All panels display data
- [ ] Prometheus scrapes successfully
- [ ] Logs ingested to Loki

---

### 8. Modernized Docker and Infrastructure Setup Cleanup

**Status:** 🔴 Not Started
**Priority:** High
**Estimated Time:** 4-6 hours

**Current Issues:**
- Multiple docker-compose files with inconsistent configuration
- Outdated Dockerfile patterns
- No health checks
- Inefficient build process
- Missing monitoring services

**Implementation:**

#### 8.1. Dockerfile Modernization
**File:** `Dockerfile` (new multi-stage production build)
```dockerfile
# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Stage 2: Runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
COPY --from=builder /app/internal/db/migrations ./internal/db/migrations
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthCheck || exit 1
CMD ["./server"]
```

**File:** `Dockerfile.dev` (development with hot reload)
```dockerfile
FROM golang:1.24-alpine
WORKDIR /app
RUN go install github.com/air-verse/air@latest
COPY go.mod go.sum ./
RUN go mod download
COPY . .
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]
```

#### 8.2. Docker Compose Consolidation
**File:** `docker-compose.yml` (production)
- PostgreSQL with proper volumes
- Redis with persistence
- Politburo API
- Prometheus
- Grafana
- Loki
- Health checks for all services
- Restart policies
- Resource limits

**File:** `docker-compose.dev.yml` (development)
- Hot reload for Go (Air)
- Volume mounts for source code
- Exposed ports for debugging
- Vizburo CSS watch service
- pgAdmin for database management
- No resource limits (for development)

#### 8.3. Infrastructure Files
**File:** `.air.toml` (Air configuration for hot reload)
- Watch patterns for Go files
- Exclude vendor, tmp directories
- Build commands
- Restart delay

**File:** `.dockerignore`
```
.git
.idea
*.md
node_modules
.air_tmp
logs/
*.log
.env*
!.env.example
vizburo_backup*
```

**File:** `.env.example`
```bash
# Application
APP_ENV=production
DEBUG=false
PORT=8080

# PostgreSQL
PG_HOST=db
PG_PORT=5432
PG_USER=ieuser
PG_DB=infinite
PG_PASSWORD=change_me_in_production

# Redis
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=

# Monitoring
PROMETHEUS_RETENTION=30d
LOKI_RETENTION=14d
GRAFANA_ADMIN_PASSWORD=change_me_in_production

# External APIs
INFINITE_FLIGHT_API_KEY=
AIRTABLE_API_KEY=
```

#### 8.4. Documentation Updates
**File:** `docs/DEPLOYMENT.md` (new)
- Production deployment guide
- Environment variable configuration
- Docker compose commands
- Health check verification
- Backup procedures
- Rollback procedures

**File:** `docs/DEVELOPMENT.md` (new)
- Local development setup
- Hot reload usage
- Database migrations
- Testing procedures
- Debugging tips

**Acceptance Criteria:**
- [ ] Multi-stage Dockerfile builds successfully
- [ ] Production image < 50MB
- [ ] Development hot reload works
- [ ] Health checks pass for all services
- [ ] docker-compose.yml starts production stack
- [ ] docker-compose.dev.yml starts development stack
- [ ] All services communicate correctly
- [ ] Volumes persist data correctly
- [ ] Resource limits prevent OOM
- [ ] Documentation accurate and complete

---

### 9. Refactor Workers

**Status:** 🔴 Not Started
**Priority:** Medium
**Estimated Time:** 4-6 hours

**Current Issues:**
- LogbookWorker commented out/deprecated
- No graceful shutdown
- Global channel variables
- Silent queue overflow
- No dead letter queue for failures

**Implementation:**

#### 9.1. Worker Infrastructure
**File:** `internal/workers/manager.go` (new)
```go
type WorkerManager struct {
    ctx        context.Context
    cancel     context.CancelFunc
    wg         *sync.WaitGroup
    workers    []Worker
}

type Worker interface {
    Start(ctx context.Context) error
    Stop() error
    Name() string
}
```

#### 9.2. Refactor LogbookWorker
**Decision:** Re-enable or remove completely?
- If keeping: Refactor to use context, proper queue, error handling
- If removing: Clean up references, update caching strategy

**Recommendation:** Remove and use cache-on-demand approach

#### 9.3. Refactor PirepQueueWorker
**File:** `internal/workers/pirep_queue_worker.go`
- Accept context from manager
- Implement graceful shutdown
- Add dead letter queue for failures
- Emit metrics (processing rate, error rate)
- Log structured errors with context

#### 9.4. Refactor MetaCacheWorker
**File:** `internal/workers/meta_cache_worker.go`
- Use context for cancellation
- Add metrics (sync duration, items synced)
- Structured logging
- Configurable interval

#### 9.5. Update Worker Initialization
**File:** `internal/workers/init.go`
- Use WorkerManager
- Pass cancellable context
- Wait for workers on shutdown
- Log worker lifecycle events

**File:** `cmd/server/main.go`
- Setup signal handling (SIGTERM, SIGINT)
- Create worker manager with context
- Start all workers
- Handle graceful shutdown:
  ```go
  ctx, cancel := context.WithCancel(context.Background())
  workerMgr := workers.NewManager(ctx, deps)
  workerMgr.StartAll()

  // Wait for signals
  sigChan := make(chan os.Signal, 1)
  signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
  <-sigChan

  // Graceful shutdown
  cancel()
  workerMgr.StopAll(15 * time.Second)
  ```

**Acceptance Criteria:**
- [ ] All workers accept context
- [ ] Graceful shutdown implemented
- [ ] Workers stop cleanly on SIGTERM
- [ ] Dead letter queue for failed PIREPs
- [ ] Metrics emitted by all workers
- [ ] Structured logging throughout
- [ ] No global channel variables
- [ ] Queue overflow logged and metered
- [ ] WorkerManager coordinates lifecycle

---

### 10. Comprehensive Help Command

**Status:** 🔴 Not Started
**Priority:** Medium
**Estimated Time:** 2-3 hours

**Scope:** Create API endpoint that returns comprehensive help documentation for Discord bot

**Implementation:**

#### 10.1. Create Help Handler
**File:** `internal/api/help_handlers.go` (new)
```go
type HelpHandlers struct {
    vaRepo *repositories.VAGormRepository
}

func (h *HelpHandlers) GetCommands() http.HandlerFunc {
    // Returns list of all commands with descriptions
}

func (h *HelpHandlers) GetCommandDetail() http.HandlerFunc {
    // Returns detailed help for specific command
}

func (h *HelpHandlers) GetQuickStart() http.HandlerFunc {
    // Returns quick start guide for new users
}

func (h *HelpHandlers) GetVAFeatures() http.HandlerFunc {
    // Returns features enabled for specific VA
}
```

#### 10.2. Help Content Structure
**File:** `internal/models/dtos/help.go` (new)
```go
type CommandHelp struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Usage       string   `json:"usage"`
    Examples    []string `json:"examples"`
    Aliases     []string `json:"aliases"`
    Category    string   `json:"category"` // registration, pirep, admin, etc.
    Permissions string   `json:"permissions"` // pilot, staff, admin
}

type HelpResponse struct {
    Commands   []CommandHelp `json:"commands"`
    Categories []string      `json:"categories"`
}
```

#### 10.3. API Endpoints
- `GET /api/v1/help/commands` - All commands
- `GET /api/v1/help/commands/{name}` - Specific command
- `GET /api/v1/help/quickstart` - Quick start guide
- `GET /api/v1/help/va/{va_id}/features` - VA-specific features

#### 10.4. Help Content
Document all commands:
- **/register** - Register as new pilot
- **/log** - Submit PIREP
- **/stats** - View pilot statistics
- **/live** - View live flights
- **/world-tour** - World tour commands
- **/dashboard** - Generate dashboard link
- **/help** - Show help
- Admin commands (role management, configuration)

**Acceptance Criteria:**
- [ ] Help endpoint returns all commands
- [ ] Command details include examples
- [ ] Commands categorized properly
- [ ] Permissions indicated for each command
- [ ] Quick start guide complete
- [ ] VA-specific features endpoint works
- [ ] Discord bot can consume API format
- [ ] Help content accurate and complete

---

## Testing Checklist

### Unit Tests
- [ ] All handler tests pass
- [ ] Repository tests pass with GORM
- [ ] Service tests pass
- [ ] Worker tests pass
- [ ] Middleware tests pass

### Integration Tests
- [ ] User registration flow (end-to-end)
- [ ] VA setup flow (end-to-end)
- [ ] PIREP submission flow (end-to-end)
- [ ] Live flights endpoint (with Live API)
- [ ] Worker processing (with Redis)
- [ ] Metrics collection (verify counts)

### API Tests
- [ ] All endpoints return correct status codes
- [ ] Error responses properly formatted
- [ ] Authentication works correctly
- [ ] Authorization enforced by role
- [ ] Response times acceptable (<2s)

### Load Tests (optional, recommended)
- [ ] 100 concurrent users
- [ ] 1000 requests/minute
- [ ] Worker queue handles backlog
- [ ] No memory leaks
- [ ] Cache hit rates acceptable

---

## Deployment Checklist

### Pre-Deployment
- [ ] All tests passing
- [ ] Code reviewed
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Version tagged (v1.0.0)
- [ ] Database migrations tested
- [ ] Backup procedures documented
- [ ] Rollback plan documented

### Deployment Steps
1. [ ] Merge world_tour → main
2. [ ] Tag release: `git tag v1.0.0`
3. [ ] Build Docker images
4. [ ] Run database migrations
5. [ ] Deploy to production
6. [ ] Verify health checks
7. [ ] Check metrics in Grafana
8. [ ] Test critical flows
9. [ ] Monitor logs for errors

### Post-Deployment
- [ ] Monitor error rates (< 1%)
- [ ] Monitor response times (p95 < 1s)
- [ ] Monitor resource usage (< 80%)
- [ ] Verify queue processing
- [ ] Verify cache hit rates
- [ ] Announce release
- [ ] Update documentation links

---

## Risk Management

### High Risk Items
1. **sqlx to GORM migration** - Breaking change, thorough testing required
2. **Worker refactoring** - Affects background processing, needs careful testing
3. **Metrics implementation** - Performance impact, need to verify overhead

### Mitigation Strategies
- Feature flags for new functionality
- Canary deployment (10% → 50% → 100%)
- Quick rollback plan ready
- Database backups before migration
- Staging environment testing

### Rollback Triggers
- Error rate > 5%
- Response time p95 > 5 seconds
- Memory usage > 90%
- Queue backlog > 1000 items
- Critical feature broken

---

## Success Criteria

### Technical Metrics
- [ ] All tests passing (100%)
- [ ] Code coverage > 60%
- [ ] No critical/high security vulnerabilities
- [ ] Docker image size < 50MB
- [ ] Application memory < 200MB
- [ ] Response time p95 < 1 second
- [ ] Error rate < 1%

### Feature Completeness
- [ ] All 10 goals completed
- [ ] Documentation complete
- [ ] API stable and consistent
- [ ] Monitoring functional
- [ ] Infrastructure clean

### User Experience
- [ ] Registration smooth and intuitive
- [ ] PIREP submission works reliably
- [ ] Error messages clear and actionable
- [ ] Help documentation comprehensive
- [ ] Dashboard functional

---

## Timeline

### Day 1 (Jan 27) - Foundation
- [ ] Remove sqlx dependencies (6-8 hours)
- [ ] Set up task tracking
- [ ] Begin handler refactoring

### Day 2 (Jan 28) - API Completion
- [ ] Finish handler refactoring (6-8 hours)
- [ ] Test registration flows (3 hours)
- [ ] Test PIREP submission (3 hours)

### Day 3 (Jan 29) - Features & Infrastructure
- [ ] Make live endpoint functional (4-6 hours)
- [ ] Flight Mode Management System - Phase 1 (6-8 hours)
  - [ ] Create flight mode wizard UI (4 hours)
  - [ ] Implement create handler and validation (2 hours)
  - [ ] Active mode limit enforcement (1 hour)
  - [ ] Testing and refinement (1 hour)

### Day 4 (Jan 30) - Monitoring & Workers
- [ ] Complete metrics implementation (4 hours)
- [ ] Create Grafana dashboard (2 hours)
- [ ] Refactor workers (4-6 hours)

### Day 5 (Jan 31) - Docker & Testing
- [ ] Modernize Docker setup (4-6 hours)
- [ ] Comprehensive help command (2-3 hours)
- [ ] Integration testing (4 hours)

### Day 6 (Feb 1) - Testing & Documentation
- [ ] Complete all testing
- [ ] Fix any issues found
- [ ] Update documentation
- [ ] Prepare release notes

### Day 7 (Feb 2) - Release Day
- [ ] Final testing
- [ ] Tag release
- [ ] Deploy to production
- [ ] Monitor and verify

---

## Notes

- **Current branch:** world_tour (needs merge to main after completion)
- **Breaking changes:** sqlx removal, handler refactoring
- **Database migrations:** None expected, schema stable
- **API versioning:** v1 stable after this release
- **Documentation:** All docs updated as part of release

---

## Questions to Resolve

1. **LogbookWorker:** Re-enable with refactor or remove completely?
2. **Live Flights:** What regex patterns for VA filtering?
3. **Help Command:** What format does Discord bot expect?
4. **Deployment target:** Where is production environment?

### Resolved Questions
1. ✅ **PIREP Flight Mode** (Resolved Jan 29, 2026)
   - Admin can create custom flight modes via wizard UI
   - Maximum 4 active modes at a time
   - Each field requires display label + Airtable column mapping
   - EVENT mode reserved for future scope

---

**Release Manager:** Claude Code
**Document Version:** 1.0
**Last Updated:** January 28, 2026
