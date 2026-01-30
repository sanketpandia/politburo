package workers

// import (
// 	"infinite-experiment/politburo/infra/queue"
// 	"context"
// 	"fmt"
// 	"infinite-experiment/politburo/internal/db/repositories"
// 	"infinite-experiment/politburo/internal/models/dtos"
// 	gormModels "infinite-experiment/politburo/internal/models/gorm"
// 	"log"
// 	"strings"
// 	"sync"
// 	"time"

// 	"gorm.io/gorm"
// )

// // PirepQueueWorker processes PIREPs from Redis queue
// type PirepQueueWorker struct {
// 	workerID          string
// 	db                *gorm.DB
// 	redisQueue        *queue.RedisQueueService
// 	configRepo        *repositories.DataProviderConfigRepo
// 	pirepATSyncedRepo *repositories.PirepATSyncedRepo
// 	syncHistoryRepo   *repositories.VASyncHistoryRepo
// }

// // NewPirepQueueWorker creates a new PIREP queue worker
// func NewPirepQueueWorker(
// 	workerID string,
// 	db *gorm.DB,
// 	redisQueue *queue.RedisQueueService,
// 	configRepo *repositories.DataProviderConfigRepo,
// 	pirepATSyncedRepo *repositories.PirepATSyncedRepo,
// 	syncHistoryRepo *repositories.VASyncHistoryRepo,
// ) *PirepQueueWorker {
// 	return &PirepQueueWorker{
// 		workerID:          workerID,
// 		db:                db,
// 		redisQueue:        redisQueue,
// 		configRepo:        configRepo,
// 		pirepATSyncedRepo: pirepATSyncedRepo,
// 		syncHistoryRepo:   syncHistoryRepo,
// 	}
// }

// // Start begins processing PIREPs from all VA queues
// // Spawns multiple goroutines to handle different VA queues concurrently
// func (w *PirepQueueWorker) Start(ctx context.Context, numWorkers int) error {
// 	log.Printf("[PirepQueueWorker] Starting %d workers with ID prefix: %s", numWorkers, w.workerID)

// 	var wg sync.WaitGroup

// 	// Get all active VAs with Airtable configs
// 	var vaIDs []string
// 	err := w.db.WithContext(ctx).
// 		Table("va_data_provider_configs").
// 		Where("provider_type = ? AND is_active = ?", "airtable", true).
// 		Pluck("va_id", &vaIDs).Error

// 	if err != nil {
// 		return fmt.Errorf("failed to fetch active VAs: %w", err)
// 	}

// 	if len(vaIDs) == 0 {
// 		log.Printf("[PirepQueueWorker] No VAs with active Airtable configs found")
// 		return nil
// 	}

// 	log.Printf("[PirepQueueWorker] Found %d VAs to process", len(vaIDs))

// 	// Start workers for each VA
// 	for _, vaID := range vaIDs {
// 		streamName := fmt.Sprintf("pirep:sync:%s", vaID)

// 		// Ensure consumer group exists
// 		if err := w.redisQueue.CreateConsumerGroup(ctx, streamName, "pirep-workers"); err != nil {
// 			log.Printf("[PirepQueueWorker] Warning - failed to create consumer group for VA %s: %v", vaID, err)
// 		}

// 		// Start multiple workers for this VA queue
// 		for i := 0; i < numWorkers; i++ {
// 			wg.Add(1)
// 			workerName := fmt.Sprintf("%s-va-%s-worker-%d", w.workerID, vaID[:8], i)

// 			go func(vaID, workerName, streamName string) {
// 				defer wg.Done()
// 				w.processQueue(ctx, vaID, streamName, workerName)
// 			}(vaID, workerName, streamName)
// 		}
// 	}

// 	// Start a goroutine to periodically claim stale messages
// 	wg.Add(1)
// 	go func() {
// 		defer wg.Done()
// 		w.claimStaleMessages(ctx, vaIDs)
// 	}()

// 	wg.Wait()
// 	log.Printf("[PirepQueueWorker] All workers stopped")
// 	return nil
// }

// // processQueue continuously processes PIREPs from a specific VA queue
// func (w *PirepQueueWorker) processQueue(ctx context.Context, vaID, streamName, workerName string) {
// 	log.Printf("[%s] Started processing queue: %s", workerName, streamName)

