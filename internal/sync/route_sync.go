package sync

import (
	"context"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/db/repositories"
	platformVA "infinite-experiment/politburo/internal/platform/va"
	"infinite-experiment/politburo/infra/providers"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// RouteSyncJob handles syncing route data from Airtable to local database
type RouteSyncJob struct {
	db               *gorm.DB
	cache            cache.CacheInterface
	vaSvc            *platformVA.Service
	syncRepo         *Repository // Consolidated sync repository
	airportRepo      *repositories.AirportRepository
	airtableProvider *providers.AirtableProvider
}

// NewRouteSyncJob creates a new route sync job instance
func NewRouteSyncJob(
	db *gorm.DB,
	cache cache.CacheInterface,
	vaSvc *platformVA.Service,
	syncRepo *Repository,
	airportRepo *repositories.AirportRepository,
) *RouteSyncJob {
	return &RouteSyncJob{
		db:               db,
		cache:            cache,
		vaSvc:            vaSvc,
		syncRepo:         syncRepo,
		airportRepo:      airportRepo,
		airtableProvider: providers.NewAirtableProvider(cache),
	}
}

// Run executes the route sync job for all active VAs with Airtable enabled
func (j *RouteSyncJob) Run(ctx context.Context) error {
	start := time.Now()
	log.Printf("[RouteSyncJob] Starting route sync at %s", start.Format(time.RFC3339))

	// Get all VAs that have active Airtable configs
	var vaIDs []string
	err := j.db.WithContext(ctx).
		Table("va_data_provider_configs").
		Where("provider_type = ? AND config_type = ? AND is_active = ?", "airtable", "credentials", true).
		Pluck("va_id", &vaIDs).Error

	if err != nil {
		log.Printf("[RouteSyncJob] Error fetching active VAs: %v", err)
		return fmt.Errorf("failed to fetch active VAs: %w", err)
	}

	if len(vaIDs) == 0 {
		log.Printf("[RouteSyncJob] No VAs with active Airtable configs found")
		return nil
	}

	log.Printf("[RouteSyncJob] Found %d VAs with active Airtable configs", len(vaIDs))

	// Sync routes for each VA
	totalSynced := 0
	for _, vaID := range vaIDs {
		synced, err := j.SyncVARoutes(ctx, vaID)
		if err != nil {
			log.Printf("[RouteSyncJob] Error syncing routes for VA %s: %v", vaID, err)
			// Continue with other VAs even if one fails
			continue
		}
		totalSynced += synced
	}

	log.Printf("[RouteSyncJob] Completed route sync in %s. Total routes synced: %d",
		time.Since(start).Truncate(time.Millisecond), totalSynced)

	return nil
}

// SyncVARoutes syncs routes for a specific VA (exported for manual triggering)
func (j *RouteSyncJob) SyncVARoutes(ctx context.Context, vaID string) (int, error) {
	if 1 == 1 {
		return 0, nil
	}

	start := time.Now()
	log.Printf("[RouteSyncJob] Syncing routes for VA %s", vaID)

	// Get Airtable credentials
	creds, err := j.vaSvc.GetAirtableCredentials(ctx, vaID)
	if err != nil {
		return 0, fmt.Errorf("failed to get Airtable credentials: %w", err)
	}

	if creds == nil {
		log.Printf("[RouteSyncJob] No active Airtable credentials found for VA %s", vaID)
		return 0, nil
	}

	// Get route schema
	routeSchemaConfig, err := j.vaSvc.GetAirtableSchema(ctx, vaID, "route")
	if err != nil {
		return 0, fmt.Errorf("failed to get route schema: %w", err)
	}

	if routeSchemaConfig == nil {
		log.Printf("[RouteSyncJob] No route schema configured for VA %s", vaID)
		return 0, nil
	}

	if !routeSchemaConfig.Enabled {
		log.Printf("[RouteSyncJob] Route schema is disabled for VA %s", vaID)
		return 0, nil
	}

	// Convert SchemaConfig to EntitySchema for provider compatibility
	routeSchema := &platformVA.EntitySchema{
		TableName:         routeSchemaConfig.TableName,
		Enabled:           routeSchemaConfig.Enabled,
		LastModifiedField: routeSchemaConfig.LastModifiedField,
		Fields:            routeSchemaConfig.Fields,
	}

	// Get VA name for logging
	var vaName string
	j.db.WithContext(ctx).
		Table("virtual_airlines").
		Where("id = ?", vaID).
		Pluck("name", &vaName)

	log.Printf("[RouteSyncJob] VA: %s (%s), Table: %s", vaName, vaID, routeSchema.TableName)

	// Get last sync timestamp for incremental sync
	lastModified, err := j.getLastSyncTimestamp(ctx, vaID)
	if err != nil {
		log.Printf("[RouteSyncJob] VA %s: Error getting last sync timestamp: %v. Doing full sync.", vaName, err)
		lastModified = nil
	}

	if lastModified != nil {
		log.Printf("[RouteSyncJob] VA %s: Incremental sync from %s", vaName, *lastModified)
	} else {
		log.Printf("[RouteSyncJob] VA %s: Full sync (no previous sync timestamp)", vaName)
	}

	// Check if schema has last_modified_field configured
	if lastModified != nil && routeSchema.LastModifiedField == "" {
		log.Printf("[RouteSyncJob] VA %s: Warning - no last_modified_field configured in schema, cannot filter by date. Doing full sync.", vaName)
		lastModified = nil
	}

	// Set credentials in context for provider
	ctx = context.WithValue(ctx, "provider_credentials", creds)

	// Fetch routes with pagination and optional modified-since filter
	var allRecords []providers.RecordWithID
	offset := ""
	pageCount := 0

	for {
		pageCount++
		filters := &providers.SyncFilters{
			Offset:        offset,
			Limit:         100, // Batch size
			ModifiedSince: lastModified,
		}

		recordSet, err := j.airtableProvider.FetchRecords(ctx, routeSchema, filters)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch records (page %d): %w", pageCount, err)
		}

		log.Printf("[RouteSyncJob] VA %s: Fetched page %d with %d records", vaName, pageCount, len(recordSet.Records))

		allRecords = append(allRecords, recordSet.Records...)

		if !recordSet.HasMore {
			break
		}
		offset = recordSet.Offset
	}

	log.Printf("[RouteSyncJob] VA %s: Total records fetched: %d", vaName, len(allRecords))

	// Process and upsert routes
	syncedCount := 0
	errorCount := 0

	for i, record := range allRecords {
		if err := j.upsertRoute(ctx, vaID, record.ID, record.Fields, routeSchema); err != nil {
			log.Printf("[RouteSyncJob] VA %s: Error upserting record %d: %v", vaName, i+1, err)
			errorCount++
			continue
		}
		syncedCount++
	}

	log.Printf("[RouteSyncJob] VA %s: Completed in %s. Synced: %d, Errors: %d",
		vaName, time.Since(start).Truncate(time.Millisecond), syncedCount, errorCount)

	// Record successful sync in sync history
	if err := j.syncRepo.RecordSync(ctx, vaID, SyncEventRoutesAT); err != nil {
		log.Printf("[RouteSyncJob] VA %s: Warning - failed to record sync history: %v", vaName, err)
		// Don't fail the sync operation if history recording fails
	}

	return syncedCount, nil
}

