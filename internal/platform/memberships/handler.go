package memberships

// TODO: Re-enable when Service is implemented

/*
import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/platform/va"
)

// Handler consolidates membership management API handlers
type Handler struct {
	svc      *Service
	vaSvc    *va.Service
	userRepo *users.Repository
}

// NewHandler creates a new membership Handler instance
func NewHandler(
	svc *Service,
	vaSvc *va.Service,
	userRepo *users.Repository,
) *Handler {
	return &Handler{
		svc:      svc,
		vaSvc:    vaSvc,
		userRepo: userRepo,
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
		vaRecord, err := h.vaSvc.GetByDiscordServerID(r.Context(), claims.DiscordServerID())
		if err != nil || vaRecord == nil {
			common.RespondError(w, initTime, err, "VA not found", http.StatusNotFound)
			return
		}

		// Insert membership with pilot role
		_, err = h.userRepo.InsertMembership(r.Context(), user.ID, vaRecord.ID, string(roles.RolePilot), req.Callsign)
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
		vaRecord, err := h.vaSvc.GetByDiscordServerID(r.Context(), claims.DiscordServerID())
		if err != nil || vaRecord == nil {
			common.RespondError(w, initTime, err, "VA not found", http.StatusNotFound)
			return
		}

		// Update user role
		err = h.userRepo.UpdateUserRole(r.Context(), vaRecord.ID, *membership.UserID, req.Role)
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Role updated!", nil)
	}
}
*/
