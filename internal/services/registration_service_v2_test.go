package services

import (
	"context"
	"errors"
	"infinite-experiment/politburo/internal/models/dtos"
	gormModels "infinite-experiment/politburo/internal/models/gorm"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Mock LiveAPIProvider
type mockLiveAPIProvider struct {
	getUserByIfcIdFunc  func(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error)
	getUserFlightsFunc  func(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error)
}

func (m *mockLiveAPIProvider) GetUserByIfcId(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error) {
	return m.getUserByIfcIdFunc(ctx, ifcId)
}

func (m *mockLiveAPIProvider) GetUserFlights(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error) {
	return m.getUserFlightsFunc(ctx, userID, page)
}

// Setup test database
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&gormModels.User{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

func TestRegistrationServiceV2_InitUserRegistration_Success(t *testing.T) {
	db := setupTestDB(t)

	userID := "test-if-user-id"
	mockProvider := &mockLiveAPIProvider{
		getUserByIfcIdFunc: func(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error) {
			return &dtos.UserStatsResponse{
				Result: []dtos.UserStats{
					{UserID: userID, Grade: 3},
				},
			}, 200, nil
		},
		getUserFlightsFunc: func(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error) {
			return &dtos.UserFlightsResponse{
				Flights: []dtos.UserFlightEntry{
					{
						OriginAirport:      "KJFK",
						DestinationAirport: "KLAX",
					},
				},
			}, 200, nil
		},
	}

	service := NewRegistrationServiceV2(db, mockProvider)

	ctx := context.Background()
	response, err := service.InitUserRegistration(ctx, "discord-123", "server-456", "testuser", "KJFK-KLAX", nil)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !response.Status {
		t.Error("Expected status true")
	}

	if len(response.Steps) != 4 {
		t.Errorf("Expected 4 steps, got %d", len(response.Steps))
	}

	// Verify all steps passed
	for i, step := range response.Steps {
		if !step.Status {
			t.Errorf("Step %d (%s) failed: %s", i, step.Name, step.Message)
		}
	}

	// Verify user was created in database
	var user gormModels.User
	err = db.Where("discord_id = ?", "discord-123").First(&user).Error
	if err != nil {
		t.Fatalf("User not found in database: %v", err)
	}

	if user.IFCommunityID != "testuser" {
		t.Errorf("Expected IFC ID testuser, got %s", user.IFCommunityID)
	}
}

func TestRegistrationServiceV2_InitUserRegistration_DuplicateUser(t *testing.T) {
	db := setupTestDB(t)

	// Pre-insert existing user
	existingUser := gormModels.User{
		DiscordID:     "discord-123",
		IFCommunityID: "existinguser",
		IsActive:      true,
	}
	db.Create(&existingUser)

	mockProvider := &mockLiveAPIProvider{}
	service := NewRegistrationServiceV2(db, mockProvider)

	ctx := context.Background()
	response, err := service.InitUserRegistration(ctx, "discord-123", "server-456", "testuser", "KJFK-KLAX", nil)

	if err == nil {
		t.Error("Expected error for duplicate user")
	}

	if response == nil {
		t.Fatal("Expected response even on error")
	}

	if response.Status {
		t.Error("Expected status false for duplicate")
	}

	// Check first step failed
	if len(response.Steps) < 1 {
		t.Fatal("Expected at least 1 step")
	}

	if response.Steps[0].Status {
		t.Error("Expected duplicate_check step to fail")
	}
}

func TestRegistrationServiceV2_InitUserRegistration_UserNotFoundInAPI(t *testing.T) {
	db := setupTestDB(t)

	mockProvider := &mockLiveAPIProvider{
		getUserByIfcIdFunc: func(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error) {
			return &dtos.UserStatsResponse{
				Result: []dtos.UserStats{},
			}, 200, nil
		},
	}

	service := NewRegistrationServiceV2(db, mockProvider)

	ctx := context.Background()
	response, err := service.InitUserRegistration(ctx, "discord-123", "server-456", "nonexistent", "KJFK-KLAX", nil)

	if err == nil {
		t.Error("Expected error for user not found")
	}

	if response.Steps[1].Status {
		t.Error("Expected if_api_validation step to fail")
	}
}

func TestRegistrationServiceV2_InitUserRegistration_FlightMismatch(t *testing.T) {
	db := setupTestDB(t)

	userID := "test-if-user-id"
	mockProvider := &mockLiveAPIProvider{
		getUserByIfcIdFunc: func(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error) {
			return &dtos.UserStatsResponse{
				Result: []dtos.UserStats{
					{UserID: userID, Grade: 3},
				},
			}, 200, nil
		},
		getUserFlightsFunc: func(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error) {
			return &dtos.UserFlightsResponse{
				Flights: []dtos.UserFlightEntry{
					{
						OriginAirport:      "EGLL",
						DestinationAirport: "LFPG",
					},
				},
			}, 200, nil
		},
	}

	service := NewRegistrationServiceV2(db, mockProvider)

	ctx := context.Background()
	response, err := service.InitUserRegistration(ctx, "discord-123", "server-456", "testuser", "KJFK-KLAX", nil)

	if err == nil {
		t.Error("Expected error for flight mismatch")
	}

	if response.Steps[2].Status {
		t.Error("Expected flight_validation step to fail")
	}
}

func TestRegistrationServiceV2_InitUserRegistration_NoRecentFlights(t *testing.T) {
	db := setupTestDB(t)

	userID := "test-if-user-id"
	mockProvider := &mockLiveAPIProvider{
		getUserByIfcIdFunc: func(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error) {
			return &dtos.UserStatsResponse{
				Result: []dtos.UserStats{
					{UserID: userID, Grade: 3},
				},
			}, 200, nil
		},
		getUserFlightsFunc: func(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error) {
			return &dtos.UserFlightsResponse{
				Flights: []dtos.UserFlightEntry{},
			}, 200, nil
		},
	}

	service := NewRegistrationServiceV2(db, mockProvider)

	ctx := context.Background()
	response, err := service.InitUserRegistration(ctx, "discord-123", "server-456", "testuser", "KJFK-KLAX", nil)

	if err == nil {
		t.Error("Expected error for no flights")
	}

	if response.Steps[2].Status {
		t.Error("Expected flight_validation step to fail")
	}
}

func TestRegistrationServiceV2_InitUserRegistration_APIError(t *testing.T) {
	db := setupTestDB(t)

	mockProvider := &mockLiveAPIProvider{
		getUserByIfcIdFunc: func(ctx context.Context, ifcId string) (*dtos.UserStatsResponse, int, error) {
			return nil, 500, errors.New("API error")
		},
	}

	service := NewRegistrationServiceV2(db, mockProvider)

	ctx := context.Background()
	response, err := service.InitUserRegistration(ctx, "discord-123", "server-456", "testuser", "KJFK-KLAX", nil)

	if err == nil {
		t.Error("Expected error for API failure")
	}

	if response.Steps[1].Status {
		t.Error("Expected if_api_validation step to fail")
	}
}

func TestRegistrationServiceV2_findRecentFlightRoute(t *testing.T) {
	mockProvider := &mockLiveAPIProvider{
		getUserFlightsFunc: func(ctx context.Context, userID string, page int) (*dtos.UserFlightsResponse, int, error) {
			if page == 1 {
				return &dtos.UserFlightsResponse{
					Flights: []dtos.UserFlightEntry{
						{OriginAirport: "", DestinationAirport: ""}, // Invalid
					},
				}, 200, nil
			}
			if page == 2 {
				return &dtos.UserFlightsResponse{
					Flights: []dtos.UserFlightEntry{
						{OriginAirport: "KJFK", DestinationAirport: "KLAX"},
					},
				}, 200, nil
			}
			return &dtos.UserFlightsResponse{Flights: []dtos.UserFlightEntry{}}, 200, nil
		},
	}

	service := NewRegistrationServiceV2(nil, mockProvider)

	route, err := service.findRecentFlightRoute(context.Background(), "test-user")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if route != "KJFK-KLAX" {
		t.Errorf("Expected route KJFK-KLAX, got %s", route)
	}
}
