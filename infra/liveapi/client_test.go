package liveapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"infinite-experiment/politburo/infra/logging"
	metricsinfra "infinite-experiment/politburo/infra/metrics"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMain(m *testing.M) {
	_ = logging.Init("test")
	os.Exit(m.Run())
}

const (
	testAPIKey     = "test-liveapi-key"
	testSessionID  = "11111111-1111-1111-1111-111111111111"
	testFlightID   = "22222222-2222-2222-2222-222222222222"
	testUserID     = "33333333-3333-3333-3333-333333333333"
	testAircraftID = "44444444-4444-4444-4444-444444444444"
	testLiveryID   = "55555555-5555-5555-5555-555555555555"
)

func TestGeneratedWrapperUsesBearerAuthAndBaseURL(t *testing.T) {
	var gotPath, gotAuthorization, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthorization = r.Header.Get("Authorization")
		gotQuery = r.URL.RawQuery
		writeJSON(t, w, http.StatusOK, map[string]any{
			"errorCode": 0,
			"result": []map[string]any{{
				"id":                testSessionID,
				"maxUsers":          25,
				"name":              "Expert",
				"userCount":         3,
				"type":              0,
				"worldType":         3,
				"minimumGradeLevel": 2,
				"minimumAppVersion": "24.1",
			}},
		})
	}))
	defer server.Close()

	client := testClient(server.URL + "/public/v2")
	resp, err := client.GetSessions()
	if err != nil {
		t.Fatalf("GetSessions returned error: %v", err)
	}
	if len(resp.Result) != 1 || resp.Result[0].ID != testSessionID {
		t.Fatalf("unexpected sessions response: %#v", resp)
	}
	if gotPath != "/public/v2/sessions" {
		t.Fatalf("expected generated client to honor base URL path, got %q", gotPath)
	}
	if gotAuthorization != "Bearer "+testAPIKey {
		t.Fatalf("expected bearer authorization, got %q", gotAuthorization)
	}
	if strings.Contains(gotQuery, "key=") || strings.Contains(gotQuery, "apiKey=") || strings.Contains(gotQuery, "apikey=") {
		t.Fatalf("expected no query-string API key, got query %q", gotQuery)
	}
}

