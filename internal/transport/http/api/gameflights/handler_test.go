package gameflights

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"infinite-experiment/politburo/internal/cache"
	domainflights "infinite-experiment/politburo/internal/game/flights"
	"infinite-experiment/politburo/internal/transport/http/api/cachedresponse"
)

type cacheStub struct {
	flights  domainflights.Snapshot
	names    []string
	namesErr error
	err      error
}

func (s cacheStub) GetJSON(_ context.Context, key string, destination any) error {
	switch key {
	case cache.KeySessionNames:
		if s.namesErr != nil {
			return s.namesErr
		}
		*(destination.(*[]string)) = s.names
		return nil
	case cache.KeyActiveFlights("casual"):
		if s.err != nil {
			return s.err
		}
		*(destination.(*domainflights.Snapshot)) = s.flights
		return nil
	default:
		return cache.ErrMiss
	}
}

func (cacheStub) SetJSON(context.Context, string, any, time.Duration) error { return nil }

var testFlightSecret = []byte("0123456789abcdef0123456789abcdef")

func testHandler(store cache.Store) *Handler {
	return NewHandler(store, testFlightSecret)
}

func sampleFlight(state string) domainflights.Flight {
	return domainflights.Flight{
		FlightID:       "c34118e7-cbdd-4e22-8751-0cda93e41d75",
		Callsign:       "Swiss 39 Heavy",
		Latitude:       47.45,
		Longitude:      8.56,
		Heading:        329.2,
		NormalizedName: "casual",
		Normalized:     domainflights.Normalized{PilotState: state, Speed: "526 kts", VerticalSpeed: "0.0 ft/min", IsConnected: "disconnected"},
		PathSync:       &domainflights.PathSync{FPLSyncRequired: false},
	}
}

func defaultQuery(serverID string, pilotStates []string) Query {
	return Query{
		ServerID:    serverID,
		PilotStates: pilotStates,
		PageNumber:  domainflights.DefaultPageNumber,
		PageLength:  domainflights.DefaultPageLength,
	}
}

func stringPtr(value string) *string {
	return &value
}

