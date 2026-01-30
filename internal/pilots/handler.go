package pilots

import (
	"fmt"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/constants"
)

// Handler handles pilot statistics endpoints
type Handler struct {
	statsSvc *StatsService
}

// NewHandler creates a new pilot stats handler instance
func NewHandler(statsSvc *StatsService) *Handler {
	return &Handler{
		statsSvc: statsSvc,
	}
}

// GetPilotStats handles GET /api/v1/pilot/stats
// Returns comprehensive pilot statistics from the configured data provider
func (h *Handler) GetPilotStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		userDiscordID := claims.DiscordUserID()
		vaDiscordServerID := claims.DiscordServerID()
		vaUUID := claims.ServerID()

		// Validate VA exists
		if vaDiscordServerID == "" {
			common.RespondError(w, initTime, fmt.Errorf("not in a VA Server"), "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Fetch pilot stats (returns standardized mapped data)
		stats, err := h.statsSvc.GetPilotStats(r.Context(), userDiscordID, vaUUID)
		if err != nil {
			h.handlePilotStatsError(w, initTime, err)
			return
		}

		common.RespondSuccess(w, initTime, "Pilot stats fetched successfully", stats)
	}
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
