package servers

import (
	"context"
	"fmt"
	"strings"

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
) (*InitServerResponse, *ServerError) {
	vaCode = strings.ToUpper(strings.TrimSpace(vaCode))

	// 1. Validate required fields
	if vaCode == "" {
		return nil, sentinelToServerError(ErrVACreationFailed)
	}

	// 2. Check if server already registered
	existingVA, err := s.vaSvc.GetByDiscordServerID(ctx, discordServerID)
	if err == nil && existingVA != nil {
		return nil, sentinelToServerError(ErrServerAlreadyRegistered)
	}

	// 3. Check if the admin-facing VA code is already in use.
	existingCodeVA, err := s.vaSvc.GetByCode(ctx, vaCode)
	if err == nil && existingCodeVA != nil {
		return nil, sentinelToServerError(ErrVACodeAlreadyExists)
	}

	// 4. Check if user is registered
	user, err := s.usersRepo.GetUserByDiscordID(ctx, discordUserID)
	if err != nil || user == nil {
		return nil, sentinelToServerError(ErrUserNotRegistered)
	}

	// 5. Create minimal VA. Display name starts as the VA code; admins can update
	// it later from Vizburo Basic Setup.
	newVA := &va.VA{
		Name:      vaCode,
		Code:      vaCode,
		DiscordID: discordServerID,
		IsActive:  true,
	}

	if err := s.vaSvc.Create(ctx, newVA); err != nil {
		return nil, sentinelToServerError(fmt.Errorf("%w: %v", ErrVACreationFailed, err))
	}

	// 6. Create admin membership (no callsign needed for admin role)
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
		Success:       true,
		Message:       "Server initialized successfully. Continue setup in Vizburo to enable live-flight matching.",
		VACode:        newVA.Code,
		SetupRequired: true,
	}, nil
}
