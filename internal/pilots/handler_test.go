package pilots

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"infinite-experiment/politburo/internal/platform/httpdto"
	"infinite-experiment/politburo/internal/testutil"
)

// Example test demonstrating the testing pattern for handlers
// This follows the existing pattern from internal/api/user_registration_v2_test.go

func TestPilotsHandler_RegisterPilot_Success(t *testing.T) {
	// Setup
	mockRegSvc := &MockRegistrationService{
		RegisterPilotFunc: func(ctx context.Context, discordUserID, discordServerID, ifcId, lastFlight string) (*RegisterPilotResponse, error) {
			return &RegisterPilotResponse{
				Success:        true,
				Message:        "Pilot registered successfully",
				IsVARegistered: true,
			}, nil
		},
	}

	handler := NewHandler(nil, &RegistrationService{}, nil)
	// Override the regSvc for testing - in real tests, you'd need to make Handler fields accessible
	// For now, this is a template showing the pattern
	_ = mockRegSvc

	// Create request
	reqBody := RegisterPilotRequest{
		IfcId:      "testuser",
		LastFlight: "KJFK-KLAX",
	}
	claims := testutil.CreateTestClaims("discord-123", "server-456", "")
	req := testutil.MakeRequest("POST", "/api/v1/pilots/register", reqBody, claims)

	// Execute
	rr := testutil.ExecuteRequest(handler.RegisterPilot(), req)

	// Assert
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response httpdto.Response[RegisterPilotResponse]
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("Expected status ok, got %s", response.Status)
	}
}

func TestPilotsHandler_RegisterPilot_MissingClaims(t *testing.T) {
	// Setup
	mockRegSvc := &MockRegistrationService{}
	handler := NewHandler(nil, &RegistrationService{}, nil)
	_ = mockRegSvc

	// Create request without claims
	reqBody := RegisterPilotRequest{
		IfcId:      "testuser",
		LastFlight: "KJFK-KLAX",
	}
	req := testutil.MakeRequest("POST", "/api/v1/pilots/register", reqBody, nil)

	// Execute
	rr := testutil.ExecuteRequest(handler.RegisterPilot(), req)

	// Assert
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestPilotsHandler_RegisterPilot_InvalidRequest(t *testing.T) {
	// Setup
	mockRegSvc := &MockRegistrationService{}
	handler := NewHandler(nil, &RegistrationService{}, nil)
	_ = mockRegSvc

	// Test cases
	tests := []struct {
		name    string
		reqBody RegisterPilotRequest
		want    int
	}{
		{
			name:    "missing ifc_id",
			reqBody: RegisterPilotRequest{LastFlight: "KJFK-KLAX"},
			want:    http.StatusBadRequest,
		},
		{
			name:    "missing last_flight",
			reqBody: RegisterPilotRequest{IfcId: "testuser"},
			want:    http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := testutil.CreateTestClaims("discord-123", "server-456", "")
			req := testutil.MakeRequest("POST", "/api/v1/pilots/register", tt.reqBody, claims)

			rr := testutil.ExecuteRequest(handler.RegisterPilot(), req)

			if rr.Code != tt.want {
				t.Errorf("Expected status %d, got %d", tt.want, rr.Code)
			}
		})
	}
}

func TestPilotsHandler_RegisterPilot_AlreadyRegistered(t *testing.T) {
	// Setup
	mockRegSvc := &MockRegistrationService{}
	handler := NewHandler(nil, &RegistrationService{}, nil)
	_ = mockRegSvc

	// Create request with existing user ID (already registered)
	reqBody := RegisterPilotRequest{
		IfcId:      "testuser",
		LastFlight: "KJFK-KLAX",
	}
	claims := testutil.CreateTestClaims("discord-123", "server-456", "1") // userID set = already registered
	req := testutil.MakeRequest("POST", "/api/v1/pilots/register", reqBody, claims)

	// Execute
	rr := testutil.ExecuteRequest(handler.RegisterPilot(), req)

	// Assert
	if rr.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", rr.Code)
	}
}

// MockRegistrationService is a mock for RegistrationService
// This should be moved to a separate mocks file or use the testutil mocks
type MockRegistrationService struct {
	RegisterPilotFunc func(ctx context.Context, discordUserID, discordServerID, ifcId, lastFlight string) (*RegisterPilotResponse, error)
}

func (m *MockRegistrationService) RegisterPilot(ctx context.Context, discordUserID, discordServerID, ifcId, lastFlight string) (*RegisterPilotResponse, error) {
	if m.RegisterPilotFunc != nil {
		return m.RegisterPilotFunc(ctx, discordUserID, discordServerID, ifcId, lastFlight)
	}
	return nil, nil
}
