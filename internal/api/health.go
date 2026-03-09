package api

import (
	"encoding/json"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/models/entities"
	"net/http"
	"time"

	"gorm.io/gorm"
)

func HealthCheckHandler(db *gorm.DB, cache *cache.RedisCacheService, upSince time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		services := make(map[string]entities.ServiceStatus)

		// Check postgres
		pgstatus := "ok"
		pgDetails := "postgres connected"

		cacheStatus := "ok"
		cacheDetails := "redis connected"
		sqlDB, err := db.DB()
		if err != nil {
			pgstatus = "down"
			pgDetails = err.Error()
		} else if err := sqlDB.Ping(); err != nil {
			pgstatus = "down"
			pgDetails = err.Error()
		}
		services["postgres"] = entities.ServiceStatus{
			Status:  pgstatus,
			Details: pgDetails,
		}

		if cache != nil {
			if err := cache.Ping(); err != nil {
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
		uptime := now.Sub(upSince).Round(time.Second).String()

		resp := entities.HealthCheckResponse{
			Services: services,
			Status:   overallStatus,
			Uptime:   uptime,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
