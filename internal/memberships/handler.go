package memberships

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/pilots"
	"infinite-experiment/politburo/internal/platform/httpdto"
	platformMemberships "infinite-experiment/politburo/internal/platform/memberships"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

// Handler provides HTTP handlers for membership-related endpoints
type Handler struct {
	svc         *Service
	pilotRepo   *pilots.Repository
	vaConfigSvc *platformVA.ConfigService
}

// NewHandler creates a new memberships handler
func NewHandler(svc *Service, pilotRepo *pilots.Repository, vaConfigSvc *platformVA.ConfigService) *Handler {
	return &Handler{
		svc:         svc,
		pilotRepo:   pilotRepo,
		vaConfigSvc: vaConfigSvc,
	}
}

// GetUserStatus handles GET /api/v1/user/status
// Returns comprehensive user status including all VA affiliations and current VA context
func (h *Handler) GetUserStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Extract claims (populated by auth middleware)
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			logging.Warn("Unauthorized request to /user/status - missing claims")
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		// Get user ID and VA ID from claims
		userID := claims.UserID()   // Use user ID, not discord ID
		vaID := claims.ServerID()   // VA ID from claims

		// If user ID is empty, user doesn't exist
		if userID == "" {
			logging.Warn("User status request with empty user ID", "va_id", vaID)
			httpdto.WriteError(w, initTime, "USER_NOT_FOUND", "User not found", http.StatusNotFound)
			return
		}

		logging.Info("User status request", "user_id", userID, "va_id", vaID, "endpoint", "/user/status")

		// Call service
		userStatus, err := h.svc.GetUserStatus(r.Context(), userID, vaID)
		if err != nil {
			// Check if user was not found
			if errors.Is(err, platformMemberships.ErrUserNotFound) {
				logging.Warn("User not found", "user_id", userID)
				httpdto.WriteError(w, initTime, "USER_NOT_FOUND", "User not found", http.StatusNotFound)
				return
			}

			logging.Error("Failed to fetch user status", "error", err, "user_id", userID)
			httpdto.WriteError(w, initTime, "FETCH_FAILED", "Failed to fetch user status", http.StatusInternalServerError)
			return
		}

		httpdto.WriteSuccess(w, initTime, userStatus, http.StatusOK)
	}
}

// JoinVA handles POST /api/v1/memberships/join
// Allows authenticated user to join a VA with a callsign
func (h *Handler) JoinVA() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// 1. Extract and validate claims
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			logging.Warn("Unauthorized request to /memberships/join - missing claims")
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		// 2. Check if user is already a member (has a role in this VA)
		if claims.Role() != "" {
			logging.Warn("User already has membership in this VA",
				"discord_user_id", claims.DiscordUserID(),
				"va_id", claims.ServerID(),
				"existing_role", claims.Role())
			httpdto.WriteError(w, initTime, "ALREADY_MEMBER", "You are already a member of this VA", http.StatusConflict)
			return
		}

		discordUserID := claims.DiscordUserID()
		discordServerID := claims.DiscordServerID()

		// 3. Parse request body
		var req JoinVARequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logging.Warn("Invalid request body", "error", err)
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		// 4. Validate required fields
		if req.Callsign == "" {
			httpdto.WriteError(w, initTime, "MISSING_FIELD", "Callsign is required", http.StatusBadRequest)
			return
		}

		logging.Info("Membership join request", "discord_user_id", discordUserID, "callsign", req.Callsign)

		// 5. Call service
		result, err := h.svc.JoinVA(r.Context(), discordUserID, discordServerID, req.Callsign)

		if err != nil {
			logging.Error("Failed to join VA", "error", err, "discord_user_id", discordUserID)
			h.handleJoinVAError(w, r, initTime, err, claims.ServerID())
			return
		}

		logging.Info("User joined VA successfully", "user_id", result.UserID, "va_id", result.VAID)
		httpdto.WriteSuccess(w, initTime, result, http.StatusCreated)
	}
}

// handleJoinVAError maps domain errors to HTTP responses
func (h *Handler) handleJoinVAError(w http.ResponseWriter, r *http.Request, initTime time.Time, err error, vaID string) {
	if errors.Is(err, ErrUserNotFound) {
		httpdto.WriteError(w, initTime, "USER_NOT_FOUND", "User not found", http.StatusNotFound)
		return
	}

	if errors.Is(err, ErrVANotFound) {
		httpdto.WriteError(w, initTime, "VA_NOT_FOUND", "Virtual airline not found", http.StatusNotFound)
		return
	}

	if errors.Is(err, ErrUserAlreadyMember) {
		httpdto.WriteError(w, initTime, "ALREADY_MEMBER", "You are already a member of this VA", http.StatusConflict)
		return
	}

	if errors.Is(err, ErrCallsignTaken) {
		httpdto.WriteError(w, initTime, "CALLSIGN_TAKEN", "This callsign is already taken", http.StatusConflict)
		return
	}

	if errors.Is(err, ErrInvalidCallsign) {
		httpdto.WriteError(w, initTime, "INVALID_CALLSIGN", "Invalid callsign format", http.StatusBadRequest)
		return
	}

	if errors.Is(err, ErrCallsignNotInAirtable) {
		// Get sample callsigns and Airtable URL
		var sampleCallsigns []string
		var airtableURL string

		if h.pilotRepo != nil && vaID != "" {
			// Get sample callsigns
			samples, err := h.pilotRepo.GetSampleCallsigns(r.Context(), vaID, 3)
			if err == nil && len(samples) > 0 {
				sampleCallsigns = samples
			}
		}

		if h.vaConfigSvc != nil && vaID != "" {
			// Get Airtable base ID from config
			configs, err := h.vaConfigSvc.GetAllConfigValues(r.Context(), vaID)
			if err == nil {
				if baseID, ok := configs[platformVA.ConfigKeyAirtableVABase]; ok && baseID != "" {
					airtableURL = fmt.Sprintf("https://airtable.com/%s", baseID)
				}
			}
		}

		// Build error message
		message := "Your callsign could not be found in Airtable. Please enter the correct callsign as it appears in the linked Airtable."
		
		if airtableURL != "" {
			message += fmt.Sprintf(" You can find your callsign in the Airtable: %s", airtableURL)
		}

		if len(sampleCallsigns) > 0 {
			message += fmt.Sprintf(" Sample callsigns from the database: %s", strings.Join(sampleCallsigns, ", "))
		}

		httpdto.WriteError(w, initTime, "CALLSIGN_NOT_IN_AIRTABLE", message, http.StatusBadRequest)
		return
	}

	// Default error
	httpdto.WriteError(w, initTime, "INTERNAL_ERROR", "An unexpected error occurred", http.StatusInternalServerError)
}