// 	processedCount := 0
// 	errorCount := 0

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			log.Printf("[%s] Shutting down. Processed: %d, Errors: %d", workerName, processedCount, errorCount)
// 			return
// 		default:
// 			// Dequeue next PIREP (blocks for up to 5 seconds)
// 			item, messageID, err := w.redisQueue.DequeuePirep(ctx, streamName, "pirep-workers", workerName, 5*time.Second)
// 			if err != nil {
// 				log.Printf("[%s] Error dequeuing: %v", workerName, err)
// 				time.Sleep(1 * time.Second) // Back off on error
// 				continue
// 			}

// 			if item == nil {
// 				// No messages available (timeout), continue loop
// 				continue
// 			}

// 			// Process the PIREP
// 			if err := w.processPirep(ctx, item); err != nil {
// 				log.Printf("[%s] Error processing PIREP %s: %v", workerName, item.AirtableRecordID, err)
// 				errorCount++
// 				// Note: We still acknowledge to avoid reprocessing indefinitely
// 				// Production systems might want a DLQ (dead letter queue) here
// 			} else {
// 				processedCount++
// 			}

// 			// Acknowledge message
// 			if err := w.redisQueue.AckPirep(ctx, streamName, "pirep-workers", messageID); err != nil {
// 				log.Printf("[%s] Error acknowledging message %s: %v", workerName, messageID, err)
// 			}
// 		}
// 	}
// }

// // processPirep handles the actual PIREP upsert logic
// func (w *PirepQueueWorker) processPirep(ctx context.Context, item *queue.PirepQueueItem) error {
// 	// Get VA config to extract schema
// 	config, err := w.configRepo.GetActiveConfig(ctx, item.VATID, "airtable")
// 	if err != nil {
// 		return fmt.Errorf("failed to get config: %w", err)
// 	}

// 	if config == nil {
// 		return fmt.Errorf("no active config found for VA %s", item.VATID)
// 	}

// 	// Parse config data
// 	configData, err := repositories.ParseConfigData(config.ConfigData)
// 	if err != nil {
// 		return fmt.Errorf("failed to parse config: %w", err)
// 	}

// 	// Get PIREP schema
// 	pirepSchema := configData.GetSchemaByType("pirep")
// 	if pirepSchema == nil {
// 		return fmt.Errorf("pirep schema not found for VA %s", item.VATID)
// 	}

// 	// Extract and upsert PIREP
// 	return w.upsertPirep(ctx, item.VATID, item.AirtableRecordID, item.Fields, item.CreatedTime, pirepSchema)
// }

// // upsertPirep inserts or updates a PIREP record (same logic as the job)
// func (w *PirepQueueWorker) upsertPirep(ctx context.Context, vaID string, airtableRecordID string, record map[string]interface{}, createdTime string, schema *dtos.EntitySchema) error {
// 	// Extract field mappings
// 	routeField := schema.GetFieldMapping("route")
// 	flightModeField := schema.GetFieldMapping("flight_mode")
// 	flightTimeField := schema.GetFieldMapping("flight_time")
// 	pilotCallsignField := schema.GetFieldMapping("pilot_callsign")
// 	aircraftField := schema.GetFieldMapping("aircraft")
// 	liveryField := schema.GetFieldMapping("livery")
// 	routeATIDField := schema.GetFieldMapping("route_at_id")
// 	pilotATIDField := schema.GetFieldMapping("pilot_at_id")

// 	// Extract route
// 	var route string
// 	if routeField != nil {
// 		if rawRoute, ok := record[routeField.AirtableName]; ok {
// 			if routeStr, ok := rawRoute.(string); ok {
// 				route = strings.TrimSpace(routeStr)
// 			}
// 		}
// 	}

// 	// Extract flight mode
// 	var flightMode string
// 	if flightModeField != nil {
// 		if rawMode, ok := record[flightModeField.AirtableName]; ok {
// 			if modeStr, ok := rawMode.(string); ok {
// 				flightMode = strings.TrimSpace(modeStr)
// 			}
// 		}
// 	}

// 	// Extract flight time
// 	var flightTime *float64
// 	if flightTimeField != nil {
// 		if rawTime, ok := record[flightTimeField.AirtableName]; ok {
// 			switch v := rawTime.(type) {
// 			case float64:
// 				flightTime = &v
// 			case int:
// 				ft := float64(v)
// 				flightTime = &ft
// 			}
// 		}
// 	}

