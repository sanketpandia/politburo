package api

import (
	"log"
	"os"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/db"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/infra/queue"
	"infinite-experiment/politburo/infra/redis"
	"infinite-experiment/politburo/infra/security"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/pilots"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"infinite-experiment/politburo/internal/services"
	"infinite-experiment/politburo/internal/sync"
	"infinite-experiment/politburo/internal/va"

	goredis "github.com/redis/go-redis/v9"
)

type Repositories struct {
	User            *repositories.UserRepositoryGORM
	Keys            *repositories.KeysRepo
	UserVASync      *repositories.SyncRepository
	Va              *repositories.VAGormRepository      // Legacy: kept for compatibility
	VANew           *va.Repository                      // NEW: VA repository in va package
	VARoleRepo      *va.RoleRepository                  // NEW: VA role repository
	VAEventRepo     *va.EventRepository                 // NEW: VA event repository
	DataProviderCfg *repositories.DataProviderConfigRepo
	Sync            *sync.Repository // Consolidated sync repository (routes, PIREPs, history)
	Pilots          *pilots.Repository // Pilot repository (migrated from PilotATSyncedRepo)
	Aircraft        *aircraft.Repository // Combined aircraft repository
	AirportsRepo    *repositories.AirportRepository
	WorldTour       *repositories.WorldTourRepository
	VAUserRole      *repositories.VAUserRoleRepository // For pilot management - kept for compatibility
}

type Services struct {
	Cache              cache.CacheInterface             // Interface - supports Redis or in-memory
	RedisCache         *cache.RedisCacheService         // Redis cache (for jobs that need specific Redis features)
	LegacyCache        *cache.CacheService              // For services that haven't been migrated to interface yet
	Live               *common.LiveAPIService           // Changed to pointer
	User               *services.UserService
	Reg                *services.RegistrationService    // Changed to pointer
	RegV2              *services.RegistrationServiceV2
	Conf               *common.VAConfigService          // Legacy: kept for compatibility
	VAConfig           *va.ConfigService                // NEW: VA config service in va package
	VAService          *va.Service                      // NEW: Core VA service
	VAEventService     *va.EventService                 // NEW: VA event service
	VaMgmt             *services.VAManagementService    // Changed to pointer
	AirtableApi        *common.AirtableApiService       // Changed to pointer
	AirtableProvider   *providers.AirtableProvider
	AirtableSync       *services.AtSyncService          // Changed to pointer
	Flights            *flights.Service                 // NEW: Moved to flights package
	PilotStats         *pilots.StatsService             // NEW: Moved to pilots package
	PilotMgmt          *pilots.ManagementService        // NEW: Pilot management service
	DataProviderConfig *services.DataProviderConfigService
	Aircraft           *aircraft.Service                // NEW: Platform aircraft service
	RedisQueue         *queue.RedisQueueService         // Changed to pointer
	URLSigner          *security.URLSignerService
	Session            *session.SessionService
	WorldTour          *services.WorldTourService
}
type Dependencies struct {
	Repo     *Repositories
	Services *Services
}

