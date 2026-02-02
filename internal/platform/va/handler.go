package va

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/services"
)

// Handler consolidates VA management API handlers
type Handler struct {
	svc          *Service
	configSvc    *ConfigService
	eventSvc     *EventService
	userRepo     *users.Repository
	legacyVARepo *repositories.VAGormRepository // Temporary: for legacy FlightModesConfigService
}

// NewHandler creates a new VA Handler instance
func NewHandler(
	svc *Service,
	configSvc *ConfigService,
	eventSvc *EventService,
	userRepo *users.Repository,
	legacyVARepo *repositories.VAGormRepository,
) *Handler {
	return &Handler{
		svc:          svc,
		configSvc:    configSvc,
		eventSvc:     eventSvc,
		userRepo:     userRepo,
		legacyVARepo: legacyVARepo,
	}
}

// SyncUser and SetRole handlers have been moved to internal/platform/memberships/handler.go

// GetConfigs handles GET /api/v1/va/configs
// Returns all configuration values for the current VA
func (h *Handler) GetConfigs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		configs, _ := h.configSvc.GetAllConfigValues(r.Context(), claims.ServerID())

		common.RespondSuccess(w, initTime, "VA configuration fetched", configs)
	}
}

// ListConfigKeys handles GET /api/v1/va/configs/keys
// Returns list of all possible configuration keys
func (h *Handler) ListConfigKeys() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		keys := h.configSvc.ListPossibleKeys()

		common.RespondSuccess(w, initTime, "Configuration keys listed", dtos.VAConfigKeys{
			ConfigKeys: keys,
		})
	}
}

// SetConfigs handles POST /api/v1/va/configs
// Sets configuration values for the current VA
func (h *Handler) SetConfigs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		configs := make(map[string]string)
		if err := json.NewDecoder(r.Body).Decode(&configs); err != nil {
			common.RespondError(w, initTime, err, "Invalid JSON", http.StatusBadRequest)
			return
		}

		result, err := h.configSvc.SetVaConfig(r.Context(), configs)
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Config set successfully", result)
	}
}

// SetFlightModesConfig handles POST /api/v1/va/flight-modes/config
// Stores or updates flight mode configuration for a VA (admin-only)
func (h *Handler) SetFlightModesConfig() http.HandlerFunc {
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
		vaGorm, err := h.svc.GetByDiscordServerID(r.Context(), vaDiscordServerID)
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
		// Note: This still uses the services package FlightModesConfigService
		// We may want to move this into VA package later
		configSvc := services.NewFlightModesConfigService(h.legacyVARepo)
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
