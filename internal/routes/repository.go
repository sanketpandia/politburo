package routes

import (
	"context"
	"log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository handles route_at_synced table operations
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new route repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Upsert inserts or updates a route record from Airtable
// ON CONFLICT (server_id, at_id) DO UPDATE
func (r *Repository) Upsert(ctx context.Context, route *RouteATSyncedGORM) error {
	// Check if record already exists with this (server_id, at_id) combination
	var existing RouteATSyncedGORM
	err := r.db.WithContext(ctx).
		Where("server_id = ? AND at_id = ?", route.ServerID, route.ATID).
		First(&existing).Error

	isUpdate := err == nil // Record exists

	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "server_id"},
				{Name: "at_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"origin", "destination", "route", "origin_lat", "origin_lon", "destination_lat", "destination_lon", "updated_at"}),
		}).
		Create(route)

	if result.Error != nil {
		return result.Error
	}

	// Log the result for debugging
	action := "INSERT"
	if isUpdate {
		action = "UPDATE"
	}
	log.Printf("[RouteRepo] Upsert %s - RowsAffected: %d, ATID: %s, Route: %s, ServerID: %s",
		action, result.RowsAffected, route.ATID, route.Route, route.ServerID)

	return nil
}

// FindByATID finds a route by VA ID and Airtable ID
func (r *Repository) FindByATID(ctx context.Context, vaID string, atID string) (*RouteATSyncedGORM, error) {
	var route RouteATSyncedGORM

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
