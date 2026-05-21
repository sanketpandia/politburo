package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/models/dtos"
)

// LiveAPIProvider adapts the canonical infra/liveapi.Client into the provider
// interface shape used by registration-style feature services. Keep feature
// packages depending on small interfaces; keep generated LiveAPI details behind
// infra/liveapi.Client.
type LiveAPIProvider struct {
	client *liveapi.Client
}

// NewLiveAPIProvider creates a feature-facing adapter around the canonical LiveAPI client.
func NewLiveAPIProvider() *LiveAPIProvider {
	return NewLiveAPIProviderWithClient(liveapi.NewClient())
}

// NewLiveAPIProviderWithClient creates an adapter around a supplied LiveAPI client.
// Tests and DI code should use this instead of constructing the provider fields directly.
func NewLiveAPIProviderWithClient(client *liveapi.Client) *LiveAPIProvider {
	return &LiveAPIProvider{client: client}
}

// GetProviderType returns the provider type identifier.
func (p *LiveAPIProvider) GetProviderType() string {
	return "infinite_flight_live_api"
}

// GetUserByIfcId fetches user stats by Infinite Flight Community username.
func (p *LiveAPIProvider) GetUserByIfcId(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error) {
	if ifcId == "" {
		return nil, 0, &ProviderError{
			Code:    constants.ErrCodeInvalidDataFormat,
			Message: "IFC ID cannot be empty",
		}
	}
	if err := p.validateReady(ctx); err != nil {
		return nil, 0, err
	}

	resp, status, err := p.client.GetUserByIfcId(ifcId)
	if err != nil {
		return nil, status, p.providerError(status, "users", err)
	}
	return convertUserStatsResponse(resp), status, nil
}

// GetUserFlights fetches user flight history with pagination.
func (p *LiveAPIProvider) GetUserFlights(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error) {
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
	if err := p.validateReady(ctx); err != nil {
		return nil, 0, err
	}

	resp, status, err := p.client.GetUserFlights(userID, page)
	if err != nil {
		return nil, status, p.providerError(status, "user_flights", err)
	}
	return convertUserFlightsResponse(resp), status, nil
}

func (p *LiveAPIProvider) validateReady(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: constants.GetErrorMessage(constants.ErrCodeNetworkError),
			Err:     err,
		}
	}
	if p == nil || p.client == nil {
		return &ProviderError{
			Code:    constants.ErrCodeNetworkError,
			Message: "Live API client is not configured",
		}
	}
	if p.client.APIKey == "" {
		return &ProviderError{
			Code:    constants.ErrCodeInvalidAPIKey,
			Message: "IF_API_KEY environment variable is not set",
		}
	}
	return nil
}

func (p *LiveAPIProvider) providerError(statusCode int, endpointGroup string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &ProviderError{Code: constants.ErrCodeNetworkError, Message: constants.GetErrorMessage(constants.ErrCodeNetworkError), Err: err}
	}

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &ProviderError{Code: constants.ErrCodeInvalidAPIKey, Message: fmt.Sprintf("Authentication failed for Live API %s", endpointGroup), Err: err}
	case http.StatusNotFound:
		return &ProviderError{Code: "RESOURCE_NOT_FOUND", Message: fmt.Sprintf("Live API resource not found: %s", endpointGroup), Err: err}
	case http.StatusTooManyRequests:
		return &ProviderError{Code: constants.ErrCodeRateLimited, Message: constants.GetErrorMessage(constants.ErrCodeRateLimited), Err: err}
	case http.StatusBadRequest:
		return &ProviderError{Code: constants.ErrCodeInvalidDataFormat, Message: fmt.Sprintf("Bad request to Live API %s", endpointGroup), Err: err}
	default:
		return &ProviderError{Code: constants.ErrCodeNetworkError, Message: fmt.Sprintf("Live API %s request failed", endpointGroup), Err: err}
	}
}

func convertUserStatsResponse(resp *liveapi.UserStatsResponse) *dtos.UserStatsResponse {
	if resp == nil {
		return nil
	}
	result := make([]dtos.UserStats, 0, len(resp.Result))
	for _, item := range resp.Result {
		result = append(result, dtos.UserStats{
			OnlineFlights:         item.OnlineFlights,
			Violations:            item.Violations,
			XP:                    item.XP,
			LandingCount:          item.LandingCount,
			FlightTime:            item.FlightTime,
			ATCOperations:         item.ATCOperations,
			ATCRank:               item.ATCRank,
			Grade:                 item.Grade,
			Hash:                  item.Hash,
			ViolationCountByLevel: dtos.ViolationCountByLevel(item.ViolationCountByLevel),
			Roles:                 item.Roles,
			UserID:                item.UserID,
			VirtualOrganization:   item.VirtualOrganization,
			DiscourseUsername:     item.DiscourseUsername,
			Groups:                item.Groups,
			ErrorCode:             item.ErrorCode,
		})
	}
	return &dtos.UserStatsResponse{ErrorCode: resp.ErrorCode, Result: result}
}

func convertUserFlightsResponse(resp *liveapi.UserFlightsResponse) *dtos.UserFlightsResponse {
	if resp == nil {
		return nil
	}
	flights := make([]dtos.UserFlightEntry, 0, len(resp.Flights))
	for _, flight := range resp.Flights {
		flights = append(flights, dtos.UserFlightEntry(flight))
	}
	return &dtos.UserFlightsResponse{
		PageIndex:   resp.PageIndex,
		TotalPages:  resp.TotalPages,
		TotalCount:  resp.TotalCount,
		HasPrevious: resp.HasPrevious,
		HasNext:     resp.HasNext,
		Flights:     flights,
	}
}
