package flights

import (
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"log"
	"net/http"
	"strconv"
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