func InitDependencies(metricsReg *metrics.MetricsRegistry) (*Dependencies, error) {

	syncRepo := sync.NewRepository(db.PgDB)

	// Initialize VA repositories (new package)
	vaRepo := va.NewRepository(db.PgDB)
	vaRoleRepo := va.NewRoleRepository(db.PgDB)
	vaEventRepo := va.NewEventRepository(db.PgDB)

	repositories := &Repositories{
		User:            repositories.NewUserRepositoryGORM(db.PgDB),
		Keys:            repositories.NewApiKeysRepo(db.PgDB),
		Va:              repositories.NewVAGormRepository(db.PgDB), // Legacy: kept for compatibility
		VANew:           vaRepo,                                     // NEW: VA repository
		VARoleRepo:      vaRoleRepo,                                 // NEW: VA role repository
		VAEventRepo:     vaEventRepo,                                // NEW: VA event repository
		UserVASync:      repositories.NewSyncRepository(db.PgDB),
		DataProviderCfg: repositories.NewDataProviderConfigRepo(db.PgDB),
		Sync:            syncRepo, // Consolidated sync repository
		Pilots:          pilots.NewRepository(db.PgDB),
		Aircraft:        aircraft.NewRepository(db.PgDB),
		AirportsRepo:    repositories.NewAirportRepository(db.PgDB),
		WorldTour:       repositories.NewWorldTourRepository(db.PgDB, nil), // TODO: Update WorldTourRepository to use sync.Repository
		VAUserRole:      repositories.NewVAUserRoleRepository(db.PgDB), // Legacy: kept for compatibility
	}

	// Initialize cache service (Redis or in-memory based on USE_REDIS_CACHE env var)
	var cacheSvc cache.CacheInterface
	var redisCacheSvc *cache.RedisCacheService
	useRedis := os.Getenv("USE_REDIS_CACHE") == "true"
	var redisClient *goredis.Client
	if useRedis {
		// Initialize Redis client (used by both cache and queue services)
		redisClient = redis.NewRedisClient()
		redisCache, err := cache.NewRedisCacheServiceWithMetrics(redisClient, metricsReg)
		if err != nil {
			log.Printf("Failed to initialize Redis cache, falling back to in-memory: %v", err)
			cacheSvc = cache.NewCacheServiceWithMetrics(60000, 600, metricsReg)
		} else {
			log.Println("Using Redis cache")
			cacheSvc = redisCache
			redisCacheSvc = redisCache // Store Redis cache service for jobs
		}
	} else {
		log.Println("Using in-memory cache")
		cacheSvc = cache.NewCacheServiceWithMetrics(60000, 600, metricsReg)
	}

	// Always initialize RedisQueueService (required for PIREP queue processing)
	// Uses the same Redis client as cache for efficiency
	redisQSvc := queue.NewRedisQueueService(redisClient)

	// Create legacy cache wrapper for services that still need *CacheService
	var legacyCache *cache.CacheService
	if cs, ok := cacheSvc.(*cache.CacheService); ok {
		legacyCache = cs
	} else {
		// If using Redis, create a legacy in-memory cache for services that need it
		legacyCache = cache.NewCacheServiceWithMetrics(60000, 600, metricsReg)
	}

	liveSvc := common.NewLiveAPIService()
	confSvc := common.NewVAConfigService(repositories.Va, cacheSvc) // Legacy: kept for compatibility

	// Initialize VA services (new package)
	vaConfigSvc := va.NewConfigService(vaRepo, cacheSvc)
	vaService := va.NewService(vaRepo, vaRoleRepo)
	vaEventSvc := va.NewEventService(vaEventRepo)

	// Initialize providers
	liveAPIProvider := providers.NewLiveAPIProvider()

	// Initialize pilot services (stats and management)
	pilotStatsSvc := pilots.NewStatsService(db.PgDB, legacyCache, repositories.DataProviderCfg, repositories.User, confSvc, syncRepo)
	pilotMgmtSvc := pilots.NewManagementService(repositories.VAUserRole)

	// Initialize user service with GORM repository (pilot stats service removed from dependency)
	userSvc := services.NewUserService(repositories.User, nil)

	// Initialize data provider config service
	dataProviderConfigSvc := services.NewDataProviderConfigService(repositories.DataProviderCfg, cacheSvc)
	if dataProviderConfigSvc == nil {
		log.Println("WARNING: DataProviderConfigService is nil after initialization!")
	} else {
		log.Println("DataProviderConfigService initialized successfully")
	}

	// Initialize V2 registration service with GORM and LiveAPIProvider
	regServiceV2 := services.NewRegistrationServiceV2(db.PgDB, liveAPIProvider)

	// Initialize aircraft service (platform level)
	aircraftSvc := aircraft.NewService(legacyCache, repositories.Aircraft)

	// Initialize Airtable provider
	airtableProvider := providers.NewAirtableProvider(cacheSvc)

	// Initialize URL Signer service for presigned dashboard links
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("dev-secret-change-in-production")
	}
	urlSignerSvc := security.NewURLSignerService(jwtSecret, redisClient)

	// Initialize session service for UI authentication
	sessionSvc := session.NewSessionService(redisClient)

	// Initialize World Tour service
	worldTourSvc := services.NewWorldTourService(repositories.WorldTour)

	svc := &Services{
		User:               userSvc,
		Reg:                nil, // TODO: Migrate RegistrationService to GORM or remove (use RegV2)
		RegV2:              regServiceV2,
		Conf:               confSvc,      // Legacy: kept for compatibility
		VAConfig:           vaConfigSvc,  // NEW: VA config service
		VAService:          vaService,    // NEW: Core VA service
		VAEventService:     vaEventSvc,   // NEW: VA event service
		VaMgmt:             nil, // TODO: Migrate VAManagementService to GORM or deprecate
		AirtableApi:        common.NewAirtableApiService(confSvc),
		AirtableProvider:   airtableProvider,
		AirtableSync:       services.NewAtSyncService(legacyCache, repositories.UserVASync),
		Flights:            flights.NewService(legacyCache, liveSvc, confSvc, aircraftSvc),
		PilotStats:         pilotStatsSvc,
		PilotMgmt:          pilotMgmtSvc,
		DataProviderConfig: dataProviderConfigSvc,
		Aircraft:           aircraftSvc,
		Cache:              cacheSvc,
		RedisCache:         redisCacheSvc, // Redis cache for jobs (nil if not using Redis)
		LegacyCache:        legacyCache,
		Live:               liveSvc,
		RedisQueue:         redisQSvc,
		URLSigner:          urlSignerSvc,
		Session:            sessionSvc,
		WorldTour:          worldTourSvc,
	}

	return &Dependencies{
		Repo:     repositories,
		Services: svc,
	}, nil

}
