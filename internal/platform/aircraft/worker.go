package aircraft

import (
	"context"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"
	"infinite-experiment/politburo/internal/constants"
)

// Worker syncs aircraft/livery data from Infinite Flight API every 6 hours
type Worker struct {
	c          *cache.CacheInterface
	api        *liveapi.Client
	liveryRepo *Repository
	liverySvc  *Service
	metrics    *metrics.MetricsRegistry
}

// NewWorker creates a new aircraft sync worker
func NewWorker(
	c *cache.CacheInterface,
	api *liveapi.Client,
	liveryRepo *Repository,
	liverySvc *Service,
	metricsReg *metrics.MetricsRegistry,
) *Worker {
	return &Worker{
		c:          c,
		api:        api,
		liveryRepo: liveryRepo,
		liverySvc:  liverySvc,
		metrics:    metricsReg,
	}
}

// Start begins the worker's 6-hour sync loop
func (w *Worker) Start() {
	ticker := time.NewTicker(6 * time.Hour) // 4x daily sync
	defer ticker.Stop()

	// Initial sync on startup
	w.syncAircraftLiveriesTask()
	w.refillWorldStatus()

	for range ticker.C {
		w.refillWorldStatus()
		w.syncAircraftLiveriesTask()
	}
}

// syncAircraftLiveriesTask syncs aircraft/livery data from IF API to database with change detection
func (w *Worker) syncAircraftLiveriesTask() {
	ctx := context.Background()
	startTime := time.Now()
	defer func() {
		if w.metrics != nil {
			w.metrics.SyncJobDuration.WithLabelValues("aircraft_cache_job", "liveapi", "aircraft").Observe(time.Since(startTime).Seconds())
		}
	}()

	// Fetch liveries from Infinite Flight API
	resp, _, err := w.api.GetAircraftLiveries()
	if err != nil {
		logging.Error("Failed to fetch liveries from IF API", "error", err)
		return
	}

	// Check for API errors
	if resp.ErrorCode != 0 {
		logging.Error("IF API returned error code", "error_code", resp.ErrorCode)
		return
	}

	// Load existing liveries from database into map for change detection
	existingLiveries, err := w.liveryRepo.GetLiveryMap(ctx)
	if err != nil {
		logging.Error("Failed to load existing liveries from database", "error", err)
		return
	}

	// Track changes
	var toUpsert []AircraftLivery
	apiLiveryIDs := make(map[string]bool)
	addedCount := 0
	updatedCount := 0

	// Process each API livery
	for _, apiLivery := range resp.Liveries {
		apiLiveryIDs[apiLivery.LiveryId] = true

		if existingLivery, exists := existingLiveries[apiLivery.LiveryId]; exists {
			// Check if fields changed
			if existingLivery.AircraftName != apiLivery.AircraftName ||
				existingLivery.LiveryName != apiLivery.LiveryName ||
				existingLivery.AircraftID != apiLivery.AircraftID ||
				!existingLivery.IsActive {
				// Update needed
				toUpsert = append(toUpsert, ConvertLiveAPILiveryToGORM(apiLivery))
				updatedCount++
			}
		} else {
			// New livery
			toUpsert = append(toUpsert, ConvertLiveAPILiveryToGORM(apiLivery))
			addedCount++
		}
	}

	// Find removed liveries (in DB but not in API response)
	var removedIDs []string
	for liveryID := range existingLiveries {
		if !apiLiveryIDs[liveryID] {
			removedIDs = append(removedIDs, liveryID)
		}
	}

	// Execute database updates if changes detected
	hasChanges := len(toUpsert) > 0 || len(removedIDs) > 0

	if len(toUpsert) > 0 {
		if err := w.liveryRepo.UpsertBatch(ctx, toUpsert); err != nil {
			logging.Error("Failed to upsert liveries", "error", err)
			return
		}
	}

	if len(removedIDs) > 0 {
		if err := w.liveryRepo.MarkInactive(ctx, removedIDs); err != nil {
			logging.Error("Failed to mark liveries inactive", "error", err)
			return
		}
	}

	// Warm cache if changes detected (as per user requirement)
	if hasChanges {
		if err := w.liverySvc.WarmCache(ctx); err != nil {
			logging.Error("Failed to warm livery cache", "error", err)
		}
	}

	if w.metrics != nil {
		total := float64(addedCount + updatedCount)
		w.metrics.SyncJobRecordsProcessed.WithLabelValues("aircraft_cache_job", "liveapi", "aircraft", "_", "success").Add(total)
	}

	logging.Info("Livery sync completed",
		"duration", time.Since(startTime),
		"added", addedCount,
		"updated", updatedCount,
		"removed", len(removedIDs),
		"api_total", len(resp.Liveries),
		"db_total", len(existingLiveries),
	)
}

// refillWorldStatus caches world status metadata (separate concern, kept for now)
func (w *Worker) refillWorldStatus() {
	resp, err := w.api.GetSessions()
	if err != nil {
		return
	}

	c := *w.c
	c.Set(string(constants.CachePrefixWorldDetails), resp.Result, cache.WorldDetailsTTL)
	for _, world := range resp.Result {
		// Get expert server
		if world.WorldType == 3 {
			c.Set(string(constants.CachePrefixExpertServer), world.ID, cache.WorldDetailsTTL)
			break
		}
	}
}
