package flights

import (
	"context"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/queue"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"infinite-experiment/politburo/internal/platform/va"
	"infinite-experiment/politburo/internal/sessions"
	"time"
)

// CacheJob syncs live flight data with intelligent caching and waypoint tracking
// Implements the simplified 3-key cache architecture with CompleteFlight objects
type CacheJob struct {
	liveAPI           *liveapi.Client
	redisCache        *cache.RedisCacheService
	redisQueue        *queue.RedisQueueService
	vaRepo            *va.Repository
	aircraftSvc       *aircraft.Service
	metrics           *metrics.MetricsRegistry
	vaPatterns        []VAPattern
	lastPatternUpdate time.Time
}

// NewCacheJob creates a new flights cache job
func NewCacheJob(
	liveAPI *liveapi.Client,
	redisCache *cache.RedisCacheService,
	redisQueue *queue.RedisQueueService,
	vaRepo *va.Repository,
	aircraftSvc *aircraft.Service,
	metricsReg *metrics.MetricsRegistry,
) *CacheJob {
	return &CacheJob{
		liveAPI:           liveAPI,
		redisCache:        redisCache,
		redisQueue:        redisQueue,
		vaRepo:            vaRepo,
		aircraftSvc:       aircraftSvc,
		metrics:           metricsReg,
		lastPatternUpdate: time.Now().Add(-10 * time.Minute), // Force initial refresh
	}
}

// Name returns the job name for the scheduler
func (j *CacheJob) Name() string {
	return "FlightsCacheJob"
}

// Run executes the flights cache job
// This job tracks live flights for all active VAs that have callsign prefix/suffix configured.
// It does NOT require Airtable to be enabled - only callsign configuration is needed.
func (j *CacheJob) Run(ctx context.Context) error {
	startTime := time.Now()
	logging.Info("Starting flights cache job")
	defer func() {
		if j.metrics != nil {
			j.metrics.SyncJobDuration.WithLabelValues("flights_cache_job", "liveapi", "flight").Observe(time.Since(startTime).Seconds())
		}
	}()

	// 1. Refresh VA patterns every 5 minutes (only depends on callsign config, not Airtable)
	if err := j.refreshVAPatterns(ctx); err != nil {
		logging.Warn("Failed to refresh VA patterns, using cached patterns", "error", err)
	}

	sessionIDs, err := sessions.GetSessionIDs(j.redisCache)
	if err != nil {
		j.recordCacheJobFailure("cache_read")
		return err
	}
	if len(sessionIDs) == 0 {
		logging.Warn("No sessions found in cache, skipping flights cache")
		return nil
	}

	result := NewFlightAggregation()
	for _, sessionID := range sessionIDs {
		result.Merge(j.processSession(ctx, sessionID))
	}

	j.storeSessionFlightIndexes(result.SessionFlights)
	j.storeVAFlightIndexes(result.VAFlights)
	j.recordCacheJobProcessed("session", "processed", float64(result.ProcessedSessions))
	j.recordCacheJobProcessed("flight", "cached", float64(result.TotalFlights))
	j.setCacheSize("live_flights", result.TotalFlights)
	j.setCacheSize("live_session_flight_indexes", len(result.SessionFlights))
	j.setCacheSize("live_va_flight_indexes", len(result.VAFlights))

	duration := time.Since(startTime)
	logging.Info("Flights cache job completed",
		"totalSessions", result.ProcessedSessions,
		"totalFlights", result.TotalFlights,
		"duration", duration,
	)

	return nil
}

func (j *CacheJob) recordCacheJobProcessed(entityType string, status string, count float64) {
	if j.metrics == nil {
		return
	}
	j.metrics.SyncJobRecordsProcessed.WithLabelValues("flights_cache_job", "liveapi", entityType, "_", status).Add(count)
}

func (j *CacheJob) recordCacheJobFailure(errorType string) {
	if j.metrics == nil {
		return
	}
	j.metrics.SyncJobRecordsFailed.WithLabelValues("flights_cache_job", "liveapi", "flight", "_", errorType).Inc()
}

func (j *CacheJob) setCacheSize(cacheName string, count int) {
	if j.metrics == nil {
		return
	}
	j.metrics.CacheSize.WithLabelValues(cacheName).Set(float64(count))
}
