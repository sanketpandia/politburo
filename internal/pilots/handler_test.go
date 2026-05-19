package pilots

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/httpdto"
	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/platform/users"
)

func TestMain(m *testing.M) {
	_ = logging.Init("local")
	os.Exit(m.Run())
}

type fakeRegistrationHandlerService struct {
	registerPilot func(ctx context.Context, discordUserID string, discordServerID string, ifcId string, lastFlight string) (*RegisterPilotResponse, *RegistrationError)
}

func (f *fakeRegistrationHandlerService) RegisterPilot(ctx context.Context, discordUserID string, discordServerID string, ifcId string, lastFlight string) (*RegisterPilotResponse, *RegistrationError) {
	return f.registerPilot(ctx, discordUserID, discordServerID, ifcId, lastFlight)
}

type fakeLogbookUserLookup struct{}

func (fakeLogbookUserLookup) GetByDiscordID(context.Context, string) (*users.User, error) {
	return nil, nil
}

func TestRegisterPilot_MissingClaims(t *testing.T) {
	handler := NewHandler(nil, &fakeRegistrationHandlerService{}, nil, fakeLogbookUserLookup{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pilots/register", bytes.NewBufferString(`{"ifc_id":"ifc-user","last_flight":"KJFK-KLAX"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.RegisterPilot()(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	var response httpdto.Response[any]
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "UNAUTHORIZED" {
		t.Fatalf("unexpected error response: %+v", response.Error)
	}
}

func TestRegisterPilot_ValidationFailure(t *testing.T) {
	handler := NewHandler(nil, &fakeRegistrationHandlerService{}, nil, fakeLogbookUserLookup{})
	req := newPilotRequest(t, RegisterPilotRequest{LastFlight: "KJFK-KLAX"}, &auth.APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"})
	rr := httptest.NewRecorder()

	handler.RegisterPilot()(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}

	var response struct {
		Status string `json:"status"`
		Error  struct {
			Code   string `json:"code"`
			Fields []struct {
				Field string `json:"field"`
			} `json:"fields"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error.Code != "VALIDATION_FAILED" || len(response.Error.Fields) != 1 || response.Error.Fields[0].Field != "ifc_id" {
		t.Fatalf("unexpected validation response: %+v", response)
	}
}

func TestRegisterPilot_AlreadyRegisteredFromClaims(t *testing.T) {
	handler := NewHandler(nil, &fakeRegistrationHandlerService{}, nil, fakeLogbookUserLookup{})
	req := newPilotRequest(t, RegisterPilotRequest{IfcId: "ifc-user", LastFlight: "KJFK-KLAX"}, &auth.APIKeyClaims{
		UserUUID:           "user-uuid",
		DiscordUIDVal:      "discord-user",
		DiscordServerIDVal: "discord-server",
	})
	rr := httptest.NewRecorder()

	handler.RegisterPilot()(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}

	var response httpdto.Response[any]
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "USER_ALREADY_REGISTERED" {
		t.Fatalf("unexpected error response: %+v", response.Error)
	}
}

func TestRegisterPilot_Success(t *testing.T) {
	handler := NewHandler(nil, &fakeRegistrationHandlerService{
		registerPilot: func(ctx context.Context, discordUserID string, discordServerID string, ifcId string, lastFlight string) (*RegisterPilotResponse, *RegistrationError) {
			if discordUserID != "discord-user" || discordServerID != "discord-server" || ifcId != "ifc-user" || lastFlight != "KJFK-KLAX" {
				t.Fatalf("unexpected service arguments: %q %q %q %q", discordUserID, discordServerID, ifcId, lastFlight)
			}
			return &RegisterPilotResponse{Success: true, Message: "Pilot registered successfully", IsVARegistered: true}, nil
		},
	}, nil, fakeLogbookUserLookup{})
	req := newPilotRequest(t, RegisterPilotRequest{IfcId: "ifc-user", LastFlight: "KJFK-KLAX"}, &auth.APIKeyClaims{
		DiscordUIDVal:      "discord-user",
		DiscordServerIDVal: "discord-server",
		RoleValue:          roles.RolePilot,
	})
	rr := httptest.NewRecorder()

	handler.RegisterPilot()(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var response httpdto.Response[RegisterPilotResponse]
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" || !response.Result.Success || !response.Result.IsVARegistered {
		t.Fatalf("unexpected success response: %+v", response)
	}
}

func newPilotRequest(t *testing.T, body RegisterPilotRequest, claims auth.UserClaims) *http.Request {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pilots/register", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if claims != nil {
		req = req.WithContext(auth.SetUserClaims(req.Context(), claims))
	}
	return req
}

func TestRegistrationErrorJSONEnvelopeResponseTime(t *testing.T) {
	handler := NewHandler(nil, &fakeRegistrationHandlerService{
		registerPilot: func(context.Context, string, string, string, string) (*RegisterPilotResponse, *RegistrationError) {
			return nil, &RegistrationError{Code: "REGISTRATION_FAILED", Message: "Failed", StatusCode: http.StatusInternalServerError}
		},
	}, nil, fakeLogbookUserLookup{})
	req := newPilotRequest(t, RegisterPilotRequest{IfcId: "ifc-user", LastFlight: "KJFK-KLAX"}, &auth.APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"})
	rr := httptest.NewRecorder()
	before := time.Now()

	handler.RegisterPilot()(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	var response httpdto.Response[any]
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ResponseTime < 0 || response.ResponseTime > time.Since(before).Milliseconds()+1000 {
		t.Fatalf("unexpected responseTimeMs: %d", response.ResponseTime)
	}
}
