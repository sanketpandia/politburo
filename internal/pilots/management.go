package pilots

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/platform/roles"
)

// ManagementService handles pilot management operations
type ManagementService struct {
	vaRoleRepo *repositories.VAUserRoleRepository
}

// NewManagementService creates a new pilot management service
func NewManagementService(vaRoleRepo *repositories.VAUserRoleRepository) *ManagementService {
	return &ManagementService{
		vaRoleRepo: vaRoleRepo,
	}
}

// GetPilotsByVAID retrieves all pilots for a specific VA with formatted data
func (s *ManagementService) GetPilotsByVAID(ctx context.Context, vaID string, requestorRole roles.VARole) ([]PilotDTO, error) {
	vaRoles, err := s.vaRoleRepo.GetAllByVAID(ctx, vaID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pilots: %w", err)
	}

	var pilots []PilotDTO
	for _, vaRole := range vaRoles {
		// Check if requestor can modify this pilot
		canModify := requestorRole == roles.RoleAdmin

		pilot := PilotDTO{
			ID:            vaRole.ID,
			UserID:        vaRole.UserID,
			IFCommunityID: vaRole.User.IFCommunityID,
			Callsign:      vaRole.Callsign,
			Role:          string(vaRole.Role),
			JoinedAt:      vaRole.JoinedAt.Format("2006-01-02"),
			IsActive:      vaRole.IsActive,
			UpdatedAt:     vaRole.UpdatedAt.Format("2006-01-02"),
			CanRemove:     canModify,
			CanChangeRole: canModify,
		}
		pilots = append(pilots, pilot)
	}

	return pilots, nil
}

// UpdatePilotRole updates a pilot's role with validation
func (s *ManagementService) UpdatePilotRole(
	ctx context.Context,
	vaID string,
	pilotID string,
	newRole string,
	requestorRole roles.VARole,
) error {
	// Only admins can change roles
	if requestorRole != roles.RoleAdmin {
		return fmt.Errorf("only admins can change pilot roles")
	}

	// Validate new role is valid
	switch newRole {
	case string(roles.RolePilot), string(roles.RoleAirlineManager), string(roles.RoleAdmin):
		// Valid role
	default:
		return fmt.Errorf("invalid role: %s", newRole)
	}

	// Get the pilot's current role
	pilot, err := s.vaRoleRepo.GetByID(ctx, pilotID)
	if err != nil {
		return fmt.Errorf("pilot not found: %w", err)
	}

	// Verify pilot belongs to this VA
	if pilot.VAID != vaID {
		return fmt.Errorf("pilot does not belong to this VA")
	}

	// Update the role
	pilot.Role = roles.VARole(newRole)
	if err := s.vaRoleRepo.Update(ctx, pilot); err != nil {
		return fmt.Errorf("failed to update pilot role: %w", err)
	}

	return nil
}

// UpdatePilotCallsign updates a pilot's callsign with validation
func (s *ManagementService) UpdatePilotCallsign(
	ctx context.Context,
	vaID string,
	pilotID string,
	newCallsign string,
	requestorRole roles.VARole,
) error {
	// Staff and admins can change callsigns
	if requestorRole != roles.RoleAirlineManager && requestorRole != roles.RoleAdmin {
		return fmt.Errorf("only staff and admins can change callsigns")
	}

	// Trim whitespace
	newCallsign = strings.TrimSpace(newCallsign)

	// Validate callsign format (1-5 digits only, matching Discord bot validation)
	callsignRegex := regexp.MustCompile(`^\d{1,5}$`)
	if newCallsign != "" && !callsignRegex.MatchString(newCallsign) {
		return fmt.Errorf("callsign must be 1-5 digits only")
	}

	// Get the pilot's current data
	pilot, err := s.vaRoleRepo.GetByID(ctx, pilotID)
	if err != nil {
		return fmt.Errorf("pilot not found: %w", err)
	}

	// Verify pilot belongs to this VA
	if pilot.VAID != vaID {
		return fmt.Errorf("pilot does not belong to this VA")
	}

	// Check uniqueness if callsign is not empty
	if newCallsign != "" {
		existing, err := s.vaRoleRepo.GetByCallsignAndVAID(ctx, newCallsign, vaID, pilotID)
		if err != nil {
			return fmt.Errorf("failed to check callsign uniqueness: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("callsign already in use")
		}
	}

	// Update the callsign
	pilot.Callsign = newCallsign
	if err := s.vaRoleRepo.Update(ctx, pilot); err != nil {
		return fmt.Errorf("failed to update pilot callsign: %w", err)
	}

	return nil
}

// RemovePilot deactivates a pilot (soft delete)
func (s *ManagementService) RemovePilot(
	ctx context.Context,
	vaID string,
	pilotID string,
	requestorRole roles.VARole,
) error {
	// Only admins can remove pilots
	if requestorRole != roles.RoleAdmin {
		return fmt.Errorf("only admins can remove pilots")
	}

	// Get the pilot to verify they belong to this VA
	pilot, err := s.vaRoleRepo.GetByID(ctx, pilotID)
	if err != nil {
		return fmt.Errorf("pilot not found: %w", err)
	}

	// Verify pilot belongs to this VA
	if pilot.VAID != vaID {
		return fmt.Errorf("pilot does not belong to this VA")
	}

	// Soft delete (set is_active to false)
	if err := s.vaRoleRepo.Delete(ctx, pilotID); err != nil {
		return fmt.Errorf("failed to remove pilot: %w", err)
	}

	return nil
}

// SearchPilots searches for pilots in a VA by IFC community ID
func (s *ManagementService) SearchPilots(
	ctx context.Context,
	vaID string,
	query string,
	limit int,
) ([]SearchResult, error) {
	if query == "" {
		return []SearchResult{}, nil
	}

	vaRoles, err := s.vaRoleRepo.GetAllByVAID(ctx, vaID)
	if err != nil {
		return nil, fmt.Errorf("failed to search pilots: %w", err)
	}

	var results []SearchResult
	queryLower := query
	for _, vaRole := range vaRoles {
		// Case-insensitive substring match on IFC Community ID
		if contains(vaRole.User.IFCommunityID, queryLower) {
			results = append(results, SearchResult{
				Username: vaRole.User.IFCommunityID,
				UserID:   vaRole.UserID,
				Callsign: vaRole.Callsign,
				Role:     string(vaRole.Role),
			})
			if len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// Helper function for case-insensitive substring search
func contains(s, substr string) bool {
	for i := 0; i < len(s); i++ {
		if i+len(substr) <= len(s) {
			match := true
			for j := 0; j < len(substr); j++ {
				if toLower(s[i+j]) != toLower(substr[j]) {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// Helper function to convert character to lowercase
func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
