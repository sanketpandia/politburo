package api

import (
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/metrics"
	"infinite-experiment/politburo/internal/providers"
	"infinite-experiment/politburo/internal/services"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

type Repositories struct {
	User                  *repositories.UserRepositoryGORM
	Keys                  *repositories.KeysRepo
	UserVASync            *repositories.SyncRepository
	Va                    *repositories.VAGormRepository
	DataProviderCfg       *repositories.DataProviderConfigRepo
	VASyncHistory         *repositories.VASyncHistoryRepo
	PilotATSynced         *repositories.PilotATSyncedRepo
	RouteATSynced         *repositories.RouteATSyncedRepo
	PirepATSynced         *repositories.PirepATSyncedRepo
	AircraftLivery        *repositories.AircraftLiveryRepository
	LiveryAirtableMapping *repositories.LiveryAirtableMappingRepository
	AirportsRepo          *repositories.AirportRepository
	WorldTour             *repositories.WorldTourRepository
}

type Services struct {
	Cache              common.CacheInterface         // Interface - supports Redis or in-memory
	RedisCache         *common.RedisCacheService     // Redis cache (for jobs that need specific Redis features)
	LegacyCache        *common.CacheService          // For services that haven't been migrated to interface yet
	Live               *common.LiveAPIService        // Changed to pointer
	User               *services.UserService
	Reg                *services.RegistrationService // Changed to pointer
	RegV2              *services.RegistrationServiceV2
	Conf               *common.VAConfigService       // Changed to pointer
	VaMgmt             *services.VAManagementService // Changed to pointer
	AirtableApi        *common.AirtableApiService    // Changed to pointer
	AirtableProvider   *providers.AirtableProvider
	AirtableSync       *services.AtSyncService       // Changed to pointer
	Flights            *services.FlightsService      // Changed to pointer
	PilotStats         *services.PilotStatsService
	DataProviderConfig *services.DataProviderConfigService
	AircraftLivery     *common.AircraftLiveryService
	RedisQueue         *common.RedisQueueService     // Changed to pointer
	URLSigner          *common.URLSignerService
	Session            *common.SessionService
	WorldTour          *services.WorldTourService
}
type Dependencies struct {
	Repo     *Repositories
	Services *Services
}

func InitDependencies(metricsReg *metrics.MetricsRegistry) (*Dependencies, error) {

	repositories := &Repositories{
		User:                  repositories.NewUserRepositoryGORM(db.PgDB),
		Keys:                  repositories.NewApiKeysRepo(db.PgDB),
		Va:                    repositories.NewVAGormRepository(db.PgDB),
		UserVASync:            repositories.NewSyncRepository(db.PgDB),
		DataProviderCfg:       repositories.NewDataProviderConfigRepo(db.PgDB),
		VASyncHistory:         repositories.NewVASyncHistoryRepo(db.PgDB),
		PilotATSynced:         repositories.NewPilotATSyncedRepo(db.PgDB),
		RouteATSynced:         repositories.NewRouteATSyncedRepo(db.PgDB),
		PirepATSynced:         repositories.NewPirepATSyncedRepo(db.PgDB),
		AircraftLivery:        repositories.NewAircraftLiveryRepository(db.PgDB),
		LiveryAirtableMapping: repositories.NewLiveryAirtableMappingRepository(db.PgDB),
		AirportsRepo:          repositories.NewAirportRepository(db.PgDB),
		WorldTour:             repositories.NewWorldTourRepository(db.PgDB, repositories.NewRouteATSyncedRepo(db.PgDB)),
	}

	// Initialize cache service (Redis or in-memory based on USE_REDIS_CACHE env var)
	var cacheSvc common.CacheInterface
	var redisCacheSvc *common.RedisCacheService
	useRedis := os.Getenv("USE_REDIS_CACHE") == "true"
	var redisClient *redis.Client
	if useRedis {
		// Initialize Redis client (used by both cache and queue services)
		redisClient = common.NewRedisClient()
		redisCache, err := common.NewRedisCacheServiceWithMetrics(redisClient, metricsReg)
		if err != nil {
			log.Printf("Failed to initialize Redis cache, falling back to in-memory: %v", err)
			cacheSvc = common.NewCacheServiceWithMetrics(60000, 600, metricsReg)
		} else {
			log.Println("Using Redis cache")
			cacheSvc = redisCache
			redisCacheSvc = redisCache // Store Redis cache service for jobs
		}
	} else {
		log.Println("Using in-memory cache")
		cacheSvc = common.NewCacheServiceWithMetrics(60000, 600, metricsReg)
	}

	// Always initialize RedisQueueService (required for PIREP queue processing)
	// Uses the same Redis client as cache for efficiency
	redisQSvc := common.NewRedisQueueService(redisClient)

	// Create legacy cache wrapper for services that still need *CacheService
	var legacyCache *common.CacheService
	if cs, ok := cacheSvc.(*common.CacheService); ok {
		legacyCache = cs
	} else {
		// If using Redis, create a legacy in-memory cache for services that need it
		legacyCache = common.NewCacheServiceWithMetrics(60000, 600, metricsReg)
	}

	liveSvc := common.NewLiveAPIService()
	confSvc := common.NewVAConfigService(repositories.Va, cacheSvc)

	// Initialize providers
	liveAPIProvider := providers.NewLiveAPIProvider()

	// Initialize pilot stats service first (needed by UserService)
	pilotStatsSvc := services.NewPilotStatsService(db.PgDB, legacyCache, repositories.DataProviderCfg, repositories.User, confSvc, repositories.PirepATSynced, repositories.RouteATSynced)

	// Initialize user service with GORM repository and pilot stats service
	userSvc := services.NewUserService(repositories.User, pilotStatsSvc)

	// Initialize data provider config service
	dataProviderConfigSvc := services.NewDataProviderConfigService(repositories.DataProviderCfg, cacheSvc)
	if dataProviderConfigSvc == nil {
		log.Println("WARNING: DataProviderConfigService is nil after initialization!")
	} else {
		log.Println("DataProviderConfigService initialized successfully")
	}

	// Initialize V2 registration service with GORM and LiveAPIProvider
	regServiceV2 := services.NewRegistrationServiceV2(db.PgDB, liveAPIProvider)

	// Initialize aircraft livery service
	aircraftLiverySvc := common.NewAircraftLiveryService(legacyCache, repositories.AircraftLivery)

	// Initialize Airtable provider
	airtableProvider := providers.NewAirtableProvider(cacheSvc)

	// Initialize URL Signer service for presigned dashboard links
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("dev-secret-change-in-production")
	}
	urlSignerSvc := common.NewURLSignerService(jwtSecret, redisClient)

	// Initialize session service for UI authentication
	sessionSvc := common.NewSessionService(redisClient)

	// Initialize World Tour service
	worldTourSvc := services.NewWorldTourService(repositories.WorldTour)

	svc := &Services{
		User:               userSvc,
		Reg:                nil, // TODO: Migrate RegistrationService to GORM or remove (use RegV2)
		RegV2:              regServiceV2,
		Conf:               confSvc,
		VaMgmt:             nil, // TODO: Migrate VAManagementService to GORM or deprecate
		AirtableApi:        common.NewAirtableApiService(confSvc),
		AirtableProvider:   airtableProvider,
		AirtableSync:       services.NewAtSyncService(legacyCache, repositories.UserVASync),
		Flights:            services.NewFlightsService(legacyCache, liveSvc, confSvc, aircraftLiverySvc),
		PilotStats:         pilotStatsSvc,
		DataProviderConfig: dataProviderConfigSvc,
		AircraftLivery:     aircraftLiverySvc,
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
