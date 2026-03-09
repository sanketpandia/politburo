package aircraft

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository handles all aircraft livery and mapping database operations
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new aircraft repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ====================
// AircraftLivery Operations
// ====================

// GetByLiveryID fetches a single active livery by ID
func (r *Repository) GetByLiveryID(ctx context.Context, liveryID string) (*AircraftLivery, error) {
	var livery AircraftLivery

	err := r.db.WithContext(ctx).
		Where("livery_id = ? AND is_active = ?", liveryID, true).
		First(&livery).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("livery not found: %s", liveryID)
		}
		return nil, fmt.Errorf("failed to fetch livery: %w", err)
	}

	return &livery, nil
}

// GetAllActive fetches all active liveries
func (r *Repository) GetAllActive(ctx context.Context) ([]AircraftLivery, error) {
	var liveries []AircraftLivery

	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Find(&liveries).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch active liveries: %w", err)
	}

	return liveries, nil
}

// UpsertBatch performs bulk upsert with conflict resolution on livery_id
func (r *Repository) UpsertBatch(ctx context.Context, liveries []AircraftLivery) error {
	if len(liveries) == 0 {
		return nil
	}

	// Set sync timestamp for all records
	now := time.Now()
	for i := range liveries {
		liveries[i].LastSyncedAt = now
		liveries[i].UpdatedAt = now
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "livery_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"aircraft_id",
				"aircraft_name",
				"livery_name",
				"is_active",
				"updated_at",
				"last_synced_at",
			}),
		}).
		Create(&liveries).Error

	if err != nil {
		return fmt.Errorf("failed to upsert liveries batch: %w", err)
	}

	return nil
}

// GetLastSyncTime returns the most recent sync timestamp
func (r *Repository) GetLastSyncTime(ctx context.Context) (time.Time, error) {
	var result struct {
		LastSynced time.Time
	}

	err := r.db.WithContext(ctx).
		Model(&AircraftLivery{}).
		Select("MAX(last_synced_at) as last_synced").
		Scan(&result).Error

	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last sync time: %w", err)
	}

	return result.LastSynced, nil
}

// MarkInactive marks liveries as inactive by their IDs
func (r *Repository) MarkInactive(ctx context.Context, liveryIDs []string) error {
	if len(liveryIDs) == 0 {
		return nil
	}

	err := r.db.WithContext(ctx).
		Model(&AircraftLivery{}).
		Where("livery_id IN ?", liveryIDs).
		Updates(map[string]interface{}{
			"is_active":      false,
			"updated_at":     time.Now(),
			"last_synced_at": time.Now(),
		}).Error

	if err != nil {
		return fmt.Errorf("failed to mark liveries inactive: %w", err)
	}

	return nil
}

// GetLiveryMap returns a map of liveryID -> AircraftLivery for fast lookups
func (r *Repository) GetLiveryMap(ctx context.Context) (map[string]AircraftLivery, error) {
	liveries, err := r.GetAllActive(ctx)
	if err != nil {
		return nil, err
	}

	liveryMap := make(map[string]AircraftLivery, len(liveries))
	for _, livery := range liveries {
		liveryMap[livery.LiveryID] = livery
	}

	return liveryMap, nil
}

// GetUniqueAircraftNames returns distinct aircraft names from active liveries
func (r *Repository) GetUniqueAircraftNames(ctx context.Context) ([]string, error) {
	var names []string

	err := r.db.WithContext(ctx).
		Model(&AircraftLivery{}).
		Where("is_active = ?", true).
		Distinct("aircraft_name").
		Order("aircraft_name ASC").
		Pluck("aircraft_name", &names).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch unique aircraft names: %w", err)
	}

	return names, nil
}

// GetUniqueLiveryNames returns distinct livery names (airlines) from active liveries
func (r *Repository) GetUniqueLiveryNames(ctx context.Context) ([]string, error) {
	var names []string

	err := r.db.WithContext(ctx).
		Model(&AircraftLivery{}).
		Where("is_active = ?", true).
		Distinct("livery_name").
		Order("livery_name ASC").
		Pluck("livery_name", &names).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch unique livery names: %w", err)
	}

	return names, nil
}

// GetLiveriesByAircraftName finds all active liveries with matching aircraft name
func (r *Repository) GetLiveriesByAircraftName(ctx context.Context, aircraftName string) ([]AircraftLivery, error) {
	var liveries []AircraftLivery

	err := r.db.WithContext(ctx).
		Where("aircraft_name = ? AND is_active = ?", aircraftName, true).
		Find(&liveries).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch liveries by aircraft name: %w", err)
	}

	return liveries, nil
}

