package liveapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	liveapigen "infinite-experiment/politburo/infra/liveapi/generated"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/metrics"

	"github.com/google/uuid"
)

// Client wraps the Infinite Flight Live API with authentication and HTTP helpers
type Client struct {
	BaseURL   string
	APIKey    string
	Client    *http.Client
	generated *liveapigen.ClientWithResponses
	metrics   *metrics.MetricsRegistry
}

// NewClient creates a new Infinite Flight API client, reading config from environment variables
func NewClient() *Client {
	return NewClientWithMetrics(nil)
}

// NewClientWithMetrics creates a new Infinite Flight API client with optional wrapper metrics.
func NewClientWithMetrics(metricsReg *metrics.MetricsRegistry) *Client {
	baseURL := os.Getenv("IF_API_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.infiniteflight.com/public/v2" // Default
	}
	apiKey := os.Getenv("IF_API_KEY")
	httpClient := &http.Client{Timeout: 10 * time.Second}
	generated, err := liveapigen.NewClientWithResponses(
		baseURL,
		liveapigen.WithHTTPClient(httpClient),
		liveapigen.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+apiKey)
			return nil
		}),
	)
	if err != nil {
		logging.Error("Failed to initialize generated Live API client", "error", err)
	}
	return &Client{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		Client:    httpClient,
		generated: generated,
		metrics:   metricsReg,
	}
}

// SetMetrics attaches the shared Politburo metrics registry to an existing LiveAPI client.
func (c *Client) SetMetrics(metricsReg *metrics.MetricsRegistry) {
	c.metrics = metricsReg
}

func statusClass(status int) string {
	if status == 0 {
		return "none"
	}
	return fmt.Sprintf("%dxx", status/100)
}

func liveAPIStatusErrorType(status int) string {
	switch status {
	case 0:
		return "network"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "auth"
	case http.StatusNotFound:
		return "not_found"
	}
	if status >= 400 && status < 500 {
		return "status_4xx"
	}
	if status >= 500 {
		return "status_5xx"
	}
	return "none"
}

func liveAPIErrorCodeType(code liveapigen.LiveAPIErrorCode) string {
	if code == liveapigen.Ok {
		return "none"
	}
	return fmt.Sprintf("error_code_%d", code)
}

func (c *Client) observeLiveAPICall(endpointGroup string, start time.Time, status int, errorType string, err error) {
	if errorType == "" {
		if err != nil {
			errorType = liveAPIStatusErrorType(status)
		} else {
			errorType = "none"
		}
	}
	class := statusClass(status)
	duration := time.Since(start)
	if c.metrics != nil {
		c.metrics.LiveAPIRequestsTotal.WithLabelValues("liveapi", endpointGroup, class, errorType).Inc()
		c.metrics.LiveAPIRequestDuration.WithLabelValues("liveapi", endpointGroup, class, errorType).Observe(duration.Seconds())
	}
	fields := []interface{}{
		"provider", "liveapi",
		"endpoint_group", endpointGroup,
		"status_class", class,
		"status_code", status,
		"error_type", errorType,
		"duration_ms", duration.Milliseconds(),
	}
	if err != nil || status >= 400 {
		logging.Warn("LiveAPI wrapper call completed with error", fields...)
		return
	}
	logging.Debug("LiveAPI wrapper call completed", fields...)
}

func (c *Client) generatedClient() (*liveapigen.ClientWithResponses, error) {
	if c.generated != nil {
		return c.generated, nil
	}

	generated, err := liveapigen.NewClientWithResponses(
		c.BaseURL,
		liveapigen.WithHTTPClient(c.Client),
		liveapigen.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+c.APIKey)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize generated Live API client: %w", err)
	}
	c.generated = generated
	return generated, nil
}

func liveAPIStatusError(status int) error {
	switch status {
	case http.StatusNotFound:
		return errors.New("resource not found")
	case http.StatusTooManyRequests:
		return fmt.Errorf("live-api rate limited: unexpected status %d", status)
	default:
		return fmt.Errorf("unexpected status %d", status)
	}
}

func liveAPIErrorCodeError(code liveapigen.LiveAPIErrorCode) error {
	if code == liveapigen.Ok {
		return nil
	}
	return fmt.Errorf("live-api returned errorCode %d", code)
}

func parseUUIDParam(name, value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return id, nil
}

func parseAPITime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{apiLayout, time.RFC3339, time.RFC3339Nano} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse time %q", value)
}

