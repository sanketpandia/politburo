package pireps

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/queue"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

// QueueWorker processes PIREPs from Redis queue
type QueueWorker struct {
	db         *gorm.DB
	vaRepo     *platformVA.Repository
	pirepRepo  *Repository
	redisQueue *queue.RedisQueueService
	metrics    *metrics.MetricsRegistry
	workerID   string
}

// NewQueueWorker creates a new PIREP queue worker
func NewQueueWorker(
	db *gorm.DB,
	vaRepo *platformVA.Repository,
	pirepRepo *Repository,
	redisQueue *queue.RedisQueueService,
	metricsReg *metrics.MetricsRegistry,
) *QueueWorker {
	return &QueueWorker{
		db:         db,
		vaRepo:     vaRepo,
		pirepRepo:  pirepRepo,
		redisQueue: redisQueue,
		metrics:    metricsReg,
		workerID:   "pirep-worker",
	}
}

// Start begins processing PIREPs from all VA queues
// Spawns multiple goroutines to handle different VA queues concurrently
func (w *QueueWorker) Start(ctx context.Context, numWorkers int) error {
	logging.Info("PIREP queue worker starting", "num_workers", numWorkers, "worker_id", w.workerID)

	var wg sync.WaitGroup

	// Get all active VAs with Airtable configs
	var vaIDs []string
	err := w.db.WithContext(ctx).
		Table("va_data_provider_configs").
		Where("provider_type = ? AND is_active = ?", "airtable", true).
		Distinct("va_id").
		Pluck("va_id", &vaIDs).Error

	if err != nil {
		return fmt.Errorf("failed to fetch active VAs: %w", err)
	}

	if len(vaIDs) == 0 {
		logging.Info("PIREP queue worker: no VAs with active Airtable configs")
		return nil
	}

	logging.Info("PIREP queue worker: VAs to process", "count", len(vaIDs))

	// Start workers for each VA
	for _, vaID := range vaIDs {
		streamName := fmt.Sprintf("pirep:sync:%s", vaID)

		// Ensure consumer group exists
		if err := w.redisQueue.CreateConsumerGroup(ctx, streamName, "pirep-workers"); err != nil {
			logging.Warn("PIREP queue worker: failed to create consumer group", "va_id", vaID, "error", err)
		}

		// Start multiple workers for this VA queue
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			workerName := fmt.Sprintf("%s-va-%s-worker-%d", w.workerID, vaID[:8], i)

			go func(vaID, workerName, streamName string) {
				defer wg.Done()
				w.processQueue(ctx, vaID, streamName, workerName)
			}(vaID, workerName, streamName)
		}
	}

	// Start a goroutine to periodically claim stale messages
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.claimStaleMessages(ctx, vaIDs)
	}()

	wg.Wait()
	logging.Info("PIREP queue worker: all workers stopped")
	return nil
}

// processQueue continuously processes PIREPs from a specific VA queue
func (w *QueueWorker) processQueue(ctx context.Context, vaID, streamName, workerName string) {
	logging.Info("PIREP worker started", "worker", workerName, "stream", streamName)

	processedCount := 0
	errorCount := 0

	for {
		select {
		case <-ctx.Done():
			logging.Info("PIREP worker shutting down", "worker", workerName, "processed", processedCount, "errors", errorCount)
			return
		default:
			// Dequeue next PIREP (blocks for up to 5 seconds)
			item, messageID, err := w.redisQueue.DequeuePirep(ctx, streamName, "pirep-workers", workerName, 5*time.Second)
			if err != nil {
				logging.Error("PIREP worker dequeue error", "worker", workerName, "error", err)
				time.Sleep(1 * time.Second) // Back off on error
				continue
			}

			if item == nil {
				// No messages available (timeout), continue loop
				continue
			}

			if w.metrics != nil {
				w.metrics.QueueDequeuedTotal.WithLabelValues("pirep_queue", "pirep").Inc()
			}

			// Process the PIREP
			itemStart := time.Now()
			processErr := w.processPirep(ctx, item)
			elapsed := time.Since(itemStart)

			if w.metrics != nil {
				w.metrics.QueueProcessingDuration.WithLabelValues("pirep_queue", "pirep").Observe(elapsed.Seconds())
			}

			if processErr != nil {
				logging.Error("PIREP worker process error", "worker", workerName, "record_id", item.AirtableRecordID, "va_id", item.VATID, "error", processErr)
				errorCount++
				if w.metrics != nil {
					w.metrics.QueueErrorsTotal.WithLabelValues("pirep_queue", "pirep", "transient").Inc()
				}
				// Note: We still acknowledge to avoid reprocessing indefinitely
			} else {
				processedCount++
				if w.metrics != nil {
					w.metrics.QueueAcknowledgedTotal.WithLabelValues("pirep_queue", "pirep").Inc()
				}
				if processedCount%100 == 0 {
					logging.Info("PIREP worker progress", "worker", workerName, "processed", processedCount, "errors", errorCount)
				}
			}

			// Acknowledge message
			if err := w.redisQueue.AckPirep(ctx, streamName, "pirep-workers", messageID); err != nil {
				logging.Error("PIREP worker ack error", "worker", workerName, "message_id", messageID, "error", err)
			}
		}
	}
}

