package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"infinite-experiment/politburo/internal/auth"
)

type sessionLookupFunc func(*http.Request) (auth.Claims, bool, error)

func (f sessionLookupFunc) Lookup(r *http.Request) (auth.Claims, bool, error) {
	return f(r)
}

func TestUISessionAuthPassThroughWhenNil(t *testing.T) {
	called := false
	handler := UISessionAuth(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if !called || recorder.Code != http.StatusNoContent {
		t.Fatalf("pass-through failed: called=%v status=%d", called, recorder.Code)
	}
}

func TestUISessionAuthRedirectsWhenMissing(t *testing.T) {
	handler := UISessionAuth(sessionLookupFunc(func(*http.Request) (auth.Claims, bool, error) {
		return auth.Claims{}, false, nil
	}))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/auth/login" {
		t.Fatalf("status=%d location=%q", recorder.Code, recorder.Header().Get("Location"))
	}
}

func TestUISessionAuthSetsClaims(t *testing.T) {
	handler := UISessionAuth(sessionLookupFunc(func(*http.Request) (auth.Claims, bool, error) {
		return auth.Claims{PbUserID: "user-1"}, true, nil
	}))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims.PbUserID != "user-1" {
			t.Fatalf("claims = %#v ok=%v", claims, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestUISessionAuthLookupError(t *testing.T) {
	handler := UISessionAuth(sessionLookupFunc(func(*http.Request) (auth.Claims, bool, error) {
		return auth.Claims{}, false, errors.New("redis down")
	}))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
}
