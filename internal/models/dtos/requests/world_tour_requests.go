package requests

// CreateWorldTourRequest represents the request to create a new world tour
type CreateWorldTourRequest struct {
	Name             string  `json:"name" validate:"required,min=1,max=255"`
	Description      *string `json:"description,omitempty"`
	DocumentationURL *string `json:"documentation_url,omitempty"`
	FlightModeKey    string  `json:"flight_mode_key" validate:"required,min=1,max=100"`
}

// UpdateWorldTourRequest represents the request to update an existing world tour
type UpdateWorldTourRequest struct {
	Name             string  `json:"name" validate:"required,min=1,max=255"`
	Description      *string `json:"description,omitempty"`
	DocumentationURL *string `json:"documentation_url,omitempty"`
	Status           string  `json:"status,omitempty" validate:"omitempty,oneof=draft active completed cancelled"`
}

// AddTourLegRequest represents the request to add a leg to a world tour
type AddTourLegRequest struct {
	LegNumber   int     `json:"leg_number" validate:"required,min=1"`
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	RouteName   string  `json:"route_name" validate:"required,min=1,max=255"`
	Description *string `json:"description,omitempty"`
}

// UpdateTourLegRequest represents the request to update an existing tour leg
type UpdateTourLegRequest struct {
	LegNumber   int     `json:"leg_number" validate:"required,min=1"`
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	RouteName   string  `json:"route_name" validate:"required,min=1,max=255"`
	Description *string `json:"description,omitempty"`
}

// ValidateRouteRequest represents the request to validate a route for world tour
type ValidateRouteRequest struct {
	Route  string `json:"route" validate:"required"`
	UserID string `json:"user_id" validate:"required"`
}
