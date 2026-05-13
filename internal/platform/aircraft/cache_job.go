package aircraft

import (
	"context"
	"fmt"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
)

// CacheJob syncs aircraft and livery data from Infinite Flight API to Redis cache
// Runs periodically to ensure fresh aircraft/livery data is available
type CacheJob struct {
	liveAPIClient *liveapi.Client
	redisCache    *cache.RedisCacheService
	metrics       *metrics.MetricsRegistry
}

// NewCacheJob creates a new aircraft cache job
func NewCacheJob(liveAPIClient *liveapi.Client, redisCache *cache.RedisCacheService, metricsReg *metrics.MetricsRegistry) *CacheJob {
	return &CacheJob{
		liveAPIClient: liveAPIClient,
		redisCache:    redisCache,
		metrics:       metricsReg,
	}
}

// Name returns the job name for the scheduler
func (j *CacheJob) Name() string {
	return "AircraftCacheJob"
}

// Run executes the aircraft cache job
func (j *CacheJob) Run(ctx context.Context) error {
	startTime := time.Now()
	logging.Info("Starting aircraft cache job")
	defer func() {
		if j.metrics != nil {
			j.metrics.SyncJobDuration.WithLabelValues("aircraft_cache_job", "liveapi", "aircraft").Observe(time.Since(startTime).Seconds())
		}
	}()

	// Fetch aircraft and liveries from Infinite Flight API
	resp, _, err := j.liveAPIClient.GetAircraftLiveries()
	if err != nil {
		logging.Error("Failed to fetch aircraft/liveries from Infinite Flight API", "error", err)
		return fmt.Errorf("failed to fetch aircraft/liveries: %w", err)
	}

	aircraftCount := make(map[string]int) // Track unique aircraft
	liveryCount := 0

	// Cache each aircraft and livery with 24-hour TTL
	for _, livery := range resp.Liveries {
		// Cache aircraft data
		aircraftKey := cache.AircraftKey(livery.AircraftID)
		j.redisCache.Set(aircraftKey, livery.AircraftName, 24*time.Hour)
		aircraftCount[livery.AircraftID]++

		// Cache livery data
		liveryKey := cache.LiveryKey(livery.LiveryId)
		j.redisCache.Set(liveryKey, livery.LiveryName, 24*time.Hour)
		liveryCount++

		logging.Debug("Cached aircraft/livery",
			"aircraftID", livery.AircraftID,
			"aircraftName", livery.AircraftName,
			"liveryID", livery.LiveryId,
			"liveryName", livery.LiveryName,
		)
	}

	if j.metrics != nil {
		j.metrics.SyncJobRecordsProcessed.WithLabelValues("aircraft_cache_job", "liveapi", "aircraft", "_", "success").Add(float64(liveryCount))
	}

	logging.Info("Aircraft cache job completed",
		"uniqueAircraft", len(aircraftCount),
		"totalLiveries", liveryCount,
		"duration", time.Since(startTime),
	)

	return nil
}
