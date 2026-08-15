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