func TestGetActiveFlightsReturnsCachedFlights(t *testing.T) {
	lastCached := time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC)
	handler := testHandler(cacheStub{
		names: []string{"casual"},
		flights: domainflights.Snapshot{
			LastCached: lastCached,
			Result:     []domainflights.Flight{sampleFlight(domainflights.PilotStateNameInBackground), sampleFlight(domainflights.PilotStateNameActive)},
		},
	})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual", nil), defaultQuery("casual", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	var body struct {
		Data struct {
			AvailableFilters []struct {
				Name    string          `json:"name"`
				Type    string          `json:"type"`
				Current json.RawMessage `json:"current"`
				Options []string        `json:"options"`
			} `json:"availableFilters"`
			Result []domainflights.Flight `json:"result"`
			Meta   struct {
				RefreshIntervalMins int `json:"refreshIntervalMins"`
			} `json:"meta"`
			Pagination struct {
				TotalLength int `json:"totalLength"`
				PageLength  int `json:"pageLength"`
				PageNumber  int `json:"pageNumber"`
			} `json:"pagination"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data.AvailableFilters) != 3 {
		t.Fatalf("filters = %#v", body.Data.AvailableFilters)
	}
	if body.Data.AvailableFilters[0].Name != "pilotState" || body.Data.AvailableFilters[0].Type != "multi" || len(body.Data.AvailableFilters[0].Options) == 0 {
		t.Fatalf("pilotState filter = %#v", body.Data.AvailableFilters[0])
	}
	if body.Data.AvailableFilters[1].Name != "userName" || body.Data.AvailableFilters[1].Type != "string" || string(body.Data.AvailableFilters[1].Current) != `""` || body.Data.AvailableFilters[1].Options != nil {
		t.Fatalf("userName filter = %#v", body.Data.AvailableFilters[1])
	}
	if body.Data.AvailableFilters[2].Name != "callSign" || body.Data.AvailableFilters[2].Type != "string" || string(body.Data.AvailableFilters[2].Current) != `""` || body.Data.AvailableFilters[2].Options != nil {
		t.Fatalf("callSign filter = %#v", body.Data.AvailableFilters[2])
	}
	if len(body.Data.Result) != 2 || body.Data.Meta.RefreshIntervalMins != 1 {
		t.Fatalf("body = %#v", body.Data)
	}
	if body.Data.Result[0].History != nil || strings.Contains(recorder.Body.String(), `"history"`) {
		t.Fatalf("live flights response still includes history: %s", recorder.Body.String())
	}
	if body.Data.Pagination.TotalLength != 2 || body.Data.Pagination.PageLength != domainflights.DefaultPageLength || body.Data.Pagination.PageNumber != domainflights.DefaultPageNumber {
		t.Fatalf("pagination = %#v", body.Data.Pagination)
	}
}

func TestGetActiveFlightsStripsLegacyNestedHistory(t *testing.T) {
	flight := sampleFlight(domainflights.PilotStateNameActive)
	flight.History = []domainflights.Flight{{Callsign: "older"}}
	handler := testHandler(cacheStub{
		names: []string{"casual"},
		flights: domainflights.Snapshot{
			LastCached: time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC),
			Result:     []domainflights.Flight{flight},
		},
	})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual", nil), defaultQuery("casual", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	if strings.Contains(recorder.Body.String(), `"history"`) {
		t.Fatalf("response still includes history: %s", recorder.Body.String())
	}
}

func TestGetActiveFlightsFiltersPilotState(t *testing.T) {
	handler := testHandler(cacheStub{
		names: []string{"casual"},
		flights: domainflights.Snapshot{
			LastCached: time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC),
			Result:     []domainflights.Flight{sampleFlight(domainflights.PilotStateNameInBackground), sampleFlight(domainflights.PilotStateNameActive)},
		},
	})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual&pilotState=active", nil), defaultQuery("casual", []string{domainflights.PilotStateNameActive}))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	var body struct {
		Data struct {
			Result []domainflights.Flight `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data.Result) != 1 || body.Data.Result[0].Normalized.PilotState != domainflights.PilotStateNameActive {
		t.Fatalf("result = %#v", body.Data.Result)
	}
	var paginated struct {
		Data struct {
			Pagination struct {
				TotalLength int `json:"totalLength"`
				PageLength  int `json:"pageLength"`
				PageNumber  int `json:"pageNumber"`
			} `json:"pagination"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &paginated); err != nil {
		t.Fatalf("decode pagination: %v", err)
	}
	if paginated.Data.Pagination.TotalLength != 1 || paginated.Data.Pagination.PageLength != domainflights.DefaultPageLength || paginated.Data.Pagination.PageNumber != domainflights.DefaultPageNumber {
		t.Fatalf("pagination = %#v", paginated.Data.Pagination)
	}
}

func TestGetActiveFlightsRejectsUnknownServer(t *testing.T) {
	handler := testHandler(cacheStub{names: []string{"casual"}})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=expert", nil), defaultQuery("expert", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestGetActiveFlightsRejectsInvalidPilotState(t *testing.T) {
	handler := testHandler(cacheStub{names: []string{"casual"}})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual&pilotState=flying", nil), defaultQuery("casual", []string{"flying"}))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestGetActiveFlightsRequiresServerID(t *testing.T) {
	handler := testHandler(cacheStub{})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active", nil), defaultQuery("", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestGetActiveFlightsReturnsServiceUnavailableOnCacheMiss(t *testing.T) {
	handler := testHandler(cacheStub{names: []string{"casual"}, err: cache.ErrMiss})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual", nil), defaultQuery("casual", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestGetActiveFlightsRejectsSnapshotWithoutTimestamp(t *testing.T) {
	handler := testHandler(cacheStub{names: []string{"casual"}, flights: domainflights.Snapshot{}})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual", nil), defaultQuery("casual", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestGetActiveFlightsTreatsNamesReadError(t *testing.T) {
	handler := testHandler(cacheStub{namesErr: errors.New("redis down")})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual", nil), defaultQuery("casual", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func decodePagination(t *testing.T, recorder *httptest.ResponseRecorder) (resultCount, totalLength, pageLength, pageNumber int) {
	t.Helper()
	var body struct {
		Data struct {
			Result     []domainflights.Flight `json:"result"`
			Pagination struct {
				TotalLength int `json:"totalLength"`
				PageLength  int `json:"pageLength"`
				PageNumber  int `json:"pageNumber"`
			} `json:"pagination"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return len(body.Data.Result), body.Data.Pagination.TotalLength, body.Data.Pagination.PageLength, body.Data.Pagination.PageNumber
}

func TestGetActiveFlightsDefaultsToPageSizeFifty(t *testing.T) {
	flights := make([]domainflights.Flight, 60)
	for i := range flights {
		flights[i] = sampleFlight(domainflights.PilotStateNameActive)
	}
	handler := testHandler(cacheStub{
		names: []string{"casual"},
		flights: domainflights.Snapshot{
			LastCached: time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC),
			Result:     flights,
		},
	})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual", nil), defaultQuery("casual", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	resultCount, totalLength, pageLength, pageNumber := decodePagination(t, recorder)
	if resultCount != 50 || totalLength != 60 || pageLength != 50 || pageNumber != 1 {
		t.Fatalf("resultCount=%d totalLength=%d pageLength=%d pageNumber=%d", resultCount, totalLength, pageLength, pageNumber)
	}
}

