package vaadmin

import (
	"fmt"
	"net/http"
	"strings"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/pilots"
	"infinite-experiment/politburo/internal/platform/roles"
	platformVA "infinite-experiment/politburo/internal/platform/va"

	"github.com/go-chi/chi/v5"
)

// Handler handles VA admin UI endpoints
type Handler struct {
	pilotMgmtSvc     *pilots.ManagementService
	vaSvc            *platformVA.Service
	templateRenderer *templates.Renderer
}

// NewHandler creates a new VA admin handler instance
func NewHandler(pilotMgmtSvc *pilots.ManagementService, vaSvc *platformVA.Service, templateRenderer *templates.Renderer) *Handler {
	return &Handler{
		pilotMgmtSvc:     pilotMgmtSvc,
		vaSvc:            vaSvc,
		templateRenderer: templateRenderer,
	}
}

// PilotsPageHandler handles GET /dashboard/vaadmin/pilots
// Renders the full pilots management page for VA admins
func (h *Handler) PilotsPageHandler() http.HandlerFunc {
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
		data, err := templates.PrepareTemplateData(sessionData, "VA Admin - Pilots")
		if err != nil {
			logging.Error("Failed to prepare template data", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add current page identifier for menu highlighting
		data["CurrentPage"] = "vaadmin-pilots"

		// Render template
		if err := h.templateRenderer.RenderTemplate(w, "pages/vaadmin-pilots.html", data); err != nil {
			logging.Error("Error rendering pilots page", "error", err)
			http.Error(w, "Error rendering pilots page", http.StatusInternalServerError)
			return
		}
	}
}

// PilotsListHandler handles GET /dashboard/vaadmin/pilots/list
// Returns the pilots table partial (HTMX endpoint)
func (h *Handler) PilotsListHandler() http.HandlerFunc {
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

		// Get pilots from service
		pilotsList, err := h.pilotMgmtSvc.GetPilotsByVAID(
			r.Context(),
			activeVA.VAID,
			roles.VARole(activeVA.Role),
		)
		if err != nil {
			logging.Error("Failed to fetch pilots", "error", err, "va_id", activeVA.VAID)
			http.Error(w, "Failed to fetch pilots: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if pilotsList == nil {
			pilotsList = []pilots.PilotDTO{}
		}

		// Prepare template data
		data := map[string]interface{}{
			"Pilots":   pilotsList,
			"ActiveVA": activeVA,
			"IsAdmin":  activeVA.Role == "admin",
		}

		// Render partial
		if err := h.templateRenderer.RenderPartial(w, "partials/pilots-table.html", data); err != nil {
			logging.Error("Error rendering pilots table", "error", err)
			http.Error(w, "Error rendering pilots table", http.StatusInternalServerError)
			return
		}
	}
}

// UpdatePilotRoleHandler handles POST /dashboard/vaadmin/pilots/{pilot_id}/role
// Updates a pilot's role (HTMX endpoint)
func (h *Handler) UpdatePilotRoleHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data (guaranteed by auth middleware)
		sessionDataInterface := auth.GetSessionData(r.Context())
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
		err := h.pilotMgmtSvc.UpdatePilotRole(
			r.Context(),
			activeVA.VAID,
			pilotID,
			newRole,
			roles.VARole(activeVA.Role),
		)
		if err != nil {
			logging.Error("Failed to update pilot role", "error", err, "pilot_id", pilotID, "new_role", newRole)
			http.Error(w, "Failed to update pilot role: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Re-fetch pilots and render updated table
		pilotsList, err := h.pilotMgmtSvc.GetPilotsByVAID(
			r.Context(),
			activeVA.VAID,
			roles.VARole(activeVA.Role),
		)
		if err != nil {
			logging.Error("Failed to fetch updated pilots", "error", err)
			http.Error(w, "Failed to fetch updated pilots", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Pilots":   pilotsList,
			"ActiveVA": activeVA,
			"IsAdmin":  activeVA.Role == "admin",
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/pilots-table.html", data); err != nil {
			logging.Error("Error rendering pilots table", "error", err)
			http.Error(w, "Error rendering pilots table", http.StatusInternalServerError)
			return
		}
	}
}

// UpdatePilotCallsignHandler handles POST /dashboard/vaadmin/pilots/{pilot_id}/callsign
// Updates a pilot's callsign (HTMX endpoint)
func (h *Handler) UpdatePilotCallsignHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data (guaranteed by auth middleware)
		sessionDataInterface := auth.GetSessionData(r.Context())
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

		// Get pilot ID from URL parameter
		pilotID := chi.URLParam(r, "pilot_id")
		if pilotID == "" {
			http.Error(w, "Missing pilot_id in URL", http.StatusBadRequest)
			return
		}

		// Get new callsign from form data
		newCallsign := r.FormValue("callsign")

		// Update callsign via service
		err := h.pilotMgmtSvc.UpdatePilotCallsign(
			r.Context(),
			activeVA.VAID,
			pilotID,
			newCallsign,
			roles.VARole(activeVA.Role),
		)
		if err != nil {
			logging.Error("Failed to update callsign", "error", err, "pilot_id", pilotID, "callsign", newCallsign)
			http.Error(w, "Failed to update callsign: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Re-fetch pilots and render updated table
		pilotsList, err := h.pilotMgmtSvc.GetPilotsByVAID(
			r.Context(),
			activeVA.VAID,
			roles.VARole(activeVA.Role),
		)
		if err != nil {
			logging.Error("Failed to fetch updated pilots", "error", err)
			http.Error(w, "Failed to fetch updated pilots", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Pilots":   pilotsList,
			"ActiveVA": activeVA,
			"IsAdmin":  activeVA.Role == "admin",
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/pilots-table.html", data); err != nil {
			logging.Error("Error rendering pilots table", "error", err)
			http.Error(w, "Error rendering pilots table", http.StatusInternalServerError)
			return
		}
	}
}

// RemovePilotHandler handles DELETE /dashboard/vaadmin/pilots/{pilot_id}
// Removes a pilot (soft delete, HTMX endpoint)
func (h *Handler) RemovePilotHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session data (guaranteed by auth middleware)
		sessionDataInterface := auth.GetSessionData(r.Context())
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

		// Get pilot ID from URL parameter
		pilotID := chi.URLParam(r, "pilot_id")
		if pilotID == "" {
			http.Error(w, "Missing pilot_id in URL", http.StatusBadRequest)
			return
		}

		// Remove pilot via service
		err := h.pilotMgmtSvc.RemovePilot(
			r.Context(),
			activeVA.VAID,
			pilotID,
			roles.VARole(activeVA.Role),
		)
		if err != nil {
			logging.Error("Failed to remove pilot", "error", err, "pilot_id", pilotID)
			http.Error(w, "Failed to remove pilot: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Re-fetch pilots and render updated table
		pilotsList, err := h.pilotMgmtSvc.GetPilotsByVAID(
			r.Context(),
			activeVA.VAID,
			roles.VARole(activeVA.Role),
		)
		if err != nil {
			logging.Error("Failed to fetch updated pilots", "error", err)
			http.Error(w, "Failed to fetch updated pilots", http.StatusInternalServerError)
			return
		}

		data := map[string]interface{}{
			"Pilots":   pilotsList,
			"ActiveVA": activeVA,
			"IsAdmin":  activeVA.Role == "admin",
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/pilots-table.html", data); err != nil {
			logging.Error("Error rendering pilots table", "error", err)
			http.Error(w, "Error rendering pilots table", http.StatusInternalServerError)
			return
		}
	}
}

// FlightModesListHandler handles GET /dashboard/vaadmin/flight-modes/list
// Returns the flight modes list partial (HTMX endpoint)
func (h *Handler) FlightModesListHandler() http.HandlerFunc {
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

		// Fetch config from service
		config, err := h.vaSvc.GetFlightModesConfig(r.Context(), activeVA.VAID)
		if err != nil {
			logging.Error("Failed to fetch flight modes config", "error", err, "va_id", activeVA.VAID)
			http.Error(w, "Failed to fetch flight modes configuration", http.StatusInternalServerError)
			return
		}

		// Extract flight modes from config
		var modes []map[string]interface{}
		if config != nil {
			if flightModes, ok := config["flight_modes"]; ok {
				if modesMap, ok := flightModes.(map[string]interface{}); ok {
					for modeID, modeData := range modesMap {
						if modeObj, ok := modeData.(map[string]interface{}); ok {
							// Add the mode ID to the mode object
							modeObj["mode_id"] = modeID
							modes = append(modes, modeObj)
						}
					}
				}
			}
		}

		// Prepare template data
		data := map[string]interface{}{
			"Modes":    modes,
			"ActiveVA": activeVA,
			"HasModes": len(modes) > 0,
		}

		// Render partial
		if err := h.templateRenderer.RenderPartial(w, "partials/flight-modes-list.html", data); err != nil {
			logging.Error("Error rendering flight modes list", "error", err)
			http.Error(w, "Error rendering flight modes list", http.StatusInternalServerError)
			return
		}
	}
}

// GetFlightModeEditHandler handles GET /dashboard/vaadmin/flight-modes/{mode_id}/edit
// Returns the edit form for a specific flight mode (HTMX partial)
func (h *Handler) GetFlightModeEditHandler() http.HandlerFunc {
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

		// Get mode ID from URL parameter
		modeID := chi.URLParam(r, "mode_id")
		if modeID == "" {
			http.Error(w, "Missing mode_id in URL", http.StatusBadRequest)
			return
		}

		// Fetch current config
		config, err := h.vaSvc.GetFlightModesConfig(r.Context(), activeVA.VAID)
		if err != nil {
			logging.Error("Failed to fetch flight modes config", "error", err)
			http.Error(w, "Failed to fetch configuration", http.StatusInternalServerError)
			return
		}

		// Extract flight modes and find the specific mode
		flightModes, ok := config["flight_modes"].(map[string]interface{})
		if !ok {
			http.Error(w, "No flight modes configured", http.StatusBadRequest)
			return
		}

		// Get the mode data
		modeData, ok := flightModes[modeID].(map[string]interface{})
		if !ok {
			http.Error(w, fmt.Sprintf("Mode not found: %s", modeID), http.StatusNotFound)
			return
		}

		// Add the mode_id to the data for template use
		modeData["mode_id"] = modeID

		// Also include Fields array reference for the JSON template function
		if fields, ok := modeData["fields"].([]interface{}); ok {
			// Convert to proper structure for JSON marshaling in template
			modeData["Fields"] = fields
		}

		// Render the edit form partial
		if err := h.templateRenderer.RenderPartial(w, "partials/flight-mode-edit-form.html", modeData); err != nil {
			logging.Error("Error rendering edit form", "error", err)
			http.Error(w, "Error rendering edit form", http.StatusInternalServerError)
			return
		}
	}
}

// ToggleFlightModeHandler handles POST /dashboard/vaadmin/flight-modes/{mode_id}/toggle
// Enables/disables a specific flight mode (HTMX endpoint)
func (h *Handler) ToggleFlightModeHandler() http.HandlerFunc {
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

		// Get mode ID from URL parameter
		modeID := chi.URLParam(r, "mode_id")
		if modeID == "" {
			http.Error(w, "Missing mode_id in URL", http.StatusBadRequest)
			return
		}

		// Fetch current config
		config, err := h.vaSvc.GetFlightModesConfig(r.Context(), activeVA.VAID)
		if err != nil {
			logging.Error("Failed to fetch flight modes config", "error", err)
			http.Error(w, "Failed to fetch configuration", http.StatusInternalServerError)
			return
		}

		// Extract flight modes
		flightModes, ok := config["flight_modes"].(map[string]interface{})
		if !ok {
			http.Error(w, "No flight modes configured", http.StatusBadRequest)
			return
		}

		// Get the mode to toggle
		modeData, ok := flightModes[modeID].(map[string]interface{})
		if !ok {
			http.Error(w, fmt.Sprintf("Mode not found: %s", modeID), http.StatusNotFound)
			return
		}

		// Toggle enabled state
		currentEnabled, _ := modeData["enabled"].(bool)
		modeData["enabled"] = !currentEnabled

		// Update config
		config["flight_modes"] = flightModes

		// Save config using service (includes validation)
		if err := h.vaSvc.ValidateAndSaveFlightModesConfig(r.Context(), activeVA.VAID, config); err != nil {
			logging.Error("Failed to save flight modes config", "error", err)
			http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
			return
		}

		logging.Info("Toggled flight mode", "mode_id", modeID, "va_id", activeVA.VAID, "enabled", !currentEnabled)

		// Re-fetch and render updated modes list
		updatedConfig, err := h.vaSvc.GetFlightModesConfig(r.Context(), activeVA.VAID)
		if err != nil {
			http.Error(w, "Failed to fetch updated config", http.StatusInternalServerError)
			return
		}

		// Extract modes for re-render
		var modes []map[string]interface{}
		if updatedConfig != nil {
			if flightModes, ok := updatedConfig["flight_modes"]; ok {
				if modesMap, ok := flightModes.(map[string]interface{}); ok {
					for mID, mData := range modesMap {
						if mObj, ok := mData.(map[string]interface{}); ok {
							mObj["mode_id"] = mID
							modes = append(modes, mObj)
						}
					}
				}
			}
		}

		data := map[string]interface{}{
			"Modes":    modes,
			"ActiveVA": activeVA,
			"HasModes": len(modes) > 0,
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/flight-modes-list.html", data); err != nil {
			logging.Error("Error rendering updated flight modes list", "error", err)
			http.Error(w, "Error rendering updated flight modes list", http.StatusInternalServerError)
			return
		}
	}
}

