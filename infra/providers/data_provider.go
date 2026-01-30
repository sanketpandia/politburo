package providers

import (
	"context"
	"infinite-experiment/politburo/internal/models/dtos"
)

// DataProvider defines the interface for external data sources
type DataProvider interface {
	// FetchPilotRecord fetches a single pilot record by their provider-specific ID
	FetchPilotRecord(ctx context.Context, pilotID string, schema *dtos.EntitySchema) (*PilotRecord, error)

	// FetchRecords fetches multiple records with pagination support
	FetchRecords(ctx context.Context, schema *dtos.EntitySchema, filters *SyncFilters) (*RecordSet, error)

	// SubmitRecord creates a new record in the data source
	SubmitRecord(ctx context.Context, schema *dtos.EntitySchema, fields map[string]interface{}) (string, error)

	// ValidateConfig validates that the configuration is valid and can connect
	ValidateConfig(ctx context.Context, config *dtos.ProviderConfigData) (*ValidationResult, error)

	// GetProviderType returns the provider type identifier
	GetProviderType() string
}

// PilotRecord represents a pilot's data fetched from the provider
type PilotRecord struct {
	ProviderID string                 // The record ID from the provider (e.g., Airtable record ID)
	RawFields  map[string]interface{} // Raw fields as returned by provider
	Normalized map[string]interface{} // Normalized fields mapped to internal names
}

// RecordSet represents a paginated set of records
type RecordSet struct {
	Records      []RecordWithID           // Array of records with their provider IDs
	Offset       string                   // Pagination offset/cursor
	HasMore      bool                     // Whether more records exist
	TotalFetched int                      // Number of records in this batch
}

// RecordWithID represents a record with its provider-specific ID
type RecordWithID struct {
	ID          string                 // Provider-specific record ID (e.g., Airtable rec...)
	Fields      map[string]interface{} // Record fields
	CreatedTime string                 // Record creation time (ISO 8601 format)
}

// SyncFilters defines filters for fetching records
type SyncFilters struct {
	ModifiedSince *string // ISO 8601 timestamp - fetch only records modified after this
	Offset        string  // Pagination offset
	Limit         int     // Max records to fetch
	FilterFormula string  // Custom filter formula (e.g., Airtable formula for field matching)
}

// ValidationResult contains the results of config validation
type ValidationResult struct {
	IsValid         bool                `json:"is_valid"`
	PhasesCompleted []string            `json:"phases_completed"`
	PhasesFailed    []string            `json:"phases_failed"`
	Errors          []dtos.ValidationError `json:"errors"`
	Warnings        []dtos.ValidationError `json:"warnings,omitempty"`
	DurationMs      int                 `json:"duration_ms"`
}
