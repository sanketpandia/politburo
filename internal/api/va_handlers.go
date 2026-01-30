package api

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
)

// VAHandlers consolidates VA management API handlers
type VAHandlers struct {
	userRepo    *repositories.UserRepositoryGORM
	vaRepo      *repositories.VAGormRepository
	vaConfigSvc *common.VAConfigService
}

// NewVAHandlers creates a new VAHandlers instance
func NewVAHandlers(deps *Dependencies) *VAHandlers {
	return &VAHandlers{
		userRepo:    deps.Repo.User,
		vaRepo:      deps.Repo.Va,
		vaConfigSvc: deps.Services.Conf,
	}
}

// SyncUser handles POST /api/v1/va/userSync
// Syncs a user to the current VA with a callsign (creates membership)
func (h *VAHandlers) SyncUser() http.HandlerFunc {
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
		va, err := h.vaRepo.GetByDiscordServerID(r.Context(), claims.DiscordServerID())
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
func (h *VAHandlers) SetRole() http.HandlerFunc {
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
		va, err := h.vaRepo.GetByDiscordServerID(r.Context(), claims.DiscordServerID())
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
func (h *VAHandlers) GetConfigs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		configs, _ := h.vaConfigSvc.GetAllConfigValues(r.Context(), claims.ServerID())

		common.RespondSuccess(w, initTime, "VA configuration fetched", configs)
	}
}

// ListConfigKeys handles GET /api/v1/va/configs/keys
// Returns list of all possible configuration keys
func (h *VAHandlers) ListConfigKeys() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		keys := h.vaConfigSvc.ListPossibleKeys()

		common.RespondSuccess(w, initTime, "Configuration keys listed", dtos.VAConfigKeys{
			ConfigKeys: keys,
		})
	}
}

// SetConfigs handles POST /api/v1/va/configs
// Sets configuration values for the current VA
func (h *VAHandlers) SetConfigs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		configs := make(map[string]string)
		if err := json.NewDecoder(r.Body).Decode(&configs); err != nil {
			common.RespondError(w, initTime, err, "Invalid JSON", http.StatusBadRequest)
			return
		}

		result, err := h.vaConfigSvc.SetVaConfig(r.Context(), configs)
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Config set successfully", result)
	}
}
