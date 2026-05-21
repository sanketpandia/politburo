package app

import (
	"context"
	"fmt"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/db"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/messaging"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/infra/queue"
	"infinite-experiment/politburo/infra/redis"
	"infinite-experiment/politburo/infra/scheduler"
	"infinite-experiment/politburo/infra/security"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/dashboard"
	"infinite-experiment/politburo/internal/datasource"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/events"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/liverymappings"
	membershipsFeature "infinite-experiment/politburo/internal/memberships"
	"infinite-experiment/politburo/internal/pilots"
	"infinite-experiment/politburo/internal/pireps"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"infinite-experiment/politburo/internal/platform/apikeys"
	"infinite-experiment/politburo/internal/platform/claims"
	"infinite-experiment/politburo/internal/platform/memberships"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/platform/va"
	"infinite-experiment/politburo/internal/servers"
	"infinite-experiment/politburo/internal/services"
	"infinite-experiment/politburo/internal/sync"
	vaRoutes "infinite-experiment/politburo/internal/va_routes"
	"infinite-experiment/politburo/internal/vaadmin"
	"infinite-experiment/politburo/internal/webhooks"
	"os"

	watermillredis "github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	watermillmessage "github.com/ThreeDotsLabs/watermill/message"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// App holds all application dependencies organized by architectural layer
type App struct {
	Config   Config
	UpSince  time.Time
	Infra    InfraDeps
	Platform PlatformDeps
	Features FeatureDeps
}

// InfraDeps holds all infrastructure-layer dependencies
type InfraDeps struct {
	DB               *gorm.DB
	RedisClient      *goredis.Client
	RedisCache       *cache.RedisCacheService
	RedisQueue       *queue.RedisQueueService
	SessionSvc       *session.SessionService
	URLSigner        *security.URLSignerService
	MetricsReg       *metrics.MetricsRegistry
	LiveAPI          *liveapi.Client
	Scheduler        *scheduler.Scheduler
	TemplateRenderer *templates.Renderer
	SyncContainer    *sync.Container

	// Watermill messaging
	WatermillPublisher  *watermillredis.Publisher
	WatermillSubscriber *watermillredis.Subscriber
	WatermillRouter     *watermillmessage.Router
}

// PlatformDeps holds all platform-layer repositories and services
type PlatformDeps struct {
	// Repositories
	ClaimsRepo      *claims.Repository
	KeysRepo        *apikeys.Repository
	UsersRepo       *users.Repository
	VARepo          *va.Repository
	MembershipsRepo *memberships.Repository
	AircraftRepo    *aircraft.Repository

	// Services
	UsersSvc       *users.Service
	VASvc          *va.Service
	VAConfigSvc    *va.ConfigService
	MembershipsSvc *memberships.Service
	AircraftSvc    *aircraft.Service

	// Handlers
	VAHandler *va.Handler
}

// FeatureDeps holds all feature-layer services and handlers
type FeatureDeps struct {
	// Services
	MembershipsFeatureSvc *membershipsFeature.Service
	PilotsRegSvc          *pilots.RegistrationService
	ServersRegSvc         *servers.RegistrationService

	// Handlers
	AuthHandler           *auth.Handler
	DashboardHandler      *dashboard.Handler
	MembershipsHandler    *membershipsFeature.Handler
	PilotsHandler         *pilots.Handler
	PirepHandler          *pireps.Handler
	ServersHandler        *servers.Handler
	VAAdminHandler        *vaadmin.Handler
	EventsHandler         *events.Handler
	DatasourceHandler     *datasource.Handler
	LiveryMappingsHandler *liverymappings.Handler
	TourPirepHandler      *pireps.TourHandler
	WebhooksHandler       *webhooks.Handler

	// Providers
	LiveAPIProvider *providers.LiveAPIProvider

	// Jobs
	PilotSyncJob          *pilots.SyncJob
	PirepSyncJob          *pireps.PirepSyncJob
	LiveFlightsWebhookJob *webhooks.LiveFlightsWebhookJob

	// Workers
	PilotSyncWorker  *pilots.SyncWorker
	PirepQueueWorker *pireps.QueueWorker
}

// New initializes the entire application with all dependencies
func New(cfg Config) (*App, error) {
	upSince := time.Now()

	logging.Info("Initializing application", "env", cfg.AppEnv, "debug", cfg.Debug)

	app := &App{
		Config:  cfg,
		UpSince: upSince,
	}

	// Tier 0: Infrastructure
	if err := app.initInfra(); err != nil {
		return nil, fmt.Errorf("failed to initialize infrastructure: %w", err)
	}

	// Tier 1: Platform
	if err := app.initPlatform(); err != nil {
		return nil, fmt.Errorf("failed to initialize platform: %w", err)
	}

	// Tier 2: Features
	if err := app.initFeatures(); err != nil {
		return nil, fmt.Errorf("failed to initialize features: %w", err)
	}

	logging.Info("Application initialized successfully")
	return app, nil
}

// initInfra initializes all infrastructure dependencies
func (a *App) initInfra() error {
	logging.Info("Initializing infrastructure layer")

	// Initialize metrics registry
	metricsReg := metrics.NewMetricsRegistry()
	logging.Debug("Metrics registry initialized")

	// Initialize database
	pgDB, err := db.InitPostgresORM(a.Config.PG.DSN())
	if err != nil {
		return fmt.Errorf("failed to initialize postgres: %w", err)
	}
	logging.Info("Database connection established")

	// Initialize Redis client
	redisClient := redis.NewRedisClient()
	logging.Info("Redis client initialized")

	// Initialize Redis cache (non-fatal if it fails)
	redisCache, err := cache.NewRedisCacheServiceWithMetrics(redisClient, metricsReg)
	if err != nil {
		logging.Warn("Failed to initialize Redis cache, continuing without it", "error", err)
		// Continue without Redis cache - services will handle gracefully
	} else {
		logging.Info("Redis cache service initialized")
	}

	// Initialize session service
	sessionSvc := session.NewSessionService(redisClient)
	logging.Info("Session service initialized")

	// Initialize URL Signer service
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("dev-secret-change-in-production")
		logging.Warn("JWT_SECRET not set, using default (dev only)")
	}
	urlSignerSvc := security.NewURLSignerService(jwtSecret, redisClient)
	logging.Info("URL signer service initialized")

	// Initialize Live API client
	liveAPI := liveapi.NewClientWithMetrics(metricsReg)
	logging.Info("Live API client initialized")

	// Initialize scheduler
	sched := scheduler.NewScheduler()
	logging.Info("Scheduler initialized")

	// Initialize Redis queue service
	redisQueue := queue.NewRedisQueueService(redisClient)
	logging.Info("Redis queue service initialized")

	// Initialize template renderer
	templateRenderer := templates.NewRendererWithOptions(
		"templates",                   // base path
		"templates/partials",          // partials path
		"templates/layouts/base.html", // layout path
		templates.WithReloadTemplates(a.Config.AppEnv == "local"),
	)
	logging.Info("Template renderer initialized", "reload_templates", a.Config.AppEnv == "local")

	// Initialize watermill publisher (Redis Stream)
	zapLog := logging.GetLogger().Desugar()
	wmPublisher, err := messaging.NewPublisher(redisClient, zapLog)
	if err != nil {
		return fmt.Errorf("failed to initialize watermill publisher: %w", err)
	}
	logging.Info("Watermill publisher initialized")

	// Initialize watermill subscriber for PIREP sync consumer group
	wmSubscriber, err := messaging.NewSubscriber(redisClient, "pirep-handlers", "politburo-1", zapLog)
	if err != nil {
		return fmt.Errorf("failed to initialize watermill subscriber: %w", err)
	}
	logging.Info("Watermill subscriber initialized")

	// Initialize watermill router with metrics + poison-queue middleware stack
	wmRouter, err := messaging.NewRouter(metricsReg, wmPublisher, zapLog)
	if err != nil {
		return fmt.Errorf("failed to initialize watermill router: %w", err)
	}
	logging.Info("Watermill router initialized")

	a.Infra = InfraDeps{
		DB:               pgDB,
		RedisClient:      redisClient,
		RedisCache:       redisCache,
		RedisQueue:       redisQueue,
		SessionSvc:       sessionSvc,
		URLSigner:        urlSignerSvc,
		MetricsReg:       metricsReg,
		LiveAPI:          liveAPI,
		Scheduler:        sched,
		TemplateRenderer: templateRenderer,

		WatermillPublisher:  wmPublisher,
		WatermillSubscriber: wmSubscriber,
		WatermillRouter:     wmRouter,
	}

	logging.Info("Infrastructure layer initialized")
	return nil
}

