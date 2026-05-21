package flights

import (
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/queue"
	"infinite-experiment/politburo/internal/constants"
	"net/http"
	"strings"
	"time"
)

const (
	flightPlanStreamName = "flight_plan_queue"
	flightPlanGroupName  = "flight_plan_workers"
	flightPlanConsumer   = "worker_1"
	maxRetryAttempts     = 2             // Maximum retry attempts for any error
	retryKeyTTL          = 1 * time.Hour // TTL for retry tracking keys
)

// isPermanentError checks if an error is a permanent error that shouldn't be retried
func isPermanentError(err error) (bool, int) {
	if err == nil {
		return false, 0
	}

	errStr := err.Error()
	// Check for HTTP status codes in error messages
	// Format: "unexpected status 400" or "unexpected status 404"
	var status int
	if strings.Contains(errStr, "unexpected status 400") {
		status = http.StatusBadRequest
	} else if strings.Contains(errStr, "unexpected status 404") || strings.Contains(errStr, "resource not found") {
		status = http.StatusNotFound
	} else {
		return false, 0
	}

	return true, status
}

// getRetryCount gets the current retry count for a message
func (w *FlightPlanWorker) getRetryCount(msgID string) int {
	if msgID == "" {
		return 0
	}
	retryKey := fmt.Sprintf("flight_plan_retry:%s", msgID)
	val, found := w.cache.Get(retryKey)
	if !found {
		return 0
	}
	// Try to convert to int
	if count, ok := val.(int); ok {
		return count
	}
	if count, ok := val.(float64); ok {
		return int(count)
	}
	return 0
}

// incrementRetryCount increments the retry count for a message
func (w *FlightPlanWorker) incrementRetryCount(msgID string) int {
	if msgID == "" {
		return 0
	}
	retryKey := fmt.Sprintf("flight_plan_retry:%s", msgID)
	currentCount := w.getRetryCount(msgID)
	newCount := currentCount + 1
	// Store as int (will be serialized by cache)
	w.cache.Set(retryKey, newCount, retryKeyTTL)
	return newCount
}

// clearRetryCount clears the retry count for a message (on success)
func (w *FlightPlanWorker) clearRetryCount(msgID string) {
	if msgID == "" {
		return
	}
	retryKey := fmt.Sprintf("flight_plan_retry:%s", msgID)
	w.cache.Delete(retryKey)
}

// FlightPlanWorker processes flight plan requests from Redis queue
// It fetches flight plans, extracts route information, and updates cached flight data
type FlightPlanWorker struct {
	queueService *queue.RedisQueueService
	cache        cache.CacheInterface
	liveAPI      *liveapi.Client
	metrics      *metrics.MetricsRegistry
	ctx          context.Context
}

// NewFlightPlanWorker creates a new flight plan worker
func NewFlightPlanWorker(
	queueService *queue.RedisQueueService,
	cache cache.CacheInterface,
	liveAPI *liveapi.Client,
	metricsReg *metrics.MetricsRegistry,
) *FlightPlanWorker {
	return &FlightPlanWorker{
		queueService: queueService,
		cache:        cache,
		liveAPI:      liveAPI,
		metrics:      metricsReg,
		ctx:          context.Background(),
	}
}

