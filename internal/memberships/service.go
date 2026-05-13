package memberships

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/pilots"
	platformMemberships "infinite-experiment/politburo/internal/platform/memberships"
	platformUsers "infinite-experiment/politburo/internal/platform/users"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

// Service provides feature-layer membership business logic
// Orchestrates platform services to provide enriched user status
type Service struct {
	membershipsSvc *platformMemberships.Service // Platform memberships service (for GetUserStatus)
	usersSvc       *platformUsers.Service       // Platform users service
	vaSvc          *platformVA.Service          // Platform VA service
	pilotRepo      *pilots.Repository           // Pilot repository for Airtable validation
}

// NewService creates a new feature-layer memberships service
func NewService(
	membershipsSvc *platformMemberships.Service,
	usersSvc *platformUsers.Service,
	vaSvc *platformVA.Service,
	pilotRepo *pilots.Repository,
) *Service {
	return &Service{
		membershipsSvc: membershipsSvc,
		usersSvc:       usersSvc,
		vaSvc:          vaSvc,
		pilotRepo:      pilotRepo,
	}
}

// GetUserStatus retrieves user status with all VA affiliations and current VA context
// Feature-layer method that transforms platform data to API response format
func (s *Service) GetUserStatus(ctx context.Context, userID, vaID string) (*UserDetailResponse, error) {
	// Log request
	logging.Info("GetUserStatus called", "user_id", userID, "va_id", vaID)

	// Call platform service
	result, err := s.membershipsSvc.GetUserStatusByUserID(ctx, userID, vaID)
	if err != nil {
		logging.Error("Failed to get user status from platform", "error", err, "user_id", userID)
		return nil, fmt.Errorf("failed to fetch user status: %w", err)
	}

	// Transform to API response
	response := transformToUserDetailResponse(result)

	logging.Debug("User status fetched successfully", "user_id", userID, "affiliations_count", len(result.Affiliations))
	return response, nil
}

// transformToUserDetailResponse converts platform UserStatusResult to API response format
func transformToUserDetailResponse(result *platformMemberships.UserStatusResult) *UserDetailResponse {
	// Convert affiliations
	affiliations := make([]VAAffiliation, 0, len(result.Affiliations))
	for _, aff := range result.Affiliations {
		affiliations = append(affiliations, VAAffiliation{
			VAID:     aff.VAID,
			VAName:   aff.VAName,
			VACode:   aff.VACode,
			Role:     string(aff.Role), // Convert VARole to string
			IsActive: aff.IsActive,
			JoinedAt: aff.JoinedAt,
			Callsign: aff.Callsign,
		})
	}

	// Convert current VA status
	var currentVA *CurrentVAStatus
	if result.CurrentVA != nil {
		currentVA = &CurrentVAStatus{
			IsMember: result.CurrentVA.IsMember,
		}

		// Only set these fields if user is a member
		if result.CurrentVA.IsMember && result.CurrentVA.Role != nil {
			currentVA.Role = string(*result.CurrentVA.Role)
			if result.CurrentVA.IsActive != nil {
				currentVA.IsActive = *result.CurrentVA.IsActive
			}
			if result.CurrentVA.Callsign != nil {
				currentVA.Callsign = *result.CurrentVA.Callsign
			}
		}
	}

	return &UserDetailResponse{
		UserID:        result.UserID,
		DiscordID:     result.DiscordID,
		IFCommunityID: result.IFCommunityID,
		IFApiID:       result.IFApiID,
		UserName:      nil, // Not available in platform data (would need users service)
		IsActive:      result.IsActive,
		CreatedAt:     result.CreatedAt,
		Affiliations:  affiliations,
		CurrentVA:     currentVA,
	}
}

