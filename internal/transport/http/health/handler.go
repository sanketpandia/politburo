package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

type Handler struct {
	db        *sql.DB
	startedAt time.Time
}

func NewHandler(db *sql.DB, startedAt time.Time) *Handler {
	return &Handler{db: db, startedAt: startedAt}
}

func (h *Handler) GetLiveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"started_at": h.startedAt,
		"uptime":     time.Since(h.startedAt).Round(time.Second).String(),
	})
}

func (h *Handler) GetReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":   "not_ready",
			"services": map[string]string{"postgres": "down"},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ready",
		"services": map[string]string{"postgres": "ok"},
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
