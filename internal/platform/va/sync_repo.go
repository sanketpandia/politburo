package va

import (
	"context"
	"time"

	"infinite-experiment/politburo/internal/models/gorm"

	gormlib "gorm.io/gorm"
)

// SyncRepository handles sync history operations
type SyncRepository struct {
	db *gormlib.DB
}

// NewSyncRepository creates a new sync history repository
func NewSyncRepository(db *gormlib.DB) *SyncRepository {
	return &SyncRepository{db: db}
}

// RecordSync records a successful sync operation for a VA
// Simple method: just post the server ID (vaID) and sync event
func (r *SyncRepository) RecordSync(ctx context.Context, vaID string, event string) error {
	now := time.Now()

	syncHistory := gorm.VASyncHistory{
		VAID:       vaID,
		Event:      event,
		LastSyncAt: &now,
	}

	// Upsert: if record exists for this VA and event, update last_sync_at
	// Otherwise, create new record
	err := r.db.WithContext(ctx).
		Where("va_id = ? AND event = ?", vaID, event).
		Assign(gorm.VASyncHistory{LastSyncAt: &now}).
		FirstOrCreate(&syncHistory).Error

	return err
}

// GetLastSyncTimeForEvent retrieves the most recent sync timestamp across all VAs for a specific event
// Used to check if we should run initial sync on app restart
func (r *SyncRepository) GetLastSyncTimeForEvent(ctx context.Context, event string) (*time.Time, error) {
	var syncHistory gorm.VASyncHistory

	err := r.db.WithContext(ctx).
		Where("event = ?", event).
		Order("last_sync_at DESC").
		First(&syncHistory).Error

	if err != nil {
		if err == gormlib.ErrRecordNotFound {
			return nil, nil // No sync history found
		}
		return nil, err
	}

	return syncHistory.LastSyncAt, nil
}

// GetLastSyncTimeForVAAndEvent retrieves the most recent sync timestamp for a specific VA and event
// Used for incremental syncs - only sync records modified since the last sync for this VA
func (r *SyncRepository) GetLastSyncTimeForVAAndEvent(ctx context.Context, vaID string, event string) (*time.Time, error) {
	var syncHistory gorm.VASyncHistory

	err := r.db.WithContext(ctx).
		Where("va_id = ? AND event = ?", vaID, event).
		Order("last_sync_at DESC").
		First(&syncHistory).Error

	if err != nil {
		if err == gormlib.ErrRecordNotFound {
			return nil, nil // No sync history found for this VA
		}
		return nil, err
	}

	return syncHistory.LastSyncAt, nil
}