// JoinVA allows an authenticated user to join a VA with a callsign.
// On failure it returns a *MembershipError so the handler can respond without
// a switch dispatch.
func (s *Service) JoinVA(
	ctx context.Context,
	discordUserID string,
	discordServerID string,
	callsign string,
) (*JoinVAResponse, *MembershipError) {
	logging.Info("Processing JoinVA request", "discord_user_id", discordUserID, "callsign", callsign)

	// 1. Validate callsign format (not empty, reasonable length)
	if callsign == "" || len(callsign) > 20 {
		return nil, sentinelToMembershipError(ErrInvalidCallsign)
	}

	// 2. Get user by Discord ID (uses platform users service)
	user, err := s.usersSvc.GetByDiscordID(ctx, discordUserID)
	if err != nil || user == nil {
		logging.Warn("User not found", "discord_user_id", discordUserID, "error", err)
		return nil, sentinelToMembershipError(ErrUserNotFound)
	}

	// 3. Get VA by Discord server ID (uses platform VA service)
	va, err := s.vaSvc.GetByDiscordServerID(ctx, discordServerID)
	if err != nil || va == nil {
		logging.Warn("VA not found", "discord_server_id", discordServerID, "error", err)
		return nil, sentinelToMembershipError(ErrVANotFound)
	}

	// 4. Validate callsign exists in Airtable (pilot_at_synced)
	if s.pilotRepo != nil {
		pilotSync, err := s.pilotRepo.FindByCallsign(ctx, va.ID, callsign)
		if err != nil {
			logging.Error("Failed to check callsign in Airtable", "error", err, "callsign", callsign, "va_id", va.ID)
			return nil, sentinelToMembershipError(fmt.Errorf("failed to validate callsign: %w", err))
		}

		if pilotSync == nil {
			logging.Warn("Callsign not found in Airtable", "callsign", callsign, "va_id", va.ID)
			return nil, sentinelToMembershipError(ErrCallsignNotInAirtable)
		}

		logging.Info("Callsign validated in Airtable", "callsign", callsign, "va_id", va.ID, "airtable_id", pilotSync.ATID)
	}

	// 5. Check if callsign is already taken in this VA (using platform users service)
	existingUser, err := s.usersSvc.GetUserByCallsignAndVA(ctx, callsign, va.ID)
	if err != nil {
		logging.Error("Failed to check callsign availability", "error", err)
		return nil, sentinelToMembershipError(fmt.Errorf("failed to validate callsign: %w", err))
	}

	if existingUser != nil {
		if existingUser.UserID == user.ID {
			logging.Warn("User already has membership with this callsign", "user_id", user.ID, "callsign", callsign)
			return nil, sentinelToMembershipError(ErrUserAlreadyMember)
		}
		logging.Warn("Callsign already taken", "callsign", callsign, "va_id", va.ID)
		return nil, sentinelToMembershipError(ErrCallsignTaken)
	}

	// 6. Create new membership (using platform users service)
	logging.Info("Creating membership", "user_id", user.ID, "va_id", va.ID, "callsign", callsign)

	membership, err := s.usersSvc.CreateMembership(ctx, user.ID, va.ID, "pilot", callsign)
	if err != nil {
		logging.Error("Failed to create membership", "error", err)
		return nil, sentinelToMembershipError(fmt.Errorf("%w: %v", ErrMembershipCreation, err))
	}

	// 7. Link to Airtable ID immediately if pilot repo is available and callsign was found.
	// Failure here is non-fatal; the sync job will catch it.
	if s.pilotRepo != nil && membership != nil {
		pilotSync, err := s.pilotRepo.FindByCallsign(ctx, va.ID, callsign)
		if err == nil && pilotSync != nil {
			if linkErr := s.pilotRepo.UpdateUserAirtableID(ctx, membership.ID, pilotSync.ATID); linkErr != nil {
				logging.Warn("Failed to link Airtable ID immediately", "error", linkErr, "membership_id", membership.ID, "airtable_id", pilotSync.ATID)
			} else {
				logging.Info("Linked Airtable ID immediately", "membership_id", membership.ID, "airtable_id", pilotSync.ATID)
			}
		}
	}

	logging.Info("Membership created successfully", "user_id", user.ID, "va_id", va.ID)
	return &JoinVAResponse{
		Success:  true,
		Message:  "Successfully joined VA",
		UserID:   user.ID,
		VAID:     va.ID,
		Callsign: callsign,
		Role:     "pilot",
	}, nil
}
