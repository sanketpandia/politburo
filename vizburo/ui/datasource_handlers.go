package ui

import (
	"encoding/json"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/providers"
	"infinite-experiment/politburo/internal/services"
	"net/http"
)

// DatasourceSettingsHandler serves the datasource configuration page (admin only)
func DatasourceSettingsHandler(w http.ResponseWriter, r *http.Request) {
	// Get session data from context (guaranteed by auth middleware)
	sessionDataInterface := auth.GetSessionData(r.Context())
	sessionData, ok := sessionDataInterface.(*common.SessionData)
	if !ok {
		http.Error(w, "Invalid session data", http.StatusInternalServerError)
		return
	}

	// Prepare template data using common helper
	data, err := PrepareTemplateData(sessionData, "Datasource Configuration")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Render template
	if err := RenderTemplate(w, "pages/datasource-settings.html", data); err != nil {
		http.Error(w, "Error rendering datasource settings", http.StatusInternalServerError)
		return
	}
}

// GetDatasourceConfigHandler fetches the current datasource configuration (HTMX partial)
func GetDatasourceConfigHandler(
	w http.ResponseWriter,
	r *http.Request,
	configSvc *services.DataProviderConfigService,
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

	// Try to get active config (may return nil if no config exists)
	config, err := configSvc.GetActiveConfigCached(r.Context(), activeVA.VAID, "airtable")
	if err != nil {
		// If error, we'll just render an empty form
		config = nil
	}

	// Prepare template data
	data := map[string]interface{}{
		"ActiveVA": activeVA,
		"Config":   config,
	}

	// Render the config form partial
	if err := RenderPartial(w, "partials/datasource-config-form.html", data); err != nil {
		http.Error(w, "Error rendering datasource config form", http.StatusInternalServerError)
		return
	}
}

// SaveDatasourceConfigHandler saves the datasource configuration
func SaveDatasourceConfigHandler(
	w http.ResponseWriter,
	r *http.Request,
	configSvc *services.DataProviderConfigService,
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

	// Parse request body
	var req dtos.SaveProviderConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Call service to save config
	response, err := configSvc.SaveOrUpdateConfig(r.Context(), activeVA.VAID, &req, sessionData.UserID)
	if err != nil {
		// Return error as JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Return success response with updated config
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    response,
	})
}

// TestConnectionHandler tests the datasource connection and validates configuration
func TestConnectionHandler(
	w http.ResponseWriter,
	r *http.Request,
	airtableProvider *providers.AirtableProvider,
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

	// Parse request body to get the config to test
	var req dtos.SaveProviderConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request body",
		})
		return
	}

	// Validate the configuration
	validationResult, err := airtableProvider.ValidateConfig(r.Context(), &req.ConfigData)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Return validation result
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          validationResult.IsValid,
		"phasesCompleted":  validationResult.PhasesCompleted,
		"phasesFailed":     validationResult.PhasesFailed,
		"errors":           validationResult.Errors,
		"warnings":         validationResult.Warnings,
		"durationMs":       validationResult.DurationMs,
	})
}

// FetchTableFieldsHandler fetches fields from a specific Airtable table (HTMX partial)
func FetchTableFieldsHandler(
	w http.ResponseWriter,
	r *http.Request,
	airtableProvider *providers.AirtableProvider,
	configSvc *services.DataProviderConfigService,
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

	// Parse request body (form data from HTMX)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Extract form values
	tableName := r.FormValue("table_name")
	entityType := r.FormValue("entity_type")

	// Validate required fields in request
	if tableName == "" || entityType == "" {
		http.Error(w, "Table Name and Entity Type are required", http.StatusBadRequest)
		return
	}

	// Fetch saved credentials from database
	config, err := configSvc.GetActiveConfigCached(r.Context(), activeVA.VAID, "airtable")
	if err != nil || config == nil {
		http.Error(w, "No datasource configuration found. Please save your credentials first.", http.StatusBadRequest)
		return
	}

	// Validate credentials exist in saved config
	if config.Credentials.APIKey == "" || config.Credentials.BaseID == "" {
		http.Error(w, "Invalid configuration: missing credentials. Please re-save your credentials.", http.StatusBadRequest)
		return
	}

	// Fetch fields from Airtable using saved credentials
	fields, err := airtableProvider.FetchTableFields(r.Context(), config, tableName)
	if err != nil {
		http.Error(w, "Failed to fetch table fields: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Fetch sample record to show example values
	sampleRecord, err := airtableProvider.FetchSampleRecord(r.Context(), config, tableName)
	if err != nil {
		// Don't fail if we can't get sample record, just continue without it
		sampleRecord = make(map[string]interface{})
	}

	// Define internal fields based on entity type
	var internalFields []string
	switch entityType {
	case "pilot":
		internalFields = []string{"callsign", "total_hours", "category", "region", "last_activity", "join_date", "cm_status"}
	case "pirep":
		internalFields = []string{"callsign", "aircraft", "airline", "route_at_id", "flight_mode", "flight_time", "date_completed", "fuel_kg", "cargo_kg", "passengers", "pilot_remarks"}
	case "route":
		internalFields = []string{"origin", "destination", "route", "distance", "duration"}
	case "career_mode":
		internalFields = []string{"callsign", "total_cm_hours", "required_hours_to_next", "last_activity_cm", "assigned_routes", "aircraft", "airline"}
	default:
		internalFields = []string{}
	}

	// Retrieve existing schema for this entity type to get saved field mappings
	var existingSchema *dtos.EntitySchema
	if config != nil {
		// Find the schema for this entity type
		for _, schema := range config.Schemas {
			if schema.EntityType == entityType {
				existingSchema = &schema
				break
			}
		}
	}

	// Pre-compute field mappings: create map of airtable field name -> internal field name
	mappingLookup := make(map[string]string)
	if existingSchema != nil {
		for _, fieldMapping := range existingSchema.Fields {
			mappingLookup[fieldMapping.AirtableName] = fieldMapping.InternalName
		}
	}

	// Build FieldWithMapping array with pre-computed selections
	type FieldWithMapping struct {
		Name             string
		Type             string
		IsChecked        bool
		SelectedMapping  string
	}

	var fieldsWithMappings []FieldWithMapping
	for _, field := range fields {
		fwm := FieldWithMapping{
			Name:            field.Name,
			Type:            field.Type,
			IsChecked:       false,
			SelectedMapping: "",
		}

		// Check if this field has a saved mapping
		if selectedInternal, exists := mappingLookup[field.Name]; exists {
			fwm.IsChecked = true
			fwm.SelectedMapping = selectedInternal
		}

		fieldsWithMappings = append(fieldsWithMappings, fwm)
	}

	// Prepare template data
	data := map[string]interface{}{
		"EntityType":         entityType,
		"TableName":          tableName,
		"FieldsWithMappings": fieldsWithMappings,
		"InternalFields":     internalFields,
		"ActiveVA":           activeVA,
		"SampleRecord":       sampleRecord,
	}

	// Render the field mapper partial
	if err := RenderPartial(w, "partials/table-field-mapper.html", data); err != nil {
		http.Error(w, "Error rendering table field mapper", http.StatusInternalServerError)
		return
	}
}
