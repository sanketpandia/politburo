package va_routes

import (
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	platformVA "infinite-experiment/politburo/internal/platform/va"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SyncJob handles syncing route data from Airtable to local database
type SyncJob struct {
	db               *gorm.DB
	cache            cache.CacheInterface
	configRepo       *repositories.DataProviderConfigRepo
	syncHistoryRepo  *repositories.VASyncHistoryRepo
	routeRepo        *Repository
	airportRepo      *repositories.AirportRepository
	airtableProvider *providers.AirtableProvider
	metrics          *metrics.MetricsRegistry // Metrics registry for tracking
}

// NewSyncJob creates a new route sync job instance
func NewSyncJob(
	db *gorm.DB,
	cache cache.CacheInterface,
	configRepo *repositories.DataProviderConfigRepo,
	syncHistoryRepo *repositories.VASyncHistoryRepo,
	routeRepo *Repository,
	airportRepo *repositories.AirportRepository,
	metricsReg *metrics.MetricsRegistry,
) *SyncJob {
	return &SyncJob{
		db:               db,
		cache:            cache,
		configRepo:       configRepo,
		syncHistoryRepo:  syncHistoryRepo,
		routeRepo:        routeRepo,
		airportRepo:      airportRepo,
		airtableProvider: providers.NewAirtableProvider(cache),
		metrics:          metricsReg,
	}
}

// Name returns the job name for the scheduler interface
func (j *SyncJob) Name() string {
	return "route_sync_job"
}

// Run executes the route sync job for all active VAs with Airtable enabled
func (j *SyncJob) Run(ctx context.Context) error {
	start := time.Now()
	log.Printf("[RouteSyncJob] Starting route sync at %s", start.Format(time.RFC3339))

	// Get all VAs that have active Airtable configs (use DISTINCT to avoid duplicates)
	var vaIDs []string
	err := j.db.WithContext(ctx).
		Table("va_data_provider_configs").
		Where("provider_type = ? AND is_active = ?", "airtable", true).
		Distinct("va_id").
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
	totalProcessed := 0
	for _, vaID := range vaIDs {
		processed, err := j.SyncVARoutes(ctx, vaID)
		if err != nil {
			log.Printf("[RouteSyncJob] Error syncing routes for VA %s: %v", vaID, err)
			// Continue with other VAs even if one fails
			continue
		}
		totalProcessed += processed
	}

	log.Printf("[RouteSyncJob] Completed route sync in %s. Total routes synced: %d",
		time.Since(start).Truncate(time.Millisecond), totalProcessed)

	return nil
}

// SyncVARoutes syncs routes for a specific VA (exported for manual triggering)
func (j *SyncJob) SyncVARoutes(ctx context.Context, vaID string) (int, error) {
	start := time.Now()
	log.Printf("[RouteSyncJob] Syncing routes for VA %s", vaID)

	// Get credentials config for this VA
	credentialsConfig, err := j.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "credentials")
	if err != nil {
		return 0, fmt.Errorf("failed to get credentials config: %w", err)
	}

	if credentialsConfig == nil {
		log.Printf("[RouteSyncJob] No active credentials config found for VA %s", vaID)
		return 0, nil
	}

	// Parse credentials config data directly (credentials config is stored as flat structure)
	var credsData struct {
		APIKey       string            `json:"api_key"`
		BaseID       string            `json:"base_id"`
		SyncSettings dtos.SyncSettings `json:"sync_settings"`
	}
	credsBytes, err := json.Marshal(credentialsConfig.ConfigData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal credentials config data: %w", err)
	}
	if err := json.Unmarshal(credsBytes, &credsData); err != nil {
		return 0, fmt.Errorf("failed to parse credentials config data: %w", err)
	}

	// Validate credentials
	if credsData.APIKey == "" {
		return 0, fmt.Errorf("API key is empty in credentials config")
	}
	if credsData.BaseID == "" {
		return 0, fmt.Errorf("Base ID is empty in credentials config")
	}

	// Get route schema config separately
	routeConfig, err := j.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "route")
	if err != nil {
		return 0, fmt.Errorf("failed to get route schema config: %w", err)
	}

	if routeConfig == nil {
		log.Printf("[RouteSyncJob] No route schema configured for VA %s", vaID)
		return 0, nil
	}

	// Parse route schema config data (this is just the EntitySchema, not full ProviderConfigData)
	var routeSchema dtos.EntitySchema
	bytes, err := json.Marshal(routeConfig.ConfigData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal route config data: %w", err)
	}

	if err := json.Unmarshal(bytes, &routeSchema); err != nil {
		return 0, fmt.Errorf("failed to parse route schema: %w", err)
	}

	// Set entity_type if not set (for backward compatibility)
	if routeSchema.EntityType == "" {
		routeSchema.EntityType = "route"
	}

	// Check if enabled field exists in raw JSON (workaround for potential JSON parsing issue)
	var rawData map[string]interface{}
	if err := json.Unmarshal(bytes, &rawData); err == nil {
		if enabledVal, exists := rawData["enabled"]; exists {
			if enabledBool, ok := enabledVal.(bool); ok {
				routeSchema.Enabled = enabledBool
			}
		} else {
			// enabled field missing - default to true if schema has fields configured
			if len(routeSchema.Fields) > 0 {
				log.Printf("[RouteSyncJob] VA %s: Enabled field missing in JSON, defaulting to true (has %d fields)", vaID, len(routeSchema.Fields))
				routeSchema.Enabled = true
			}
		}
	}

	if !routeSchema.Enabled {
		log.Printf("[RouteSyncJob] Route schema is disabled for VA %s", vaID)
		return 0, nil
	}

	// Validate that route field is configured (required for route sync)
	hasRouteField := false
	for _, field := range routeSchema.Fields {
		if field.InternalName == "route" {
			hasRouteField = true
			break
		}
	}
	if !hasRouteField {
		log.Printf("[RouteSyncJob] Route schema for VA %s is missing required 'route' field mapping. Please configure field mappings in the datasource settings.", vaID)
		return 0, fmt.Errorf("route schema missing required 'route' field mapping")
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

	// Convert dtos.EntitySchema to platformVA.EntitySchema
	vaRouteSchema := convertDTOsEntitySchema(&routeSchema)

	// Build credentials from parsed data
	creds := &platformVA.ProviderCredentials{
		APIKey: credsData.APIKey,
		BaseID: credsData.BaseID,
		SyncSettings: platformVA.SyncSettings{
			BatchSize:          credsData.SyncSettings.BatchSize,
			RateLimitPerSecond: credsData.SyncSettings.RateLimitPerSecond,
			RetryAttempts:      credsData.SyncSettings.RetryAttempts,
			TimeoutSeconds:     credsData.SyncSettings.TimeoutSeconds,
		},
	}

	// Set credentials in context for provider
	ctx = context.WithValue(ctx, "provider_credentials", creds)

	// Fetch routes with pagination
	offset := ""
	pageCount := 0
	syncedCount := 0
	errorCount := 0

	for {
		pageCount++
		filters := &providers.SyncFilters{
			Offset:        offset,
			Limit:         100, // Batch size
			ModifiedSince: lastModified,
		}

		recordSet, err := j.airtableProvider.FetchRecords(ctx, vaRouteSchema, filters)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch records (page %d): %w", pageCount, err)
		}

		log.Printf("[RouteSyncJob] VA %s: Fetched page %d with %d records", vaName, pageCount, len(recordSet.Records))

		// Process records directly (no queue needed for routes)
		for _, record := range recordSet.Records {
			if err := j.upsertRoute(ctx, vaID, record.ID, record.Fields, vaRouteSchema); err != nil {
				log.Printf("[RouteSyncJob] VA %s: Error upserting record: %v", vaName, err)
				errorCount++
				continue
			}
			syncedCount++

			// Track metrics
			if j.metrics != nil {
				j.metrics.SyncJobRecordsProcessed.WithLabelValues("route_sync_job", "airtable", "route", vaID, "success").Inc()
			}
		}

		if !recordSet.HasMore {
			break
		}
		offset = recordSet.Offset
	}

	log.Printf("[RouteSyncJob] VA %s: Completed in %s. Synced: %d, Errors: %d",
		vaName, time.Since(start).Truncate(time.Millisecond), syncedCount, errorCount)

	// Record sync history after successful sync
	if syncedCount > 0 {
		if err := j.syncHistoryRepo.RecordSync(ctx, vaID, constants.SyncEventRoutesAT); err != nil {
			log.Printf("[RouteSyncJob] VA %s: Warning - failed to record sync history: %v", vaName, err)
		}
	}

	return syncedCount, nil
}

