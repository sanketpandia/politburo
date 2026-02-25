package flights

import (
	"context"
	"fmt"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/queue"
	"time"
)

// FlightPlanQueueMonitor monitors the flight plan queue health and metrics
type FlightPlanQueueMonitor struct {
	redisQueue *queue.RedisQueueService
	metrics    *metrics.MetricsRegistry
}

// NewFlightPlanQueueMonitor creates a new flight plan queue monitor
func NewFlightPlanQueueMonitor(redisQueue *queue.RedisQueueService, metricsReg *metrics.MetricsRegistry) *FlightPlanQueueMonitor {
	return &FlightPlanQueueMonitor{
		redisQueue: redisQueue,
		metrics:    metricsReg,
	}
}

// FlightPlanQueueStats represents statistics for the flight plan queue
type FlightPlanQueueStats struct {
	StreamName   string
	QueueLength  int64
	PendingCount int64
	LastChecked  time.Time
}

// Start begins monitoring the flight plan queue
func (m *FlightPlanQueueMonitor) Start(ctx context.Context, interval time.Duration) {
	logging.Info("Starting flight plan queue monitoring", "interval", interval.String())

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	m.checkQueue(ctx)

	for {
		select {
		case <-ctx.Done():
			logging.Info("Flight plan queue monitor shutting down")
			return
		case <-ticker.C:
			m.checkQueue(ctx)
		}
	}
}

// checkQueue checks the flight plan queue and logs its status
func (m *FlightPlanQueueMonitor) checkQueue(ctx context.Context) {
	stats, err := m.getQueueStats(ctx)
	if err != nil {
		logging.Error("Failed to get flight plan queue stats", "error", err)
		return
	}

	// Update metrics
	if m.metrics != nil {
		m.metrics.QueueDepth.WithLabelValues(flightPlanStreamName, "flight_plan").Set(float64(stats.QueueLength))
		m.metrics.QueuePending.WithLabelValues(flightPlanStreamName, "flight_plan").Set(float64(stats.PendingCount))
	}

	// Determine status
	status := "OK"
	if stats.PendingCount > 1000 {
		status = "HIGH PENDING"
	} else if stats.QueueLength > 5000 {
		status = "HIGH QUEUE"
	}

	logging.Info("Flight plan queue health check",
		"queue_length", stats.QueueLength,
		"pending_count", stats.PendingCount,
		"status", status,
	)

	// Log warning if queue needs attention
	if stats.PendingCount > 1000 || stats.QueueLength > 5000 {
		logging.Warn("Flight plan queue needs attention",
			"queue_length", stats.QueueLength,
			"pending_count", stats.PendingCount,
		)
	}
}

// getQueueStats retrieves statistics for the flight plan queue
func (m *FlightPlanQueueMonitor) getQueueStats(ctx context.Context) (*FlightPlanQueueStats, error) {
	queueLength, err := m.redisQueue.GetQueueLength(ctx, flightPlanStreamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue length: %w", err)
	}

	pendingCount, err := m.redisQueue.GetPendingCount(ctx, flightPlanStreamName, flightPlanGroupName)
	if err != nil {
		// If consumer group doesn't exist yet, pending count is 0
		pendingCount = 0
	}

	return &FlightPlanQueueStats{
		StreamName:   flightPlanStreamName,
		QueueLength:  queueLength,
		PendingCount: pendingCount,
		LastChecked:  time.Now(),
	}, nil
}

// GetQueueStats returns current queue statistics (for API endpoints)
func (m *FlightPlanQueueMonitor) GetQueueStats(ctx context.Context) (*FlightPlanQueueStats, error) {
	return m.getQueueStats(ctx)
}

// TrimOldMessages removes old processed messages from the queue to prevent memory bloat
// Keeps only the most recent maxLen messages
func (m *FlightPlanQueueMonitor) TrimOldMessages(ctx context.Context, maxLen int64) error {
	logging.Info("Trimming old flight plan queue messages", "max_length", maxLen)

	if err := m.redisQueue.TrimStream(ctx, flightPlanStreamName, maxLen); err != nil {
		return fmt.Errorf("failed to trim stream: %w", err)
	}

	logging.Info("Successfully trimmed flight plan queue", "max_length", maxLen)
	return nil
}

// StartAutoTrim starts automatic trimming of old messages
func (m *FlightPlanQueueMonitor) StartAutoTrim(ctx context.Context, interval time.Duration, maxLen int64) {
	logging.Info("Starting auto-trim for flight plan queue",
		"interval", interval.String(),
		"max_length", maxLen,
	)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Info("Flight plan queue auto-trim shutting down")
			return
		case <-ticker.C:
			if err := m.TrimOldMessages(ctx, maxLen); err != nil {
				logging.Error("Auto-trim error", "error", err)
			}
		}
	}
}