// initPlatform initializes all platform repositories and services
func (a *App) initPlatform() error {
	logging.Info("Initializing platform layer")

	// Initialize repositories
	claimsRepo := claims.NewRepository(a.Infra.DB)
	keysRepo := apikeys.NewRepository(a.Infra.DB)
	usersRepo := users.NewRepository(a.Infra.DB)
	vaRepo := va.NewRepository(a.Infra.DB)
	membershipsRepo := memberships.NewRepository(a.Infra.DB)
	aircraftRepo := aircraft.NewRepository(a.Infra.DB)

	logging.Debug("Platform repositories initialized")

	// Initialize services
	usersSvc := users.NewService(usersRepo)
	// VA service with cache support for data provider configs
	var vaSvc *va.Service
	if a.Infra.RedisCache != nil {
		vaSvc = va.NewServiceWithCache(vaRepo, a.Infra.RedisCache)
	} else {
		vaSvc = va.NewService(vaRepo)
	}
	membershipsSvc := memberships.NewService(membershipsRepo)
	// Aircraft service uses Redis cache directly (no legacy cache)
	aircraftSvc := aircraft.NewService(a.Infra.RedisCache, aircraftRepo)

	// Initialize VA config service (with aircraft dependencies for mapping resolution)
	vaConfigSvc := va.NewConfigService(vaRepo, a.Infra.RedisCache, aircraftRepo, aircraftSvc)

	// Initialize VA handler (for API endpoints)
	// Note: legacyVARepo is needed for FlightModesConfigService compatibility
	legacyVARepo := repositories.NewVAGormRepository(a.Infra.DB)
	vaHandler := va.NewHandler(vaSvc, vaConfigSvc, usersRepo, legacyVARepo)

	logging.Debug("Platform services initialized")

	a.Platform = PlatformDeps{
		ClaimsRepo:      claimsRepo,
		KeysRepo:        keysRepo,
		UsersRepo:       usersRepo,
		VARepo:          vaRepo,
		MembershipsRepo: membershipsRepo,
		AircraftRepo:    aircraftRepo,
		UsersSvc:        usersSvc,
		VASvc:           vaSvc,
		VAConfigSvc:     vaConfigSvc,
		MembershipsSvc:  membershipsSvc,
		AircraftSvc:     aircraftSvc,
		VAHandler:       vaHandler,
	}

	logging.Info("Platform layer initialized")
	return nil
}

