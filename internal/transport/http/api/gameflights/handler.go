package gameflights

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"infinite-experiment/politburo/internal/cache"
	domainflights "infinite-experiment/politburo/internal/game/flights"
	"infinite-experiment/politburo/internal/transport/http/api/cachedresponse"
	"infinite-experiment/politburo/internal/transport/http/response"
)

type Handler struct {
	cache  cache.Store
	tokens *domainflights.Tokens
}

type Query struct {
	ServerID    string
	PilotStates []string
	UserName    string
	CallSign    string
	PageNumber  int
	PageLength  int
}

type TrimmedFlight struct {
	FlightID  string  `json:"flightId"`
	Callsign  string  `json:"callsign"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Heading   float64 `json:"heading"`
}

func NewHandler(cacheStore cache.Store, secret []byte) *Handler {
	return &Handler{cache: cacheStore, tokens: domainflights.NewTokens(secret)}
}

func (h *Handler) GetActiveFlights(w http.ResponseWriter, r *http.Request, query Query) {
	if query.PageNumber < 1 {
		response.WriteError(w, http.StatusBadRequest, "INVALID_QUERY_FILTER", "pageNumber must be at least 1")
		return
	}
	if query.PageLength < 1 {
		response.WriteError(w, http.StatusBadRequest, "INVALID_QUERY_FILTER", "pageLength must be at least 1")
		return
	}

	loaded, ok := h.loadFiltered(w, r, query)
	if !ok {
		return
	}

	totalLength := len(loaded.result)
	paged := domainflights.Paginate(loaded.result, query.PageNumber, query.PageLength)
	for i := range paged {
		paged[i].History = nil
	}
	logFilterUsage("active", query, loaded.selected, totalLength)

	response.WriteJSON(w, http.StatusOK, cachedresponse.Response[domainflights.Flight]{
		Data: cachedresponse.Data[domainflights.Flight]{
			AvailableFilters: loaded.filters,
			Result:           paged,
			Meta:             loaded.meta,
			Pagination: &cachedresponse.Pagination{
				TotalLength: totalLength,
				PageLength:  query.PageLength,
				PageNumber:  query.PageNumber,
			},
		},
	})
}

func (h *Handler) GetTrimmedActiveFlights(w http.ResponseWriter, r *http.Request, query Query) {
	loaded, ok := h.loadFiltered(w, r, query)
	if !ok {
		return
	}

	trimmed := make([]TrimmedFlight, 0, len(loaded.result))
	for _, flight := range loaded.result {
		token, err := h.tokens.Encode(domainflights.MarkerToken{
			FlightID: flight.FlightID,
			ServerID: query.ServerID,
		})
		if err != nil {
			slog.Error("encode trimmed flight token", "error", err, "serverId", query.ServerID)
			response.WriteError(w, http.StatusInternalServerError, "FLIGHT_TOKEN_FAILED", "failed to encode flight markers")
			return
		}
		trimmed = append(trimmed, TrimmedFlight{
			FlightID:  token,
			Callsign:  flight.Callsign,
			Latitude:  flight.Latitude,
			Longitude: flight.Longitude,
			Heading:   flight.Heading,
		})
	}
	logFilterUsage("trimmed", query, loaded.selected, len(trimmed))

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"availableFilters": loaded.filters,
			"result":           trimmed,
			"count":            len(trimmed),
			"meta":             loaded.meta,
		},
	})
}

func (h *Handler) GetActiveFlight(w http.ResponseWriter, r *http.Request, flightToken string) {
	token, err := h.tokens.Decode(strings.TrimSpace(flightToken))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_FLIGHT_ID", "invalid flight id")
		return
	}

	snapshot, ok := h.readSnapshot(w, r, token.ServerID)
	if !ok {
		return
	}
	for _, flight := range snapshot.Result {
		if flight.FlightID != token.FlightID {
			continue
		}
		flight.History = nil
		slog.Info("active flight detail", "serverId", token.ServerID)
		response.WriteJSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"result": flight,
				"meta": cachedresponse.Meta{
					LastCached:          snapshot.LastCached,
					RefreshIntervalMins: int(domainflights.RefreshInterval / time.Minute),
				},
			},
		})
		return
	}
	response.WriteError(w, http.StatusNotFound, "FLIGHT_NOT_FOUND", "flight is not in the current snapshot")
}

type loadedFlights struct {
	result   []domainflights.Flight
	selected []string
	filters  []cachedresponse.Filter
	meta     cachedresponse.Meta
}

func (h *Handler) loadFiltered(w http.ResponseWriter, r *http.Request, query Query) (loadedFlights, bool) {
	if query.ServerID == "" {
		response.WriteError(w, http.StatusBadRequest, "INVALID_QUERY_FILTER", "serverId is required")
		return loadedFlights{}, false
	}

	selected := make([]string, 0, len(query.PilotStates))
	seen := make(map[string]struct{}, len(query.PilotStates))
	for _, name := range query.PilotStates {
		parsed, ok := domainflights.ParsePilotStateName(name)
		if !ok {
			response.WriteError(w, http.StatusBadRequest, "INVALID_QUERY_FILTER", "invalid pilotState value")
			return loadedFlights{}, false
		}
		if _, exists := seen[parsed]; exists {
			continue
		}
		seen[parsed] = struct{}{}
		selected = append(selected, parsed)
	}

	query.UserName = strings.TrimSpace(query.UserName)
	query.CallSign = strings.TrimSpace(query.CallSign)

	if !h.knownServer(r, w, query.ServerID) {
		return loadedFlights{}, false
	}

	snapshot, ok := h.readSnapshot(w, r, query.ServerID)
	if !ok {
		return loadedFlights{}, false
	}

	result := snapshot.Result
	if result == nil {
		result = make([]domainflights.Flight, 0)
	}
	if len(selected) > 0 || query.UserName != "" || query.CallSign != "" {
		filtered := make([]domainflights.Flight, 0, len(result))
		for _, flight := range result {
			if len(selected) > 0 {
				if _, exists := seen[flight.Normalized.PilotState]; !exists {
					continue
				}
			}
			if !domainflights.ContainsFold(domainflights.Username(flight), query.UserName) {
				continue
			}
			if !domainflights.ContainsFold(flight.Callsign, query.CallSign) {
				continue
			}
			filtered = append(filtered, flight)
		}
		result = filtered
	}

	return loadedFlights{
		result:   result,
		selected: selected,
		filters:  flightFilters(selected, query.UserName, query.CallSign),
		meta: cachedresponse.Meta{
			LastCached:          snapshot.LastCached,
			RefreshIntervalMins: int(domainflights.RefreshInterval / time.Minute),
		},
	}, true
}

func (h *Handler) readSnapshot(w http.ResponseWriter, r *http.Request, serverID string) (domainflights.Snapshot, bool) {
	snapshot := domainflights.Snapshot{}
	if err := h.cache.GetJSON(r.Context(), cache.KeyActiveFlights(serverID), &snapshot); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, cache.ErrMiss) {
			status = http.StatusServiceUnavailable
			slog.Warn("active flights cache miss", "serverId", serverID, "error", err)
		} else {
			slog.Error("read active flights cache", "error", err, "serverId", serverID)
		}
		response.WriteError(w, status, "ACTIVE_FLIGHTS_CACHE_UNAVAILABLE", "active flights cache is unavailable")
		return domainflights.Snapshot{}, false
	}
	if snapshot.LastCached.IsZero() {
		slog.Error("read active flights cache", "error", "lastCached is missing", "serverId", serverID)
		response.WriteError(w, http.StatusInternalServerError, "ACTIVE_FLIGHTS_CACHE_UNAVAILABLE", "active flights cache is unavailable")
		return domainflights.Snapshot{}, false
	}
	if snapshot.Result == nil {
		snapshot.Result = make([]domainflights.Flight, 0)
	}
	return snapshot, true
}

func flightFilters(selected []string, userName, callSign string) []cachedresponse.Filter {
	return []cachedresponse.Filter{
		{
			Name:    "pilotState",
			Type:    "multi",
			Desc:    "Restrict results to one or more pilot states",
			Current: selected,
			Default: []string{},
			Options: domainflights.PilotStateNames(),
		},
		{
			Name:    "userName",
			Type:    "string",
			Desc:    "Restrict results to flights whose username contains this value",
			Current: userName,
			Default: "",
		},
		{
			Name:    "callSign",
			Type:    "string",
			Desc:    "Restrict results to flights whose callsign contains this value",
			Current: callSign,
			Default: "",
		},
	}
}

func logFilterUsage(endpoint string, query Query, selected []string, count int) {
	slog.Info("active flights filter",
		"endpoint", endpoint,
		"serverId", query.ServerID,
		"pilotState", selected,
		"userName", query.UserName,
		"callSign", query.CallSign,
		"count", count,
	)
}

func (h *Handler) knownServer(r *http.Request, w http.ResponseWriter, serverID string) bool {
	var names []string
	if err := h.cache.GetJSON(r.Context(), cache.KeySessionNames, &names); err != nil {
		if errors.Is(err, cache.ErrMiss) {
			return true
		}
		slog.Error("read session names cache", "error", err)
		response.WriteError(w, http.StatusInternalServerError, "ACTIVE_FLIGHTS_CACHE_UNAVAILABLE", "active flights cache is unavailable")
		return false
	}
	for _, name := range names {
		if name == serverID {
			return true
		}
	}
	response.WriteError(w, http.StatusBadRequest, "INVALID_QUERY_FILTER", "unknown serverId")
	return false
}
