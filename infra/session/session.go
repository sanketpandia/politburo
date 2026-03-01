package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// VAMembership represents a user's membership in a virtual airline
type VAMembership struct {
	VAID            string `json:"va_id"`
	VACode          string `json:"va_code"`
	VAName          string `json:"va_name"`
	Role            string `json:"role"`
	DiscordServerID string `json:"discord_server_id"`
}

// SessionData represents a user's session with multi-VA support
type SessionData struct {
	SessionID       string         `json:"session_id"`
	UserID          string         `json:"user_id"`
	ActiveVAID      string         `json:"active_va_id"`
	DiscordID       string         `json:"discord_id"`
	DiscordServerID string         `json:"discord_server_id"`
	Username        string         `json:"username"`
	VirtualAirlines []VAMembership `json:"virtual_airlines"`
	CreatedAt       time.Time      `json:"created_at"`
	ExpiresAt       time.Time      `json:"expires_at"`
}

// SessionService manages user sessions in Redis
type SessionService struct {
	redis *redis.Client
}

// NewSessionService creates a new session service
func NewSessionService(redis *redis.Client) *SessionService {
	return &SessionService{
		redis: redis,
	}
}

// CreateSession creates a new session for a user with their VAs
func (s *SessionService) CreateSession(
	ctx context.Context,
	userID, activeVAID, discordID, discordServerID, username string,
	virtualAirlines []VAMembership,
) (string, error) {
	sessionID := uuid.New().String()

	now := time.Now()
	expiresAt := now.Add(7 * 24 * time.Hour) // 7 days

	session := SessionData{
		SessionID:       sessionID,
		UserID:          userID,
		ActiveVAID:      activeVAID,
		DiscordID:       discordID,
		DiscordServerID: discordServerID,
		Username:        username,
		VirtualAirlines: virtualAirlines,
		CreatedAt:       now,
		ExpiresAt:       expiresAt,
	}

	log.Printf("[SessionService] CreateSession: sessionID=%s, userID=%s, numVAs=%d",
		sessionID, userID, len(virtualAirlines))

	// Serialize to JSON
	data, err := json.Marshal(session)
	if err != nil {
		log.Printf("[SessionService] ERROR: Failed to marshal session: %v", err)
		return "", fmt.Errorf("failed to marshal session: %w", err)
	}

	log.Printf("[SessionService] Session serialized, data length=%d bytes", len(data))

	// Store in Redis with 7-day TTL
	ttl := 7 * 24 * time.Hour
	log.Printf("[SessionService] About to store in Redis with key: session:%s, TTL: %v", sessionID, ttl)

	err = s.redis.Set(ctx, "session:"+sessionID, data, ttl).Err()
	if err != nil {
		log.Printf("[SessionService] ERROR: Failed to store session in Redis: %v", err)
		return "", fmt.Errorf("failed to store session: %w", err)
	}

	log.Printf("[SessionService] SUCCESS: Session stored in Redis")
	return sessionID, nil
}

