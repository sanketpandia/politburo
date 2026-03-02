package flights

import (
	"encoding/json"
	"html/template"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	platformVA "infinite-experiment/politburo/internal/platform/va"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handler consolidates all flight-related API handlers
type Handler struct {
	svc         *Service
	vaConfigSvc *platformVA.ConfigService
	legacyCache *cache.CacheService
}

// NewHandler creates a new Handler instance
func NewHandler(svc *Service, vaConfigSvc *platformVA.ConfigService, legacyCache *cache.CacheService) *Handler {
	return &Handler{
		svc:         svc,
		vaConfigSvc: vaConfigSvc,
		legacyCache: legacyCache,
	}
}

// GetVALiveFlights handles GET /api/v1/va/live
// Returns live flights for the current VA filtered by callsign pattern
// Uses cache-first approach with API fallback
func (h *Handler) GetVALiveFlights() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		// Use cache-first method (falls back to API if cache miss)
		flights, err := h.svc.GetVALiveFlightsFromCache(r.Context(), claims.ServerID())
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusBadRequest)
			return
		}

		common.RespondSuccess(w, initTime, "Live flights fetched", flights)
	}
}

// GetLiveSessions handles GET /api/v1/live/sessions
// Returns all available Infinite Flight live sessions/servers
func (h *Handler) GetLiveSessions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		servers, err := h.svc.GetLiveServers()
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Live sessions fetched", servers)
	}
}

// GetUserFlights handles GET /api/v1/user/{user_id}/flights
// Returns paginated flight history for a specific user
func (h *Handler) GetUserFlights() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Extract path parameter
		userID := chi.URLParam(r, "user_id")
		if userID == "" {
			common.RespondError(w, initTime, nil, "Invalid IFC ID Received", http.StatusBadRequest)
			return
		}

		// Parse query parameter 'page'
		page := 1
		if qs := r.URL.Query().Get("page"); qs != "" {
			p, err := strconv.Atoi(qs)
			if err != nil || p <= 0 {
				common.RespondError(w, initTime, nil, "Invalid page parameter", http.StatusBadRequest)
				return
			}
			page = p
		}

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		serverID, ok := h.vaConfigSvc.GetConfigVal(r.Context(), claims.ServerID(), platformVA.ConfigKeyIFServerID)
		if !ok {
			common.RespondError(w, initTime, nil, "IF Server not configured for VA", http.StatusInternalServerError)
			return
		}

		// Call service
		dto, err := h.svc.GetUserFlights(userID, page, serverID)
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Fetched Results", dto)
	}
}

// GetFlightFromCache handles GET /public/flight
// Returns cached flight information for a given flight ID (from query param `i`)
func (h *Handler) GetFlightFromCache() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		flightID := r.URL.Query().Get("i")
		log.Printf("API CALLED: %s", flightID)

		if flightID == "" {
			common.RespondError(w, initTime, nil, "Missing required flight ID", http.StatusBadRequest)
			return
		}

		result := common.GetFlightFromCache(h.legacyCache, flightID)
		if result == nil {
			common.RespondError(w, initTime, nil, "Flight details not found or unavailable. Please try to regenerate the link via /logbook command", http.StatusNotFound)
			return
		}

		common.RespondSuccess(w, initTime, "Data found", result)
	}
}

// GetUserFlightsFromCache handles GET /public/flight/user
// Returns cached user flights information for a given user ID (from query param `u`)
func (h *Handler) GetUserFlightsFromCache() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		userID := r.URL.Query().Get("u")
		log.Printf("API CALLED: %s", userID)

		if userID == "" {
			common.RespondError(w, initTime, nil, "Missing required flight ID", http.StatusBadRequest)
			return
		}

		result := common.GetUserFlightsFromCache(h.legacyCache, userID)
		if result == nil {
			common.RespondError(w, initTime, nil, "Flight details not found or unavailable. Please try to regenerate the link via /logbook command", http.StatusNotFound)
			return
		}

		common.RespondSuccess(w, initTime, "Data found", result)
	}
}

// GetVALiveFlightsFromCache handles GET /api/v1/flights/va
// Returns live flights for the current VA from prepopulated cache (new cache structure)
// Reads flight IDs from game:live:vaflights:<va_id> and fetches each CompleteFlight object
// This is more efficient than the old approach as it reads directly from cache populated by FlightsCacheJob
// Also includes a signed link for browser access to the live flights page
func GetVALiveFlightsFromCache(redisCache *cache.RedisCacheService, authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		// Get VA ID and User ID from claims
		vaID := claims.ServerID()
		userID := claims.UserID()
		if vaID == "" {
			common.RespondError(w, initTime, nil, "VA ID not found in claims", http.StatusBadRequest)
			return
		}

		// Fetch flights using common service function
		flights, err := GetVALiveFlightsDTOs(redisCache, vaID)
		if err != nil {
			logging.Warn("Failed to fetch flights from cache", "error", err, "vaID", vaID)
			common.RespondError(w, initTime, err, "Failed to fetch flights", http.StatusInternalServerError)
			return
		}

		// Generate signed link for browser access
		var signedLink string
		if userID != "" {
			token, err := authSvc.GenerateSignedLink(r.Context(), userID, vaID, "/dashboard/live", 15*time.Minute)
			if err != nil {
				logging.Warn("Failed to generate signed link", "error", err, "userID", userID, "vaID", vaID)
				// Continue without signed link - not a critical error
			} else {
				// Use helper functions from auth package to format the signed link URL
				uiBaseURL := auth.GetUIBaseURL(r)
				signedLink = auth.FormatSignedLinkURL(uiBaseURL, token)
			}
		}

		// Return response with flights array and signed link
		response := VALiveFlightsResponse{
			Flights:    flights,
			SignedLink: signedLink,
		}

		common.RespondSuccess(w, initTime, "Live flights fetched", response)
	}
}

