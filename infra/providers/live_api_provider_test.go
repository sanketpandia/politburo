package providers

import (
	"context"
	"encoding/json"
	"infinite-experiment/politburo/internal/models/dtos"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLiveAPIProvider_GetUserByIfcId_Success(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.URL.Path != "/users" {
			t.Errorf("Expected path /users, got %s", r.URL.Path)
		}

		response := dtos.UserStatsResponse{
			ErrorCode: 0,
			Result: []dtos.UserStats{
				{
					UserID:            "test-user-id-123",
					DiscourseUsername: strPtr("testuser"),
					OnlineFlights:     100,
					Grade:             3,
				},
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &LiveAPIProvider{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Client:  &http.Client{},
	}

	ctx := context.Background()
	result, status, err := provider.GetUserByIfcId(ctx, "testuser")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if status != http.StatusOK {
		t.Errorf("Expected status 200, got %d", status)
	}

	if len(result.Result) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(result.Result))
	}

	if result.Result[0].UserID != "test-user-id-123" {
		t.Errorf("Expected UserID test-user-id-123, got %s", result.Result[0].UserID)
	}
}

func TestLiveAPIProvider_GetUserByIfcId_EmptyID(t *testing.T) {
	provider := NewLiveAPIProvider()

	ctx := context.Background()
	_, status, err := provider.GetUserByIfcId(ctx, "")

	if err == nil {
		t.Error("Expected error for empty IFC ID")
	}

	if status != 0 {
		t.Errorf("Expected status 0, got %d", status)
	}
}

func TestLiveAPIProvider_GetUserByIfcId_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "User not found"}`))
	}))
	defer server.Close()

	provider := &LiveAPIProvider{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Client:  &http.Client{},
	}

	ctx := context.Background()
	_, status, err := provider.GetUserByIfcId(ctx, "nonexistent")

	if err == nil {
		t.Error("Expected error for 404 response")
	}

	if status != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", status)
	}
}

func TestLiveAPIProvider_GetUserFlights_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		response := dtos.UserFlightsRawResponse{
			ErrorCode: 0,
			Result: dtos.UserFlightsResponse{
				PageIndex:  1,
				TotalPages: 5,
				TotalCount: 100,
				HasNext:    true,
				Flights: []dtos.UserFlightEntry{
					{
						ID:                 "flight-1",
						Callsign:           "TEST123",
						OriginAirport:      "KJFK",
						DestinationAirport: "KLAX",
						TotalTime:          5.5,
					},
				},
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := &LiveAPIProvider{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Client:  &http.Client{},
	}

	ctx := context.Background()
	result, status, err := provider.GetUserFlights(ctx, "test-user-id", 1)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if status != http.StatusOK {
		t.Errorf("Expected status 200, got %d", status)
	}

	if len(result.Flights) != 1 {
		t.Fatalf("Expected 1 flight, got %d", len(result.Flights))
	}

	if result.Flights[0].OriginAirport != "KJFK" {
		t.Errorf("Expected origin KJFK, got %s", result.Flights[0].OriginAirport)
	}
}

func TestLiveAPIProvider_GetUserFlights_InvalidPage(t *testing.T) {
	provider := NewLiveAPIProvider()
	ctx := context.Background()

	_, status, err := provider.GetUserFlights(ctx, "test-user", 0)

	if err == nil {
		t.Error("Expected error for page number < 1")
	}

	if status != 0 {
		t.Errorf("Expected status 0, got %d", status)
	}
}

func TestLiveAPIProvider_GetUserFlights_EmptyUserID(t *testing.T) {
	provider := NewLiveAPIProvider()
	ctx := context.Background()

	_, status, err := provider.GetUserFlights(ctx, "", 1)

	if err == nil {
		t.Error("Expected error for empty user ID")
	}

	if status != 0 {
		t.Errorf("Expected status 0, got %d", status)
	}
}

// Helper function
func strPtr(s string) *string {
	return &s
}
