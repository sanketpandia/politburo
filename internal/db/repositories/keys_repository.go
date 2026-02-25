package repositories

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/models/entities"
	gormModels "infinite-experiment/politburo/internal/models/gorm"

	"gorm.io/gorm"
)

type KeysRepo struct {
	db *gorm.DB
}

func NewApiKeysRepo(db *gorm.DB) *KeysRepo {
	return &KeysRepo{db: db}
}

func (r *KeysRepo) GetStatus(ctx context.Context, key string) (*entities.ApiKey, error) {
	var gormKey gormModels.ApiKey

	err := r.db.WithContext(ctx).
		Where("id = ?", key).
		First(&gormKey).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("failed to fetch API key: %w", err)
	}

	// Convert GORM model to entity
	return &entities.ApiKey{
		ApiKey: gormKey.ID,
		Status: gormKey.Status,
	}, nil
}
