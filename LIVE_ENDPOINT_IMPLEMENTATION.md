# Live Flights Endpoint Implementation Plan

## Overview

This document outlines the plan to enhance the existing `/api/v1/va/live` endpoint with:
- Database-backed aircraft/livery metadata storage
- Optimized caching strategy (1-minute TTL for live flights)
- 4x daily sync of aircraft/liveries with change detection
- Proper parallelization for route enrichment (already implemented)
- Rounded speed values

## Current State

### Existing Endpoint
- **Route**: `GET /api/v1/va/live` (politburo/internal/routes/router.go:97)
- **Handler**: `VaFlightsHandler` (politburo/internal/api/flights.go:107)
- **Service Method**: `FlightsService.GetVALiveFlights()` (politburo/internal/services/flights_service.go:477)
- **Auth Required**: Member role (requires registered user)
- **Returns**: Array of `LiveFlight` DTOs

### Current Implementation Flow
1. Fetch VA config (server ID, callsign prefix/suffix) via `VAConfigService`
2. Call `GetLiveFlights(sessionId)` → caches for 2 minutes (politburo/internal/services/flights_service.go:364)
3. Filter flights by callsign prefix/suffix using `FilterFlights()`
4. Enrich with route data via `enrichFlightData()` → parallelized with errgroup (8 workers)
5. Return filtered `LiveFlight` array

### Data Structures

#### LiveFlight DTO (politburo/internal/models/dtos/responses.go:241)
```go
type LiveFlight struct {
    Callsign       string `json:"callsign"`
    CallsignVar    string `json:"callsignVar"`
    CallsignPrefix string `json:"callsignPrefix"`
    CallsignSuffix string `json:"callsignSuffix"`

    SessionID  string `json:"sessionID"`
    FlightID   string `json:"flightID"`
    AircraftId string `json:"aircraftID"`
    LiveryId   string `json:"liveryID"`
    Username   string `json:"username"`
    UserID     string `json:"userID"`

    Aircraft string `json:"aircraft"`      // Aircraft name from livery lookup
    Livery   string `json:"livery"`        // Livery name from livery lookup

    AltitudeFt  int    `json:"altitude"`   // Rounded to nearest 100
    SpeedKts    int    `json:"speed"`      // Currently floor(), needs rounding
    Origin      string `json:"origin"`     // From flight plan waypoints
    Destination string `json:"destination"` // From flight plan waypoints

    ReportTime  time.Time `json:"lastReport"`
    IsConnected bool      `json:"isConnected"`
}
```

#### FlightEntry (from Infinite Flight API)
```go
type FlightEntry struct {
    Username            string  `json:"username"`
    Callsign            string  `json:"callsign"`
    Latitude            float64 `json:"latitude"`
    Longitude           float64 `json:"longitude"`
    Altitude            float64 `json:"altitude"`
    Speed               float64 `json:"speed"`
    VerticalSpeed       float64 `json:"verticalSpeed"`
    Track               float64 `json:"track"`
    LastReport          string  `json:"lastReport"`
    FlightID            string  `json:"flightId"`
    UserID              string  `json:"userId"`
    AircraftID          string  `json:"aircraftId"`
    LiveryID            string  `json:"liveryId"`
    VirtualOrganization string  `json:"virtualOrganization"`
    PilotState          int     `json:"pilotState"`
    IsConnected         bool    `json:"isConnected"`
}
```

#### AircraftLivery (from Infinite Flight API)
```go
type AircraftLivery struct {
    LiveryId     string `json:"id"`
    AircraftID   string `json:"aircraftID"`
    LiveryName   string `json:"liveryName"`
    AircraftName string `json:"aircraftName"`
}
```

### Infinite Flight API Endpoints Used

1. **Get Session Flights**
   - URL: `https://api.infiniteflight.com/public/v2/sessions/{sessionId}/flights`
   - Method: GET
   - Auth: Bearer token
   - Returns: `FlightsResponse` with array of `FlightEntry`

