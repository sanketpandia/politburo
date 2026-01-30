package routes

import (
	"infinite-experiment/politburo/internal/common"
	"context"
	"net/http"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/internal/api"
	"infinite-experiment/politburo/infra/db"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/jobs"
	"infinite-experiment/politburo/internal/middleware"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"infinite-experiment/politburo/internal/sync"
	"infinite-experiment/politburo/internal/workers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func RegisterRoutes(upSince time.Time) http.Handler {

	// initialize Chi router
	r := chi.NewRouter()

	// Initialize metrics registry
	metricsReg := metrics.NewMetricsRegistry()

	// global middleware
	r.Use(middleware.RequestIDMiddleware)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://localhost:8081"}, // Allow all origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key", "X-Server-Id", "X-Discord-Id"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	logging.Info("Router initialized with metrics and logging middleware")
	// health check
	r.Get("/healthCheck", api.HealthCheckHandler(db.PgDB, upSince))

	// Initialize dependencies using DI pattern
	deps, err := api.InitDependencies(metricsReg)
	if err != nil {
		panic("Failed to initialize dependencies: " + err.Error())
	}

	// Initialize handlers with dependencies
	handlers := api.NewHandlers(deps)

	// Legacy: Keep individual references for old handlers that haven't been migrated yet
	userRepoGorm := deps.Repo.User
	keyRepo := deps.Repo.Keys
	legacyCacheSvc := deps.Services.LegacyCache
	cfgSvc := deps.Services.Conf
	vaMgmtSvc := deps.Services.VaMgmt
	atApiSvc := deps.Services.AirtableApi
	syncSvc := deps.Services.AirtableSync
	flightSvc := deps.Services.Flights

	// Get session and URL signer services from dependencies (reuses same Redis client)
	sessionSvc := deps.Services.Session
	urlSigner := deps.Services.URLSigner

	// Initialize VA repositories for UI
	vaGormRepo := repositories.NewVAGORMRepository(db.PgDB)
	vaUserRoleRepo := repositories.NewVAUserRoleRepository(db.PgDB)
	eventRepo := repositories.NewVAEventRepository(db.PgDB)
	routeRepo := repositories.NewRouteATSyncedRepo(db.PgDB)

	// Update AuthMiddleware to use session service
	// This will be passed to middleware when creating handlers

	// Register UI routes (separate from API)
	// Note: FlightModesConfigService will be created inside ui_routes to avoid circular imports
	RegisterUIRoutes(r, metricsReg, sessionSvc, urlSigner, userRepoGorm, vaUserRoleRepo, vaGormRepo, flightSvc, deps.Services.Cache, deps.Services.Live, deps.Services.DataProviderConfig, deps.Services.AirtableProvider, deps.Repo.Va, eventRepo, routeRepo)

	// Setup workers and jobs first
	logger := logging.GetLogger().Desugar() // Get non-sugared logger for sync jobs

	// Initialize sync jobs (routes only) - runs every 10 minutes
	_ = sync.InitializeJobs(
		context.Background(),
		db.PgDB,
		deps.Services.Cache,
		deps.Repo.DataProviderCfg,
		deps.Repo.Sync, // Consolidated sync repository
		deps.Repo.AirportsRepo,
		logger,
	)

	// Initialize non-sync jobs (pilot sync, cache jobs, backfill)
	// Note: Pilot sync still uses sync.Repository for history tracking
	jobsContainer := jobs.InitializeJobs(
		context.Background(),
		db.PgDB,
		deps.Services.Cache,      // Use CacheInterface (supports Redis or in-memory)
		deps.Repo.DataProviderCfg,
		nil,                      // TODO: Update pilot sync to use sync.Repository
		deps.Repo.Pilots,
		cfgSvc,
		deps.Services.Live,       // LiveAPIService for cache jobs
		deps.Services.RedisCache, // RedisCacheService for cache jobs (nil if not using Redis)
	)

	// Initialize aircraft worker separately (platform level)
	aircraftWorker := aircraft.NewWorker(
		&deps.Services.Cache,
		deps.Services.Live,
		deps.Repo.Aircraft,
		deps.Services.Aircraft,
	)
	go aircraftWorker.Start()

	// Initialize other workers (includes LogbookWorker)
	workers.InitWorkers(
		db.PgDB,
		&deps.Services.Cache,
		deps.Services.Live,
		deps.Services.Aircraft,
		deps.Services.RedisQueue,
		deps.Repo.DataProviderCfg,
	)

	// Initialize jobs handler for manual triggering
	jobsHandler := api.NewJobsHandler(jobsContainer.PilotSync)

	// Initialize airport loader service
	airportLoader := common.NewAirportLoaderService(db.PgDB)

	// Register API routes (after jobsHandler is initialized)
	RegisterAPIRoutes(r, metricsReg, userRepoGorm, keyRepo, handlers, legacyCacheSvc, cfgSvc, vaMgmtSvc, atApiSvc, syncSvc, flightSvc, jobsHandler, deps, airportLoader, sessionSvc)

	return r
}
