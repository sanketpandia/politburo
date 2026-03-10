package pilots

import (
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/infra/queue"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	platformVA "infinite-experiment/politburo/internal/platform/va"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SyncJob handles syncing pilot data from Airtable to local database
type SyncJob struct {
	db               *gorm.DB
	cache            cache.CacheInterface
	configRepo       *repositories.DataProviderConfigRepo
	syncRepo         *platformVA.SyncRepository
	pilotRepo        *Repository
	airtableProvider *providers.AirtableProvider
	redisQueue       *queue.RedisQueueService // Redis queue for async processing
	useQueue         bool                     // Whether to use queue-based processing
	metrics          *metrics.MetricsRegistry // Metrics registry for tracking
}

// NewSyncJob creates a new pilot sync job instance
func NewSyncJob(
	db *gorm.DB,
	cache cache.CacheInterface,
	configRepo *repositories.DataProviderConfigRepo,
	syncRepo *platformVA.SyncRepository,
	pilotRepo *Repository,
	redisQueue *queue.RedisQueueService,
	metricsReg *metrics.MetricsRegistry,
) *SyncJob {
	return &SyncJob{
		db:               db,
		cache:            cache,
		configRepo:       configRepo,
		syncRepo:         syncRepo,
		pilotRepo:        pilotRepo,
		airtableProvider: providers.NewAirtableProvider(cache),
		redisQueue:       redisQueue,
		useQueue:         redisQueue != nil, // Use queue if provided
		metrics:          metricsReg,
	}
}

// Name returns the job name for the scheduler interface
func (j *SyncJob) Name() string {
	return "pilot_sync_job"
}

// Run executes the pilot sync job for all active VAs with Airtable enabled
func (j *SyncJob) Run(ctx context.Context) error {
	start := time.Now()
	log.Printf("[PilotSyncJob] Starting pilot sync at %s", start.Format(time.RFC3339))

	// Get all VAs that have active Airtable configs (use DISTINCT to avoid duplicates)
	var vaIDs []string
	err := j.db.WithContext(ctx).
		Table("va_data_provider_configs").
		Where("provider_type = ? AND is_active = ?", "airtable", true).
		Distinct("va_id").
		Pluck("va_id", &vaIDs).Error

	if err != nil {
		log.Printf("[PilotSyncJob] Error fetching active VAs: %v", err)
		return fmt.Errorf("failed to fetch active VAs: %w", err)
	}

	if len(vaIDs) == 0 {
		log.Printf("[PilotSyncJob] No VAs with active Airtable configs found")
		return nil
	}

	log.Printf("[PilotSyncJob] Found %d VAs with active Airtable configs", len(vaIDs))

	// Sync pilots for each VA
	totalProcessed := 0
	for _, vaID := range vaIDs {
		// Sync regular pilots
		processed, err := j.SyncVAPilots(ctx, vaID)
		if err != nil {
			log.Printf("[PilotSyncJob] Error syncing pilots for VA %s: %v", vaID, err)
			// Continue with other VAs even if one fails
			continue
		}
		totalProcessed += processed

		// Sync career mode pilots
		cmProcessed, err := j.SyncVACareerModePilots(ctx, vaID)
		if err != nil {
			log.Printf("[PilotSyncJob] Error syncing career mode pilots for VA %s: %v", vaID, err)
			// Continue even if career mode sync fails
		} else {
			totalProcessed += cmProcessed
		}
	}

	if j.useQueue {
		log.Printf("[PilotSyncJob] Completed pilot sync in %s. Total pilots enqueued: %d",
			time.Since(start).Truncate(time.Millisecond), totalProcessed)
	} else {
		log.Printf("[PilotSyncJob] Completed pilot sync in %s. Total pilots synced: %d",
			time.Since(start).Truncate(time.Millisecond), totalProcessed)
	}

	return nil
}

// SyncVAPilots syncs pilots for a specific VA (exported for manual triggering)
func (j *SyncJob) SyncVAPilots(ctx context.Context, vaID string) (int, error) {
	start := time.Now()
	log.Printf("[PilotSyncJob] Syncing pilots for VA %s", vaID)

	// Get credentials config for this VA
	credentialsConfig, err := j.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "credentials")
	if err != nil {
		return 0, fmt.Errorf("failed to get credentials config: %w", err)
	}

	if credentialsConfig == nil {
		log.Printf("[PilotSyncJob] No active credentials config found for VA %s", vaID)
		return 0, nil
	}

	// Parse credentials config data directly (credentials config is stored as flat structure)
	// The config_data for credentials type is: {"api_key": "...", "base_id": "...", "sync_settings": {...}}
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

	// Get pilot schema config separately
	pilotConfig, err := j.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "pilot")
	if err != nil {
		return 0, fmt.Errorf("failed to get pilot schema config: %w", err)
	}

	if pilotConfig == nil {
		log.Printf("[PilotSyncJob] No pilot schema configured for VA %s", vaID)
		return 0, nil
	}

	// Parse pilot schema config data (this is just the EntitySchema, not full ProviderConfigData)
	var pilotSchema dtos.EntitySchema
	bytes, err := json.Marshal(pilotConfig.ConfigData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal pilot config data: %w", err)
	}

	// Debug: Log raw config data before parsing
	log.Printf("[PilotSyncJob] VA %s: Raw pilot config data: %s", vaID, string(bytes))

	if err := json.Unmarshal(bytes, &pilotSchema); err != nil {
		return 0, fmt.Errorf("failed to parse pilot schema: %w", err)
	}

	// Set entity_type if not set (for backward compatibility)
	if pilotSchema.EntityType == "" {
		pilotSchema.EntityType = "pilot"
	}

	// Debug: Log parsed schema values and raw JSON
	log.Printf("[PilotSyncJob] VA %s: Parsed pilot schema - Enabled: %v, TableName: %s, Fields: %d", vaID, pilotSchema.Enabled, pilotSchema.TableName, len(pilotSchema.Fields))
	log.Printf("[PilotSyncJob] VA %s: Raw config JSON: %s", vaID, string(bytes))

	// Check if enabled field exists in raw JSON (workaround for potential JSON parsing issue)
	var rawData map[string]interface{}
	if err := json.Unmarshal(bytes, &rawData); err == nil {
		if enabledVal, exists := rawData["enabled"]; exists {
			if enabledBool, ok := enabledVal.(bool); ok {
				// Override with value from raw JSON if different
				if enabledBool != pilotSchema.Enabled {
					log.Printf("[PilotSyncJob] VA %s: Enabled field mismatch - raw JSON: %v, parsed: %v, using raw value", vaID, enabledBool, pilotSchema.Enabled)
					pilotSchema.Enabled = enabledBool
				}
			}
		} else {
			// enabled field missing - default to true if schema has fields configured
			if len(pilotSchema.Fields) > 0 {
				log.Printf("[PilotSyncJob] VA %s: Enabled field missing in JSON, defaulting to true (has %d fields)", vaID, len(pilotSchema.Fields))
				pilotSchema.Enabled = true
			}
		}
	}

	if !pilotSchema.Enabled {
		log.Printf("[PilotSyncJob] Pilot schema is disabled for VA %s", vaID)
		return 0, nil
	}

	// Validate that callsign field is configured (required for pilot sync)
	hasCallsignField := false
	for _, field := range pilotSchema.Fields {
		if field.InternalName == "callsign" {
			hasCallsignField = true
			break
		}
	}
	if !hasCallsignField {
		log.Printf("[PilotSyncJob] Pilot schema for VA %s is missing required 'callsign' field mapping. Please configure field mappings in the datasource settings.", vaID)
		return 0, fmt.Errorf("pilot schema missing required 'callsign' field mapping")
	}

	// Get VA name for logging
	var vaName string
	j.db.WithContext(ctx).
		Table("virtual_airlines").
		Where("id = ?", vaID).
		Pluck("name", &vaName)

	log.Printf("[PilotSyncJob] VA: %s (%s), Table: %s", vaName, vaID, pilotSchema.TableName)

	// Get last sync timestamp for incremental sync
	lastModified, err := j.getLastSyncTimestamp(ctx, vaID)
	if err != nil {
		log.Printf("[PilotSyncJob] VA %s: Error getting last sync timestamp: %v. Doing full sync.", vaName, err)
		lastModified = nil
	}

	if lastModified != nil {
		log.Printf("[PilotSyncJob] VA %s: Incremental sync from %s", vaName, *lastModified)
	} else {
		log.Printf("[PilotSyncJob] VA %s: Full sync (no previous sync timestamp)", vaName)
	}

	// Check if schema has last_modified_field configured
	if lastModified != nil && pilotSchema.LastModifiedField == "" {
		log.Printf("[PilotSyncJob] VA %s: Warning - no last_modified_field configured in schema, cannot filter by date. Doing full sync.", vaName)
		lastModified = nil
	}

	// Convert dtos.EntitySchema to platformVA.EntitySchema
	vaPilotSchema := convertDTOsEntitySchema(&pilotSchema)

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

	// Log credentials extraction for debugging (mask API key)
	apiKeyMasked := ""
	if len(creds.APIKey) > 8 {
		apiKeyMasked = creds.APIKey[:8] + "..."
	} else {
		apiKeyMasked = "***"
	}
	log.Printf("[PilotSyncJob] VA %s: Extracted credentials - BaseID: %s, APIKey: %s", vaID, creds.BaseID, apiKeyMasked)

	// Set credentials in context for provider
	ctx = context.WithValue(ctx, "provider_credentials", creds)

	// Fetch pilots with pagination and enqueue to Redis (if enabled) or process directly
	offset := ""
	pageCount := 0
	enqueuedCount := 0
	syncedCount := 0
	errorCount := 0

	streamName := "pilot_sync_queue"

	// If using queue, ensure consumer group exists
	if j.useQueue {
		if err := j.redisQueue.CreateConsumerGroup(ctx, streamName, "pilot-workers"); err != nil {
			log.Printf("[PilotSyncJob] VA %s: Warning - failed to create consumer group: %v", vaName, err)
			// Continue anyway - group might already exist
		}
	}

	// Convert schema to queue format for serialization
	queueSchema := convertSchemaToQueueFormat(vaPilotSchema)

	for {
		pageCount++
		filters := &providers.SyncFilters{
			Offset:        offset,
			Limit:         100, // Batch size
			ModifiedSince: lastModified,
		}

		recordSet, err := j.airtableProvider.FetchRecords(ctx, vaPilotSchema, filters)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch records (page %d): %w", pageCount, err)
		}

		log.Printf("[PilotSyncJob] VA %s: Fetched page %d with %d records", vaName, pageCount, len(recordSet.Records))

		if j.useQueue {
			// Queue-based processing: Enqueue batch to Redis
			var queueItems []*queue.PilotQueueItem
			for _, record := range recordSet.Records {
				queueItems = append(queueItems, &queue.PilotQueueItem{
					VAID:             vaID,
					AirtableRecordID: record.ID,
					Fields:           record.Fields,
					Schema:           queueSchema,
				})
				log.Printf("[PilotSyncJob] VA %s: Enqueueing pilot - ATID: %s", vaName, record.ID)
			}

			if err := j.redisQueue.EnqueuePilotBatch(ctx, streamName, queueItems); err != nil {
				log.Printf("[PilotSyncJob] VA %s: Error enqueuing batch: %v", vaName, err)
				errorCount += len(queueItems)
			} else {
				enqueuedCount += len(queueItems)
				// Track metrics
				if j.metrics != nil {
					j.metrics.QueueEnqueuedTotal.WithLabelValues(streamName, "pilot").Add(float64(len(queueItems)))
					j.metrics.SyncJobRecordsProcessed.WithLabelValues("pilot_sync_job", "airtable", "pilot", vaID, "enqueued").Add(float64(len(queueItems)))
				}
			}
		} else {
			// Direct processing: Process immediately (streaming)
			for _, record := range recordSet.Records {
				if err := j.upsertPilot(ctx, vaID, record.ID, record.Fields, vaPilotSchema); err != nil {
					log.Printf("[PilotSyncJob] VA %s: Error upserting record: %v", vaName, err)
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
		log.Printf("[PilotSyncJob] VA %s: Completed in %s. Enqueued: %d, Errors: %d",
			vaName, time.Since(start).Truncate(time.Millisecond), enqueuedCount, errorCount)
		log.Printf("[PilotSyncJob] VA %s: Queue: %s - Workers will process items asynchronously", vaName, streamName)
		// Record sync history after successful enqueue
		if enqueuedCount > 0 {
			if err := j.syncRepo.RecordSync(ctx, vaID, platformVA.SyncEventPilotsAT); err != nil {
				log.Printf("[PilotSyncJob] VA %s: Warning - failed to record sync history: %v", vaName, err)
			}
		}
	} else {
		log.Printf("[PilotSyncJob] VA %s: Completed in %s. Synced: %d, Errors: %d",
			vaName, time.Since(start).Truncate(time.Millisecond), syncedCount, errorCount)
		// Record sync history after direct processing
		if syncedCount > 0 {
			if err := j.syncRepo.RecordSync(ctx, vaID, platformVA.SyncEventPilotsAT); err != nil {
				log.Printf("[PilotSyncJob] VA %s: Warning - failed to record sync history: %v", vaName, err)
			}
		}
	}

	// Return count (enqueued if using queue, synced if direct)
	if j.useQueue {
		return enqueuedCount, nil
	}
	return syncedCount, nil
}

// SyncVACareerModePilots syncs career mode pilots for a specific VA
func (j *SyncJob) SyncVACareerModePilots(ctx context.Context, vaID string) (int, error) {
	start := time.Now()
	log.Printf("[PilotSyncJob] Syncing career mode pilots for VA %s", vaID)

	// Get credentials config for this VA
	credentialsConfig, err := j.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "credentials")
	if err != nil {
		return 0, fmt.Errorf("failed to get credentials config: %w", err)
	}

	if credentialsConfig == nil {
		log.Printf("[PilotSyncJob] No active credentials config found for VA %s", vaID)
		return 0, nil
	}

	// Parse credentials config data
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

	// Get career mode schema config
	careerModeConfig, err := j.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "career_mode")
	if err != nil {
		return 0, fmt.Errorf("failed to get career mode schema config: %w", err)
	}

	if careerModeConfig == nil {
		log.Printf("[PilotSyncJob] No career mode schema configured for VA %s", vaID)
		return 0, nil
	}

	// Parse career mode schema config data
	var careerModeSchema dtos.EntitySchema
	bytes, err := json.Marshal(careerModeConfig.ConfigData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal career mode config data: %w", err)
	}

	if err := json.Unmarshal(bytes, &careerModeSchema); err != nil {
		return 0, fmt.Errorf("failed to parse career mode schema: %w", err)
	}

	// Set entity_type if not set
	if careerModeSchema.EntityType == "" {
		careerModeSchema.EntityType = "career_mode"
	}

	// Check if enabled
	if !careerModeSchema.Enabled {
		log.Printf("[PilotSyncJob] Career mode schema is disabled for VA %s", vaID)
		return 0, nil
	}

	// Validate that callsign field is configured
	hasCallsignField := false
	for _, field := range careerModeSchema.Fields {
		if field.InternalName == "callsign" {
			hasCallsignField = true
			break
		}
	}
	if !hasCallsignField {
		log.Printf("[PilotSyncJob] Career mode schema for VA %s is missing required 'callsign' field mapping", vaID)
		return 0, fmt.Errorf("career mode schema missing required 'callsign' field mapping")
	}

	// Get VA name for logging
	var vaName string
	j.db.WithContext(ctx).
		Table("virtual_airlines").
		Where("id = ?", vaID).
		Pluck("name", &vaName)

	log.Printf("[PilotSyncJob] VA: %s (%s), Career Mode Table: %s", vaName, vaID, careerModeSchema.TableName)

	// Get last sync timestamp for incremental sync (using career mode specific event)
	lastModified, err := j.getLastSyncTimestampForEvent(ctx, vaID, platformVA.SyncEventCareerModePilotsAT)
	if err != nil {
		log.Printf("[PilotSyncJob] VA %s: Error getting last career mode sync timestamp: %v. Doing full sync.", vaName, err)
		lastModified = nil
	}

	if lastModified != nil {
		log.Printf("[PilotSyncJob] VA %s: Incremental career mode sync from %s", vaName, *lastModified)
	} else {
		log.Printf("[PilotSyncJob] VA %s: Full career mode sync (no previous sync timestamp)", vaName)
	}

	// Check if schema has last_modified_field configured
	if lastModified != nil && careerModeSchema.LastModifiedField == "" {
		log.Printf("[PilotSyncJob] VA %s: Warning - no last_modified_field configured in career mode schema, cannot filter by date. Doing full sync.", vaName)
		lastModified = nil
	}

	// Convert dtos.EntitySchema to platformVA.EntitySchema
	vaCareerModeSchema := convertDTOsEntitySchema(&careerModeSchema)

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

	// Fetch career mode pilots with pagination
	offset := ""
	pageCount := 0
	enqueuedCount := 0
	syncedCount := 0
	errorCount := 0

	streamName := "pilot_sync_queue"

	// If using queue, ensure consumer group exists
	if j.useQueue {
		if err := j.redisQueue.CreateConsumerGroup(ctx, streamName, "pilot-workers"); err != nil {
			log.Printf("[PilotSyncJob] VA %s: Warning - failed to create consumer group: %v", vaName, err)
		}
	}

	// Convert schema to queue format for serialization
	queueSchema := convertSchemaToQueueFormat(vaCareerModeSchema)

	for {
		pageCount++
		filters := &providers.SyncFilters{
			Offset:        offset,
			Limit:         100, // Batch size
			ModifiedSince: lastModified,
		}

		recordSet, err := j.airtableProvider.FetchRecords(ctx, vaCareerModeSchema, filters)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch career mode records (page %d): %w", pageCount, err)
		}

		log.Printf("[PilotSyncJob] VA %s: Fetched career mode page %d with %d records", vaName, pageCount, len(recordSet.Records))

		if j.useQueue {
			// Queue-based processing: Enqueue batch to Redis
			var queueItems []*queue.PilotQueueItem
			for _, record := range recordSet.Records {
				queueItems = append(queueItems, &queue.PilotQueueItem{
					VAID:             vaID,
					AirtableRecordID: record.ID,
					Fields:           record.Fields,
					Schema:           queueSchema,
				})
				log.Printf("[PilotSyncJob] VA %s: Enqueueing career mode pilot - ATID: %s", vaName, record.ID)
			}

			if err := j.redisQueue.EnqueuePilotBatch(ctx, streamName, queueItems); err != nil {
				log.Printf("[PilotSyncJob] VA %s: Error enqueuing career mode batch: %v", vaName, err)
				errorCount += len(queueItems)
			} else {
				enqueuedCount += len(queueItems)
				// Track metrics
				if j.metrics != nil {
					j.metrics.QueueEnqueuedTotal.WithLabelValues(streamName, "career_mode_pilot").Add(float64(len(queueItems)))
					j.metrics.SyncJobRecordsProcessed.WithLabelValues("pilot_sync_job", "airtable", "career_mode", vaID, "enqueued").Add(float64(len(queueItems)))
				}
			}
		} else {
			// Direct processing: Process immediately
			for _, record := range recordSet.Records {
				if err := j.upsertPilot(ctx, vaID, record.ID, record.Fields, vaCareerModeSchema); err != nil {
					log.Printf("[PilotSyncJob] VA %s: Error upserting career mode record: %v", vaName, err)
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
		log.Printf("[PilotSyncJob] VA %s: Completed career mode sync in %s. Enqueued: %d, Errors: %d",
			vaName, time.Since(start).Truncate(time.Millisecond), enqueuedCount, errorCount)
		log.Printf("[PilotSyncJob] VA %s: Queue: %s - Workers will process items asynchronously", vaName, streamName)
		// Record sync history after successful enqueue (using career mode specific event)
		if enqueuedCount > 0 {
			if err := j.syncRepo.RecordSync(ctx, vaID, platformVA.SyncEventCareerModePilotsAT); err != nil {
				log.Printf("[PilotSyncJob] VA %s: Warning - failed to record career mode sync history: %v", vaName, err)
			}
		}
	} else {
		log.Printf("[PilotSyncJob] VA %s: Completed career mode sync in %s. Synced: %d, Errors: %d",
			vaName, time.Since(start).Truncate(time.Millisecond), syncedCount, errorCount)
		// Record sync history after direct processing (using career mode specific event)
		if syncedCount > 0 {
			if err := j.syncRepo.RecordSync(ctx, vaID, platformVA.SyncEventCareerModePilotsAT); err != nil {
				log.Printf("[PilotSyncJob] VA %s: Warning - failed to record career mode sync history: %v", vaName, err)
			}
		}
	}

	// Return count (enqueued if using queue, synced if direct)
	if j.useQueue {
		return enqueuedCount, nil
	}
	return syncedCount, nil
}

// upsertPilot updates or creates a pilot record in va_user_roles and pilot_at_synced
func (j *SyncJob) upsertPilot(ctx context.Context, vaID string, airtableRecordID string, record map[string]interface{}, schema *platformVA.EntitySchema) error {
	// Extract callsign from record using field mapping
	callsignField := schema.GetFieldMapping("callsign")
	if callsignField == nil {
		return fmt.Errorf("callsign field not configured in schema")
	}

	rawCallsign, ok := record[callsignField.AirtableName]
	if !ok {
		return fmt.Errorf("callsign field '%s' not found in record", callsignField.AirtableName)
	}

	callsign, ok := rawCallsign.(string)
	if !ok {
		return fmt.Errorf("callsign is not a string: %v", rawCallsign)
	}

	// Clean and validate callsign
	callsign = strings.TrimSpace(callsign)
	if callsign == "" {
		return fmt.Errorf("callsign is empty")
	}

	// Upsert into pilot_at_synced table first (keeps our database in sync with Airtable)
	// Determine pilot type based on entity type
	pilotType := PilotTypeRegular
	if schema.EntityType == "career_mode" {
		pilotType = PilotTypeCareerMode
	}
	log.Printf("[PilotSyncJob] Upserting pilot - ATID: %s, Callsign: %s, ServerID: %s, Type: %s", airtableRecordID, callsign, vaID, pilotType)
	pilotATSynced := &PilotATSyncedGORM{
		ATID:       airtableRecordID,
		Callsign:   callsign,
		Registered: false, // Will be updated to true if found in va_user_roles
		ServerID:   vaID,
		PilotType:  pilotType,
	}

	// Find user by callsign in va_user_roles for this VA
	// If found, update airtable_pilot_id and mark as registered

	var existingRole struct {
		ID       string  `gorm:"column:id"`
		UserID   string  `gorm:"column:user_id"`
		VAID     string  `gorm:"column:va_id"`
		Callsign *string `gorm:"column:callsign"`
	}

	err := j.db.WithContext(ctx).
		Table("va_user_roles").
		Where("va_id = ? AND LOWER(callsign) = LOWER(?)", vaID, callsign).
		Select("id, user_id, va_id, callsign").
		First(&existingRole).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Pilot not found in database - this is expected for pilots who haven't registered yet
			log.Printf("[PilotSyncJob] Callsign %s not found in VA %s - pilot may not be registered yet", callsign, vaID)
			// Still upsert into pilot_at_synced with registered=false
			if err := j.pilotRepo.Upsert(ctx, pilotATSynced); err != nil {
				log.Printf("[PilotSyncJob] ERROR: Failed to upsert pilot %s (ATID: %s) into pilot_at_synced: %v", callsign, airtableRecordID, err)
				return fmt.Errorf("failed to upsert pilot into pilot_at_synced: %w", err)
			}
			log.Printf("[PilotSyncJob] Successfully upserted pilot %s (ATID: %s) into pilot_at_synced with registered=false", callsign, airtableRecordID)
			return nil
		}
		return fmt.Errorf("failed to query existing role: %w", err)
	}

	// User found - mark as registered
	pilotATSynced.Registered = true

	// Upsert into pilot_at_synced
	if err := j.pilotRepo.Upsert(ctx, pilotATSynced); err != nil {
		log.Printf("[PilotSyncJob] ERROR: Failed to upsert pilot %s (ATID: %s) into pilot_at_synced: %v", callsign, airtableRecordID, err)
		return fmt.Errorf("failed to upsert pilot into pilot_at_synced: %w", err)
	}
	log.Printf("[PilotSyncJob] Successfully upserted pilot %s (ATID: %s) into pilot_at_synced with registered=true", callsign, airtableRecordID)

	// Update the appropriate pilot ID field based on pilot type
	updateFields := map[string]interface{}{
		"updated_at": time.Now(),
	}
	if pilotType == PilotTypeCareerMode {
		updateFields["career_mode_pilot_id"] = airtableRecordID
	} else {
		updateFields["airtable_pilot_id"] = airtableRecordID
	}

	err = j.db.WithContext(ctx).
		Table("va_user_roles").
		Where("id = ?", existingRole.ID).
		Updates(updateFields).Error

	if err != nil {
		if pilotType == PilotTypeCareerMode {
			return fmt.Errorf("failed to update career_mode_pilot_id for callsign %s: %w", callsign, err)
		}
		return fmt.Errorf("failed to update airtable_pilot_id for callsign %s: %w", callsign, err)
	}

	if pilotType == PilotTypeCareerMode {
		log.Printf("[PilotSyncJob] Updated career_mode_pilot_id for callsign %s (record: %s)", callsign, airtableRecordID)
	} else {
		log.Printf("[PilotSyncJob] Updated airtable_pilot_id for callsign %s (record: %s)", callsign, airtableRecordID)
		// Try to link career mode pilot if career mode schema exists (only for regular pilots)
		j.linkCareerModePilot(ctx, vaID, callsign, existingRole.ID)
	}

	// Invalidate cache for this pilot's stats
	cacheKey := fmt.Sprintf("pilot_stats:%s:%s", vaID, airtableRecordID)
	j.cache.Delete(cacheKey)

	return nil
}