func toStringSlice(ids []uuid.UUID) []string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, id.String())
	}
	return values
}

// doGET performs a GET request with auth header and parses JSON into result
func (c *Client) doGET(endpointGroup, endpoint string, result interface{}) (int, error) {
	start := time.Now()
	req, err := http.NewRequest("GET", c.BaseURL+endpoint, nil)
	if err != nil {
		c.observeLiveAPICall(endpointGroup, start, 0, "request_build", err)
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		c.observeLiveAPICall(endpointGroup, start, 0, "network", err)
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		err := errors.New("resource not found")
		c.observeLiveAPICall(endpointGroup, start, resp.StatusCode, "not_found", err)
		return resp.StatusCode, err
	}
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected status %d", resp.StatusCode)
		c.observeLiveAPICall(endpointGroup, start, resp.StatusCode, liveAPIStatusErrorType(resp.StatusCode), err)
		return resp.StatusCode, err
	}

	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		c.observeLiveAPICall(endpointGroup, start, resp.StatusCode, "decode_error", err)
		return resp.StatusCode, err
	}
	c.observeLiveAPICall(endpointGroup, start, resp.StatusCode, "none", nil)
	return resp.StatusCode, nil
}

// doPost performs a POST request with JSON payload and auth header
func (c *Client) doPost(endpointGroup, endpoint string, payload interface{}, result interface{}) (int, error) {
	start := time.Now()
	// Serialize body
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		c.observeLiveAPICall(endpointGroup, start, 0, "encode_error", err)
		return 0, err
	}

	// Build request
	req, err := http.NewRequest("POST", c.BaseURL+endpoint, buf)
	if err != nil {
		c.observeLiveAPICall(endpointGroup, start, 0, "request_build", err)
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.Client.Do(req)
	if err != nil {
		c.observeLiveAPICall(endpointGroup, start, 0, "network", err)
		return 0, err
	}
	defer resp.Body.Close()

	// Read body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.observeLiveAPICall(endpointGroup, start, resp.StatusCode, "read_error", err)
		return resp.StatusCode, err
	}

	// Restore body for JSON decode
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Status check and unmarshal
	switch resp.StatusCode {
	case http.StatusOK:
		err := json.NewDecoder(resp.Body).Decode(result)
		if err != nil {
			c.observeLiveAPICall(endpointGroup, start, resp.StatusCode, "decode_error", err)
			return resp.StatusCode, err
		}
		c.observeLiveAPICall(endpointGroup, start, resp.StatusCode, "none", nil)
		return resp.StatusCode, nil
	case http.StatusNotFound:
		err := errors.New("resource not found")
		c.observeLiveAPICall(endpointGroup, start, resp.StatusCode, "not_found", err)
		return resp.StatusCode, err
	default:
		err := fmt.Errorf("unexpected status %d", resp.StatusCode)
		c.observeLiveAPICall(endpointGroup, start, resp.StatusCode, liveAPIStatusErrorType(resp.StatusCode), err)
		return resp.StatusCode, err
	}
}

// GetUserGrade fetches user grade information
func (c *Client) GetUserGrade(userID string) (*UserGradeResponse, int, error) {
	start := time.Now()
	client, err := c.generatedClient()
	if err != nil {
		c.observeLiveAPICall("users", start, 0, "client_init", err)
		return nil, 0, err
	}
	id, err := parseUUIDParam("userID", userID)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.GetUserGradeWithResponse(context.Background(), id)
	if err != nil {
		c.observeLiveAPICall("users", start, 0, "network", err)
		return nil, 0, err
	}
	status := resp.StatusCode()
	if status != http.StatusOK {
		err := liveAPIStatusError(status)
		c.observeLiveAPICall("users", start, status, liveAPIStatusErrorType(status), err)
		return nil, status, err
	}
	if resp.JSON200 == nil {
		err := errors.New("live-api returned empty user grade response")
		c.observeLiveAPICall("users", start, status, "empty_response", err)
		return nil, status, err
	}
	if err := liveAPIErrorCodeError(resp.JSON200.ErrorCode); err != nil {
		c.observeLiveAPICall("users", start, status, liveAPIErrorCodeType(resp.JSON200.ErrorCode), err)
		return nil, status, err
	}
	c.observeLiveAPICall("users", start, status, "none", nil)
	return &UserGradeResponse{UserID: userID, Grade: resp.JSON200.Result.GradeDetails.GradeIndex}, status, nil
}

