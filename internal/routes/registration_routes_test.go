package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/app"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/memberships"
	"infinite-experiment/politburo/internal/pilots"
	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/servers"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func init() {
	_ = logging.Init("local")
}

type registrationRoutePilotServiceStub struct{}

func (registrationRoutePilotServiceStub) RegisterPilot(context.Context, string, string, string, string) (*pilots.RegisterPilotResponse, *pilots.RegistrationError) {
	return &pilots.RegisterPilotResponse{Success: true, Message: "Pilot registered successfully", IsVARegistered: true}, nil
}

type registrationRouteMembershipServiceStub struct{}

func (registrationRouteMembershipServiceStub) GetUserStatus(context.Context, string, string, string, string) (*memberships.UserDetailResponse, error) {
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

func (registrationRouteMembershipServiceStub) JoinVA(context.Context, string, string, string) (*memberships.JoinVAResponse, *memberships.MembershipError) {
	userID := uuid.MustParse("1cfa3e1e-5de1-4eff-ad0c-6fcb1bcd510a")
	vaID := uuid.MustParse("0d0e5756-5797-4a9d-8645-b6127e633922")
	return &memberships.JoinVAResponse{Success: true, Message: "Successfully joined VA", UserID: userID.String(), VAID: vaID.String(), Callsign: "IFE123", Role: "pilot"}, nil
}

type registrationRouteServerServiceStub struct{}

func (registrationRouteServerServiceStub) InitServer(context.Context, string, string, string) (*servers.InitServerResponse, *servers.ServerError) {
	return &servers.InitServerResponse{Success: true, Message: "Server initialized successfully", VACode: "IFE", SetupRequired: true}, nil
}

type registrationRouteAuthServiceStub struct{}

func (registrationRouteAuthServiceStub) GetUserAndVAFromDiscordIDs(context.Context, string, string) (*users.User, auth.VAInfo, error) {
	return &users.User{ID: uuid.MustParse("1cfa3e1e-5de1-4eff-ad0c-6fcb1bcd510a").String()}, auth.VAInfo{ID: uuid.MustParse("0d0e5756-5797-4a9d-8645-b6127e633922").String()}, nil
}

func (registrationRouteAuthServiceStub) GenerateSignedLink(context.Context, string, string, string, time.Duration) (string, error) {
	return "signed-token", nil
}

func (registrationRouteAuthServiceStub) DestroyAllSessionsByIFCId(context.Context, string) (int, error) {
	return 0, nil
}

func (registrationRouteAuthServiceStub) DeleteSession(context.Context, string) error { return nil }

type registrationRouteLookupStub struct{}

func (registrationRouteLookupStub) GetByDiscordID(context.Context, string) (*users.User, error) {
	return nil, nil
}

func TestRegisterRegistrationRoutesMountsGeneratedUserRegister(t *testing.T) {
	t.Setenv("UI_BASE_URL", "https://viz.example.com")
	router := chi.NewRouter()
	registerRegistrationRoutes(router, registrationRouteApp())

	tests := []struct {
		name       string
		method     string
		path       string
		body       any
		claims     auth.UserClaims
		wantStatus int
	}{
		{name: "register user", method: http.MethodPost, path: "/user/register", body: map[string]string{"ifc_id": "ifc-user", "last_flight": "KJFK-KLAX"}, claims: registrationRouteClaims("", "", false), wantStatus: http.StatusCreated},
		{name: "init server", method: http.MethodPost, path: "/server/init", body: map[string]string{"va_code": "IFE"}, claims: registrationRouteClaims("", "", false), wantStatus: http.StatusCreated},
		{name: "join membership", method: http.MethodPost, path: "/memberships/join", body: map[string]string{"callsign": "IFE123"}, claims: registrationRouteClaims("", "", false), wantStatus: http.StatusCreated},
		{name: "user status", method: http.MethodGet, path: "/user/status", claims: registrationRouteClaims(uuid.MustParse("1cfa3e1e-5de1-4eff-ad0c-6fcb1bcd510a").String(), roles.RolePilot.String(), true), wantStatus: http.StatusOK},
		{name: "signed link", method: http.MethodPost, path: "/signed-link", body: map[string]any{"redirect_to": "/dashboard", "ttl_minutes": 15}, claims: registrationRouteClaims("", "", false), wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := registrationRouteRequest(t, tt.method, tt.path, tt.body)
			req = req.WithContext(auth.SetUserClaims(req.Context(), tt.claims))
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d (%s)", tt.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestRegisterRegistrationRoutesDoesNotMountLegacyPilotRegister(t *testing.T) {
	router := chi.NewRouter()
	registerRegistrationRoutes(router, registrationRouteApp())

	req := registrationRouteRequest(t, http.MethodPost, "/pilots/register", map[string]string{"ifc_id": "ifc-user", "last_flight": "KJFK-KLAX"})
	req = req.WithContext(auth.SetUserClaims(req.Context(), registrationRouteClaims("", "", false)))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected legacy /pilots/register to be unmounted with 404, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func registrationRouteApp() *app.App {
	return &app.App{Features: app.FeatureDeps{
		AuthHandler:        auth.NewHandler(registrationRouteAuthServiceStub{}, nil),
		MembershipsHandler: memberships.NewHandler(registrationRouteMembershipServiceStub{}, nil, nil),
		PilotsHandler:      pilots.NewHandler(nil, registrationRoutePilotServiceStub{}, nil, registrationRouteLookupStub{}),
		ServersHandler:     servers.NewHandler(registrationRouteServerServiceStub{}),
	}}
}

func registrationRouteRequest(t *testing.T, method string, path string, body any) *http.Request {
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

func registrationRouteClaims(userID string, role string, includeVA bool) auth.UserClaims {
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
