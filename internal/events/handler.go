package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/flights"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/httpdto"
	platformVA "infinite-experiment/politburo/internal/platform/va"
	vaRoutes "infinite-experiment/politburo/internal/va_routes"

	"github.com/go-chi/chi/v5"
)

// Handler handles events UI and API endpoints
type Handler struct {
	eventSvc         *Service
	templateRenderer *templates.Renderer
	routeRepo        *vaRoutes.Repository
	flightsSvc       *flights.Service
	vaSvc            *platformVA.Service
}

// NewHandler creates a new events handler instance
func NewHandler(
	eventSvc *Service,
	templateRenderer *templates.Renderer,
	routeRepo *vaRoutes.Repository,
	flightsSvc *flights.Service,
	vaSvc *platformVA.Service,
) *Handler {
	return &Handler{
		eventSvc:         eventSvc,
		templateRenderer: templateRenderer,
		routeRepo:        routeRepo,
		flightsSvc:       flightsSvc,
		vaSvc:            vaSvc,
	}
}

// ============================================================================
// UI Endpoints (HTMX - return HTML partials)
// ============================================================================

// EventsPageHandler handles GET /dashboard/events
// Renders the full events management page
func (h *Handler) EventsPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}

		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		// Prepare template data
		data, err := templates.PrepareTemplateData(sessionData, "Events Management")
		if err != nil {
			logging.Error("Failed to prepare template data", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data["CurrentPage"] = "events"

		// Render template
		if err := h.templateRenderer.RenderTemplate(w, "pages/events.html", data); err != nil {
			logging.Error("Error rendering events page", "error", err)
			http.Error(w, "Error rendering events page", http.StatusInternalServerError)
			return
		}
	}
}

// EventsListHandler handles GET /dashboard/events/list
// Returns the events list partial (HTMX endpoint)
func (h *Handler) EventsListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		// Get events from service
		events, err := h.eventSvc.ListEvents(r.Context(), activeVA.VAID, nil, nil)
		if err != nil {
			logging.Error("Failed to fetch events", "error", err, "va_id", activeVA.VAID)
			http.Error(w, "Failed to fetch events: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Prepare template data
		data := map[string]interface{}{
			"Events":   events,
			"ActiveVA": activeVA,
		}

		// Render partial
		if err := h.templateRenderer.RenderPartial(w, "partials/events-list.html", data); err != nil {
			logging.Error("Error rendering events list", "error", err)
			http.Error(w, "Error rendering events list", http.StatusInternalServerError)
			return
		}
	}
}

// EventFormHandler handles GET /dashboard/events/form or /dashboard/events/form/{event_id}
// Returns the event form partial (HTMX endpoint)
func (h *Handler) EventFormHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		eventID := chi.URLParam(r, "event_id")
		var eventData map[string]interface{}

		if eventID != "" {
			// Load existing event
			event, err := h.eventSvc.GetEvent(r.Context(), eventID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Event not found: %s", eventID), http.StatusNotFound)
				return
			}

			// Format dates if they exist
			var startDateStr, endDateStr string
			if event.StartDate.Valid {
				startDateStr = event.StartDate.Time.Format("2006-01-02T15:04")
			}
			if event.EndDate.Valid {
				endDateStr = event.EndDate.Time.Format("2006-01-02T15:04")
			}

			eventData = map[string]interface{}{
				"IsEdit":      true,
				"ID":          event.ID,
				"Name":        event.Name,
				"Description": event.Description,
				"Status":      event.Status,
				"StartDate":   startDateStr,
				"EndDate":     endDateStr,
				"IsActive":    event.IsActive(),
			}
		} else {
			// New event
			eventData = map[string]interface{}{
				"IsEdit": false,
			}
		}

		eventData["ActiveVA"] = activeVA

		// Render partial
		if err := h.templateRenderer.RenderPartial(w, "partials/event-form.html", eventData); err != nil {
			logging.Error("Error rendering event form", "error", err)
			http.Error(w, "Error rendering event form", http.StatusInternalServerError)
			return
		}
	}
}