func TestGeneratedWrapperMappings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sessions":
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 0, "result": []map[string]any{{"id": testSessionID, "maxUsers": 25, "name": "Expert", "userCount": 3, "type": 0, "worldType": 3, "minimumGradeLevel": 2, "minimumAppVersion": "24.1"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/sessions/"+testSessionID+"/flights":
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 0, "result": []map[string]any{{"username": "Pilot", "callsign": "IE123", "latitude": 1.25, "longitude": 2.5, "altitude": 32000, "speed": 420, "verticalSpeed": -500, "track": 180, "lastReport": "2026-05-21 12:34:56Z", "flightId": testFlightID, "userId": testUserID, "aircraftId": testAircraftID, "liveryId": testLiveryID, "virtualOrganization": "IE", "pilotState": 0, "isConnected": true}}})
		case r.Method == http.MethodGet && r.URL.Path == "/sessions/"+testSessionID+"/flights/"+testFlightID+"/flightplan":
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 0, "result": map[string]any{"flightPlanId": "66666666-6666-6666-6666-666666666666", "flightId": testFlightID, "waypoints": []string{"KSFO", "KLAX"}, "lastUpdate": "2026-05-21 12:34:56Z", "flightPlanItems": []map[string]any{{"name": "KSFO", "type": 3, "identifier": "KSFO", "altitude": 0, "location": map[string]any{"latitude": 37.6, "longitude": -122.3, "altitude": 0}}}}})
		case r.Method == http.MethodGet && r.URL.Path == "/aircraft/liveries":
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 0, "result": []map[string]any{{"id": testLiveryID, "aircraftID": testAircraftID, "liveryName": "Infinite", "aircraftName": "A350"}}})
		case r.Method == http.MethodPost && r.URL.Path == "/users":
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 0, "result": []map[string]any{{"onlineFlights": 1, "violations": 2, "xp": 1234, "landingCount": 5, "flightTime": 90, "atcOperations": 6, "atcRank": 7, "grade": 3, "hash": "hash", "violationCountByLevel": map[string]any{"level1": 1, "level2": 2, "level3": 3}, "roles": []int{1, 2}, "userId": testUserID, "virtualOrganization": "IE", "discourseUsername": "Pilot", "groups": []string{testSessionID}, "errorCode": 0}}})
		case r.Method == http.MethodGet && r.URL.Path == "/users/"+testUserID+"/flights":
			if r.URL.Query().Get("page") != "2" {
				t.Fatalf("expected page query 2, got %q", r.URL.RawQuery)
			}
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 0, "result": map[string]any{"pageIndex": 2, "totalPages": 4, "totalCount": 8, "hasPreviousPage": true, "hasNextPage": true, "data": []map[string]any{{"id": testFlightID, "created": "2026-05-21 12:34:56Z", "userId": testUserID, "aircraftId": testAircraftID, "liveryId": testLiveryID, "callsign": "IE123", "server": "Expert", "dayTime": 1.5, "nightTime": 0.5, "totalTime": 2, "landingCount": 1, "originAirport": "KSFO", "destinationAirport": "KLAX", "xp": 250, "worldType": 3, "violations": []map[string]any{{"level": 1}}}}}})
		case r.Method == http.MethodGet && r.URL.Path == "/users/"+testUserID:
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 0, "result": map[string]any{"atcOperations": 1, "errorCode": 0, "gradeDetails": map[string]any{"gradeIndex": 4, "grades": []any{}, "ruleDefinitions": []any{}}, "groups": []string{}, "lastLevel1ViolationDate": "", "lastLevel2ViolationDate": "", "lastLevel3ViolationDate": "", "lastReportViolationDate": "", "roles": []int{}, "total12MonthsViolations": 0, "totalXP": 100, "userId": testUserID, "violationCountByLevel": map[string]any{"level1": 0, "level2": 0, "level3": 0}}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := testClient(server.URL)

	sessions, err := client.GetSessions()
	if err != nil || sessions.Result[0].Name != "Expert" {
		t.Fatalf("GetSessions = %#v, %v", sessions, err)
	}
	flights, status, err := client.GetFlights(testSessionID)
	if err != nil || status != http.StatusOK || flights.Flights[0].Callsign != "IE123" || flights.Flights[0].Username != "Pilot" {
		t.Fatalf("GetFlights = %#v, %d, %v", flights, status, err)
	}
	plan, status, err := client.GetFlightPlan(testSessionID, testFlightID)
	if err != nil || status != http.StatusOK || plan.FlightID != testFlightID || len(plan.FlightPlanItems) != 1 {
		t.Fatalf("GetFlightPlan = %#v, %d, %v", plan, status, err)
	}
	liveries, status, err := client.GetAircraftLiveries()
	if err != nil || status != http.StatusOK || liveries.Liveries[0].AircraftName != "A350" {
		t.Fatalf("GetAircraftLiveries = %#v, %d, %v", liveries, status, err)
	}
	stats, status, err := client.GetUserByIfcId("Pilot")
	if err != nil || status != http.StatusOK || stats.Result[0].XP != 1234 || stats.Result[0].Groups[0] != testSessionID {
		t.Fatalf("GetUserByIfcId = %#v, %d, %v", stats, status, err)
	}
	userFlights, status, err := client.GetUserFlights(testUserID, 2)
	if err != nil || status != http.StatusOK || userFlights.Flights[0].OriginAirport != "KSFO" || !userFlights.HasNext {
		t.Fatalf("GetUserFlights = %#v, %d, %v", userFlights, status, err)
	}
	grade, status, err := client.GetUserGrade(testUserID)
	if err != nil || status != http.StatusOK || grade.Grade != 4 {
		t.Fatalf("GetUserGrade = %#v, %d, %v", grade, status, err)
	}
}

func TestGeneratedWrapperStatusAndErrorCodeHandling(t *testing.T) {
	t.Run("429 returns rate limit error and status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusTooManyRequests, map[string]any{"errorCode": 3, "result": nil})
		}))
		defer server.Close()

		_, status, err := testClient(server.URL).GetFlights(testSessionID)
		if status != http.StatusTooManyRequests {
			t.Fatalf("expected status 429, got %d", status)
		}
		if err == nil || !strings.Contains(err.Error(), "rate limited") {
			t.Fatalf("expected explicit rate-limit error, got %v", err)
		}
	})

	t.Run("nonzero errorCode is surfaced", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 6, "result": []any{}})
		}))
		defer server.Close()

		_, err := testClient(server.URL).GetSessions()
		if err == nil || !strings.Contains(err.Error(), "errorCode 6") {
			t.Fatalf("expected nonzero errorCode error, got %v", err)
		}
	})
}

