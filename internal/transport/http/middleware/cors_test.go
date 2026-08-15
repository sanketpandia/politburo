package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	handler := CORS([]string{"http://localhost:8081"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/game/sessions/active", nil)
	request.Header.Set("Origin", "http://localhost:8081")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8081" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestCORSHandlesPreflight(t *testing.T) {
	called := false
	handler := CORS([]string{"http://localhost:8081"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	request := httptest.NewRequest(http.MethodOptions, "/game/sessions/active", nil)
	request.Header.Set("Origin", "http://localhost:8081")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	request.Header.Set("Access-Control-Request-Headers", "x-api-key")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	if called {
		t.Fatal("preflight reached the endpoint handler")
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Fatalf("Access-Control-Allow-Methods = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "X-API-Key") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want X-API-Key", got)
	}
}

func TestCORSDoesNotAllowUnknownOrigin(t *testing.T) {
	handler := CORS([]string{"http://localhost:8081"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodGet, "/game/sessions/active", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}
