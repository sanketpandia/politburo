package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/platform/httpdto"
)

func TestRequireDiscordBotContextMiddleware(t *testing.T) {
	_ = logging.Init("local")

	tests := []struct {
		name          string
		headers       map[string]string
		wantStatus    int
		wantNext      bool
		wantErrorCode string
	}{
		{
			name: "both new headers present",
			headers: map[string]string{
				DiscordUserIDHeader:   "discord-user",
				DiscordServerIDHeader: "discord-server",
			},
			wantStatus: http.StatusNoContent,
			wantNext:   true,
		},
		{
			name: "missing Discord user header",
			headers: map[string]string{
				DiscordServerIDHeader: "discord-server",
			},
			wantStatus:    http.StatusForbidden,
			wantErrorCode: "MISSING_DISCORD_CONTEXT",
		},
		{
			name: "missing Discord server header",
			headers: map[string]string{
				DiscordUserIDHeader: "discord-user",
			},
			wantStatus:    http.StatusForbidden,
			wantErrorCode: "MISSING_DISCORD_CONTEXT",
		},
		{
			name: "blank header values",
			headers: map[string]string{
				DiscordUserIDHeader:   "   ",
				DiscordServerIDHeader: "\t",
			},
			wantStatus:    http.StatusForbidden,
			wantErrorCode: "MISSING_DISCORD_CONTEXT",
		},
		{
			name: "old headers only",
			headers: map[string]string{
				"X-Discord-Id": "discord-user",
				"X-Server-Id":  "discord-server",
			},
			wantStatus:    http.StatusForbidden,
			wantErrorCode: "MISSING_DISCORD_CONTEXT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			claims := &auth.APIKeyClaims{DiscordUIDVal: "existing-user", DiscordServerIDVal: "existing-server"}
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				gotClaims := auth.GetUserClaims(r.Context())
				if gotClaims != claims {
					t.Fatalf("claims were mutated or replaced")
				}
				w.WriteHeader(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/pilots/register", nil)
			req = req.WithContext(auth.SetUserClaims(req.Context(), claims))
			for name, value := range tt.headers {
				req.Header.Set(name, value)
			}
			rr := httptest.NewRecorder()

			RequireDiscordBotContextMiddleware()(next).ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (%s)", tt.wantStatus, rr.Code, rr.Body.String())
			}
			if nextCalled != tt.wantNext {
				t.Fatalf("next called = %t, want %t", nextCalled, tt.wantNext)
			}
			if tt.wantErrorCode != "" {
				var response httpdto.Response[any]
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if response.Error == nil || response.Error.Code != tt.wantErrorCode {
					t.Fatalf("unexpected error response: %+v", response)
				}
			}
		})
	}
}
