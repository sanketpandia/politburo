package api

import (
	"infinite-experiment/politburo/internal/common"
	"encoding/json"
	"infinite-experiment/politburo/internal/jobs"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/services"
	"net/http"
	"time"
)

func DebugHandler(svc common.AirtableApiService, svc1 services.AtSyncService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		jobs.SyncRoutesJob(r.Context(), &svc, &svc1, nil)

		resp := dtos.APIResponse{
			ResponseTime: common.GetResponseTime(initTime),
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}
