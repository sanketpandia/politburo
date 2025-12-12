package responses

import "time"

// WorldTourResponse represents a world tour in API responses
type WorldTourResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      *string   `json:"description,omitempty"`
	DocumentationURL *string   `json:"documentation_url,omitempty"`
	Status           string    `json:"status"`
	FlightModeKey    string    `json:"flight_mode_key"`
	TotalLegs        int       `json:"total_legs"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// WorldTourWithLegsResponse represents a world tour with its legs
type WorldTourWithLegsResponse struct {
	WorldTourResponse
	Legs []WorldTourLegResponse `json:"legs"`
}

// WorldTourLegResponse represents a tour leg in API responses
type WorldTourLegResponse struct {
	ID            string    `json:"id"`
	LegNumber     int       `json:"leg_number"`
	Name          string    `json:"name"`
	RouteName     string    `json:"route_name"`
	RouteATID     *string   `json:"route_at_id,omitempty"`
	RouteResolved bool      `json:"route_resolved"`
	Description   *string   `json:"description,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// UserProgressResponse represents a user's progress in a world tour
type UserProgressResponse struct {
	UserID               string                `json:"user_id"`
	TourID               string                `json:"tour_id"`
	TourName             string                `json:"tour_name"`
	CompletedLegs        []CompletedLeg        `json:"completed_legs"`
	NextLeg              *WorldTourLegResponse `json:"next_leg,omitempty"`
	TotalLegs            int                   `json:"total_legs"`
	CompletedCount       int                   `json:"completed_count"`
	CompletionPercentage float32               `json:"completion_percentage"`
}

// CompletedLeg represents a completed tour leg
type CompletedLeg struct {
	LegNumber   int    `json:"leg_number"`
	RouteName   string `json:"route_name"`
	LegName     string `json:"leg_name"`
	CompletedAt string `json:"completed_at"`
}

// RouteValidationResponse represents the result of route validation
type RouteValidationResponse struct {
	IsWorldTourRoute bool                  `json:"is_world_tour_route"`
	TourID           *string               `json:"tour_id,omitempty"`
	Leg              *WorldTourLegResponse `json:"leg,omitempty"`
	IsNextLeg        bool                  `json:"is_next_leg"`
	FlightModeKey    *string               `json:"flight_mode_key,omitempty"`
	Message          string                `json:"message"`
}

// LeaderboardResponse represents the tour leaderboard
type LeaderboardResponse struct {
	TourName    string             `json:"tour_name"`
	TourID      string             `json:"tour_id"`
	TotalLegs   int                `json:"total_legs"`
	Leaderboard []LeaderboardEntry `json:"leaderboard"`
}

// LeaderboardEntry represents a user's position in the leaderboard
type LeaderboardEntry struct {
	UserID         string `json:"user_id"`
	Callsign       string `json:"callsign"`
	CompletedLegs  int    `json:"completed_legs"`
	LastCompletion string `json:"last_completion,omitempty"`
	IsFinished     bool   `json:"is_finished"`
	Ranking        int    `json:"ranking"`
}

// ActiveTourResponse represents the active tour for a VA
type ActiveTourResponse struct {
	HasActiveTour bool                       `json:"has_active_tour"`
	Tour          *WorldTourWithLegsResponse `json:"tour,omitempty"`
}