// CreateEventHandler handles POST /dashboard/events/create
// Creates a new event (HTMX endpoint)
func (h *Handler) CreateEventHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID := claims.UserID()
		if userID == "" {
			http.Error(w, "User ID not found", http.StatusUnauthorized)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}

		name := r.FormValue("name")
		description := r.FormValue("description")
		status := r.FormValue("status")
		startDateStr := r.FormValue("start_date")
		endDateStr := r.FormValue("end_date")

		// Validate required fields
		if name == "" {
			http.Error(w, "Event name is required", http.StatusBadRequest)
			return
		}

		// Parse dates (both optional)
		var startDate, endDate *time.Time
		if startDateStr != "" {
			parsedStart, err := time.Parse("2006-01-02T15:04", startDateStr)
			if err != nil {
				http.Error(w, "Invalid start date format", http.StatusBadRequest)
				return
			}
			startDate = &parsedStart
		}

		if endDateStr != "" {
			parsedEnd, err := time.Parse("2006-01-02T15:04", endDateStr)
			if err != nil {
				http.Error(w, "Invalid end date format", http.StatusBadRequest)
				return
			}
			endDate = &parsedEnd
		}

		var descPtr *string
		if description != "" {
			descPtr = &description
		}

		req := CreateEventRequest{
			Name:        name,
			Description: descPtr,
			Status:      status,
			StartDate:   startDate,
			EndDate:     endDate,
		}

		event, err := h.eventSvc.CreateEvent(r.Context(), activeVA.VAID, userID, req)
		if err != nil {
			logging.Error("Failed to create event", "error", err)
			http.Error(w, "Failed to create event: "+err.Error(), http.StatusBadRequest)
			return
		}

		logging.Info("Event created", "event_id", event.ID, "va_id", activeVA.VAID)

		// Re-fetch events and render updated list
		events, err := h.eventSvc.ListEvents(r.Context(), activeVA.VAID, nil, nil)
		if err != nil {
			logging.Error("Failed to fetch events after create", "error", err)
			http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Events":   events,
			"ActiveVA": activeVA,
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/events-list.html", data); err != nil {
			logging.Error("Error rendering events list", "error", err)
			http.Error(w, "Error rendering events list", http.StatusInternalServerError)
			return
		}
	}
}

// UpdateEventHandler handles POST /dashboard/events/{event_id}/update
// Updates an existing event (HTMX endpoint)
func (h *Handler) UpdateEventHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID := claims.UserID()
		if userID == "" {
			http.Error(w, "User ID not found", http.StatusUnauthorized)
			return
		}

		eventID := chi.URLParam(r, "event_id")
		if eventID == "" {
			http.Error(w, "Missing event_id", http.StatusBadRequest)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}

		req := UpdateEventRequest{}

		if name := r.FormValue("name"); name != "" {
			req.Name = &name
		}
		if description := r.FormValue("description"); description != "" {
			req.Description = &description
		}
		if status := r.FormValue("status"); status != "" {
			req.Status = &status
		}

		if startDateStr := r.FormValue("start_date"); startDateStr != "" {
			startDate, err := time.Parse("2006-01-02T15:04", startDateStr)
			if err != nil {
				http.Error(w, "Invalid start date format", http.StatusBadRequest)
				return
			}
			req.StartDate = &startDate
		}

		if endDateStr := r.FormValue("end_date"); endDateStr != "" {
			endDate, err := time.Parse("2006-01-02T15:04", endDateStr)
			if err != nil {
				http.Error(w, "Invalid end date format", http.StatusBadRequest)
				return
			}
			req.EndDate = &endDate
		}

		_, err := h.eventSvc.UpdateEvent(r.Context(), eventID, userID, req)
		if err != nil {
			logging.Error("Failed to update event", "error", err)
			http.Error(w, "Failed to update event: "+err.Error(), http.StatusBadRequest)
			return
		}

		logging.Info("Event updated", "event_id", eventID, "va_id", activeVA.VAID)

		// Re-fetch events and render updated list
		events, err := h.eventSvc.ListEvents(r.Context(), activeVA.VAID, nil, nil)
		if err != nil {
			logging.Error("Failed to fetch events after update", "error", err)
			http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Events":   events,
			"ActiveVA": activeVA,
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/events-list.html", data); err != nil {
			logging.Error("Error rendering events list", "error", err)
			http.Error(w, "Error rendering events list", http.StatusInternalServerError)
			return
		}
	}
}

// DeleteEventHandler handles DELETE /dashboard/events/{event_id}
// Deletes an event (HTMX endpoint)
func (h *Handler) DeleteEventHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		eventID := chi.URLParam(r, "event_id")
		if eventID == "" {
			http.Error(w, "Missing event_id", http.StatusBadRequest)
			return
		}

		if err := h.eventSvc.DeleteEvent(r.Context(), eventID); err != nil {
			logging.Error("Failed to delete event", "error", err)
			http.Error(w, "Failed to delete event", http.StatusInternalServerError)
			return
		}

		logging.Info("Event deleted", "event_id", eventID, "va_id", activeVA.VAID)

		// Re-fetch events and render updated list
		events, err := h.eventSvc.ListEvents(r.Context(), activeVA.VAID, nil, nil)
		if err != nil {
			logging.Error("Failed to fetch events after delete", "error", err)
			http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Events":   events,
			"ActiveVA": activeVA,
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/events-list.html", data); err != nil {
			logging.Error("Error rendering events list", "error", err)
			http.Error(w, "Error rendering events list", http.StatusInternalServerError)
			return
		}
	}
}

