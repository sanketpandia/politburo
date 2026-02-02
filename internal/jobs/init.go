package jobs

import (
	"context"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/flights"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// JobsContainer holds all initialized non-sync jobs
// Note: Sync jobs (routes, PIREPs) are now in sync.Container
type JobsContainer struct {
	SessionCache *SessionCacheJob
	FlightsCache *flights.CacheJob
}

// InitializeJobs initializes and starts all non-sync background jobs
// Note: Sync jobs (routes, PIREPs) are initialized via sync.InitializeJobs
func InitializeJobs(
	ctx context.Context,
	db *gorm.DB,
	cache cache.CacheInterface,
	configRepo *repositories.DataProviderConfigRepo,
	syncHistoryRepo *repositories.VASyncHistoryRepo, // Still needed by pilot sync job
	vaConfigService *common.VAConfigService,
	liveAPIService *common.LiveAPIService,
	redisCache *cache.RedisCacheService,
) *JobsContainer {
	// Initialize pilot sync job (syncs pilots from Airtable every 10 minutes)
	// pilotSyncJob := pilots.NewSyncJob(
	// 	db,
	// 	cache,
	// 	configRepo,
	// 	syncHistoryRepo,
	// 	pilotRepo,
	// 	vaConfigService,
	// )

	// Initialize PIREP backfill job (backfills missing pilot/route data every 15 minutes)
	// // TODO: Update PIREPBackfill to accept sync.Repository instead of RouteATSyncedRepo
	// pirepBackfillJob := workers.NewPIREPBackfill(
	// 	db,
	// 	cache,
	// 	repositories.RouteATSyncedRepo{}, // Empty struct - TODO: migrate to sync.Repository
	// 	*pilotRepo,                       // Pilots repository (dereferenced)
	// )

	// Start scheduled jobs in background
	// go pilotSyncJob.RunScheduled(ctx, 10*time.Minute)
	// go pirepBackfillJob.RunScheduled(ctx, 10*time.Minute)

	// Initialize and start cache jobs (if Redis is enabled)
	var sessionCacheJob *SessionCacheJob
	var flightsCacheJob *flights.CacheJob

	if redisCache != nil && liveAPIService != nil {
		// Get logger for cache jobs
		logger := logging.GetLogger()

		// Initialize session cache job (runs every 5 minutes)
		sessionCacheJob = NewSessionCacheJob(liveAPIService, redisCache, logger)

		// Run session cache job immediately to populate cache on startup
		if err := sessionCacheJob.Run(ctx); err != nil {
			logger.Warn("Initial session cache job failed", zap.Error(err))
		}

		// Start scheduled session cache job
		go sessionCacheJob.RunScheduled(ctx, 5*time.Minute)
		logger.Info("Session cache job scheduled (every 5 minutes)")

		// Initialize flights cache job (runs every 1 minute)
		flightsCacheJob = flights.NewCacheJob(liveAPIService, redisCache, vaConfigService, logger)

		// Run flights cache job immediately (depends on session cache)
		if err := flightsCacheJob.Run(ctx); err != nil {
			logger.Warn("Initial flights cache job failed", zap.Error(err))
		}

		// Start scheduled flights cache job
		go flightsCacheJob.RunScheduled(ctx, 1*time.Minute)
		logger.Info("Flights cache job scheduled (every 1 minute)")
	}

	return &JobsContainer{
		SessionCache: sessionCacheJob,
		FlightsCache: flightsCacheJob,
	}
}