// linkCareerModePilot attempts to link a career mode pilot record for the given callsign
func (j *SyncJob) linkCareerModePilot(ctx context.Context, vaID string, callsign string, userRoleID string) {
	// Get career mode schema config
	careerModeConfig, err := j.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "career_mode")
	if err != nil {
		log.Printf("[PilotSyncJob] Error getting career mode config for VA %s: %v", vaID, err)
		return
	}

	if careerModeConfig == nil {
		// Career mode not configured - this is fine, just return
		return
	}

	// Parse career mode schema config
	var careerModeSchema dtos.EntitySchema
	bytes, err := json.Marshal(careerModeConfig.ConfigData)
	if err != nil {
		log.Printf("[PilotSyncJob] Error marshaling career mode config: %v", err)
		return
	}

	if err := json.Unmarshal(bytes, &careerModeSchema); err != nil {
		log.Printf("[PilotSyncJob] Error parsing career mode schema: %v", err)
		return
	}

	// Set entity_type if not set
	if careerModeSchema.EntityType == "" {
		careerModeSchema.EntityType = "career_mode"
	}

	// Check if enabled
	if !careerModeSchema.Enabled {
		log.Printf("[PilotSyncJob] Career mode schema is disabled for VA %s", vaID)
		return
	}

	// Get credentials config (reuse from regular pilot sync context)
	credentialsConfig, err := j.configRepo.GetActiveConfigByType(ctx, vaID, "airtable", "credentials")
	if err != nil || credentialsConfig == nil {
		log.Printf("[PilotSyncJob] Error getting credentials for career mode linking: %v", err)
		return
	}

	var credsData struct {
		APIKey       string            `json:"api_key"`
		BaseID       string            `json:"base_id"`
		SyncSettings dtos.SyncSettings `json:"sync_settings"`
	}
	credsBytes, err := json.Marshal(credentialsConfig.ConfigData)
	if err != nil {
		log.Printf("[PilotSyncJob] Error marshaling credentials: %v", err)
		return
	}
	if err := json.Unmarshal(credsBytes, &credsData); err != nil {
		log.Printf("[PilotSyncJob] Error parsing credentials: %v", err)
		return
	}

	// Build credentials
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

	// Get callsign field name from schema
	var callsignFieldName string
	for _, field := range careerModeSchema.Fields {
		if field.InternalName == "callsign" {
			callsignFieldName = field.AirtableName
			break
		}
	}

	if callsignFieldName == "" {
		log.Printf("[PilotSyncJob] Career mode schema missing callsign field mapping")
		return
	}

	// Set credentials in context
	ctx = context.WithValue(ctx, "provider_credentials", creds)

	// Convert schema
	vaCareerModeSchema := convertDTOsEntitySchema(&careerModeSchema)

	// Build filter formula to find career mode record by callsign
	filterFormula := fmt.Sprintf("{%s} = '%s'", callsignFieldName, callsign)
	filters := &providers.SyncFilters{
		FilterFormula: filterFormula,
		Limit:         1,
	}

	// Fetch career mode record
	recordSet, err := j.airtableProvider.FetchRecords(ctx, vaCareerModeSchema, filters)
	if err != nil {
		log.Printf("[PilotSyncJob] Error fetching career mode record for callsign %s: %v", callsign, err)
		return
	}

	if len(recordSet.Records) == 0 {
		log.Printf("[PilotSyncJob] No career mode record found for callsign %s", callsign)
		return
	}

	careerModeRecordID := recordSet.Records[0].ID

	// Update career_mode_pilot_id
	err = j.pilotRepo.UpdateUserCareerModePilotID(ctx, userRoleID, careerModeRecordID)
	if err != nil {
		log.Printf("[PilotSyncJob] Error updating career_mode_pilot_id for callsign %s: %v", callsign, err)
		return
	}

	log.Printf("[PilotSyncJob] Updated career_mode_pilot_id for callsign %s (record: %s)", callsign, careerModeRecordID)
}