// LegFormHandler handles GET /dashboard/events/{event_id}/legs/form or /dashboard/events/{event_id}/legs/form/{leg_id}
// Returns the leg form partial (HTMX endpoint)
func (h *Handler) LegFormHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		eventID := chi.URLParam(r, "event_id")
		legID := chi.URLParam(r, "leg_id")

		var legData map[string]interface{}

		if legID != "" {
			// Load existing leg
			leg, err := h.eventSvc.GetEventLeg(r.Context(), legID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Leg not found: %s", legID), http.StatusNotFound)
				return
			}

			// If route_at_id is set, fetch route details for display
			var routeDisplay string
			if leg.RouteATID != nil && *leg.RouteATID != "" {
				route, err := h.routeRepo.FindByATID(r.Context(), activeVA.VAID, *leg.RouteATID)
				if err == nil && route != nil {
					routeDisplay = route.Route
					if route.Origin != "" || route.Destination != "" {
						routeDisplay += " (" + route.Origin + "-" + route.Destination + ")"
					}
				}
			}

			legData = map[string]interface{}{
				"IsEdit":       true,
				"ID":           leg.ID,
				"EventID":      leg.EventID,
				"LegNumber":    leg.LegNumber,
				"Origin":       leg.Origin,
				"Destination":  leg.Destination,
				"RouteATID":    leg.RouteATID,
				"RouteDisplay": routeDisplay,
				"Description":  leg.Description,
				"ThumbnailURL": leg.ThumbnailURL,
			}
		} else {
			// New leg
			legData = map[string]interface{}{
				"IsEdit":  false,
				"EventID": eventID,
			}
		}

		legData["ActiveVA"] = activeVA

		// Render partial
		if err := h.templateRenderer.RenderPartial(w, "partials/event-leg-form.html", legData); err != nil {
			logging.Error("Error rendering leg form", "error", err)
			http.Error(w, "Error rendering leg form", http.StatusInternalServerError)
			return
		}
	}
}

// RouteSearchHandler handles GET /dashboard/events/routes/search
// Returns filtered routes for HTMX autocomplete in leg form
func (h *Handler) RouteSearchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			logging.Error("RouteSearchHandler: No session data")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			logging.Error("RouteSearchHandler: Invalid session data type")
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}

		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			logging.Error("RouteSearchHandler: No active VA")
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		// Get search query
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		logging.Info("RouteSearchHandler: Search query", "query", query, "va_id", activeVA.VAID)

		// If no query, return empty results
		if query == "" {
			logging.Info("RouteSearchHandler: Empty query, returning empty results")
			if err := h.templateRenderer.RenderPartial(w, "partials/route-search-results.html", map[string]interface{}{
				"Results": []interface{}{},
			}); err != nil {
				logging.Error("Error rendering search results", "error", err)
				http.Error(w, "Error rendering search results", http.StatusInternalServerError)
			}
			return
		}

		// Get all routes for this VA
		allRoutes, err := h.routeRepo.GetAllByVA(r.Context(), activeVA.VAID)
		if err != nil {
			logging.Error("Failed to fetch routes", "error", err, "va_id", activeVA.VAID)
			http.Error(w, "Failed to fetch routes", http.StatusInternalServerError)
			return
		}

		logging.Info("RouteSearchHandler: Fetched routes", "count", len(allRoutes), "va_id", activeVA.VAID)

		// Filter routes by query (case-insensitive search on route, origin, or destination)
		queryLower := strings.ToLower(query)
		var results []map[string]interface{}

		for _, route := range allRoutes {
			// Check if query matches route code, origin, or destination
			routeLower := strings.ToLower(route.Route)
			originLower := strings.ToLower(route.Origin)
			destLower := strings.ToLower(route.Destination)

			if strings.Contains(routeLower, queryLower) ||
				strings.Contains(originLower, queryLower) ||
				strings.Contains(destLower, queryLower) {

				results = append(results, map[string]interface{}{
					"Route":       route.Route,
					"Origin":      route.Origin,
					"Destination": route.Destination,
					"ATID":        route.ATID,
				})

				// Limit to 10 results
				if len(results) >= 10 {
					break
				}
			}
		}

		logging.Info("RouteSearchHandler: Filtered results", "count", len(results), "query", query)

		// Prepare template data
		data := map[string]interface{}{
			"Results": results,
		}

		// Render search results partial
		if err := h.templateRenderer.RenderPartial(w, "partials/route-search-results.html", data); err != nil {
			logging.Error("Error rendering route search results", "error", err)
			http.Error(w, "Error rendering route search results", http.StatusInternalServerError)
			return
		}
	}
}

