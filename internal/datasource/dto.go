package datasource

import (
	"fmt"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

// DatasourceType represents the type of datasource
type DatasourceType string

const (
	DatasourceTypeAirtable DatasourceType = "airtable"
	// Future: DatasourceTypeGoogleSheets = "google_sheets"
)

// DatasourceConfig is the base interface for all datasource configurations
type DatasourceConfig interface {
	GetType() DatasourceType
	Validate() error
}

// AirtableConfig implements DatasourceConfig for Airtable
type AirtableConfig struct {
	Credentials *platformVA.ProviderCredentials
	Schemas     map[string]*platformVA.SchemaConfig
}

// GetType returns the datasource type
func (a *AirtableConfig) GetType() DatasourceType {
	return DatasourceTypeAirtable
}

// Validate validates the Airtable configuration
func (a *AirtableConfig) Validate() error {
	if a.Credentials == nil {
		return fmt.Errorf("credentials are required")
	}
	if a.Credentials.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	if a.Credentials.BaseID == "" {
		return fmt.Errorf("base ID is required")
	}
	return nil
}

// NewAirtableConfig creates a new Airtable configuration
func NewAirtableConfig(creds *platformVA.ProviderCredentials, schemas map[string]*platformVA.SchemaConfig) *AirtableConfig {
	return &AirtableConfig{
		Credentials: creds,
		Schemas:     schemas,
	}
}

// GetDatasourceTypeName returns a human-readable name for the datasource type
func GetDatasourceTypeName(dt DatasourceType) string {
	switch dt {
	case DatasourceTypeAirtable:
		return "Airtable"
	default:
		return string(dt)
	}
}

// GetAllDatasourceTypes returns all available datasource types
func GetAllDatasourceTypes() []DatasourceType {
	return []DatasourceType{
		DatasourceTypeAirtable,
	}
}
