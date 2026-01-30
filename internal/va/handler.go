package va

import (
	"infinite-experiment/politburo/internal/common"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/services"
)

// Handler consolidates VA management API handlers
type Handler struct {
	svc            *Service
	configSvc      *ConfigService
	eventSvc       *EventService
	userRepo       *repositories.UserRepositoryGORM
	legacyVARepo   *repositories.VAGormRepository  // Temporary: for legacy FlightModesConfigService
}

// NewHandler creates a new VA Handler instance
func NewHandler(
	svc *Service,
	configSvc *ConfigService,
	eventSvc *EventService,
	userRepo *repositories.UserRepositoryGORM,
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

// SyncUser handles POST /api/v1/va/userSync
// Syncs a user to the current VA with a callsign (creates membership)
func (h *Handler) SyncUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		var req dtos.SyncUser
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid JSON", http.StatusBadRequest)
			return
		}

		log.Printf("Sync request received: userID=%s callsign=%s", req.UserID, req.Callsign)

		// Check if user is registered
		user, err := h.userRepo.GetUserByDiscordID(r.Context(), req.UserID)
		if err != nil {
			common.RespondError(w, initTime, err, fmt.Sprintf("User not registered: %s", err.Error()), http.StatusBadRequest)
			return
		}

		// Check if user already has membership in this VA
		membership, err := h.userRepo.FindUserMembership(r.Context(), claims.DiscordServerID(), req.UserID)
		if err == nil && membership != nil && membership.Role != nil {
			common.RespondError(w, initTime, fmt.Errorf("user already synced"), "User already synced to this VA", http.StatusConflict)
			return
		}

		// Get VA record to get the VA ID
		va, err := h.svc.GetByDiscordServerID(r.Context(), claims.DiscordServerID())
		if err != nil || va == nil {
			common.RespondError(w, initTime, err, "VA not found", http.StatusNotFound)
			return
		}

		// Insert membership with pilot role
		_, err = h.userRepo.InsertMembership(r.Context(), user.ID, va.ID, string(roles.RolePilot), req.Callsign)
		if err != nil {
			common.RespondError(w, initTime, err, fmt.Sprintf("Failed to sync user: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "User synced successfully", nil)
	}
}

// SetRole handles POST /api/v1/va/setRole
// Updates a user's role in the current VA (admin-only)
func (h *Handler) SetRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		var req dtos.SetRole
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.RespondError(w, initTime, err, "Invalid JSON", http.StatusBadRequest)
			return
		}

		log.Printf("Set role request: userID=%s role=%s", req.UserID, req.Role)

		// Check if user has membership in this VA
		membership, err := h.userRepo.FindUserMembership(r.Context(), claims.DiscordServerID(), req.UserID)
		if err != nil || membership == nil {
			common.RespondError(w, initTime, err, "User is not synced to this VA; please sync before assigning a role", http.StatusBadRequest)
			return
		}

		// Get VA record
		va, err := h.svc.GetByDiscordServerID(r.Context(), claims.DiscordServerID())
		if err != nil || va == nil {
			common.RespondError(w, initTime, err, "VA not found", http.StatusNotFound)
			return
		}

		// Update user role
		err = h.userRepo.UpdateUserRole(r.Context(), va.ID, *membership.UserID, req.Role)
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Role updated!", nil)
	}
}

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
