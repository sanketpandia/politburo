package memberships

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/httpdto"
	"infinite-experiment/politburo/internal/platform/roles"
)

func TestMain(m *testing.M) {
	_ = logging.Init("local")
	os.Exit(m.Run())
}

type fakeMembershipsService struct {
	getUserStatus func(ctx context.Context, userID string, vaID string) (*UserDetailResponse, error)
	joinVA        func(ctx context.Context, discordUserID string, discordServerID string, callsign string) (*JoinVAResponse, *MembershipError)
}

func (f *fakeMembershipsService) GetUserStatus(ctx context.Context, userID string, vaID string) (*UserDetailResponse, error) {
	return f.getUserStatus(ctx, userID, vaID)
}

func (f *fakeMembershipsService) JoinVA(ctx context.Context, discordUserID string, discordServerID string, callsign string) (*JoinVAResponse, *MembershipError) {
	return f.joinVA(ctx, discordUserID, discordServerID, callsign)
}

type fakeCallsignSampler struct {
	getSampleCallsigns func(ctx context.Context, vaID string, limit int) ([]string, error)
}

func (f *fakeCallsignSampler) GetSampleCallsigns(ctx context.Context, vaID string, limit int) ([]string, error) {
	return f.getSampleCallsigns(ctx, vaID, limit)
}

type fakeVAConfigReader struct {
	getAllConfigValues func(ctx context.Context, vaID string) (map[string]string, error)
}

func (f *fakeVAConfigReader) GetAllConfigValues(ctx context.Context, vaID string) (map[string]string, error) {
	return f.getAllConfigValues(ctx, vaID)
}

func TestJoinVA_MissingClaims(t *testing.T) {
	handler := NewHandler(&fakeMembershipsService{}, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memberships/join", bytes.NewBufferString(`{"callsign":"IFE123"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.JoinVA()(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestJoinVA_AlreadyMember(t *testing.T) {
	handler := NewHandler(&fakeMembershipsService{}, nil, nil)
	req := newJoinMembershipRequest(t, JoinVARequest{Callsign: "IFE123"}, &auth.APIKeyClaims{
		DiscordUIDVal:      "discord-user",
		DiscordServerIDVal: "discord-server",
		VaUUID:             "0d0e5756-5797-4a9d-8645-b6127e633922",
		RoleValue:          roles.RolePilot,
	})
	rr := httptest.NewRecorder()

	handler.JoinVA()(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestJoinVA_ValidationFailure(t *testing.T) {
	handler := NewHandler(&fakeMembershipsService{}, nil, nil)
	req := newJoinMembershipRequest(t, JoinVARequest{}, &auth.APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"})
	rr := httptest.NewRecorder()

	handler.JoinVA()(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
}

func TestJoinVA_CallsignNotInAirtableEnrichesMessage(t *testing.T) {
	handler := NewHandler(&fakeMembershipsService{
		joinVA: func(context.Context, string, string, string) (*JoinVAResponse, *MembershipError) {
			return nil, &MembershipError{Code: "CALLSIGN_NOT_IN_AIRTABLE", Message: "Callsign missing", StatusCode: http.StatusBadRequest}
		},
	}, &fakeCallsignSampler{
		getSampleCallsigns: func(context.Context, string, int) ([]string, error) {
			return []string{"IFE001", "IFE002"}, nil
		},
	}, &fakeVAConfigReader{
		getAllConfigValues: func(context.Context, string) (map[string]string, error) {
			return map[string]string{"airtable_va_base": "appBase123"}, nil
		},
	})
	req := newJoinMembershipRequest(t, JoinVARequest{Callsign: "IFE999"}, &auth.APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server", VaUUID: "0d0e5756-5797-4a9d-8645-b6127e633922"})
	rr := httptest.NewRecorder()

	handler.JoinVA()(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var response httpdto.Response[any]
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || !strings.Contains(response.Error.Message, "IFE001, IFE002") || !strings.Contains(response.Error.Message, "https://airtable.com/appBase123") {
		t.Fatalf("unexpected enriched error: %+v", response.Error)
	}
}

func TestJoinVA_Success(t *testing.T) {
	handler := NewHandler(&fakeMembershipsService{
		joinVA: func(ctx context.Context, discordUserID string, discordServerID string, callsign string) (*JoinVAResponse, *MembershipError) {
			if discordUserID != "discord-user" || discordServerID != "discord-server" || callsign != "IFE123" {
				t.Fatalf("unexpected join arguments: %q %q %q", discordUserID, discordServerID, callsign)
			}
			return &JoinVAResponse{
				Success:  true,
				Message:  "Successfully joined VA",
				UserID:   "ef2e5964-d290-46da-af3b-95df4ed1db88",
				VAID:     "0d0e5756-5797-4a9d-8645-b6127e633922",
				Callsign: "IFE123",
				Role:     "pilot",
			}, nil
		},
	}, nil, nil)
	req := newJoinMembershipRequest(t, JoinVARequest{Callsign: "IFE123"}, &auth.APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"})
	rr := httptest.NewRecorder()

	handler.JoinVA()(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var response httpdto.Response[JoinVAResponse]
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "ok" || !response.Result.Success || response.Result.Callsign != "IFE123" {
		t.Fatalf("unexpected success response: %+v", response)
	}
	if response.ResponseTime < 0 || response.ResponseTime > time.Second.Milliseconds() {
		t.Fatalf("unexpected response time: %d", response.ResponseTime)
	}
}

func newJoinMembershipRequest(t *testing.T, body JoinVARequest, claims auth.UserClaims) *http.Request {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memberships/join", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if claims != nil {
		req = req.WithContext(auth.SetUserClaims(req.Context(), claims))
	}
	return req
}
