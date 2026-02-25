package sync

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/db/repositories"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

// Container holds all initialized sync jobs
type Container struct {
	RouteSync *RouteSyncJob
}

// InitializeJobs initializes and starts all sync jobs
func InitializeJobs(
	ctx context.Context,
	db *gorm.DB,
	cache cache.CacheInterface,
	vaSvc *platformVA.Service,
	syncRepo *Repository,
	airportRepo *repositories.AirportRepository,
	logger *zap.Logger,
) *Container {
	// Initialize route sync job (syncs routes from Airtable every 10 minutes)
	routeSyncJob := NewRouteSyncJob(
		db,
		cache,
		vaSvc,
		syncRepo,
		airportRepo,
	)

	// Start scheduled sync jobs in background (all run every 10 minutes)
	go routeSyncJob.RunScheduled(ctx, 10*time.Minute)

	logger.Info("Sync jobs initialized and scheduled",
		zap.String("route_interval", "10m"),
	)

	return &Container{
		RouteSync: routeSyncJob,
	}
}
