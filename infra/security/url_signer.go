package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// SignedToken represents a presigned URL token
type SignedToken struct {
	UserID    string
	VAID      string
	TokenID   string
	ExpiresAt time.Time
}

// SignedLinkData represents the data stored in Redis for a signed link
type SignedLinkData struct {
	UserID     string    `json:"user_id"`
	VAID       string    `json:"va_id"`
	RedirectTo string    `json:"redirect_to"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// URLSignerService generates and validates presigned URLs for dashboard access
type URLSignerService struct {
	secretKey []byte
	redis     *redis.Client
}

// NewURLSignerService creates a new URL signer service
func NewURLSignerService(secretKey []byte, redis *redis.Client) *URLSignerService {
	return &URLSignerService{
		secretKey: secretKey,
		redis:     redis,
	}
}

// GenerateSignedLink generates a signed link with redirect URL support
func (s *URLSignerService) GenerateSignedLink(
	ctx context.Context,
	userID, vaID, redirectTo string,
	ttl time.Duration,
) (string, error) {
	tokenID := uuid.New().String()
	expiresAt := time.Now().Add(ttl)

	// Default redirect to /dashboard if not specified
	if redirectTo == "" {
		redirectTo = "/dashboard"
	}

	// Create JWT claims
	claims := jwt.MapClaims{
		"user_id": userID,
		"va_id":   vaID,
		"jti":     tokenID,
		"exp":     expiresAt.Unix(),
		"iat":     time.Now().Unix(),
	}

	// Sign with HMAC
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	// Store redirect URL and metadata in Redis with same TTL
	linkData := SignedLinkData{
		UserID:     userID,
		VAID:       vaID,
		RedirectTo: redirectTo,
		ExpiresAt:  expiresAt,
	}
	dataJSON, err := json.Marshal(linkData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal link data: %w", err)
	}

	// Store in Redis with key: signed_link:{tokenID}
	redisKey := "signed_link:" + tokenID
	err = s.redis.Set(ctx, redisKey, dataJSON, ttl).Err()
	if err != nil {
		return "", fmt.Errorf("failed to store link data: %w", err)
	}

	return tokenString, nil
}

// GetSignedLinkData retrieves the redirect URL and metadata for a token
func (s *URLSignerService) GetSignedLinkData(ctx context.Context, tokenID string) (*SignedLinkData, error) {
	redisKey := "signed_link:" + tokenID
	val, err := s.redis.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		return nil, errors.New("link data not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get link data: %w", err)
	}

	var linkData SignedLinkData
	err = json.Unmarshal([]byte(val), &linkData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal link data: %w", err)
	}

	return &linkData, nil
}

// ValidateToken validates a presigned URL token
func (s *URLSignerService) ValidateToken(ctx context.Context, tokenString string) (*SignedToken, error) {
	// Parse and validate JWT
	token, err := jwt.ParseWithClaims(tokenString, &jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Extract claims
	userID, ok := (*claims)["user_id"].(string)
	if !ok {
		return nil, errors.New("missing or invalid user_id claim")
	}

	vaID, ok := (*claims)["va_id"].(string)
	if !ok {
		return nil, errors.New("missing or invalid va_id claim")
	}

	tokenID, ok := (*claims)["jti"].(string)
	if !ok {
		return nil, errors.New("missing or invalid jti claim")
	}

	expFloat, ok := (*claims)["exp"].(float64)
	if !ok {
		return nil, errors.New("missing or invalid exp claim")
	}
	expiresAt := time.Unix(int64(expFloat), 0)

	// Check if expired
	if time.Now().After(expiresAt) {
		return nil, errors.New("token expired")
	}

	// Check if token already used
	isUsed, err := s.IsTokenUsed(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to check token usage: %w", err)
	}
	if isUsed {
		return nil, errors.New("token already used")
	}

	return &SignedToken{
		UserID:    userID,
		VAID:      vaID,
		TokenID:   tokenID,
		ExpiresAt: expiresAt,
	}, nil
}

// MarkTokenAsUsed marks a token as used (single-use enforcement)
func (s *URLSignerService) MarkTokenAsUsed(ctx context.Context, tokenID string) error {
	// Store token ID in Redis with TTL matching token expiration
	// Using 15 minute default TTL if token expires later
	ttl := 15 * time.Minute

	err := s.redis.Set(ctx, "used_token:"+tokenID, "1", ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to mark token as used: %w", err)
	}

	return nil
}

// IsTokenUsed checks if a token has already been used
func (s *URLSignerService) IsTokenUsed(ctx context.Context, tokenID string) (bool, error) {
	result, err := s.redis.Get(ctx, "used_token:"+tokenID).Result()
	if err == redis.Nil {
		return false, nil // Token not used
	}
	if err != nil {
		return false, fmt.Errorf("failed to check token usage: %w", err)
	}
	return result == "1", nil
}
