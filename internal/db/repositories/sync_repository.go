package repositories

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/models/entities"
	gormModels "infinite-experiment/politburo/internal/models/gorm"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PilotATSyncedGORM is a local copy to avoid circular imports
// This should be removed when AtSyncService is fully migrated
type PilotATSyncedGORM struct {
	ID         string `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	ATID       string `gorm:"column:at_id;type:varchar(20);not null"`
	Callsign   string `gorm:"column:callsign;type:varchar(20)"`
	Registered bool   `gorm:"column:registered;default:false"`
	ServerID   string `gorm:"column:server_id;type:uuid"`
}

func (PilotATSyncedGORM) TableName() string {
	return "pilot_at_synced"
}

// PilotATSynced is a local copy to avoid circular imports
// This should be removed when AtSyncService is fully migrated
type PilotATSynced struct {
	ATID       string
	Callsign   string
	Registered bool
	ServerID   string
}

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
	pilot *PilotATSynced) error {

	gormPilot := &PilotATSyncedGORM{
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
