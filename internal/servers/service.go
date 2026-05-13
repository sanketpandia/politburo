package servers

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/platform/roles"
	"infinite-experiment/politburo/internal/platform/users"
	"infinite-experiment/politburo/internal/platform/va"
)

type RegistrationService struct {
	usersSvc  *users.Service
	vaSvc     *va.Service
	usersRepo *users.Repository
}

func NewRegistrationService(
	usersSvc *users.Service,
	vaSvc *va.Service,
	usersRepo *users.Repository,
) *RegistrationService {
	return &RegistrationService{
		usersSvc:  usersSvc,
		vaSvc:     vaSvc,
		usersRepo: usersRepo,
	}
}

// InitServer registers a new VA for the given Discord server.
// On failure it returns a *ServerError so the handler can respond without a
// switch dispatch.
func (s *RegistrationService) InitServer(
	ctx context.Context,
	discordServerID string,
	discordUserID string,
	vaCode string,
	vaName string,
	callsignPrefix string,
	callsignSuffix string,
) (*InitServerResponse, *ServerError) {

	// 1. Validate required fields
	if vaCode == "" || vaName == "" {
		return nil, sentinelToServerError(ErrVACreationFailed)
	}

	// 2. Validate at least one callsign pattern exists
	if callsignPrefix == "" && callsignSuffix == "" {
		return nil, sentinelToServerError(ErrInvalidCallsignConfig)
	}

	// 3. Check if server already registered
	existingVA, err := s.vaSvc.GetByDiscordServerID(ctx, discordServerID)
	if err == nil && existingVA != nil {
		return nil, sentinelToServerError(ErrServerAlreadyRegistered)
	}

	// 4. Check if user is registered
	user, err := s.usersRepo.GetUserByDiscordID(ctx, discordUserID)
	if err != nil || user == nil {
		return nil, sentinelToServerError(ErrUserNotRegistered)
	}

	// 5. Create VA
	newVA := &va.VA{
		Name:      vaName,
		Code:      vaCode,
		DiscordID: discordServerID,
		IsActive:  true,
	}

	if err := s.vaSvc.Create(ctx, newVA); err != nil {
		return nil, sentinelToServerError(fmt.Errorf("%w: %v", ErrVACreationFailed, err))
	}

	// 6. Store callsign configs
	if callsignPrefix != "" {
		if err := s.vaSvc.UpsertConfig(ctx, newVA.ID, "callsign_prefix", callsignPrefix); err != nil {
			return nil, sentinelToServerError(fmt.Errorf("failed to store callsign prefix: %w", err))
		}
	}

	if callsignSuffix != "" {
		if err := s.vaSvc.UpsertConfig(ctx, newVA.ID, "callsign_suffix", callsignSuffix); err != nil {
			return nil, sentinelToServerError(fmt.Errorf("failed to store callsign suffix: %w", err))
		}
	}

	// 7. Create admin membership (no callsign needed for admin role)
	_, err = s.usersRepo.InsertMembership(
		ctx,
		user.ID,
		newVA.ID,
		string(roles.RoleAdmin),
		"", // Empty callsign for admin
	)
	if err != nil {
		return nil, sentinelToServerError(fmt.Errorf("failed to create admin membership: %w", err))
	}

	return &InitServerResponse{
		Success: true,
		Message: "Server initialized successfully",
		VACode:  newVA.Code,
		VAID:    newVA.ID,
	}, nil
}
