package dtos

// FormField represents a form field in a flight mode
type FormField struct {
	Name           string `json:"name"`
	Type           string `json:"type"` // text, textarea, number, date
	Label          string `json:"label"`
	Required       bool   `json:"required"`
	Value          string `json:"value,omitempty"` // Optional default/prepopulated value
	ShowInDiscord  bool   `json:"show_in_discord"` // Controls visibility in Discord modal form
}

// AutoRouteConfig represents an auto-mapped route for a flight mode
type AutoRouteConfig struct {
	RouteName  string  `json:"route_name"`
	Multiplier float64 `json:"multiplier"`
}

// ValidationConfig represents validation rules for a flight mode
type ValidationConfig struct {
	AllowAnyCurrentRoute bool     `json:"allow_any_current_route"`
	AllowedRoutes        []string `json:"allowed_routes"`
	ValidationMode       string   `json:"validation_mode"` // exact_match, any
}

// FlightModeConfig represents the configuration for a single flight mode
type FlightModeConfig struct {
	Enabled                bool                   `json:"enabled"`
	DisplayName            string                 `json:"display_name"`
	Description            string                 `json:"description,omitempty"`
	RequiresRouteSelection bool                   `json:"requires_route_selection"`
	AutofillRoute          string                 `json:"autofill_route,omitempty"` // Optional route to prepopulate in modal
	Fields                 []FormField            `json:"fields"`
	AutoRoute              *AutoRouteConfig       `json:"auto_route,omitempty"`
	Validations            ValidationConfig       `json:"validations"`
	Metadata               map[string]interface{} `json:"metadata,omitempty"`
}

// UserInfo represents the current user's flight information
type UserInfo struct {
	Callsign             string `json:"callsign"`
	IFCUsername          string `json:"ifc_username"`
	CurrentAircraft      string `json:"current_aircraft"`
	CurrentLivery        string `json:"current_livery"`
	CurrentRoute         string `json:"current_route"`
	CurrentFlightStatus  string `json:"current_flight_status"`
	CurrentAltitude      int    `json:"current_altitude,omitempty"` // Altitude in feet at time of request
	CurrentSpeed         int    `json:"current_speed,omitempty"`    // Speed in knots at time of request
	Multiplier           float64 `json:"multiplier,omitempty"`       // Mode multiplier for reference
}

// RouteOption represents a selectable route option
type RouteOption struct {
	RouteID    string  `json:"route_id"`
	Name       string  `json:"name"`
	Multiplier float64 `json:"multiplier"`
}

// ModeResponse represents a single flight mode in the config response
type ModeResponse struct {
	ModeID                 string         `json:"mode_id"`
	DisplayName            string         `json:"display_name"`
	Status                 string         `json:"status"` // valid, invalid
	RequiresRouteSelection bool           `json:"requires_route_selection"`
	Fields                 []FormField    `json:"fields"`
	AvailableRoutes        []RouteOption  `json:"available_routes,omitempty"`
	AutoRoute              *RouteOption   `json:"auto_route,omitempty"`
	ErrorReason            string         `json:"error_reason,omitempty"`
}

// SimpleModeResponse represents a minimal flight mode response for GET /api/v1/pireps/config
type SimpleModeResponse struct {
	ModeID                 string      `json:"mode_id"`
	DisplayName            string      `json:"display_name"`
	Status                 string      `json:"status"` // valid, invalid
	RequiresRouteSelection bool        `json:"requires_route_selection"`
	AutofillRoute          string      `json:"autofill_route,omitempty"` // Optional route to prepopulate in modal
	Fields                 []FormField `json:"fields"`
	ErrorReason            string      `json:"error_reason,omitempty"`
}

// ConfigResponse represents the response from GET /api/v1/pireps/config
type ConfigResponse struct {
	UserInfo       UserInfo       `json:"user_info"`
	AvailableModes []ModeResponse `json:"available_modes"`
}

// SimpleConfigResponse represents a minimal response from GET /api/v1/pireps/config
type SimpleConfigResponse struct {
	UserInfo       UserInfo             `json:"user_info"`
	AvailableModes []SimpleModeResponse `json:"available_modes"`
}

// PirepSubmitRequest represents the request body for POST /api/v1/pireps/submit
type PirepSubmitRequest struct {
	Mode         string `json:"mode"`
	RouteID      string `json:"route_id,omitempty"`
	FlightTime   string `json:"flight_time"`
	PilotRemarks string `json:"pilot_remarks,omitempty"`
	FuelKg       *int   `json:"fuel_kg,omitempty"`
	CargoKg      *int   `json:"cargo_kg,omitempty"`
	Passengers   *int   `json:"passengers,omitempty"`
	Inputs       map[string]string `json:"inputs,omitempty"`
}

// PirepSubmitResponse represents the response from POST /api/v1/pireps/submit
type PirepSubmitResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	PirepID      string `json:"pirep_id,omitempty"`
	ErrorType    string `json:"error_type,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}
