package ui

import (
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/services"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// This package contains authentication and UI handlers for Vizburo.
// See auth.go for authentication handlers.

// DashboardHandler serves the main dashboard page for authenticated users
func DashboardHandler(w http.ResponseWriter, r *http.Request) {
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
	sessionData, ok := sessionDataInterface.(*common.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	// Prepare template data using common helper
	data, err := PrepareTemplateData(sessionData, "Dashboard")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render template
	if err := RenderTemplate(w, "pages/dashboard.html", data); err != nil {
		http.Error(w, "Error rendering dashboard", http.StatusInternalServerError)
		return
	}
}

// LogbookHandler serves the logbook page for staff and admin users
// Role check: Staff middleware ensures only staff and admin can access this
func LogbookHandler(w http.ResponseWriter, r *http.Request) {
	// Get session data from context (guaranteed by auth middleware)
	sessionDataInterface := auth.GetSessionData(r.Context())
	sessionData, ok := sessionDataInterface.(*common.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	// Prepare template data using common helper
	data, err := PrepareTemplateData(sessionData, "Logbook")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render template
	if err := RenderTemplate(w, "pages/logbook.html", data); err != nil {
		http.Error(w, "Error rendering logbook", http.StatusInternalServerError)
		return
	}
}

// LogbookFlightsHandler returns a paginated list of flights for a given user (HTMX partial)
func LogbookFlightsHandler(
	w http.ResponseWriter,
	r *http.Request,
	flightSvc *services.FlightsService,
) {
	// Get user claims from context
	claims := auth.GetUserClaims(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get session data from context
	sessionDataInterface := auth.GetSessionData(r.Context())
	if sessionDataInterface == nil {
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

	// For now, assume we can use a placeholder session ID
	// In production, this should come from the request context
	sessionID := ""

	// Fetch flights from service
	flightHistory, err := flightSvc.GetUserFlights(userID, page, sessionID)
	if err != nil {
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
	// Use pagination metadata from the Live API response
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

	// Render partial (no base layout)
	if err := RenderPartial(w, "partials/flight-list.html", data); err != nil {
		http.Error(w, "Error rendering flight list", http.StatusInternalServerError)
		return
	}
}

// FlightMapHandler returns flight route map data with cached route (HTMX partial)
func FlightMapHandler(
	w http.ResponseWriter,
	r *http.Request,
	cache common.CacheInterface,
	liveAPI *common.LiveAPIService,
	flightSvc *services.FlightsService,
) {
	// Get user claims
	claims := auth.GetUserClaims(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get session data
	sessionDataInterface := auth.GetSessionData(r.Context())
	if sessionDataInterface == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get flight_id and session_id from URL params
	flightID := chi.URLParam(r, "flight_id")
	sessionID := chi.URLParam(r, "session_id")
	if flightID == "" {
		http.Error(w, "flight_id parameter required", http.StatusBadRequest)
		return
	}
	if sessionID == "" {
		http.Error(w, "session_id parameter required", http.StatusBadRequest)
		return
	}

	// Get page from query params for metadata lookup (optional, defaults to 1)
	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Get IFC ID (username) from query params to look up actual IF user ID
	ifcID := r.URL.Query().Get("user_id")
	if ifcID == "" {
		log.Printf("[FlightMapHandler] Warning: user_id (IFC ID) not provided, cannot fetch metadata")
	}

	// Debug logging
	log.Printf("[FlightMapHandler] Processing flight: session_id=%s, flight_id=%s, ifc_id=%s, page=%d", sessionID, flightID, ifcID, page)

	// Look up actual IF user ID from IFC ID
	var actualUserID string
	if ifcID != "" {
		userStats, _, err := liveAPI.GetUserByIfcId(ifcID)
		if err != nil {
			log.Printf("[FlightMapHandler] Failed to lookup IF user ID for IFC ID %s: %v", ifcID, err)
		} else if len(userStats.Result) > 0 {
			actualUserID = userStats.Result[0].UserID
			log.Printf("[FlightMapHandler] Resolved IFC ID %s to IF user ID %s", ifcID, actualUserID)
		} else {
			log.Printf("[FlightMapHandler] No results when looking up IFC ID %s", ifcID)
		}
	}

	// Try to get from cache first - use combo key with session_id and flight_id
	cacheKey := string(constants.CachePrefixFlightHistory) + sessionID + "_" + flightID
	cachedVal, found := cache.Get(cacheKey)
	var flightInfo *dtos.FlightInfo

	if found && cachedVal != nil {
		// Redis returns the value as a generic interface{} (map[string]interface{} from JSON unmarshal)
		// We need to marshal it back to JSON then unmarshal to FlightInfo struct
		jsonData, err := json.Marshal(cachedVal)
		if err == nil {
			var flight dtos.FlightInfo
			if err := json.Unmarshal(jsonData, &flight); err == nil {
				flightInfo = &flight
				log.Printf("[FlightMapHandler] Route data cache hit: %s_%s, route points=%d", sessionID, flightID, len(flight.Route))
			} else {
				log.Printf("[FlightMapHandler] Route data unmarshal failed: %v", err)
			}
		} else {
			log.Printf("[FlightMapHandler] Route data marshal failed: %v", err)
		}
	} else {
		log.Printf("[FlightMapHandler] Route data cache miss: %s_%s", sessionID, flightID)
	}

	// Fetch metadata using FlightsService (handles caching internally)
	// This ensures we use the same cache instance and get properly typed data
	var metaRecord *dtos.HistoryRecord
	if actualUserID != "" && page > 0 && ifcID != "" {
		historyDto, err := flightSvc.GetUserFlights(ifcID, page, sessionID)
		if err != nil {
			log.Printf("[FlightMapHandler] Failed to fetch user flights: %v", err)
		} else if historyDto != nil && len(historyDto.Records) > 0 {
			// Find the matching flight in the history
			for _, record := range historyDto.Records {
				if record.FlightID == flightID {
					metaRecord = &record
					log.Printf("[FlightMapHandler] Metadata found: %s, aircraft=%s, callsign=%s", flightID, record.Aircraft, record.Callsign)
					break
				}
			}
			if metaRecord == nil {
				log.Printf("[FlightMapHandler] Flight %s not found in history page %d (found %d records)", flightID, page, len(historyDto.Records))
			}
		} else {
			log.Printf("[FlightMapHandler] No flight history returned for user %s, page %d", actualUserID, page)
		}
	} else {
		log.Printf("[FlightMapHandler] Skipping metadata fetch: user_id=%s, page=%d, ifc_id=%s", actualUserID, page, ifcID)
	}

	apiKey := os.Getenv("IF_API_KEY")

	// Determine which partial to render based on data availability
	var partialPath string
	var data map[string]interface{}

	// State 1: Flight with route available
	if flightInfo != nil {
		log.Printf("[FlightMapHandler] Rendering with-route state (route available)")
		partialPath = "partials/flight-map/with-route.html"

		route := flightInfo.Route
		if len(route) > 1000 {
			route = downsampleRouteRDP(route)
		}

		data = map[string]interface{}{
			"FlightID":  flightID,
			"SessionID": sessionID,
			"APIKey":    apiKey,
			"Meta":      metaRecord, // Metadata from flight history cache
			"Path":      route,
			"Origin":    flightInfo.Origin,
			"Dest":      flightInfo.Dest,
			"RouteMeta": flightInfo.Meta, // Max speed/alt from route
		}
	} else if metaRecord != nil {
		// State 2: Metadata available but no route
		log.Printf("[FlightMapHandler] Rendering metadata-only state (route unavailable)")
		partialPath = "partials/flight-map/metadata-only.html"

		data = map[string]interface{}{
			"FlightID":  flightID,
			"SessionID": sessionID,
			"APIKey":    apiKey,
			"Meta":      metaRecord,
		}
	} else {
		// State 3: No flight data available
		log.Printf("[FlightMapHandler] Rendering empty state (no flight data)")
		partialPath = "partials/flight-map/empty.html"

		data = map[string]interface{}{
			"FlightID":  flightID,
			"SessionID": sessionID,
			"APIKey":    apiKey,
		}
	}

	// Render appropriate partial
	if err := RenderPartial(w, partialPath, data); err != nil {
		log.Printf("[FlightMapHandler] Error rendering partial %s: %v", partialPath, err)
		http.Error(w, "Error rendering flight map", http.StatusInternalServerError)
		return
	}
}

// PilotSearchHandler returns pilot search results for autocomplete (HTMX partial)
func PilotSearchHandler(
	w http.ResponseWriter,
	r *http.Request,
	vaRoleRepo *repositories.VAUserRoleRepository,
) {
	// Get user claims
	claims := auth.GetUserClaims(r.Context())
	if claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get session data
	sessionDataInterface := auth.GetSessionData(r.Context())
	if sessionDataInterface == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionData, ok := sessionDataInterface.(*common.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	// Get active VA
	activeVA := sessionData.GetActiveVA()
	if activeVA == nil {
		http.Error(w, "No active VA found", http.StatusInternalServerError)
		return
	}

	// Role check
	if activeVA.Role != "staff" && activeVA.Role != "admin" {
		http.Error(w, "Permission denied", http.StatusForbidden)
		return
	}

	// Get search query
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		// Return empty results partial
		if err := RenderPartial(w, "partials/search-results.html", map[string]interface{}{
			"Results": []interface{}{},
		}); err != nil {
			http.Error(w, "Error rendering search results", http.StatusInternalServerError)
		}
		return
	}

	// Get all VA members
	vaRoles, err := vaRoleRepo.GetAllByVAID(r.Context(), activeVA.VAID)
	if err != nil {
		http.Error(w, "Failed to search pilots", http.StatusInternalServerError)
		return
	}

	// Filter by IFCommunityID match (case-insensitive)
	queryLower := strings.ToLower(query)
	var results []map[string]interface{}
	for _, vaRole := range vaRoles {
		ifcID := strings.ToLower(vaRole.User.IFCommunityID)
		if strings.Contains(ifcID, queryLower) {
			results = append(results, map[string]interface{}{
				"Username": vaRole.User.IFCommunityID,
				"Role":     vaRole.Role,
				"VAName":   activeVA.VAName,
			})
			if len(results) >= 10 {
				break
			}
		}
	}

	// Prepare template data
	data := map[string]interface{}{
		"Results": results,
	}

	// Render partial
	if err := RenderPartial(w, "partials/search-results.html", data); err != nil {
		http.Error(w, "Error rendering search results", http.StatusInternalServerError)
		return
	}
}

// perpendicularDistance calculates the perpendicular distance from a point to a line segment
// Used by Ramer-Douglas-Peucker algorithm for polyline simplification
func perpendicularDistance(point, lineStart, lineEnd dtos.RouteWaypoint) float64 {
	// Parse coordinates from string format
	lat1 := parseCoord(lineStart.Lat)
	lng1 := parseCoord(lineStart.Long)
	lat2 := parseCoord(lineEnd.Lat)
	lng2 := parseCoord(lineEnd.Long)
	lat0 := parseCoord(point.Lat)
	lng0 := parseCoord(point.Long)

	// Calculate perpendicular distance using point-to-line formula
	numerator := math.Abs((lat2-lat1)*(lng1-lng0) - (lng2-lng1)*(lat1-lat0))
	denominator := math.Sqrt((lat2-lat1)*(lat2-lat1) + (lng2-lng1)*(lng2-lng1))

	if denominator == 0 {
		// Points are identical, return distance between point and start
		return math.Sqrt((lat0-lat1)*(lat0-lat1) + (lng0-lng1)*(lng0-lng1))
	}

	return numerator / denominator
}

// parseCoord converts a coordinate string to float64
func parseCoord(coordStr string) float64 {
	var val float64
	_, _ = fmt.Sscanf(coordStr, "%f", &val)
	return val
}

// rdpSimplify recursively simplifies a polyline using the Ramer-Douglas-Peucker algorithm
// epsilon: maximum distance threshold for point removal
func rdpSimplify(points []dtos.RouteWaypoint, epsilon float64) []dtos.RouteWaypoint {
	if len(points) < 3 {
		return points
	}

	// Find the point with maximum distance from line segment
	maxDist := 0.0
	maxIdx := 0
	for i := 1; i < len(points)-1; i++ {
		dist := perpendicularDistance(points[i], points[0], points[len(points)-1])
		if dist > maxDist {
			maxDist = dist
			maxIdx = i
		}
	}

	// If max distance is less than epsilon, remove all intermediate points
	if maxDist < epsilon {
		return []dtos.RouteWaypoint{points[0], points[len(points)-1]}
	}

	// Otherwise, recursively simplify the two segments
	leftSegment := rdpSimplify(points[:maxIdx+1], epsilon)
	rightSegment := rdpSimplify(points[maxIdx:], epsilon)

	// Merge segments (avoid duplicating the point at maxIdx)
	result := make([]dtos.RouteWaypoint, 0, len(leftSegment)+len(rightSegment)-1)
	result = append(result, leftSegment...)
	result = append(result, rightSegment[1:]...)

	return result
}

// downsampleRouteRDP reduces the number of waypoints using the Ramer-Douglas-Peucker algorithm
// Targets 50-60% reduction for better visual accuracy while maintaining route geometry
// Automatically adjusts epsilon based on route size to achieve target reduction
func downsampleRouteRDP(route []dtos.RouteWaypoint) []dtos.RouteWaypoint {
	if len(route) <= 1000 {
		return route
	}

	// Auto-tune epsilon to target 55% reduction
	// Base epsilon: 0.0005 degrees (~55m at equator) for ~1500 point route
	// Scale with route size for consistent reduction percentage
	baseEpsilon := 0.0005
	sizeRatio := float64(len(route)) / 1500.0
	epsilon := baseEpsilon * sizeRatio

	simplified := rdpSimplify(route, epsilon)

	// If we removed too many points (>70% reduction), increase epsilon and retry
	if float64(len(simplified))/float64(len(route)) < 0.30 {
		epsilon *= 0.7
		simplified = rdpSimplify(route, epsilon)
	}

	// If we removed too few points (<30% reduction), decrease epsilon
	if float64(len(simplified))/float64(len(route)) > 0.75 {
		epsilon *= 1.5
		simplified = rdpSimplify(route, epsilon)
	}

	log.Printf("[downsampleRouteRDP] Reduced %d points to %d points (%.1f%% reduction)",
		len(route), len(simplified), 100.0*(1.0-float64(len(simplified))/float64(len(route))))

	return simplified
}

// MapResetHandler returns empty map state (HTMX partial)
func MapResetHandler(w http.ResponseWriter, r *http.Request) {
	// Get session data
	sessionDataInterface := auth.GetSessionData(r.Context())
	if sessionDataInterface == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	data := map[string]interface{}{
		"Path": []interface{}{},
	}

	// Render empty map
	if err := RenderPartial(w, "partials/flight-map-empty.html", data); err != nil {
		http.Error(w, "Error rendering map", http.StatusInternalServerError)
		return
	}
}

// PilotsHandler returns the main pilots page
// Role check: Staff middleware ensures only staff and admin can access this
func PilotsHandler(w http.ResponseWriter, r *http.Request) {
	// Get session data from context (guaranteed by auth middleware)
	sessionDataInterface := auth.GetSessionData(r.Context())
	sessionData, ok := sessionDataInterface.(*common.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	// Prepare template data using common helper
	data, err := PrepareTemplateData(sessionData, "Pilots")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := RenderTemplate(w, "pages/pilots.html", data); err != nil {
		http.Error(w, "Error rendering pilots page", http.StatusInternalServerError)
		return
	}
}

// PilotsListHandler returns a list of pilots for the active VA (HTMX partial)
func PilotsListHandler(
	w http.ResponseWriter,
	r *http.Request,
	pilotMgmtSvc *services.PilotManagementService,
) {
	// Get session data
	sessionDataInterface := auth.GetSessionData(r.Context())
	if sessionDataInterface == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionData, ok := sessionDataInterface.(*common.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	activeVA := sessionData.GetActiveVA()
	if activeVA == nil {
		http.Error(w, "No active VA found", http.StatusInternalServerError)
		return
	}

	// Get pilots from service
	pilots, err := pilotMgmtSvc.GetPilotsByVAID(
		r.Context(),
		activeVA.VAID,
		constants.VARole(activeVA.Role),
	)
	if err != nil {
		http.Error(w, "Failed to fetch pilots: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if pilots == nil {
		pilots = []services.PilotDTO{}
	}

	// Prepare template data
	data := map[string]interface{}{
		"Pilots":    pilots,
		"ActiveVA":  activeVA,
		"IsAdmin":   activeVA.Role == "admin",
	}

	// Render partial
	if err := RenderPartial(w, "partials/pilots-table.html", data); err != nil {
		http.Error(w, "Error rendering pilots table", http.StatusInternalServerError)
		return
	}
}

// UpdatePilotRoleHandler updates a pilot's role (HTMX endpoint)
// Role check: Admin middleware ensures only admins can access this
func UpdatePilotRoleHandler(
	w http.ResponseWriter,
	r *http.Request,
	pilotMgmtSvc *services.PilotManagementService,
) {
	// Get session data (guaranteed by auth middleware)
	sessionDataInterface := auth.GetSessionData(r.Context())
	sessionData, ok := sessionDataInterface.(*common.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	activeVA := sessionData.GetActiveVA()
	if activeVA == nil {
		http.Error(w, "No active VA found", http.StatusInternalServerError)
		return
	}

	// Get pilot ID from URL parameter
	pilotID := chi.URLParam(r, "pilot_id")
	if pilotID == "" {
		http.Error(w, "Missing pilot_id in URL", http.StatusBadRequest)
		return
	}

	// Get new role from form data
	newRole := r.FormValue("role")
	if newRole == "" {
		http.Error(w, "Missing role field", http.StatusBadRequest)
		return
	}

	// Update role via service (service validates admin role for defense-in-depth)
	err := pilotMgmtSvc.UpdatePilotRole(
		r.Context(),
		activeVA.VAID,
		pilotID,
		newRole,
		constants.VARole(activeVA.Role),
	)
	if err != nil {
		http.Error(w, "Failed to update pilot role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-fetch pilots and render updated table
	pilots, err := pilotMgmtSvc.GetPilotsByVAID(
		r.Context(),
		activeVA.VAID,
		constants.VARole(activeVA.Role),
	)
	if err != nil {
		http.Error(w, "Failed to fetch updated pilots", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Pilots":   pilots,
		"ActiveVA": activeVA,
		"IsAdmin":  activeVA.Role == "admin",
	}

	if err := RenderPartial(w, "partials/pilots-table.html", data); err != nil {
		http.Error(w, "Error rendering pilots table", http.StatusInternalServerError)
		return
	}
}

// UpdatePilotCallsignHandler updates a pilot's callsign (HTMX endpoint)
// Role check: Staff middleware ensures staff or admin can access this
func UpdatePilotCallsignHandler(
	w http.ResponseWriter,
	r *http.Request,
	pilotMgmtSvc *services.PilotManagementService,
) {
	// Get session data (guaranteed by auth middleware)
	sessionDataInterface := auth.GetSessionData(r.Context())
	sessionData, ok := sessionDataInterface.(*common.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	activeVA := sessionData.GetActiveVA()
	if activeVA == nil {
		http.Error(w, "No active VA found", http.StatusInternalServerError)
		return
	}

	// Get pilot ID from URL parameter
	pilotID := chi.URLParam(r, "pilot_id")
	if pilotID == "" {
		http.Error(w, "Missing pilot_id in URL", http.StatusBadRequest)
		return
	}

	// Get new callsign from form data
	newCallsign := r.FormValue("callsign")

	// Update callsign via service
	err := pilotMgmtSvc.UpdatePilotCallsign(
		r.Context(),
		activeVA.VAID,
		pilotID,
		newCallsign,
		constants.VARole(activeVA.Role),
	)
	if err != nil {
		http.Error(w, "Failed to update callsign: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Re-fetch pilots and render updated table
	pilots, err := pilotMgmtSvc.GetPilotsByVAID(
		r.Context(),
		activeVA.VAID,
		constants.VARole(activeVA.Role),
	)
	if err != nil {
		http.Error(w, "Failed to fetch updated pilots", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Pilots":   pilots,
		"ActiveVA": activeVA,
		"IsAdmin":  activeVA.Role == "admin",
	}

	if err := RenderPartial(w, "partials/pilots-table.html", data); err != nil {
		http.Error(w, "Error rendering pilots table", http.StatusInternalServerError)
		return
	}
}

// RemovePilotHandler removes a pilot from the VA (HTMX endpoint)
// Role check: Admin middleware ensures only admins can access this
func RemovePilotHandler(
	w http.ResponseWriter,
	r *http.Request,
	pilotMgmtSvc *services.PilotManagementService,
) {
	// Get session data (guaranteed by auth middleware)
	sessionDataInterface := auth.GetSessionData(r.Context())
	sessionData, ok := sessionDataInterface.(*common.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	activeVA := sessionData.GetActiveVA()
	if activeVA == nil {
		http.Error(w, "No active VA found", http.StatusInternalServerError)
		return
	}

	// Get pilot ID from URL parameter
	pilotID := chi.URLParam(r, "pilot_id")
	if pilotID == "" {
		http.Error(w, "Missing pilot_id in URL", http.StatusBadRequest)
		return
	}

	// Remove pilot via service (service validates admin role for defense-in-depth)
	err := pilotMgmtSvc.RemovePilot(
		r.Context(),
		activeVA.VAID,
		pilotID,
		constants.VARole(activeVA.Role),
	)
	if err != nil {
		http.Error(w, "Failed to remove pilot: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Re-fetch pilots and render updated table
	pilots, err := pilotMgmtSvc.GetPilotsByVAID(
		r.Context(),
		activeVA.VAID,
		constants.VARole(activeVA.Role),
	)
	if err != nil {
		http.Error(w, "Failed to fetch updated pilots", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Pilots":   pilots,
		"ActiveVA": activeVA,
		"IsAdmin":  activeVA.Role == "admin",
	}

	if err := RenderPartial(w, "partials/pilots-table.html", data); err != nil {
		http.Error(w, "Error rendering pilots table", http.StatusInternalServerError)
		return
	}
}
