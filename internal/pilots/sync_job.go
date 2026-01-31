package pilots

import (
	"context"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
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
	syncHistoryRepo  *repositories.VASyncHistoryRepo
	pilotRepo        *Repository
	linkingJob       *LinkingJob
	airtableProvider *providers.AirtableProvider
}

// NewSyncJob creates a new pilot sync job instance
func NewSyncJob(
	db *gorm.DB,
	cache cache.CacheInterface,
	configRepo *repositories.DataProviderConfigRepo,
	syncHistoryRepo *repositories.VASyncHistoryRepo,
	pilotRepo *Repository,
	vaConfigService *common.VAConfigService,
) *SyncJob {
	return &SyncJob{
		db:               db,
		cache:            cache,
		configRepo:       configRepo,
		syncHistoryRepo:  syncHistoryRepo,
		pilotRepo:        pilotRepo,
		linkingJob:       NewLinkingJob(db, vaConfigService, pilotRepo),
		airtableProvider: providers.NewAirtableProvider(cache),
	}
}

// Run executes the pilot sync job for all active VAs with Airtable enabled
func (j *SyncJob) Run(ctx context.Context) error {
	if 1 == 1 {
		return nil
	}
	start := time.Now()
	log.Printf("[PilotSyncJob] Starting pilot sync at %s", start.Format(time.RFC3339))

	// Get all VAs that have active Airtable configs
	var vaIDs []string
	err := j.db.WithContext(ctx).
		Table("va_data_provider_configs").
		Where("provider_type = ? AND is_active = ?", "airtable", true).
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
	totalSynced := 0
	for _, vaID := range vaIDs {
		synced, err := j.SyncVAPilots(ctx, vaID)
		if err != nil {
			log.Printf("[PilotSyncJob] Error syncing pilots for VA %s: %v", vaID, err)
			// Continue with other VAs even if one fails
			continue
		}
		totalSynced += synced
	}

	log.Printf("[PilotSyncJob] Completed pilot sync in %s. Total pilots synced: %d",
		time.Since(start).Truncate(time.Millisecond), totalSynced)

	return nil
}

// SyncVAPilots syncs pilots for a specific VA (exported for manual triggering)
func (j *SyncJob) SyncVAPilots(ctx context.Context, vaID string) (int, error) {
	start := time.Now()
	log.Printf("[PilotSyncJob] Syncing pilots for VA %s", vaID)

	// Get active config for this VA
	config, err := j.configRepo.GetActiveConfig(ctx, vaID, "airtable")
	if err != nil {
		return 0, fmt.Errorf("failed to get active config: %w", err)
	}

	if config == nil {
		log.Printf("[PilotSyncJob] No active config found for VA %s", vaID)
		return 0, nil
	}

	// Parse config data
	configData, err := repositories.ParseConfigData(config.ConfigData)
	if err != nil {
		return 0, fmt.Errorf("failed to parse config data: %w", err)
	}

	// Get pilot schema
	pilotSchema := configData.GetSchemaByType("pilot")
	if pilotSchema == nil {
		log.Printf("[PilotSyncJob] No pilot schema configured for VA %s", vaID)
		return 0, nil
	}

	if !pilotSchema.Enabled {
		log.Printf("[PilotSyncJob] Pilot schema is disabled for VA %s", vaID)
		return 0, nil
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

	// Set config in context for provider
	ctx = context.WithValue(ctx, "provider_config", configData)

	// Fetch pilots with pagination and optional modified-since filter
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

		recordSet, err := j.airtableProvider.FetchRecords(ctx, pilotSchema, filters)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch records (page %d): %w", pageCount, err)
		}

		log.Printf("[PilotSyncJob] VA %s: Fetched page %d with %d records", vaName, pageCount, len(recordSet.Records))

		allRecords = append(allRecords, recordSet.Records...)

		if !recordSet.HasMore {
			break
		}
		offset = recordSet.Offset
	}

	log.Printf("[PilotSyncJob] VA %s: Total records fetched: %d", vaName, len(allRecords))

	// Process and upsert pilots
	syncedCount := 0
	errorCount := 0

	for i, record := range allRecords {
		if err := j.upsertPilot(ctx, vaID, record.ID, record.Fields, pilotSchema); err != nil {
			log.Printf("[PilotSyncJob] VA %s: Error upserting record %d: %v", vaName, i+1, err)
			errorCount++
			continue
		}
		syncedCount++
	}

	log.Printf("[PilotSyncJob] VA %s: Completed in %s. Synced: %d, Errors: %d",
		vaName, time.Since(start).Truncate(time.Millisecond), syncedCount, errorCount)

	// Record successful sync in sync history
	if err := j.syncHistoryRepo.RecordSync(ctx, vaID, constants.SyncEventPilotsAT); err != nil {
		log.Printf("[PilotSyncJob] VA %s: Warning - failed to record sync history: %v", vaName, err)
		// Don't fail the sync operation if history recording fails
	}

	return syncedCount, nil
}

