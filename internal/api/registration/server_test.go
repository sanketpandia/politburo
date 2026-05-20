package registration

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
	registrationgen "infinite-experiment/politburo/internal/api/generated/registration"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/memberships"
	"infinite-experiment/politburo/internal/pilots"
	"infinite-experiment/politburo/internal/platform/httpdto"
	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/servers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) {
	_ = logging.Init("local")
	os.Exit(m.Run())
}

type pilotServiceStub struct{}

func (pilotServiceStub) RegisterPilot(context.Context, string, string, string, string) (*pilots.RegisterPilotResponse, *pilots.RegistrationError) {
	return &pilots.RegisterPilotResponse{Success: true, Message: "Pilot registered successfully", IsVARegistered: true}, nil
}

type membershipServiceStub struct{}

func (membershipServiceStub) GetUserStatus(context.Context, string, string, string, string) (*memberships.UserDetailResponse, error) {
	userID := uuid.MustParse("1cfa3e1e-5de1-4eff-ad0c-6fcb1bcd510a")
	vaID := uuid.MustParse("0d0e5756-5797-4a9d-8645-b6127e633922")
	now := time.Now().UTC().Round(time.Second)
	return &memberships.UserDetailResponse{
		IsRegistered:     true,
		GlobalUserExists: true,
		UserID:           userID.String(),
		DiscordID:        "discord-user",
		IFCommunityID:    "ifc-user",
		IsActive:         true,
		CreatedAt:        &now,
		Affiliations: []memberships.VAAffiliation{{
			VAID:     vaID.String(),
			VAName:   "Infinite",
			VACode:   "IFE",
			Role:     "pilot",
			IsActive: true,
			JoinedAt: now,
			Callsign: "IFE123",
		}},
		CurrentServer: memberships.CurrentServerStatus{DiscordServerID: "discord-server", IsConfiguredVA: true, VAID: vaID.String(), VAName: "Infinite", VACode: "IFE"},
		CurrentVA:     &memberships.CurrentVAStatus{IsMember: true, VAID: vaID.String(), VAName: "Infinite", VACode: "IFE", Role: "pilot", IsActive: true, Callsign: "IFE123"},
		MembershipsSummary: memberships.MembershipsSummary{
			TotalCount: 1, ActiveCount: 1,
		},
	}, nil
}

func (membershipServiceStub) JoinVA(context.Context, string, string, string) (*memberships.JoinVAResponse, *memberships.MembershipError) {
	userID := uuid.MustParse("1cfa3e1e-5de1-4eff-ad0c-6fcb1bcd510a")
	vaID := uuid.MustParse("0d0e5756-5797-4a9d-8645-b6127e633922")
	return &memberships.JoinVAResponse{Success: true, Message: "Successfully joined VA", UserID: userID.String(), VAID: vaID.String(), Callsign: "IFE123", Role: "pilot"}, nil
}

type serverServiceStub struct{}

func (serverServiceStub) InitServer(context.Context, string, string, string, string, string, string) (*servers.InitServerResponse, *servers.ServerError) {
	vaID := uuid.MustParse("0d0e5756-5797-4a9d-8645-b6127e633922")
	return &servers.InitServerResponse{Success: true, Message: "Server initialized successfully", VACode: "IFE", VAID: vaID.String()}, nil
}

type authServiceStub struct{}

func (authServiceStub) GetUserAndVAFromDiscordIDs(context.Context, string, string) (*users.User, auth.VAInfo, error) {
	return &users.User{ID: uuid.MustParse("1cfa3e1e-5de1-4eff-ad0c-6fcb1bcd510a").String()}, auth.VAInfo{ID: uuid.MustParse("0d0e5756-5797-4a9d-8645-b6127e633922").String()}, nil
}

func (authServiceStub) GenerateSignedLink(context.Context, string, string, string, time.Duration) (string, error) {
	return "signed-token", nil
}

func (authServiceStub) DestroyAllSessionsByIFCId(context.Context, string) (int, error) { return 0, nil }

func (authServiceStub) DeleteSession(context.Context, string) error { return nil }

type lookupStub struct{}

func (lookupStub) GetByDiscordID(context.Context, string) (*users.User, error) { return nil, nil }