// upsertRoute updates or creates a route record in route_at_synced
func (j *RouteSyncJob) upsertRoute(ctx context.Context, vaID string, airtableRecordID string, record map[string]interface{}, schema *platformVA.EntitySchema) error {
	// Extract field mappings
	originField := schema.GetFieldMapping("origin")
	destField := schema.GetFieldMapping("destination")
	routeField := schema.GetFieldMapping("route")

	// Route is MANDATORY
	if routeField == nil {
		return fmt.Errorf("route field not configured in schema")
	}

	// Extract route (required)
	rawRoute, ok := record[routeField.AirtableName]
	if !ok {
		return fmt.Errorf("route field '%s' not found in record", routeField.AirtableName)
	}
	route, ok := rawRoute.(string)
	if !ok {
		return fmt.Errorf("route is not a string: %v", rawRoute)
	}
	route = strings.TrimSpace(route)
	if route == "" {
		return fmt.Errorf("route field is empty")
	}

	// Extract origin (optional - can be empty for event routes)
	var origin string
	if originField != nil {
		if rawOrigin, ok := record[originField.AirtableName]; ok {
			if originStr, ok := rawOrigin.(string); ok {
				origin = strings.TrimSpace(originStr)
			}
		}
	}

	// Extract destination (optional - can be empty for event routes)
	var destination string
	if destField != nil {
		if rawDest, ok := record[destField.AirtableName]; ok {
			if destStr, ok := rawDest.(string); ok {
				destination = strings.TrimSpace(destStr)
			}
		}
	}

	// Create route entity
	routeATSynced := &RouteATSynced{
		ATID:        airtableRecordID,
		ServerID:    vaID,
		Origin:      origin,
		Destination: destination,
		Route:       route,
	}

	// Parse route field to extract ICAO codes and enrich with airport coordinates
	j.enrichRouteWithAirportData(ctx, routeATSynced)

	// Upsert into route_at_synced table
	if err := j.syncRepo.UpsertRoute(ctx, routeATSynced); err != nil {
		return fmt.Errorf("failed to upsert route: %w", err)
	}

	// Log with route as primary identifier and coordinates if available
	var logMsg string
	if routeATSynced.OriginLat.Valid && routeATSynced.DestinationLat.Valid {
		logMsg = fmt.Sprintf("[RouteSyncJob] Upserted route '%s' with coordinates: origin (%.4f, %.4f) dest (%.4f, %.4f) (record: %s)",
			route, routeATSynced.OriginLat.Float64, routeATSynced.OriginLon.Float64,
			routeATSynced.DestinationLat.Float64, routeATSynced.DestinationLon.Float64, airtableRecordID)
	} else if origin != "" && destination != "" {
		logMsg = fmt.Sprintf("[RouteSyncJob] Upserted route '%s' (%s → %s) (no airport data found) (record: %s)",
			route, origin, destination, airtableRecordID)
	} else {
		logMsg = fmt.Sprintf("[RouteSyncJob] Upserted route '%s' (event/special) (record: %s)", route, airtableRecordID)
	}
	log.Print(logMsg)

	return nil
}

