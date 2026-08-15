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

type SessionsClient interface {
	GetSessions(context.Context) ([]Session, error)
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