func TestLiveAPIObservabilityClassifiesStatusErrors(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		errorType   string
		statusClass string
	}{
		{name: "rate limited", status: http.StatusTooManyRequests, errorType: "rate_limited", statusClass: "4xx"},
		{name: "unauthorized", status: http.StatusUnauthorized, errorType: "auth", statusClass: "4xx"},
		{name: "forbidden", status: http.StatusForbidden, errorType: "auth", statusClass: "4xx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, tt.status, map[string]any{"errorCode": 4, "result": nil})
			}))
			defer server.Close()

			client, _ := testClientWithMetrics(server.URL)
			_, err := client.GetSessions()
			if err == nil {
				t.Fatal("expected status error")
			}
			assertLiveAPICounter(t, client, "sessions", tt.statusClass, tt.errorType, 1)
		})
	}
}

func TestLiveAPIObservabilityClassifiesGeneratedResponseErrors(t *testing.T) {
	tests := []struct {
		name      string
		handler   http.HandlerFunc
		errorType string
	}{
		{
			name: "nonzero error code",
			handler: func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 6, "result": []any{}})
			},
			errorType: "error_code_6",
		},
		{
			name: "empty generated response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			errorType: "empty_response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			client, _ := testClientWithMetrics(server.URL)
			_, err := client.GetSessions()
			if err == nil {
				t.Fatal("expected response error")
			}
			assertLiveAPICounter(t, client, "sessions", "2xx", tt.errorType, 1)
		})
	}
}

func TestLiveAPIObservabilityClassifiesDecodeAndNetworkErrors(t *testing.T) {
	t.Run("decode error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{`))
		}))
		defer server.Close()

		client, _ := testClientWithMetrics(server.URL)
		_, _, err := client.GetATIS()
		if err == nil {
			t.Fatal("expected decode error")
		}
		assertLiveAPICounter(t, client, "flights", "2xx", "decode_error", 1)
	})

	t.Run("network error", func(t *testing.T) {
		client, _ := testClientWithMetrics("http://liveapi.test")
		client.Client = &http.Client{Transport: errorRoundTripper{err: errors.New("dial failed")}}

		_, err := client.GetSessions()
		if err == nil {
			t.Fatal("expected network error")
		}
		assertLiveAPICounter(t, client, "sessions", "none", "network", 1)
	})
}

func TestGeneratedWrapperNullableFieldsMapSafely(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sessions/" + testSessionID + "/flights":
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 0, "result": []map[string]any{{"username": nil, "callsign": "IE123", "latitude": 1, "longitude": 2, "altitude": 3, "speed": 4, "verticalSpeed": 5, "track": 6, "lastReport": "2026-05-21 12:34:56Z", "flightId": testFlightID, "userId": testUserID, "aircraftId": testAircraftID, "liveryId": testLiveryID, "virtualOrganization": nil, "pilotState": 0, "isConnected": true}}})
		case "/users/" + testUserID + "/flights":
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 0, "result": map[string]any{"pageIndex": 1, "totalPages": 1, "totalCount": 1, "hasPreviousPage": false, "hasNextPage": false, "data": []map[string]any{{"id": testFlightID, "created": "2026-05-21 12:34:56Z", "userId": testUserID, "aircraftId": testAircraftID, "liveryId": testLiveryID, "callsign": "IE123", "server": "Expert", "dayTime": 1, "nightTime": 0, "totalTime": 1, "landingCount": 1, "originAirport": nil, "destinationAirport": nil, "xp": 100, "worldType": 3, "violations": []any{}}}}})
		case "/users":
			writeJSON(t, w, http.StatusOK, map[string]any{"errorCode": 0, "result": []map[string]any{{"onlineFlights": 0, "violations": 0, "xp": 0, "landingCount": 0, "flightTime": 0, "atcOperations": 0, "atcRank": nil, "grade": 1, "hash": "hash", "violationCountByLevel": map[string]any{"level1": 0, "level2": 0, "level3": 0}, "roles": []int{}, "userId": testUserID, "virtualOrganization": nil, "discourseUsername": nil, "groups": []string{}, "errorCode": 0}}})
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := testClient(server.URL)
	flights, _, err := client.GetFlights(testSessionID)
	if err != nil {
		t.Fatalf("GetFlights returned error: %v", err)
	}
	if flights.Flights[0].Username != "" || flights.Flights[0].VirtualOrganization != "" {
		t.Fatalf("expected nullable flight strings to map to empty strings: %#v", flights.Flights[0])
	}
	userFlights, _, err := client.GetUserFlights(testUserID, 1)
	if err != nil {
		t.Fatalf("GetUserFlights returned error: %v", err)
	}
	if userFlights.Flights[0].OriginAirport != "" || userFlights.Flights[0].DestinationAirport != "" {
		t.Fatalf("expected nullable airports to map to empty strings: %#v", userFlights.Flights[0])
	}
	stats, _, err := client.GetUserByIfcId("Pilot")
	if err != nil {
		t.Fatalf("GetUserByIfcId returned error: %v", err)
	}
	if stats.Result[0].ATCRank != nil || stats.Result[0].VirtualOrganization != nil || stats.Result[0].DiscourseUsername != nil {
		t.Fatalf("expected nullable stats pointers to remain nil: %#v", stats.Result[0])
	}
}

