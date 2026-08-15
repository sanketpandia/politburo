package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"infinite-experiment/politburo/internal/auth"
	"infinite-experiment/politburo/internal/cache"

	"github.com/google/uuid"
)

const TTL = 7 * 24 * time.Hour

type JSONCache interface {
	GetJSON(context.Context, string, any) error
	SetJSON(context.Context, string, any, time.Duration) error
	Delete(context.Context, string) error
}

type Service struct {
	cache JSONCache
	now   func() time.Time
}

func NewService(store JSONCache) *Service {
	return &Service{cache: store, now: time.Now}
}

type CreateInput struct {
	UserID          string
	DiscordID       string
	DiscordServerID string
	Username        string
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Session, error) {
	now := s.now().UTC()
	session := Session{
		SessionID:       uuid.NewString(),
		UserID:          input.UserID,
		DiscordID:       input.DiscordID,
		DiscordServerID: input.DiscordServerID,
		Username:        input.Username,
		CreatedAt:       now,
		ExpiresAt:       now.Add(TTL),
	}
	if err := s.cache.SetJSON(ctx, cache.KeySession(session.SessionID), session, TTL); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) Get(ctx context.Context, sessionID string) (Session, error) {
	var session Session
	if err := s.cache.GetJSON(ctx, cache.KeySession(sessionID), &session); err != nil {
		return Session{}, err
	}
	if session.Expired(s.now()) {
		_ = s.Delete(ctx, sessionID)
		return Session{}, cache.ErrMiss
	}
	return session, nil
}

func (s *Service) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	return s.cache.Delete(ctx, cache.KeySession(sessionID))
}

func (s *Service) Lookup(r *http.Request) (auth.Claims, bool, error) {
	sessionID := SessionIDFromRequest(r)
	if sessionID == "" {
		return auth.Claims{}, false, nil
	}
	session, err := s.Get(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, cache.ErrMiss) {
			return auth.Claims{}, false, nil
		}
		return auth.Claims{}, false, err
	}
	return session.Claims(), true, nil
}
