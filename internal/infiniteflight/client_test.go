package infiniteflight

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestClientGetSessions(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/public/v2/sessions" {
			t.Errorf("path = %q, want /public/v2/sessions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		return response(http.StatusOK, `{
			"errorCode": 0,
			"result": [{
				"id": "1f5ff830-8e4d-4477-89e7-21c136d54844",
				"name": "Casual",
				"worldType": 1,
				"type": 0,
				"minimumGradeLevel": 0,
				"userCount": 176,
				"minimumAppVersion": "25.1",
				"maximumAppVersion": null,
				"maxUsers": 12000
			}]
		}`), nil
	})}

	client, err := newClient("https://example.test/public/v2", "test-key", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	sessions, err := client.GetSessions(context.Background())
	if err != nil {
		t.Fatalf("GetSessions() error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].Name != "Casual" || sessions[0].UserCount != 176 {
		t.Fatalf("sessions = %#v", sessions)
	}
}

func TestClientGetSessionsRejectsUpstreamErrorCode(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"errorCode":4,"result":[]}`), nil
	})}

	client, err := newClient("https://example.test/public/v2", "test-key", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.GetSessions(context.Background())
	if err == nil || !strings.Contains(err.Error(), "errorCode 4") {
		t.Fatalf("GetSessions() error = %v, want errorCode 4", err)
	}
}

func TestClientGetSessionFlights(t *testing.T) {
	sessionID := "ed323139-baa7-4834-b9d6-5fb9f19ff11e"
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/public/v2/sessions/"+sessionID+"/flights" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		return response(http.StatusOK, `{
			"errorCode": 0,
			"result": [{
				"username": "Hantder_Broncano_Jar",
				"callsign": "Swiss 39 Heavy",
				"latitude": 63.042991638183594,
				"longitude": -30.311138153076172,
				"altitude": 35999.953,
				"speed": 525.6486,
				"verticalSpeed": 0.00010479759,
				"track": 106.800766,
				"lastReport": "2026-08-15 05:09:53Z",
				"flightId": "c34118e7-cbdd-4e22-8751-0cda93e41d75",
				"userId": "0d85b360-92b5-4e62-ac64-41bd1c829772",
				"aircraftId": "e258f6d4-4503-4dde-b25c-1fb9067061e2",
				"liveryId": "df597aaf-456c-4878-9d84-45201f2aae74",
				"heading": 106.800766,
				"virtualOrganization": "Infinite Flight Airport Editing Team [IFAET]",
				"pilotState": 3,
				"isConnected": false
			}]
		}`), nil
	})}

	client, err := newClient("https://example.test/public/v2", "test-key", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	flights, err := client.GetSessionFlights(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetSessionFlights() error = %v", err)
	}
	if len(flights) != 1 || flights[0].Callsign != "Swiss 39 Heavy" || flights[0].FlightID != "c34118e7-cbdd-4e22-8751-0cda93e41d75" {
		t.Fatalf("flights = %#v", flights)
	}
	if flights[0].PilotState != 3 || flights[0].IsConnected || flights[0].Heading != 106.800766 {
		t.Fatalf("flight fields = %#v", flights[0])
	}
}

func TestClientGetSessionFlightsRejectsUpstreamErrorCode(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"errorCode":4,"result":[]}`), nil
	})}
	client, err := newClient("https://example.test/public/v2", "test-key", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.GetSessionFlights(context.Background(), "ed323139-baa7-4834-b9d6-5fb9f19ff11e")
	if err == nil || !strings.Contains(err.Error(), "errorCode 4") {
		t.Fatalf("GetSessionFlights() error = %v", err)
	}
}

func TestClientGetAircraftLiveries(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/public/v2/aircraft/liveries" {
			t.Errorf("path = %q", r.URL.Path)
		}
		return response(http.StatusOK, `{
			"errorCode": 0,
			"result": [{
				"id": "df597aaf-456c-4878-9d84-45201f2aae74",
				"aircraftID": "e258f6d4-4503-4dde-b25c-1fb9067061e2",
				"aircraftName": "Airbus A350",
				"liveryName": "Swiss"
			}]
		}`), nil
	})}
	client, err := newClient("https://example.test/public/v2", "test-key", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	liveries, err := client.GetAircraftLiveries(context.Background())
	if err != nil {
		t.Fatalf("GetAircraftLiveries() error = %v", err)
	}
	if len(liveries) != 1 || liveries[0].ID != "df597aaf-456c-4878-9d84-45201f2aae74" || liveries[0].AircraftName != "Airbus A350" || liveries[0].LiveryName != "Swiss" {
		t.Fatalf("liveries = %#v", liveries)
	}
}

func TestClientGetAircraftLiveriesRejectsHTTPError(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusUnauthorized, `{"errorCode":4,"result":null}`), nil
	})}
	client, err := newClient("https://example.test/public/v2", "test-key", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.GetAircraftLiveries(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 401, errorCode 4") {
		t.Fatalf("GetAircraftLiveries() error = %v", err)
	}
}

func TestClientGetSessionsRejectsHTTPError(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusUnauthorized, `{"errorCode":4,"result":null}`), nil
	})}

	client, err := newClient("https://example.test/public/v2", "test-key", httpClient)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.GetSessions(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 401, errorCode 4") {
		t.Fatalf("GetSessions() error = %v", err)
	}
}