// CreateLegHandler handles POST /dashboard/events/{event_id}/legs/create
// Creates a new leg (HTMX endpoint)
func (h *Handler) CreateLegHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID := claims.UserID()
		if userID == "" {
			http.Error(w, "User ID not found", http.StatusUnauthorized)
			return
		}

		eventID := chi.URLParam(r, "event_id")
		if eventID == "" {
			http.Error(w, "Missing event_id", http.StatusBadRequest)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}

		origin := r.FormValue("origin")
		destination := r.FormValue("destination")
		routeATID := r.FormValue("route_at_id")
		description := r.FormValue("description")
		thumbnailURL := r.FormValue("thumbnail_url")

		if origin == "" || destination == "" {
			http.Error(w, "Origin and destination are required", http.StatusBadRequest)
			return
		}

		req := CreateEventLegRequest{
			Origin:      origin,
			Destination: destination,
		}

		if routeATID != "" {
			req.RouteATID = &routeATID
		}
		if description != "" {
			req.Description = &description
		}
		if thumbnailURL != "" {
			req.ThumbnailURL = &thumbnailURL
		}

		_, err := h.eventSvc.CreateEventLeg(r.Context(), eventID, userID, req)
		if err != nil {
			logging.Error("Failed to create leg", "error", err)
			http.Error(w, "Failed to create leg: "+err.Error(), http.StatusBadRequest)
			return
		}

		logging.Info("Leg created", "event_id", eventID)

		// Re-fetch events and render updated list
		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}
		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		events, err := h.eventSvc.ListEvents(r.Context(), activeVA.VAID, nil, nil)
		if err != nil {
			logging.Error("Failed to fetch events after leg create", "error", err)
			http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Events":   events,
			"ActiveVA": activeVA,
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/events-list.html", data); err != nil {
			logging.Error("Error rendering events list", "error", err)
			http.Error(w, "Error rendering events list", http.StatusInternalServerError)
			return
		}
	}
}

// UpdateLegHandler handles POST /dashboard/events/{event_id}/legs/{leg_id}/update
// Updates an existing leg (HTMX endpoint)
func (h *Handler) UpdateLegHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID := claims.UserID()
		if userID == "" {
			http.Error(w, "User ID not found", http.StatusUnauthorized)
			return
		}

		legID := chi.URLParam(r, "leg_id")
		if legID == "" {
			http.Error(w, "Missing leg_id", http.StatusBadRequest)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}

		req := UpdateEventLegRequest{}

		if origin := r.FormValue("origin"); origin != "" {
			req.Origin = &origin
		}
		if destination := r.FormValue("destination"); destination != "" {
			req.Destination = &destination
		}
		if routeATID := r.FormValue("route_at_id"); routeATID != "" {
			req.RouteATID = &routeATID
		}
		if description := r.FormValue("description"); description != "" {
			req.Description = &description
		}
		if thumbnailURL := r.FormValue("thumbnail_url"); thumbnailURL != "" {
			req.ThumbnailURL = &thumbnailURL
		}

		_, err := h.eventSvc.UpdateEventLeg(r.Context(), legID, userID, req)
		if err != nil {
			logging.Error("Failed to update leg", "error", err)
			http.Error(w, "Failed to update leg: "+err.Error(), http.StatusBadRequest)
			return
		}

		logging.Info("Leg updated", "leg_id", legID)

		// Re-fetch events and render updated list
		sessionDataInterface := auth.GetSessionData(r.Context())
		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok || sessionData == nil {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}
		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		events, err := h.eventSvc.ListEvents(r.Context(), activeVA.VAID, nil, nil)
		if err != nil {
			logging.Error("Failed to fetch events after leg update", "error", err)
			http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Events":   events,
			"ActiveVA": activeVA,
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/events-list.html", data); err != nil {
			logging.Error("Error rendering events list", "error", err)
			http.Error(w, "Error rendering events list", http.StatusInternalServerError)
			return
		}
	}
}

