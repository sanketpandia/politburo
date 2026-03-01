package datasource

import (
	"encoding/json"
	"fmt"
	"net/http"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/providers"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/models/dtos"
	platformVA "infinite-experiment/politburo/internal/platform/va"

	"github.com/go-chi/chi/v5"
)

// Handler handles datasource configuration UI endpoints
type Handler struct {
	vaSvc            *platformVA.Service
	templateRenderer *templates.Renderer
	airtableProvider *providers.AirtableProvider
}

// NewHandler creates a new datasource handler instance
func NewHandler(vaSvc *platformVA.Service, templateRenderer *templates.Renderer, airtableProvider *providers.AirtableProvider) *Handler {
	return &Handler{
		vaSvc:            vaSvc,
		templateRenderer: templateRenderer,
		airtableProvider: airtableProvider,
	}
}

// DatasourcePageHandler handles GET /dashboard/settings/datasource
// Renders the full datasource configuration page
func (h *Handler) DatasourcePageHandler() http.HandlerFunc {
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
		data, err := templates.PrepareTemplateData(sessionData, "Datasource Configuration")
		if err != nil {
			logging.Error("Failed to prepare template data", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data["CurrentPage"] = "datasource"

		// Render template
		if err := h.templateRenderer.RenderTemplate(w, "pages/datasource.html", data); err != nil {
			logging.Error("Error rendering datasource page", "error", err)
			http.Error(w, "Error rendering datasource page", http.StatusInternalServerError)
			return
		}
	}
}

// GetDatasourceStatusHandler handles GET /dashboard/settings/datasource/status
// Returns HTMX partial showing current datasource status
func (h *Handler) GetDatasourceStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		// Check if credentials exist
		creds, err := h.vaSvc.GetAirtableCredentials(r.Context(), activeVA.VAID)
		if err != nil {
			logging.Error("Failed to get credentials", "error", err)
			http.Error(w, "Failed to check datasource status", http.StatusInternalServerError)
			return
		}

		// Check if schemas exist
		schemas, err := h.vaSvc.GetAirtableSchemas(r.Context(), activeVA.VAID)
		if err != nil {
			logging.Error("Failed to get schemas", "error", err)
			http.Error(w, "Failed to check datasource status", http.StatusInternalServerError)
			return
		}

		// Determine status
		hasCredentials := creds != nil
		hasSchemas := len(schemas) > 0

		data := map[string]interface{}{
			"ActiveVA":       activeVA,
			"HasCredentials": hasCredentials,
			"HasSchemas":     hasSchemas,
			"Credentials":    creds,
			"Schemas":        schemas,
		}

		// Render status partial
		if err := h.templateRenderer.RenderPartial(w, "partials/datasource-status.html", data); err != nil {
			logging.Error("Error rendering datasource status", "error", err)
			http.Error(w, "Error rendering datasource status", http.StatusInternalServerError)
			return
		}
	}
}

// GetSchemaTypeSelectorHandler handles GET /dashboard/settings/datasource/schema-selector
// Returns HTMX partial for selecting schema type to add
func (h *Handler) GetSchemaTypeSelectorHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		// Get existing schemas to show which are already configured
		schemas, err := h.vaSvc.GetAirtableSchemas(r.Context(), activeVA.VAID)
		if err != nil {
			logging.Error("Failed to get schemas", "error", err)
			// Continue anyway - just won't show configured status
			schemas = make(map[string]*platformVA.SchemaConfig)
		}

		// Define available schema types
		schemaTypes := []map[string]interface{}{
			{
				"Type":              "pilot",
				"DisplayName":       "Pilot",
				"Description":       "Sync pilot data including callsigns, ranks, and flight hours",
				"Icon":              "👨‍✈️",
				"AlreadyConfigured": schemas["pilot"] != nil,
			},
			{
				"Type":              "route",
				"DisplayName":       "Route",
				"Description":       "Sync flight routes with origin, destination, and route information",
				"Icon":              "🛫",
				"AlreadyConfigured": schemas["route"] != nil,
			},
			{
				"Type":              "pirep",
				"DisplayName":       "PIREP",
				"Description":       "Sync flight reports with flight details, aircraft, and completion data",
				"Icon":              "📋",
				"AlreadyConfigured": schemas["pirep"] != nil,
			},
			{
				"Type":              "career_mode",
				"DisplayName":       "Career Mode",
				"Description":       "Sync career mode progress including hours, ranks, and assigned routes",
				"Icon":              "🎯",
				"AlreadyConfigured": schemas["career_mode"] != nil,
			},
		}

		data := map[string]interface{}{
			"SchemaTypes": schemaTypes,
			"ActiveVA":    activeVA,
		}

		// Render schema selector partial
		if err := h.templateRenderer.RenderPartial(w, "partials/datasource-schema-selector.html", data); err != nil {
			logging.Error("Error rendering schema selector", "error", err)
			http.Error(w, "Error rendering schema selector", http.StatusInternalServerError)
			return
		}
	}
}

