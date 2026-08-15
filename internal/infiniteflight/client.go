package infiniteflight

import (
	"context"
	"fmt"
	"net/http"
	"time"

	infiniteflightapi "infinite-experiment/politburo/internal/api/generated/infiniteflight"
)

type Session struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	WorldType         int       `json:"worldType"`
	Type              int       `json:"type"`
	MinimumGradeLevel int       `json:"minimumGradeLevel"`
	UserCount         int       `json:"userCount"`
	MinimumAppVersion string    `json:"minimumAppVersion"`
	MaximumAppVersion *string   `json:"maximumAppVersion"`
	MaxUsers          int       `json:"maxUsers"`
	NormalizedName    string    `json:"normalizedName"`
	Timestamp         time.Time `json:"timestamp"`
}

type Flight struct {
	Username            *string
	Callsign            string
	Latitude            float64
	Longitude           float64
	Altitude            float64
	Speed               float64
	VerticalSpeed       float64
	Track               float64
	Heading             float64
	LastReport          string
	FlightID            string
	UserID              string
	AircraftID          string
	LiveryID            string
	VirtualOrganization *string
	PilotState          int
	IsConnected         bool
}

type Livery struct {
	ID           string `json:"id"`
	AircraftID   string `json:"aircraftId"`
	AircraftName string `json:"aircraftName"`
	LiveryName   string `json:"liveryName"`
}

type SessionsClient interface {
	GetSessions(context.Context) ([]Session, error)
}

type FlightsClient interface {
	GetSessionFlights(context.Context, string) ([]Flight, error)
}

type LiveriesClient interface {
	GetAircraftLiveries(context.Context) ([]Livery, error)
}

type ClientAPI interface {
	SessionsClient
	FlightsClient
	LiveriesClient
}

type Client struct {
	generated *infiniteflightapi.ClientWithResponses
}

func NewClient(baseURL, apiKey string, timeout time.Duration) (*Client, error) {
	return newClient(baseURL, apiKey, &http.Client{Timeout: timeout})
}

func newClient(baseURL, apiKey string, httpClient *http.Client) (*Client, error) {
	generated, err := infiniteflightapi.NewClientWithResponses(
		baseURL,
		infiniteflightapi.WithHTTPClient(httpClient),
		infiniteflightapi.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+apiKey)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create Infinite Flight client: %w", err)
	}
	return &Client{generated: generated}, nil
}

func (c *Client) GetSessions(ctx context.Context) ([]Session, error) {
	response, err := c.generated.GetSessionsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("get Infinite Flight sessions: %w", err)
	}
	if response.JSON200 == nil {
		if response.JSONDefault != nil {
			return nil, fmt.Errorf("get Infinite Flight sessions: HTTP %d, errorCode %d", response.StatusCode(), response.JSONDefault.ErrorCode)
		}
		return nil, fmt.Errorf("get Infinite Flight sessions: unexpected HTTP %d", response.StatusCode())
	}
	if response.JSON200.ErrorCode != 0 {
		return nil, fmt.Errorf("get Infinite Flight sessions: errorCode %d", response.JSON200.ErrorCode)
	}

	sessions := make([]Session, 0, len(response.JSON200.Result))
	for _, item := range response.JSON200.Result {
		sessions = append(sessions, Session{
			ID: item.Id, Name: item.Name, WorldType: item.WorldType, Type: item.Type,
			MinimumGradeLevel: item.MinimumGradeLevel, UserCount: item.UserCount,
			MinimumAppVersion: item.MinimumAppVersion, MaximumAppVersion: item.MaximumAppVersion,
			MaxUsers: item.MaxUsers,
		})
	}
	return sessions, nil
}

func (c *Client) GetSessionFlights(ctx context.Context, sessionID string) ([]Flight, error) {
	response, err := c.generated.GetSessionFlightsWithResponse(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get Infinite Flight session flights: %w", err)
	}
	if response.JSON200 == nil {
		if response.JSONDefault != nil {
			return nil, fmt.Errorf("get Infinite Flight session flights: HTTP %d, errorCode %d", response.StatusCode(), response.JSONDefault.ErrorCode)
		}
		return nil, fmt.Errorf("get Infinite Flight session flights: unexpected HTTP %d", response.StatusCode())
	}
	if response.JSON200.ErrorCode != 0 {
		return nil, fmt.Errorf("get Infinite Flight session flights: errorCode %d", response.JSON200.ErrorCode)
	}

	flights := make([]Flight, 0, len(response.JSON200.Result))
	for _, item := range response.JSON200.Result {
		heading := 0.0
		if item.Heading != nil {
			heading = *item.Heading
		}
		flights = append(flights, Flight{
			Username: item.Username, Callsign: item.Callsign, Latitude: item.Latitude,
			Longitude: item.Longitude, Altitude: item.Altitude, Speed: item.Speed,
			VerticalSpeed: item.VerticalSpeed, Track: item.Track, Heading: heading,
			LastReport: item.LastReport, FlightID: item.FlightId, UserID: item.UserId,
			AircraftID: item.AircraftId, LiveryID: item.LiveryId,
			VirtualOrganization: item.VirtualOrganization, PilotState: item.PilotState,
			IsConnected: item.IsConnected,
		})
	}
	return flights, nil
}

func (c *Client) GetAircraftLiveries(ctx context.Context) ([]Livery, error) {
	response, err := c.generated.GetAircraftLiveriesWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("get Infinite Flight aircraft liveries: %w", err)
	}
	if response.JSON200 == nil {
		if response.JSONDefault != nil {
			return nil, fmt.Errorf("get Infinite Flight aircraft liveries: HTTP %d, errorCode %d", response.StatusCode(), response.JSONDefault.ErrorCode)
		}
		return nil, fmt.Errorf("get Infinite Flight aircraft liveries: unexpected HTTP %d", response.StatusCode())
	}
	if response.JSON200.ErrorCode != 0 {
		return nil, fmt.Errorf("get Infinite Flight aircraft liveries: errorCode %d", response.JSON200.ErrorCode)
	}

	liveries := make([]Livery, 0, len(response.JSON200.Result))
	for _, item := range response.JSON200.Result {
		liveries = append(liveries, Livery{
			ID: item.Id, AircraftID: item.AircraftID, AircraftName: item.AircraftName, LiveryName: item.LiveryName,
		})
	}
	return liveries, nil
}
