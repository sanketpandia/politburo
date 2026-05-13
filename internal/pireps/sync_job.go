package pireps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"gorm.io/gorm"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/infra/queue"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

// PirepSyncJob handles syncing PIREP data from Airtable to local database
type PirepSyncJob struct {
	db               *gorm.DB
	cache            cache.CacheInterface
	vaRepo           *platformVA.Repository     // VA repository for fetching configs
	pirepRepo        *Repository                // PIREP repository for upserts
	syncRepo         *platformVA.SyncRepository // Sync repository for sync history
	airtableProvider *providers.AirtableProvider
	redisQueue       *queue.RedisQueueService // Redis queue for async processing
	useQueue         bool                     // Whether to use queue-based processing
	publisher        message.Publisher        // Watermill publisher for dual-write; may be nil
	metrics          *metrics.MetricsRegistry
}

// NewPirepSyncJob creates a new PIREP sync job instance
func NewPirepSyncJob(
	db *gorm.DB,
	cache cache.CacheInterface,
	vaRepo *platformVA.Repository,
	pirepRepo *Repository,
	syncRepo *platformVA.SyncRepository,
	redisQueue *queue.RedisQueueService,
	metricsReg *metrics.MetricsRegistry,
) *PirepSyncJob {
	return &PirepSyncJob{
		db:               db,
		cache:            cache,
		vaRepo:           vaRepo,
		pirepRepo:        pirepRepo,
		syncRepo:         syncRepo,
		airtableProvider: providers.NewAirtableProvider(cache),
		redisQueue:       redisQueue,
		useQueue:         redisQueue != nil,
		metrics:          metricsReg,
	}
}

// SetPublisher wires a watermill Publisher for dual-write. When set, each
// Airtable record is also published to TopicPirepSync in addition to the
// existing Redis queue path. A nil publisher is silently ignored.
func (j *PirepSyncJob) SetPublisher(pub message.Publisher) {
	j.publisher = pub
}

// Run executes the PIREP sync job for all active VAs with Airtable enabled
func (j *PirepSyncJob) Run(ctx context.Context) error {
	start := time.Now()
	logging.Info("PIREP sync job starting")
	defer func() {
		if j.metrics != nil {
			j.metrics.SyncJobDuration.WithLabelValues("pirep_sync_job", "airtable", "pirep").Observe(time.Since(start).Seconds())
		}
	}()

	// Get all VAs that have active Airtable configs
	var vaIDs []string
	err := j.db.WithContext(ctx).
		Table("va_data_provider_configs").
		Where("provider_type = ? AND config_type = ? AND is_active = ?", "airtable", "pirep", true).
		Pluck("va_id", &vaIDs).Error

	if err != nil {
		logging.Error("PIREP sync job: failed to fetch active VAs", "error", err)
		return fmt.Errorf("failed to fetch active VAs: %w", err)
	}

	if len(vaIDs) == 0 {
		logging.Info("PIREP sync job: no VAs with active Airtable PIREP configs")
		return nil
	}

	logging.Info("PIREP sync job: VAs with active configs", "count", len(vaIDs))

	// Sync PIREPs for each VA
	totalSynced := 0
	for _, vaID := range vaIDs {
		synced, err := j.SyncVAPireps(ctx, vaID)
		if err != nil {
			logging.Error("PIREP sync job: failed to sync VA", "va_id", vaID, "error", err)
			// Continue with other VAs even if one fails
			continue
		}
		j.syncRepo.RecordSync(ctx, vaID, platformVA.SyncEventPirepsAT)
		totalSynced += synced
	}

	logging.Info("PIREP sync job completed", "duration", time.Since(start).Truncate(time.Millisecond), "total_synced", totalSynced)

	return nil
}

