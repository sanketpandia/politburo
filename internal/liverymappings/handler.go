package liverymappings

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/infra/templates"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"infinite-experiment/politburo/internal/platform/httpdto"
	platformVA "infinite-experiment/politburo/internal/platform/va"

	"github.com/go-chi/chi/v5"
)

// Handler handles livery mapping UI and API endpoints
type Handler struct {
	aircraftRepo     *aircraft.Repository
	vaConfigSvc      *platformVA.ConfigService
	templateRenderer *templates.Renderer
}

// NewHandler creates a new livery mappings handler
func NewHandler(aircraftRepo *aircraft.Repository, vaConfigSvc *platformVA.ConfigService, templateRenderer *templates.Renderer) *Handler {
	return &Handler{
		aircraftRepo:     aircraftRepo,
		vaConfigSvc:      vaConfigSvc,
		templateRenderer: templateRenderer,
	}
}

// ListMappingsPageHandler handles GET /dashboard/settings/livery-mappings
// Renders the livery mappings management page
func (h *Handler) ListMappingsPageHandler() http.HandlerFunc {
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

		activeVA := sessionData.GetActiveVA()
		if activeVA == nil {
			http.Error(w, "No active VA found", http.StatusInternalServerError)
			return
		}

		// Prepare template data
		data, err := templates.PrepareTemplateData(sessionData, "Livery Mappings")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data["CurrentPage"] = "datasource"

		// Render template
		if err := h.templateRenderer.RenderTemplate(w, "pages/livery-mappings.html", data); err != nil {
			logging.Error("Failed to render livery mappings page", "error", err)
			http.Error(w, "Failed to render page", http.StatusInternalServerError)
			return
		}
	}
}

// ListMappingsHandler handles GET /api/v1/admin/livery-mappings
// Returns all livery mappings for the active VA
func (h *Handler) ListMappingsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get VA ID from claims
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, start, "unauthorized", "Missing authentication", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		if vaID == "" {
			httpdto.WriteError(w, start, "invalid_request", "VA ID not found", http.StatusBadRequest)
			return
		}

		// Fetch all mappings for this VA
		mappings, err := h.aircraftRepo.GetMappingsByVA(r.Context(), vaID)
		if err != nil {
			logging.Error("Failed to fetch livery mappings", "error", err, "vaID", vaID)
			httpdto.WriteError(w, start, "internal_error", "Failed to fetch mappings", http.StatusInternalServerError)
			return
		}

		// Fetch all active liveries for reference
		liveries, err := h.aircraftRepo.GetAllActive(r.Context())
		if err != nil {
			logging.Error("Failed to fetch liveries", "error", err)
			httpdto.WriteError(w, start, "internal_error", "Failed to fetch liveries", http.StatusInternalServerError)
			return
		}

		// Create a map of livery_id -> AircraftLivery for quick lookup
		liveryMap := make(map[string]aircraft.AircraftLivery)
		for _, livery := range liveries {
			liveryMap[livery.LiveryID] = livery
		}

		// Build response with enriched data
		type MappingResponse struct {
			ID           string `json:"id"`
			LiveryID     string `json:"liveryId"`
			FieldType    string `json:"fieldType"`
			SourceValue  string `json:"sourceValue"`
			TargetValue  string `json:"targetValue"`
			IsActive     bool   `json:"isActive"`
			CreatedAt    string `json:"createdAt"`
			UpdatedAt    string `json:"updatedAt"`
			AircraftName string `json:"aircraftName,omitempty"`
			LiveryName   string `json:"liveryName,omitempty"`
		}

		response := make([]MappingResponse, 0, len(mappings))
		for _, mapping := range mappings {
			mr := MappingResponse{
				ID:          mapping.ID,
				LiveryID:    mapping.LiveryID,
				FieldType:   mapping.FieldType,
				SourceValue: mapping.SourceValue,
				TargetValue: mapping.TargetValue,
				IsActive:    mapping.IsActive,
				CreatedAt:   mapping.CreatedAt.Format(time.RFC3339),
				UpdatedAt:   mapping.UpdatedAt.Format(time.RFC3339),
			}

			// Enrich with livery info if available
			if livery, ok := liveryMap[mapping.LiveryID]; ok {
				mr.AircraftName = livery.AircraftName
				mr.LiveryName = livery.LiveryName
			}

			response = append(response, mr)
		}

		httpdto.WriteSuccess(w, start, response, http.StatusOK)
	}
}