// GetDatasourceTypeSelectorHandler handles GET /dashboard/settings/datasource/type-selector
// Returns HTMX partial for selecting datasource type
func (h *Handler) GetDatasourceTypeSelectorHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"Types": GetAllDatasourceTypes(),
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/datasource-type-selector.html", data); err != nil {
			logging.Error("Error rendering type selector", "error", err)
			http.Error(w, "Error rendering type selector", http.StatusInternalServerError)
			return
		}
	}
}

// GetCredentialsFormHandler handles GET /dashboard/settings/datasource/credentials-form
// Returns HTMX partial for credentials form (datasource-specific)
func (h *Handler) GetCredentialsFormHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		datasourceType := r.URL.Query().Get("type")
		if datasourceType == "" {
			datasourceType = string(DatasourceTypeAirtable)
		}

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

		// Get existing credentials if any
		var existingCreds *platformVA.ProviderCredentials
		if datasourceType == string(DatasourceTypeAirtable) {
			creds, err := h.vaSvc.GetAirtableCredentials(r.Context(), activeVA.VAID)
			if err == nil && creds != nil {
				existingCreds = creds
			}
		}

		data := map[string]interface{}{
			"DatasourceType": datasourceType,
			"Credentials":    existingCreds,
			"ActiveVA":       activeVA,
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/datasource-credentials-form.html", data); err != nil {
			logging.Error("Error rendering credentials form", "error", err)
			http.Error(w, "Error rendering credentials form", http.StatusInternalServerError)
			return
		}
	}
}

// SaveCredentialsHandler handles POST /dashboard/settings/datasource/credentials
// Saves credentials via VA service
func (h *Handler) SaveCredentialsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		datasourceType := r.FormValue("datasource_type")
		if datasourceType == "" {
			datasourceType = string(DatasourceTypeAirtable)
		}

		// Parse form data based on datasource type
		if datasourceType == string(DatasourceTypeAirtable) {
			creds := &platformVA.ProviderCredentials{
				APIKey: r.FormValue("api_key"),
				BaseID: r.FormValue("base_id"),
				SyncSettings: platformVA.SyncSettings{
					BatchSize:          parseIntOrDefault(r.FormValue("batch_size"), 100),
					RateLimitPerSecond: parseIntOrDefault(r.FormValue("rate_limit_per_second"), 5),
					RetryAttempts:      parseIntOrDefault(r.FormValue("retry_attempts"), 3),
					TimeoutSeconds:     parseIntOrDefault(r.FormValue("timeout_seconds"), 30),
				},
			}

			// Validate
			if creds.APIKey == "" {
				http.Error(w, "API key is required", http.StatusBadRequest)
				return
			}
			if creds.BaseID == "" {
				http.Error(w, "Base ID is required", http.StatusBadRequest)
				return
			}

			// Save via VA service
			if err := h.vaSvc.SaveAirtableCredentials(r.Context(), activeVA.VAID, creds); err != nil {
				logging.Error("Failed to save credentials", "error", err)
				http.Error(w, "Failed to save credentials: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Return success - reload status
			w.Header().Set("HX-Trigger", "credentials-saved")
			http.Redirect(w, r, "/dashboard/settings/datasource/status", http.StatusSeeOther)
			return
		}

		http.Error(w, "Unsupported datasource type", http.StatusBadRequest)
	}
}

