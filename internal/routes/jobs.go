package routes

import (
	"context"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/scheduler"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"infinite-experiment/politburo/internal/sessions"
	vaRoutes "infinite-experiment/politburo/internal/va_routes"
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

	// Route sync job - runs every 10 minutes
	// Syncs routes from Airtable to local database
	// Initialize here to avoid import cycle (routes/jobs.go imports app, app would import va_routes)
	configRepo := repositories.NewDataProviderConfigRepo(application.Infra.DB)
	syncHistoryRepo := repositories.NewVASyncHistoryRepo(application.Infra.DB)
	routeRepo := vaRoutes.NewRepository(application.Infra.DB)
	airportRepo := repositories.NewAirportRepository(application.Infra.DB)
	routeSyncJob := vaRoutes.NewSyncJob(
		application.Infra.DB,
		application.Infra.RedisCache,
		configRepo,
		syncHistoryRepo,
		routeRepo,
		airportRepo,
		application.Infra.MetricsReg,
	)
	registry.Add(routeSyncJob, "0 */10 * * * *")
	logging.Info("Route sync job registered (every 10 minutes)")

	// PIREP sync job - runs every 5 minutes
	// Syncs PIREPs from Airtable to local database (incremental sync only)
	if application.Features.PirepSyncJob != nil {
		registry.Add(application.Features.PirepSyncJob, "0 */5 * * * *") // Every 5 minutes
		logging.Info("PIREP sync job registered (every 5 minutes)")
	}

	// Live flights webhook job - runs at :00 and :30 past every hour (every 30th minute)
	// POSTs Discord webhook payload with current VA live flights snapshot
	// Cron (sec min hour dom month dow): "0 0,30 * * * *" = at second 0, minutes 0 and 30
	if application.Features.LiveFlightsWebhookJob != nil {
		registry.Add(application.Features.LiveFlightsWebhookJob, "0 10,40 * * * *")
		logging.Info("Live flights webhook job registered (at :00 and :30 past each hour)")
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

	// Start PIREP queue worker
	// Processes PIREPs from Redis queue (enqueued by PIREP sync job)
	if application.Features.PirepQueueWorker != nil {
		go func() {
			// Start with 5 workers per VA
			if err := application.Features.PirepQueueWorker.Start(ctx, 5); err != nil {
				logging.Error("PIREP queue worker stopped with error", "error", err)
			}
		}()
		logging.Info("PIREP queue worker started (5 workers per VA)")
	}

	// Start aircraft livery sync worker
	// Syncs aircraft/livery data from Infinite Flight API to database every 6 hours
	if application.Infra.LiveAPI != nil && application.Infra.RedisCache != nil {
		// Create cache interface wrapper (RedisCache implements CacheInterface)
		var cacheInterface cache.CacheInterface = application.Infra.RedisCache
		aircraftWorker := aircraft.NewWorker(
			&cacheInterface,
			application.Infra.LiveAPI,
			application.Platform.AircraftRepo,
			application.Platform.AircraftSvc,
		)
		go func() {
			aircraftWorker.Start()
		}()
		logging.Info("Aircraft livery sync worker started (syncs every 6 hours)")
	}

	return nil
}
