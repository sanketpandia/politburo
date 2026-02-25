package pilots

import (
	"context"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/queue"
	"sync"
	"time"
)

const (
	pilotSyncStreamName = "pilot_sync_queue"
	pilotSyncGroupName  = "pilot-workers"
	pilotSyncConsumer   = "worker_1"
	maxRetries          = 3
	retryDelay          = 5 * time.Second
	retryKeyTTL         = 1 * time.Hour // TTL for retry tracking keys
)

// SyncWorker processes pilot sync requests from Redis queue
// It processes pilot records from Airtable and upserts them to the database
type SyncWorker struct {
	queueService *queue.RedisQueueService
	syncJob      *SyncJob // Reuse sync job for upsert logic
	metrics      *metrics.MetricsRegistry
	cache        cache.CacheInterface
	retryCounts  map[string]int // messageID -> retry count
	retryMutex   sync.RWMutex   // Protects retryCounts map
}

// NewSyncWorker creates a new pilot sync worker
func NewSyncWorker(
	queueService *queue.RedisQueueService,
	syncJob *SyncJob,
	metricsReg *metrics.MetricsRegistry,
	cache cache.CacheInterface,
) *SyncWorker {
	return &SyncWorker{
		queueService: queueService,
		syncJob:      syncJob,
		metrics:      metricsReg,
		cache:        cache,
		retryCounts:  make(map[string]int),
	}
}

// Start begins processing pilot sync requests from the queue
func (w *SyncWorker) Start(ctx context.Context) error {
	// Create consumer group if it doesn't exist
	if err := w.queueService.CreateConsumerGroup(ctx, pilotSyncStreamName, pilotSyncGroupName); err != nil {
		logging.Warn("Failed to create consumer group (may already exist)", "error", err)
	}

	logging.Info("Pilot sync worker started", "stream", pilotSyncStreamName, "group", pilotSyncGroupName)

	// Main processing loop
	for {
		select {
		case <-ctx.Done():
			logging.Info("Pilot sync worker stopping")
			return nil
		default:
			if err := w.processNext(ctx); err != nil {
				logging.Error("Error processing pilot sync item", "error", err)
				time.Sleep(retryDelay)
			}
		}
	}
}

// processNext processes the next item from the queue
func (w *SyncWorker) processNext(ctx context.Context) error {
	// Dequeue item
	item, msgID, err := w.queueService.DequeuePilot(ctx, pilotSyncStreamName, pilotSyncGroupName, pilotSyncConsumer, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to dequeue pilot: %w", err)
	}
	if item == nil {
		return nil // No items available
	}

	// Track dequeued metric
	if w.metrics != nil {
		w.metrics.QueueDequeuedTotal.WithLabelValues(pilotSyncStreamName, "pilot").Inc()
	}

	// Process with metrics
	start := time.Now()
	err = w.processPilot(ctx, item)
	duration := time.Since(start)

	if err != nil {
		// Handle retry logic
		retryCount := w.getRetryCount(msgID)
		if retryCount < maxRetries {
			w.incrementRetryCount(msgID)
			logging.Warn("Pilot sync failed, will retry", "msg_id", msgID, "retry", retryCount+1, "error", err)
			// Track retry metric
			if w.metrics != nil {
				w.metrics.QueueRetriesTotal.WithLabelValues(pilotSyncStreamName, "pilot").Inc()
				w.metrics.QueueErrorsTotal.WithLabelValues(pilotSyncStreamName, "pilot", "transient").Inc()
			}
			// Don't ACK, let it be retried
			return nil
		} else {
			// Max retries exceeded
			if w.metrics != nil {
				w.metrics.QueueErrorsTotal.WithLabelValues(pilotSyncStreamName, "pilot", "max_retries_exceeded").Inc()
				w.metrics.SyncJobRecordsFailed.WithLabelValues("pilot_sync_job", "airtable", "pilot", item.VAID, "max_retries_exceeded").Inc()
			}
			logging.Error("Pilot sync failed after max retries", "msg_id", msgID, "va_id", item.VAID, "error", err)
			// ACK to remove from queue
			if ackErr := w.queueService.AckPilot(ctx, pilotSyncStreamName, pilotSyncGroupName, msgID); ackErr != nil {
				logging.Error("Failed to ACK pilot sync message after max retries", "error", ackErr)
			} else if w.metrics != nil {
				w.metrics.QueueAcknowledgedTotal.WithLabelValues(pilotSyncStreamName, "pilot").Inc()
			}
			w.clearRetryCount(msgID)
			return nil
		}
	}

	// Success - ACK and record metrics
	if err := w.queueService.AckPilot(ctx, pilotSyncStreamName, pilotSyncGroupName, msgID); err != nil {
		logging.Error("Failed to ACK pilot sync message", "error", err)
	} else {
		if w.metrics != nil {
			w.metrics.QueueAcknowledgedTotal.WithLabelValues(pilotSyncStreamName, "pilot").Inc()
		}
	}

	if w.metrics != nil {
		w.metrics.QueueProcessingDuration.WithLabelValues(pilotSyncStreamName, "pilot").Observe(duration.Seconds())
		w.metrics.SyncJobRecordsProcessed.WithLabelValues("pilot_sync_job", "airtable", "pilot", item.VAID, "success").Inc()
	}
	w.clearRetryCount(msgID)

	return nil
}

// processPilot processes a single pilot record
func (w *SyncWorker) processPilot(ctx context.Context, item *queue.PilotQueueItem) error {
	// Convert queue schema back to platformVA.EntitySchema
	schema := convertQueueSchemaToVA(item.Schema)
	if schema == nil {
		return fmt.Errorf("failed to convert queue schema to VA schema")
	}

	// Reuse sync job's upsert logic
	return w.syncJob.upsertPilot(ctx, item.VAID, item.AirtableRecordID, item.Fields, schema)
}

// getRetryCount gets the current retry count for a message
func (w *SyncWorker) getRetryCount(msgID string) int {
	if msgID == "" {
		return 0
	}

	// Try cache first
	if w.cache != nil {
		retryKey := fmt.Sprintf("pilot_sync_retry:%s", msgID)
		val, found := w.cache.Get(retryKey)
		if found {
			if count, ok := val.(int); ok {
				return count
			}
			if count, ok := val.(float64); ok {
				return int(count)
			}
		}
	}

	// Fall back to in-memory map
	w.retryMutex.RLock()
	defer w.retryMutex.RUnlock()
	return w.retryCounts[msgID]
}

// incrementRetryCount increments the retry count for a message
func (w *SyncWorker) incrementRetryCount(msgID string) int {
	if msgID == "" {
		return 0
	}

	retryKey := fmt.Sprintf("pilot_sync_retry:%s", msgID)
	currentCount := w.getRetryCount(msgID)
	newCount := currentCount + 1

	// Store in cache if available
	if w.cache != nil {
		w.cache.Set(retryKey, newCount, retryKeyTTL)
	}

	// Also store in memory map
	w.retryMutex.Lock()
	w.retryCounts[msgID] = newCount
	w.retryMutex.Unlock()

	return newCount
}

// clearRetryCount clears the retry count for a message (on success)
func (w *SyncWorker) clearRetryCount(msgID string) {
	if msgID == "" {
		return
	}

	retryKey := fmt.Sprintf("pilot_sync_retry:%s", msgID)

	// Clear from cache
	if w.cache != nil {
		w.cache.Delete(retryKey)
	}

	// Clear from memory map
	w.retryMutex.Lock()
	delete(w.retryCounts, msgID)
	w.retryMutex.Unlock()
}