// TestConnectionHandler handles POST /dashboard/settings/datasource/test-connection
// Tests the datasource connection
func (h *Handler) TestConnectionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionDataInterface := auth.GetSessionData(r.Context())
		if sessionDataInterface == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		datasourceType := r.FormValue("datasource_type")
		if datasourceType == "" {
			datasourceType = string(DatasourceTypeAirtable)
		}

		if datasourceType == string(DatasourceTypeAirtable) {
			creds := &platformVA.ProviderCredentials{
				APIKey: r.FormValue("api_key"),
				BaseID: r.FormValue("base_id"),
			}

			if creds.APIKey == "" || creds.BaseID == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "API key and Base ID are required",
				})
				return
			}

			// Test connection using provider
			result, err := h.airtableProvider.ValidateConfig(r.Context(), creds)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			if result.IsValid {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"message": "Connection successful",
					"result":  result,
				})
			} else {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Connection failed",
					"result":  result,
				})
			}
			return
		}

		http.Error(w, "Unsupported datasource type", http.StatusBadRequest)
	}
}

// GetSchemaConfigHandler handles GET /dashboard/settings/datasource/schema/{schemaType}
// Returns HTMX partial for schema configuration
func (h *Handler) GetSchemaConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		schemaType := chi.URLParam(r, "schemaType")
		if schemaType == "" {
			http.Error(w, "Schema type is required", http.StatusBadRequest)
			return
		}

		// Get existing schema if any
		schema, err := h.vaSvc.GetAirtableSchema(r.Context(), activeVA.VAID, schemaType)
		if err != nil {
			logging.Error("Failed to get schema", "error", err)
			// Continue with nil schema for new configuration
		}

		// Get credentials to fetch table fields
		creds, err := h.vaSvc.GetAirtableCredentials(r.Context(), activeVA.VAID)
		if err != nil || creds == nil {
			http.Error(w, "Credentials must be configured first", http.StatusBadRequest)
			return
		}

		data := map[string]interface{}{
			"SchemaType":  schemaType,
			"Schema":      schema,
			"Credentials": creds,
			"ActiveVA":    activeVA,
		}

		if err := h.templateRenderer.RenderPartial(w, "partials/datasource-schema-config.html", data); err != nil {
			logging.Error("Error rendering schema config", "error", err)
			http.Error(w, "Error rendering schema config", http.StatusInternalServerError)
			return
		}
	}
}

// SyncTableSchemaHandler handles POST /dashboard/settings/datasource/schema/{schemaType}/sync
// Fetches table fields from Airtable and returns them for field mapping
func (h *Handler) SyncTableSchemaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		schemaType := chi.URLParam(r, "schemaType")
		if schemaType == "" {
			http.Error(w, "Schema type is required", http.StatusBadRequest)
			return
		}

		tableName := r.FormValue("table_name")
		if tableName == "" {
			http.Error(w, "Table name is required", http.StatusBadRequest)
			return
		}

		// Get credentials
		creds, err := h.vaSvc.GetAirtableCredentials(r.Context(), activeVA.VAID)
		if err != nil || creds == nil {
			http.Error(w, "Credentials must be configured first", http.StatusBadRequest)
			return
		}

		// Fetch table fields from Airtable
		fields, err := h.airtableProvider.FetchTableFields(r.Context(), creds, tableName)
		if err != nil {
			logging.Error("Failed to fetch table fields", "error", err)
			http.Error(w, "Failed to fetch table fields: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Get existing schema to preserve mappings
		existingSchema, _ := h.vaSvc.GetAirtableSchema(r.Context(), activeVA.VAID, schemaType)

		// Get internal fields for this schema type
		internalFields := getInternalFieldsForSchemaType(schemaType)

		// Prepare data for template
		data := map[string]interface{}{
			"SchemaType":     schemaType,
			"TableName":      tableName,
			"AirtableFields": fields,
			"InternalFields": internalFields,
			"ExistingSchema": existingSchema,
			"ActiveVA":       activeVA,
		}

		// Render field mapper partial
		if err := h.templateRenderer.RenderPartial(w, "partials/datasource-field-mapper.html", data); err != nil {
			logging.Error("Error rendering field mapper", "error", err)
			http.Error(w, "Error rendering field mapper", http.StatusInternalServerError)
			return
		}
	}
}

