package jobs

import (
	"context"
	"fmt"
	"infinite-experiment/politburo/internal/common"
	"strings"
	"time"

	"go.uber.org/zap"
)

// SessionCacheJob syncs all Infinite Flight sessions/servers to cache
// Runs periodically to ensure fresh session data is available
type SessionCacheJob struct {
	liveAPISvc  *common.LiveAPIService
	redisSvc    *common.RedisCacheService
	logger      *zap.SugaredLogger
	lastRunTime time.Time
}

// NewSessionCacheJob creates a new session cache job
func NewSessionCacheJob(liveAPISvc *common.LiveAPIService, redisSvc *common.RedisCacheService, logger *zap.SugaredLogger) *SessionCacheJob {
	return &SessionCacheJob{
		liveAPISvc: liveAPISvc,
		redisSvc:   redisSvc,
		logger:     logger,
	}
}

// Run executes the session cache job
func (j *SessionCacheJob) Run(ctx context.Context) error {
	startTime := time.Now()
	j.logger.Info("Starting session cache job")

	// Fetch all servers from Infinite Flight Live API
	sessionsResp, err := j.liveAPISvc.GetSessions()
	if err != nil {
		j.logger.Errorw("Failed to get servers from Infinite Live API", "error", err)
		return fmt.Errorf("failed to get servers: %w", err)
	}

	sessionIDs := make([]string, 0, len(sessionsResp.Result))

	// Cache each server with 24-hour TTL
	for _, server := range sessionsResp.Result {
		sessionIDs = append(sessionIDs, server.ID)

		// Cache full server object
		cacheKey := fmt.Sprintf("if:session:%s", server.ID)
		j.redisSvc.Set(cacheKey, server, 24*time.Hour)

		// Cache session name separately for quick lookups
		nameKey := fmt.Sprintf("if:session:name:%s", server.ID)
		j.redisSvc.Set(nameKey, server.Name, 24*time.Hour)

		j.logger.Debugw("Cached session",
			"sessionID", server.ID,
			"sessionName", server.Name,
		)
	}

	// Cache the list of all session IDs for iteration by other jobs
	sessionListStr := strings.Join(sessionIDs, "|")
	j.redisSvc.Set("if:sessions", sessionListStr, 24*time.Hour)

	duration := time.Since(startTime)
	j.lastRunTime = time.Now()

	j.logger.Infow("Session cache job completed",
		"sessionCount", len(sessionIDs),
		"duration", duration,
	)

	return nil
}

// RunScheduled runs the session cache job on a schedule
func (j *SessionCacheJob) RunScheduled(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := j.Run(ctx); err != nil {
				j.logger.Errorw("Session cache job failed", "error", err)
			}
		case <-ctx.Done():
			j.logger.Info("Session cache job stopped")
			return
		}
	}
}

// GetLastRunTime returns the last time this job ran successfully
func (j *SessionCacheJob) GetLastRunTime() time.Time {
	return j.lastRunTime
}