// DeleteLegHandler handles DELETE /dashboard/events/{event_id}/legs/{leg_id}
// Deletes a leg (HTMX endpoint)
func (h *Handler) DeleteLegHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		legID := chi.URLParam(r, "leg_id")
		if legID == "" {
			http.Error(w, "Missing leg_id", http.StatusBadRequest)
			return
		}

		if err := h.eventSvc.DeleteEventLeg(r.Context(), legID); err != nil {
			logging.Error("Failed to delete leg", "error", err)
			http.Error(w, "Failed to delete leg", http.StatusInternalServerError)
			return
		}

		logging.Info("Leg deleted", "leg_id", legID)

		// Re-fetch events and render updated list
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		sessionData, ok := sessionDataInterface.(*session.SessionData)
		if !ok {
			http.Error(w, "Invalid session data", http.StatusInternalServerError)
			return
		}
		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		events, err := h.eventSvc.ListEvents(r.Context(), activeVA.VAID, nil, nil)
		if err != nil {
			logging.Error("Failed to fetch events after leg delete", "error", err)
			http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Events":   events,
			"ActiveVA": activeVA,
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/events-list.html", data); err != nil {
			logging.Error("Error rendering events list", "error", err)
			http.Error(w, "Error rendering events list", http.StatusInternalServerError)
			return
		}
	}
}

// ============================================================================
// API Endpoints (JSON - return JSON)
// ============================================================================

// ListEvents handles GET /api/v1/events
// Query params: status, active_only
func (h *Handler) ListEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		if vaID == "" {
			httpdto.WriteError(w, initTime, "NOT_MEMBER", "You are not a member of this virtual airline. Please join the VA first using the /membership command.", http.StatusForbidden)
			return
		}

		// Parse query params
		status := r.URL.Query().Get("status")
		activeOnlyStr := r.URL.Query().Get("active_only")
		var activeOnly *bool
		if activeOnlyStr != "" {
			val, err := strconv.ParseBool(activeOnlyStr)
			if err == nil {
				activeOnly = &val
			}
		}

		var statusPtr *string
		if status != "" {
			statusPtr = &status
		}

		events, err := h.eventSvc.ListEvents(r.Context(), vaID, statusPtr, activeOnly)
		if err != nil {
			logging.Error("Failed to list events", "error", err, "va_id", vaID)
			httpdto.WriteError(w, initTime, "LIST_ERROR", "Failed to list events", http.StatusInternalServerError)
			return
		}

		// Convert to responses
		responses := make([]EventResponse, 0, len(events))
		for _, event := range events {
			responses = append(responses, h.eventSvc.ToResponse(&event))
		}

		httpdto.WriteSuccess(w, initTime, responses, http.StatusOK)
	}
}

// GetEvent handles GET /api/v1/events/{id}
// Returns full event with legs
func (h *Handler) GetEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		eventID := chi.URLParam(r, "id")
		if eventID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Event ID is required", http.StatusBadRequest)
			return
		}

		event, err := h.eventSvc.GetEvent(r.Context(), eventID)
		if err != nil {
			logging.Error("Failed to get event", "error", err, "event_id", eventID)
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		// Verify VA ownership
		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		response := h.eventSvc.ToResponse(event)
		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}

// GetEventSummary handles GET /api/v1/events/{id}/summary
// Returns event summary (leg count, status, etc.)
func (h *Handler) GetEventSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		eventID := chi.URLParam(r, "id")
		if eventID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Event ID is required", http.StatusBadRequest)
			return
		}

		summary, err := h.eventSvc.GetEventSummary(r.Context(), eventID)
		if err != nil {
			logging.Error("Failed to get event summary", "error", err, "event_id", eventID)
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		// Verify VA ownership
		event, err := h.eventSvc.GetEvent(r.Context(), eventID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		httpdto.WriteSuccess(w, initTime, summary, http.StatusOK)
	}
}

// GetEventLeg handles GET /api/v1/events/{id}/legs/{leg_id}
// Returns specific leg
func (h *Handler) GetEventLeg() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		legID := chi.URLParam(r, "leg_id")
		if legID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Leg ID is required", http.StatusBadRequest)
			return
		}

		leg, err := h.eventSvc.GetEventLeg(r.Context(), legID)
		if err != nil {
			logging.Error("Failed to get leg", "error", err, "leg_id", legID)
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Leg not found", http.StatusNotFound)
			return
		}

		// Verify VA ownership via event
		event, err := h.eventSvc.GetEvent(r.Context(), leg.EventID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		response := h.eventSvc.ToLegResponse(leg)
		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}

// CreateEvent handles POST /api/v1/events
func (h *Handler) CreateEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		userID := claims.UserID()
		if vaID == "" || userID == "" {
			httpdto.WriteError(w, initTime, "INVALID_CLAIMS", "Invalid user claims", http.StatusBadRequest)
			return
		}

		var req CreateEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		event, err := h.eventSvc.CreateEvent(r.Context(), vaID, userID, req)
		if err != nil {
			logging.Error("Failed to create event", "error", err, "va_id", vaID)
			httpdto.WriteError(w, initTime, "CREATE_ERROR", err.Error(), http.StatusBadRequest)
			return
		}

		response := h.eventSvc.ToResponse(event)
		httpdto.WriteSuccess(w, initTime, response, http.StatusCreated)
	}
}

