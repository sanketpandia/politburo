package jobs

import (
	"context"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/logging"
	"infinite-experiment/politburo/internal/workers"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// JobsContainer holds all initialized jobs
type JobsContainer struct {
	PilotSync     *PilotSyncJob
	RouteSync     *RouteSyncJob
	PirepSync     *PirepSyncJob
	PIREPBackfill *workers.PIREPBackfill
	SessionCache  *SessionCacheJob
	FlightsCache  *FlightsCacheJob
}

// InitializeJobs initializes and starts all background jobs
func InitializeJobs(
	ctx context.Context,
	db *gorm.DB,
	cache common.CacheInterface,
	configRepo *repositories.DataProviderConfigRepo,
	syncHistoryRepo *repositories.VASyncHistoryRepo,
	pilotATSyncedRepo *repositories.PilotATSyncedRepo,
	routeATSyncedRepo *repositories.RouteATSyncedRepo,
	pirepATSyncedRepo *repositories.PirepATSyncedRepo,
	airportIcaoRepo *repositories.AirportRepository,
	vaConfigService *common.VAConfigService,
	redisQueue *common.RedisQueueService,
	liveAPIService *common.LiveAPIService,
	redisCache *common.RedisCacheService,
) *JobsContainer {
	// Initialize pilot sync job (syncs pilots from Airtable every hour)
	pilotSyncJob := NewPilotSyncJob(
		db,
		cache,
		configRepo,
		syncHistoryRepo,
		pilotATSyncedRepo,
		vaConfigService,
	)

	// Initialize route sync job (syncs routes from Airtable every hour)
	routeSyncJob := NewRouteSyncJob(
		db,
		cache,
		configRepo,
		syncHistoryRepo,
		routeATSyncedRepo,
		airportIcaoRepo,
	)

	// Initialize PIREP sync job (syncs PIREPs from Airtable every hour)
	pirepSyncJob := NewPirepSyncJob(
		db,
		cache,
		configRepo,
		syncHistoryRepo,
		pirepATSyncedRepo,
		redisQueue,
	)

	// Initialize PIREP backfill job (backfills missing pilot/route data every 15 minutes)
	pirepBackfillJob := workers.NewPIREPBackfill(
		db,
		cache,
		*routeATSyncedRepo,
		*pilotATSyncedRepo,
	)

	// Start scheduled sync jobs in background (all run every 10 minutes)
	go pilotSyncJob.RunScheduled(ctx, 10*time.Minute)
	go routeSyncJob.RunScheduled(ctx, 10*time.Minute)
	go pirepSyncJob.RunScheduled(ctx, 10*time.Minute)
	go pirepBackfillJob.RunScheduled(ctx, 10*time.Minute)

	// Initialize and start cache jobs (if Redis is enabled)
	var sessionCacheJob *SessionCacheJob
	var flightsCacheJob *FlightsCacheJob

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
		flightsCacheJob = NewFlightsCacheJob(liveAPIService, redisCache, vaConfigService, logger)

		// Run flights cache job immediately (depends on session cache)
		if err := flightsCacheJob.Run(ctx); err != nil {
			logger.Warn("Initial flights cache job failed", zap.Error(err))
		}

		// Start scheduled flights cache job
		go flightsCacheJob.RunScheduled(ctx, 1*time.Minute)
		logger.Info("Flights cache job scheduled (every 1 minute)")
	}

	return &JobsContainer{
		PilotSync:     pilotSyncJob,
		RouteSync:     routeSyncJob,
		PirepSync:     pirepSyncJob,
		PIREPBackfill: pirepBackfillJob,
		SessionCache:  sessionCacheJob,
		FlightsCache:  flightsCacheJob,
	}
}
