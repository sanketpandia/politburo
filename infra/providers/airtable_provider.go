package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/models/dtos"
	platformVA "infinite-experiment/politburo/internal/platform/va"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// AirtableProvider implements DataProvider for Airtable
type AirtableProvider struct {
	client     *http.Client
	cacheStore cache.CacheInterface
}

// NewAirtableProvider creates a new Airtable provider
func NewAirtableProvider(c cache.CacheInterface) *AirtableProvider {
	return &AirtableProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheStore: c,
	}
}

// GetProviderType returns the provider type identifier
func (p *AirtableProvider) GetProviderType() string {
	return "airtable"
}

// FetchPilotRecord fetches a single pilot record by Airtable record ID
func (p *AirtableProvider) FetchPilotRecord(ctx context.Context, pilotID string, schema *platformVA.EntitySchema) (*PilotRecord, error) {
	// Get credentials from context (should be set by service layer)
	creds, ok := ctx.Value("provider_credentials").(*platformVA.ProviderCredentials)
	if !ok {
		return nil, fmt.Errorf("provider credentials not found in context")
	}

	// Build Airtable API URL with proper encoding for table name
	encodedTableName := url.PathEscape(schema.TableName)
	apiURL := fmt.Sprintf("https://api.airtable.com/v0/%s/%s/%s",
		creds.BaseID,
		encodedTableName,
		pilotID,
	)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+creds.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	// Handle error responses
	if err := p.handleHTTPError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var airtableResp AirtableRecordResponse
	if err := json.NewDecoder(resp.Body).Decode(&airtableResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Transform to PilotRecord
	record := &PilotRecord{
		ProviderID: airtableResp.ID,
		RawFields:  airtableResp.Fields,
		Normalized: p.normalizeFields(airtableResp.Fields, schema),
	}

	return record, nil
}

// FetchRecords fetches multiple records with pagination
func (p *AirtableProvider) FetchRecords(ctx context.Context, schema *platformVA.EntitySchema, filters *SyncFilters) (*RecordSet, error) {
	creds, ok := ctx.Value("provider_credentials").(*platformVA.ProviderCredentials)
	if !ok {
		return nil, fmt.Errorf("provider credentials not found in context")
	}

	// Build request payload
	payload := p.buildFetchPayload(schema, filters)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Log payload for debugging
	if filters != nil && filters.ModifiedSince != nil {
		log.Printf("[AirtableProvider] FetchRecords - Table: %s, LastModifiedField: %s, ModifiedSince: %s, FilterFormula: %v",
			schema.TableName, schema.LastModifiedField, *filters.ModifiedSince, payload["filterByFormula"])
		log.Printf("[AirtableProvider] Full payload: %s", string(payloadBytes))
	}

	// Build URL with proper encoding for table name (spaces, special chars)
	encodedTableName := url.PathEscape(schema.TableName)
	apiURL := fmt.Sprintf("https://api.airtable.com/v0/%s/%s/listRecords",
		creds.BaseID,
		encodedTableName,
	)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+creds.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	// Handle error responses
	if err := p.handleHTTPError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var airtableResp AirtableListResponse
	if err := json.NewDecoder(resp.Body).Decode(&airtableResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Transform to RecordSet with IDs
	records := make([]RecordWithID, len(airtableResp.Records))
	for i, rec := range airtableResp.Records {
		records[i] = RecordWithID{
			ID:          rec.ID,
			Fields:      rec.Fields,
			CreatedTime: rec.CreatedTime,
		}
	}

	recordSet := &RecordSet{
		Records:      records,
		Offset:       airtableResp.Offset,
		HasMore:      airtableResp.Offset != "",
		TotalFetched: len(records),
	}

	return recordSet, nil
}

// SubmitRecord creates a new record in Airtable
func (p *AirtableProvider) SubmitRecord(ctx context.Context, schema *platformVA.EntitySchema, fields map[string]interface{}) (string, error) {
	creds, ok := ctx.Value("provider_credentials").(*platformVA.ProviderCredentials)
	if !ok {
		return "", fmt.Errorf("provider credentials not found in context")
	}

	// Build request payload
	payload := map[string]interface{}{
		"records": []map[string]interface{}{
			{
				"fields": fields,
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Build Airtable API URL with proper encoding for table name
	encodedTableName := url.PathEscape(schema.TableName)
	apiURL := fmt.Sprintf("https://api.airtable.com/v0/%s/%s",
		creds.BaseID,
		encodedTableName,
	)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+creds.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := p.client.Do(req)
	if err != nil {
		return "", &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	// Handle error responses
	if err := p.handleHTTPError(resp); err != nil {
		return "", err
	}

	// Parse response
	var airtableResp struct {
		Records []struct {
			ID string `json:"id"`
		} `json:"records"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&airtableResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(airtableResp.Records) == 0 {
		return "", fmt.Errorf("no records returned from Airtable")
	}

	return airtableResp.Records[0].ID, nil
}

// ValidateConfig validates the Airtable configuration
func (p *AirtableProvider) ValidateConfig(ctx context.Context, creds *platformVA.ProviderCredentials) (*ValidationResult, error) {
	startTime := time.Now()
	result := &ValidationResult{
		IsValid:         true,
		PhasesCompleted: []string{},
		PhasesFailed:    []string{},
		Errors:          []ValidationError{},
		Warnings:        []ValidationError{},
	}

	// Phase 1: Credential Validation
	if err := p.validateCredentials(ctx, creds); err != nil {
		result.IsValid = false
		result.PhasesFailed = append(result.PhasesFailed, "credential_validation")
		if provErr, ok := err.(*ProviderError); ok {
			result.Errors = append(result.Errors, ValidationError{
				Phase:     "credential_validation",
				Error:     provErr.Message,
				ErrorCode: provErr.Code,
				Timestamp: time.Now().Format(time.RFC3339),
			})
		}
		result.DurationMs = int(time.Since(startTime).Milliseconds())
		return result, nil
	}
	result.PhasesCompleted = append(result.PhasesCompleted, "credential_validation")

	// Phase 2: Table Validation
	// TODO: Implement in future
	result.PhasesCompleted = append(result.PhasesCompleted, "table_validation")

	// Phase 3: Field Validation
	// TODO: Implement in future
	result.PhasesCompleted = append(result.PhasesCompleted, "field_validation")

	result.DurationMs = int(time.Since(startTime).Milliseconds())
	return result, nil
}

// validateCredentials checks if the API key and base ID are valid
func (p *AirtableProvider) validateCredentials(ctx context.Context, creds *platformVA.ProviderCredentials) error {
	url := fmt.Sprintf("https://api.airtable.com/v0/meta/bases/%s/tables", creds.BaseID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+creds.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	return p.handleHTTPError(resp)
}

// handleHTTPError converts HTTP errors to ProviderError
func (p *AirtableProvider) handleHTTPError(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &ProviderError{
			Code:    constants.ErrCodeInvalidAPIKey,
			Message: constants.GetErrorMessage(constants.ErrCodeInvalidAPIKey),
			Details: string(body),
		}
	case http.StatusNotFound:
		return &ProviderError{
			Code:    constants.ErrCodeInvalidBaseID,
			Message: constants.GetErrorMessage(constants.ErrCodeInvalidBaseID),
			Details: string(body),
		}
	case http.StatusTooManyRequests:
		return &ProviderError{
			Code:    constants.ErrCodeRateLimited,
			Message: constants.GetErrorMessage(constants.ErrCodeRateLimited),
			Details: string(body),
		}
	default:
		return &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
			Details: string(body),
		}
	}
}

// normalizeFields maps raw Airtable fields to internal field names
func (p *AirtableProvider) normalizeFields(rawFields map[string]interface{}, schema *platformVA.EntitySchema) map[string]interface{} {
	normalized := make(map[string]interface{})

	for _, fieldMapping := range schema.Fields {
		if value, exists := rawFields[fieldMapping.AirtableName]; exists {
			normalized[fieldMapping.InternalName] = value
		} else if fieldMapping.DefaultValue != nil {
			normalized[fieldMapping.InternalName] = *fieldMapping.DefaultValue
		}
	}

	return normalized
}

// buildFetchPayload builds the request payload for fetching records
func (p *AirtableProvider) buildFetchPayload(schema *platformVA.EntitySchema, filters *SyncFilters) map[string]interface{} {
	payload := make(map[string]interface{})

	// Add fields to fetch
	fieldNames := schema.GetAirtableFieldNames()
	if len(fieldNames) > 0 {
		payload["fields"] = fieldNames
	}

	// Add filter - prioritize custom filter formula over modified since
	if filters != nil {
		if filters.FilterFormula != "" {
			// Use custom filter formula if provided
			payload["filterByFormula"] = filters.FilterFormula
		} else if filters.ModifiedSince != nil && schema.LastModifiedField != "" {
			// Fall back to modified since filter
			// Airtable requires double quotes for date strings in formulas
			formula := fmt.Sprintf("IS_AFTER({%s}, \"%s\")", schema.LastModifiedField, *filters.ModifiedSince)
			payload["filterByFormula"] = formula
		}
	}

	// Add pagination
	if filters != nil && filters.Offset != "" {
		payload["offset"] = filters.Offset
	}

	// Add limit
	if filters != nil && filters.Limit > 0 {
		payload["pageSize"] = filters.Limit
	}

	return payload
}

// Airtable API response structures

type AirtableRecordResponse struct {
	ID          string                 `json:"id"`
	Fields      map[string]interface{} `json:"fields"`
	CreatedTime string                 `json:"createdTime,omitempty"`
}

type AirtableListResponse struct {
	Records []AirtableRecordResponse `json:"records"`
	Offset  string                   `json:"offset,omitempty"`
}

// ProviderError represents a provider-specific error
type ProviderError struct {
	Code    string
	Message string
	Details string
	Err     error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// FetchTableFields fetches metadata for a specific table from Airtable
// It calls the Airtable Metadata API to get all fields in a table
func (p *AirtableProvider) FetchTableFields(ctx context.Context, creds *platformVA.ProviderCredentials, tableName string) ([]dtos.AirtableFieldMetadata, error) {
	url := fmt.Sprintf("https://api.airtable.com/v0/meta/bases/%s/tables", creds.BaseID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+creds.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	// Handle error responses
	if err := p.handleHTTPError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var baseMetadata dtos.AirtableBaseMetadata
	if err := json.NewDecoder(resp.Body).Decode(&baseMetadata); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Find the requested table
	for _, table := range baseMetadata.Tables {
		if table.Name == tableName {
			return table.Fields, nil
		}
	}

	return nil, fmt.Errorf("table '%s' not found in base", tableName)
}

// FetchSampleRecord fetches the first record from a table to show sample data
func (p *AirtableProvider) FetchSampleRecord(ctx context.Context, config *dtos.ProviderConfigData, tableName string) (map[string]interface{}, error) {
	encodedTableName := url.PathEscape(tableName)
	apiURL := fmt.Sprintf("https://api.airtable.com/v0/%s/%s?pageSize=1",
		config.Credentials.BaseID,
		encodedTableName,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.Credentials.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	// Handle error responses
	if err := p.handleHTTPError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var airtableResp AirtableListResponse
	if err := json.NewDecoder(resp.Body).Decode(&airtableResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Return first record's fields if it exists
	if len(airtableResp.Records) > 0 {
		return airtableResp.Records[0].Fields, nil
	}

	// Return empty map if no records found
	return make(map[string]interface{}), nil
}
