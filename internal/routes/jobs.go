package routes

import (
	"infinite-experiment/politburo/infra/scheduler"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"infinite-experiment/politburo/internal/sessions"
)

// RegisterScheduledJobs registers all cron jobs with the scheduler
// All dependencies are provided through the App struct
func RegisterScheduledJobs(application *app.App) error {
	registry := scheduler.NewRegistry(application.Infra.Scheduler)

	// Session cache job - runs every 5 minutes
	// Caches all active Infinite Flight sessions/servers to Redis
	sessionJob := sessions.NewCacheJob(application.Infra.LiveAPI, application.Infra.RedisCache)
	registry.Add(sessionJob, "0 */5 * * * *") // Every 5 minutes

	// Aircraft cache job - runs every hour
	// Caches aircraft and livery data from Infinite Flight API to Redis
	aircraftJob := aircraft.NewCacheJob(application.Infra.LiveAPI, application.Infra.RedisCache)
	registry.Add(aircraftJob, "0 0 * * * *") // Every hour

	return nil
}
