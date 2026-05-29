package va

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/users"

	"github.com/go-chi/chi/v5"
)

// Handler consolidates VA management API handlers
type Handler struct {
	svc          *Service
	configSvc    *ConfigService
	userRepo     *users.Repository
	legacyVARepo *repositories.VAGormRepository // Temporary: for legacy FlightModesConfigService
}

// NewHandler creates a new VA Handler instance
func NewHandler(
	svc *Service,
	configSvc *ConfigService,
	userRepo *users.Repository,
	legacyVARepo *repositories.VAGormRepository,
) *Handler {
	return &Handler{
		svc:          svc,
		configSvc:    configSvc,
		userRepo:     userRepo,
		legacyVARepo: legacyVARepo,
	}
}

// SyncUser and SetRole handlers have been moved to internal/platform/memberships/handler.go

// GetConfigs handles GET /api/v1/va/configs
// Returns all configuration values for the current VA
func (h *Handler) GetConfigs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		configs, _ := h.configSvc.GetAllConfigValues(r.Context(), claims.ServerID())

		common.RespondSuccess(w, initTime, "VA configuration fetched", configs)
	}
}

// ListConfigKeys handles GET /api/v1/va/configs/keys
// Returns list of all possible configuration keys
func (h *Handler) ListConfigKeys() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		keys := h.configSvc.ListPossibleKeys()

		common.RespondSuccess(w, initTime, "Configuration keys listed", dtos.VAConfigKeys{
			ConfigKeys: keys,
		})
	}
}

// SetConfigs handles POST /api/v1/va/configs
// Sets configuration values for the current VA
func (h *Handler) SetConfigs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		configs := make(map[string]string)
		if err := json.NewDecoder(r.Body).Decode(&configs); err != nil {
			common.RespondError(w, initTime, err, "Invalid JSON", http.StatusBadRequest)
			return
		}

		result, err := h.configSvc.SetVaConfig(r.Context(), configs)
		if err != nil {
			common.RespondError(w, initTime, err, err.Error(), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Config set successfully", result)
	}
}

// SetFlightModesConfig handles POST /api/v1/va/flight-modes/config
// Stores or updates flight mode configuration for a VA (admin-only)
func (h *Handler) SetFlightModesConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		// Get claims from context
		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaDiscordServerID := claims.DiscordServerID()

		// Validate VA exists
		if vaDiscordServerID == "" {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Get VA by Discord Server ID
		vaGorm, err := h.svc.GetByDiscordServerID(r.Context(), vaDiscordServerID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to fetch VA configuration", http.StatusInternalServerError)
			return
		}

		if vaGorm == nil {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "Virtual airline not found", http.StatusNotFound)
			return
		}

		// Parse request body
		var configPayload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&configPayload); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

	// Validate + save strict v2 config via platform VA service
	if err := h.svc.ValidateAndSaveFlightModesConfig(r.Context(), vaGorm.ID, configPayload); err != nil {
		common.RespondError(w, initTime, err, "Invalid configuration", http.StatusBadRequest)
		return
	}

		// Get the number of modes for response
		flightModes := configPayload["flight_modes"].(map[string]interface{})

		response := map[string]interface{}{
			"success": true,
			"message": "Flight modes configuration saved successfully",
			"va_id":   vaGorm.ID,
			"modes":   len(flightModes),
		}

		common.RespondSuccess(w, initTime, "Flight modes configuration saved successfully", response)
	}
}

// ============================================================================
// Airtable Data Provider API Handlers
// ============================================================================