// UpdateFlightModeHandler handles POST /dashboard/vaadmin/flight-modes/{mode_id}/update
// Updates a specific flight mode configuration (HTMX endpoint)
func (h *Handler) UpdateFlightModeHandler() http.HandlerFunc {
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

		// Get mode ID from URL parameter
		modeID := chi.URLParam(r, "mode_id")
		if modeID == "" {
			http.Error(w, "Missing mode_id in URL", http.StatusBadRequest)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}

		displayName := strings.TrimSpace(r.FormValue("display_name"))
		description := strings.TrimSpace(r.FormValue("description"))
		requiresRouteStr := r.FormValue("requires_route_selection")
		requiresRoute := requiresRouteStr == "on" || requiresRouteStr == "true"

		// Validate required field
		if displayName == "" {
			http.Error(w, "Display name is required", http.StatusBadRequest)
			return
		}

		// Fetch current config
		config, err := h.vaSvc.GetFlightModesConfig(r.Context(), activeVA.VAID)
		if err != nil {
			logging.Error("Failed to fetch flight modes config", "error", err)
			http.Error(w, "Failed to fetch configuration", http.StatusInternalServerError)
			return
		}

		// Extract flight modes
		flightModes, ok := config["flight_modes"].(map[string]interface{})
		if !ok {
			flightModes = make(map[string]interface{})
		}

		// Get the mode to update
		modeData, ok := flightModes[modeID].(map[string]interface{})
		if !ok {
			http.Error(w, fmt.Sprintf("Mode not found: %s", modeID), http.StatusNotFound)
			return
		}

		// Update the mode fields
		modeData["display_name"] = displayName
		modeData["description"] = description
		modeData["requires_route_selection"] = requiresRoute

		// Process field visibility updates from form
		if fields, ok := modeData["fields"].([]interface{}); ok {
			for _, fieldData := range fields {
				if field, ok := fieldData.(map[string]interface{}); ok {
					fieldName, _ := field["name"].(string)

					// Check if field visibility was toggled (the form sends field_show_* checkboxes)
					// By default, if not in the form, it wasn't checked (unchecked checkbox), so show_in_discord = false
					// If it was in the form AND checked, show_in_discord = true
					fieldShowValue := r.FormValue("field_show_" + fieldName)
					field["show_in_discord"] = fieldShowValue == "on" || fieldShowValue == "true"
				}
			}
		}

		// Update config
		config["flight_modes"] = flightModes

		// Save config using service (includes validation)
		if err := h.vaSvc.ValidateAndSaveFlightModesConfig(r.Context(), activeVA.VAID, config); err != nil {
			logging.Error("Failed to save flight modes config", "error", err)
			http.Error(w, "Failed to save configuration: "+err.Error(), http.StatusInternalServerError)
			return
		}

		logging.Info("Updated flight mode", "mode_id", modeID, "va_id", activeVA.VAID)

		// Re-fetch and render updated modes list
		updatedConfig, err := h.vaSvc.GetFlightModesConfig(r.Context(), activeVA.VAID)
		if err != nil {
			http.Error(w, "Failed to fetch updated config", http.StatusInternalServerError)
			return
		}

		// Extract modes for re-render
		var modes []map[string]interface{}
		if updatedConfig != nil {
			if flightModes, ok := updatedConfig["flight_modes"]; ok {
				if modesMap, ok := flightModes.(map[string]interface{}); ok {
					for mID, mData := range modesMap {
						if mObj, ok := mData.(map[string]interface{}); ok {
							mObj["mode_id"] = mID
							modes = append(modes, mObj)
						}
					}
				}
			}
		}

		data := map[string]interface{}{
			"Modes":    modes,
			"ActiveVA": activeVA,
			"HasModes": len(modes) > 0,
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/flight-modes-list.html", data); err != nil {
			logging.Error("Error rendering updated flight modes list", "error", err)
			http.Error(w, "Error rendering updated flight modes list", http.StatusInternalServerError)
			return
		}
	}
}
