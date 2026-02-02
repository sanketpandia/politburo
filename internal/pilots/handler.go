package pilots

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/platform/httpdto"
)

// Handler handles pilot statistics and registration endpoints
type Handler struct {
	statsSvc *StatsService
	regSvc   *RegistrationService
}

// NewHandler creates a new pilot handler instance
func NewHandler(statsSvc *StatsService, regSvc *RegistrationService) *Handler {
	return &Handler{
		statsSvc: statsSvc,
		regSvc:   regSvc,
	}
}

// RegisterPilot handles POST /api/v1/pilots/register
// Registers a new pilot with IFC credentials and returns VA registration status
func (h *Handler) RegisterPilot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// 1. Extract and validate claims
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			logging.Warn("Unauthorized request to /pilots/register - missing claims")
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		discordUserID := claims.DiscordUserID()
		discordServerID := claims.DiscordServerID()
		userID := claims.UserID()

		// 2. Duplicate check (simplified via claims)
		if userID != "" {
			logging.Warn("User already registered", "user_id", userID, "discord_id", discordUserID)
			httpdto.WriteError(w, initTime, "USER_ALREADY_REGISTERED", "User is already registered", http.StatusConflict)
			return
		}

		// 3. Parse request body
		var req RegisterPilotRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logging.Warn("Invalid request body", "error", err)
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		// 4. Validate required fields
		if req.IfcId == "" {
			logging.Warn("Missing IFC ID in request")
			httpdto.WriteError(w, initTime, "MISSING_FIELD", "IFC ID is required", http.StatusBadRequest)
			return
		}

		if req.LastFlight == "" {
			logging.Warn("Missing last flight in request")
			httpdto.WriteError(w, initTime, "MISSING_FIELD", "Last flight is required", http.StatusBadRequest)
			return
		}

		logging.Info("Pilot registration request", "discord_id", discordUserID, "ifc_id", req.IfcId, "server_id", discordServerID)

		// 5. Call service
		result, err := h.regSvc.RegisterPilot(
			r.Context(),
			discordUserID,
			discordServerID,
			req.IfcId,
			req.LastFlight,
		)

		if err != nil {
			logging.Error("Failed to register pilot", "error", err, "discord_id", discordUserID)
			h.handleRegistrationError(w, initTime, err)
			return
		}

		logging.Info("Pilot registered successfully", "discord_id", discordUserID, "is_va", result.IsVARegistered)
		httpdto.WriteSuccess(w, initTime, result, http.StatusCreated)
	}
}

func (h *Handler) PilotStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

	}
}

// // GetPilotStats handles GET /api/v1/pilot/stats
// // Returns comprehensive pilot statistics from the configured data provider
// func (h *Handler) GetPilotStats() http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		initTime := time.Now()

// 		// Get claims from context
// 		claims := auth.GetUserClaims(r.Context())
// 		if claims == nil {
// 			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
// 			return
// 		}

// 		userDiscordID := claims.DiscordUserID()
// 		vaDiscordServerID := claims.DiscordServerID()
// 		vaUUID := claims.ServerID()

// 		// Validate VA exists
// 		if vaDiscordServerID == "" {
// 			common.RespondError(w, initTime, fmt.Errorf("not in a VA Server"), "Virtual airline not found", http.StatusNotFound)
// 			return
// 		}

// 		// Fetch pilot stats (returns standardized mapped data)
// 		stats, err := h.statsSvc.GetPilotStats(r.Context(), userDiscordID, vaUUID)
// 		if err != nil {
// 			h.handlePilotStatsError(w, initTime, err)
// 			return
// 		}

// 		common.RespondSuccess(w, initTime, "Pilot stats fetched successfully", stats)
// 	}
// }

// handleRegistrationError maps registration service errors to appropriate HTTP responses
func (h *Handler) handleRegistrationError(w http.ResponseWriter, initTime time.Time, err error) {
	// Check specific error types
	if errors.Is(err, ErrIFCUserNotFound) {
		httpdto.WriteError(w, initTime, "IFC_USER_NOT_FOUND", "IFC user not found. Please verify your IFC username.", http.StatusNotFound)
		return
	}

	if errors.Is(err, ErrNoRecentFlights) {
		httpdto.WriteError(w, initTime, "NO_RECENT_FLIGHTS", "No recent flights found in your logbook.", http.StatusBadRequest)
		return
	}

	if errors.Is(err, ErrFlightMismatch) {
		httpdto.WriteError(w, initTime, "FLIGHT_MISMATCH", "Last flight verification failed. Please verify your last flight route.", http.StatusBadRequest)
		return
	}

	if errors.Is(err, ErrRegistrationFailed) {
		httpdto.WriteError(w, initTime, "REGISTRATION_FAILED", "Failed to register user. Please try again.", http.StatusInternalServerError)
		return
	}

	// Default to internal server error
	httpdto.WriteError(w, initTime, "INTERNAL_ERROR", "An unexpected error occurred during registration.", http.StatusInternalServerError)
}

// handlePilotStatsError maps service errors to appropriate HTTP responses
func (h *Handler) handlePilotStatsError(w http.ResponseWriter, initTime time.Time, err error) {
	// Check if it's a StatsError with specific error code
	if statsErr, ok := err.(*StatsError); ok {
		statusCode := h.mapErrorCodeToHTTPStatus(statsErr.Code)
		message := statsErr.Message

		// Use the error code in the message if available
		if statsErr.Code != "" {
			message = constants.GetErrorMessage(statsErr.Code)
		}

		common.RespondError(w, initTime, err, message, statusCode)
		return
	}

	// Default to internal server error for unknown errors
	common.RespondError(w, initTime, err, "An unexpected error occurred", http.StatusInternalServerError)
}

// mapErrorCodeToHTTPStatus maps error codes to HTTP status codes
func (h *Handler) mapErrorCodeToHTTPStatus(errorCode string) int {
	switch errorCode {
	// 400 Bad Request - Client errors (user action required)
	case constants.ErrCodePilotNotSynced:
		return http.StatusBadRequest
	case constants.ErrCodePilotAirtableIDMissing:
		return http.StatusBadRequest
	case constants.ErrCodeConfigMalformed:
		return http.StatusBadRequest

	// 404 Not Found - Resource doesn't exist
	case constants.ErrCodeConfigNotFound:
		return http.StatusNotFound
	case constants.ErrCodePilotNotFoundInAirtable:
		return http.StatusNotFound
	case constants.ErrCodeVAAirtableNotEnabled:
		return http.StatusNotFound
	case constants.ErrCodeTableNotFound:
		return http.StatusNotFound
	case constants.ErrCodeInvalidBaseID:
		return http.StatusNotFound

	// 401 Unauthorized - Authentication failed
	case constants.ErrCodeInvalidAPIKey:
		return http.StatusUnauthorized
	case constants.ErrCodeAuthenticationFailed:
		return http.StatusUnauthorized

	// 403 Forbidden - Authenticated but no permission
	case constants.ErrCodeTableAccessDenied:
		return http.StatusForbidden

	// 429 Too Many Requests - Rate limiting
	case constants.ErrCodeRateLimited:
		return http.StatusTooManyRequests

	// 500 Internal Server Error - System/network errors (default)
	case constants.ErrCodeNetworkError:
		return http.StatusInternalServerError
	case constants.ErrCodeValidationTimeout:
		return http.StatusInternalServerError

	default:
		return http.StatusInternalServerError
	}
}