2. **Get Aircraft Liveries** (currently cached only)
   - URL: `https://api.infiniteflight.com/public/v2/aircraft/liveries`
   - Method: GET
   - Auth: Bearer token
   - Returns: `AircraftLiveriesResponse` with array of `AircraftLivery`

3. **Get Flight Plan** (for route enrichment)
   - URL: `https://api.infiniteflight.com/public/v2/sessions/{sessionId}/flights/{flightId}/flightplan`
   - Method: GET
   - Auth: Bearer token
   - Cached: 5 minutes
   - Returns: `FlightPlanResponse` with waypoints array

### VA Configuration Keys Used
- `if_server_id`: Infinite Flight server/session ID
- `callsign_prefix`: Filter prefix (e.g., "AAL" for American Airlines)
- `callsign_suffix`: Filter suffix (optional)

## Implementation Plan

### Phase 1: Database Schema for Aircraft/Liveries Persistence
**Goal**: Store aircraft and livery data in PostgreSQL instead of relying solely on cache

**Files to Create**:
- `politburo/internal/db/migrations/008_aircraft_liveries_metadata.sql`

**Schema**:
```sql
-- Create aircraft_liveries table
CREATE TABLE aircraft_liveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    livery_id VARCHAR(100) UNIQUE NOT NULL,
    aircraft_id VARCHAR(100) NOT NULL,
    aircraft_name TEXT NOT NULL,
    livery_name TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_synced_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_aircraft_liveries_livery_id ON aircraft_liveries(livery_id);
CREATE INDEX idx_aircraft_liveries_aircraft_id ON aircraft_liveries(aircraft_id);
CREATE INDEX idx_aircraft_liveries_active ON aircraft_liveries(is_active);

-- Composite index for common queries
CREATE INDEX idx_aircraft_liveries_active_livery ON aircraft_liveries(is_active, livery_id);

-- Index for sync operations
CREATE INDEX idx_aircraft_liveries_sync ON aircraft_liveries(last_synced_at);
```

### Phase 2: Entity Models
**Goal**: Create GORM entity for aircraft_liveries table

**Files to Create**:
- `politburo/internal/models/entities/aircraft_livery.go`

**Entity**:
```go
package entities

import (
    "time"
)

type AircraftLivery struct {
    ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" db:"id"`
    LiveryID      string    `gorm:"type:varchar(100);uniqueIndex;not null" db:"livery_id"`
    AircraftID    string    `gorm:"type:varchar(100);index;not null" db:"aircraft_id"`
    AircraftName  string    `gorm:"type:text;not null" db:"aircraft_name"`
    LiveryName    string    `gorm:"type:text;not null" db:"livery_name"`
    IsActive      bool      `gorm:"default:true" db:"is_active"`
    CreatedAt     time.Time `gorm:"default:now()" db:"created_at"`
    UpdatedAt     time.Time `gorm:"default:now()" db:"updated_at"`
    LastSyncedAt  time.Time `gorm:"default:now()" db:"last_synced_at"`
}

func (AircraftLivery) TableName() string {
    return "aircraft_liveries"
}
```

### Phase 3: Repository Layer
**Goal**: Create data access layer for aircraft/liveries

**Files to Create**:
- `politburo/internal/db/repositories/aircraft_livery_repository.go`

**Repository Interface**:
```go
package repositories

import (
    "context"
    "infinite-experiment/politburo/internal/models/entities"
    "time"
)

type AircraftLiveryRepository struct {
    // Use GORM DB instance
}

// GetByLiveryID fetches a single livery by ID
func (r *AircraftLiveryRepository) GetByLiveryID(ctx context.Context, liveryID string) (*entities.AircraftLivery, error)

// GetAllActive fetches all active liveries
func (r *AircraftLiveryRepository) GetAllActive(ctx context.Context) ([]entities.AircraftLivery, error)

// UpsertBatch performs bulk upsert with conflict resolution
func (r *AircraftLiveryRepository) UpsertBatch(ctx context.Context, liveries []entities.AircraftLivery) error

