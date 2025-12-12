package ui

import (
	"fmt"
	"infinite-experiment/politburo/internal/common"
	"log"
)

// PrepareTemplateData creates a standardized template data map with common fields and role flags
// This eliminates code duplication across all page handlers
func PrepareTemplateData(sessionData *common.SessionData, pageTitle string) (map[string]interface{}, error) {
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

	// Debug logging to track role parsing
	log.Printf("[PrepareTemplateData] VA=%s, Role='%s', IsAdmin=%v, IsStaff=%v",
		activeVA.VAName, activeVA.Role, isAdmin, isStaff)

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
	}, nil
}
