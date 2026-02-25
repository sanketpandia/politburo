package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/models/dtos"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Mock RegistrationServiceV2
type mockRegistrationServiceV2 struct {
	initUserRegistrationFunc func(ctx context.Context, discordUserID, discordServerID, ifcId, lastFlight string, callsign *string) (*dtos.InitApiResponse, error)
	linkUserToVAFunc         func(ctx context.Context, discordUserID, discordServerID, callsign string) (map[string]interface{}, error)
}

func (m *mockRegistrationServiceV2) InitUserRegistration(ctx context.Context, discordUserID, discordServerID, ifcId, lastFlight string, callsign *string) (*dtos.InitApiResponse, error) {
	return m.initUserRegistrationFunc(ctx, discordUserID, discordServerID, ifcId, lastFlight, callsign)
}

func (m *mockRegistrationServiceV2) LinkUserToVA(ctx context.Context, discordUserID, discordServerID, callsign string) (map[string]interface{}, error) {
	if m.linkUserToVAFunc != nil {
		return m.linkUserToVAFunc(ctx, discordUserID, discordServerID, callsign)
	}
	return nil, nil
}

func TestInitUserRegistrationHandlerV2_Success(t *testing.T) {
	mockService := &mockRegistrationServiceV2{
		initUserRegistrationFunc: func(ctx context.Context, discordUserID, discordServerID, ifcId, lastFlight string, callsign *string) (*dtos.InitApiResponse, error) {
			return &dtos.InitApiResponse{
				IfcId:   ifcId,
				Status:  true,
				Message: "User registered successfully",
				Steps: []dtos.RegistrationStep{
					{Name: "duplicate_check", Status: true, Message: "OK"},
					{Name: "if_api_validation", Status: true, Message: "OK"},
					{Name: "flight_validation", Status: true, Message: "OK"},
					{Name: "database_insert", Status: true, Message: "OK"},
				},
			}, nil
		},
	}

	handler := InitUserRegistrationHandlerV2(mockService)

	reqBody := dtos.InitUserRegistrationReq{
		IfcId:      "testuser",
		LastFlight: "KJFK-KLAX",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/user/register/init", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	// Add claims to context
	claims := &auth.APIKeyClaims{
		DiscordUIDVal:      "discord-123",
		DiscordServerIDVal: "server-456",
	}
	ctx := auth.SetUserClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response dtos.APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("Expected status ok, got %s", response.Status)
	}
}

func TestInitUserRegistrationHandlerV2_MissingClaims(t *testing.T) {
	mockService := &mockRegistrationServiceV2{}
	handler := InitUserRegistrationHandlerV2(mockService)

	reqBody := dtos.InitUserRegistrationReq{
		IfcId:      "testuser",
		LastFlight: "KJFK-KLAX",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/user/register/init", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestInitUserRegistrationHandlerV2_InvalidJSON(t *testing.T) {
	mockService := &mockRegistrationServiceV2{}
	handler := InitUserRegistrationHandlerV2(mockService)

	req := httptest.NewRequest("POST", "/api/v1/user/register/init", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	claims := &auth.APIKeyClaims{
		DiscordUIDVal: "discord-123",
	}
	ctx := auth.SetUserClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestInitUserRegistrationHandlerV2_MissingIfcId(t *testing.T) {
	mockService := &mockRegistrationServiceV2{}
	handler := InitUserRegistrationHandlerV2(mockService)

	reqBody := dtos.InitUserRegistrationReq{
		IfcId:      "", // Empty
		LastFlight: "KJFK-KLAX",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/user/register/init", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	claims := &auth.APIKeyClaims{
		DiscordUIDVal: "discord-123",
	}
	ctx := auth.SetUserClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var response dtos.APIResponse
	json.NewDecoder(rr.Body).Decode(&response)

	if response.Message != "IFC ID is required" {
		t.Errorf("Expected error message about IFC ID, got %s", response.Message)
	}
}

func TestInitUserRegistrationHandlerV2_MissingLastFlight(t *testing.T) {
	mockService := &mockRegistrationServiceV2{}
	handler := InitUserRegistrationHandlerV2(mockService)

	reqBody := dtos.InitUserRegistrationReq{
		IfcId:      "testuser",
		LastFlight: "", // Empty
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/user/register/init", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	claims := &auth.APIKeyClaims{
		DiscordUIDVal: "discord-123",
	}
	ctx := auth.SetUserClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestInitUserRegistrationHandlerV2_ServiceError(t *testing.T) {
	mockService := &mockRegistrationServiceV2{
		initUserRegistrationFunc: func(ctx context.Context, discordUserID, discordServerID, ifcId, lastFlight string, callsign *string) (*dtos.InitApiResponse, error) {
			return &dtos.InitApiResponse{
				IfcId:  ifcId,
				Status: false,
				Steps: []dtos.RegistrationStep{
					{Name: "duplicate_check", Status: false, Message: "User already exists"},
				},
			}, fmt.Errorf("DUPLICATE_USER: User already registered")
		},
	}

	handler := InitUserRegistrationHandlerV2(mockService)

	reqBody := dtos.InitUserRegistrationReq{
		IfcId:      "testuser",
		LastFlight: "KJFK-KLAX",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/user/register/init", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	claims := &auth.APIKeyClaims{
		DiscordUIDVal: "discord-123",
	}
	ctx := auth.SetUserClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var response dtos.APIResponse
	json.NewDecoder(rr.Body).Decode(&response)

	if response.Status != "error" {
		t.Errorf("Expected status error, got %s", response.Status)
	}
}
