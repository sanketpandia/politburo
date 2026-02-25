package pireps

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/infra/queue"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/sync"
)

// PirepSyncJob handles syncing PIREP data from Airtable to local database
type PirepSyncJob struct {
	db               *gorm.DB
	cache            cache.CacheInterface
	configRepo       *repositories.DataProviderConfigRepo
	pirepRepo        *Repository      // PIREP repository for upserts
	syncRepo         *sync.Repository // Sync repository for sync history
	airtableProvider *providers.AirtableProvider
	redisQueue       *queue.RedisQueueService // Redis queue for async processing
	useQueue         bool                      // Whether to use queue-based processing
}

// NewPirepSyncJob creates a new PIREP sync job instance
func NewPirepSyncJob(
	db *gorm.DB,
	cache cache.CacheInterface,
	configRepo *repositories.DataProviderConfigRepo,
	pirepRepo *Repository,
	syncRepo *sync.Repository,
	redisQueue *queue.RedisQueueService,
) *PirepSyncJob {
	return &PirepSyncJob{
		db:               db,
		cache:            cache,
		configRepo:       configRepo,
		pirepRepo:        pirepRepo,
		syncRepo:         syncRepo,
		airtableProvider: providers.NewAirtableProvider(cache),
		redisQueue:       redisQueue,
		useQueue:         redisQueue != nil, // Use queue if provided
	}
}

// Run executes the PIREP sync job for all active VAs with Airtable enabled
func (j *PirepSyncJob) Run(ctx context.Context) error {
	start := time.Now()
	log.Printf("[PirepSyncJob] Starting PIREP sync at %s", start.Format(time.RFC3339))

	// Get all VAs that have active Airtable configs
	var vaIDs []string
	err := j.db.WithContext(ctx).
		Table("va_data_provider_configs").
		Where("provider_type = ? AND is_active = ?", "airtable", true).
		Pluck("va_id", &vaIDs).Error

	if err != nil {
		log.Printf("[PirepSyncJob] Error fetching active VAs: %v", err)
		return fmt.Errorf("failed to fetch active VAs: %w", err)
	}

	if len(vaIDs) == 0 {
		log.Printf("[PirepSyncJob] No VAs with active Airtable configs found")
		return nil
	}

	log.Printf("[PirepSyncJob] Found %d VAs with active Airtable configs", len(vaIDs))

	// Sync PIREPs for each VA
	totalSynced := 0
	for _, vaID := range vaIDs {
		synced, err := j.SyncVAPireps(ctx, vaID)
		if err != nil {
			log.Printf("[PirepSyncJob] Error syncing PIREPs for VA %s: %v", vaID, err)
			// Continue with other VAs even if one fails
			continue
		}
		j.syncRepo.RecordSync(ctx, vaID, sync.SyncEventPirepsAT)
		totalSynced += synced
	}

	log.Printf("[PirepSyncJob] Completed PIREP sync in %s. Total PIREPs synced: %d",
		time.Since(start).Truncate(time.Millisecond), totalSynced)

	return nil
}