// SaveSchemaHandler handles POST /dashboard/settings/datasource/schema/{schemaType}
// Saves schema configuration via VA service
func (h *Handler) SaveSchemaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		schemaType := chi.URLParam(r, "schemaType")
		if schemaType == "" {
			http.Error(w, "Schema type is required", http.StatusBadRequest)
			return
		}

		// Parse form data
		tableName := r.FormValue("table_name")
		enabled := r.FormValue("enabled") == "on" || r.FormValue("enabled") == "true"
		lastModifiedField := r.FormValue("last_modified_field")

		if tableName == "" {
			http.Error(w, "Table name is required", http.StatusBadRequest)
			return
		}

		// Parse field mappings from form
		// Form fields are named like: field_mapping[internal_field_name]=airtable_field_name
		fieldMappings := []platformVA.FieldMapping{}
		internalFields := getInternalFieldsForSchemaType(schemaType)

		// Get existing schema to preserve data types from Airtable metadata
		existingSchema, _ := h.vaSvc.GetAirtableSchema(r.Context(), activeVA.VAID, schemaType)
		existingFieldMap := make(map[string]platformVA.FieldMapping)
		if existingSchema != nil {
			for _, field := range existingSchema.Fields {
				existingFieldMap[field.InternalName] = field
			}
		}

		// Get credentials to fetch field metadata for data type inference
		creds, _ := h.vaSvc.GetAirtableCredentials(r.Context(), activeVA.VAID)
		var airtableFields []dtos.AirtableFieldMetadata
		if creds != nil && tableName != "" {
			fields, err := h.airtableProvider.FetchTableFields(r.Context(), creds, tableName)
			if err == nil {
				airtableFields = fields
			}
		}

		// Build field type map from Airtable metadata
		fieldTypeMap := make(map[string]string)
		for _, field := range airtableFields {
			fieldTypeMap[field.Name] = field.Type
		}

		for _, internalField := range internalFields {
			airtableFieldName := r.FormValue("field_mapping[" + internalField.Name + "]")
			if airtableFieldName != "" {
				// Infer data type from Airtable field type
				dataType := "string" // default
				if atType, ok := fieldTypeMap[airtableFieldName]; ok {
					dataType = mapAirtableTypeToInternalType(atType)
				} else if existingField, ok := existingFieldMap[internalField.Name]; ok {
					// Use existing data type if available
					dataType = existingField.DataType
				}

				fieldMapping := platformVA.FieldMapping{
					InternalName:  internalField.Name,
					AirtableName:  airtableFieldName,
					DataType:      dataType,
					Required:      internalField.Required,
					DisplayName:   internalField.DisplayName,
					IsUserVisible: internalField.IsUserVisible,
				}
				fieldMappings = append(fieldMappings, fieldMapping)
			}
		}

		schema := &platformVA.SchemaConfig{
			TableName:         tableName,
			Enabled:           enabled,
			LastModifiedField: lastModifiedField,
			Fields:            fieldMappings,
		}

		// Save via VA service
		if err := h.vaSvc.SaveAirtableSchema(r.Context(), activeVA.VAID, schemaType, schema); err != nil {
			logging.Error("Failed to save schema", "error", err)
			http.Error(w, "Failed to save schema: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Return success - reload status
		w.Header().Set("HX-Trigger", "schema-saved")
		http.Redirect(w, r, "/dashboard/settings/datasource/status", http.StatusSeeOther)
	}
}

// InternalFieldDefinition defines an internal field that can be mapped
type InternalFieldDefinition struct {
	Name          string
	DisplayName   string
	Required      bool
	IsUserVisible bool
	Description   string
}

