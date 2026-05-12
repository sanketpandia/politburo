package pireps

import (
	"context"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository handles pirep_at_synced table operations
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new PIREP repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Upsert inserts or updates a PIREP record from Airtable
// ON CONFLICT (server_id, at_id) DO UPDATE
func (r *Repository) Upsert(ctx context.Context, pirep *PirepATSynced) error {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "server_id"},
				{Name: "at_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"route", "flight_mode", "flight_time", "pilot_callsign",
				"aircraft", "livery", "route_at_id", "pilot_at_id", "at_created_time", "updated_at",
			}),
		}).
		Create(pirep).Error; err != nil {
		logging.Error("Failed to upsert PIREP", "at_id", pirep.ATID, "va_id", pirep.ServerID, "error", err)
		return err
	}
	return nil
}

// FindByATID finds a PIREP by VA ID and Airtable ID
func (r *Repository) FindByATID(ctx context.Context, vaID string, atID string) (*PirepATSynced, error) {
	var pirep PirepATSynced

	err := r.db.WithContext(ctx).
		Where("server_id = ? AND at_id = ?", vaID, atID).
		First(&pirep).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &pirep, nil
}

// GetByPilot returns all PIREPs for a specific pilot callsign, ordered by creation time descending
func (r *Repository) GetByPilot(ctx context.Context, vaID string, pilotCallsign string, limit int) ([]PirepATSynced, error) {
	var pireps []PirepATSynced

	query := r.db.WithContext(ctx).
		Where("server_id = ? AND pilot_callsign = ?", vaID, pilotCallsign).
		Order("at_created_time DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&pireps).Error
	if err != nil {
		return nil, err
	}

	return pireps, nil
}

// GetByVA returns all PIREPs for a specific VA, ordered by creation time descending
func (r *Repository) GetByVA(ctx context.Context, vaID string, limit int) ([]PirepATSynced, error) {
	var pireps []PirepATSynced

	query := r.db.WithContext(ctx).
		Where("server_id = ?", vaID).
		Order("at_created_time DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&pireps).Error
	if err != nil {
		return nil, err
	}

	return pireps, nil
}

// CountByVA returns the total number of PIREPs for a specific VA
func (r *Repository) CountByVA(ctx context.Context, vaID string) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&PirepATSynced{}).
		Where("server_id = ?", vaID).
		Count(&count).Error

	return count, err
}

// CountByPilot returns the total number of PIREPs for a specific pilot
func (r *Repository) CountByPilot(ctx context.Context, vaID string, pilotCallsign string) (int64, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&PirepATSynced{}).
		Where("server_id = ? AND pilot_callsign = ?", vaID, pilotCallsign).
		Count(&count).Error

	return count, err
}

// FindByATIDs finds PIREPs by VA ID and a list of Airtable IDs, ordered by creation time descending
func (r *Repository) FindByATIDs(ctx context.Context, vaID string, atIDs []string, limit int) ([]PirepATSynced, error) {
	var pireps []PirepATSynced

	if len(atIDs) == 0 {
		return pireps, nil
	}

	query := r.db.WithContext(ctx).
		Where("server_id = ? AND at_id IN ?", vaID, atIDs).
		Order("at_created_time DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&pireps).Error
	if err != nil {
		return nil, err
	}

	return pireps, nil
}

// GetMaxATCreatedTime returns the maximum at_created_time for a specific VA
// This is useful for checking the latest record that was synced
// Note: at_created_time is the Airtable record creation time, NOT the "Last Modified" field
func (r *Repository) GetMaxATCreatedTime(ctx context.Context, vaID string) (*time.Time, error) {
	var maxTime *time.Time

	err := r.db.WithContext(ctx).
		Model(&PirepATSynced{}).
		Where("server_id = ? AND at_created_time IS NOT NULL", vaID).
		Select("MAX(at_created_time)").
		Scan(&maxTime).Error

	if err != nil {
		return nil, err
	}

	return maxTime, nil
}
