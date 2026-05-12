package health

import (
	"encoding/json"
	"net/http"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/models/entities"

	"gorm.io/gorm"
)

// HealthCheckHandler returns an HTTP handler that reports the liveness of critical services.
func HealthCheckHandler(db *gorm.DB, redisCache *cache.RedisCacheService, upSince time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		services := make(map[string]entities.ServiceStatus)

		// Check postgres
		pgStatus := "ok"
		pgDetails := "postgres connected"
		sqlDB, err := db.DB()
		if err != nil {
			pgStatus = "down"
			pgDetails = err.Error()
		} else if err := sqlDB.Ping(); err != nil {
			pgStatus = "down"
			pgDetails = err.Error()
		}
		services["postgres"] = entities.ServiceStatus{
			Status:  pgStatus,
			Details: pgDetails,
		}

		// Check redis cache
		cacheStatus := "ok"
		cacheDetails := "redis connected"
		if redisCache != nil {
			if err := redisCache.Ping(); err != nil {
				cacheStatus = "down"
				cacheDetails = err.Error()
			}
		} else {
			cacheStatus = "down"
			cacheDetails = "redis client not initialized"
		}
		services["cache"] = entities.ServiceStatus{
			Status:  cacheStatus,
			Details: cacheDetails,
		}

		overallStatus := "ok"
		for _, svc := range services {
			if svc.Status != "ok" {
				overallStatus = "down"
				break
			}
		}

		now := time.Now()
		resp := entities.HealthCheckResponse{
			Services: services,
			Status:   overallStatus,
			UpSince:  upSince,
			Uptime:   now.Sub(upSince).Round(time.Second).String(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
