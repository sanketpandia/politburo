package templates

import (
	"fmt"

	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/infra/session"
	platformui "infinite-experiment/politburo/internal/platform/ui"
)

// PrepareTemplateData creates a standardized template data map with common fields and role flags
// This eliminates code duplication across all page handlers
func PrepareTemplateData(sessionData *session.SessionData, pageTitle string) (map[string]interface{}, error) {
	if sessionData == nil {
		return nil, fmt.Errorf("session data cannot be nil")
	}

	// Get active VA
	activeVA := sessionData.GetActiveVA()
	if activeVA == nil {
		return nil, fmt.Errorf("no active VA found")
	}

	// Determine role flags
	isAdmin := activeVA.Role == "admin"
	isStaff := activeVA.Role == "staff" || activeVA.Role == "admin"

	// Get menu items based on role
	menuItems := platformui.GetMenuItems(activeVA)

	// Debug logging to track role parsing
	logging.Debug("Preparing template data", "va", activeVA.VAName, "role", activeVA.Role, "isAdmin", isAdmin, "isStaff", isStaff, "menuItems", len(menuItems))

	// Return standardized template data
	return map[string]interface{}{
		"ActiveVA":        activeVA,
		"VirtualAirlines": sessionData.VirtualAirlines,
		"Username":        sessionData.Username,
		"UserID":          sessionData.UserID,
		"ActiveVAID":      sessionData.ActiveVAID,
		"PageTitle":       pageTitle,
		"IsAdmin":         isAdmin,
		"IsStaff":         isStaff,
		"IsPilot":         activeVA.Role == "pilot",
		"MenuItems":       menuItems, // Add menu items to template data
	}, nil
}
