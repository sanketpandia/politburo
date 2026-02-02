package pilots

import (
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/platform/memberships"

	"gorm.io/gorm"
)

type Service struct {
	cache  *cache.CacheService
	gormDB *gorm.DB
	repo   *memberships.Repository
}
