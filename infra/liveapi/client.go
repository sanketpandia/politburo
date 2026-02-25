package liveapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"infinite-experiment/politburo/infra/logging"
)

// Client wraps the Infinite Flight Live API with authentication and HTTP helpers
type Client struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// NewClient creates a new Infinite Flight API client, reading config from environment variables
func NewClient() *Client {
	baseURL := os.Getenv("IF_API_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.infiniteflight.com/public/v2" // Default
	}
	apiKey := os.Getenv("IF_API_KEY")
	client := &http.Client{Timeout: 10 * time.Second}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Client:  client,
	}
}

// doGET performs a GET request with auth header and parses JSON into result
func (c *Client) doGET(endpoint string, result interface{}) (int, error) {
	req, err := http.NewRequest("GET", c.BaseURL+endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return resp.StatusCode, errors.New("resource not found")
	}
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return resp.StatusCode, json.NewDecoder(resp.Body).Decode(result)
}

// doPost performs a POST request with JSON payload and auth header
func (c *Client) doPost(endpoint string, payload interface{}, result interface{}) (int, error) {
	// Serialize body
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return 0, err
	}

	// Build request
	req, err := http.NewRequest("POST", c.BaseURL+endpoint, buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Read body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}

	// Restore body for JSON decode
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Status check and unmarshal
	switch resp.StatusCode {
	case http.StatusOK:
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(result)
	case http.StatusNotFound:
		return resp.StatusCode, errors.New("resource not found")
	default:
		return resp.StatusCode, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
}

// GetUserGrade fetches user grade information
func (c *Client) GetUserGrade(userID string) (*UserGradeResponse, int, error) {
	var r UserGradeResponse
	status, err := c.doGET("/user/grade/"+userID, &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetUserByIfcId fetches user stats by Infinite Flight Community ID
func (c *Client) GetUserByIfcId(ifcId string) (*UserStatsResponse, int, error) {
	var r UserStatsResponse
	reqBody := LiveApiUserStatsReq{
		DiscourseNames: []string{ifcId},
	}
	status, err := c.doPost("/users", reqBody, &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetSessions fetches all active multiplayer sessions/servers
func (c *Client) GetSessions() (*SessionsResponse, error) {
	var r SessionsResponse
	_, err := c.doGET("/sessions", &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetFlightRoute fetches the flight route for a specific flight
func (c *Client) GetFlightRoute(flightID string, sessionId string) (*FlightRouteResponse, int, error) {
	var r FlightRouteResponse
	status, err := c.doGET("/sessions/"+sessionId+"/flights/"+flightID+"/route", &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetATC fetches active ATC sessions
func (c *Client) GetATC() (*ATCResponse, int, error) {
	var r ATCResponse
	status, err := c.doGET("/atc", &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetFlights fetches all flights in a specific session
func (c *Client) GetFlights(sessionId string) (*FlightsResponse, int, error) {
	var r FlightsResponse
	logging.Info("Fetching flights for session", "sessionId", sessionId)
	status, err := c.doGET("/sessions/"+sessionId+"/flights", &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetAircraftLiveries fetches all aircraft and livery data
func (c *Client) GetAircraftLiveries() (*AircraftLiveriesResponse, int, error) {
	var r AircraftLiveriesResponse
	status, err := c.doGET("/aircraft/liveries", &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetUserFlights fetches flight history for a specific user
func (c *Client) GetUserFlights(userID string, page int) (*UserFlightsResponse, int, error) {
	var r UserFlightsRawResponse
	status, err := c.doGET("/users/"+userID+"/flights?page="+fmt.Sprint(page), &r)
	if err != nil {
		logging.Error("Failed to fetch user flights",
			"userId", userID,
			"page", page,
			"status", status,
			"error", err,
		)
		return nil, status, err
	}
	return &r.Result, status, nil
}

// GetWorldStatus fetches current world status
func (c *Client) GetWorldStatus() (*WorldStatusResponse, int, error) {
	var r WorldStatusResponse
	status, err := c.doGET("/world/status", &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetATIS fetches ATIS information
func (c *Client) GetATIS() (*ATISResponse, int, error) {
	var r ATISResponse
	status, err := c.doGET("/atis", &r)
	if err != nil {
		return nil, status, err
	}
	return &r, status, nil
}

// GetFlightPlan fetches the flight plan for a specific flight
func (c *Client) GetFlightPlan(sessionID, flightID string) (*FlightPlanResponse, int, error) {
	var wrap FlightPlanWrapper
	endpoint := "/sessions/" + sessionID + "/flights/" + flightID + "/flightplan"

	status, err := c.doGET(endpoint, &wrap)
	if err != nil {
		return nil, status, err
	}
	if wrap.ErrorCode != 0 {
		return nil, status,
			fmt.Errorf("live-api returned errorCode %d", wrap.ErrorCode)
	}
	return &wrap.Result, status, nil
}