// upsertRoute updates or creates a route record in route_at_synced
func (j *SyncJob) upsertRoute(ctx context.Context, vaID string, airtableRecordID string, record map[string]interface{}, schema *platformVA.EntitySchema) error {
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
	routeATSynced := &RouteATSyncedGORM{
		ATID:        airtableRecordID,
		ServerID:    vaID,
		Origin:      origin,
		Destination: destination,
		Route:       route,
	}

	// Parse route field to extract ICAO codes and enrich with airport coordinates
	j.enrichRouteWithAirportData(ctx, routeATSynced)

	// Upsert into route_at_synced table
	if err := j.routeRepo.Upsert(ctx, routeATSynced); err != nil {
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
func (j *SyncJob) enrichRouteWithAirportData(ctx context.Context, routeATSynced *RouteATSyncedGORM) {
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
func (j *SyncJob) getLastSyncTimestamp(ctx context.Context, vaID string) (*string, error) {
	lastSyncTime, err := j.syncHistoryRepo.GetLastSyncTimeForEvent(ctx, constants.SyncEventRoutesAT)

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

// convertDTOsEntitySchema converts dtos.EntitySchema to platformVA.EntitySchema
func convertDTOsEntitySchema(dtoSchema *dtos.EntitySchema) *platformVA.EntitySchema {
	if dtoSchema == nil {
		return nil
	}
	fields := make([]platformVA.FieldMapping, len(dtoSchema.Fields))
	for i, f := range dtoSchema.Fields {
		fields[i] = platformVA.FieldMapping{
			InternalName:  f.InternalName,
			AirtableName:  f.AirtableName,
			DataType:      f.DataType,
			Required:      f.Required,
			DefaultValue:  f.DefaultValue,
			DisplayName:   f.DisplayName,
			IsUserVisible: f.IsUserVisible,
			DisplayFormat: f.DisplayFormat,
			BotMetadata:   f.BotMetadata,
		}
	}
	return &platformVA.EntitySchema{
		EntityType:        dtoSchema.EntityType,
		TableName:         dtoSchema.TableName,
		Enabled:           dtoSchema.Enabled,
		LastModifiedField: dtoSchema.LastModifiedField,
		Fields:            fields,
	}
}
