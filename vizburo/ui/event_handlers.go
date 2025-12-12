package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/gorm"
	"infinite-experiment/politburo/internal/services"

	"github.com/go-chi/chi/v5"
)

// EventsHandler serves the main Events management page for admin users
func EventsHandler(w http.ResponseWriter, r *http.Request) {
	// Get session data from context (guaranteed by admin middleware)
	sessionDataInterface := auth.GetSessionData(r.Context())
	sessionData, ok := sessionDataInterface.(*common.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data, err := PrepareTemplateData(sessionData, "Events Management")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render template
	if err := RenderTemplate(w, "pages/events.html", data); err != nil {
		http.Error(w, "Error rendering events page", http.StatusInternalServerError)
		return
	}
}

// GetEventsListHandler returns the list of events for the VA (HTMX partial)
func GetEventsListHandler(
	w http.ResponseWriter,
	r *http.Request,
	eventRepo *repositories.VAEventRepository,
	routeRepo *repositories.RouteATSyncedRepo,
) {
	// Get session data from context
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

	// Get all events for this VA
	events, err := eventRepo.GetByVA(r.Context(), activeVA.VAID)
	if err != nil {
		log.Printf("[GetEventsListHandler] Failed to fetch events: %v", err)
		http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
		return
	}

	// Get all routes for this VA for route reference
	allRoutes, err := routeRepo.GetAllByVA(r.Context(), activeVA.VAID)
	if err != nil {
		log.Printf("[GetEventsListHandler] Failed to fetch routes: %v", err)
		// Continue without routes, they're not critical
		allRoutes = []gorm.RouteATSynced{}
	}

	// Prepare template data
	data := map[string]interface{}{
		"Events":      events,
		"ActiveVA":    activeVA,
		"HasEvents":   len(events) > 0,
		"AllRoutes":   allRoutes,
	}

	// Render partial
	if err := RenderPartial(w, "partials/events-list.html", data); err != nil {
		http.Error(w, "Error rendering events list", http.StatusInternalServerError)
		return
	}
}

// GetEventFormHandler returns the create/edit form for an event (HTMX partial)
func GetEventFormHandler(
	w http.ResponseWriter,
	r *http.Request,
	eventRepo *repositories.VAEventRepository,
	routeRepo *repositories.RouteATSyncedRepo,
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

	// Get all routes for dropdown
	allRoutes, err := routeRepo.GetAllByVA(r.Context(), activeVA.VAID)
	if err != nil {
		log.Printf("[GetEventFormHandler] Failed to fetch routes: %v", err)
		allRoutes = []gorm.RouteATSynced{}
	}

	// Check if editing existing event
	eventID := chi.URLParam(r, "event_id")
	var eventData map[string]interface{}

	if eventID != "" {
		// Load existing event
		event, err := eventRepo.GetByID(r.Context(), eventID)
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
			"EventName":   event.EventName,
			"Description": event.Description,
			"Route":       event.PredefinedRoute,
			"StartDate":   startDateStr,
			"EndDate":     endDateStr,
			"IsActive":    event.IsActive,
		}
	} else {
		// New event
		eventData = map[string]interface{}{
			"IsEdit": false,
		}
	}

	eventData["AllRoutes"] = allRoutes
	eventData["ActiveVA"] = activeVA

	// Render partial
	if err := RenderPartial(w, "partials/event-form.html", eventData); err != nil {
		http.Error(w, "Error rendering event form", http.StatusInternalServerError)
		return
	}
}

// CreateEventHandler creates a new event (POST endpoint)
func CreateEventHandler(
	w http.ResponseWriter,
	r *http.Request,
	eventRepo *repositories.VAEventRepository,
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

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	eventName := r.FormValue("event_name")
	description := r.FormValue("description")
	route := r.FormValue("predefined_route")
	routeATID := r.FormValue("route_at_id")
	startDateStr := r.FormValue("start_date")
	endDateStr := r.FormValue("end_date")

	// Validate required fields
	if eventName == "" {
		http.Error(w, "Event name is required", http.StatusBadRequest)
		return
	}

	if route == "" {
		http.Error(w, "Route is required", http.StatusBadRequest)
		return
	}

	// Parse dates (both optional now)
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

	// Create event service
	eventSvc := services.NewVAEventService(eventRepo)

	// Create event
	req := services.CreateEventRequest{
		EventName:       eventName,
		PredefinedRoute: route,
		StartDate:       startDate,
		EndDate:         endDate,
		CreatedByID:     &sessionData.UserID,
	}

	if description != "" {
		req.Description = &description
	}

	if routeATID != "" {
		req.RouteATID = &routeATID
	}

	event, err := eventSvc.CreateEvent(r.Context(), activeVA.VAID, req)
	if err != nil {
		log.Printf("[CreateEventHandler] Failed to create event: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create event: %s", err.Error()), http.StatusBadRequest)
		return
	}

	log.Printf("[CreateEventHandler] Created event %s for VA %s", event.ID, activeVA.VAID)

	// Return success response (trigger list refresh)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Event created successfully",
		"event_id": event.ID,
	})
}

