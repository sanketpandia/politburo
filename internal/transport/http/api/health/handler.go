package health

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/transport/http/response"
)

type Handler struct {
	db        *sql.DB
	redis     redisPinger
	startedAt time.Time
}

type redisPinger interface {
	Ping(context.Context) error
}

func NewHandler(db *sql.DB, redis redisPinger, startedAt time.Time) *Handler {
	return &Handler{db: db, redis: redis, startedAt: startedAt}
}

func (h *Handler) GetLiveness(w http.ResponseWriter, _ *http.Request) {
	response.WriteJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"started_at": h.startedAt,
		"uptime":     time.Since(h.startedAt).Round(time.Second).String(),
	})
}

func (h *Handler) GetReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		response.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":   "not_ready",
			"services": map[string]string{"postgres": "down"},
		})
		return
	}
	if err := h.redis.Ping(ctx); err != nil {
		response.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":   "not_ready",
			"services": map[string]string{"postgres": "ok", "redis": "down"},
		})
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{
		"status":   "ready",
		"services": map[string]string{"postgres": "ok", "redis": "ok"},
	})
}