// Start begins processing flight plan requests from the queue
func (w *FlightPlanWorker) Start(ctx context.Context) error {
	w.ctx = ctx

	// Create consumer group if it doesn't exist
	if err := w.queueService.CreateConsumerGroup(ctx, flightPlanStreamName, flightPlanGroupName); err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	logging.Info("Flight plan worker started", "stream", flightPlanStreamName, "group", flightPlanGroupName)

	// Start stale message claimer in background
	go w.claimStaleMessages(ctx)

	// Process messages with delay between each
	for {
		select {
		case <-ctx.Done():
			logging.Info("Flight plan worker stopping")
			return nil
		default:
			// Dequeue a flight plan request (block for 5 seconds)
			item, msgID, err := w.queueService.DequeueFlightPlan(ctx, flightPlanStreamName, flightPlanGroupName, flightPlanConsumer, 5*time.Second)
			if err != nil {
				logging.Error("Failed to dequeue flight plan", "error", err)
				if w.metrics != nil {
					w.metrics.QueueErrorsTotal.WithLabelValues(flightPlanStreamName, "flight_plan", "dequeue_error").Inc()
				}
				time.Sleep(1 * time.Second)
				continue
			}

			if item == nil {
				// No messages available, continue loop
				continue
			}

			// Track dequeued metric
			if w.metrics != nil {
				w.metrics.QueueDequeuedTotal.WithLabelValues(flightPlanStreamName, "flight_plan").Inc()
			}

			// Track processing duration
			startTime := time.Now()
			// Process the flight plan request
			if err := w.processFlightPlan(ctx, item); err != nil {
				// Track processing duration
				if w.metrics != nil {
					duration := time.Since(startTime).Seconds()
					w.metrics.QueueProcessingDuration.WithLabelValues(flightPlanStreamName, "flight_plan").Observe(duration)
				}

				// Increment retry count for this message
				retryCount := w.incrementRetryCount(msgID)

				// Track error metric
				if w.metrics != nil {
					errorType := "transient"
					isPermanent, statusCode := isPermanentError(err)
					if isPermanent {
						errorType = fmt.Sprintf("permanent_%d", statusCode)
					}
					w.metrics.QueueErrorsTotal.WithLabelValues(flightPlanStreamName, "flight_plan", errorType).Inc()
				}

				// Check if we've exceeded max retry attempts
				if retryCount >= maxRetryAttempts {
					logging.Warn("Max retry attempts reached - acknowledging message (will be re-queued by flights worker if flight becomes active)",
						"sessionID", item.SessionID,
						"flightID", item.FlightID,
						"retryCount", retryCount,
						"error", err,
					)
					// ACK the message - flights cache worker will re-queue if flight becomes active again
					if msgID != "" {
						if ackErr := w.queueService.AckFlightPlan(ctx, flightPlanStreamName, flightPlanGroupName, msgID); ackErr != nil {
							logging.Error("Failed to ack max-retry message", "messageID", msgID, "error", ackErr)
						} else if w.metrics != nil {
							w.metrics.QueueAcknowledgedTotal.WithLabelValues(flightPlanStreamName, "flight_plan").Inc()
						}
						// Clear retry count since we're done with this message
						w.clearRetryCount(msgID)
					}
					continue
				}

				// Check if this is a permanent error (400, 404) that shouldn't be retried
				isPermanent, statusCode := isPermanentError(err)
				if isPermanent {
					logging.Warn("Permanent error processing flight plan - acknowledging to stop retries",
						"sessionID", item.SessionID,
						"flightID", item.FlightID,
						"statusCode", statusCode,
						"retryCount", retryCount,
						"error", err,
					)
					// ACK the message to stop infinite retries for permanent errors
					if msgID != "" {
						if ackErr := w.queueService.AckFlightPlan(ctx, flightPlanStreamName, flightPlanGroupName, msgID); ackErr != nil {
							logging.Error("Failed to ack permanent error message", "messageID", msgID, "error", ackErr)
						} else if w.metrics != nil {
							w.metrics.QueueAcknowledgedTotal.WithLabelValues(flightPlanStreamName, "flight_plan").Inc()
						}
						// Clear retry count since we're done with this message
						w.clearRetryCount(msgID)
					}
					continue
				}

				// Check if flight is no longer in cache (user went offline)
				flightKey := cache.LiveFlightKey(item.FlightID)
				if _, found := w.cache.Get(flightKey); !found {
					logging.Warn("Flight no longer in cache (user likely offline) - acknowledging to stop retries",
						"sessionID", item.SessionID,
						"flightID", item.FlightID,
						"retryCount", retryCount,
						"error", err,
					)
					// ACK the message since the flight is gone
					if msgID != "" {
						if ackErr := w.queueService.AckFlightPlan(ctx, flightPlanStreamName, flightPlanGroupName, msgID); ackErr != nil {
							logging.Error("Failed to ack offline flight message", "messageID", msgID, "error", ackErr)
						} else if w.metrics != nil {
							w.metrics.QueueAcknowledgedTotal.WithLabelValues(flightPlanStreamName, "flight_plan").Inc()
						}
						// Clear retry count since we're done with this message
						w.clearRetryCount(msgID)
					}
					continue
				}

				// Transient error - log and let it be retried (up to maxRetryAttempts)
				logging.Error("Failed to process flight plan (transient error - will retry)",
					"sessionID", item.SessionID,
					"flightID", item.FlightID,
					"retryCount", retryCount,
					"maxRetries", maxRetryAttempts,
					"error", err,
				)
				// Track retry metric
				if w.metrics != nil {
					w.metrics.QueueRetriesTotal.WithLabelValues(flightPlanStreamName, "flight_plan").Inc()
				}
				// Don't ack on transient error - let it be retried (up to maxRetryAttempts)
				continue
			}

			// Track processing duration on success
			if w.metrics != nil {
				duration := time.Since(startTime).Seconds()
				w.metrics.QueueProcessingDuration.WithLabelValues(flightPlanStreamName, "flight_plan").Observe(duration)
			}

			// Success - clear retry count and acknowledge
			w.clearRetryCount(msgID)
			if msgID != "" {
				if err := w.queueService.AckFlightPlan(ctx, flightPlanStreamName, flightPlanGroupName, msgID); err != nil {
					logging.Error("Failed to ack flight plan message", "messageID", msgID, "error", err)
				} else if w.metrics != nil {
					w.metrics.QueueAcknowledgedTotal.WithLabelValues(flightPlanStreamName, "flight_plan").Inc()
				}
			}

			// Add delay between processing (spacing out API calls)
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// claimStaleMessages periodically claims messages that have been pending for too long
// This handles messages from dead workers or crashes
func (w *FlightPlanWorker) claimStaleMessages(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Claim messages that have been idle for 5+ minutes
			items, _, err := w.queueService.ClaimStaleFlightPlans(ctx, flightPlanStreamName, flightPlanGroupName, flightPlanConsumer+"-claimer", 5*time.Minute)
			if err != nil {
				logging.Debug("Failed to claim stale flight plan messages", "error", err)
				continue
			}

			if len(items) > 0 {
				logging.Info("Claimed stale flight plan messages", "count", len(items))
				// Process claimed items (they will be handled by the main loop on next iteration)
				// Note: These messages are now assigned to our consumer, so they'll be picked up by DequeueFlightPlan
			}
		}
	}
}

// processFlightPlan processes a single flight plan request
func (w *FlightPlanWorker) processFlightPlan(ctx context.Context, item *queue.FlightPlanQueueItem) error {
	flightID := item.FlightID
	sessionID := item.SessionID

	// Get the cached flight to check phase and last update time
	flightKey := cache.LiveFlightKey(flightID)
	cachedVal, found := w.cache.Get(flightKey)
	if !found {
		return fmt.Errorf("flight not found in cache: %s", flightID)
	}

	// Convert cached value to CompleteFlight
	var completeFlight CompleteFlight
	jsonBytes, err := json.Marshal(cachedVal)
	if err != nil {
		return fmt.Errorf("failed to marshal cached flight: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &completeFlight); err != nil {
		return fmt.Errorf("failed to unmarshal cached flight: %w", err)
	}

	// Check if we should fetch based on phase and last flight plan fetch time
	// Note: This check should rarely fail since we filter at enqueue time,
	// but it's kept as a safety check in case flight state changed
	shouldFetch, delay := ShouldFetchFlightPlan(&completeFlight)
	if !shouldFetch {
		logging.Debug("Skipping flight plan fetch - too soon (safety check)",
			"flightID", flightID,
			"phase", completeFlight.Phase,
			"lastFlightPlanFetch", completeFlight.LastFlightPlanFetch,
		)
		return nil
	}

	// Fetch flight plan with the standardized LiveAPI-derived TTL.
	// This uses the same cache key as flights.Service for consistency
	fpl, err := w.getFlightPlanCached(sessionID, flightID)
	if err != nil {
		return fmt.Errorf("failed to fetch flight plan: %w", err)
	}

	// Extract origin and destination from waypoints
	origin, destination := w.extractRouteFromFPL(fpl)

	// Store full flight plan in separate cache key for direct access.
	fplKey := cache.FlightPlanKey(flightID)
	w.cache.Set(fplKey, fpl, cache.FlightPlanTTL)

	// Update the CompleteFlight with route information
	now := time.Now().UTC()
	completeFlight.Origin = origin
	completeFlight.Destination = destination
	completeFlight.LastFlightPlanFetch = now
	// Note: Don't update LastUpdated here - that's managed by cache_job.go

	// Save updated flight back to cache (preserve existing TTL)
	w.cache.Set(flightKey, completeFlight, cache.LiveFlightTTL)

	logging.Debug("Updated flight plan",
		"flightID", flightID,
		"origin", origin,
		"destination", destination,
		"phase", completeFlight.Phase,
	)

	// Add delay based on phase before next fetch
	if delay > 0 {
		time.Sleep(delay)
	}

	return nil
}

// getFlightPlanCached fetches flight plan from API (bypasses cache) and updates cache
// Returns the flight plan or an error (which may be a permanent error for 400/404 status codes)
// The worker always fetches fresh data to ensure flight plans are up-to-date
func (w *FlightPlanWorker) getFlightPlanCached(sessionID, flightID string) (*liveapi.FlightPlanResponse, error) {
	// Always fetch fresh data from API (bypass cache)
	fpl, status, err := w.liveAPI.GetFlightPlan(sessionID, flightID)
	if err != nil {
		// Preserve status code in error for permanent error detection
		// The error from liveapi.Client already includes status in the message
		// but we can enhance it for better detection
		if status == http.StatusBadRequest || status == http.StatusNotFound {
			return nil, fmt.Errorf("unexpected status %d: %w", status, err)
		}
		return nil, err
	}

	// Update cache with fresh data for other parts of the system.
	// Use same cache key pattern as flights.Service for consistency
	cacheKey := string(constants.CachePrefixFPL) + sessionID + "_" + flightID
	w.cache.Set(cacheKey, *fpl, cache.FlightPlanTTL)

	return fpl, nil
}

// extractRouteFromFPL extracts origin and destination from flight plan waypoints
func (w *FlightPlanWorker) extractRouteFromFPL(fpl *liveapi.FlightPlanResponse) (origin, destination string) {
	waypoints := fpl.Waypoints
	if len(waypoints) < 2 {
		return "", ""
	}

	// First waypoint is origin (if it's a 4-character ICAO code)
	first := waypoints[0]
	if len(first) == 4 {
		origin = strings.ToUpper(first)
	}

	// Last waypoint is destination (if it's a 4-character ICAO code)
	last := waypoints[len(waypoints)-1]
	if len(last) == 4 {
		destination = strings.ToUpper(last)
	}

	return origin, destination
}