func TestGetActiveFlightsPaginatesResults(t *testing.T) {
	flights := make([]domainflights.Flight, 60)
	for i := range flights {
		flights[i] = sampleFlight(domainflights.PilotStateNameActive)
		flights[i].Callsign = "flight-" + string(rune('A'+i%26))
	}
	handler := testHandler(cacheStub{
		names: []string{"casual"},
		flights: domainflights.Snapshot{
			LastCached: time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC),
			Result:     flights,
		},
	})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual&pageNumber=2&pageLength=10", nil), Query{ServerID: "casual", PageNumber: 2, PageLength: 10})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	resultCount, totalLength, pageLength, pageNumber := decodePagination(t, recorder)
	if resultCount != 10 || totalLength != 60 || pageLength != 10 || pageNumber != 2 {
		t.Fatalf("resultCount=%d totalLength=%d pageLength=%d pageNumber=%d", resultCount, totalLength, pageLength, pageNumber)
	}
}

func TestGetActiveFlightsReturnsEmptyPagePastEnd(t *testing.T) {
	handler := testHandler(cacheStub{
		names: []string{"casual"},
		flights: domainflights.Snapshot{
			LastCached: time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC),
			Result:     []domainflights.Flight{sampleFlight(domainflights.PilotStateNameActive)},
		},
	})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual&pageNumber=3&pageLength=50", nil), Query{ServerID: "casual", PageNumber: 3, PageLength: 50})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	resultCount, totalLength, pageLength, pageNumber := decodePagination(t, recorder)
	if resultCount != 0 || totalLength != 1 || pageLength != 50 || pageNumber != 3 {
		t.Fatalf("resultCount=%d totalLength=%d pageLength=%d pageNumber=%d", resultCount, totalLength, pageLength, pageNumber)
	}
}

