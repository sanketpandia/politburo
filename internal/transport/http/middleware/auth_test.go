package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"infinite-experiment/politburo/internal/auth"
)

func TestAPIKeyAuthPassThroughWhenLookupNil(t *testing.T) {
	called := false
	handler := APIKeyAuth(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/x", nil))
	if !called || recorder.Code != http.StatusNoContent {
		t.Fatalf("pass-through failed: called=%v status=%d", called, recorder.Code)
	}
}

func TestAPIKeyAuthSkipsNonAPIPaths(t *testing.T) {
	lookup := apiKeyLookupFunc(func(*http.Request, string) (auth.Claims, bool, error) {
		t.Fatal("lookup should not run for non-api paths")
		return auth.Claims{}, false, nil
	})
	called := false
	handler := APIKeyAuth(lookup)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if !called || recorder.Code != http.StatusNoContent {
		t.Fatalf("health pass-through failed: called=%v status=%d", called, recorder.Code)
	}
}

func TestAPIKeyAuthRequiresKeyOnAPIPaths(t *testing.T) {
	lookup := apiKeyLookupFunc(func(_ *http.Request, key string) (auth.Claims, bool, error) {
		return auth.Claims{}, key == "good", nil
	})
	handler := APIKeyAuth(lookup)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/sessions/active", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/game/sessions/active", nil)
	req.Header.Set(APIKeyHeader, "good")
	req.Header.Set(DiscordUserIDHeader, "user")
	req.Header.Set(DiscordServerIDHeader, "server")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
}

type apiKeyLookupFunc func(*http.Request, string) (auth.Claims, bool, error)

func (f apiKeyLookupFunc) Lookup(r *http.Request, apiKey string) (auth.Claims, bool, error) {
	return f(r, apiKey)
}

func TestRequireDiscordBotContext(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequireDiscordBotContext()(next)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", recorder.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(DiscordUserIDHeader, "user")
	req.Header.Set(DiscordServerIDHeader, "server")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
}