// GetUserByIfcId fetches user stats by Infinite Flight Community ID
func (c *Client) GetUserByIfcId(ifcId string) (*UserStatsResponse, int, error) {
	start := time.Now()
	client, err := c.generatedClient()
	if err != nil {
		c.observeLiveAPICall("users", start, 0, "client_init", err)
		return nil, 0, err
	}
	discourseNames := []string{ifcId}
	resp, err := client.GetUserStatsWithResponse(context.Background(), liveapigen.UserStatsRequest{
		DiscourseNames: &discourseNames,
	})
	if err != nil {
		c.observeLiveAPICall("users", start, 0, "network", err)
		return nil, 0, err
	}
	status := resp.StatusCode()
	if status != http.StatusOK {
		err := liveAPIStatusError(status)
		c.observeLiveAPICall("users", start, status, liveAPIStatusErrorType(status), err)
		return nil, status, err
	}
	if resp.JSON200 == nil {
		err := errors.New("live-api returned empty user stats response")
		c.observeLiveAPICall("users", start, status, "empty_response", err)
		return nil, status, err
	}
	if err := liveAPIErrorCodeError(resp.JSON200.ErrorCode); err != nil {
		c.observeLiveAPICall("users", start, status, liveAPIErrorCodeType(resp.JSON200.ErrorCode), err)
		return nil, status, err
	}
	stats := make([]UserStats, 0, len(resp.JSON200.Result))
	for _, item := range resp.JSON200.Result {
		stats = append(stats, UserStats{
			OnlineFlights:         item.OnlineFlights,
			Violations:            item.Violations,
			XP:                    int(item.Xp),
			LandingCount:          item.LandingCount,
			FlightTime:            int(item.FlightTime),
			ATCOperations:         item.AtcOperations,
			ATCRank:               item.AtcRank,
			Grade:                 item.Grade,
			Hash:                  item.Hash,
			ViolationCountByLevel: ViolationCountByLevel(item.ViolationCountByLevel),
			Roles:                 item.Roles,
			UserID:                item.UserId.String(),
			VirtualOrganization:   item.VirtualOrganization,
			DiscourseUsername:     item.DiscourseUsername,
			Groups:                toStringSlice(item.Groups),
			ErrorCode:             item.ErrorCode,
		})
	}
	c.observeLiveAPICall("users", start, status, "none", nil)
	return &UserStatsResponse{ErrorCode: int(resp.JSON200.ErrorCode), Result: stats}, status, nil
}

// GetSessions fetches all active multiplayer sessions/servers
func (c *Client) GetSessions() (*SessionsResponse, error) {
	start := time.Now()
	client, err := c.generatedClient()
	if err != nil {
		c.observeLiveAPICall("sessions", start, 0, "client_init", err)
		return nil, err
	}
	resp, err := client.GetSessionsWithResponse(context.Background())
	if err != nil {
		c.observeLiveAPICall("sessions", start, 0, "network", err)
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		err := liveAPIStatusError(resp.StatusCode())
		c.observeLiveAPICall("sessions", start, resp.StatusCode(), liveAPIStatusErrorType(resp.StatusCode()), err)
		return nil, err
	}
	if resp.JSON200 == nil {
		err := errors.New("live-api returned empty sessions response")
		c.observeLiveAPICall("sessions", start, resp.StatusCode(), "empty_response", err)
		return nil, err
	}
	if err := liveAPIErrorCodeError(resp.JSON200.ErrorCode); err != nil {
		c.observeLiveAPICall("sessions", start, resp.StatusCode(), liveAPIErrorCodeType(resp.JSON200.ErrorCode), err)
		return nil, err
	}
	sessions := make([]Session, 0, len(resp.JSON200.Result))
	for _, session := range resp.JSON200.Result {
		sessions = append(sessions, Session{
			MaxUsers:          session.MaxUsers,
			ID:                session.Id.String(),
			Name:              session.Name,
			UserCount:         session.UserCount,
			Type:              session.Type,
			WorldType:         session.WorldType,
			MinimumGradeLevel: session.MinimumGradeLevel,
			MinimumAppVersion: session.MinimumAppVersion,
			MaximumAppVersion: session.MaximumAppVersion,
		})
	}
	c.observeLiveAPICall("sessions", start, resp.StatusCode(), "none", nil)
	return &SessionsResponse{ErrorCode: int(resp.JSON200.ErrorCode), Result: sessions}, nil
}

