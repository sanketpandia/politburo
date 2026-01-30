package pilots

import (
	"context"

	gormlib "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository handles pilot_at_synced table operations
type Repository struct {
	db *gormlib.DB
}

// NewRepository creates a new pilot repository
func NewRepository(db *gormlib.DB) *Repository {
	return &Repository{db: db}
}

// Upsert inserts or updates a pilot record from Airtable
// ON CONFLICT (server_id, at_id) DO UPDATE
func (r *Repository) Upsert(ctx context.Context, pilot *PilotATSyncedGORM) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "server_id"},
				{Name: "at_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"callsign", "registered"}),
		}).
		Create(pilot).Error
}

// FindByCallsign finds a pilot by VA ID and callsign (case-insensitive)
func (r *Repository) FindByCallsign(ctx context.Context, vaID string, callsign string) (*PilotATSyncedGORM, error) {
	var pilot PilotATSyncedGORM

	err := r.db.WithContext(ctx).
		Where("server_id = ? AND LOWER(callsign) = LOWER(?)", vaID, callsign).
		First(&pilot).Error

	if err != nil {
		if err == gormlib.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &pilot, nil
}

// GetUnlinkedUsers returns all users in va_user_roles who don't have airtable_pilot_id set
func (r *Repository) GetUnlinkedUsers(ctx context.Context, vaID string) ([]UnlinkedUser, error) {
	var users []UnlinkedUser

	err := r.db.WithContext(ctx).
		Table("va_user_roles").
		Where("va_id = ? AND callsign IS NOT NULL AND callsign != '' AND (airtable_pilot_id IS NULL OR airtable_pilot_id = '')", vaID).
		Select("id, callsign").
		Find(&users).Error

	if err != nil {
		return nil, err
	}

	return users, nil
}

// UpdateUserAirtableID updates the airtable_pilot_id for a user in va_user_roles
func (r *Repository) UpdateUserAirtableID(ctx context.Context, userRoleID string, airtableID string) error {
	return r.db.WithContext(ctx).
		Table("va_user_roles").
		Where("id = ?", userRoleID).
		Update("airtable_pilot_id", airtableID).Error
}

// FindByATID finds a Pilot by VA ID and Airtable ID
func (r *Repository) FindByATID(ctx context.Context, vaID string, atID string) (*PilotATSyncedGORM, error) {
	var pilot PilotATSyncedGORM

	err := r.db.WithContext(ctx).
		Where("server_id = ? AND at_id = ?", vaID, atID).
		First(&pilot).Error

	if err != nil {
		if err == gormlib.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &pilot, nil
}
