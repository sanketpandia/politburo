package apikeys

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"infinite-experiment/politburo/internal/cache"
)

type repoStub struct {
	active bool
	found  bool
	err    error
	calls  int
}

func (s *repoStub) Active(context.Context, string) (bool, bool, error) {
	s.calls++
	return s.active, s.found, s.err
}

type cacheMem struct {
	data map[string][]byte
}

func newCacheMem() *cacheMem {
	return &cacheMem{data: make(map[string][]byte)}
}

func (c *cacheMem) GetJSON(_ context.Context, key string, destination any) error {
	raw, ok := c.data[key]
	if !ok {
		return cache.ErrMiss
	}
	return json.Unmarshal(raw, destination)
}

func (c *cacheMem) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.data[key] = raw
	return nil
}

func TestLookupCachesActiveStatus(t *testing.T) {
	repo := &repoStub{active: true, found: true}
	lookup := NewLookup(repo, newCacheMem())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/game/sessions/active", nil)

	claims, ok, err := lookup.Lookup(req, "key-1")
	if err != nil || !ok || !claims.APIKeyPresent {
		t.Fatalf("first lookup: ok=%v err=%v claims=%#v", ok, err, claims)
	}
	_, ok, err = lookup.Lookup(req, "key-1")
	if err != nil || !ok {
		t.Fatalf("second lookup: ok=%v err=%v", ok, err)
	}
	if repo.calls != 1 {
		t.Fatalf("repo calls = %d, want 1 (cached)", repo.calls)
	}
}

func TestLookupRejectsInactiveAndMissing(t *testing.T) {
	lookup := NewLookup(&repoStub{active: false, found: true}, newCacheMem())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok, err := lookup.Lookup(req, "inactive"); err != nil || ok {
		t.Fatalf("inactive key: ok=%v err=%v", ok, err)
	}

	lookup = NewLookup(&repoStub{found: false}, newCacheMem())
	if _, ok, err := lookup.Lookup(req, "missing"); err != nil || ok {
		t.Fatalf("missing key: ok=%v err=%v", ok, err)
	}
}

func TestLookupPropagatesRepoError(t *testing.T) {
	lookup := NewLookup(&repoStub{err: errors.New("db down")}, newCacheMem())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, _, err := lookup.Lookup(req, "key"); err == nil {
		t.Fatal("expected error")
	}
}