// GetFlightRoute fetches the flight route for a specific flight
func (c *Client) GetFlightRoute(flightID string, sessionId string) (*FlightRouteResponse, int, error) {
	start := time.Now()
	client, err := c.generatedClient()
	if err != nil {
		c.observeLiveAPICall("flights", start, 0, "client_init", err)
		return nil, 0, err
	}
	sid, err := parseUUIDParam("sessionID", sessionId)
	if err != nil {
		return nil, 0, err
	}
	fid, err := parseUUIDParam("flightID", flightID)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.GetFlightRouteWithResponse(context.Background(), sid, fid)
	if err != nil {
		c.observeLiveAPICall("flights", start, 0, "network", err)
		return nil, 0, err
	}
	status := resp.StatusCode()
	if status != http.StatusOK {
		err := liveAPIStatusError(status)
		c.observeLiveAPICall("flights", start, status, liveAPIStatusErrorType(status), err)
		return nil, status, err
	}
	if resp.JSON200 == nil {
		err := errors.New("live-api returned empty flight route response")
		c.observeLiveAPICall("flights", start, status, "empty_response", err)
		return nil, status, err
	}
	if err := liveAPIErrorCodeError(resp.JSON200.ErrorCode); err != nil {
		c.observeLiveAPICall("flights", start, status, liveAPIErrorCodeType(resp.JSON200.ErrorCode), err)
		return nil, status, err
	}
	positions := make([]FlightPosition, 0, len(resp.JSON200.Result))
	for _, position := range resp.JSON200.Result {
		date, err := parseAPITime(position.Date)
		if err != nil {
			c.observeLiveAPICall("flights", start, status, "decode_error", err)
			return nil, status, err
		}
		positions = append(positions, FlightPosition{
			Latitude:    position.Latitude,
			Longitude:   position.Longitude,
			Altitude:    position.Altitude,
			Track:       position.Track,
			GroundSpeed: position.GroundSpeed,
			Date:        date,
		})
	}
	c.observeLiveAPICall("flights", start, status, "none", nil)
	return &FlightRouteResponse{ErrorCode: int(resp.JSON200.ErrorCode), Result: positions}, status, nil
}

