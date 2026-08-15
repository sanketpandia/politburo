package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/session"
	appui "infinite-experiment/politburo/internal/ui"
)

type ticketConsumerStub struct {
	ticket *auth.LoginTicket
	err    error
	calls  int
}

func (s *ticketConsumerStub) Consume(context.Context, string) (*auth.LoginTicket, error) {
	s.calls++
	return s.ticket, s.err
}

type sessionManagerStub struct {
	created session.Session
	create  error
	deleted string
}

func (s *sessionManagerStub) Create(context.Context, session.CreateInput) (session.Session, error) {
	return s.created, s.create
}

func (s *sessionManagerStub) Delete(_ context.Context, sessionID string) error {
	s.deleted = sessionID
	return nil
}

func newUIHandler(t *testing.T, sessions sessionManager, tickets ticketConsumer) *Handler {
	t.Helper()
	renderer, err := appui.NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	return NewHandler(renderer, sessions, tickets)
}

func TestLoginConsumesTicketThenSetsCookie(t *testing.T) {
	tickets := &ticketConsumerStub{ticket: &auth.LoginTicket{
		UserID: "u1", DiscordUserID: "ds-1", RedirectTo: "/maps/flights/active",
	}}
	sessions := &sessionManagerStub{created: session.Session{SessionID: "sess-abc"}}
	handler := newUIHandler(t, sessions, tickets)

	req := httptest.NewRequest(http.MethodGet, "/auth/login?token=once", nil)
	recorder := httptest.NewRecorder()
	handler.Login(recorder, req)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", recorder.Code)
	}
	if loc := recorder.Header().Get("Location"); loc != "/maps/flights/active" {
		t.Fatalf("location = %q", loc)
	}
	cookie := recorder.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "session_id=sess-abc") {
		t.Fatalf("set-cookie = %q", cookie)
	}
	if tickets.calls != 1 {
		t.Fatalf("consume calls = %d", tickets.calls)
	}
}

func TestLoginReplayRendersExpired(t *testing.T) {
	handler := newUIHandler(t, &sessionManagerStub{}, &ticketConsumerStub{err: auth.ErrInvalidTicket})
	req := httptest.NewRequest(http.MethodGet, "/auth/login?token=used", nil)
	recorder := httptest.NewRecorder()
	handler.Login(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if cookie := recorder.Header().Get("Set-Cookie"); strings.Contains(cookie, "session_id=sess") {
		t.Fatalf("expired login must not set session cookie: %q", cookie)
	}
}

func TestActiveFlightsMapRenders(t *testing.T) {
	handler := newUIHandler(t, &sessionManagerStub{}, &ticketConsumerStub{})
	req := httptest.NewRequest(http.MethodGet, "/maps/flights/active", nil)
	recorder := httptest.NewRecorder()
	handler.ActiveFlightsMap(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `id="leaflet-map"`) || !strings.Contains(body, `id="maps-filters"`) {
		t.Fatalf("missing map shell: %s", body)
	}
	if !strings.Contains(body, "/static/js/maps/active-flights.mjs") {
		t.Fatalf("missing page script: %s", body)
	}
}

func TestLogoutDeletesSessionAndCookie(t *testing.T) {
	sessions := &sessionManagerStub{}
	handler := newUIHandler(t, sessions, &ticketConsumerStub{})
	req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "sess-abc"})
	recorder := httptest.NewRecorder()
	handler.Logout(recorder, req)
	if sessions.deleted != "sess-abc" {
		t.Fatalf("deleted = %q", sessions.deleted)
	}
	cookie := recorder.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "session_id=") || (!strings.Contains(cookie, "Max-Age=0") && !strings.Contains(cookie, "Max-Age=-1")) {
		t.Fatalf("set-cookie = %q", cookie)
	}
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", recorder.Code)
	}
}
