package api

import (
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/auth"
	"net/http"
	"time"
)

// SyncAirportsHandler handles GET /api/v1/admin/sync-airports
// Syncs airport data from the embedded airports dataset
func SyncAirportsHandler(airportLoader *common.AirportLoaderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		// Check if user is god-mode user
		if !auth.IsGodMode(claims.DiscordUserID()) {
			common.RespondError(w, initTime, nil, "Unauthorized: god-mode required", http.StatusForbidden)
			return
		}

		// Load airports from embedded JSON
		count, err := airportLoader.LoadAirportsFromEmbedded(r.Context())
		if err != nil {
			common.RespondError(w, initTime, nil, "Failed to sync airports: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Get stats after loading
		stats, err := airportLoader.GetStats(r.Context())
		if err != nil {
			common.RespondError(w, initTime, nil, "Failed to get stats: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"imported": count,
			"stats":    stats,
		}

		common.RespondSuccess(w, initTime, "Airports synced successfully", response)
	}
}
