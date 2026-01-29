package repositories

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/models/entities"
	gormModels "infinite-experiment/politburo/internal/models/gorm"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SyncRepository struct {
	db *gorm.DB
}

func NewSyncRepository(db *gorm.DB) *SyncRepository {
	return &SyncRepository{
		db: db,
	}
}

func (svc SyncRepository) UpsertPilot(
	ctx context.Context,
	pilot *entities.PilotATSynced) error {

	gormPilot := &gormModels.PilotATSynced{
		ATID:       pilot.ATID,
		Callsign:   pilot.Callsign,
		Registered: pilot.Registered,
		ServerID:   pilot.ServerID,
	}

	err := svc.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "server_id"}, {Name: "at_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"callsign", "registered"}),
		}).
		Create(gormPilot).Error

	if err != nil {
		return fmt.Errorf("failed to upsert pilot: %w", err)
	}

	return nil
}

func (svc SyncRepository) UpsertRoute(
	ctx context.Context,
	route *entities.RouteATSynced) error {

	gormRoute := &gormModels.RouteATSynced{
		ATID:        route.ATID,
		Origin:      route.Origin,
		Destination: route.Destination,
		ServerID:    route.ServerID,
		Route:       route.Route,
	}

	err := svc.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "server_id"}, {Name: "at_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"origin", "destination", "route"}),
		}).
		Create(gormRoute).Error

	if err != nil {
		return fmt.Errorf("failed to upsert route: %w", err)
	}

	return nil
}