func TestUUIDValidationRejectsInvalidIDsBeforeRequest(t *testing.T) {
	client := testClient("http://liveapi.test")
	transport := &countingRoundTripper{}
	client.Client = &http.Client{Transport: transport}

	assertInvalid := func(name string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "invalid") {
			t.Fatalf("%s expected invalid UUID error, got %v", name, err)
		}
		if transport.count != 0 {
			t.Fatalf("%s made %d HTTP requests before validation failed", name, transport.count)
		}
	}

	_, _, err := client.GetFlights("not-a-uuid")
	assertInvalid("GetFlights", err)
	_, _, err = client.GetFlightPlan("not-a-uuid", testFlightID)
	assertInvalid("GetFlightPlan sessionID", err)
	_, _, err = client.GetFlightPlan(testSessionID, "not-a-uuid")
	assertInvalid("GetFlightPlan flightID", err)
	_, _, err = client.GetFlightRoute("not-a-uuid", testSessionID)
	assertInvalid("GetFlightRoute flightID", err)
	_, _, err = client.GetFlightRoute(testFlightID, "not-a-uuid")
	assertInvalid("GetFlightRoute sessionID", err)
	_, _, err = client.GetUserFlights("not-a-uuid", 1)
	assertInvalid("GetUserFlights", err)
	_, _, err = client.GetUserGrade("not-a-uuid")
	assertInvalid("GetUserGrade", err)
}

func TestLiveAPIDateParsingFormats(t *testing.T) {
	tests := []string{
		"2026-05-21 12:34:56Z",
		"2026-05-21T12:34:56Z",
		"2026-05-21T12:34:56.123456789Z",
	}
	for _, value := range tests {
		parsed, err := parseAPITime(value)
		if err != nil {
			t.Fatalf("parseAPITime(%q) returned error: %v", value, err)
		}
		if parsed.IsZero() {
			t.Fatalf("parseAPITime(%q) returned zero time", value)
		}

		var apiTime APITime
		if err := apiTime.UnmarshalJSON([]byte(`"` + value + `"`)); err != nil {
			t.Fatalf("APITime.UnmarshalJSON(%q) returned error: %v", value, err)
		}
		if apiTime.Time.IsZero() {
			t.Fatalf("APITime.UnmarshalJSON(%q) returned zero time", value)
		}
	}
}

func testClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  testAPIKey,
		Client:  &http.Client{Timeout: 2 * time.Second},
	}
}

func testClientWithMetrics(baseURL string) (*Client, *prometheus.Registry) {
	reg := prometheus.NewRegistry()
	requests := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{Name: "politburo_liveapi_requests_total", Help: "test LiveAPI requests"},
		[]string{"provider", "endpoint_group", "status_class", "error_type"},
	)
	duration := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{Name: "politburo_liveapi_request_duration_seconds", Help: "test LiveAPI duration"},
		[]string{"provider", "endpoint_group", "status_class", "error_type"},
	)
	client := testClient(baseURL)
	client.SetMetrics(&metricsinfra.MetricsRegistry{
		LiveAPIRequestsTotal:   *requests,
		LiveAPIRequestDuration: *duration,
	})
	return client, reg
}

func assertLiveAPICounter(t *testing.T, client *Client, endpointGroup, statusClass, errorType string, want float64) {
	t.Helper()
	got := testutil.ToFloat64(client.metrics.LiveAPIRequestsTotal.WithLabelValues("liveapi", endpointGroup, statusClass, errorType))
	if got != want {
		t.Fatalf("LiveAPIRequestsTotal(%s, %s, %s) = %v, want %v", endpointGroup, statusClass, errorType, got, want)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("failed to write JSON response: %v", err)
	}
}

type countingRoundTripper struct {
	count int
}

func (rt *countingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	rt.count++
	return nil, errors.New("unexpected request")
}

type errorRoundTripper struct {
	err error
}

func (rt errorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, rt.err
}
