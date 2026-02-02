package apikeys

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new GORM-based API keys repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetStatus retrieves an API key by ID and returns its status
func (r *Repository) GetStatus(ctx context.Context, key string) (*ApiKey, error) {
	var apiKey ApiKey

	err := r.db.WithContext(ctx).
		Where("id = ?", key).
		First(&apiKey).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("failed to fetch API key: %w", err)
	}

	return &apiKey, nil
}