// initFeatures initializes all feature services and handlers
func (a *App) initFeatures() error {
	logging.Info("Initializing features layer")

	// Initialize providers
	liveAPIProvider := providers.NewLiveAPIProvider()
	logging.Debug("Live API provider initialized")

	// Initialize pilot repository
	pilotRepo := pilots.NewRepository(a.Infra.DB)
	logging.Debug("Pilot repository initialized")

	// Initialize feature services
	membershipsFeatureSvc := membershipsFeature.NewService(
		a.Platform.MembershipsSvc, // Keep for GetUserStatus
		a.Platform.UsersSvc,       // Users service for user lookup and membership creation
		a.Platform.VASvc,          // VA service for VA lookup
		pilotRepo,                 // Pilot repository for Airtable validation
	)
	pilotsRegSvc := pilots.NewRegistrationService(
		a.Platform.UsersSvc,
		a.Platform.VASvc,
		liveAPIProvider,
	)
	serversRegSvc := servers.NewRegistrationService(
		a.Platform.UsersSvc,
		a.Platform.VASvc,
		a.Platform.UsersRepo,
	)

	logging.Debug("Feature services initialized")

	// Initialize logbook service
	logbookSvc := pilots.NewLogbookService(
		a.Infra.LiveAPI,
		a.Platform.AircraftSvc,
	)
	logging.Debug("Logbook service initialized")

	// Initialize pilot management service
	vaUserRoleRepo := repositories.NewVAUserRoleRepository(a.Infra.DB)
	pilotMgmtSvc := pilots.NewManagementService(vaUserRoleRepo)
	logging.Debug("Pilot management service initialized")

	// Initialize events feature
	eventRepo := events.NewRepository(a.Infra.DB)
	eventSvc := events.NewService(eventRepo)
	routeRepo := vaRoutes.NewRepository(a.Infra.DB)

	// Initialize flights service for events handler (needed for live flight lookup)
	// Use platform services: infra LiveAPI client and platform VA config service
	flightsSvc := flights.NewService(a.Infra.RedisCache, a.Infra.LiveAPI, a.Platform.VAConfigSvc, a.Platform.AircraftSvc)

	// Use platform VA service (not legacy repo) in events handler
	eventsHandler := events.NewHandler(eventSvc, a.Infra.TemplateRenderer, routeRepo, flightsSvc, a.Platform.VASvc)
	logging.Debug("Events feature initialized")

	// Initialize sync jobs (route sync, etc.)
	syncRepo := sync.NewRepository(a.Infra.DB)
	airportRepo := repositories.NewAirportRepository(a.Infra.DB)
	logger := logging.GetLogger().Desugar() // Convert SugaredLogger to Logger
	syncContainer := sync.InitializeJobs(
		context.Background(),
		a.Infra.DB,
		a.Infra.RedisCache,
		a.Platform.VASvc,
		syncRepo,
		airportRepo,
		logger,
	)
	a.Infra.SyncContainer = syncContainer
	logging.Debug("Sync jobs initialized")

	// Initialize platform VA sync repository for sync history
	vaSyncRepo := va.NewSyncRepository(a.Infra.DB)

	// Initialize pilot sync job
	configRepo := repositories.NewDataProviderConfigRepo(a.Infra.DB)
	pilotSyncJob := pilots.NewSyncJob(
		a.Infra.DB,
		a.Infra.RedisCache,
		configRepo,
		vaSyncRepo,
		pilotRepo,
		a.Infra.RedisQueue, // Add queue parameter
		a.Infra.MetricsReg, // Add metrics registry
	)
	logging.Debug("Pilot sync job initialized")

	// Initialize pilot sync worker (if queue enabled)
	var pilotSyncWorker *pilots.SyncWorker
	if a.Infra.RedisQueue != nil {
		pilotSyncWorker = pilots.NewSyncWorker(
			a.Infra.RedisQueue,
			pilotSyncJob,
			a.Infra.MetricsReg,
			a.Infra.RedisCache,
		)
		logging.Debug("Pilot sync worker initialized")
	}

	// Initialize PIREP sync job
	pirepRepo := pireps.NewRepository(a.Infra.DB)
	pirepSyncJob := pireps.NewPirepSyncJob(
		a.Infra.DB,
		a.Infra.RedisCache,
		a.Platform.VARepo,
		pirepRepo,
		vaSyncRepo,
		a.Infra.RedisQueue,
		a.Infra.MetricsReg,
	)
	// Wire watermill publisher for dual-write to TopicPirepSync.
	if a.Infra.WatermillPublisher != nil {
		pirepSyncJob.SetPublisher(a.Infra.WatermillPublisher)
	}
	logging.Debug("PIREP sync job initialized")

	// Initialize watermill PIREP messaging handler and register with the router.
	pirepMsgHandler := pireps.NewMessagingHandler(a.Infra.DB, a.Platform.VARepo, pirepRepo)
	if a.Infra.WatermillRouter != nil && a.Infra.WatermillSubscriber != nil {
		pireps.RegisterPirepHandlers(a.Infra.WatermillRouter, a.Infra.WatermillSubscriber, pirepMsgHandler)
	}
	logging.Debug("PIREP watermill handler registered")

	// Initialize PIREP queue worker (if queue enabled)
	var pirepQueueWorker *pireps.QueueWorker
	if a.Infra.RedisQueue != nil {
		pirepQueueWorker = pireps.NewQueueWorker(
			a.Infra.DB,
			a.Platform.VARepo,
			pirepRepo,
			a.Infra.RedisQueue,
			a.Infra.MetricsReg,
		)
		logging.Debug("PIREP queue worker initialized")
	}

	// Initialize legacy VA config service (needed by StatsService)
	// Create legacy cache service for StatsService
	legacyCacheSvc := cache.NewCacheService(60000, 600)
	// Create another legacy VARepo instance for StatsService (legacyVARepo is scoped to initPlatform)
	legacyVARepoForStats := repositories.NewVAGormRepository(a.Infra.DB)
	legacyVAConfigSvc := common.NewVAConfigService(legacyVARepoForStats, legacyCacheSvc)

	// Initialize StatsService
	statsSvc := pilots.NewStatsService(
		a.Infra.DB,
		legacyCacheSvc,
		configRepo,
		a.Platform.UsersRepo,
		legacyVAConfigSvc,
		syncRepo,
	)
	logging.Debug("Stats service initialized")

	// Initialize dashboard service and handler
	dashboardSvc := dashboard.NewService(eventSvc, pirepRepo, statsSvc, a.Infra.DB)
	dashboardHandler := dashboard.NewHandler(a.Infra.TemplateRenderer, dashboardSvc, a.Infra.SessionSvc)

	// Initialize handlers
	membershipsHandler := membershipsFeature.NewHandler(membershipsFeatureSvc, pilotRepo, a.Platform.VAConfigSvc)
	pilotsHandler := pilots.NewHandler(statsSvc, pilotsRegSvc, logbookSvc, a.Platform.UsersSvc)
	serversHandler := servers.NewHandler(serversRegSvc)

	// Initialize datasource handler
	airtableProvider := providers.NewAirtableProvider(a.Infra.RedisCache)
	datasourceHandler := datasource.NewHandler(a.Platform.VASvc, a.Infra.TemplateRenderer, airtableProvider)

	// Initialize tour PIREP feature (extracted from inline handler in routes/router.go)
	tourPirepSvc := pireps.NewTourPirepService(
		a.Platform.VASvc,
		a.Platform.VAConfigSvc,
		a.Platform.UsersRepo,
		eventSvc,
		syncRepo,
		a.Infra.RedisCache,
		airtableProvider,
		configRepo,
	)
	tourPirepHandler := pireps.NewTourHandler(tourPirepSvc)
	logging.Debug("Tour PIREP handler initialized")

	// Initialize full PIREP handler (GetConfig + Submit endpoints).
	legacyUserRepo := repositories.NewUserRepositoryGORM(a.Infra.DB)
	legacyVARepoForPirep := repositories.NewVAGormRepository(a.Infra.DB)
	legacyLiveAPISvc := common.NewLiveAPIService()
	pirepValidator := pireps.NewFlightModeValidationService(legacyLiveAPISvc, legacyCacheSvc)
	dataProviderConfigSvc := services.NewDataProviderConfigService(configRepo, legacyCacheSvc)
	pirepSvc := pireps.NewService(
		legacyUserRepo,
		pilotRepo,
		syncRepo,
		a.Platform.AircraftRepo,
		configRepo,
		airtableProvider,
		pirepValidator,
		legacyCacheSvc,
		flightsSvc,
		legacyVAConfigSvc,
		dataProviderConfigSvc,
	)
	pirepHandler := pireps.NewHandler(
		legacyUserRepo,
		legacyVARepoForPirep,
		legacyCacheSvc,
		legacyLiveAPISvc,
		flightsSvc,
		legacyVAConfigSvc,
		pirepSvc,
		pirepValidator,
	)
	logging.Debug("Full PIREP handler initialized")

	// Initialize auth handler (used by API routes; avoids constructing it redundantly in router.go).
	authSvcAdapter := &appVAServiceAdapter{svc: a.Platform.VASvc}
	authSvc := auth.NewService(
		a.Infra.SessionSvc,
		a.Infra.URLSigner,
		a.Platform.ClaimsRepo,
		a.Platform.UsersSvc,
		authSvcAdapter,
	)
	authHandler := auth.NewHandler(authSvc, a.Infra.TemplateRenderer)
	logging.Debug("Auth handler initialized")

	// Initialize livery mappings handler
	liveryMappingsHandler := liverymappings.NewHandler(a.Platform.AircraftRepo, a.Platform.VAConfigSvc, a.Infra.TemplateRenderer)

	// VA webhook repo (used by vaadmin UI and webhooks API/job)
	vaWebhookRepo := va.NewWebhookRepo(a.Infra.DB)
	webhooksHandler := webhooks.NewHandler(vaWebhookRepo, a.Platform.VASvc)
	liveFlightsWebhookJob := webhooks.NewLiveFlightsWebhookJob(
		vaWebhookRepo,
		a.Platform.VARepo,
		a.Infra.RedisCache,
		a.Infra.MetricsReg,
	)
	vaAdminHandler := vaadmin.NewHandler(pilotMgmtSvc, a.Platform.VASvc, a.Platform.VAConfigSvc, vaWebhookRepo, liveFlightsWebhookJob, a.Infra.TemplateRenderer)

	logging.Debug("Feature handlers initialized")

	a.Features = FeatureDeps{
		MembershipsFeatureSvc: membershipsFeatureSvc,
		PilotsRegSvc:          pilotsRegSvc,
		ServersRegSvc:         serversRegSvc,
		AuthHandler:           authHandler,
		DashboardHandler:      dashboardHandler,
		MembershipsHandler:    membershipsHandler,
		PilotsHandler:         pilotsHandler,
		PirepHandler:          pirepHandler,
		ServersHandler:        serversHandler,
		VAAdminHandler:        vaAdminHandler,
		EventsHandler:         eventsHandler,
		DatasourceHandler:     datasourceHandler,
		LiveryMappingsHandler: liveryMappingsHandler,
		TourPirepHandler:      tourPirepHandler,
		LiveAPIProvider:       liveAPIProvider,
		PilotSyncJob:          pilotSyncJob,
		PirepSyncJob:          pirepSyncJob,
		PilotSyncWorker:       pilotSyncWorker,
		PirepQueueWorker:      pirepQueueWorker,
		WebhooksHandler:       webhooksHandler,
		LiveFlightsWebhookJob: liveFlightsWebhookJob,
	}

	logging.Info("Features layer initialized")
	return nil
}

