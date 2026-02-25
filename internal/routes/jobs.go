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
		application.Infra.MetricsReg,
	)
	registry.Add(flightsJob, "0 * * * * *") // Every minute with second precision

	// Pilot sync job - runs every 10 minutes
	// Syncs pilots from Airtable to local database via queue
	if application.Features.PilotSyncJob != nil {
		registry.Add(application.Features.PilotSyncJob, "0 */1 * * * *")
	}

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
			application.Infra.MetricsReg,
		)
		go func() {
			if err := flightPlanWorker.Start(ctx); err != nil {
				logging.Error("Flight plan worker stopped with error", "error", err)
			}
		}()
		logging.Info("Flight plan worker started")

		// Start flight plan queue monitor
		// Monitors queue health every 30 seconds
		monitor := flights.NewFlightPlanQueueMonitor(
			application.Infra.RedisQueue,
			application.Infra.MetricsReg,
		)
		go monitor.Start(ctx, 30*time.Second)
		logging.Info("Flight plan queue monitor started")

		// Start automatic queue trimming
		// Trims old messages every 30 minutes, keeping only the most recent 1,000 messages
		go monitor.StartAutoTrim(ctx, 30*time.Minute, 1000)
		logging.Info("Flight plan queue auto-trim started")
	}

	// Start pilot sync worker
	if application.Features.PilotSyncWorker != nil {
		go func() {
			if err := application.Features.PilotSyncWorker.Start(ctx); err != nil {
				logging.Error("Pilot sync worker stopped with error", "error", err)
			}
		}()
		logging.Info("Pilot sync worker started")
	}

	return nil
}