// SyncVAPireps syncs PIREPs for a specific VA (exported for manual triggering)
func (j *PirepSyncJob) SyncVAPireps(ctx context.Context, vaID string) (int, error) {
	start := time.Now()
	log.Printf("[PirepSyncJob] Syncing PIREPs for VA %s", vaID)

	// Get active config for this VA
	config, err := j.configRepo.GetActiveConfig(ctx, vaID, "airtable")
	if err != nil {
		return 0, fmt.Errorf("failed to get active config: %w", err)
	}

	if config == nil {
		log.Printf("[PirepSyncJob] No active config found for VA %s", vaID)
		return 0, nil
	}

	// Parse config data
	configData, err := repositories.ParseConfigData(config.ConfigData)
	if err != nil {
		return 0, fmt.Errorf("failed to parse config data: %w", err)
	}

	// Get PIREP schema
	pirepSchema := configData.GetSchemaByType("pirep")
	if pirepSchema == nil {
		log.Printf("[PirepSyncJob] No pirep schema configured for VA %s", vaID)
		return 0, nil
	}

	if !pirepSchema.Enabled {
		log.Printf("[PirepSyncJob] PIREP schema is disabled for VA %s", vaID)
		return 0, nil
	}

	// Get VA name for logging
	var vaName string
	j.db.WithContext(ctx).
		Table("virtual_airlines").
		Where("id = ?", vaID).
		Pluck("name", &vaName)

	log.Printf("[PirepSyncJob] VA: %s (%s), Table: %s", vaName, vaID, pirepSchema.TableName)

	// Get last sync timestamp for incremental sync
	lastModified, err := j.getLastSyncTimestamp(ctx, vaID)
	if err != nil {
		log.Printf("[PirepSyncJob] VA %s: Error getting last sync timestamp: %v. Doing full sync.", vaName, err)
		lastModified = nil
	}

	if lastModified != nil {
		log.Printf("[PirepSyncJob] VA %s: Incremental sync from %s", vaName, *lastModified)
	} else {
		log.Printf("[PirepSyncJob] VA %s: Full sync (no previous sync timestamp)", vaName)
	}

	// Check if schema has last_modified_field configured
	if lastModified != nil && pirepSchema.LastModifiedField == "" {
		log.Printf("[PirepSyncJob] VA %s: Warning - no last_modified_field configured in schema, cannot filter by date. Doing full sync.", vaName)
		lastModified = nil
	}

	// Set config in context for provider
	ctx = context.WithValue(ctx, "provider_config", configData)

	// Fetch PIREPs with pagination and enqueue to Redis (if enabled) or process directly
	offset := ""
	pageCount := 0
	enqueuedCount := 0
	syncedCount := 0
	errorCount := 0

	streamName := fmt.Sprintf("pirep:sync:%s", vaID)

	// If using queue, ensure consumer group exists
	if j.useQueue {
		if err := j.redisQueue.CreateConsumerGroup(ctx, streamName, "pirep-workers"); err != nil {
			log.Printf("[PirepSyncJob] VA %s: Warning - failed to create consumer group: %v", vaName, err)
			// Continue anyway - group might already exist
		}
	}

	for {
		pageCount++
		filters := &providers.SyncFilters{
			Offset:        offset,
			Limit:         100, // Batch size
			ModifiedSince: lastModified,
		}

		recordSet, err := j.airtableProvider.FetchRecords(ctx, pirepSchema, filters)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch records (page %d): %w", pageCount, err)
		}

		log.Printf("[PirepSyncJob] VA %s: Fetched page %d with %d records", vaName, pageCount, len(recordSet.Records))

		if j.useQueue {
			// Queue-based processing: Enqueue batch to Redis
			var queueItems []*queue.PirepQueueItem
			for _, record := range recordSet.Records {
				queueItems = append(queueItems, &queue.PirepQueueItem{
					VATID:            vaID,
					AirtableRecordID: record.ID,
					Fields:           record.Fields,
					CreatedTime:      record.CreatedTime,
				})
			}

			if err := j.redisQueue.EnqueuePirepBatch(ctx, streamName, queueItems); err != nil {
				log.Printf("[PirepSyncJob] VA %s: Error enqueuing batch: %v", vaName, err)
				errorCount += len(queueItems)
			} else {
				enqueuedCount += len(queueItems)
			}
		} else {
			// Direct processing: Process immediately (streaming)
			for _, record := range recordSet.Records {
				if err := j.upsertPirep(ctx, vaID, record.ID, record.Fields, record.CreatedTime, pirepSchema); err != nil {
					log.Printf("[PirepSyncJob] VA %s: Error upserting record: %v", vaName, err)
					errorCount++
					continue
				}
				syncedCount++
			}
		}

		if !recordSet.HasMore {
			break
		}
		offset = recordSet.Offset
	}

	if j.useQueue {
		log.Printf("[PirepSyncJob] VA %s: Completed in %s. Enqueued: %d, Errors: %d",
			vaName, time.Since(start).Truncate(time.Millisecond), enqueuedCount, errorCount)
		log.Printf("[PirepSyncJob] VA %s: Queue: %s - Workers will process items asynchronously", vaName, streamName)
	} else {
		log.Printf("[PirepSyncJob] VA %s: Completed in %s. Synced: %d, Errors: %d",
			vaName, time.Since(start).Truncate(time.Millisecond), syncedCount, errorCount)
	}

	// Record successful sync in sync history (only if not using queue or if direct processing)
	if !j.useQueue {
		if err := j.syncRepo.RecordSync(ctx, vaID, SyncEventPirepsAT); err != nil {
			log.Printf("[PirepSyncJob] VA %s: Warning - failed to record sync history: %v", vaName, err)
		}
	}

	// Return count (enqueued if using queue, synced if direct)
	if j.useQueue {
		return enqueuedCount, nil
	}
	return syncedCount, nil
}