// UpdateEvent handles PUT /api/v1/events/{id}
func (h *Handler) UpdateEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		userID := claims.UserID()
		if userID == "" {
			httpdto.WriteError(w, initTime, "INVALID_CLAIMS", "User ID not found", http.StatusBadRequest)
			return
		}

		eventID := chi.URLParam(r, "id")
		if eventID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Event ID is required", http.StatusBadRequest)
			return
		}

		// Verify VA ownership
		event, err := h.eventSvc.GetEvent(r.Context(), eventID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		var req UpdateEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		updatedEvent, err := h.eventSvc.UpdateEvent(r.Context(), eventID, userID, req)
		if err != nil {
			logging.Error("Failed to update event", "error", err, "event_id", eventID)
			httpdto.WriteError(w, initTime, "UPDATE_ERROR", err.Error(), http.StatusBadRequest)
			return
		}

		response := h.eventSvc.ToResponse(updatedEvent)
		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}

// DeleteEvent handles DELETE /api/v1/events/{id}
func (h *Handler) DeleteEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		eventID := chi.URLParam(r, "id")
		if eventID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Event ID is required", http.StatusBadRequest)
			return
		}

		// Verify VA ownership
		event, err := h.eventSvc.GetEvent(r.Context(), eventID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		if err := h.eventSvc.DeleteEvent(r.Context(), eventID); err != nil {
			logging.Error("Failed to delete event", "error", err, "event_id", eventID)
			httpdto.WriteError(w, initTime, "DELETE_ERROR", err.Error(), http.StatusInternalServerError)
			return
		}

		httpdto.WriteSuccess(w, initTime, map[string]string{"message": "Event deleted successfully"}, http.StatusOK)
	}
}

// UpdateEventStatus handles PATCH /api/v1/events/{id}/status
func (h *Handler) UpdateEventStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		userID := claims.UserID()
		if userID == "" {
			httpdto.WriteError(w, initTime, "INVALID_CLAIMS", "User ID not found", http.StatusBadRequest)
			return
		}

		eventID := chi.URLParam(r, "id")
		if eventID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Event ID is required", http.StatusBadRequest)
			return
		}

		// Verify VA ownership
		event, err := h.eventSvc.GetEvent(r.Context(), eventID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		updatedEvent, err := h.eventSvc.UpdateEventStatus(r.Context(), eventID, userID, req.Status)
		if err != nil {
			logging.Error("Failed to update event status", "error", err, "event_id", eventID)
			httpdto.WriteError(w, initTime, "UPDATE_ERROR", err.Error(), http.StatusBadRequest)
			return
		}

		response := h.eventSvc.ToResponse(updatedEvent)
		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}

// GetEventLegs handles GET /api/v1/events/{id}/legs
func (h *Handler) GetEventLegs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		eventID := chi.URLParam(r, "id")
		if eventID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Event ID is required", http.StatusBadRequest)
			return
		}

		// Verify VA ownership
		event, err := h.eventSvc.GetEvent(r.Context(), eventID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		legs, err := h.eventSvc.GetEventLegs(r.Context(), eventID)
		if err != nil {
			logging.Error("Failed to get event legs", "error", err, "event_id", eventID)
			httpdto.WriteError(w, initTime, "LIST_ERROR", "Failed to get legs", http.StatusInternalServerError)
			return
		}

		responses := make([]EventLegResponse, 0, len(legs))
		for _, leg := range legs {
			responses = append(responses, h.eventSvc.ToLegResponse(&leg))
		}

		httpdto.WriteSuccess(w, initTime, responses, http.StatusOK)
	}
}

// CreateEventLeg handles POST /api/v1/events/{id}/legs
func (h *Handler) CreateEventLeg() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		userID := claims.UserID()
		if userID == "" {
			httpdto.WriteError(w, initTime, "INVALID_CLAIMS", "User ID not found", http.StatusBadRequest)
			return
		}

		eventID := chi.URLParam(r, "id")
		if eventID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Event ID is required", http.StatusBadRequest)
			return
		}

		// Verify VA ownership
		event, err := h.eventSvc.GetEvent(r.Context(), eventID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		var req CreateEventLegRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		leg, err := h.eventSvc.CreateEventLeg(r.Context(), eventID, userID, req)
		if err != nil {
			logging.Error("Failed to create leg", "error", err, "event_id", eventID)
			httpdto.WriteError(w, initTime, "CREATE_ERROR", err.Error(), http.StatusBadRequest)
			return
		}

		response := h.eventSvc.ToLegResponse(leg)
		httpdto.WriteSuccess(w, initTime, response, http.StatusCreated)
	}
}

