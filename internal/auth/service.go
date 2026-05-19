package auth

import (
	"context"
	"fmt"
	"time"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/security"
	"infinite-experiment/politburo/infra/session"
	platformClaims "infinite-experiment/politburo/internal/platform/claims"
	platformUsers "infinite-experiment/politburo/internal/platform/users"
)

// VAService interface to avoid circular dependency with platform/va
type VAService interface {
	GetByDiscordServerID(ctx context.Context, discordServerID string) (VAInfo, error)
}

// VAInfo represents minimal VA information needed by auth service
type VAInfo struct {
	ID   string
	Name string
	Code string
}

// Service provides authentication business logic
type Service struct {
	sessionSvc *session.SessionService
	urlSigner  *security.URLSignerService
	claimsRepo *platformClaims.Repository
	usersSvc   *platformUsers.Service
	vaSvc      VAService
}

// NewService creates a new auth service
func NewService(
	sessionSvc *session.SessionService,
	urlSigner *security.URLSignerService,
	claimsRepo *platformClaims.Repository,
	usersSvc *platformUsers.Service,
	vaSvc VAService,
) *Service {
	return &Service{
		sessionSvc: sessionSvc,
		urlSigner:  urlSigner,
		claimsRepo: claimsRepo,
		usersSvc:   usersSvc,
		vaSvc:      vaSvc,
	}
}

// CreateSessionFromTokenResult represents the result of creating a session from a token
type CreateSessionFromTokenResult struct {
	SessionID  string
	RedirectTo string
}

// CreateSessionFromToken validates a token, creates a session, and returns redirect URL
func (s *Service) CreateSessionFromToken(ctx context.Context, tokenString string) (*CreateSessionFromTokenResult, error) {
	// Validate token
	signedToken, err := s.urlSigner.ValidateToken(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired token: %w", err)
	}

	// Get redirect URL from Redis
	linkData, err := s.urlSigner.GetSignedLinkData(ctx, signedToken.TokenID)
	redirectTo := "/dashboard" // Default fallback
	if err == nil && linkData != nil && linkData.RedirectTo != "" {
		redirectTo = linkData.RedirectTo
	}

	// Mark token as used (single-use enforcement)
	err = s.urlSigner.MarkTokenAsUsed(ctx, signedToken.TokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to mark token as used: %w", err)
	}

	// Fetch user data by ID
	user, err := s.usersSvc.GetByID(ctx, signedToken.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	discordID := user.DiscordID
	username := ""
	if user.UserName != nil {
		username = *user.UserName
	}

	// Fetch all VAs for this user
	vaMemberships, err := s.claimsRepo.GetAllVAMembershipsByUserID(ctx, signedToken.UserID)
	if err != nil {
		logging.Error("Failed to load user VAs", "error", err, "user_id", signedToken.UserID)
		return nil, fmt.Errorf("failed to load user VAs: %w", err)
	}

	// Convert to session VAMembership array
	var virtualAirlines []session.VAMembership
	for _, vaMembership := range vaMemberships {
		virtualAirlines = append(virtualAirlines, session.VAMembership{
			VAID:            vaMembership.VAID,
			VACode:          vaMembership.VACode,
			VAName:          vaMembership.VAName,
			Role:            string(vaMembership.Role),
			DiscordServerID: vaMembership.DiscordServerID,
		})
	}

	// Create session with default VA from token
	sessionID, err := s.sessionSvc.CreateSession(
		ctx,
		signedToken.UserID,
		signedToken.VAID,
		discordID,
		"", // Discord server ID will be set from active VA
		username,
		virtualAirlines,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	logging.Info("Session created from token", "session_id", sessionID, "user_id", signedToken.UserID, "num_vas", len(virtualAirlines))

	return &CreateSessionFromTokenResult{
		SessionID:  sessionID,
		RedirectTo: redirectTo,
	}, nil
}

// GetUserAndVAFromDiscordIDs looks up user and VA by Discord IDs
func (s *Service) GetUserAndVAFromDiscordIDs(ctx context.Context, discordUserID, discordServerID string) (*platformUsers.User, VAInfo, error) {
	// Get user by Discord ID
	user, err := s.usersSvc.GetByDiscordID(ctx, discordUserID)
	if err != nil {
		return nil, VAInfo{}, fmt.Errorf("user not found: %w", err)
	}

	// Get VA by Discord server ID
	va, err := s.vaSvc.GetByDiscordServerID(ctx, discordServerID)
	if err != nil {
		return nil, VAInfo{}, fmt.Errorf("VA not found: %w", err)
	}

	return user, va, nil
}

// GenerateSignedLink generates a signed link with redirect URL
func (s *Service) GenerateSignedLink(
	ctx context.Context,
	userID, vaID, redirectTo string,
	ttl time.Duration,
) (string, error) {
	return s.urlSigner.GenerateSignedLink(ctx, userID, vaID, redirectTo, ttl)
}

func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	s.sessionSvc.DeleteSession(ctx, sessionID)
	return nil
}

// DestroyAllSessionsByIFCId destroys all sessions for a user identified by their IFC ID
func (s *Service) DestroyAllSessionsByIFCId(ctx context.Context, ifcId string) (int, error) {
	// Look up user by IFC ID
	user, err := s.usersSvc.GetUserByIFCId(ctx, ifcId)
	if err != nil {
		return 0, fmt.Errorf("failed to lookup user by IFC ID: %w", err)
	}
	if user == nil {
		return 0, fmt.Errorf("user not found with IFC ID: %s", ifcId)
	}

	// Delete all sessions for this user
	deletedCount, err := s.sessionSvc.DeleteAllSessionsForUser(ctx, user.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete sessions: %w", err)
	}

	logging.Info("Destroyed all sessions for user", "ifc_id", ifcId, "user_id", user.ID, "deleted_count", deletedCount)
	return deletedCount, nil
}
