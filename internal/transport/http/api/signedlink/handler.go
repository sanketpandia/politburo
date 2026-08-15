package signedlink

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/transport/http/response"
	"infinite-experiment/politburo/internal/users"
)

type userLookup interface {
	GetByDiscordID(ctx context.Context, discordID string) (*users.User, error)
}

type ticketIssuer interface {
	Issue(ctx context.Context, ticket auth.LoginTicket) (string, error)
}

type Handler struct {
	users     userLookup
	tickets   ticketIssuer
	uiBaseURL string
}

func NewHandler(users userLookup, tickets ticketIssuer, uiBaseURL string) *Handler {
	return &Handler{users: users, tickets: tickets, uiBaseURL: uiBaseURL}
}

type requestBody struct {
	RedirectTo string `json:"redirectTo"`
}

func (h *Handler) GenerateSignedLink(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
		return
	}
	if claims.DsUserID == "" {
		response.WriteError(w, http.StatusForbidden, "MISSING_DISCORD_CONTEXT", "Missing required Discord context header: X-Discord-User-Id")
		return
	}

	var body requestBody
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&body); err != nil && err != io.EOF {
			response.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
			return
		}
	}
	redirectTo, err := auth.NormalizeRedirect(body.RedirectTo)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_REDIRECT", "redirectTo must be a relative path")
		return
	}

	user, err := h.users.GetByDiscordID(r.Context(), claims.DsUserID)
	if err != nil {
		slog.Error("lookup user for signed link", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "USER_LOOKUP_FAILED", "user lookup failed")
		return
	}
	if user == nil {
		response.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}

	token, err := h.tickets.Issue(r.Context(), auth.LoginTicket{
		UserID:          user.ID,
		DiscordUserID:   user.DiscordID,
		DiscordServerID: claims.DsServerID,
		Username:        user.DisplayName(),
		RedirectTo:      redirectTo,
	})
	if err != nil {
		slog.Error("issue signed link ticket", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "GENERATION_FAILED", "failed to generate signed link")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"url":        auth.FormatLoginURL(h.uiBaseURL, token),
			"expiresIn":  int(auth.TicketTTL.Seconds()),
			"redirectTo": redirectTo,
		},
	})
}