// processPirep handles the actual PIREP upsert logic
func (w *QueueWorker) processPirep(ctx context.Context, item *queue.PirepQueueItem) error {
	// Get PIREP schema using platform VA repository
	pirepSchema, err := w.vaRepo.GetAirtableSchema(ctx, item.VATID, "pirep")
	if err != nil {
		return fmt.Errorf("failed to get schema: %w", err)
	}

	if pirepSchema == nil {
		return fmt.Errorf("no pirep schema found for VA %s", item.VATID)
	}

	// Convert platform schema to EntitySchema format for upsert logic
	entitySchema := pirepSchema.ToEntitySchema("pirep")

	// Extract and upsert PIREP (reuse sync job logic)
	return w.upsertPirep(ctx, item.VATID, item.AirtableRecordID, item.Fields, item.CreatedTime, entitySchema)
}

// upsertPirep inserts or updates a PIREP record (same logic as the sync job)
func (w *QueueWorker) upsertPirep(ctx context.Context, vaID string, airtableRecordID string, record map[string]interface{}, createdTime string, schema *platformVA.EntitySchema) error {
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
		err := w.db.WithContext(ctx).
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
		err := w.db.WithContext(ctx).
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
	// Use callsign from pilot_at_synced if available, otherwise keep the original
	finalPilotCallsign := pilotCallsignFromSync
	if finalPilotCallsign == "" {
		// Fallback: keep empty or any value that was extracted from Airtable
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
	if err := w.pirepRepo.Upsert(ctx, pirepATSynced); err != nil {
		return fmt.Errorf("failed to upsert PIREP: %w", err)
	}

	return nil
}

// claimStaleMessages periodically claims messages that have been idle too long
func (w *QueueWorker) claimStaleMessages(ctx context.Context, vaIDs []string) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, vaID := range vaIDs {
				streamName := fmt.Sprintf("pirep:sync:%s", vaID)
				claimerName := fmt.Sprintf("%s-claimer", w.workerID)

				items, messageIDs, err := w.redisQueue.ClaimStalePireps(ctx, streamName, "pirep-workers", claimerName, 5*time.Minute)
				if err != nil {
					logging.Error("PIREP worker: failed to claim stale messages", "va_id", vaID, "error", err)
					continue
				}

				if len(items) > 0 {
					logging.Info("PIREP worker: claimed stale messages", "va_id", vaID, "count", len(items))

					// Process claimed items
					for i, item := range items {
						if err := w.processPirep(ctx, item); err != nil {
							logging.Error("PIREP worker: failed to process claimed PIREP", "record_id", item.AirtableRecordID, "va_id", item.VATID, "error", err)
						}

						// Acknowledge
						if err := w.redisQueue.AckPirep(ctx, streamName, "pirep-workers", messageIDs[i]); err != nil {
							logging.Error("PIREP worker: failed to ack claimed message", "message_id", messageIDs[i], "error", err)
						}
					}
				}
			}
		}
	}
}