// SaveAirtableCredentialsHandler handles POST /api/v1/admin/airtable/credentials
// Saves or updates Airtable credentials for the current VA
func (h *Handler) SaveAirtableCredentialsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		if vaID == "" {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "VA ID not found", http.StatusBadRequest)
			return
		}

		var creds ProviderCredentials
		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate
		if creds.APIKey == "" {
			common.RespondError(w, initTime, fmt.Errorf("api_key required"), "API key is required", http.StatusBadRequest)
			return
		}
		if creds.BaseID == "" {
			common.RespondError(w, initTime, fmt.Errorf("base_id required"), "Base ID is required", http.StatusBadRequest)
			return
		}

		// Save via service
		if err := h.svc.SaveAirtableCredentials(r.Context(), vaID, &creds); err != nil {
			common.RespondError(w, initTime, err, "Failed to save credentials: "+err.Error(), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Credentials saved successfully", map[string]string{
			"va_id": vaID,
		})
	}
}

// SaveAirtableSchemaHandler handles POST /api/v1/admin/airtable/schema/{schemaType}
// Saves or updates a specific schema configuration
func (h *Handler) SaveAirtableSchemaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		if vaID == "" {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "VA ID not found", http.StatusBadRequest)
			return
		}

		// Get schemaType from URL path (chi router)
		schemaType := chi.URLParam(r, "schemaType")
		// Fallback to query param
		if schemaType == "" {
			schemaType = r.URL.Query().Get("schemaType")
		}
		if schemaType == "" {
			common.RespondError(w, initTime, fmt.Errorf("schema_type required"), "Schema type is required", http.StatusBadRequest)
			return
		}

		var schema SchemaConfig
		if err := json.NewDecoder(r.Body).Decode(&schema); err != nil {
			common.RespondError(w, initTime, err, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate
		if schema.TableName == "" {
			common.RespondError(w, initTime, fmt.Errorf("table_name required"), "Table name is required", http.StatusBadRequest)
			return
		}

		// Save via service
		if err := h.svc.SaveAirtableSchema(r.Context(), vaID, schemaType, &schema); err != nil {
			common.RespondError(w, initTime, err, "Failed to save schema: "+err.Error(), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Schema saved successfully", map[string]interface{}{
			"va_id":       vaID,
			"schema_type": schemaType,
		})
	}
}

// GetAirtableSchemaHandler handles GET /api/v1/admin/airtable/schema/{schemaType}
// Returns a specific schema configuration
func (h *Handler) GetAirtableSchemaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		if vaID == "" {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "VA ID not found", http.StatusBadRequest)
			return
		}

		// Get schemaType from URL path (chi router)
		schemaType := chi.URLParam(r, "schemaType")
		// Fallback to query param
		if schemaType == "" {
			schemaType = r.URL.Query().Get("schemaType")
		}
		if schemaType == "" {
			common.RespondError(w, initTime, fmt.Errorf("schema_type required"), "Schema type is required", http.StatusBadRequest)
			return
		}

		schema, err := h.svc.GetAirtableSchema(r.Context(), vaID, schemaType)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to get schema: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if schema == nil {
			common.RespondError(w, initTime, nil, "Schema not found", http.StatusNotFound)
			return
		}

		common.RespondSuccess(w, initTime, "Schema retrieved successfully", schema)
	}
}

// GetAirtableSchemasHandler handles GET /api/v1/admin/airtable/schemas
// Returns all schema configurations for the current VA
func (h *Handler) GetAirtableSchemasHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initTime := time.Now()

		claims := auth.GetUserClaims(r.Context())
		if claims == nil {
			common.RespondError(w, initTime, nil, "Unauthorized: missing claims", http.StatusUnauthorized)
			return
		}

		vaID := claims.ServerID()
		if vaID == "" {
			common.RespondError(w, initTime, fmt.Errorf("va not found"), "VA ID not found", http.StatusBadRequest)
			return
		}

		schemas, err := h.svc.GetAirtableSchemas(r.Context(), vaID)
		if err != nil {
			common.RespondError(w, initTime, err, "Failed to get schemas: "+err.Error(), http.StatusInternalServerError)
			return
		}

		common.RespondSuccess(w, initTime, "Schemas retrieved successfully", schemas)
	}
}
