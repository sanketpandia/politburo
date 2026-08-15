package ui

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/session"
	appui "infinite-experiment/politburo/internal/ui"
)

type ticketConsumer interface {
	Consume(ctx context.Context, token string) (*auth.LoginTicket, error)
}

type sessionManager interface {
	Create(ctx context.Context, input session.CreateInput) (session.Session, error)
	Delete(ctx context.Context, sessionID string) error
}

type Handler struct {
	renderer *appui.Renderer
	sessions sessionManager
	tickets  ticketConsumer
}

func NewHandler(renderer *appui.Renderer, sessions sessionManager, tickets ticketConsumer) *Handler {
	return &Handler{renderer: renderer, sessions: sessions, tickets: tickets}
}

// Dashboard is a stub landing page for the rewrite UI surface.
func (h *Handler) Dashboard(w http.ResponseWriter, _ *http.Request) {
	if err := h.renderer.Render(w, "dashboard", map[string]any{
		"Title": "Politburo",
	}); err != nil {
		http.Error(w, "template render failed", http.StatusInternalServerError)
	}
}

// ActiveFlightsMap is the full-screen live flights map.
func (h *Handler) ActiveFlightsMap(w http.ResponseWriter, _ *http.Request) {
	if err := h.renderer.Render(w, "maps-active-flights", map[string]any{
		"Title": "Active flights",
	}); err != nil {
		http.Error(w, "template render failed", http.StatusInternalServerError)
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.renderExpired(w)
		return
	}
	ticket, err := h.tickets.Consume(r.Context(), token)
	if err != nil {
		if !errors.Is(err, auth.ErrInvalidTicket) {
			slog.Error("consume login ticket", "error", err)
		}
		h.renderExpired(w)
		return
	}
	created, err := h.sessions.Create(r.Context(), session.CreateInput{
		UserID:          ticket.UserID,
		DiscordID:       ticket.DiscordUserID,
		DiscordServerID: ticket.DiscordServerID,
		Username:        ticket.Username,
	})
	if err != nil {
		slog.Error("create session from login ticket", "error", err)
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}
	session.SetCookie(w, r, created.SessionID)
	http.Redirect(w, r, ticket.RedirectTo, http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.Delete(r.Context(), session.SessionIDFromRequest(r)); err != nil {
		slog.Error("delete session on logout", "error", err)
	}
	session.ClearCookie(w, r)
	http.Redirect(w, r, auth.DefaultRedirect, http.StatusSeeOther)
}

func (h *Handler) renderExpired(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	if err := h.renderer.Render(w, "auth-login", map[string]any{
		"Title": "Sign-in link expired",
	}); err != nil {
		slog.Error("render auth-login page", "error", err)
		http.Error(w, "sign-in unavailable", http.StatusInternalServerError)
	}
}

// Static serves embedded CSS/JS under /static/.
func Static() http.Handler {
	sub, err := fs.Sub(appui.Assets, "static")
	if err != nil {
		panic("ui static assets missing: " + err.Error())
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
}
