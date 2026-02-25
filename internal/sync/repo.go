package sync

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository handles all sync-related database operations
// Consolidates route, PIREP, and sync history operations
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new sync repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ====================
// Route Operations
// ====================

// UpsertRoute inserts or updates a route record from Airtable
// ON CONFLICT (server_id, at_id) DO UPDATE
func (r *Repository) UpsertRoute(ctx context.Context, route *RouteATSynced) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "server_id"},
				{Name: "at_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"origin", "destination", "route", "updated_at"}),
		}).
		Create(route).Error
}

// FindRouteByATID finds a route by VA ID and Airtable ID
func (r *Repository) FindRouteByATID(ctx context.Context, vaID string, atID string) (*RouteATSynced, error) {
	var route RouteATSynced

	err := r.db.WithContext(ctx).
		Where("server_id = ? AND at_id = ?", vaID, atID).
		First(&route).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &route, nil
}

// GetAllRoutesByVA returns all routes for a specific VA
func (r *Repository) GetAllRoutesByVA(ctx context.Context, vaID string) ([]RouteATSynced, error) {
	var routes []RouteATSynced

	err := r.db.WithContext(ctx).
		Where("server_id = ?", vaID).
		Order("origin ASC, destination ASC").
		Find(&routes).Error

	if err != nil {
		return nil, err
	}

	return routes, nil
}

// CountRoutesByVA returns the total number of routes for a specific VA
func (r *Repository) CountRoutesByVA(ctx context.Context, vaID string) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&RouteATSynced{}).
		Where("server_id = ?", vaID).
		Count(&count).Error

	return count, err
}

// FindRouteByName finds a route by VA ID and route name (case-insensitive)
func (r *Repository) FindRouteByName(ctx context.Context, vaID string, routeName string) (*RouteATSynced, error) {
	var route RouteATSynced

	err := r.db.WithContext(ctx).
		Where("server_id = ? AND LOWER(route) = LOWER(?)", vaID, routeName).
		First(&route).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &route, nil
}

// ====================
// Sync History Operations
// ====================

// RecordSync records a successful sync operation for a VA
// Simple method: just post the server ID (vaID) and sync event
func (r *Repository) RecordSync(ctx context.Context, vaID string, event string) error {
	now := time.Now()

	syncHistory := VASyncHistory{
		VAID:       vaID,
		Event:      event,
		LastSyncAt: &now,
	}

	// Upsert: if record exists for this VA and event, update last_sync_at
	// Otherwise, create new record
	err := r.db.WithContext(ctx).
		Where("va_id = ? AND event = ?", vaID, event).
		Assign(VASyncHistory{LastSyncAt: &now}).
		FirstOrCreate(&syncHistory).Error

	return err
}

// GetLastSyncTimeForEvent retrieves the most recent sync timestamp across all VAs for a specific event
// Used to check if we should run initial sync on app restart
func (r *Repository) GetLastSyncTimeForEvent(ctx context.Context, event string) (*time.Time, error) {
	var syncHistory VASyncHistory

	err := r.db.WithContext(ctx).
		Where("event = ?", event).
		Order("last_sync_at DESC").
		First(&syncHistory).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No sync history found
		}
		return nil, err
	}

	return syncHistory.LastSyncAt, nil
}