// GetSession retrieves a session from Redis
func (s *SessionService) GetSession(ctx context.Context, sessionID string) (*SessionData, error) {
	log.Printf("[SessionService] GetSession: Looking for session with ID=%s", sessionID)

	val, err := s.redis.Get(ctx, "session:"+sessionID).Result()
	if err != nil {
		if err == redis.Nil {
			log.Printf("[SessionService] ERROR: Session not found in Redis for ID=%s", sessionID)
			return nil, errors.New("session not found")
		}
		log.Printf("[SessionService] ERROR: Redis error getting session: %v", err)
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	log.Printf("[SessionService] Found session data in Redis, length=%d bytes", len(val))

	var session SessionData
	err = json.Unmarshal([]byte(val), &session)
	if err != nil {
		log.Printf("[SessionService] ERROR: Failed to unmarshal session: %v", err)
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	log.Printf("[SessionService] Session unmarshaled successfully: userID=%s, activeVA=%s",
		session.UserID, session.ActiveVAID)

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		log.Printf("[SessionService] WARNING: Session expired for ID=%s", sessionID)
		s.DeleteSession(ctx, sessionID) // Clean up expired session
		return nil, errors.New("session expired")
	}

	return &session, nil
}

// DeleteSession deletes a session from Redis
func (s *SessionService) DeleteSession(ctx context.Context, sessionID string) error {
	err := s.redis.Del(ctx, "session:"+sessionID).Err()
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// RefreshSession extends the session expiration
func (s *SessionService) RefreshSession(ctx context.Context, sessionID string) error {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Update expiration
	session.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)

	// Serialize and store
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	ttl := 7 * 24 * time.Hour
	err = s.redis.Set(ctx, "session:"+sessionID, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}

	return nil
}

// SwitchActiveVA updates the active VA in a session
func (s *SessionService) SwitchActiveVA(ctx context.Context, sessionID, newVAID string) error {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Verify user belongs to new VA
	if !session.HasVA(newVAID) {
		return errors.New("user is not a member of this VA")
	}

	// Update active VA
	session.ActiveVAID = newVAID

	// Save back to Redis
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	ttl := 7 * 24 * time.Hour
	err = s.redis.Set(ctx, "session:"+sessionID, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// GetActiveVA returns the active VA membership
func (s *SessionData) GetActiveVA() *VAMembership {
	for i, va := range s.VirtualAirlines {
		if va.VAID == s.ActiveVAID {
			return &s.VirtualAirlines[i]
		}
	}
	return nil
}

// HasVA checks if user is a member of a specific VA
func (s *SessionData) HasVA(vaID string) bool {
	for _, va := range s.VirtualAirlines {
		if va.VAID == vaID {
			return true
		}
	}
	return false
}

// DeleteAllSessionsForUser deletes all sessions for a given user ID
// This scans all session keys in Redis and deletes those matching the userID
func (s *SessionService) DeleteAllSessionsForUser(ctx context.Context, userID string) (int, error) {
	log.Printf("[SessionService] DeleteAllSessionsForUser: Looking for sessions for userID=%s", userID)

	// Scan all session keys with pattern "session:*"
	var cursor uint64
	var deletedCount int
	var keysToDelete []string

	// Use SCAN to iterate through all session keys
	for {
		var keys []string
		var err error
		keys, cursor, err = s.redis.Scan(ctx, cursor, "session:*", 100).Result()
		if err != nil {
			return 0, fmt.Errorf("failed to scan session keys: %w", err)
		}

		// Check each key to see if it belongs to the user
		for _, key := range keys {
			val, err := s.redis.Get(ctx, key).Result()
			if err != nil {
				if err == redis.Nil {
					continue // Key was deleted between scan and get
				}
				log.Printf("[SessionService] ERROR: Failed to get session %s: %v", key, err)
				continue
			}

			var session SessionData
			if err := json.Unmarshal([]byte(val), &session); err != nil {
				log.Printf("[SessionService] ERROR: Failed to unmarshal session %s: %v", key, err)
				continue
			}

			// If this session belongs to the user, mark it for deletion
			if session.UserID == userID {
				keysToDelete = append(keysToDelete, key)
			}
		}

		// If cursor is 0, we've scanned all keys
		if cursor == 0 {
			break
		}
	}

	// Delete all matching sessions
	if len(keysToDelete) > 0 {
		err := s.redis.Del(ctx, keysToDelete...).Err()
		if err != nil {
			return 0, fmt.Errorf("failed to delete sessions: %w", err)
		}
		deletedCount = len(keysToDelete)
		log.Printf("[SessionService] DeleteAllSessionsForUser: Deleted %d sessions for userID=%s", deletedCount, userID)
	} else {
		log.Printf("[SessionService] DeleteAllSessionsForUser: No sessions found for userID=%s", userID)
	}

	return deletedCount, nil
}
