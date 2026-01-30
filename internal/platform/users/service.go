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
// Note: This method builds VAAffiliation responses but cannot preload VA details
// due to platform layer restrictions (platform cannot depend on feature packages)
// VA details will need to be enriched by the calling feature layer if needed
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
			VAName:   "",        // To be populated by feature layer
			VACode:   "",        // To be populated by feature layer
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