// GetFlightByID handles GET /api/v1/flights/{flight_id}
// Returns a single CompleteFlight from cache by flight ID
func GetFlightByID(redisCache *cache.RedisCacheService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get flight ID from path parameter
		flightID := chi.URLParam(r, "flight_id")
		if flightID == "" {
			common.RespondError(w, initTime, nil, "Missing flight_id parameter", http.StatusBadRequest)
			return
		}

		// Get flight from cache
		flightKey := cache.LiveFlightKey(flightID)
		flightVal, found := redisCache.Get(flightKey)
		if !found {
			common.RespondError(w, initTime, nil, "Flight not found", http.StatusNotFound)
			return
		}

		// Convert cached value to CompleteFlight
		jsonBytes, err := json.Marshal(flightVal)
		if err != nil {
			logging.Warn("Failed to marshal cached flight", "flightID", flightID, "error", err)
			common.RespondError(w, initTime, err, "Failed to process flight data", http.StatusInternalServerError)
			return
		}

		var flight CompleteFlight
		if err := json.Unmarshal(jsonBytes, &flight); err != nil {
			logging.Warn("Failed to unmarshal cached flight", "flightID", flightID, "error", err)
			common.RespondError(w, initTime, err, "Failed to parse flight data", http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Flight fetched", flight)
	}
}

// GetFlightWaypoints handles GET /dashboard/flights/{flight_id}/waypoints
// Returns just the waypoints array for route mapping (UI-friendly endpoint)
func GetFlightWaypoints(redisCache *cache.RedisCacheService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get flight ID from path parameter
		flightID := chi.URLParam(r, "flight_id")
		if flightID == "" {
			common.RespondError(w, initTime, nil, "Missing flight_id parameter", http.StatusBadRequest)
			return
		}

		// Get flight from cache
		flightKey := cache.LiveFlightKey(flightID)
		flightVal, found := redisCache.Get(flightKey)
		if !found {
			common.RespondError(w, initTime, nil, "Flight not found", http.StatusNotFound)
			return
		}

		// Convert cached value to CompleteFlight
		jsonBytes, err := json.Marshal(flightVal)
		if err != nil {
			logging.Warn("Failed to marshal cached flight", "flightID", flightID, "error", err)
			common.RespondError(w, initTime, err, "Failed to process flight data", http.StatusInternalServerError)
			return
		}

		var flight CompleteFlight
		if err := json.Unmarshal(jsonBytes, &flight); err != nil {
			logging.Warn("Failed to unmarshal cached flight", "flightID", flightID, "error", err)
			common.RespondError(w, initTime, err, "Failed to parse flight data", http.StatusInternalServerError)
			return
		}

		// Return just the waypoints array
		common.RespondSuccess(w, initTime, "Waypoints fetched", flight.Waypoints)
	}
}

// LiveFlightsPageHandler handles GET /dashboard/live
// Serves the live flights page with flights rendered on Gleo map (staff+ only)
func LiveFlightsPageHandler(redisCache *cache.RedisCacheService) http.HandlerFunc {
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

		// Check if user is staff or admin
		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}
		// Get VA ID from claims
		vaID := claims.ServerID()
		if vaID == "" {
			http.Error(w, "VA ID not found in claims", http.StatusInternalServerError)
			return
		}

		// Fetch flights using common service function
		flights, err := GetVALiveFlightsDTOs(redisCache, vaID)
		if err != nil {
			logging.Warn("Failed to fetch flights from cache", "error", err, "vaID", vaID)
			// Continue with empty flights array - not a critical error for page rendering
			flights = []VALiveFlightDTO{}
		}

		// Prepare template data
		data, err := templates.PrepareTemplateData(sessionData, "Live Flights")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add current page identifier for menu highlighting
		data["CurrentPage"] = "live"

		// Add flights data as JSON for the template (using template.JS for safe embedding)
		flightsJSON, err := json.Marshal(flights)
		if err != nil {
			logging.Warn("Failed to marshal flights for template", "error", err)
			data["FlightsJSON"] = template.JS("[]")
		} else {
			data["FlightsJSON"] = template.JS(flightsJSON)
		}

		// Create renderer configured for flights templates
		renderer := templates.NewRenderer(
			"templates",                   // BasePath (feature templates)
			"templates/partials",          // PartialsPath (shared partials)
			"templates/layouts/base.html", // LayoutPath (shared layout)
		)

		// Render template
		if err := renderer.RenderTemplate(w, "pages/live.html", data); err != nil {
			logging.Error("Error rendering live flights page", "error", err)
			http.Error(w, "Error rendering live flights page", http.StatusInternalServerError)
			return
		}
	}
}