// upsertPirep updates or creates a PIREP record in pirep_at_synced
func (j *PirepSyncJob) upsertPirep(ctx context.Context, vaID string, airtableRecordID string, record map[string]interface{}, createdTime string, schema *dtos.EntitySchema) error {
	// Extract field mappings
	routeField := schema.GetFieldMapping("route")
	flightModeField := schema.GetFieldMapping("flight_mode")
	flightTimeField := schema.GetFieldMapping("flight_time")
	pilotCallsignField := schema.GetFieldMapping("pilot_callsign")
	aircraftField := schema.GetFieldMapping("aircraft")
	liveryField := schema.GetFieldMapping("livery")
	routeATIDField := schema.GetFieldMapping("route_at_id")
	pilotATIDField := schema.GetFieldMapping("pilot_at_id")

	// Extract route (optional but recommended)
	var route string
	if routeField != nil {
		if rawRoute, ok := record[routeField.AirtableName]; ok {
			if routeStr, ok := rawRoute.(string); ok {
				route = strings.TrimSpace(routeStr)
			}
		}
	}

	// Extract flight mode (optional)
	var flightMode string
	if flightModeField != nil {
		if rawMode, ok := record[flightModeField.AirtableName]; ok {
			if modeStr, ok := rawMode.(string); ok {
				flightMode = strings.TrimSpace(modeStr)
			}
		}
	}

	// Extract flight time (optional)
	var flightTime *float64
	if flightTimeField != nil {
		if rawTime, ok := record[flightTimeField.AirtableName]; ok {
			switch v := rawTime.(type) {
			case float64:
				flightTime = &v
			case int:
				ft := float64(v)
				flightTime = &ft
			}
		}
	}

	// Extract pilot callsign (optional but recommended)
	var pilotCallsign string
	if pilotCallsignField != nil {
		if rawCallsign, ok := record[pilotCallsignField.AirtableName]; ok {
			if callsignStr, ok := rawCallsign.(string); ok {
				pilotCallsign = strings.TrimSpace(callsignStr)
			}
		}
	}

	// Extract aircraft (optional - use string as is)
	var aircraft string
	if aircraftField != nil {
		if rawAircraft, ok := record[aircraftField.AirtableName]; ok {
			if aircraftStr, ok := rawAircraft.(string); ok {
				aircraft = strings.TrimSpace(aircraftStr)
			}
		}
	}

	// Extract livery (optional - use string as is)
	var livery string
	if liveryField != nil {
		if rawLivery, ok := record[liveryField.AirtableName]; ok {
			if liveryStr, ok := rawLivery.(string); ok {
				livery = strings.TrimSpace(liveryStr)
			}
		}
	}

	// Extract route_at_id (optional reference)
	var routeATID *string
	if routeATIDField != nil {
		if rawRouteID, ok := record[routeATIDField.AirtableName]; ok {
			// Airtable returns array of record IDs for linked records
			if idArray, ok := rawRouteID.([]interface{}); ok && len(idArray) > 0 {
				if idStr, ok := idArray[0].(string); ok {
					routeATID = &idStr
				}
			}
		}
	}

	// Extract pilot_at_id (optional reference)
	var pilotATID *string
	if pilotATIDField != nil {
		if rawPilotID, ok := record[pilotATIDField.AirtableName]; ok {
			// Airtable returns array of record IDs for linked records
			if idArray, ok := rawPilotID.([]interface{}); ok && len(idArray) > 0 {
				if idStr, ok := idArray[0].(string); ok {
					pilotATID = &idStr
				}
			}
		}
	}

	// Parse Airtable created time
	var atCreatedTime *time.Time
	if createdTime != "" {
		if t, err := time.Parse(time.RFC3339, createdTime); err == nil {
			atCreatedTime = &t
		}
	}

	// Create PIREP entity
	pirepATSynced := &PirepATSynced{
		ATID:          airtableRecordID,
		ServerID:      vaID,
		Route:         route,
		FlightMode:    flightMode,
		FlightTime:    flightTime,
		PilotCallsign: pilotCallsign,
		Aircraft:      aircraft,
		Livery:        livery,
		RouteATID:     routeATID,
		PilotATID:     pilotATID,
		ATCreatedTime: atCreatedTime,
	}

	// Upsert into pirep_at_synced table
	if err := j.pirepRepo.Upsert(ctx, pirepATSynced); err != nil {
		return fmt.Errorf("failed to upsert PIREP: %w", err)
	}

	// Log with relevant info
	log.Printf("[PirepSyncJob] Upserted PIREP: pilot=%s, route=%s, aircraft=%s, livery=%s, mode=%s, time=%.2fh (record: %s)",
		pilotCallsign, route, aircraft, livery, flightMode, getFlightTimeValue(flightTime), airtableRecordID)

	return nil
}

// Helper to get flight time value safely
func getFlightTimeValue(ft *float64) float64 {
	if ft == nil {
		return 0.0
	}
	return *ft
}

// getLastSyncTimestamp gets the most recent sync timestamp for this VA from sync history
func (j *PirepSyncJob) getLastSyncTimestamp(ctx context.Context, vaID string) (*string, error) {
	lastSyncTime, err := j.syncRepo.GetLastSyncTimeForEvent(ctx, sync.SyncEventPirepsAT)

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

// RunScheduled runs the PIREP sync job on a schedule
func (j *PirepSyncJob) RunScheduled(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	if err := j.Run(ctx); err != nil {
		log.Printf("[PirepSyncJob] Error in initial run: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := j.Run(ctx); err != nil {
				log.Printf("[PirepSyncJob] Error in scheduled run: %v", err)
			}
		case <-ctx.Done():
			log.Printf("[PirepSyncJob] Shutting down scheduled sync")
			return
		}
	}
}
