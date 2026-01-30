package airports

import (
	"context"

	"gorm.io/gorm"
)

// Repository handles airport table operations
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new airport repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindByICAO finds an airport by ICAO code (case-insensitive)
func (r *Repository) FindByICAO(ctx context.Context, icao string) (*Airport, error) {
	var airport Airport

	err := r.db.WithContext(ctx).
		Where("UPPER(icao) = UPPER(?)", icao).
		First(&airport).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &airport, nil
}

// FindByIATA finds an airport by IATA code (case-insensitive)
func (r *Repository) FindByIATA(ctx context.Context, iata string) (*Airport, error) {
	var airport Airport

	err := r.db.WithContext(ctx).
		Where("UPPER(iata) = UPPER(?)", iata).
		First(&airport).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &airport, nil
}

// BatchInsert inserts multiple airports
func (r *Repository) BatchInsert(ctx context.Context, airports []Airport) error {
	return r.db.WithContext(ctx).
		CreateInBatches(airports, 100).Error
}

// DeleteAll deletes all airports (useful for re-importing)
func (r *Repository) DeleteAll(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("1 = 1").
		Delete(&Airport{}).Error
}

// Count returns total number of airports
func (r *Repository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Airport{}).Count(&count).Error
	return count, err
}
