package session

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"infinite-experiment/politburo/internal/cache"
)

type memCache struct {
	data map[string][]byte
}

func newMemCache() *memCache {
	return &memCache{data: map[string][]byte{}}
}

func (c *memCache) GetJSON(_ context.Context, key string, dest any) error {
	raw, ok := c.data[key]
	if !ok {
		return cache.ErrMiss
	}
	return json.Unmarshal(raw, dest)
}

func (c *memCache) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.data[key] = raw
	return nil
}

func (c *memCache) Delete(_ context.Context, key string) error {
	delete(c.data, key)
	return nil
}

func TestCreateGetAndDeleteSession(t *testing.T) {
	store := newMemCache()
	svc := NewService(store)
	created, err := svc.Create(context.Background(), CreateInput{
		UserID: "user-1", DiscordID: "ds-1", DiscordServerID: "guild-1", Username: "pilot",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.SessionID == "" || created.UserID != "user-1" {
		t.Fatalf("session = %#v", created)
	}
	got, err := svc.Get(context.Background(), created.SessionID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.DiscordID != "ds-1" || got.DiscordServerID != "guild-1" {
		t.Fatalf("got = %#v", got)
	}
	claims := got.Claims()
	if claims.PbUserID != "user-1" || claims.DsUserID != "ds-1" || claims.DsServerID != "guild-1" || claims.Role != "" || claims.PbServerID != "" {
		t.Fatalf("claims = %#v", claims)
	}
	if err := svc.Delete(context.Background(), created.SessionID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := svc.Get(context.Background(), created.SessionID); !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("Get() after delete error = %v, want miss", err)
	}
}

func TestLookupFromCookie(t *testing.T) {
	svc := NewService(newMemCache())
	created, err := svc.Create(context.Background(), CreateInput{UserID: "user-1", DiscordID: "ds-1"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	claims, ok, err := svc.Lookup(req)
	if err != nil || ok {
		t.Fatalf("missing cookie: ok=%v err=%v claims=%#v", ok, err, claims)
	}

	req.AddCookie(&http.Cookie{Name: CookieName, Value: created.SessionID})
	claims, ok, err = svc.Lookup(req)
	if err != nil || !ok || claims.PbUserID != "user-1" {
		t.Fatalf("valid cookie: ok=%v err=%v claims=%#v", ok, err, claims)
	}
}

func TestExpiredSessionIsTreatedAsMiss(t *testing.T) {
	store := newMemCache()
	svc := NewService(store)
	svc.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	created, err := svc.Create(context.Background(), CreateInput{UserID: "user-1", DiscordID: "ds-1"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	svc.now = func() time.Time { return time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC) }
	if _, err := svc.Get(context.Background(), created.SessionID); err != cache.ErrMiss {
		t.Fatalf("Get() error = %v, want miss", err)
	}
}

func TestSetAndClearCookie(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/auth/login", nil)
	SetCookie(recorder, req, "sess-1")
	header := recorder.Header().Get("Set-Cookie")
	if !strings.Contains(header, "session_id=sess-1") || !strings.Contains(header, "Path=/") || !strings.Contains(header, "HttpOnly") {
		t.Fatalf("set cookie = %q", header)
	}
	if strings.Contains(strings.ToLower(header), "domain=") {
		t.Fatalf("cookie must not set Domain: %q", header)
	}
	if strings.Contains(strings.ToLower(header), "secure") {
		t.Fatalf("http cookie must not be Secure: %q", header)
	}

	httpsReq := httptest.NewRequest(http.MethodGet, "https://example.test/auth/login", nil)
	httpsReq.Header.Set("X-Forwarded-Proto", "https")
	recorder = httptest.NewRecorder()
	SetCookie(recorder, httpsReq, "sess-2")
	header = recorder.Header().Get("Set-Cookie")
	if !strings.Contains(strings.ToLower(header), "secure") {
		t.Fatalf("https cookie should be Secure: %q", header)
	}

	recorder = httptest.NewRecorder()
	ClearCookie(recorder, req)
	header = recorder.Header().Get("Set-Cookie")
	if !strings.Contains(header, "session_id=") || !strings.Contains(header, "Path=/") {
		t.Fatalf("clear cookie = %q", header)
	}
	if !strings.Contains(header, "Max-Age=0") && !strings.Contains(header, "Max-Age=-1") {
		t.Fatalf("clear cookie missing max-age: %q", header)
	}
}