// GetLiveriesByLiveryName finds all active liveries with matching livery name
func (r *Repository) GetLiveriesByLiveryName(ctx context.Context, liveryName string) ([]AircraftLivery, error) {
	var liveries []AircraftLivery

	err := r.db.WithContext(ctx).
		Where("livery_name = ? AND is_active = ?", liveryName, true).
		Find(&liveries).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch liveries by livery name: %w", err)
	}

	return liveries, nil
}

// ====================
// LiveryAirtableMapping Operations
// ====================

// GetMapping retrieves a single mapping by VA ID, livery ID, and field type
func (r *Repository) GetMapping(ctx context.Context, vaID, liveryID, fieldType string) (*LiveryAirtableMapping, error) {
	var mapping LiveryAirtableMapping

	err := r.db.WithContext(ctx).
		Where("va_id = ? AND livery_id = ? AND field_type = ? AND is_active = ?",
			vaID, liveryID, fieldType, true).
		First(&mapping).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("mapping not found: va_id=%s, livery_id=%s, field_type=%s", vaID, liveryID, fieldType)
		}
		return nil, fmt.Errorf("failed to fetch mapping: %w", err)
	}

	return &mapping, nil
}

// GetMappingsByLivery retrieves both aircraft and airline mappings for a livery
// Returns a map with "aircraft" and "airline" keys containing their target values
func (r *Repository) GetMappingsByLivery(ctx context.Context, vaID, liveryID string) (map[string]string, error) {
	var mappings []LiveryAirtableMapping

	err := r.db.WithContext(ctx).
		Where("va_id = ? AND livery_id = ? AND is_active = ?",
			vaID, liveryID, true).
		Find(&mappings).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch livery mappings: %w", err)
	}

	result := make(map[string]string)
	for _, m := range mappings {
		result[m.FieldType] = m.TargetValue
	}

	return result, nil
}

// UpsertMappings performs an upsert operation for multiple mappings
// Uses the composite unique index (va_id, livery_id, field_type) for conflict resolution
func (r *Repository) UpsertMappings(ctx context.Context, mappings []LiveryAirtableMapping) error {
	if len(mappings) == 0 {
		return nil
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "va_id"},
				{Name: "livery_id"},
				{Name: "field_type"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"source_value",
				"target_value",
				"is_active",
				"updated_at",
			}),
		}).
		Create(&mappings).Error

	if err != nil {
		return fmt.Errorf("failed to upsert livery mappings: %w", err)
	}

	return nil
}

// GetMappingsByVA retrieves all active mappings for a specific VA
func (r *Repository) GetMappingsByVA(ctx context.Context, vaID string) ([]LiveryAirtableMapping, error) {
	var mappings []LiveryAirtableMapping

	err := r.db.WithContext(ctx).
		Where("va_id = ? AND is_active = ?", vaID, true).
		Find(&mappings).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch VA mappings: %w", err)
	}

	return mappings, nil
}

// GetMappingsByLiveryIDs retrieves mappings for multiple livery IDs in a VA
func (r *Repository) GetMappingsByLiveryIDs(ctx context.Context, vaID string, liveryIDs []string) (map[string]map[string]string, error) {
	var mappings []LiveryAirtableMapping

	err := r.db.WithContext(ctx).
		Where("va_id = ? AND livery_id IN ? AND is_active = ?",
			vaID, liveryIDs, true).
		Find(&mappings).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch livery mappings: %w", err)
	}

	// Build nested map: livery_id -> field_type -> target_value
	result := make(map[string]map[string]string)
	for _, m := range mappings {
		if result[m.LiveryID] == nil {
			result[m.LiveryID] = make(map[string]string)
		}
		result[m.LiveryID][m.FieldType] = m.TargetValue
	}

	return result, nil
}

// DeleteByLiveryID deletes all mappings for a livery (soft delete via is_active flag)
func (r *Repository) DeleteByLiveryID(ctx context.Context, vaID, liveryID string) error {
	err := r.db.WithContext(ctx).
		Model(&LiveryAirtableMapping{}).
		Where("va_id = ? AND livery_id = ?", vaID, liveryID).
		Update("is_active", false).Error

	if err != nil {
		return fmt.Errorf("failed to delete livery mappings: %w", err)
	}

	return nil
}