// GetATC fetches active ATC sessions
func (c *Client) GetATC() (*ATCResponse, int, error) {
	var r ATCResponse
	status, err := c.doGET("flights", "/atc", &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetFlights fetches all flights in a specific session
func (c *Client) GetFlights(sessionId string) (*FlightsResponse, int, error) {
	start := time.Now()
	client, err := c.generatedClient()
	if err != nil {
		c.observeLiveAPICall("flights", start, 0, "client_init", err)
		return nil, 0, err
	}
	sid, err := parseUUIDParam("sessionID", sessionId)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.GetSessionFlightsWithResponse(context.Background(), sid)
	if err != nil {
		c.observeLiveAPICall("flights", start, 0, "network", err)
		return nil, 0, err
	}
	status := resp.StatusCode()
	if status != http.StatusOK {
		err := liveAPIStatusError(status)
		c.observeLiveAPICall("flights", start, status, liveAPIStatusErrorType(status), err)
		return nil, status, err
	}
	if resp.JSON200 == nil {
		err := errors.New("live-api returned empty flights response")
		c.observeLiveAPICall("flights", start, status, "empty_response", err)
		return nil, status, err
	}
	if err := liveAPIErrorCodeError(resp.JSON200.ErrorCode); err != nil {
		c.observeLiveAPICall("flights", start, status, liveAPIErrorCodeType(resp.JSON200.ErrorCode), err)
		return nil, status, err
	}
	flights := make([]FlightEntry, 0, len(resp.JSON200.Result))
	for _, flight := range resp.JSON200.Result {
		virtualOrganization := ""
		if flight.VirtualOrganization != nil {
			virtualOrganization = *flight.VirtualOrganization
		}
		username := ""
		if flight.Username != nil {
			username = *flight.Username
		}
		flights = append(flights, FlightEntry{
			Username:            username,
			Callsign:            flight.Callsign,
			Latitude:            flight.Latitude,
			Longitude:           flight.Longitude,
			Altitude:            flight.Altitude,
			Speed:               flight.Speed,
			VerticalSpeed:       flight.VerticalSpeed,
			Track:               flight.Track,
			LastReport:          flight.LastReport,
			FlightID:            flight.FlightId.String(),
			UserID:              flight.UserId.String(),
			AircraftID:          flight.AircraftId.String(),
			LiveryID:            flight.LiveryId.String(),
			VirtualOrganization: virtualOrganization,
			PilotState:          flight.PilotState,
			IsConnected:         flight.IsConnected,
		})
	}
	c.observeLiveAPICall("flights", start, status, "none", nil)
	return &FlightsResponse{Flights: flights}, status, nil
}

// GetAircraftLiveries fetches all aircraft and livery data
func (c *Client) GetAircraftLiveries() (*AircraftLiveriesResponse, int, error) {
	start := time.Now()
	client, err := c.generatedClient()
	if err != nil {
		c.observeLiveAPICall("aircraft", start, 0, "client_init", err)
		return nil, 0, err
	}
	resp, err := client.GetAircraftLiveriesWithResponse(context.Background())
	if err != nil {
		c.observeLiveAPICall("aircraft", start, 0, "network", err)
		return nil, 0, err
	}
	status := resp.StatusCode()
	if status != http.StatusOK {
		err := liveAPIStatusError(status)
		c.observeLiveAPICall("aircraft", start, status, liveAPIStatusErrorType(status), err)
		return nil, status, err
	}
	if resp.JSON200 == nil {
		err := errors.New("live-api returned empty aircraft liveries response")
		c.observeLiveAPICall("aircraft", start, status, "empty_response", err)
		return nil, status, err
	}
	if err := liveAPIErrorCodeError(resp.JSON200.ErrorCode); err != nil {
		c.observeLiveAPICall("aircraft", start, status, liveAPIErrorCodeType(resp.JSON200.ErrorCode), err)
		return nil, status, err
	}
	liveries := make([]AircraftLivery, 0, len(resp.JSON200.Result))
	for _, livery := range resp.JSON200.Result {
		liveries = append(liveries, AircraftLivery{
			LiveryId:     livery.Id.String(),
			AircraftID:   livery.AircraftID.String(),
			LiveryName:   livery.LiveryName,
			AircraftName: livery.AircraftName,
		})
	}
	c.observeLiveAPICall("aircraft", start, status, "none", nil)
	return &AircraftLiveriesResponse{Liveries: liveries, ErrorCode: int(resp.JSON200.ErrorCode)}, status, nil
}

// GetUserFlights fetches flight history for a specific user
func (c *Client) GetUserFlights(userID string, page int) (*UserFlightsResponse, int, error) {
	start := time.Now()
	client, err := c.generatedClient()
	if err != nil {
		c.observeLiveAPICall("users", start, 0, "client_init", err)
		return nil, 0, err
	}
	uid, err := parseUUIDParam("userID", userID)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.GetUserFlightsWithResponse(context.Background(), uid, &liveapigen.GetUserFlightsParams{Page: &page})
	if err != nil {
		c.observeLiveAPICall("users", start, 0, "network", err)
		return nil, 0, err
	}
	status := resp.StatusCode()
	if status != http.StatusOK {
		err := liveAPIStatusError(status)
		c.observeLiveAPICall("users", start, status, liveAPIStatusErrorType(status), err)
		return nil, status, err
	}
	if resp.JSON200 == nil {
		err := errors.New("live-api returned empty user flights response")
		c.observeLiveAPICall("users", start, status, "empty_response", err)
		return nil, status, err
	}
	if err := liveAPIErrorCodeError(resp.JSON200.ErrorCode); err != nil {
		c.observeLiveAPICall("users", start, status, liveAPIErrorCodeType(resp.JSON200.ErrorCode), err)
		return nil, status, err
	}
	flights := make([]UserFlightEntry, 0, len(resp.JSON200.Result.Data))
	for _, flight := range resp.JSON200.Result.Data {
		created, err := parseAPITime(flight.Created)
		if err != nil {
			c.observeLiveAPICall("users", start, status, "decode_error", err)
			return nil, status, err
		}
		originAirport := ""
		if flight.OriginAirport != nil {
			originAirport = *flight.OriginAirport
		}
		destinationAirport := ""
		if flight.DestinationAirport != nil {
			destinationAirport = *flight.DestinationAirport
		}
		flights = append(flights, UserFlightEntry{
			ID:                 flight.Id.String(),
			Created:            created,
			UserID:             flight.UserId.String(),
			AircraftID:         flight.AircraftId.String(),
			LiveryID:           flight.LiveryId.String(),
			Callsign:           flight.Callsign,
			Server:             flight.Server,
			DayTime:            flight.DayTime,
			NightTime:          flight.NightTime,
			TotalTime:          flight.TotalTime,
			LandingCount:       flight.LandingCount,
			OriginAirport:      originAirport,
			DestinationAirport: destinationAirport,
			XP:                 flight.Xp,
			WorldType:          flight.WorldType,
			Violations:         make([]any, len(flight.Violations)),
		})
	}
	c.observeLiveAPICall("users", start, status, "none", nil)
	return &UserFlightsResponse{
		PageIndex:   resp.JSON200.Result.PageIndex,
		TotalPages:  resp.JSON200.Result.TotalPages,
		TotalCount:  resp.JSON200.Result.TotalCount,
		HasPrevious: resp.JSON200.Result.HasPreviousPage,
		HasNext:     resp.JSON200.Result.HasNextPage,
		Flights:     flights,
	}, status, nil
}

// GetWorldStatus fetches current world status
func (c *Client) GetWorldStatus() (*WorldStatusResponse, int, error) {
	var r WorldStatusResponse
	status, err := c.doGET("sessions", "/world/status", &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetATIS fetches ATIS information
func (c *Client) GetATIS() (*ATISResponse, int, error) {
	var r ATISResponse
	status, err := c.doGET("flights", "/atis", &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetFlightPlan fetches the flight plan for a specific flight
func (c *Client) GetFlightPlan(sessionID, flightID string) (*FlightPlanResponse, int, error) {
	start := time.Now()
	client, err := c.generatedClient()
	if err != nil {
		c.observeLiveAPICall("flight_plan", start, 0, "client_init", err)
		return nil, 0, err
	}
	sid, err := parseUUIDParam("sessionID", sessionID)
	if err != nil {
		return nil, 0, err
	}
	fid, err := parseUUIDParam("flightID", flightID)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.GetFlightPlanWithResponse(context.Background(), sid, fid)
	if err != nil {
		c.observeLiveAPICall("flight_plan", start, 0, "network", err)
		return nil, 0, err
	}
	status := resp.StatusCode()
	if status != http.StatusOK {
		err := liveAPIStatusError(status)
		c.observeLiveAPICall("flight_plan", start, status, liveAPIStatusErrorType(status), err)
		return nil, status, err
	}
	if resp.JSON200 == nil {
		err := errors.New("live-api returned empty flight plan response")
		c.observeLiveAPICall("flight_plan", start, status, "empty_response", err)
		return nil, status, err
	}
	if err := liveAPIErrorCodeError(resp.JSON200.ErrorCode); err != nil {
		c.observeLiveAPICall("flight_plan", start, status, liveAPIErrorCodeType(resp.JSON200.ErrorCode), err)
		return nil, status, err
	}
	lastUpdate, err := parseAPITime(resp.JSON200.Result.LastUpdate)
	if err != nil {
		c.observeLiveAPICall("flight_plan", start, status, "decode_error", err)
		return nil, status, err
	}
	c.observeLiveAPICall("flight_plan", start, status, "none", nil)
	return &FlightPlanResponse{
		FlightPlanID:    resp.JSON200.Result.FlightPlanId.String(),
		FlightID:        resp.JSON200.Result.FlightId.String(),
		Waypoints:       resp.JSON200.Result.Waypoints,
		LastUpdate:      APITime{Time: lastUpdate},
		FlightPlanItems: convertFlightPlanItems(resp.JSON200.Result.FlightPlanItems),
	}, status, nil
}

func convertFlightPlanItems(items []liveapigen.FlightPlanItem) []FlightPlanItem {
	converted := make([]FlightPlanItem, 0, len(items))
	for _, item := range items {
		children := []FlightPlanItem(nil)
		if item.Children != nil {
			children = convertFlightPlanItems(*item.Children)
		}
		converted = append(converted, FlightPlanItem{
			Name:       item.Name,
			Type:       item.Type,
			Children:   children,
			Identifier: item.Identifier,
			Altitude:   item.Altitude,
			Location: Location{
				Latitude:  item.Location.Latitude,
				Longitude: item.Location.Longitude,
				Altitude:  item.Location.Altitude,
			},
		})
	}
	return converted
}
