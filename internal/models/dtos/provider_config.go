package dtos

// ProviderConfigData represents the full JSONB structure stored in config_data
type ProviderConfigData struct {
	Version      string           `json:"version"`
	Provider     string           `json:"provider"`
	Credentials  ProviderCreds    `json:"credentials"`
	Schemas      []EntitySchema   `json:"schemas"`
	SyncSettings SyncSettings     `json:"sync_settings"`
}

// ProviderCreds stores authentication credentials
type ProviderCreds struct {
	APIKey string `json:"api_key"`
	BaseID string `json:"base_id"`
}

// EntitySchema defines how to sync a specific entity type (pilot, route, etc.)
type EntitySchema struct {
	EntityType        string          `json:"entity_type"` // "pilot", "route", "pirep", custom types
	TableName         string          `json:"table_name"`  // Airtable table name
	Enabled           bool            `json:"enabled"`
	Fields            []FieldMapping  `json:"fields"`
	LastModifiedField string          `json:"last_modified_field,omitempty"`
	
	// Career mode specific configuration
	CareerModeFlightMode *string `json:"career_mode_flight_mode,omitempty"` // Flight mode to filter PIREPs for last flown route
}

// FieldMapping maps an internal field to an external provider field
type FieldMapping struct {
	InternalName string  `json:"internal_name"` // What our system calls it
	AirtableName string  `json:"airtable_name"` // What the VA's table calls it
	DataType     string  `json:"data_type"`     // "string", "int", "float", "date", "boolean"
	Required     bool    `json:"required"`
	DefaultValue *string `json:"default_value,omitempty"`

	// Display/presentation layer for user-facing APIs
	DisplayName   string  `json:"display_name,omitempty"`    // Standardized field name: "flight_hours", "rank", etc.
	IsUserVisible bool    `json:"is_user_visible"`           // Show in user-facing APIs like /api/v1/pilot/stats
	DisplayFormat *string `json:"display_format,omitempty"`  // Optional formatting hint: "duration", "date", etc.

	// Bot metadata enrichment - only ONE field should have this set to true
	// When true, bot-generated metadata (flight stats, aircraft, livery, etc.) will be appended to this field
	BotMetadata bool `json:"bot_metadata"`
}

// SyncSettings defines sync behavior preferences
type SyncSettings struct {
	BatchSize           int `json:"batch_size"`
	RateLimitPerSecond  int `json:"rate_limit_per_second"`
	RetryAttempts       int `json:"retry_attempts"`
	TimeoutSeconds      int `json:"timeout_seconds"`
}

// ValidationError represents a validation error in JSONB format
type ValidationError struct {
	Phase       string                 `json:"phase"`        // Which validation phase failed
	EntityType  string                 `json:"entity_type,omitempty"`
	TableName   string                 `json:"table_name,omitempty"`
	Error       string                 `json:"error"`        // Human-readable error message
	ErrorCode   string                 `json:"error_code"`   // Machine-readable code
	Details     map[string]interface{} `json:"details,omitempty"` // Additional context
	Timestamp   string                 `json:"timestamp"`
}

// GetSchemaByType returns the schema for a specific entity type
func (c *ProviderConfigData) GetSchemaByType(entityType string) *EntitySchema {
	for i := range c.Schemas {
		if c.Schemas[i].EntityType == entityType {
			return &c.Schemas[i]
		}
	}
	return nil
}

// HasField checks if a schema has a specific internal field name
func (s *EntitySchema) HasField(internalName string) bool {
	for _, field := range s.Fields {
		if field.InternalName == internalName {
			return true
		}
	}
	return false
}

// GetFieldMapping returns the field mapping for an internal name
func (s *EntitySchema) GetFieldMapping(internalName string) *FieldMapping {
	for i := range s.Fields {
		if s.Fields[i].InternalName == internalName {
			return &s.Fields[i]
		}
	}
	return nil
}

// GetAirtableFieldNames returns all Airtable field names for fetching
func (s *EntitySchema) GetAirtableFieldNames() []string {
	names := make([]string, len(s.Fields))
	for i, field := range s.Fields {
		names[i] = field.AirtableName
	}
	return names
}

// SaveProviderConfigRequest is the request body for saving/updating a provider config
type SaveProviderConfigRequest struct {
	ProviderType string             `json:"provider_type"` // "airtable", "google_sheets", etc.
	ConfigData   ProviderConfigData `json:"config_data"`
	IsActive     bool               `json:"is_active"` // Whether to activate immediately
}
