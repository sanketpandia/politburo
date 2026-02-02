package routes

import (
	"context"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/scheduler"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/flights"
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

	// Flights cache job - runs every minute
	// Tracks live flights with VA filtering and embedded waypoints
	flightsJob := flights.NewCacheJob(
		application.Infra.LiveAPI,
		application.Infra.RedisCache,
		application.Infra.RedisQueue,
		application.Platform.VARepo,
		application.Platform.AircraftSvc,
	)
	registry.Add(flightsJob, "0 * * * * *") // Every minute with second precision

	return nil
}

// RegisterWorkers initializes and starts all background workers
func RegisterWorkers(application *app.App) error {
	ctx := context.Background()

	// Initialize flight plan worker
	// This worker processes flight plan requests from the queue
	// It fetches flight plans, extracts route information, and updates cached flight data
	if application.Infra.RedisQueue != nil && application.Infra.RedisCache != nil && application.Infra.LiveAPI != nil {
		// Start flight plan worker
		flightPlanWorker := flights.NewFlightPlanWorker(
			application.Infra.RedisQueue,
			application.Infra.RedisCache,
			application.Infra.LiveAPI,
		)
		go func() {
			if err := flightPlanWorker.Start(ctx); err != nil {
				logging.Error("Flight plan worker stopped with error", "error", err)
			}
		}()
		logging.Info("Flight plan worker started")

		// Start flight plan queue monitor
		// Monitors queue health every 30 seconds
		monitor := flights.NewFlightPlanQueueMonitor(application.Infra.RedisQueue)
		go monitor.Start(ctx, 30*time.Second)
		logging.Info("Flight plan queue monitor started")

		// Start automatic queue trimming
		// Trims old messages every hour, keeping only the most recent 10,000 messages
		go monitor.StartAutoTrim(ctx, 1*time.Hour, 10000)
		logging.Info("Flight plan queue auto-trim started")
	}

	return nil
}