// GetLastSyncTime returns the most recent sync timestamp
func (r *AircraftLiveryRepository) GetLastSyncTime(ctx context.Context) (time.Time, error)

// MarkInactive marks liveries as inactive by their IDs
func (r *AircraftLiveryRepository) MarkInactive(ctx context.Context, liveryIDs []string) error

// GetLiveryMap returns a map of liveryID -> AircraftLivery for fast lookups
func (r *AircraftLiveryRepository) GetLiveryMap(ctx context.Context) (map[string]entities.AircraftLivery, error)
```

### Phase 4: Aircraft Livery Service
**Goal**: Create service layer for aircraft/livery operations with caching

**Files to Create**:
- `politburo/internal/common/aircraft_livery_service.go`

**Service Interface**:
```go
package common

import (
    "context"
    "infinite-experiment/politburo/internal/db/repositories"
    "infinite-experiment/politburo/internal/models/dtos"
)

type AircraftLiveryService struct {
    cache *CacheService
    repo  *repositories.AircraftLiveryRepository
}

func NewAircraftLiveryService(cache *CacheService, repo *repositories.AircraftLiveryRepository) *AircraftLiveryService

// GetAircraftLivery fetches livery data (cache-first, then DB)
func (s *AircraftLiveryService) GetAircraftLivery(ctx context.Context, liveryID string) (*dtos.AircraftLivery, error)

// GetAircraftName returns just the aircraft name for a livery ID
func (s *AircraftLiveryService) GetAircraftName(ctx context.Context, liveryID string) string

// GetLiveryName returns just the livery name for a livery ID
func (s *AircraftLiveryService) GetLiveryName(ctx context.Context, liveryID string) string

// WarmCache loads all active liveries into cache
func (s *AircraftLiveryService) WarmCache(ctx context.Context) error
```

**Caching Strategy**:
- Cache key pattern: `LIVERY_{liveryID}`
- Cache TTL: 24 hours (liveries rarely change)
- Fallback: If cache miss, query DB
- Warming: Load all active liveries on startup and after sync

### Phase 5: Update Meta Cache Worker
**Goal**: Sync aircraft/liveries 4 times per day with change detection

**Files to Modify**:
- `politburo/internal/workers/meta_cache_worker.go`

**Changes**:
1. Change ticker from 30 minutes to 6 hours (4x daily)
2. Update `refillAirframeCacheTask` to:
   - Fetch liveries from Infinite Flight API
   - Load existing liveries from database
   - Detect changes (additions, removals, updates)
   - Only write to DB if changes detected
   - Warm cache after sync

**New Worker Signature**:
```go
func StartCacheFiller(
    c *CacheService,
    api *LiveAPIService,
    liveryRepo *repositories.AircraftLiveryRepository,
    liverySvc *AircraftLiveryService,
)
```

**Sync Logic Pseudocode**:
```
1. Fetch liveries from Infinite Flight API
2. Load existing liveries from DB into map (key: liveryID)
3. For each API livery:
   a. If exists in DB:
      - Compare fields (aircraft_name, livery_name)
      - If changed: Add to update batch
   b. If not exists in DB:
      - Add to insert batch
4. For each DB livery:
   a. If not in API response:
      - Mark as inactive
5. Execute batch operations:
   - Upsert new/updated liveries
   - Mark removed liveries as inactive
