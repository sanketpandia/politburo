package jobs

import (
	"infinite-experiment/politburo/internal/cache"
	"infinite-experiment/politburo/internal/infiniteflight"
	flightsjob "infinite-experiment/politburo/internal/jobs/flights"
	liveriesjob "infinite-experiment/politburo/internal/jobs/liveries"
	"infinite-experiment/politburo/internal/jobs/schedules"
	sessionsjob "infinite-experiment/politburo/internal/jobs/sessions"
	"infinite-experiment/politburo/internal/scheduler"
)

// Register is the single composition point for scheduled work.
func Register(jobScheduler *scheduler.Scheduler, client infiniteflight.ClientAPI, cacheStore cache.Store) error {
	if err := jobScheduler.Register(sessionsjob.New(client, cacheStore), schedules.SessionsSync); err != nil {
		return err
	}
	if err := jobScheduler.Register(liveriesjob.New(client, cacheStore), schedules.LiveriesSync); err != nil {
		return err
	}
	return jobScheduler.Register(flightsjob.New(client, cacheStore), schedules.FlightsSync)
}
