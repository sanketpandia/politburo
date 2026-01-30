package ui

import (
	"infinite-experiment/politburo/infra/session"
	"fmt"
	"log"
	"net/http"
	"strings"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/services"

	"github.com/go-chi/chi/v5"
)

// PirepConfigHandler serves the main PIREP configuration page for admin users
func PirepConfigHandler(w http.ResponseWriter, r *http.Request) {
	// Get session data from context (guaranteed by admin middleware)
	sessionDataInterface := auth.GetSessionData(r.Context())
	sessionData, ok := sessionDataInterface.(*session.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data, err := PrepareTemplateData(sessionData, "PIREP Configuration")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render template
	if err := RenderTemplate(w, "pages/pirep-config.html", data); err != nil {
		http.Error(w, "Error rendering PIREP config page", http.StatusInternalServerError)
		return
	}
}

// GetPirepConfigHandler returns the list of configured PIREP modes (HTMX partial)
func GetPirepConfigHandler(
	w http.ResponseWriter,
	r *http.Request,
	vaGormRepo *repositories.VAGormRepository,
) {
	// Get session data from context
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

	// Create service with repository (avoids circular imports)
	configSvc := services.NewFlightModesConfigService(vaGormRepo)

	// Fetch config from service
	config, err := configSvc.GetConfig(r.Context(), activeVA.VAID)
	if err != nil {
		log.Printf("[GetPirepConfigHandler] Failed to fetch config: %v", err)
		http.Error(w, "Failed to fetch PIREP configuration", http.StatusInternalServerError)
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
	if err := RenderPartial(w, "partials/pirep-modes-list.html", data); err != nil {
		http.Error(w, "Error rendering modes list", http.StatusInternalServerError)
		return
	}
}

// UpdatePirepModeHandler updates a specific PIREP mode configuration (HTMX endpoint)
func UpdatePirepModeHandler(
	w http.ResponseWriter,
	r *http.Request,
	vaGormRepo *repositories.VAGormRepository,
) {
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

	// Create service with repository (avoids circular imports)
	configSvc := services.NewFlightModesConfigService(vaGormRepo)

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
	config, err := configSvc.GetConfig(r.Context(), activeVA.VAID)
	if err != nil {
		log.Printf("[UpdatePirepModeHandler] Failed to fetch config: %v", err)
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
	if err := configSvc.ValidateAndSaveConfig(r.Context(), activeVA.VAID, config); err != nil {
		log.Printf("[UpdatePirepModeHandler] Failed to save config: %v", err)
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	log.Printf("[UpdatePirepModeHandler] Updated mode %s for VA %s", modeID, activeVA.VAID)

	// Re-fetch and render updated modes list
	updatedConfig, err := configSvc.GetConfig(r.Context(), activeVA.VAID)
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

	if err := RenderPartial(w, "partials/pirep-modes-list.html", data); err != nil {
		http.Error(w, "Error rendering updated modes list", http.StatusInternalServerError)
		return
	}
}

// TogglePirepModeHandler enables/disables a specific PIREP mode (HTMX endpoint)
func TogglePirepModeHandler(
	w http.ResponseWriter,
	r *http.Request,
	vaGormRepo *repositories.VAGormRepository,
) {
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

	// Create service with repository (avoids circular imports)
	configSvc := services.NewFlightModesConfigService(vaGormRepo)

	// Get mode ID from URL parameter
	modeID := chi.URLParam(r, "mode_id")
	if modeID == "" {
		http.Error(w, "Missing mode_id in URL", http.StatusBadRequest)
		return
	}

	// Fetch current config
	config, err := configSvc.GetConfig(r.Context(), activeVA.VAID)
	if err != nil {
		log.Printf("[TogglePirepModeHandler] Failed to fetch config: %v", err)
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
	if err := configSvc.ValidateAndSaveConfig(r.Context(), activeVA.VAID, config); err != nil {
		log.Printf("[TogglePirepModeHandler] Failed to save config: %v", err)
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	log.Printf("[TogglePirepModeHandler] Toggled mode %s for VA %s (enabled=%v)", modeID, activeVA.VAID, !currentEnabled)

	// Re-fetch and render updated modes list
	updatedConfig, err := configSvc.GetConfig(r.Context(), activeVA.VAID)
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

	if err := RenderPartial(w, "partials/pirep-modes-list.html", data); err != nil {
		http.Error(w, "Error rendering updated modes list", http.StatusInternalServerError)
		return
	}
}

// GetPirepModeEditHandler returns the edit form for a specific PIREP mode (HTMX endpoint)
func GetPirepModeEditHandler(
	w http.ResponseWriter,
	r *http.Request,
	vaGormRepo *repositories.VAGormRepository,
) {
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

	// Create service with repository
	configSvc := services.NewFlightModesConfigService(vaGormRepo)

	// Get mode ID from URL parameter
	modeID := chi.URLParam(r, "mode_id")
	if modeID == "" {
		http.Error(w, "Missing mode_id in URL", http.StatusBadRequest)
		return
	}

	// Fetch current config
	config, err := configSvc.GetConfig(r.Context(), activeVA.VAID)
	if err != nil {
		log.Printf("[GetPirepModeEditHandler] Failed to fetch config: %v", err)
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
	if err := RenderPartial(w, "partials/pirep-mode-edit-form.html", modeData); err != nil {
		http.Error(w, "Error rendering edit form", http.StatusInternalServerError)
		return
	}
}
