# Aircraft Platform Package

Platform-level infrastructure for Infinite Flight aircraft and livery metadata.

## Overview

This package provides core functionality for managing aircraft and livery data from the Infinite Flight API. It's platform-level infrastructure because it handles game-specific data rather than business logic.

## Components

### Model (`model.go`)
- **AircraftLivery**: GORM entity for aircraft/livery metadata from Infinite Flight API
- **LiveryAirtableMapping**: GORM entity for VA-specific livery field mappings to Airtable

### Repository (`repo.go`)
Unified repository handling all aircraft-related database operations:
- Aircraft livery CRUD operations
- Bulk upsert with conflict resolution
- Livery mapping operations for Airtable integration

### Service (`service.go`)
Cache-first lookup service with 24-hour TTL:
- `GetAircraftLivery(ctx, liveryID)` - Fetch livery metadata
- `GetAircraftName(ctx, liveryID)` - Get aircraft name
- `GetLiveryName(ctx, liveryID)` - Get livery name
- `WarmCache(ctx)` - Preload all active liveries into cache

### Worker (`worker.go`)
Background sync worker that runs every 6 hours:
- Fetches latest aircraft/livery data from Infinite Flight API
- Performs change detection to minimize database writes
- Marks removed liveries as inactive
- Warms cache when changes are detected

## Usage

```go
// Initialize repository and service
repo := aircraft.NewRepository(db)
svc := aircraft.NewService(cache, repo)

// Initialize and start worker
worker := aircraft.NewWorker(&cacheInterface, liveAPI, repo, svc)
go worker.Start()

// Lookup livery data
livery := svc.GetAircraftLivery(ctx, "livery-uuid-123")
if livery != nil {
    fmt.Println(livery.AircraftName) // "Boeing 737-800"
    fmt.Println(livery.LiveryName)   // "Southwest Airlines"
}
```

## Dependencies

- `infra/cache` - CacheService for 24-hour TTL caching
- `internal/common` - LiveAPIService for Infinite Flight API integration
- `gorm.io/gorm` - ORM for database operations

## Database Tables

- `aircraft_liveries` - Stores aircraft/livery metadata
- `livery_airtable_mappings` - Stores VA-specific field mappings for Airtable

## Sync Strategy

The worker syncs every 6 hours (4x daily) and performs:
1. Fetch all liveries from Infinite Flight API
2. Load existing liveries from database into map
3. Compare to detect additions, updates, and removals
4. Batch upsert changes to database
5. Mark removed liveries as inactive
6. Warm cache if any changes detected

This change detection strategy minimizes database writes and ensures cache consistency.
