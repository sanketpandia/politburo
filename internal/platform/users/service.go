package users

import (
	"context"
	"fmt"
	"log"

	"infinite-experiment/politburo/internal/models/dtos/responses"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) RegisterUser(ctx context.Context, discordID, ifCommunityID string, ifApiID *string, isActive bool) error {
	_, err := s.repo.InsertUser(ctx, discordID, ifCommunityID, ifApiID, isActive)
	return err
}

// GetUserDetails retrieves user details with VA affiliations and current VA status
// Note: This platform-level method provides basic user data only.
// VA enrichment (names, codes) is handled by feature layer (internal/memberships)
// which can orchestrate multiple platform services without circular dependencies.
func (s *Service) GetUserDetails(ctx context.Context, userDiscordID, vaDiscordServerID string) (*responses.UserDetailResponse, error) {
	// Fetch user with all VA affiliations
	user, err := s.repo.GetUserWithVAAffiliations(ctx, userDiscordID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user details: %w", err)
	}

	// Build affiliations list
	affiliations := make([]responses.VAAffiliation, 0, len(user.UserVARoles))
	var currentVA *responses.CurrentVAStatus

	for _, vaRole := range user.UserVARoles {
		// Note: VA details (Name, Code, DiscordID) are not preloaded in platform layer
		// to avoid circular dependencies. The feature layer should enrich this data
		// For now, we'll create minimal affiliations
		affiliation := responses.VAAffiliation{
			VAID:     vaRole.VAID,
			VAName:   "", // To be populated by feature layer
			VACode:   "", // To be populated by feature layer
			Role:     string(vaRole.Role),
			IsActive: vaRole.IsActive,
			JoinedAt: vaRole.JoinedAt,
			Callsign: vaRole.Callsign,
		}
		affiliations = append(affiliations, affiliation)

		// Check if this is the current VA (from context)
		// Note: We can't compare VA.DiscordID here without loading VA
		// This logic may need to be moved to feature layer
		log.Printf("[GetUserDetails] Processing VA affiliation: VAID=%s, Role=%s", vaRole.VAID, vaRole.Role)

		// For now, create a basic current VA status
		// Feature layer should override this with proper VA matching
		if currentVA == nil && vaRole.IsActive {
			currentVA = &responses.CurrentVAStatus{
				IsMember: true,
				Role:     string(vaRole.Role),
				IsActive: vaRole.IsActive,
				Callsign: vaRole.Callsign,
			}
		}
	}

	// If current VA not found in affiliations, set IsMember to false
	if currentVA == nil {
		currentVA = &responses.CurrentVAStatus{
			IsMember: false,
			IsActive: false,
		}
	}

	// Build response
	response := &responses.UserDetailResponse{
		UserID:        user.ID,
		DiscordID:     user.DiscordID,
		IFCommunityID: user.IFCommunityID,
		IFApiID:       user.IFApiID,
		UserName:      user.UserName,
		IsActive:      user.IsActive,
		CreatedAt:     user.CreatedAt,
		Affiliations:  affiliations,
		CurrentVA:     currentVA,
	}

	return response, nil
}

// CreateMembership creates a new user-VA membership with role and callsign
func (s *Service) CreateMembership(ctx context.Context, userID, vaID string, role, callsign string) (*UserVARole, error) {
	return s.repo.InsertMembership(ctx, userID, vaID, role, callsign)
}

// GetUserByCallsignAndVA checks if a callsign is available in a VA
// Returns the membership if callsign is taken, nil if available
func (s *Service) GetUserByCallsignAndVA(ctx context.Context, callsign string, vaID string) (*UserVARole, error) {
	return s.repo.GetUserByCallsignAndVA(ctx, callsign, vaID)
}

// GetByDiscordID retrieves a user by Discord ID
func (s *Service) GetByDiscordID(ctx context.Context, discordID string) (*User, error) {
	return s.repo.GetUserByDiscordID(ctx, discordID)
}

// GetByID retrieves a user by ID
func (s *Service) GetByID(ctx context.Context, userID string) (*User, error) {
	return s.repo.GetByID(ctx, userID)
}
