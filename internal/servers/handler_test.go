package servers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/httpdto"
)

func TestMain(m *testing.M) {
	_ = logging.Init("local")
	os.Exit(m.Run())
}

type fakeServerRegistrationService struct {
	initServer func(ctx context.Context, discordServerID string, discordUserID string, vaCode string) (*InitServerResponse, *ServerError)
}

func (f *fakeServerRegistrationService) InitServer(ctx context.Context, discordServerID string, discordUserID string, vaCode string) (*InitServerResponse, *ServerError) {
	return f.initServer(ctx, discordServerID, discordUserID, vaCode)
}

func TestInitServer_MissingClaims(t *testing.T) {
	handler := NewHandler(&fakeServerRegistrationService{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/init", bytes.NewBufferString(`{"va_code":"IFE"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.InitServer()(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestInitServer_MissingVACode(t *testing.T) {
	handler := NewHandler(&fakeServerRegistrationService{})
	req := newInitServerRequest(t, InitServerRequest{}, &auth.APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"})
	rr := httptest.NewRecorder()

	handler.InitServer()(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestInitServer_ServerAlreadyRegistered(t *testing.T) {
	handler := NewHandler(&fakeServerRegistrationService{
		initServer: func(context.Context, string, string, string) (*InitServerResponse, *ServerError) {
			return nil, &ServerError{Code: "SERVER_ALREADY_REGISTERED", Message: "already registered", StatusCode: http.StatusConflict}
		},
	})
	req := newInitServerRequest(t, InitServerRequest{VACode: "IFE"}, &auth.APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"})
	rr := httptest.NewRecorder()

	handler.InitServer()(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestInitServer_UserNotRegistered(t *testing.T) {
	handler := NewHandler(&fakeServerRegistrationService{
		initServer: func(context.Context, string, string, string) (*InitServerResponse, *ServerError) {
			return nil, &ServerError{Code: "USER_NOT_REGISTERED", Message: "register first", StatusCode: http.StatusBadRequest}
		},
	})
	req := newInitServerRequest(t, InitServerRequest{VACode: "IFE"}, &auth.APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"})
	rr := httptest.NewRecorder()

	handler.InitServer()(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestInitServer_Success(t *testing.T) {
	handler := NewHandler(&fakeServerRegistrationService{
		initServer: func(ctx context.Context, discordServerID string, discordUserID string, vaCode string) (*InitServerResponse, *ServerError) {
			if discordServerID != "discord-server" || discordUserID != "discord-user" || vaCode != "IFE" {
				t.Fatalf("unexpected service arguments: %q %q %q", discordServerID, discordUserID, vaCode)
			}
			return &InitServerResponse{Success: true, Message: "Server initialized successfully", VACode: vaCode, SetupRequired: true}, nil
		},
	})
	req := newInitServerRequest(t, InitServerRequest{VACode: "IFE"}, &auth.APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"})
	rr := httptest.NewRecorder()

	handler.InitServer()(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var response httpdto.Response[InitServerResponse]
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" || !response.Result.Success || response.Result.VACode != "IFE" || !response.Result.SetupRequired {
		t.Fatalf("unexpected success response: %+v", response)
	}
}

func newInitServerRequest(t *testing.T, body InitServerRequest, claims auth.UserClaims) *http.Request {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/init", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if claims != nil {
		req = req.WithContext(auth.SetUserClaims(req.Context(), claims))
	}
	return req
}
