package pilots

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/httpdto"
	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/platform/validation"

	"github.com/go-chi/chi/v5"
)

// Handler handles pilot statistics and registration endpoints
type Handler struct {
	statsSvc   *StatsService
	regSvc     registrationHandlerService
	logbookSvc *LogbookService
	usersSvc   logbookUserLookupService
}

type registrationHandlerService interface {
	RegisterPilot(ctx context.Context, discordUserID string, discordServerID string, ifcId string, lastFlight string) (*RegisterPilotResponse, *RegistrationError)
}

type logbookUserLookupService interface {
	GetByDiscordID(ctx context.Context, discordID string) (*users.User, error)
}

// NewHandler creates a new pilot handler instance
func NewHandler(statsSvc *StatsService, regSvc registrationHandlerService, logbookSvc *LogbookService, usersSvc logbookUserLookupService) *Handler {
	return &Handler{
		statsSvc:   statsSvc,
		regSvc:     regSvc,
		logbookSvc: logbookSvc,
		usersSvc:   usersSvc,
	}
}

// RegisterPilot handles POST /api/v1/user/register
// Registers a new pilot with IFC credentials and returns VA registration status
func (h *Handler) RegisterPilot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// 1. Extract and validate claims
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			logging.Warn("Unauthorized request to /user/register - missing claims")
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

		// 3. Decode and validate request body
		var req RegisterPilotRequest
		if decodeErr, ve := validation.DecodeAndValidate(r, &req); decodeErr != nil {
			logging.Warn("Invalid request body", "error", decodeErr)
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		} else if ve != nil {
			httpdto.WriteValidationError(w, initTime, ve)
			return
		}

		logging.Info("Pilot registration request", "discord_id", discordUserID, "ifc_id", req.IfcId, "server_id", discordServerID)

		// 4. Call service
		result, svcErr := h.regSvc.RegisterPilot(
			r.Context(),
			discordUserID,
			discordServerID,
			req.IfcId,
			req.LastFlight,
		)

		if svcErr != nil {
			logging.Error("Failed to register pilot", "error", svcErr, "discord_id", discordUserID)
			httpdto.WriteError(w, initTime, svcErr.Code, svcErr.Message, svcErr.StatusCode)
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

// GetPilotStats handles GET /api/v1/pilot/stats
// Returns comprehensive pilot statistics from the configured data provider
func (h *Handler) GetPilotStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		userDiscordID := claims.DiscordUserID()
		vaUUID := claims.ServerID()

		// Validate we have required IDs
		if userDiscordID == "" {
			httpdto.WriteError(w, initTime, "MISSING_DISCORD_ID", "Discord user ID not found in claims", http.StatusBadRequest)
			return
		}

		if vaUUID == "" {
			httpdto.WriteError(w, initTime, "MISSING_VA_ID", "VA ID not found in claims", http.StatusBadRequest)
			return
		}

		// Fetch pilot stats (returns standardized mapped data)
		stats, err := h.statsSvc.GetPilotStats(r.Context(), userDiscordID, vaUUID)
		if err != nil {
			h.handlePilotStatsError(w, initTime, err)
			return
		}

		// Check if stats response is empty (no data found)
		if stats == nil || (stats.GameStats == nil && stats.ProviderData == nil && stats.CareerModeData == nil) {
			httpdto.WriteError(w, initTime, "NO_STATS_FOUND", "No pilot statistics found. Make sure you're registered with /register and have connected your VA account!", http.StatusNotFound)
			return
		}

		httpdto.WriteSuccess(w, initTime, stats, http.StatusOK)
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

		httpdto.WriteError(w, initTime, statsErr.Code, message, statusCode)
		return
	}

	// Default to internal server error for unknown errors
	httpdto.WriteError(w, initTime, "INTERNAL_ERROR", "An unexpected error occurred", http.StatusInternalServerError)
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

// GetUserLogbook handles GET /api/v1/pilots/{ifc_id}/logbook?page=1
// Returns paginated flight history for a user by IFC ID
func (h *Handler) GetUserLogbook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Extract path parameter
		ifcID := chi.URLParam(r, "ifc_id")
		if ifcID == "" {
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid IFC ID", http.StatusBadRequest)
			return
		}

		// Parse query parameter 'page'
		page := 1
		if qs := r.URL.Query().Get("page"); qs != "" {
			p, err := strconv.Atoi(qs)
			if err != nil || p <= 0 {
				httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid page parameter", http.StatusBadRequest)
				return
			}
			page = p
		}

		// Get claims for logging/audit (not required for API call itself)
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		// Call service
		dto, err := h.logbookSvc.GetUserLogbook(ifcID, page)
		if err != nil {
			logging.Warn("Failed to fetch user logbook", "ifc_id", ifcID, "page", page, "error", err)
			httpdto.WriteError(w, initTime, "FETCH_ERROR", dto.Error, http.StatusInternalServerError)
			return
		}

		httpdto.WriteSuccess(w, initTime, dto, http.StatusOK)
	}
}