// Shutdown gracefully shuts down all application resources
func (a *App) Shutdown(ctx context.Context) {
	logging.Info("Shutting down application")

	// Shutdown in reverse order of initialization

	// Stop watermill router (closes subscriber connections and drains handlers)
	if a.Infra.WatermillRouter != nil {
		if err := a.Infra.WatermillRouter.Close(); err != nil {
			logging.Error("Failed to close watermill router", "error", err)
		}
	}

	// Close watermill publisher
	if a.Infra.WatermillPublisher != nil {
		if err := a.Infra.WatermillPublisher.Close(); err != nil {
			logging.Error("Failed to close watermill publisher", "error", err)
		}
	}

	// Stop scheduler (waits for running jobs)
	if a.Infra.Scheduler != nil {
		a.Infra.Scheduler.Stop()
	}

	// Close Redis cache
	if a.Infra.RedisCache != nil {
		if err := a.Infra.RedisCache.Close(); err != nil {
			logging.Error("Failed to close Redis cache", "error", err)
		}
	}

	// Close Redis client
	if a.Infra.RedisClient != nil {
		if err := a.Infra.RedisClient.Close(); err != nil {
			logging.Error("Failed to close Redis client", "error", err)
		}
	}

	// Close database connection
	if a.Infra.DB != nil {
		sqlDB, err := a.Infra.DB.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				logging.Error("Failed to close database", "error", err)
			}
		}
	}

	logging.Info("Application shutdown complete")
}

// appVAServiceAdapter adapts *va.Service to the auth.VAService interface.
// Defined here to keep the auth handler construction inside app.go without
// creating a cycle: auth does not import va, so this thin bridge is needed.
type appVAServiceAdapter struct {
	svc *va.Service
}

// GetByDiscordServerID implements auth.VAService.
func (a *appVAServiceAdapter) GetByDiscordServerID(ctx context.Context, discordServerID string) (auth.VAInfo, error) {
	v, err := a.svc.GetByDiscordServerID(ctx, discordServerID)
	if err != nil {
		return auth.VAInfo{}, err
	}
	return auth.VAInfo{
		ID:   v.ID,
		Name: v.Name,
		Code: v.Code,
	}, nil
}