// CreateMappingHandler handles POST /api/v1/admin/livery-mappings
// Creates livery mappings for all liveries matching the source value
func (h *Handler) CreateMappingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get VA ID from claims
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, start, "unauthorized", "Missing authentication", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		if vaID == "" {
			httpdto.WriteError(w, start, "invalid_request", "VA ID not found", http.StatusBadRequest)
			return
		}

		// Parse request body
		var req struct {
			FieldType   string `json:"fieldType"`
			SourceValue string `json:"sourceValue"`
			TargetValue string `json:"targetValue"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpdto.WriteError(w, start, "invalid_request", "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if req.FieldType == "" || req.SourceValue == "" || req.TargetValue == "" {
			httpdto.WriteError(w, start, "invalid_request", "Missing required fields", http.StatusBadRequest)
			return
		}

		// Validate field type
		if req.FieldType != "aircraft" && req.FieldType != "airline" {
			httpdto.WriteError(w, start, "invalid_request", "Field type must be 'aircraft' or 'airline'", http.StatusBadRequest)
			return
		}

		// Find all liveries matching the source value based on field type
		var matchingLiveries []aircraft.AircraftLivery
		var err error

		if req.FieldType == "aircraft" {
			matchingLiveries, err = h.aircraftRepo.GetLiveriesByAircraftName(r.Context(), req.SourceValue)
		} else {
			matchingLiveries, err = h.aircraftRepo.GetLiveriesByLiveryName(r.Context(), req.SourceValue)
		}

		if err != nil {
			logging.Error("Failed to find matching liveries", "error", err, "fieldType", req.FieldType, "sourceValue", req.SourceValue)
			httpdto.WriteError(w, start, "internal_error", "Failed to find matching liveries", http.StatusInternalServerError)
			return
		}

		if len(matchingLiveries) == 0 {
			httpdto.WriteError(w, start, "not_found", "No liveries found matching the source value", http.StatusNotFound)
			return
		}

		// Create mappings for all matching liveries
		mappings := make([]aircraft.LiveryAirtableMapping, 0, len(matchingLiveries))
		for _, livery := range matchingLiveries {
			mapping := aircraft.LiveryAirtableMapping{
				VAID:        vaID,
				LiveryID:    livery.LiveryID,
				FieldType:   req.FieldType,
				SourceValue: req.SourceValue,
				TargetValue: req.TargetValue,
				IsActive:    true,
			}
			mappings = append(mappings, mapping)
		}

		// Upsert all mappings (handles conflicts)
		if err := h.aircraftRepo.UpsertMappings(r.Context(), mappings); err != nil {
			logging.Error("Failed to create livery mappings", "error", err, "vaID", vaID, "fieldType", req.FieldType, "sourceValue", req.SourceValue, "count", len(mappings))
			httpdto.WriteError(w, start, "internal_error", "Failed to create mappings", http.StatusInternalServerError)
			return
		}

		// Build response with summary
		type MappingResponse struct {
			Count       int    `json:"count"`
			FieldType   string `json:"fieldType"`
			SourceValue string `json:"sourceValue"`
			TargetValue string `json:"targetValue"`
			Message     string `json:"message"`
		}

		response := MappingResponse{
			Count:       len(mappings),
			FieldType:   req.FieldType,
			SourceValue: req.SourceValue,
			TargetValue: req.TargetValue,
			Message:     fmt.Sprintf("Created %d mapping(s) successfully", len(mappings)),
		}

		httpdto.WriteSuccess(w, start, response, http.StatusCreated)
	}
}

// DeleteMappingHandler handles DELETE /api/v1/admin/livery-mappings/{id}
// Soft deletes a livery mapping by setting is_active to false
func (h *Handler) DeleteMappingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get VA ID from claims
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, start, "unauthorized", "Missing authentication", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		if vaID == "" {
			httpdto.WriteError(w, start, "invalid_request", "VA ID not found", http.StatusBadRequest)
			return
		}

		// Get mapping ID from URL path
		mappingID := chi.URLParam(r, "id")
		if mappingID == "" {
			httpdto.WriteError(w, start, "invalid_request", "Mapping ID required", http.StatusBadRequest)
			return
		}

		// Fetch mapping to get livery_id
		// We need to get all mappings and find the one with matching ID
		mappings, err := h.aircraftRepo.GetMappingsByVA(r.Context(), vaID)
		if err != nil {
			httpdto.WriteError(w, start, "internal_error", "Failed to fetch mappings", http.StatusInternalServerError)
			return
		}

		var targetMapping *aircraft.LiveryAirtableMapping
		for i := range mappings {
			if mappings[i].ID == mappingID {
				targetMapping = &mappings[i]
				break
			}
		}

		if targetMapping == nil {
			httpdto.WriteError(w, start, "not_found", "Mapping not found", http.StatusNotFound)
			return
		}

		// Soft delete by setting is_active to false
		// We'll update the mapping to set is_active = false
		targetMapping.IsActive = false
		if err := h.aircraftRepo.UpsertMappings(r.Context(), []aircraft.LiveryAirtableMapping{*targetMapping}); err != nil {
			logging.Error("Failed to delete livery mapping", "error", err, "mappingID", mappingID)
			httpdto.WriteError(w, start, "internal_error", "Failed to delete mapping", http.StatusInternalServerError)
			return
		}

		httpdto.WriteSuccess(w, start, map[string]string{"message": "Mapping deleted successfully"}, http.StatusOK)
	}
}

// GetAvailableLiveriesHandler handles GET /api/v1/admin/livery-mappings/liveries
// Returns all available liveries for selection
func (h *Handler) GetAvailableLiveriesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Fetch all active liveries
		liveries, err := h.aircraftRepo.GetAllActive(r.Context())
		if err != nil {
			logging.Error("Failed to fetch liveries", "error", err)
			httpdto.WriteError(w, start, "internal_error", "Failed to fetch liveries", http.StatusInternalServerError)
			return
		}

		type LiveryResponse struct {
			LiveryID     string `json:"liveryId"`
			AircraftID   string `json:"aircraftId"`
			AircraftName string `json:"aircraftName"`
			LiveryName   string `json:"liveryName"`
		}

		response := make([]LiveryResponse, 0, len(liveries))
		for _, livery := range liveries {
			response = append(response, LiveryResponse{
				LiveryID:     livery.LiveryID,
				AircraftID:   livery.AircraftID,
				AircraftName: livery.AircraftName,
				LiveryName:   livery.LiveryName,
			})
		}

		httpdto.WriteSuccess(w, start, response, http.StatusOK)
	}
}

// GetUniqueAircraftHandler handles GET /api/v1/admin/livery-mappings/unique-aircraft
// Returns distinct aircraft names from active liveries
func (h *Handler) GetUniqueAircraftHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Fetch unique aircraft names
		aircraftNames, err := h.aircraftRepo.GetUniqueAircraftNames(r.Context())
		if err != nil {
			logging.Error("Failed to fetch unique aircraft names", "error", err)
			httpdto.WriteError(w, start, "internal_error", "Failed to fetch unique aircraft names", http.StatusInternalServerError)
			return
		}

		httpdto.WriteSuccess(w, start, aircraftNames, http.StatusOK)
	}
}

// GetUniqueLiveriesHandler handles GET /api/v1/admin/livery-mappings/unique-liveries
// Returns distinct livery names (airlines) from active liveries
func (h *Handler) GetUniqueLiveriesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Fetch unique livery names
		liveryNames, err := h.aircraftRepo.GetUniqueLiveryNames(r.Context())
		if err != nil {
			logging.Error("Failed to fetch unique livery names", "error", err)
			httpdto.WriteError(w, start, "internal_error", "Failed to fetch unique livery names", http.StatusInternalServerError)
			return
		}

		httpdto.WriteSuccess(w, start, liveryNames, http.StatusOK)
	}
}

// GetDefaultsHandler handles GET /api/v1/admin/livery-mappings/defaults
// Returns the default aircraft and airline values for the active VA
func (h *Handler) GetDefaultsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get VA ID from claims
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, start, "unauthorized", "Missing authentication", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		if vaID == "" {
			httpdto.WriteError(w, start, "invalid_request", "VA ID not found", http.StatusBadRequest)
			return
		}

		// Get default values
		defaultAircraft, _ := h.vaConfigSvc.GetConfigVal(r.Context(), vaID, platformVA.ConfigKeyDefaultAircraft)
		defaultAirline, _ := h.vaConfigSvc.GetConfigVal(r.Context(), vaID, platformVA.ConfigKeyDefaultAirline)

		response := map[string]string{
			"defaultAircraft": defaultAircraft,
			"defaultAirline":  defaultAirline,
		}

		httpdto.WriteSuccess(w, start, response, http.StatusOK)
	}
}

// SetDefaultsHandler handles POST /api/v1/admin/livery-mappings/defaults
// Sets the default aircraft and airline values for the active VA
func (h *Handler) SetDefaultsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get VA ID from claims
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			httpdto.WriteError(w, start, "unauthorized", "Missing authentication", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		if vaID == "" {
			httpdto.WriteError(w, start, "invalid_request", "VA ID not found", http.StatusBadRequest)
			return
		}

		// Parse request body
		var req struct {
			DefaultAircraft string `json:"defaultAircraft"`
			DefaultAirline  string `json:"defaultAirline"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpdto.WriteError(w, start, "invalid_request", "Invalid request body", http.StatusBadRequest)
			return
		}

		// Set default values using SetVaConfig
		configs := make(map[string]string)
		if req.DefaultAircraft != "" {
			configs[platformVA.ConfigKeyDefaultAircraft] = req.DefaultAircraft
		}
		if req.DefaultAirline != "" {
			configs[platformVA.ConfigKeyDefaultAirline] = req.DefaultAirline
		}

		// If both are empty, we can still update (to clear them)
		if len(configs) == 0 {
			// Allow clearing defaults by sending empty strings
			configs[platformVA.ConfigKeyDefaultAircraft] = ""
			configs[platformVA.ConfigKeyDefaultAirline] = ""
		}

		_, err := h.vaConfigSvc.SetVaConfig(r.Context(), configs)
		if err != nil {
			logging.Error("Failed to set default values", "error", err, "vaID", vaID)
			httpdto.WriteError(w, start, "internal_error", "Failed to set default values", http.StatusInternalServerError)
			return
		}

		// Return updated values
		defaultAircraft, _ := h.vaConfigSvc.GetConfigVal(r.Context(), vaID, platformVA.ConfigKeyDefaultAircraft)
		defaultAirline, _ := h.vaConfigSvc.GetConfigVal(r.Context(), vaID, platformVA.ConfigKeyDefaultAirline)

		response := map[string]string{
			"defaultAircraft": defaultAircraft,
			"defaultAirline":  defaultAirline,
		}

		httpdto.WriteSuccess(w, start, response, http.StatusOK)
	}
}
