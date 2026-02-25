package api

import (
	"infinite-experiment/politburo/internal/common"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/services"
	"net/http"
	"time"
)

// VAConfigHandlers handles VA configuration endpoints
type VAConfigHandlers struct {
	vaRepo *repositories.VAGormRepository
}

// NewVAConfigHandlers creates a new VA config handlers instance
func NewVAConfigHandlers(deps *Dependencies) *VAConfigHandlers {
	return &VAConfigHandlers{
		vaRepo: deps.Repo.Va,
	}
}

// SetFlightModesConfig handles POST /api/v1/va/flight-modes/config
// Stores or updates flight mode configuration for a VA (admin-only)
func (h *VAConfigHandlers) SetFlightModesConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaDiscordServerID := claims.DiscordServerID()

		// Validate VA exists
		if vaDiscordServerID == "" {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Get VA by Discord Server ID
		vaGorm, err := h.vaRepo.GetByDiscordServerID(r.Context(), vaDiscordServerID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to fetch VA configuration", http.StatusInternalServerError)
			return
		}

		if vaGorm == nil {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Parse request body
		var configPayload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&configPayload); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Use service to validate and save configuration
		configSvc := services.NewFlightModesConfigService(h.vaRepo)
		if err := configSvc.ValidateAndSaveConfig(r.Context(), vaGorm.ID, configPayload); err != nil {
			common.RespondError(w, initTime, err, "Invalid configuration", http.StatusBadRequest)
			return
		}

		// Get the number of modes for response
		flightModes := configPayload["flight_modes"].(map[string]interface{})

		response := map[string]interface{}{
			"success": true,
			"message": "Flight modes configuration saved successfully",
			"va_id":   vaGorm.ID,
			"modes":   len(flightModes),
		}

		common.RespondSuccess(w, initTime, "Flight modes configuration saved successfully", response)
	}
}