6. Warm cache with fresh data
7. Log sync stats (added, updated, removed counts)
```

### Phase 6: Update Flight Service
**Goal**: Integrate AircraftLiveryService and optimize caching

**Files to Modify**:
- `politburo/internal/services/flights_service.go`

**Changes**:

1. **Add AircraftLiveryService dependency** (line 19-23):
```go
type FlightsService struct {
    Cache         *common.CacheService
    ApiService    *common.LiveAPIService
    Cfg           *common.VAConfigService
    LiverySvc     *common.AircraftLiveryService  // NEW
}
```

2. **Update constructor** (line 63):
```go
func NewFlightsService(
    cache *common.CacheService,
    liveApi *common.LiveAPIService,
    cfgSvc *common.VAConfigService,
    liverySvc *common.AircraftLiveryService,  // NEW
) *FlightsService {
    return &FlightsService{
        Cache:      cache,
        ApiService: liveApi,
        Cfg:        cfgSvc,
        LiverySvc:  liverySvc,  // NEW
    }
}
```

3. **Update GetLiveFlights cache TTL** (line 364):
```go
// Change from 2*time.Minute to 1*time.Minute
val, err := svc.Cache.GetOrSet(cacheKey, 1*time.Minute, func() (any, error) {
```

4. **Update mapToLiveFlight** (line 285-339):
```go
// Replace line 310:
// OLD: eqpmnt := common.GetAircraftLivery(flt.LiveryID, svc.Cache)
// NEW:
acft, liv := "", ""
if liveryData := svc.LiverySvc.GetAircraftLivery(context.Background(), flt.LiveryID); liveryData != nil {
    acft = liveryData.AircraftName
    liv = liveryData.LiveryName
}

// Replace line 308 (round speed):
// OLD: spd := int(flt.Speed)
// NEW:
spd := int(math.Round(flt.Speed))
```

5. **Update GetUserFlights** (line 163):
```go
// Replace line 163:
// OLD: eqpmnt := common.GetAircraftLivery(rec.LiveryID, svc.Cache)
// NEW:
aircraftName, liveryName := "", ""
if liveryData := svc.LiverySvc.GetAircraftLivery(context.Background(), rec.LiveryID); liveryData != nil {
    aircraftName = liveryData.AircraftName
    liveryName = liveryData.LiveryName
}

// Remove lines 165-171 (now handled above)
```

### Phase 7: Dependency Injection Updates
**Goal**: Wire up new services and repositories

**Files to Modify**:
- `politburo/internal/api/dependencies.go` (or wherever DI is configured)
- `politburo/internal/routes/router.go`

**Changes in router.go** (around line 38-62):
```go
// Add to Dependencies struct:
type Dependencies struct {
    // ... existing fields
    AircraftLiveryRepo *repositories.AircraftLiveryRepository  // NEW
    AircraftLiverySvc  *common.AircraftLiveryService           // NEW
}

// Initialize in RegisterRoutes:
aircraftLiveryRepo := repositories.NewAircraftLiveryRepository(db.PgDB)
aircraftLiverySvc := common.NewAircraftLiveryService(legacyCacheSvc, aircraftLiveryRepo)

// Update FlightsService initialization:
flightSvc := services.NewFlightsService(legacyCacheSvc, liveSvc, cfgSvc, aircraftLiverySvc)

// Update StartCacheFiller:
go workers.StartCacheFiller(legacyCacheSvc, liveSvc, aircraftLiveryRepo, aircraftLiverySvc)
```

### Phase 8: Testing & Validation
**Goal**: Ensure all components work correctly

**Test Scenarios**:
1. **Cold start** (empty DB):
   - Worker should sync all liveries on first run
   - Cache should be warmed with all liveries
   - Flight queries should return aircraft/livery names

2. **Warm cache**:
   - Live flights should be cached for 1 minute
   - Routes should be cached for 5 minutes
   - Liveries should be cached for 24 hours

3. **DB fallback**:
   - When cache misses, should query DB
   - Should re-populate cache after DB query

4. **Change detection**:
   - Add new livery to API mock → Should INSERT
   - Modify livery in API mock → Should UPDATE
   - Remove livery from API mock → Should mark inactive

5. **Parallel route enrichment**:
   - Verify 8 concurrent workers are processing routes
   - Check logs for enrichment timing
   - Ensure no race conditions

6. **Speed rounding**:
   - Verify speed values are rounded (e.g., 245.7 → 246)

## Caching Strategy Summary

| Data Type | Cache Key Pattern | TTL | Fallback |
|-----------|------------------|-----|----------|
| Live flights | `LIVE_FLIGHTS_{sessionId}` | 1 minute | Live API |
| Flight plan | `FPL_{sessionId}_{flightId}` | 5 minutes | Live API |
| User by IFC ID | `LIVE_USER_{ifcId}` | 15 minutes | Live API |
| User flights | `LIVE_FLIGHTS_{userId}_{page}` | 15 minutes | Live API |
| Aircraft livery | `LIVERY_{liveryId}` | 24 hours | Database |
| Sessions | `SERVERS` | 5 minutes | Live API |
| VA config | `VA_CONFIG_{vaId}` | 10 minutes | Database |

## Migration Checklist

- [ ] Create migration file `008_aircraft_liveries_metadata.sql`
- [ ] Create entity model `entities/aircraft_livery.go`
- [ ] Create repository `repositories/aircraft_livery_repository.go`
- [ ] Create service `common/aircraft_livery_service.go`
- [ ] Update `workers/meta_cache_worker.go` (sync logic + 6-hour ticker)
- [ ] Update `services/flights_service.go` (use new service + round speed)
- [ ] Update dependency injection in `routes/router.go`
- [ ] Run migration manually: `psql -U ieuser -d infinite -f 008_aircraft_liveries_metadata.sql`
- [ ] Test cold start (empty DB)
- [ ] Test warm cache
- [ ] Test change detection
- [ ] Verify speed rounding
- [ ] Check parallel route enrichment logs
- [ ] Update Swagger docs if needed
- [ ] Remove deprecated `GetAircraftLivery` function

## Environment Variables Required

Already configured:
- `IF_API_BASE_URL`: Infinite Flight API base URL
- `IF_API_KEY`: Infinite Flight API key
- `PG_HOST`, `PG_PORT`, `PG_USER`, `PG_PASSWORD`, `PG_DB`: PostgreSQL connection

## API Response Example

**Request**: `GET /api/v1/va/live`

**Headers**:
```
X-API-Key: <your-api-key>
X-Discord-Server-Id: <discord-server-id>
X-Discord-User-Id: <discord-user-id>
```

**Response**:
```json
{
  "status": "success",
  "message": "Live flights fetched",
  "response_time": "245ms",
  "data": [
    {
      "callsign": "AAL123",
      "callsignVar": "123",
      "callsignPrefix": "AAL",
      "callsignSuffix": "",
      "sessionID": "7e5dcd44-1fb8-423b-aa68-891aebbba79d",
      "flightID": "f8d5c3a2-9b1e-4f6d-8c7a-3e2d1f0b9a8c",
      "aircraftID": "de510d3d-04f8-46e0-8d65-55b888f33129",
      "liveryID": "c875c0e9-19c2-420d-8fb4-32c151bd797c",
      "username": "PilotName",
      "userID": "3f8b28bf-bbb1-4024-80ae-2a0ea9b30685",
      "aircraft": "Boeing 737-800",
      "livery": "American Airlines",
      "altitude": 35000,
      "speed": 456,
      "origin": "KJFK",
      "destination": "KLAX",
      "lastReport": "2025-10-24T15:30:00Z",
      "isConnected": true
    }
  ]
}
```

## Questions for User

If any of the following are unclear during implementation:
1. **Migration execution**: Should migrations be auto-applied or manual?
2. **Error handling**: How should livery lookup failures be handled? (return empty string, log warning, return error?)
3. **Sync job failure**: If livery sync fails, should it retry immediately or wait for next cycle?
4. **Cache warming**: Should cache be warmed on every sync, or only on startup?
5. **Inactive liveries**: Should inactive liveries be returned in queries, or filtered out?

## Next Steps

1. Start with Phase 1 (database migration)
2. Build up through entity → repository → service layers
3. Update worker and flight service
4. Wire everything together with DI
5. Test thoroughly with different scenarios
6. Deploy and monitor

---

**Document Version**: 1.0
**Last Updated**: 2025-10-24
**Status**: Ready for implementation