// enrichRouteWithAirportData parses the route field (format: KJFK-EGLL) and enriches with airport coordinates
func (j *RouteSyncJob) enrichRouteWithAirportData(ctx context.Context, routeATSynced *RouteATSynced) {
	if routeATSynced.Route == "" {
		return
	}

	// Parse route field on "-" separator
	parts := strings.Split(routeATSynced.Route, "-")
	if len(parts) != 2 {
		// Route doesn't match expected format, skip gracefully
		log.Printf("[RouteSyncJob] Route '%s' does not match ICAO-ICAO format, skipping airport enrichment", routeATSynced.Route)
		return
	}

	originICAO := strings.TrimSpace(parts[0])
	destICAO := strings.TrimSpace(parts[1])

	// Look up origin airport
	if originICAO != "" {
		if origin, err := j.airportRepo.FindByICAO(ctx, originICAO); err != nil {
			log.Printf("[RouteSyncJob] Error looking up origin airport %s: %v", originICAO, err)
		} else if origin != nil {
			routeATSynced.OriginLat.Float64 = origin.Latitude
			routeATSynced.OriginLat.Valid = true
			routeATSynced.OriginLon.Float64 = origin.Longitude
			routeATSynced.OriginLon.Valid = true
		} else {
			// Origin airport not found
			log.Printf("[RouteSyncJob] Origin airport %s not found in database", originICAO)
		}
	}

	// Look up destination airport
	if destICAO != "" {
		if dest, err := j.airportRepo.FindByICAO(ctx, destICAO); err != nil {
			log.Printf("[RouteSyncJob] Error looking up destination airport %s: %v", destICAO, err)
		} else if dest != nil {
			routeATSynced.DestinationLat.Float64 = dest.Latitude
			routeATSynced.DestinationLat.Valid = true
			routeATSynced.DestinationLon.Float64 = dest.Longitude
			routeATSynced.DestinationLon.Valid = true
		} else {
			// Destination airport not found
			log.Printf("[RouteSyncJob] Destination airport %s not found in database", destICAO)
		}
	}
}

// getLastSyncTimestamp gets the most recent sync timestamp for this VA from sync history
func (j *RouteSyncJob) getLastSyncTimestamp(ctx context.Context, vaID string) (*string, error) {
	lastSyncTime, err := j.syncRepo.GetLastSyncTimeForEvent(ctx, SyncEventRoutesAT)

	if err != nil {
		return nil, fmt.Errorf("failed to query last sync timestamp: %w", err)
	}

	// If no sync history found, return nil (do full sync)
	if lastSyncTime == nil {
		return nil, nil
	}

	// Format as ISO 8601 string for Airtable filtering
	timestamp := lastSyncTime.Format(time.RFC3339)
	return &timestamp, nil
}

// RunScheduled runs the route sync job on a schedule
func (j *RouteSyncJob) RunScheduled(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	if err := j.Run(ctx); err != nil {
		log.Printf("[RouteSyncJob] Error in initial run: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := j.Run(ctx); err != nil {
				log.Printf("[RouteSyncJob] Error in scheduled run: %v", err)
			}
		case <-ctx.Done():
			log.Printf("[RouteSyncJob] Shutting down scheduled sync")
			return
		}
	}
}
