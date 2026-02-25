package ui

import (
	"infinite-experiment/politburo/infra/session"
	"infinite-experiment/politburo/internal/platform/roles"
)

// MenuItem represents a single menu item
type MenuItem struct {
	Label        string
	Path         string
	PageID       string
	RequiredRole roles.VARole
	Icon         string // Optional icon identifier
}

// GetMenuItems returns menu items based on user role
// This is a pure function that takes session data and returns menu configuration
func GetMenuItems(activeVA *session.VAMembership) []MenuItem {
	baseItems := []MenuItem{
		{
			Label:        "Dashboard",
			Path:         "/dashboard",
			PageID:       "dashboard",
			RequiredRole: roles.RolePilot,
			Icon:         "dashboard",
		},
	}

	// Staff and admin get additional items
	if activeVA.Role == string(roles.RoleAirlineManager) || activeVA.Role == string(roles.RoleAdmin) {
		baseItems = append(baseItems, []MenuItem{
			{
				Label:        "Live Flights",
				Path:         "/dashboard/live",
				PageID:       "live",
				RequiredRole: roles.RoleAirlineManager,
				Icon:         "live",
			},
			{
				Label:        "Logbook",
				Path:         "/dashboard/logbook",
				PageID:       "logbook",
				RequiredRole: roles.RoleAirlineManager,
				Icon:         "logbook",
			},
			{
				Label:        "Pilots",
				Path:         "/dashboard/pilots",
				PageID:       "pilots",
				RequiredRole: roles.RoleAirlineManager,
				Icon:         "pilots",
			},
		}...)
	}

	// Admin-only items
	if activeVA.Role == string(roles.RoleAdmin) {
		baseItems = append(baseItems, []MenuItem{
			{
				Label:        "VA Admin",
				Path:         "/dashboard/vaadmin/pilots",
				PageID:       "vaadmin-pilots",
				RequiredRole: roles.RoleAdmin,
				Icon:         "vaadmin",
			},
			{
				Label:        "Datasource",
				Path:         "/dashboard/settings/datasource",
				PageID:       "datasource",
				RequiredRole: roles.RoleAdmin,
				Icon:         "datasource",
			},
			{
				Label:        "PIREP",
				Path:         "/dashboard/settings/pirep",
				PageID:       "pirep",
				RequiredRole: roles.RoleAdmin,
				Icon:         "pirep",
			},
			{
				Label:        "Events",
				Path:         "/dashboard/events",
				PageID:       "events",
				RequiredRole: roles.RoleAdmin,
				Icon:         "events",
			},
		}...)
	}

	// Filter items based on user's role
	var filteredItems []MenuItem
	userRole := roles.VARole(activeVA.Role)

	for _, item := range baseItems {
		if hasAccess(userRole, item.RequiredRole) {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems
}

// hasAccess checks if user role has access to required role
func hasAccess(userRole, requiredRole roles.VARole) bool {
	roleHierarchy := map[roles.VARole]int{
		roles.RolePilot:          1,
		roles.RoleAirlineManager: 2,
		roles.RoleAdmin:          3,
	}
	userLevel := roleHierarchy[userRole]
	requiredLevel := roleHierarchy[requiredRole]
	return userLevel >= requiredLevel
}
