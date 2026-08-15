package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"infinite-experiment/politburo/internal/cache"
)

type memCache struct {
	data map[string][]byte
}

func newMemCache() *memCache {
	return &memCache{data: map[string][]byte{}}
}

func (c *memCache) GetJSON(_ context.Context, key string, dest any) error {
	raw, ok := c.data[key]
	if !ok {
		return cache.ErrMiss
	}
	return json.Unmarshal(raw, dest)
}

func (c *memCache) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.data[key] = raw
	return nil
}

func (c *memCache) Delete(_ context.Context, key string) error {
	delete(c.data, key)
	return nil
}

func (c *memCache) GetDelJSON(_ context.Context, key string, dest any) error {
	raw, ok := c.data[key]
	if !ok {
		return cache.ErrMiss
	}
	delete(c.data, key)
	return json.Unmarshal(raw, dest)
}

func testSecret() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

func TestTicketIssueAndConsumeOnce(t *testing.T) {
	tickets := NewTickets(newMemCache(), testSecret())
	token, err := tickets.Issue(context.Background(), LoginTicket{
		UserID: "user-1", DiscordUserID: "ds-1", DiscordServerID: "guild-1", RedirectTo: "/dashboard",
	})
	if err != nil || token == "" {
		t.Fatalf("Issue() token=%q err=%v", token, err)
	}
	got, err := tickets.Consume(context.Background(), token)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if got.UserID != "user-1" || got.DiscordUserID != "ds-1" || got.DiscordServerID != "guild-1" {
		t.Fatalf("ticket = %#v", got)
	}
	if _, err := tickets.Consume(context.Background(), token); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("second Consume() error = %v, want ErrInvalidTicket", err)
	}
}

func TestTicketRejectsGarbageAndWrongKey(t *testing.T) {
	tickets := NewTickets(newMemCache(), testSecret())
	if _, err := tickets.Consume(context.Background(), "not-a-token"); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("garbage Consume() error = %v", err)
	}
	token, err := tickets.Issue(context.Background(), LoginTicket{UserID: "user-1", DiscordUserID: "ds-1", RedirectTo: "/dashboard"})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	other := NewTickets(newMemCache(), []byte("abcdef0123456789abcdef0123456789"))
	if _, err := other.Consume(context.Background(), token); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("wrong key Consume() error = %v", err)
	}
}

func TestNormalizeRedirect(t *testing.T) {
	got, err := NormalizeRedirect("")
	if err != nil || got != DefaultRedirect {
		t.Fatalf("empty = %q err=%v", got, err)
	}
	got, err = NormalizeRedirect("/maps/flights/active")
	if err != nil || got != "/maps/flights/active" {
		t.Fatalf("relative = %q err=%v", got, err)
	}
	for _, bad := range []string{"https://evil.example", "//evil.example", "dashboard"} {
		if _, err := NormalizeRedirect(bad); err == nil {
			t.Fatalf("NormalizeRedirect(%q) error = nil", bad)
		}
	}
}

func TestFormatLoginURL(t *testing.T) {
	url := FormatLoginURL("https://ui.example", "abc+def")
	if url != "https://ui.example/auth/login?token=abc%2Bdef" {
		t.Fatalf("url = %q", url)
	}
}
