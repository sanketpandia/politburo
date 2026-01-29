package api

import (
	"encoding/json"
	"infinite-experiment/politburo/internal/models/entities"
	"net/http"
	"time"

	"gorm.io/gorm"
)

// HealthCheckHandler handles GET /healthCheck
//
// @Summary Health check
// @Description Verifies the server is running.
// @Tags Misc
// @Success 200 {string} string "ok"
// @Router /healthCheck [get]
func HealthCheckHandler(db *gorm.DB, upSince time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		services := make(map[string]entities.ServiceStatus)

		// Check postgres
		pgstatus := "ok"
		pgDetails := "Postgres Connected"
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
