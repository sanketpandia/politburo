package workers

import (
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/queue"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/platform/aircraft"

	"gorm.io/gorm"
)

type WorkersContainer struct {
	// MetaCacheWorker removed - now in aircraft package
}

func InitWorkers(
	db *gorm.DB,
	c *cache.CacheInterface,
	liveAPI *common.LiveAPIService,
	aircraftSvc *aircraft.Service,
	redQ *queue.RedisQueueService,
	dataProvCfg *repositories.DataProviderConfigRepo,
) *WorkersContainer {
	// Aircraft worker (MetaCacheFiller) moved to aircraft package - initialized in router.go

	// Start the logbook worker to cache flight routes on-demand
	go LogbookWorker(*c, liveAPI, aircraftSvc)

	// DISABLED: PIREP workers disabled pending functionality redesign
	// qWorker := NewPirepQueueWorker("pirep_queue", db, redQ, dataProvCfg, pirepSyncedRepo, vaSyncHRepo)
	// monitor := NewPirepQueueMonitor(db, redQ)
	// go qWorker.Start(context.Background(), 5)
	// go monitor.Start(context.Background(), 30*time.Second)

	return &WorkersContainer{}
}
