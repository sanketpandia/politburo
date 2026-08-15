package gamesessions

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"infinite-experiment/politburo/internal/cache"
	domainsessions "infinite-experiment/politburo/internal/game/sessions"
	"infinite-experiment/politburo/internal/infiniteflight"
)

type cacheStub struct {
	snapshot domainsessions.Snapshot
	err      error
}

func (s cacheStub) GetJSON(_ context.Context, key string, destination any) error {
	if key != cache.KeyActiveSessions {
		return errors.New("unexpected cache key")
	}
	if s.err != nil {
		return s.err
	}
	*(destination.(*domainsessions.Snapshot)) = s.snapshot
	return nil
}

func (cacheStub) SetJSON(context.Context, string, any, time.Duration) error { return nil }

func TestGetActiveSessionsReturnsCachedResponse(t *testing.T) {
	lastCached := time.Date(2026, time.August, 14, 5, 0, 0, 123, time.UTC)
	handler := NewHandler(cacheStub{snapshot: domainsessions.Snapshot{
		Result:     []infiniteflight.Session{{ID: "8c772474-bb70-4294-ad40-09f8cbf3b289", Name: "Casual"}},
		LastCached: lastCached,
		History: []domainsessions.Snapshot{{
			Result:     []infiniteflight.Session{{ID: "prior", Name: "Training"}},
			LastCached: lastCached.Add(-domainsessions.RefreshInterval),
		}},
	}})
	history := true
	recorder := httptest.NewRecorder()
	handler.GetActiveSessions(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/sessions/active?history=true", nil), &history)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", recorder.Code, recorder.Body)
	}
	var body struct {
		Data struct {
			AvailableFilters []struct {
				Name    string `json:"name"`
				Current bool   `json:"current"`
				Default bool   `json:"default"`
			} `json:"availableFilters"`
			Result  []infiniteflight.Session  `json:"result"`
			History []domainsessions.Snapshot `json:"history"`
			Meta    struct {
				LastCached          time.Time `json:"lastCached"`
				RefreshIntervalMins int       `json:"refreshIntervalMins"`
			} `json:"meta"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data.AvailableFilters) != 1 || body.Data.AvailableFilters[0].Name != "history" || !body.Data.AvailableFilters[0].Current || body.Data.AvailableFilters[0].Default {
		t.Fatalf("available filters = %#v", body.Data.AvailableFilters)
	}
	if len(body.Data.Result) != 1 || body.Data.Result[0].Name != "Casual" {
		t.Fatalf("result = %#v", body.Data.Result)
	}
	if len(body.Data.History) != 1 || len(body.Data.History[0].Result) != 1 || body.Data.History[0].Result[0].Name != "Training" {
		t.Fatalf("history = %#v, want prior snapshot", body.Data.History)
	}
	if !body.Data.Meta.LastCached.Equal(lastCached) || body.Data.Meta.RefreshIntervalMins != 5 {
		t.Fatalf("meta = %#v", body.Data.Meta)
	}
}

func TestGetActiveSessionsOmitsHistoryWhenNotRequested(t *testing.T) {
	lastCached := time.Date(2026, time.August, 14, 5, 0, 0, 0, time.UTC)
	handler := NewHandler(cacheStub{snapshot: domainsessions.Snapshot{
		Result:     []infiniteflight.Session{{ID: "current", Name: "Casual"}},
		LastCached: lastCached,
		History:    []domainsessions.Snapshot{{LastCached: lastCached.Add(-domainsessions.RefreshInterval)}},
	}})
	recorder := httptest.NewRecorder()
	handler.GetActiveSessions(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/sessions/active", nil), nil)

	var body struct {
		Data struct {
			Result  []infiniteflight.Session  `json:"result"`
			History []domainsessions.Snapshot `json:"history"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data.Result) != 1 || body.Data.Result[0].ID != "current" {
		t.Fatalf("result = %#v, want current result only", body.Data.Result)
	}
	if body.Data.History == nil || len(body.Data.History) != 0 {
		t.Fatalf("history = %#v, want empty array", body.Data.History)
	}
}

func TestGetActiveSessionsReturnsServiceUnavailableOnCacheMiss(t *testing.T) {
	handler := NewHandler(cacheStub{err: cache.ErrMiss})
	recorder := httptest.NewRecorder()
	handler.GetActiveSessions(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/sessions/active", nil), nil)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}

func TestGetActiveSessionsRejectsSnapshotWithoutTimestamp(t *testing.T) {
	handler := NewHandler(cacheStub{snapshot: domainsessions.Snapshot{}})
	recorder := httptest.NewRecorder()
	handler.GetActiveSessions(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/sessions/active", nil), nil)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
}