// getLastSyncTimestamp gets the most recent sync timestamp for this VA from sync history
// This is used for incremental syncing - only fetch records modified after this time
// DEPRECATED: Use getLastSyncTimestampForEvent instead
func (j *SyncJob) getLastSyncTimestamp(ctx context.Context, vaID string) (*string, error) {
	return j.getLastSyncTimestampForEvent(ctx, vaID, platformVA.SyncEventPilotsAT)
}

// getLastSyncTimestampForEvent gets the most recent sync timestamp for this VA and specific event from sync history
// This is used for incremental syncing - only fetch records modified after this time
func (j *SyncJob) getLastSyncTimestampForEvent(ctx context.Context, vaID string, event string) (*string, error) {
	lastSyncTime, err := j.syncRepo.GetLastSyncTimeForVAAndEvent(ctx, vaID, event)

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

// convertSchemaToQueueFormat converts platformVA.EntitySchema to queue.PilotSchema for serialization
func convertSchemaToQueueFormat(schema *platformVA.EntitySchema) *queue.PilotSchema {
	if schema == nil {
		return nil
	}
	fields := make([]queue.FieldMapping, len(schema.Fields))
	for i, f := range schema.Fields {
		fields[i] = queue.FieldMapping{
			InternalName: f.InternalName,
			AirtableName: f.AirtableName,
			DataType:     f.DataType,
		}
	}
	return &queue.PilotSchema{
		EntityType:        schema.EntityType,
		TableName:         schema.TableName,
		LastModifiedField: schema.LastModifiedField,
		Fields:            fields,
	}
}

// convertQueueSchemaToVA converts queue.PilotSchema back to platformVA.EntitySchema
func convertQueueSchemaToVA(queueSchema *queue.PilotSchema) *platformVA.EntitySchema {
	if queueSchema == nil {
		return nil
	}
	fields := make([]platformVA.FieldMapping, len(queueSchema.Fields))
	for i, f := range queueSchema.Fields {
		fields[i] = platformVA.FieldMapping{
			InternalName: f.InternalName,
			AirtableName: f.AirtableName,
			DataType:     f.DataType,
		}
	}
	return &platformVA.EntitySchema{
		EntityType:        queueSchema.EntityType,
		TableName:         queueSchema.TableName,
		Enabled:           true, // Assume enabled if in queue
		LastModifiedField: queueSchema.LastModifiedField,
		Fields:            fields,
	}
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

// getCredentialsFromConfig extracts credentials from old config structure
func getCredentialsFromConfig(configData *dtos.ProviderConfigData) (*platformVA.ProviderCredentials, error) {
	if configData == nil {
		return nil, fmt.Errorf("config data is nil")
	}
	return &platformVA.ProviderCredentials{
		APIKey: configData.Credentials.APIKey,
		BaseID: configData.Credentials.BaseID,
		SyncSettings: platformVA.SyncSettings{
			BatchSize:          configData.SyncSettings.BatchSize,
			RateLimitPerSecond: configData.SyncSettings.RateLimitPerSecond,
			RetryAttempts:      configData.SyncSettings.RetryAttempts,
			TimeoutSeconds:     configData.SyncSettings.TimeoutSeconds,
		},
	}, nil
}