func TestGetActiveFlightsRejectsInvalidPage(t *testing.T) {
	handler := testHandler(cacheStub{names: []string{"casual"}})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual&pageNumber=0", nil), Query{ServerID: "casual", PageNumber: 0, PageLength: domainflights.DefaultPageLength})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestGetActiveFlightsRejectsInvalidPageLength(t *testing.T) {
	handler := testHandler(cacheStub{names: []string{"casual"}})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual&pageLength=0", nil), Query{ServerID: "casual", PageNumber: domainflights.DefaultPageNumber, PageLength: 0})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestGetActiveFlightsFiltersUserNameAndCallSign(t *testing.T) {
	swiss := sampleFlight(domainflights.PilotStateNameActive)
	swiss.Username = stringPtr("Hantder_Broncano_Jar")
	swiss.Callsign = "Swiss 39 Heavy"
	lufthansa := sampleFlight(domainflights.PilotStateNameActive)
	lufthansa.Username = stringPtr("OtherPilot")
	lufthansa.Callsign = "Lufthansa 123"
	anonymous := sampleFlight(domainflights.PilotStateNameActive)
	anonymous.Callsign = "N123AB"
	handler := testHandler(cacheStub{
		names: []string{"casual"},
		flights: domainflights.Snapshot{
			LastCached: time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC),
			Result:     []domainflights.Flight{swiss, lufthansa, anonymous},
		},
	})

	t.Run("userName", func(t *testing.T) {
		query := defaultQuery("casual", nil)
		query.UserName = "hantder"
		recorder := httptest.NewRecorder()
		handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual&userName=hantder", nil), query)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
		}
		resultCount, totalLength, _, _ := decodePagination(t, recorder)
		if resultCount != 1 || totalLength != 1 {
			t.Fatalf("resultCount=%d totalLength=%d", resultCount, totalLength)
		}
		var body struct {
			Data struct {
				Result           []domainflights.Flight `json:"result"`
				AvailableFilters []struct {
					Name    string          `json:"name"`
					Current json.RawMessage `json:"current"`
					Options []string        `json:"options"`
				} `json:"availableFilters"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Data.Result[0].Callsign != "Swiss 39 Heavy" {
			t.Fatalf("result = %#v", body.Data.Result)
		}
		if string(body.Data.AvailableFilters[1].Current) != `"hantder"` || body.Data.AvailableFilters[1].Options != nil {
			t.Fatalf("userName filter = %#v", body.Data.AvailableFilters[1])
		}
	})

	t.Run("callSign", func(t *testing.T) {
		query := defaultQuery("casual", nil)
		query.CallSign = "swiss"
		recorder := httptest.NewRecorder()
		handler.GetActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active?serverId=casual&callSign=swiss", nil), query)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
		}
		resultCount, totalLength, _, _ := decodePagination(t, recorder)
		if resultCount != 1 || totalLength != 1 {
			t.Fatalf("resultCount=%d totalLength=%d", resultCount, totalLength)
		}
	})
}

func TestGetTrimmedActiveFlightsReturnsMarkersWithoutPaging(t *testing.T) {
	lastCached := time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC)
	handler := testHandler(cacheStub{
		names: []string{"casual"},
		flights: domainflights.Snapshot{
			LastCached: lastCached,
			Result:     []domainflights.Flight{sampleFlight(domainflights.PilotStateNameActive), sampleFlight(domainflights.PilotStateNameInBackground)},
		},
	})
	recorder := httptest.NewRecorder()
	handler.GetTrimmedActiveFlights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active/trimmed?serverId=casual", nil), Query{ServerID: "casual"})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	var body struct {
		Data struct {
			Count  int `json:"count"`
			Result []struct {
				FlightID  string  `json:"flightId"`
				Callsign  string  `json:"callsign"`
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
				Heading   float64 `json:"heading"`
				UserID    string  `json:"userId"`
			} `json:"result"`
			Pagination *cachedresponse.Pagination `json:"pagination"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Count != 2 || len(body.Data.Result) != 2 || body.Data.Pagination != nil {
		t.Fatalf("count=%d result=%d pagination=%#v", body.Data.Count, len(body.Data.Result), body.Data.Pagination)
	}
	marker := body.Data.Result[0]
	if marker.Callsign != "Swiss 39 Heavy" || marker.Latitude != 47.45 || marker.Longitude != 8.56 || marker.Heading != 329.2 {
		t.Fatalf("marker = %#v", marker)
	}
	if marker.UserID != "" {
		t.Fatalf("trimmed payload leaked userId: %#v", marker)
	}
	if marker.FlightID == sampleFlight(domainflights.PilotStateNameActive).FlightID {
		t.Fatal("trimmed flightId must be encrypted")
	}
	token, err := domainflights.NewTokens(testFlightSecret).Decode(marker.FlightID)
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if token.FlightID != "c34118e7-cbdd-4e22-8751-0cda93e41d75" || token.ServerID != "casual" {
		t.Fatalf("token = %#v", token)
	}
}

func TestGetActiveFlightResolvesEncryptedMarker(t *testing.T) {
	lastCached := time.Date(2026, time.August, 15, 6, 0, 0, 0, time.UTC)
	flight := sampleFlight(domainflights.PilotStateNameActive)
	handler := testHandler(cacheStub{
		names: []string{"casual"},
		flights: domainflights.Snapshot{
			LastCached: lastCached,
			Result:     []domainflights.Flight{flight},
		},
	})
	token, err := domainflights.NewTokens(testFlightSecret).Encode(domainflights.MarkerToken{
		FlightID: flight.FlightID,
		ServerID: "casual",
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	recorder := httptest.NewRecorder()
	handler.GetActiveFlight(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active/detail?flightId="+url.QueryEscape(token), nil), token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
	var body struct {
		Data struct {
			Result domainflights.Flight `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Result.FlightID != flight.FlightID || body.Data.Result.Callsign != flight.Callsign {
		t.Fatalf("result = %#v", body.Data.Result)
	}
}

func TestGetActiveFlightRejectsInvalidToken(t *testing.T) {
	handler := testHandler(cacheStub{names: []string{"casual"}})
	recorder := httptest.NewRecorder()
	handler.GetActiveFlight(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/game/flights/active/detail?flightId=not-a-token", nil), "not-a-token")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body)
	}
}