// getInternalFieldsForSchemaType returns the list of internal fields for a schema type
func getInternalFieldsForSchemaType(schemaType string) []InternalFieldDefinition {
	switch schemaType {
	case "pilot":
		return []InternalFieldDefinition{
			{Name: "callsign", DisplayName: "Callsign", Required: true, IsUserVisible: true, Description: "Pilot callsign (mandatory)"},
			{Name: "discord_user_id", DisplayName: "Discord User ID", Required: false, IsUserVisible: false, Description: "Discord user ID for linking"},
			{Name: "if_community_id", DisplayName: "IF Community ID", Required: false, IsUserVisible: true, Description: "Infinite Flight Community ID"},
			{Name: "rank", DisplayName: "Rank", Required: false, IsUserVisible: true, Description: "Pilot rank"},
			{Name: "flight_hours", DisplayName: "Flight Hours", Required: false, IsUserVisible: true, Description: "Total flight hours"},
			{Name: "status", DisplayName: "Status", Required: false, IsUserVisible: true, Description: "Pilot status"},
			{Name: "last_flight_date", DisplayName: "Last Flight Date", Required: false, IsUserVisible: true, Description: "Date of last flight"},
			{Name: "join_date", DisplayName: "Join Date", Required: false, IsUserVisible: true, Description: "Date pilot joined"},
			{Name: "region", DisplayName: "Region", Required: false, IsUserVisible: true, Description: "Pilot region"},
		}
	case "route":
		return []InternalFieldDefinition{
			{Name: "route", DisplayName: "Route", Required: true, IsUserVisible: true, Description: "Route string (e.g., KJFK-EGLL)"},
			{Name: "origin", DisplayName: "Origin", Required: false, IsUserVisible: true, Description: "Origin ICAO code"},
			{Name: "destination", DisplayName: "Destination", Required: false, IsUserVisible: true, Description: "Destination ICAO code"},
			{Name: "distance", DisplayName: "Distance", Required: false, IsUserVisible: true, Description: "Route distance"},
			{Name: "duration", DisplayName: "Duration", Required: false, IsUserVisible: true, Description: "Estimated flight duration"},
		}
	case "pirep":
		return []InternalFieldDefinition{
			{Name: "callsign", DisplayName: "Callsign", Required: true, IsUserVisible: true, Description: "Pilot callsign (linked to pilot record)"},
			{Name: "ifc_username", DisplayName: "IF Community Username", Required: false, IsUserVisible: true, Description: "Infinite Flight Community username"},
			{Name: "route_at_id", DisplayName: "Route Airtable ID", Required: false, IsUserVisible: false, Description: "Reference to route record (linked field)"},
			{Name: "aircraft", DisplayName: "Aircraft", Required: false, IsUserVisible: true, Description: "Aircraft type"},
			{Name: "airline", DisplayName: "Airline", Required: false, IsUserVisible: true, Description: "Airline name"},
			{Name: "flight_mode", DisplayName: "Flight Mode", Required: false, IsUserVisible: true, Description: "Flight mode name (e.g., Passenger Flight, Cargo Flight)"},
			{Name: "flight_time", DisplayName: "Flight Time", Required: true, IsUserVisible: true, Description: "Flight duration in seconds (with multiplier applied)"},
			{Name: "date_completed", DisplayName: "Date Completed", Required: false, IsUserVisible: true, Description: "Date flight was completed"},
			{Name: "pilot_remarks", DisplayName: "Pilot Remarks", Required: false, IsUserVisible: true, Description: "Pilot notes and remarks (may include bot metadata)"},
			{Name: "fuel_kg", DisplayName: "Fuel (kg)", Required: false, IsUserVisible: true, Description: "Fuel consumed in kilograms"},
			{Name: "cargo_kg", DisplayName: "Cargo (kg)", Required: false, IsUserVisible: true, Description: "Cargo weight in kilograms"},
			{Name: "passengers", DisplayName: "Passengers", Required: false, IsUserVisible: true, Description: "Number of passengers"},
		}
	case "career_mode":
		return []InternalFieldDefinition{
			{Name: "callsign", DisplayName: "Callsign", Required: true, IsUserVisible: true, Description: "Pilot callsign"},
			{Name: "total_cm_hours", DisplayName: "Total Career Mode Hours", Required: false, IsUserVisible: true, Description: "Total hours in career mode"},
			{Name: "required_hours_to_next", DisplayName: "Hours to Next Rank", Required: false, IsUserVisible: true, Description: "Hours needed for next rank"},
			{Name: "last_activity_cm", DisplayName: "Last Career Mode Activity", Required: false, IsUserVisible: true, Description: "Last activity date"},
			{Name: "assigned_routes", DisplayName: "Assigned Routes", Required: false, IsUserVisible: true, Description: "Assigned route IDs"},
			{Name: "aircraft", DisplayName: "Aircraft", Required: false, IsUserVisible: true, Description: "Preferred aircraft"},
		}
	default:
		return []InternalFieldDefinition{}
	}
}

// mapAirtableTypeToInternalType maps Airtable field types to internal data types
func mapAirtableTypeToInternalType(airtableType string) string {
	switch airtableType {
	case "number":
		return "float"
	case "date":
		return "date"
	case "dateTime":
		return "datetime"
	case "checkbox":
		return "boolean"
	case "singleSelect", "multipleSelects":
		return "string"
	case "url", "email", "phoneNumber":
		return "string"
	default:
		return "string"
	}
}

// Helper function to parse int from form value with default
func parseIntOrDefault(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return defaultValue
	}
	return result
}