// upsertPilot updates or creates a pilot record in va_user_roles and pilot_at_synced
func (j *SyncJob) upsertPilot(ctx context.Context, vaID string, airtableRecordID string, record map[string]interface{}, schema *dtos.EntitySchema) error {
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
	pilotATSynced := &PilotATSyncedGORM{
		ATID:       airtableRecordID,
		Callsign:   callsign,
		Registered: false, // Will be updated to true if found in va_user_roles
		ServerID:   vaID,
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
				log.Printf("[PilotSyncJob] Warning: failed to upsert into pilot_at_synced: %v", err)
			}
			return nil
		}
		return fmt.Errorf("failed to query existing role: %w", err)
	}

	// User found - mark as registered
	pilotATSynced.Registered = true

	// Upsert into pilot_at_synced
	if err := j.pilotRepo.Upsert(ctx, pilotATSynced); err != nil {
		log.Printf("[PilotSyncJob] Warning: failed to upsert into pilot_at_synced: %v", err)
	}

	// Update the airtable_pilot_id and updated_at timestamp in va_user_roles
	err = j.db.WithContext(ctx).
		Table("va_user_roles").
		Where("id = ?", existingRole.ID).
		Updates(map[string]interface{}{
			"airtable_pilot_id": airtableRecordID,
			"updated_at":        time.Now(),
		}).Error

	if err != nil {
		return fmt.Errorf("failed to update airtable_pilot_id for callsign %s: %w", callsign, err)
	}

	log.Printf("[PilotSyncJob] Updated airtable_pilot_id for callsign %s (record: %s)", callsign, airtableRecordID)

	// Invalidate cache for this pilot's stats
	cacheKey := fmt.Sprintf("pilot_stats:%s:%s", vaID, airtableRecordID)
	j.cache.Delete(cacheKey)

	return nil
}

// getLastSyncTimestamp gets the most recent sync timestamp for this VA from sync history
// This is used for incremental syncing - only fetch records modified after this time
func (j *SyncJob) getLastSyncTimestamp(ctx context.Context, vaID string) (*string, error) {
	lastSyncTime, err := j.syncHistoryRepo.GetLastSyncTimeForEvent(ctx, constants.SyncEventPilotsAT)

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

// shouldRunInitialSync checks if enough time has passed since the last sync
// Returns true if the last sync was more than 4 hours ago or if no sync has occurred
func (j *SyncJob) shouldRunInitialSync(ctx context.Context) bool {
	lastSyncTime, err := j.syncHistoryRepo.GetLastSyncTimeForEvent(ctx, constants.SyncEventPilotsAT)

	if err != nil {
		log.Printf("[PilotSyncJob] Error checking last sync time: %v. Running sync anyway.", err)
		return true
	}

	// If no sync history found, run the sync
	if lastSyncTime == nil {
		log.Printf("[PilotSyncJob] No previous sync found. Running initial sync.")
		return true
	}

	// Check if more than 4 hours have passed
	timeSinceLastSync := time.Since(*lastSyncTime)
	fourHours := 4 * time.Hour

	if timeSinceLastSync > fourHours {
		log.Printf("[PilotSyncJob] Last sync was %s ago (> 4 hours). Running sync.", timeSinceLastSync.Truncate(time.Minute))
		return true
	}

	log.Printf("[PilotSyncJob] Last sync was %s ago (< 4 hours). Skipping initial sync.", timeSinceLastSync.Truncate(time.Minute))
	return false
}

// RunScheduled runs the pilot sync job on a schedule (e.g., every hour)
func (j *SyncJob) RunScheduled(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start only if last sync was more than 4 hours ago
	if j.shouldRunInitialSync(ctx) {
		if err := j.Run(ctx); err != nil {
			log.Printf("[PilotSyncJob] Error in initial run: %v", err)
		}
	}

	// Always run pilot linking after sync (even if sync was skipped)
	if err := j.linkingJob.Run(ctx); err != nil {
		log.Printf("[PilotLinkingJob] Error in initial linking: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := j.Run(ctx); err != nil {
				log.Printf("[PilotSyncJob] Error in scheduled run: %v", err)
			}
			// Run linking after each scheduled sync
			if err := j.linkingJob.Run(ctx); err != nil {
				log.Printf("[PilotLinkingJob] Error in scheduled linking: %v", err)
			}
		case <-ctx.Done():
			log.Printf("[PilotSyncJob] Shutting down scheduled sync")
			return
		}
	}
}