// UpdateEventLeg handles PUT /api/v1/events/{id}/legs/{leg_id}
func (h *Handler) UpdateEventLeg() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		userID := claims.UserID()
		if userID == "" {
			httpdto.WriteError(w, initTime, "INVALID_CLAIMS", "User ID not found", http.StatusBadRequest)
			return
		}

		legID := chi.URLParam(r, "leg_id")
		if legID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Leg ID is required", http.StatusBadRequest)
			return
		}

		// Verify VA ownership via leg's event
		leg, err := h.eventSvc.GetEventLeg(r.Context(), legID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Leg not found", http.StatusNotFound)
			return
		}

		event, err := h.eventSvc.GetEvent(r.Context(), leg.EventID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		var req UpdateEventLegRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		updatedLeg, err := h.eventSvc.UpdateEventLeg(r.Context(), legID, userID, req)
		if err != nil {
			logging.Error("Failed to update leg", "error", err, "leg_id", legID)
			httpdto.WriteError(w, initTime, "UPDATE_ERROR", err.Error(), http.StatusBadRequest)
			return
		}

		response := h.eventSvc.ToLegResponse(updatedLeg)
		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}

// UpdateEventLegAdditionalData handles PATCH /api/v1/events/{id}/legs/{leg_id}/additional-data
func (h *Handler) UpdateEventLegAdditionalData() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		userID := claims.UserID()
		if userID == "" {
			httpdto.WriteError(w, initTime, "INVALID_CLAIMS", "User ID not found", http.StatusBadRequest)
			return
		}

		legID := chi.URLParam(r, "leg_id")
		if legID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Leg ID is required", http.StatusBadRequest)
			return
		}

		// Verify VA ownership via leg's event
		leg, err := h.eventSvc.GetEventLeg(r.Context(), legID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Leg not found", http.StatusNotFound)
			return
		}

		event, err := h.eventSvc.GetEvent(r.Context(), leg.EventID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		var req struct {
			AdditionalData AdditionalData `json:"additional_data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpdto.WriteError(w, initTime, "INVALID_REQUEST", "Invalid request body", http.StatusBadRequest)
			return
		}

		updatedLeg, err := h.eventSvc.UpdateEventLegAdditionalData(r.Context(), legID, userID, req.AdditionalData)
		if err != nil {
			logging.Error("Failed to update leg additional data", "error", err, "leg_id", legID)
			httpdto.WriteError(w, initTime, "UPDATE_ERROR", err.Error(), http.StatusBadRequest)
			return
		}

		response := h.eventSvc.ToLegResponse(updatedLeg)
		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}

// DeleteEventLeg handles DELETE /api/v1/events/{id}/legs/{leg_id}
func (h *Handler) DeleteEventLeg() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		legID := chi.URLParam(r, "leg_id")
		if legID == "" {
			httpdto.WriteError(w, initTime, "MISSING_ID", "Leg ID is required", http.StatusBadRequest)
			return
		}

		// Verify VA ownership via leg's event
		leg, err := h.eventSvc.GetEventLeg(r.Context(), legID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Leg not found", http.StatusNotFound)
			return
		}

		event, err := h.eventSvc.GetEvent(r.Context(), leg.EventID)
		if err != nil {
			httpdto.WriteError(w, initTime, "NOT_FOUND", "Event not found", http.StatusNotFound)
			return
		}

		vaID := claims.ServerID()
		if event.VAID != vaID {
			httpdto.WriteError(w, initTime, "FORBIDDEN", "Access denied", http.StatusForbidden)
			return
		}

		if err := h.eventSvc.DeleteEventLeg(r.Context(), legID); err != nil {
			logging.Error("Failed to delete leg", "error", err, "leg_id", legID)
			httpdto.WriteError(w, initTime, "DELETE_ERROR", err.Error(), http.StatusInternalServerError)
			return
		}

		httpdto.WriteSuccess(w, initTime, map[string]string{"message": "Leg deleted successfully"}, http.StatusOK)
	}
}

// GetEventPirepConfig handles GET /api/v1/events/pirep-config
// Finds user by if_community_id in live flights and matches to active event leg
// Returns event leg information and flight data for PIREP filing
func (h *Handler) GetEventPirepConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, initTime, "UNAUTHORIZED", "Missing authentication claims", http.StatusUnauthorized)
			return
		}

		// Get if_community_id from query parameter
		ifCommunityID := strings.TrimSpace(r.URL.Query().Get("if_community_id"))
		if ifCommunityID == "" {
			httpdto.WriteError(w, initTime, "MISSING_PARAM", "if_community_id query parameter is required", http.StatusBadRequest)
			return
		}

		// Get VA from Discord server ID
		vaDiscordServerID := claims.DiscordServerID()
		if vaDiscordServerID == "" {
			httpdto.WriteError(w, initTime, "INVALID_VA", "VA Discord server ID not found", http.StatusBadRequest)
			return
		}

		va, err := h.vaSvc.GetByDiscordServerID(r.Context(), vaDiscordServerID)
		if err != nil || va == nil {
			httpdto.WriteError(w, initTime, "VA_NOT_FOUND", "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Get VA live flights
		vaFlights, err := h.flightsSvc.GetVALiveFlights(r.Context(), va.ID)
		if err != nil {
			logging.Error("Failed to fetch VA live flights", "error", err, "va_id", va.ID)
			httpdto.WriteError(w, initTime, "FLIGHTS_ERROR", "Failed to fetch live flights", http.StatusInternalServerError)
			return
		}

		if vaFlights == nil || len(*vaFlights) == 0 {
			httpdto.WriteError(w, initTime, "NO_FLIGHTS", "No live flights found", http.StatusNotFound)
			return
		}

		// Find user's live flight by matching if_community_id to Username
		var userFlight *dtos.LiveFlight
		for i := range *vaFlights {
			flight := &(*vaFlights)[i]
			if strings.EqualFold(flight.Username, ifCommunityID) {
				userFlight = flight
				break
			}
		}

		if userFlight == nil {
			httpdto.WriteError(w, initTime, "FLIGHT_NOT_FOUND", "No active flight found for this user", http.StatusNotFound)
			return
		}

		// Get active multi-legged event for the VA
		activeEvent, err := h.eventSvc.GetActiveMultiLegEvent(r.Context(), va.ID)
		if err != nil {
			logging.Error("Failed to get active multi-leg event", "error", err, "va_id", va.ID)
			httpdto.WriteError(w, initTime, "EVENT_ERROR", "Failed to get active event", http.StatusInternalServerError)
			return
		}

		if activeEvent == nil {
			httpdto.WriteError(w, initTime, "NO_EVENT", "No active multi-legged event found", http.StatusNotFound)
			return
		}

		// Match flight route (origin-destination) to event leg
		// Normalize ICAO codes for comparison (uppercase, trim)
		flightOrigin := strings.ToUpper(strings.TrimSpace(userFlight.Origin))
		flightDest := strings.ToUpper(strings.TrimSpace(userFlight.Destination))

		var matchedLeg *EventLeg
		for i := range activeEvent.Legs {
			leg := &activeEvent.Legs[i]
			legOrigin := strings.ToUpper(strings.TrimSpace(leg.Origin))
			legDest := strings.ToUpper(strings.TrimSpace(leg.Destination))

			// Match if origin and destination match (in either direction for round trips)
			if (legOrigin == flightOrigin && legDest == flightDest) ||
				(legOrigin == flightDest && legDest == flightOrigin) {
				matchedLeg = leg
				break
			}
		}

		if matchedLeg == nil {
			httpdto.WriteError(w, initTime, "LEG_NOT_MATCHED", fmt.Sprintf("Flight route %s-%s does not match any event leg", flightOrigin, flightDest), http.StatusNotFound)
			return
		}

		// Build response with event leg information and flight data
		response := map[string]interface{}{
			"event": map[string]interface{}{
				"id":     activeEvent.ID,
				"name":   activeEvent.Name,
				"status": activeEvent.Status,
			},
			"leg": map[string]interface{}{
				"id":            matchedLeg.ID,
				"leg_number":    matchedLeg.LegNumber,
				"origin":        matchedLeg.Origin,
				"destination":   matchedLeg.Destination,
				"description":   matchedLeg.Description,
				"thumbnail_url": matchedLeg.ThumbnailURL,
			},
			"flight": map[string]interface{}{
				"callsign":    userFlight.Callsign,
				"aircraft":    userFlight.Aircraft,
				"livery":      userFlight.Livery,
				"livery_id":   userFlight.LiveryId,
				"origin":      userFlight.Origin,
				"destination": userFlight.Destination,
				"altitude":    userFlight.AltitudeFt,
				"speed":       userFlight.SpeedKts,
			},
		}

		httpdto.WriteSuccess(w, initTime, response, http.StatusOK)
	}
}