// GetUserLogbookSelf handles GET /api/v1/user/{ifc_id}/flights
// Returns a paginated logbook for any IFC user. A caller may always view their
// own logbook. Viewing another user's logbook requires the staff or admin role.
func (h *Handler) GetUserLogbookSelf() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		ifcID := chi.URLParam(r, "ifc_id")
		if ifcID == "" {
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid IFC ID", http.StatusBadRequest)
			return
		}

		page := 1
		if qs := r.URL.Query().Get("page"); qs != "" {
			p, err := strconv.Atoi(qs)
			if err != nil || p <= 0 {
				httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid page parameter", http.StatusBadRequest)
				return
			}
			page = p
		}

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		// Self-ownership check: look up the caller's stored IFC ID and compare.
		// Staff and admin may bypass the ownership check.
		callerRole := claims.Role()
		isStaff := callerRole == roles.RoleAirlineManager.String() ||
			callerRole == roles.RoleAdmin.String() ||
			auth.IsGodMode(r)

		if !isStaff {
			discordID := claims.DiscordUserID()
			caller, err := h.usersSvc.GetByDiscordID(r.Context(), discordID)
			if err != nil {
				logging.Error("GetUserLogbookSelf: failed to look up caller", "discord_id", discordID, "error", err)
				httpdto.WriteError(w, initTime, "INTERNAL_ERROR", "Failed to verify ownership", http.StatusInternalServerError)
				return
			}
			if caller == nil || caller.IFCommunityID != ifcID {
				httpdto.WriteError(w, initTime, "FORBIDDEN", "You may only view your own logbook", http.StatusForbidden)
				return
			}
		}

		dto, err := h.logbookSvc.GetUserLogbook(ifcID, page)
		if err != nil {
			logging.Warn("GetUserLogbookSelf: failed to fetch logbook", "ifc_id", ifcID, "page", page, "error", err)
			httpdto.WriteError(w, initTime, "FETCH_ERROR", dto.Error, http.StatusInternalServerError)
			return
		}

		httpdto.WriteSuccess(w, initTime, dto, http.StatusOK)
	}
}

// LogbookPageHandler handles GET /dashboard/logbook
// Serves the logbook page for staff/admin users only (restricted by middleware)
func (h *Handler) LogbookPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user claims from context (injected by auth middleware)
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}

		// Get session data from context
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}

		// Cast to SessionData
		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		// Prepare template data
		data, err := templates.PrepareTemplateData(sessionData, "Logbook")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add current page identifier for menu highlighting
		data["CurrentPage"] = "logbook"

		// Create renderer configured for templates
		renderer := templates.NewRenderer(
			"templates",                   // BasePath (feature templates)
			"templates/partials",          // PartialsPath (shared partials)
			"templates/layouts/base.html", // LayoutPath (shared layout)
		)

		// Render template
		if err := renderer.RenderTemplate(w, "pages/logbook.html", data); err != nil {
			logging.Error("Error rendering logbook page", "error", err)
			http.Error(w, "Error rendering logbook page", http.StatusInternalServerError)
			return
		}
	}
}

// LogbookFlightsHandler handles GET /dashboard/logbook/flights?user_id={ifc_id}&page=1
// Returns HTMX partial with flight list for a user
func (h *Handler) LogbookFlightsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get query parameters
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "user_id parameter required", http.StatusBadRequest)
			return
		}

		pageStr := r.URL.Query().Get("page")
		if pageStr == "" {
			pageStr = "1"
		}
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}

		// Fetch flights from service
		flightHistory, err := h.logbookSvc.GetUserLogbook(userID, page)
		if err != nil {
			logging.Warn("Failed to fetch flights", "user_id", userID, "page", page, "error", err)
			http.Error(w, "Failed to fetch flights: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if flightHistory == nil || flightHistory.Records == nil {
			flightHistory = &dtos.FlightHistoryDto{
				PageNo:  page,
				Records: []dtos.HistoryRecord{},
			}
		}

		// Prepare template data with pagination
		data := map[string]interface{}{
			"Flights":     flightHistory.Records,
			"PageNo":      page,
			"HasNext":     flightHistory.HasNext,
			"HasPrevious": flightHistory.HasPrevious,
			"TotalPages":  flightHistory.TotalPages,
			"TotalCount":  flightHistory.TotalCount,
			"NextPage":    page + 1,
			"PrevPage":    page - 1,
			"UserID":      userID,
		}

		// Create renderer for partials
		renderer := templates.NewRenderer(
			"templates/partials",
			"templates/partials",
			"", // No layout for partials
		)

		// Render partial (no base layout)
		if err := renderer.RenderPartial(w, "flight-list.html", data); err != nil {
			logging.Error("Error rendering flight list", "error", err)
			http.Error(w, "Error rendering flight list", http.StatusInternalServerError)
			return
		}
	}
}
