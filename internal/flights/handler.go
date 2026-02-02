package flights

import (
	"encoding/json"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handler consolidates all flight-related API handlers
type Handler struct {
	svc         *Service
	vaConfigSvc *common.VAConfigService
	legacyCache *cache.CacheService
}

// NewHandler creates a new Handler instance
func NewHandler(svc *Service, vaConfigSvc *common.VAConfigService, legacyCache *cache.CacheService) *Handler {
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

		serverID, ok := h.vaConfigSvc.GetConfigVal(r.Context(), claims.ServerID(), common.ConfigKeyIFServerID)
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
func GetVALiveFlightsFromCache(redisCache *cache.RedisCacheService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		// Get VA ID from claims (auth already processes Discord ID to UUID)
		vaID := claims.ServerID()
		if vaID == "" {
			common.RespondError(w, initTime, nil, "VA ID not found in claims", http.StatusBadRequest)
			return
		}

		// Get flight IDs list from cache
		vaFlightsKey := cache.LiveVAFlightsKey(vaID)
		flightIDsVal, found := redisCache.Get(vaFlightsKey)
		if !found {
			// No flights cached for this VA - return empty array
			common.RespondSuccess(w, initTime, "No live flights found", []VALiveFlightDTO{})
			return
		}

		// Parse flight IDs string (pipe-separated)
		flightIDsStr, ok := flightIDsVal.(string)
		if !ok {
			logging.Warn("Invalid flight IDs format in cache", "vaID", vaID, "type", "%T", flightIDsVal)
			common.RespondError(w, initTime, nil, "Invalid cache format", http.StatusInternalServerError)
			return
		}

		if flightIDsStr == "" {
			common.RespondSuccess(w, initTime, "No live flights found", []VALiveFlightDTO{})
			return
		}

		flightIDs := strings.Split(flightIDsStr, "|")
		flights := make([]VALiveFlightDTO, 0, len(flightIDs))

		// Fetch each flight object from cache
		for _, flightID := range flightIDs {
			if flightID == "" {
				continue
			}

			flightKey := cache.LiveFlightKey(flightID)
			flightVal, found := redisCache.Get(flightKey)
			if !found {
				logging.Debug("Flight not found in cache", "flightID", flightID)
				continue
			}

			// Convert cached value to CompleteFlight
			jsonBytes, err := json.Marshal(flightVal)
			if err != nil {
				logging.Warn("Failed to marshal cached flight", "flightID", flightID, "error", err)
				continue
			}

			var flight CompleteFlight
			if err := json.Unmarshal(jsonBytes, &flight); err != nil {
				logging.Warn("Failed to unmarshal cached flight", "flightID", flightID, "error", err)
				continue
			}

			// Convert to DTO (excludes internal fields and ensures UTC timestamps)
			dto := ToVALiveFlightDTO(&flight)
			flights = append(flights, *dto)
		}

		common.RespondSuccess(w, initTime, "Live flights fetched", flights)
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