func TestStrictServer_ComradeBotFlowEndpoints(t *testing.T) {
	t.Setenv("UI_BASE_URL", "https://viz.example.com")
	pilotsHandler := pilots.NewHandler(nil, pilotServiceStub{}, nil, lookupStub{})
	membershipsHandler := memberships.NewHandler(membershipServiceStub{}, nil, nil)
	serversHandler := servers.NewHandler(serverServiceStub{})
	authHandler := &auth.Handler{}
	authHandler = auth.NewHandler(authServiceStub{}, nil)

	strictServer := registrationgen.NewStrictHandler(NewServer(pilotsHandler, membershipsHandler, serversHandler, authHandler), nil)
	router := chi.NewRouter()
	registrationgen.HandlerFromMux(strictServer, router)

	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		claims     auth.UserClaims
		wantStatus int
	}{
		{name: "register pilot", method: http.MethodPost, path: "/pilots/register", body: map[string]string{"ifc_id": "ifc-user", "last_flight": "KJFK-KLAX"}, claims: apiKeyClaims("", "", false), wantStatus: http.StatusCreated},
		{name: "init server", method: http.MethodPost, path: "/server/init", body: map[string]string{"va_code": "IFE", "va_name": "Infinite", "callsign_prefix": "IFE"}, claims: apiKeyClaims("", "", false), wantStatus: http.StatusCreated},
		{name: "join membership", method: http.MethodPost, path: "/memberships/join", body: map[string]string{"callsign": "IFE123"}, claims: apiKeyClaims("", "", false), wantStatus: http.StatusCreated},
		{name: "user status", method: http.MethodGet, path: "/user/status", claims: apiKeyClaims(uuid.MustParse("1cfa3e1e-5de1-4eff-ad0c-6fcb1bcd510a").String(), roles.RolePilot.String(), true), wantStatus: http.StatusOK},
		{name: "signed link", method: http.MethodPost, path: "/signed-link", body: map[string]any{"redirect_to": "/dashboard", "ttl_minutes": 15}, claims: apiKeyClaims("", "", false), wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newComradeBotRequest(t, tt.method, tt.path, tt.body)
			if tt.claims != nil {
				req = req.WithContext(auth.SetUserClaims(req.Context(), tt.claims))
			}
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d (%s)", tt.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestStrictServer_UnauthorizedWithoutBotHeaders(t *testing.T) {
	pilotsHandler := pilots.NewHandler(nil, pilotServiceStub{}, nil, lookupStub{})
	membershipsHandler := memberships.NewHandler(membershipServiceStub{}, nil, nil)
	serversHandler := servers.NewHandler(serverServiceStub{})
	authHandler := auth.NewHandler(authServiceStub{}, nil)

	strictServer := registrationgen.NewStrictHandler(NewServer(pilotsHandler, membershipsHandler, serversHandler, authHandler), nil)
	router := chi.NewRouter()
	registrationgen.HandlerFromMux(strictServer, router)

	req := httptest.NewRequest(http.MethodPost, "/pilots/register", bytes.NewBufferString(`{"ifc_id":"ifc-user","last_flight":"KJFK-KLAX"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestStrictServer_ForbiddenDiscordContextMapsErrorEnvelope(t *testing.T) {
	forbiddenHandler := func(w http.ResponseWriter, r *http.Request) {
		httpdto.WriteError(w, time.Now(), "MISSING_DISCORD_CONTEXT", "Missing required Discord context headers: X-Discord-User-Id and X-Discord-Server-Id", http.StatusForbidden)
	}

	strictServer := registrationgen.NewStrictHandler(NewServerFromHandlers(Handlers{
		JoinMembership:     forbiddenHandler,
		RegisterPilot:      forbiddenHandler,
		InitServer:         forbiddenHandler,
		GenerateSignedLink: forbiddenHandler,
		GetUserStatus:      forbiddenHandler,
	}), nil)
	router := chi.NewRouter()
	registrationgen.HandlerFromMux(strictServer, router)

	tests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "register pilot", method: http.MethodPost, path: "/pilots/register", body: map[string]string{"ifc_id": "ifc-user", "last_flight": "KJFK-KLAX"}},
		{name: "init server", method: http.MethodPost, path: "/server/init", body: map[string]string{"va_code": "IFE", "va_name": "Infinite", "callsign_prefix": "IFE"}},
		{name: "join membership", method: http.MethodPost, path: "/memberships/join", body: map[string]string{"callsign": "IFE123"}},
		{name: "user status", method: http.MethodGet, path: "/user/status"},
		{name: "signed link", method: http.MethodPost, path: "/signed-link", body: map[string]any{"redirect_to": "/dashboard", "ttl_minutes": 15}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newComradeBotRequest(t, tt.method, tt.path, tt.body)
			req = req.WithContext(auth.SetUserClaims(req.Context(), apiKeyClaims("", "", false)))
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d (%s)", rr.Code, rr.Body.String())
			}
			var response registrationgen.ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error == nil || response.Error.Code != "MISSING_DISCORD_CONTEXT" {
				t.Fatalf("unexpected error response: %+v", response)
			}
		})
	}
}

func TestStrictServer_ValidationFailureMatchesHandlerEnvelope(t *testing.T) {
	pilotsHandler := pilots.NewHandler(nil, pilotServiceStub{}, nil, lookupStub{})
	membershipsHandler := memberships.NewHandler(membershipServiceStub{}, nil, nil)
	serversHandler := servers.NewHandler(serverServiceStub{})
	authHandler := auth.NewHandler(authServiceStub{}, nil)

	strictServer := registrationgen.NewStrictHandler(NewServer(pilotsHandler, membershipsHandler, serversHandler, authHandler), nil)
	router := chi.NewRouter()
	registrationgen.HandlerFromMux(strictServer, router)

	req := newComradeBotRequest(t, http.MethodPost, "/pilots/register", map[string]string{"last_flight": "KJFK-KLAX"})
	req = req.WithContext(auth.SetUserClaims(req.Context(), apiKeyClaims("", "", false)))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}

	var response registrationgen.ValidationErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != registrationgen.VALIDATIONFAILED {
		t.Fatalf("unexpected validation response: %+v", response)
	}
}

func newComradeBotRequest(t *testing.T, method string, path string, body any) *http.Request {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("X-API-Key", "test-api-key")
	req.Header.Set("X-Discord-User-Id", "discord-user")
	req.Header.Set("X-Discord-Server-Id", "discord-server")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func apiKeyClaims(userID string, role string, includeVA bool) auth.UserClaims {
	claims := &auth.APIKeyClaims{
		DiscordUIDVal:      "discord-user",
		DiscordServerIDVal: "discord-server",
		UserUUID:           userID,
	}
	if includeVA {
		claims.VaUUID = uuid.MustParse("0d0e5756-5797-4a9d-8645-b6127e633922").String()
	}
	if role != "" {
		claims.RoleValue = roles.VARole(role)
	}
	return claims
}
