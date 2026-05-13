package sessions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
)

// CacheJob syncs all Infinite Flight sessions/servers to Redis cache
// Runs periodically to ensure fresh session data is available
type CacheJob struct {
	liveAPIClient *liveapi.Client
	redisCache    *cache.RedisCacheService
	metrics       *metrics.MetricsRegistry
}

// NewCacheJob creates a new session cache job
func NewCacheJob(liveAPIClient *liveapi.Client, redisCache *cache.RedisCacheService, metricsReg *metrics.MetricsRegistry) *CacheJob {
	return &CacheJob{
		liveAPIClient: liveAPIClient,
		redisCache:    redisCache,
		metrics:       metricsReg,
	}
}

// Name returns the job name for the scheduler
func (j *CacheJob) Name() string {
	return "SessionCacheJob"
}

// Run executes the session cache job
func (j *CacheJob) Run(ctx context.Context) error {
	startTime := time.Now()
	logging.Info("Starting session cache job")
	defer func() {
		if j.metrics != nil {
			j.metrics.SyncJobDuration.WithLabelValues("session_cache_job", "liveapi", "session").Observe(time.Since(startTime).Seconds())
		}
	}()

	// Fetch all servers from Infinite Flight Live API
	sessionsResp, err := j.liveAPIClient.GetSessions()
	if err != nil {
		logging.Error("Failed to get servers from Infinite Live API", "error", err)
		return fmt.Errorf("failed to get servers: %w", err)
	}

	sessionIDs := make([]string, 0, len(sessionsResp.Result))

	// Cache each session with 24-hour TTL
	for _, server := range sessionsResp.Result {
		sessionIDs = append(sessionIDs, server.ID)

		// Cache full session object using helper function
		cacheKey := cache.SessionKey(server.ID)
		j.redisCache.Set(cacheKey, server, 24*time.Hour)

		// Cache session name separately for quick lookups
		nameKey := cache.SessionNameKey(server.ID)
		j.redisCache.Set(nameKey, server.Name, 24*time.Hour)

		logging.Debug("Cached session",
			"sessionID", server.ID,
			"sessionName", server.Name,
		)
	}

	// Cache the list of all session IDs for iteration by other jobs
	sessionListStr := strings.Join(sessionIDs, "|")
	j.redisCache.Set(cache.KeySessionList, sessionListStr, 24*time.Hour)

	if j.metrics != nil {
		j.metrics.CacheSize.WithLabelValues("session_cache").Set(float64(len(sessionIDs)))
	}

	logging.Info("Session cache job completed",
		"sessionCount", len(sessionIDs),
		"duration", time.Since(startTime),
	)

	return nil
}
