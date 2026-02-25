package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/models/dtos"
	"io"
	"net/http"
	"os"
	"time"
)

// LiveAPIProvider implements a provider for Infinite Flight Live API
type LiveAPIProvider struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// NewLiveAPIProvider creates a new Infinite Flight Live API provider
func NewLiveAPIProvider() *LiveAPIProvider {
	baseURL := os.Getenv("IF_API_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.infiniteflight.com/public/v2" // Default
	}
	apiKey := os.Getenv("IF_API_KEY")

	return &LiveAPIProvider{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetProviderType returns the provider type identifier
func (p *LiveAPIProvider) GetProviderType() string {
	return "infinite_flight_live_api"
}

// ============================================================================
// User Registration Methods
// ============================================================================

// GetUserByIfcId fetches user stats by Infinite Flight Community username
func (p *LiveAPIProvider) GetUserByIfcId(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error) {
	// Input validation
	if ifcId == "" {
		return nil, 0, &ProviderError{
			Code:    constants.ErrCodeInvalidDataFormat,
			Message: "IFC ID cannot be empty",
		}
	}

	// Build request body
	reqBody := dtos.LiveApiUserStatsReq{
		DiscourseNames: []string{ifcId},
	}

	// Make POST request
	var result dtos.UserStatsResponse
	status, err := p.doPost(ctx, "/users", reqBody, &result)
	if err != nil {
		return nil, status, err
	}

	return &result, status, nil
}

// GetUserFlights fetches user flight history with pagination
func (p *LiveAPIProvider) GetUserFlights(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error) {
	// Input validation
	if userID == "" {
		return nil, 0, &ProviderError{
			Code:    constants.ErrCodeInvalidDataFormat,
			Message: "User ID cannot be empty",
		}
	}

	if page < 1 {
		return nil, 0, &ProviderError{
			Code:    constants.ErrCodeInvalidDataFormat,
			Message: "Page number must be greater than 0",
		}
	}

	// Build endpoint
	endpoint := fmt.Sprintf("/users/%s/flights?page=%d", userID, page)

	// Make GET request
	var rawResp dtos.UserFlightsRawResponse
	status, err := p.doGET(ctx, endpoint, &rawResp)
	if err != nil {
		return nil, status, err
	}

	return &rawResp.Result, status, nil
}

// ============================================================================
// HTTP Helper Methods
// ============================================================================

// doGET performs a GET request with authentication
func (p *LiveAPIProvider) doGET(ctx context.Context, endpoint string, result interface{}) (int, error) {
	// Validate API key
	if p.APIKey == "" {
		return 0, &ProviderError{
			Code:    constants.ErrCodeInvalidAPIKey,
			Message: "IF_API_KEY environment variable is not set",
		}
	}

	// Build request
	url := p.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: "Failed to create request",
			Err:     err,
		}
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := p.Client.Do(req)
	if err != nil {
		return 0, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if err := p.handleHTTPError(resp, endpoint); err != nil {
		return resp.StatusCode, err
	}

	// Parse response
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: "Failed to decode response",
			Details: string(bodyBytes),
			Err:     err,
		}
	}

	return resp.StatusCode, nil
}

// doPost performs a POST request with authentication and JSON body
func (p *LiveAPIProvider) doPost(ctx context.Context, endpoint string, payload interface{}, result interface{}) (int, error) {
	// Validate API key
	if p.APIKey == "" {
		return 0, &ProviderError{
			Code:    constants.ErrCodeInvalidAPIKey,
			Message: "IF_API_KEY environment variable is not set",
		}
	}

	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: "Failed to marshal request body",
			Err:     err,
		}
	}

	// Build request
	url := p.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return 0, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: "Failed to create request",
			Err:     err,
		}
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := p.Client.Do(req)
	if err != nil {
		return 0, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}
	defer resp.Body.Close()

	// Read body for potential error messages
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: "Failed to read response body",
			Err:     readErr,
		}
	}

	// Handle HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, p.buildHTTPError(resp.StatusCode, endpoint, string(bodyBytes))
	}

	// Parse response
	if err := json.Unmarshal(bodyBytes, result); err != nil {
		return resp.StatusCode, &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: "Failed to decode response",
			Details: string(bodyBytes),
			Err:     err,
		}
	}

	return resp.StatusCode, nil
}

// handleHTTPError converts HTTP errors to ProviderError
func (p *LiveAPIProvider) handleHTTPError(resp *http.Response, endpoint string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return p.buildHTTPError(resp.StatusCode, endpoint, string(bodyBytes))
}

// buildHTTPError creates appropriate error based on status code
func (p *LiveAPIProvider) buildHTTPError(statusCode int, endpoint string, body string) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return &ProviderError{
			Code:    constants.ErrCodeInvalidAPIKey,
			Message: fmt.Sprintf("Authentication failed for endpoint %s", endpoint),
			Details: body,
		}
	case http.StatusNotFound:
		return &ProviderError{
			Code:    "RESOURCE_NOT_FOUND",
			Message: fmt.Sprintf("Resource not found: %s", endpoint),
			Details: body,
		}
	case http.StatusTooManyRequests:
		return &ProviderError{
			Code:    constants.ErrCodeRateLimited,
			Message: constants.GetErrorMessage(constants.ErrCodeRateLimited),
			Details: body,
		}
	case http.StatusBadRequest:
		return &ProviderError{
			Code:    constants.ErrCodeInvalidDataFormat,
			Message: fmt.Sprintf("Bad request to %s", endpoint),
			Details: body,
		}
	default:
		return &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: fmt.Sprintf("HTTP %d from %s: %s", statusCode, endpoint, body),
			Details: body,
		}
	}
}
