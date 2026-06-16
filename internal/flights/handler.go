package flights

import (
	"encoding/json"
	"html/template"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/platform/httpdto"
	platformVA "infinite-experiment/politburo/internal/platform/va"
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

type FlightPathPoint struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Altitude  float64   `json:"altitude,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Name      string    `json:"name,omitempty"`
}

type CachedFlightPathsResponse struct {
	FlightID          string            `json:"flight_id"`
	FlightPlanPresent bool              `json:"flight_plan_present"`
	FlightPlan        []FlightPathPoint `json:"flight_plan"`
	FlownRoute        []FlightPathPoint `json:"flown_route"`
	MaxSpeed          *int              `json:"max_speed"`
	MaxAltitude       *int              `json:"max_altitude"`
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
		logging.Debug("GetFlightFromCache called", "flight_id", flightID)

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
		logging.Debug("GetUserFlightsFromCache called", "user_id", userID)

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

type VALiveFlightsContractHandler struct {
	redisCache *cache.RedisCacheService
	authSvc    *auth.Service
}

func NewVALiveFlightsContractHandler(redisCache *cache.RedisCacheService, authSvc *auth.Service) *VALiveFlightsContractHandler {
	return &VALiveFlightsContractHandler{redisCache: redisCache, authSvc: authSvc}
}

// GetVALiveFlightsFromCache handles GET /api/v1/flights/va.
// Returns live flights for the current VA from prepopulated cache (new cache structure).
// Reads flight IDs from game:live:vaflights:<va_id> and fetches each CompleteFlight object.
// This is more efficient than the old approach as it reads directly from cache populated by FlightsCacheJob.
// Also includes a signed link for browser access to the live flights page.
func (h *VALiveFlightsContractHandler) GetVALiveFlightsFromCache() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Unauthorized", http.StatusUnauthorized)
			return
		}
		if claims.UserID() == "" {
			httpdto.WriteError(w, initTime, "USER_NOT_REGISTERED", "You must register before viewing live flights.", http.StatusForbidden)
			return
		}
		if claims.Role() == "" {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "You must be an active VA member to view live flights.", http.StatusForbidden)
			return
		}

		resolvedVA := ResolvedVAContext{
			VAID:   claims.ServerID(),
			UserID: claims.UserID(),
		}
		response, err := BuildVALiveFlightsResponse(
			r.Context(),
			h.redisCache,
			resolvedVA,
			h.authSvc,
			auth.GetUIBaseURL(r),
		)
		if err == ErrVAContextNotConfigured {
			httpdto.WriteError(w, initTime, VAContextNotConfiguredCode, "VA context is not configured for this request.", http.StatusForbidden)
			return
		}
		if err != nil {
			logging.Warn("failed to fetch live flights from cache", "error", err)
			httpdto.WriteError(w, initTime, LiveFlightsUnavailableCode, "Live flights are temporarily unavailable.", http.StatusInternalServerError)
			return
		}

		logging.Info("live flights response built",
			"flight_count", len(response.Flights),
			"code", response.Code,
			"signed_link_available", response.SignedLink != "",
			"summary_top_route_present", response.Summary.TopRoute != nil,
		)
		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
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

		// Calculate max speed and max altitude from waypoints
		var maxSpeed, maxAltitude *int
		if len(flight.Waypoints) > 0 {
			maxSpeedVal := flight.Waypoints[0].Speed
			maxAltitudeVal := flight.Waypoints[0].Altitude

			for _, wp := range flight.Waypoints {
				if wp.Speed > maxSpeedVal {
					maxSpeedVal = wp.Speed
				}
				if wp.Altitude > maxAltitudeVal {
					maxAltitudeVal = wp.Altitude
				}
			}

			maxSpeed = &maxSpeedVal
			maxAltitude = &maxAltitudeVal
		}

		// Return waypoints with calculated max values
		response := map[string]interface{}{
			"waypoints":    flight.Waypoints,
			"max_speed":    maxSpeed,
			"max_altitude": maxAltitude,
		}

		common.RespondSuccess(w, initTime, "Waypoints fetched", response)
	}
}

// GetCachedFlightPaths handles GET /dashboard/flights/{flight_id}/paths.
// It returns only cached data: planned flight-plan coordinates if present and flown waypoint history.
func GetCachedFlightPaths(redisCache *cache.RedisCacheService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()
		flightID := chi.URLParam(r, "flight_id")
		if flightID == "" {
			common.RespondError(w, initTime, nil, "Missing flight_id parameter", http.StatusBadRequest)
			return
		}

		flight, found := getCachedCompleteFlightForHandler(redisCache, flightID)
		if !found {
			common.RespondError(w, initTime, nil, "Flight not found", http.StatusNotFound)
			return
		}

		flightPlan := []FlightPathPoint{}
		flightPlanPresent := false
		if planVal, found := redisCache.Get(cache.FlightPlanKey(flightID)); found {
			if points, err := cachedFlightPlanPoints(planVal); err != nil {
				logging.Warn("Failed to parse cached flight plan", "flightID", flightID, "error", err)
			} else {
				flightPlan = points
				flightPlanPresent = len(points) > 0
			}
		}

		flownRoute := make([]FlightPathPoint, 0, len(flight.Waypoints))
		var maxSpeed, maxAltitude *int
		if len(flight.Waypoints) > 0 {
			maxSpeedVal := flight.Waypoints[0].Speed
			maxAltitudeVal := flight.Waypoints[0].Altitude
			for _, wp := range flight.Waypoints {
				flownRoute = append(flownRoute, FlightPathPoint{Latitude: wp.Latitude, Longitude: wp.Longitude, Altitude: float64(wp.Altitude), Timestamp: wp.Timestamp.UTC()})
				if wp.Speed > maxSpeedVal {
					maxSpeedVal = wp.Speed
				}
				if wp.Altitude > maxAltitudeVal {
					maxAltitudeVal = wp.Altitude
				}
			}
			maxSpeed = &maxSpeedVal
			maxAltitude = &maxAltitudeVal
		}

		common.RespondSuccess(w, initTime, "Flight paths fetched", CachedFlightPathsResponse{FlightID: flightID, FlightPlanPresent: flightPlanPresent, FlightPlan: flightPlan, FlownRoute: flownRoute, MaxSpeed: maxSpeed, MaxAltitude: maxAltitude})
	}
}

func getCachedCompleteFlightForHandler(redisCache *cache.RedisCacheService, flightID string) (*CompleteFlight, bool) {
	flightVal, found := redisCache.Get(cache.LiveFlightKey(flightID))
	if !found {
		return nil, false
	}
	jsonBytes, err := json.Marshal(flightVal)
	if err != nil {
		return nil, false
	}
	var flight CompleteFlight
	if err := json.Unmarshal(jsonBytes, &flight); err != nil {
		return nil, false
	}
	return &flight, true
}

func cachedFlightPlanPoints(planVal interface{}) ([]FlightPathPoint, error) {
	jsonBytes, err := json.Marshal(planVal)
	if err != nil {
		return nil, err
	}
	var plan liveapi.FlightPlanResponse
	if err := json.Unmarshal(jsonBytes, &plan); err != nil {
		return nil, err
	}
	points := make([]FlightPathPoint, 0, len(plan.FlightPlanItems))
	appendFlightPlanItemPoints(&points, plan.FlightPlanItems)
	return points, nil
}

func appendFlightPlanItemPoints(points *[]FlightPathPoint, items []liveapi.FlightPlanItem) {
	for _, item := range items {
		if item.Location.Latitude != 0 || item.Location.Longitude != 0 {
			*points = append(*points, FlightPathPoint{Latitude: item.Location.Latitude, Longitude: item.Location.Longitude, Altitude: item.Location.Altitude, Name: item.Name})
		}
		if len(item.Children) > 0 {
			appendFlightPlanItemPoints(points, item.Children)
		}
	}
}

// LivePageHandler renders the server-side Live flights dashboard page.
type LivePageHandler struct {
	redisCache       *cache.RedisCacheService
	templateRenderer *templates.Renderer
}

// NewLivePageHandler creates a Live page handler with injected infrastructure.
func NewLivePageHandler(redisCache *cache.RedisCacheService, templateRenderer *templates.Renderer) *LivePageHandler {
	return &LivePageHandler{redisCache: redisCache, templateRenderer: templateRenderer}
}

// LiveFlightsPageHandler handles GET /dashboard/live.
// Serves the live flights page with a list-first, map-enhanced layout.
func (h *LivePageHandler) LiveFlightsPageHandler() http.HandlerFunc {
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
		// Use the active VA UUID so the lookup matches game:live:vaflights:<va_id>.
		vaID := activeVA.VAID
		if vaID == "" {
			vaID = claims.ServerID()
		}
		if vaID == "" {
			http.Error(w, "VA ID not found in claims", http.StatusInternalServerError)
			return
		}

		// Fetch flights using common service function
		flights, err := GetVALiveFlightsDTOs(h.redisCache, vaID)
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
		data["Flights"] = flights
		data["FlightCount"] = len(flights)
		data["LiveStatus"] = "Live data updates automatically"

		// Add flights data as JSON for the template (using template.JS for safe embedding)
		flightsJSON, err := json.Marshal(flights)
		if err != nil {
			logging.Warn("Failed to marshal flights for template", "error", err)
			data["FlightsJSON"] = template.JS("[]")
		} else {
			data["FlightsJSON"] = template.JS(flightsJSON)
		}

		// Render template
		if err := h.templateRenderer.RenderTemplate(w, "pages/live.html", data); err != nil {
			if templates.IsClientDisconnect(err) {
				logging.Debug("Client disconnected while rendering live flights page")
				return
			}
			logging.Error("Error rendering live flights page", "error", err)
			http.Error(w, "Error rendering live flights page", http.StatusInternalServerError)
			return
		}
	}
}
