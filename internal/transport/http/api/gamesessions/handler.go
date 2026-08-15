package gamesessions

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/cache"
	domainsessions "infinite-experiment/politburo/internal/game/sessions"
	"infinite-experiment/politburo/internal/infiniteflight"
	"infinite-experiment/politburo/internal/transport/http/api/cachedresponse"
	"infinite-experiment/politburo/internal/transport/http/response"
)

type Handler struct {
	cache cache.Store
}

func NewHandler(cacheStore cache.Store) *Handler {
	return &Handler{cache: cacheStore}
}

func (h *Handler) GetActiveSessions(w http.ResponseWriter, r *http.Request, history *bool) {
	snapshot := domainsessions.Snapshot{}
	if err := h.cache.GetJSON(r.Context(), cache.KeyActiveSessions, &snapshot); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, cache.ErrMiss) {
			status = http.StatusServiceUnavailable
			slog.Warn("active sessions cache miss", "error", err)
		} else {
			slog.Error("read active sessions cache", "error", err)
		}
		response.WriteError(w, status, "ACTIVE_SESSIONS_CACHE_UNAVAILABLE", "active sessions cache is unavailable")
		return
	}
	if snapshot.LastCached.IsZero() {
		slog.Error("read active sessions cache", "error", "lastCached is missing")
		response.WriteError(w, http.StatusInternalServerError, "ACTIVE_SESSIONS_CACHE_UNAVAILABLE", "active sessions cache is unavailable")
		return
	}

	includeHistory := history != nil && *history
	result := snapshot.Result
	if result == nil {
		result = make([]infiniteflight.Session, 0)
	}

	historyResult := make([]any, 0)
	if includeHistory {
		historyResult = make([]any, 0, len(snapshot.History))
		for _, historicalSnapshot := range snapshot.History {
			historicalSnapshot.History = nil
			historyResult = append(historyResult, historicalSnapshot)
		}
	}
	body := cachedresponse.Response[infiniteflight.Session]{
		Data: cachedresponse.Data[infiniteflight.Session]{
			AvailableFilters: []cachedresponse.Filter{{
				Name: "history", Type: "boolean", Desc: "Show last 50 records",
				Current: includeHistory, Default: false,
			}},
			Result:  result,
			History: historyResult,
			Meta: cachedresponse.Meta{
				LastCached:          snapshot.LastCached,
				RefreshIntervalMins: int(domainsessions.RefreshInterval / time.Minute),
			},
		},
	}
	response.WriteJSON(w, http.StatusOK, body)
}
