package jobs

import (
	"infinite-experiment/politburo/internal/cache"
	"infinite-experiment/politburo/internal/infiniteflight"
	"infinite-experiment/politburo/internal/jobs/schedules"
	sessionsjob "infinite-experiment/politburo/internal/jobs/sessions"
	"infinite-experiment/politburo/internal/scheduler"
)

// Register is the single composition point for scheduled work.
func Register(jobScheduler *scheduler.Scheduler, sessionsClient infiniteflight.SessionsClient, cacheStore cache.Store) error {
	return jobScheduler.Register(sessionsjob.New(sessionsClient, cacheStore), schedules.SessionsSync)
}
