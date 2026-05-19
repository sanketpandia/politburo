package auth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/templates"
	platformUsers "infinite-experiment/politburo/internal/platform/users"
)

func TestMain(m *testing.M) {
	_ = logging.Init("local")
	os.Exit(m.Run())
}

type stubTokenSvc struct {
	result *CreateSessionFromTokenResult
	err    error
}

func (s *stubTokenSvc) CreateSessionFromToken(_ context.Context, _ string) (*CreateSessionFromTokenResult, error) {
	return s.result, s.err
}

type fakeHandlerService struct {
	getUserAndVAFromDiscordIDs func(ctx context.Context, discordUserID string, discordServerID string) (*platformUsers.User, VAInfo, error)
	generateSignedLink         func(ctx context.Context, userID string, vaID string, redirectTo string, ttl time.Duration) (string, error)
	destroyAllSessionsByIFCId  func(ctx context.Context, ifcId string) (int, error)
	deleteSession              func(ctx context.Context, sessionID string) error
}

func (f *fakeHandlerService) GetUserAndVAFromDiscordIDs(ctx context.Context, discordUserID string, discordServerID string) (*platformUsers.User, VAInfo, error) {
	return f.getUserAndVAFromDiscordIDs(ctx, discordUserID, discordServerID)
}

func (f *fakeHandlerService) GenerateSignedLink(ctx context.Context, userID string, vaID string, redirectTo string, ttl time.Duration) (string, error) {
	return f.generateSignedLink(ctx, userID, vaID, redirectTo, ttl)
}

func (f *fakeHandlerService) DestroyAllSessionsByIFCId(ctx context.Context, ifcId string) (int, error) {
	if f.destroyAllSessionsByIFCId == nil {
		return 0, nil
	}
	return f.destroyAllSessionsByIFCId(ctx, ifcId)
}

func (f *fakeHandlerService) DeleteSession(ctx context.Context, sessionID string) error {
	if f.deleteSession == nil {
		return nil
	}
	return f.deleteSession(ctx, sessionID)
}

func newTestRenderer(_ *testing.T) *templates.Renderer {
	return templates.NewRenderer(
		"templates",
		"templates/partials",
		"templates/layouts/base.html",
	)
}

func newTestHandler(tokenSvc tokenSessionCreator, svc handlerService, renderer *templates.Renderer) *Handler {
	return &Handler{svc: svc, tokenSvc: tokenSvc, renderer: renderer}
}

func TestShortToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"abc", "abc"},
		{"abcdef", "abcdef"},
		{"abcdefghij", "abcdef"},
	}
	for _, tc := range tests {
		if got := shortToken(tc.input); got != tc.want {
			t.Errorf("shortToken(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestTokenLogin_EmptyToken(t *testing.T) {
	renderer := newTestRenderer(t)
	h := newTestHandler(nil, nil, renderer)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rr := httptest.NewRecorder()

	h.TokenLogin()(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", ct)
	}
	if !strings.Contains(rr.Body.String(), `id="auth-login-page"`) {
		t.Errorf("expected auth-login page marker in body")
	}
	if !strings.Contains(rr.Body.String(), `data-state="expired"`) {
		t.Errorf("expected data-state=expired in body")
	}
}

func TestTokenLogin_InvalidToken(t *testing.T) {
	renderer := newTestRenderer(t)
	stub := &stubTokenSvc{err: errors.New("invalid or expired token")}
	h := newTestHandler(stub, nil, renderer)

	req := httptest.NewRequest(http.MethodGet, "/auth/login?token=ABCDEFGHIJ", nil)
	rr := httptest.NewRecorder()

	h.TokenLogin()(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `data-state="expired"`) {
		t.Errorf("expected data-state=expired in body")
	}
	if !strings.Contains(body, "ABCDEF") {
		t.Errorf("expected truncated token prefix ABCDEF in body")
	}
}

func TestTokenLogin_Success(t *testing.T) {
	renderer := newTestRenderer(t)
	stub := &stubTokenSvc{result: &CreateSessionFromTokenResult{SessionID: "sess-abc", RedirectTo: "/dashboard"}}
	h := newTestHandler(stub, nil, renderer)

	req := httptest.NewRequest(http.MethodGet, "/auth/login?token=validtoken", nil)
	req.Host = "localhost"
	rr := httptest.NewRecorder()

	h.TokenLogin()(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %q", loc)
	}
	if cookies := rr.Header().Get("Set-Cookie"); !strings.Contains(cookies, "session_id=sess-abc") {
		t.Errorf("expected session_id cookie, got %q", cookies)
	}
	if strings.Contains(rr.Body.String(), `id="auth-login-page"`) {
		t.Errorf("renderer should not have been invoked on successful token login")
	}
}

func TestGetUIBaseURL_EnvOverride(t *testing.T) {
	t.Setenv("UI_BASE_URL", "https://ui.example.com")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "ignored.example.com"
	req.Header.Set("X-Forwarded-Host", "forwarded.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := GetUIBaseURL(req); got != "https://ui.example.com" {
		t.Fatalf("GetUIBaseURL() = %q, want %q", got, "https://ui.example.com")
	}
}

func TestGetUIBaseURL_FromForwardedHeaders(t *testing.T) {
	t.Setenv("UI_BASE_URL", "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "internal.example.com:8080"
	req.Header.Set("X-Forwarded-Host", "app.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := GetUIBaseURL(req); got != "https://app.example.com" {
		t.Fatalf("GetUIBaseURL() = %q, want %q", got, "https://app.example.com")
	}
}

func TestGetUIBaseURL_FromTLSFallback(t *testing.T) {
	t.Setenv("UI_BASE_URL", "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "secure.example.com"
	req.TLS = &tls.ConnectionState{}
	if got := GetUIBaseURL(req); got != "https://secure.example.com" {
		t.Fatalf("GetUIBaseURL() = %q, want %q", got, "https://secure.example.com")
	}
}

func TestFormatSignedLinkURL(t *testing.T) {
	got := FormatSignedLinkURL("https://ui.example.com", "abc123")
	if got != "https://ui.example.com/auth/login?token=abc123" {
		t.Fatalf("FormatSignedLinkURL() = %q", got)
	}
}

func TestVerifyGodMode_MissingClaims(t *testing.T) {
	renderer := newTestRenderer(t)
	h := newTestHandler(nil, nil, renderer)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/verify-god", nil)
	rr := httptest.NewRecorder()

	h.VerifyGodMode()(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}

	var body struct {
		Status string `json:"status"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Status != "error" || body.Error.Code != "UNAUTHORIZED" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestVerifyGodMode_Success(t *testing.T) {
	t.Setenv("GOD_MODE", "discord-user-1")
	t.Setenv("GOD_MODE_KEY", "god-key")
	renderer := newTestRenderer(t)
	h := newTestHandler(nil, nil, renderer)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/verify-god", nil)
	req.Header.Set("X-God-Mode-Key", "god-key")
	req = req.WithContext(SetUserClaims(req.Context(), &APIKeyClaims{DiscordUIDVal: "discord-user-1"}))
	rr := httptest.NewRecorder()

	h.VerifyGodMode()(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var body struct {
		Status string `json:"status"`
		Result struct {
			IsGod bool `json:"is_god"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Status != "ok" || !body.Result.IsGod {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestVerifyGodMode_False(t *testing.T) {
	t.Setenv("GOD_MODE", "discord-user-1")
	t.Setenv("GOD_MODE_KEY", "god-key")
	renderer := newTestRenderer(t)
	h := newTestHandler(nil, nil, renderer)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/verify-god", nil)
	req.Header.Set("X-God-Mode-Key", "wrong-key")
	req = req.WithContext(SetUserClaims(req.Context(), &APIKeyClaims{DiscordUIDVal: "discord-user-2"}))
	rr := httptest.NewRecorder()

	h.VerifyGodMode()(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var body struct {
		Result struct {
			IsGod bool `json:"is_god"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body.Result.IsGod {
		t.Fatalf("expected is_god=false")
	}
}

func TestGenerateSignedLink_Defaults(t *testing.T) {
	t.Setenv("UI_BASE_URL", "https://viz.example.com")
	renderer := newTestRenderer(t)
	h := newTestHandler(nil, &fakeHandlerService{
		getUserAndVAFromDiscordIDs: func(context.Context, string, string) (*platformUsers.User, VAInfo, error) {
			return &platformUsers.User{ID: "user-uuid"}, VAInfo{ID: "va-uuid"}, nil
		},
		generateSignedLink: func(ctx context.Context, userID string, vaID string, redirectTo string, ttl time.Duration) (string, error) {
			if userID != "user-uuid" || vaID != "va-uuid" || redirectTo != "/dashboard" || ttl != 15*time.Minute {
				t.Fatalf("unexpected service args: %q %q %q %v", userID, vaID, redirectTo, ttl)
			}
			return "signed-token", nil
		},
	}, renderer)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/signed-link", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(SetUserClaims(req.Context(), &APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"}))
	rr := httptest.NewRecorder()

	h.GenerateSignedLink()(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var response struct {
		Result struct {
			URL        string `json:"url"`
			ExpiresIn  int    `json:"expires_in"`
			RedirectTo string `json:"redirect_to"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if response.Result.URL != "https://viz.example.com/auth/login?token=signed-token" || response.Result.ExpiresIn != 900 || response.Result.RedirectTo != "/dashboard" {
		t.Fatalf("unexpected result: %+v", response.Result)
	}
}

func TestGenerateSignedLink_LookupFailure(t *testing.T) {
	renderer := newTestRenderer(t)
	h := newTestHandler(nil, &fakeHandlerService{
		getUserAndVAFromDiscordIDs: func(context.Context, string, string) (*platformUsers.User, VAInfo, error) {
			return nil, VAInfo{}, errors.New("missing user")
		},
		generateSignedLink: func(context.Context, string, string, string, time.Duration) (string, error) {
			return "", nil
		},
	}, renderer)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/signed-link", strings.NewReader(`{"redirect_to":"/dashboard"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(SetUserClaims(req.Context(), &APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"}))
	rr := httptest.NewRecorder()

	h.GenerateSignedLink()(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGenerateSignedLink_GenerationFailure(t *testing.T) {
	renderer := newTestRenderer(t)
	h := newTestHandler(nil, &fakeHandlerService{
		getUserAndVAFromDiscordIDs: func(context.Context, string, string) (*platformUsers.User, VAInfo, error) {
			return &platformUsers.User{ID: "user-uuid"}, VAInfo{ID: "va-uuid"}, nil
		},
		generateSignedLink: func(context.Context, string, string, string, time.Duration) (string, error) {
			return "", errors.New("signing failed")
		},
	}, renderer)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/signed-link", strings.NewReader(`{"redirect_to":"/dashboard","ttl_minutes":30}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(SetUserClaims(req.Context(), &APIKeyClaims{DiscordUIDVal: "discord-user", DiscordServerIDVal: "discord-server"}))
	rr := httptest.NewRecorder()

	h.GenerateSignedLink()(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}
