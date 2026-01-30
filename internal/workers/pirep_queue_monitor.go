package workers

import (
	"infinite-experiment/politburo/infra/queue"
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// PirepQueueMonitor monitors Redis queue health and metrics
type PirepQueueMonitor struct {
	db         *gorm.DB
	redisQueue *queue.RedisQueueService
}

// NewPirepQueueMonitor creates a new queue monitor
func NewPirepQueueMonitor(db *gorm.DB, redisQueue *queue.RedisQueueService) *PirepQueueMonitor {
	return &PirepQueueMonitor{
		db:         db,
		redisQueue: redisQueue,
	}
}

// QueueStats represents statistics for a single VA queue
type QueueStats struct {
	VAID         string
	VAName       string
	StreamName   string
	QueueLength  int64
	PendingCount int64
	LastChecked  time.Time
}

// Start begins monitoring all VA queues
func (m *PirepQueueMonitor) Start(ctx context.Context, interval time.Duration) {
	log.Printf("[PirepQueueMonitor] Starting queue monitoring (interval: %s)", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	m.checkQueues(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[PirepQueueMonitor] Shutting down")
			return
		case <-ticker.C:
			m.checkQueues(ctx)
		}
	}
}

// checkQueues checks all VA queues and logs their status
func (m *PirepQueueMonitor) checkQueues(ctx context.Context) {
	// Get all active VAs
	var vaData []struct {
		ID   string `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}

	err := m.db.WithContext(ctx).
		Table("virtual_airlines va").
		Select("va.id, va.name").
		Joins("JOIN va_data_provider_configs cfg ON va.id = cfg.va_id").
		Where("cfg.provider_type = ? AND cfg.is_active = ?", "airtable", true).
		Find(&vaData).Error

	if err != nil {
		log.Printf("[PirepQueueMonitor] Error fetching VAs: %v", err)
		return
	}

	if len(vaData) == 0 {
		return
	}

	log.Printf("[PirepQueueMonitor] ========== Queue Health Check ==========")

	totalQueueLength := int64(0)
	totalPending := int64(0)
	alertCount := 0

	for _, va := range vaData {
		streamName := fmt.Sprintf("pirep:sync:%s", va.ID)

		stats, err := m.getQueueStats(ctx, va.ID, va.Name, streamName)
		if err != nil {
			log.Printf("[PirepQueueMonitor] Error getting stats for VA %s: %v", va.Name, err)
			continue
		}

		totalQueueLength += stats.QueueLength
		totalPending += stats.PendingCount

		// Log queue status
		status := "OK"
		if stats.PendingCount > 1000 {
			status = "HIGH PENDING"
			alertCount++
		} else if stats.QueueLength > 5000 {
			status = "HIGH QUEUE"
			alertCount++
		}

		log.Printf("[PirepQueueMonitor] VA: %s | Queue: %d | Pending: %d | Status: %s",
			va.Name, stats.QueueLength, stats.PendingCount, status)
	}

	log.Printf("[PirepQueueMonitor] TOTAL - Queue: %d | Pending: %d | Alerts: %d",
		totalQueueLength, totalPending, alertCount)
	log.Printf("[PirepQueueMonitor] ========================================")

	// Trigger alerts if needed
	if alertCount > 0 {
		log.Printf("[PirepQueueMonitor] ⚠️  WARNING: %d queues need attention", alertCount)
	}
}

// getQueueStats retrieves statistics for a specific queue
func (m *PirepQueueMonitor) getQueueStats(ctx context.Context, vaID, vaName, streamName string) (*QueueStats, error) {
	queueLength, err := m.redisQueue.GetQueueLength(ctx, streamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue length: %w", err)
	}

	pendingCount, err := m.redisQueue.GetPendingCount(ctx, streamName, "pirep-workers")
	if err != nil {
		// If consumer group doesn't exist yet, pending count is 0
		pendingCount = 0
	}

	return &QueueStats{
		VAID:         vaID,
		VAName:       vaName,
		StreamName:   streamName,
		QueueLength:  queueLength,
		PendingCount: pendingCount,
		LastChecked:  time.Now(),
	}, nil
}

// GetAllQueueStats returns stats for all VA queues (for API endpoints)
func (m *PirepQueueMonitor) GetAllQueueStats(ctx context.Context) ([]QueueStats, error) {
	var vaData []struct {
		ID   string `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}

	err := m.db.WithContext(ctx).
		Table("virtual_airlines va").
		Select("va.id, va.name").
		Joins("JOIN va_data_provider_configs cfg ON va.id = cfg.va_id").
		Where("cfg.provider_type = ? AND cfg.is_active = ?", "airtable", true).
		Find(&vaData).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch VAs: %w", err)
	}

	var stats []QueueStats
	for _, va := range vaData {
		streamName := fmt.Sprintf("pirep:sync:%s", va.ID)
		stat, err := m.getQueueStats(ctx, va.ID, va.Name, streamName)
		if err != nil {
			log.Printf("[PirepQueueMonitor] Error getting stats for VA %s: %v", va.Name, err)
			continue
		}
		stats = append(stats, *stat)
	}

	return stats, nil
}

// TrimOldMessages removes old processed messages from queues to prevent memory bloat
func (m *PirepQueueMonitor) TrimOldMessages(ctx context.Context, maxLen int64) error {
	log.Printf("[PirepQueueMonitor] Trimming old messages (max length: %d)", maxLen)

	var vaIDs []string
	err := m.db.WithContext(ctx).
		Table("va_data_provider_configs").
		Where("provider_type = ? AND is_active = ?", "airtable", true).
		Pluck("va_id", &vaIDs).Error

	if err != nil {
		return fmt.Errorf("failed to fetch VAs: %w", err)
	}

	for _, vaID := range vaIDs {
		streamName := fmt.Sprintf("pirep:sync:%s", vaID)
		if err := m.redisQueue.TrimStream(ctx, streamName, maxLen); err != nil {
			log.Printf("[PirepQueueMonitor] Error trimming stream %s: %v", streamName, err)
			continue
		}
	}

	log.Printf("[PirepQueueMonitor] Successfully trimmed %d streams", len(vaIDs))
	return nil
}

// StartAutoTrim starts automatic trimming of old messages
func (m *PirepQueueMonitor) StartAutoTrim(ctx context.Context, interval time.Duration, maxLen int64) {
	log.Printf("[PirepQueueMonitor] Starting auto-trim (interval: %s, max length: %d)", interval, maxLen)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[PirepQueueMonitor] Auto-trim shutting down")
			return
		case <-ticker.C:
			if err := m.TrimOldMessages(ctx, maxLen); err != nil {
				log.Printf("[PirepQueueMonitor] Auto-trim error: %v", err)
			}
		}
	}
}