// UpdateEventHandler updates an existing event (POST endpoint)
func UpdateEventHandler(
	w http.ResponseWriter,
	r *http.Request,
	eventRepo *repositories.VAEventRepository,
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

	// Create update request
	req := services.UpdateEventRequest{}

	eventName := r.FormValue("event_name")
	if eventName != "" {
		req.EventName = &eventName
	}

	description := r.FormValue("description")
	if description != "" {
		req.Description = &description
	}

	route := r.FormValue("predefined_route")
	if route != "" {
		req.PredefinedRoute = &route
	}

	routeATID := r.FormValue("route_at_id")
	if routeATID != "" {
		req.RouteATID = &routeATID
	}

	startDateStr := r.FormValue("start_date")
	if startDateStr != "" {
		startDate, err := time.Parse("2006-01-02T15:04", startDateStr)
		if err != nil {
			http.Error(w, "Invalid start date format", http.StatusBadRequest)
			return
		}
		req.StartDate = &startDate
	}

	endDateStr := r.FormValue("end_date")
	if endDateStr != "" {
		endDate, err := time.Parse("2006-01-02T15:04", endDateStr)
		if err != nil {
			http.Error(w, "Invalid end date format", http.StatusBadRequest)
			return
		}
		req.EndDate = &endDate
	}

	// Update event service
	eventSvc := services.NewVAEventService(eventRepo)
	event, err := eventSvc.UpdateEvent(r.Context(), eventID, req)
	if err != nil {
		log.Printf("[UpdateEventHandler] Failed to update event: %v", err)
		http.Error(w, fmt.Sprintf("Failed to update event: %s", err.Error()), http.StatusBadRequest)
		return
	}

	log.Printf("[UpdateEventHandler] Updated event %s for VA %s", event.ID, activeVA.VAID)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Event updated successfully",
	})
}

// DeleteEventHandler deletes an event (DELETE endpoint)
func DeleteEventHandler(
	w http.ResponseWriter,
	r *http.Request,
	eventRepo *repositories.VAEventRepository,
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

	eventID := chi.URLParam(r, "event_id")
	if eventID == "" {
		http.Error(w, "Missing event_id", http.StatusBadRequest)
		return
	}

	// Delete event
	eventSvc := services.NewVAEventService(eventRepo)
	if err := eventSvc.DeleteEvent(r.Context(), eventID); err != nil {
		log.Printf("[DeleteEventHandler] Failed to delete event: %v", err)
		http.Error(w, "Failed to delete event", http.StatusInternalServerError)
		return
	}

	log.Printf("[DeleteEventHandler] Deleted event %s for VA %s", eventID, activeVA.VAID)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Event deleted successfully",
	})
}

// RouteSearchHandler returns filtered routes for HTMX autocomplete in event modal
func RouteSearchHandler(
	w http.ResponseWriter,
	r *http.Request,
	routeRepo *repositories.RouteATSyncedRepo,
) {
	// Get session data from context
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

	// Get search query
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// If no query, return empty results
	if query == "" {
		if err := RenderPartial(w, "partials/route-search-results.html", map[string]interface{}{
			"Results": []interface{}{},
		}); err != nil {
			http.Error(w, "Error rendering search results", http.StatusInternalServerError)
		}
		return
	}

	// Get all routes for this VA
	allRoutes, err := routeRepo.GetAllByVA(r.Context(), activeVA.VAID)
	if err != nil {
		log.Printf("[RouteSearchHandler] Failed to fetch routes: %v", err)
		http.Error(w, "Failed to fetch routes", http.StatusInternalServerError)
		return
	}

	// Filter routes by query (case-insensitive search on route, origin, or destination)
	queryLower := strings.ToLower(query)
	var results []map[string]interface{}

	for _, route := range allRoutes {
		// Check if query matches route code, origin, or destination
		if strings.Contains(strings.ToLower(route.Route), queryLower) ||
			strings.Contains(strings.ToLower(route.Origin), queryLower) ||
			strings.Contains(strings.ToLower(route.Destination), queryLower) {

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

	// Prepare template data
	data := map[string]interface{}{
		"Results": results,
	}

	// Render partial
	if err := RenderPartial(w, "partials/route-search-results.html", data); err != nil {
		http.Error(w, "Error rendering search results", http.StatusInternalServerError)
		return
	}
}
