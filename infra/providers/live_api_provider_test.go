package providers

import (
	"context"
	"encoding/json"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/models/dtos"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	_ = logging.Init("test")
	os.Exit(m.Run())
}

const (
	testUserUUID     = "813ef838-f55f-40ba-99a1-594c4c28c86f"
	testFlightUUID   = "b5c5a0c3-e578-41d5-8070-ef39d95ed7b7"
	testAircraftUUID = "11111111-1111-1111-1111-111111111111"
	testLiveryUUID   = "22222222-2222-2222-2222-222222222222"
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

		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Expected bearer auth header, got %q", got)
		}
		if r.URL.Query().Get("apikey") != "" {
			t.Errorf("Expected no apikey query parameter, got %q", r.URL.RawQuery)
		}

		response := dtos.UserStatsResponse{
			ErrorCode: 0,
			Result: []dtos.UserStats{
				{
					UserID:            testUserUUID,
					DiscourseUsername: strPtr("testuser"),
					OnlineFlights:     100,
					Grade:             3,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	provider := newTestLiveAPIProvider(server)

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

	if result.Result[0].UserID != testUserUUID {
		t.Errorf("Expected UserID %s, got %s", testUserUUID, result.Result[0].UserID)
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

	provider := newTestLiveAPIProvider(server)

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
		if r.URL.Path != "/users/"+testUserUUID+"/flights" {
			t.Errorf("Expected user flights path, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Errorf("Expected page=1, got %q", got)
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
						ID:                 testFlightUUID,
						Created:            mustParseTime(t, "2026-05-21T08:00:00Z"),
						UserID:             testUserUUID,
						AircraftID:         testAircraftUUID,
						LiveryID:           testLiveryUUID,
						Callsign:           "TEST123",
						OriginAirport:      "KJFK",
						DestinationAirport: "KLAX",
						TotalTime:          5.5,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	provider := newTestLiveAPIProvider(server)

	ctx := context.Background()
	result, status, err := provider.GetUserFlights(ctx, testUserUUID, 1)

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

func newTestLiveAPIProvider(server *httptest.Server) *LiveAPIProvider {
	return NewLiveAPIProviderWithClient(&liveapi.Client{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Client:  server.Client(),
	})
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", value, err)
	}
	return parsed
}