// SyncVAPireps syncs PIREPs for a specific VA (exported for manual triggering)
func (j *PirepSyncJob) SyncVAPireps(ctx context.Context, vaID string) (int, error) {
	start := time.Now()
	logging.Info("PIREP sync: syncing VA", "va_id", vaID)

	// Get PIREP schema using new platform/va config structure
	schemaConfig, err := j.vaRepo.GetAirtableSchema(ctx, vaID, "pirep")
	if err != nil {
		return 0, fmt.Errorf("failed to get PIREP schema: %w", err)
	}

	if schemaConfig == nil {
		logging.Info("PIREP sync: no schema configured", "va_id", vaID)
		return 0, nil
	}

	if !schemaConfig.Enabled {
		logging.Info("PIREP sync: schema disabled", "va_id", vaID)
		return 0, nil
	}

	// Convert SchemaConfig to EntitySchema for provider
	pirepSchema := schemaConfig.ToEntitySchema("pirep")

	// Get Airtable credentials for provider
	creds, err := j.vaRepo.GetAirtableCredentials(ctx, vaID)
	if err != nil {
		return 0, fmt.Errorf("failed to get Airtable credentials: %w", err)
	}

	if creds == nil {
		logging.Info("PIREP sync: no Airtable credentials", "va_id", vaID)
		return 0, nil
	}

	// Get VA name for logging
	var vaName string
	j.db.WithContext(ctx).
		Table("virtual_airlines").
		Where("id = ?", vaID).
		Pluck("name", &vaName)

	logging.Debug("PIREP sync: schema details", "va_name", vaName, "va_id", vaID, "table", pirepSchema.TableName, "last_modified_field", pirepSchema.LastModifiedField)

	// Check if schema has last_modified_field configured - REQUIRED for incremental sync
	if pirepSchema.LastModifiedField == "" {
		logging.Info("PIREP sync: skipping — no last_modified_field configured", "va_id", vaID, "va_name", vaName)
		return 0, nil
	}

	// Get last sync timestamp for incremental sync
	lastModified, err := j.getLastSyncTimestamp(ctx, vaID)
	if err != nil {
		logging.Error("PIREP sync: failed to get last sync timestamp", "va_id", vaID, "va_name", vaName, "error", err)
		return 0, nil
	}

	if lastModified != nil {
		logging.Info("PIREP sync: incremental sync", "va_id", vaID, "since", *lastModified)
	} else {
		logging.Info("PIREP sync: full sync (no prior timestamp)", "va_id", vaID)
	}

	// Set credentials in context for provider
	ctx = context.WithValue(ctx, "provider_credentials", creds)

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
			logging.Warn("PIREP sync: failed to create consumer group", "va_id", vaID, "error", err)
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

		logging.Debug("PIREP sync: fetched page", "va_id", vaID, "page", pageCount, "count", len(recordSet.Records))

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
				logging.Error("PIREP sync: failed to enqueue batch", "va_id", vaID, "error", err)
				errorCount += len(queueItems)
			} else {
				enqueuedCount += len(queueItems)
				if j.metrics != nil {
					j.metrics.QueueEnqueuedTotal.WithLabelValues("pirep_queue", "pirep").Add(float64(len(queueItems)))
					j.metrics.SyncJobRecordsProcessed.WithLabelValues("pirep_sync_job", "airtable", "pirep", vaID, "enqueued").Add(float64(len(queueItems)))
				}
			}

			// Dual-write: also publish each item to the watermill topic so the
			// new consumer group can process it independently of the Redis queue path.
			if j.publisher != nil {
				for _, qi := range queueItems {
					if err := PublishPirepItem(ctx, j.publisher, qi); err != nil {
						logging.Error("PIREP sync: failed to publish to watermill topic",
							"va_id", vaID,
							"record_id", qi.AirtableRecordID,
							"error", err,
						)
						// Non-fatal: watermill path is additive; log and continue.
					}
				}
			}
		} else {
			// Direct processing: Process immediately (streaming)
			for _, record := range recordSet.Records {
				if err := j.upsertPirep(ctx, vaID, record.ID, record.Fields, record.CreatedTime, pirepSchema); err != nil {
					logging.Error("PIREP sync: failed to upsert record", "va_id", vaID, "record_id", record.ID, "error", err)
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
		// Check queue status
		queueLength, _ := j.redisQueue.GetQueueLength(ctx, streamName)
		pendingCount, _ := j.redisQueue.GetPendingCount(ctx, streamName, "pirep-workers")

		// Get max at_created_time from database to see latest synced record
		maxTime, _ := j.pirepRepo.GetMaxATCreatedTime(ctx, vaID)
		maxTimeStr := "none"
		if maxTime != nil {
			maxTimeStr = maxTime.Format(time.RFC3339)
		}

		logging.Info("PIREP sync: VA queue-mode completed",
			"va_id", vaID,
			"duration", time.Since(start).Truncate(time.Millisecond),
			"enqueued", enqueuedCount,
			"errors", errorCount,
			"stream", streamName,
			"queue_length", queueLength,
			"pending", pendingCount,
			"max_at_created_time", maxTimeStr,
		)
	} else {
		logging.Info("PIREP sync: VA direct-mode completed",
			"va_id", vaID,
			"duration", time.Since(start).Truncate(time.Millisecond),
			"synced", syncedCount,
			"errors", errorCount,
		)
	}

	// Record successful sync in sync history (only if not using queue or if direct processing)
	if !j.useQueue {
		if err := j.syncRepo.RecordSync(ctx, vaID, platformVA.SyncEventPirepsAT); err != nil {
			logging.Warn("PIREP sync: failed to record sync history", "va_id", vaID, "error", err)
		}
	}

	// Return count (enqueued if using queue, synced if direct)
	if j.useQueue {
		return enqueuedCount, nil
	}
	return syncedCount, nil
}

// upsertPirep updates or creates a PIREP record in pirep_at_synced
func (j *PirepSyncJob) upsertPirep(ctx context.Context, vaID string, airtableRecordID string, record map[string]interface{}, createdTime string, schema *platformVA.EntitySchema) error {
	// Extract field mappings - support both old and new field names
	// Map "callsign" (new config) to "pilot_callsign" (database field)
	callsignField := schema.GetFieldMapping("callsign")
	pilotCallsignField := schema.GetFieldMapping("pilot_callsign")
	if callsignField != nil && pilotCallsignField == nil {
		// Use callsign field if pilot_callsign not found
		pilotCallsignField = callsignField
	}

	// Map "airline" (new config) to "livery" (database field)
	airlineField := schema.GetFieldMapping("airline")
	liveryField := schema.GetFieldMapping("livery")
	if airlineField != nil && liveryField == nil {
		// Use airline field if livery not found
		liveryField = airlineField
	}

	routeField := schema.GetFieldMapping("route")
	flightModeField := schema.GetFieldMapping("flight_mode")
	flightTimeField := schema.GetFieldMapping("flight_time")
	aircraftField := schema.GetFieldMapping("aircraft")
	routeATIDField := schema.GetFieldMapping("route_at_id")
	pilotATIDField := schema.GetFieldMapping("pilot_at_id")

	// Extract route (optional but recommended)
	// Note: Route field can be either a string or a linked record array
	var route string
	var routeATIDFromRoute *string
	if routeField != nil {
		if rawRoute, ok := record[routeField.AirtableName]; ok {
			// Check if it's a linked record array (most common case)
			if idArray, ok := rawRoute.([]interface{}); ok && len(idArray) > 0 {
				// Extract the first ID from the linked record array
				if idStr, ok := idArray[0].(string); ok {
					routeATIDFromRoute = &idStr
				}
			} else if routeStr, ok := rawRoute.(string); ok {
				// Fallback: treat as string
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
	// Note: Callsign field can be either a string or a linked record array
	// We extract the Airtable ID from linked records, but don't store the callsign string
	// Instead, we'll look it up from pilot_at_synced and get IFC ID
	var pilotATIDFromCallsign *string
	if pilotCallsignField != nil {
		if rawCallsign, ok := record[pilotCallsignField.AirtableName]; ok {
			// Check if it's a linked record array (most common case)
			if idArray, ok := rawCallsign.([]interface{}); ok && len(idArray) > 0 {
				// Extract the first ID from the linked record array
				if idStr, ok := idArray[0].(string); ok {
					pilotATIDFromCallsign = &idStr
				}
			}
			// Note: We don't store the callsign string here - we'll look it up from pilot_at_synced
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
	// Priority: 1) Explicit route_at_id field mapping, 2) From Route linked record array
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
	// Fallback: use ID extracted from Route field if no explicit route_at_id field
	if routeATID == nil && routeATIDFromRoute != nil {
		routeATID = routeATIDFromRoute
	}

	// Extract pilot_at_id (optional reference)
	// Priority: 1) Explicit pilot_at_id field mapping, 2) From Callsign linked record array
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
	// Fallback: use ID extracted from Callsign field if no explicit pilot_at_id field
	if pilotATID == nil && pilotATIDFromCallsign != nil {
		pilotATID = pilotATIDFromCallsign
	}

	// Look up pilot from pilot_at_synced to get callsign
	var pilotCallsignFromSync string
	if pilotATID != nil && *pilotATID != "" {
		// Query pilot_at_synced to get callsign
		type PilotSynced struct {
			Callsign string `gorm:"column:callsign"`
		}
		var pilotSynced PilotSynced
		err := j.db.WithContext(ctx).
			Table("pilot_at_synced").
			Where("at_id = ? AND server_id = ?", *pilotATID, vaID).
			First(&pilotSynced).Error
		
		if err == nil && pilotSynced.Callsign != "" {
			pilotCallsignFromSync = pilotSynced.Callsign
		}
	}

	// Look up route from route_at_synced to populate route field if missing
	if routeATID != nil && *routeATID != "" && route == "" {
		type RouteSynced struct {
			Route string `gorm:"column:route"`
		}
		var routeSynced RouteSynced
		err := j.db.WithContext(ctx).
			Table("route_at_synced").
			Where("at_id = ? AND server_id = ?", *routeATID, vaID).
			First(&routeSynced).Error
		
		if err == nil && routeSynced.Route != "" {
			route = routeSynced.Route
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
	// Use callsign from pilot_at_synced if available, otherwise keep empty
	finalPilotCallsign := pilotCallsignFromSync
	if finalPilotCallsign == "" {
		// Fallback: keep empty
		finalPilotCallsign = ""
	}
	
	pirepATSynced := &PirepATSynced{
		ATID:          airtableRecordID,
		ServerID:      vaID,
		Route:         route,
		FlightMode:    flightMode,
		FlightTime:    flightTime,
		PilotCallsign: finalPilotCallsign, // Store callsign from pilot_at_synced
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

	logging.Debug("PIREP upserted",
		"record_id", airtableRecordID,
		"pilot", finalPilotCallsign,
		"route", route,
		"aircraft", aircraft,
		"livery", livery,
		"mode", flightMode,
		"flight_time_h", getFlightTimeValue(flightTime),
	)

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
	lastSyncTime, err := j.syncRepo.GetLastSyncTimeForVAAndEvent(ctx, vaID, platformVA.SyncEventPirepsAT)

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

// Name returns the job name for the scheduler
func (j *PirepSyncJob) Name() string {
	return "pirep-sync"
}

// RunScheduled runs the PIREP sync job on a schedule
func (j *PirepSyncJob) RunScheduled(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	if err := j.Run(ctx); err != nil {
		logging.Error("PIREP sync: initial run failed", "error", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := j.Run(ctx); err != nil {
				logging.Error("PIREP sync: scheduled run failed", "error", err)
			}
		case <-ctx.Done():
			logging.Info("PIREP sync: scheduled job shutting down")
			return
		}
	}
}
