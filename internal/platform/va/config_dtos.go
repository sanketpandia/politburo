package va

import (
	"encoding/json"
	"fmt"
)

// ProviderCredentials - credentials and sync settings only
type ProviderCredentials struct {
	APIKey       string       `json:"api_key"`
	BaseID       string       `json:"base_id"`
	SyncSettings SyncSettings `json:"sync_settings"`
}

// SchemaConfig - single schema configuration
type SchemaConfig struct {
	TableName         string         `json:"table_name"`
	Enabled           bool           `json:"enabled"`
	LastModifiedField string         `json:"last_modified_field,omitempty"`
	Fields            []FieldMapping `json:"fields"`

	// Career mode specific configuration
	CareerModeFlightMode *string `json:"career_mode_flight_mode,omitempty"` // Flight mode to filter PIREPs for last flown route
}

// EntitySchema defines how to sync a specific entity type
type EntitySchema struct {
	EntityType        string         `json:"entity_type"` // "pilot", "route", "pirep"
	TableName         string         `json:"table_name"`  // Airtable table name
	Enabled           bool           `json:"enabled"`
	Fields            []FieldMapping `json:"fields"`
	LastModifiedField string         `json:"last_modified_field,omitempty"`

	// Career mode specific configuration
	CareerModeFlightMode *string `json:"career_mode_flight_mode,omitempty"` // Flight mode to filter PIREPs for last flown route
}

// FieldMapping maps an internal field to an external provider field
type FieldMapping struct {
	InternalName  string  `json:"internal_name"`
	AirtableName  string  `json:"airtable_name"`
	DataType      string  `json:"data_type"`
	Required      bool    `json:"required"`
	DefaultValue  *string `json:"default_value,omitempty"`
	DisplayName   string  `json:"display_name,omitempty"`
	IsUserVisible bool    `json:"is_user_visible"`
	DisplayFormat *string `json:"display_format,omitempty"`
	BotMetadata   bool    `json:"bot_metadata"`
}

// SyncSettings defines sync behavior preferences
type SyncSettings struct {
	BatchSize          int `json:"batch_size"`
	RateLimitPerSecond int `json:"rate_limit_per_second"`
	RetryAttempts      int `json:"retry_attempts"`
	TimeoutSeconds     int `json:"timeout_seconds"`
}

// FeaturePilotStatsConfig defines bounded pilot-stats card configuration.
type FeaturePilotStatsConfig struct {
	Enabled bool                    `json:"enabled"`
	Cards   []PilotStatsFeatureCard `json:"cards,omitempty"`
}

type PilotStatsFeatureCard struct {
	CardID          string   `json:"card_id"`
	Mode            string   `json:"mode"` // direct_field|latest_row|recent_flights|bounded_aggregate
	SourceSchema    string   `json:"source_schema,omitempty"`
	SelectedFields  []string `json:"selected_fields,omitempty"`
	DisplayFormat   string   `json:"display_format,omitempty"`
	AggregationMode string   `json:"aggregation_mode,omitempty"`
	Enabled         bool     `json:"enabled"`
}

// Helper methods for EntitySchema

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

// Helper functions for parsing/marshaling

// ParseCredentialsConfig parses credentials from JSONB
func ParseCredentialsConfig(jsonb JSONB) (*ProviderCredentials, error) {
	if jsonb == nil {
		return nil, fmt.Errorf("jsonb is nil")
	}

	bytes, err := json.Marshal(jsonb)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jsonb: %w", err)
	}

	var creds ProviderCredentials
	if err := json.Unmarshal(bytes, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials config: %w", err)
	}

	return &creds, nil
}

// ParseSchemaConfig parses schema from JSONB
func ParseSchemaConfig(jsonb JSONB) (*SchemaConfig, error) {
	if jsonb == nil {
		return nil, fmt.Errorf("jsonb is nil")
	}

	bytes, err := json.Marshal(jsonb)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jsonb: %w", err)
	}

	var schema SchemaConfig
	if err := json.Unmarshal(bytes, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema config: %w", err)
	}

	return &schema, nil
}

// ParseFeaturePilotStatsConfig parses feature_pilot_stats config from JSONB.
func ParseFeaturePilotStatsConfig(jsonb JSONB) (*FeaturePilotStatsConfig, error) {
	if jsonb == nil {
		return nil, fmt.Errorf("jsonb is nil")
	}

	bytes, err := json.Marshal(jsonb)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jsonb: %w", err)
	}

	var cfg FeaturePilotStatsConfig
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse feature pilot stats config: %w", err)
	}

	return &cfg, nil
}

// MarshalCredentialsConfig marshals credentials to JSONB
func MarshalCredentialsConfig(creds *ProviderCredentials) (JSONB, error) {
	if creds == nil {
		return nil, fmt.Errorf("credentials is nil")
	}

	bytes, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	var jsonb JSONB
	if err := json.Unmarshal(bytes, &jsonb); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to jsonb: %w", err)
	}

	return jsonb, nil
}

// MarshalSchemaConfig marshals schema to JSONB
func MarshalSchemaConfig(schema *SchemaConfig) (JSONB, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}

	bytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	var jsonb JSONB
	if err := json.Unmarshal(bytes, &jsonb); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to jsonb: %w", err)
	}

	return jsonb, nil
}

// ToEntitySchema converts SchemaConfig to EntitySchema
// This is useful when you need EntitySchema for provider methods
func (s *SchemaConfig) ToEntitySchema(entityType string) *EntitySchema {
	entitySchema := &EntitySchema{
		EntityType:        entityType,
		TableName:         s.TableName,
		Enabled:           s.Enabled,
		LastModifiedField: s.LastModifiedField,
		Fields:            s.Fields,
	}
	// Copy career mode flight mode if this is a career mode schema
	if entityType == "career_mode" {
		entitySchema.CareerModeFlightMode = s.CareerModeFlightMode
	}
	return entitySchema
}