// 	// Extract pilot callsign
// 	var pilotCallsign string
// 	if pilotCallsignField != nil {
// 		if rawCallsign, ok := record[pilotCallsignField.AirtableName]; ok {
// 			if callsignStr, ok := rawCallsign.(string); ok {
// 				pilotCallsign = strings.TrimSpace(callsignStr)
// 			}
// 		}
// 	}

// 	// Extract aircraft
// 	var aircraft string
// 	if aircraftField != nil {
// 		if rawAircraft, ok := record[aircraftField.AirtableName]; ok {
// 			if aircraftStr, ok := rawAircraft.(string); ok {
// 				aircraft = strings.TrimSpace(aircraftStr)
// 			}
// 		}
// 	}

// 	// Extract livery
// 	var livery string
// 	if liveryField != nil {
// 		if rawLivery, ok := record[liveryField.AirtableName]; ok {
// 			if liveryStr, ok := rawLivery.(string); ok {
// 				livery = strings.TrimSpace(liveryStr)
// 			}
// 		}
// 	}

// 	// Extract route_at_id
// 	var routeATID *string
// 	if routeATIDField != nil {
// 		if rawRouteID, ok := record[routeATIDField.AirtableName]; ok {
// 			if idArray, ok := rawRouteID.([]interface{}); ok && len(idArray) > 0 {
// 				if idStr, ok := idArray[0].(string); ok {
// 					routeATID = &idStr
// 				}
// 			}
// 		}
// 	}

// 	// Extract pilot_at_id
// 	var pilotATID *string
// 	if pilotATIDField != nil {
// 		if rawPilotID, ok := record[pilotATIDField.AirtableName]; ok {
// 			if idArray, ok := rawPilotID.([]interface{}); ok && len(idArray) > 0 {
// 				if idStr, ok := idArray[0].(string); ok {
// 					pilotATID = &idStr
// 				}
// 			}
// 		}
// 	}

// 	// Parse created time
// 	var atCreatedTime *time.Time
// 	if createdTime != "" {
// 		if t, err := time.Parse(time.RFC3339, createdTime); err == nil {
// 			atCreatedTime = &t
// 		}
// 	}

// 	// Create PIREP entity
// 	pirepATSynced := &gormModels.PirepATSynced{
// 		ATID:          airtableRecordID,
// 		ServerID:      vaID,
// 		Route:         route,
// 		FlightMode:    flightMode,
// 		FlightTime:    flightTime,
// 		PilotCallsign: pilotCallsign,
// 		Aircraft:      aircraft,
// 		Livery:        livery,
// 		RouteATID:     routeATID,
// 		PilotATID:     pilotATID,
// 		ATCreatedTime: atCreatedTime,
// 	}

// 	// Upsert
// 	if err := w.pirepATSyncedRepo.Upsert(ctx, pirepATSynced); err != nil {
// 		return fmt.Errorf("failed to upsert: %w", err)
// 	}

// 	return nil
// }

// // claimStaleMessages periodically claims messages that have been idle too long
// func (w *PirepQueueWorker) claimStaleMessages(ctx context.Context, vaIDs []string) {
// 	ticker := time.NewTicker(2 * time.Minute)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-ticker.C:
// 			for _, vaID := range vaIDs {
// 				streamName := fmt.Sprintf("pirep:sync:%s", vaID)
// 				claimerName := fmt.Sprintf("%s-claimer", w.workerID)

// 				items, messageIDs, err := w.redisQueue.ClaimStalePireps(ctx, streamName, "pirep-workers", claimerName, 5*time.Minute)
// 				if err != nil {
// 					log.Printf("[PirepQueueWorker] Error claiming stale messages for VA %s: %v", vaID, err)
// 					continue
// 				}

// 				if len(items) > 0 {
// 					log.Printf("[PirepQueueWorker] Claimed %d stale messages for VA %s", len(items), vaID)

// 					// Process claimed items
// 					for i, item := range items {
// 						if err := w.processPirep(ctx, item); err != nil {
// 							log.Printf("[PirepQueueWorker] Error processing claimed PIREP: %v", err)
// 						}

// 						// Acknowledge
// 						if err := w.redisQueue.AckPirep(ctx, streamName, "pirep-workers", messageIDs[i]); err != nil {
// 							log.Printf("[PirepQueueWorker] Error acknowledging claimed message: %v", err)
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// }
