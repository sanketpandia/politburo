package services

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/db/repositories"
	"infinite-experiment/politburo/internal/models/dtos/requests"
	"infinite-experiment/politburo/internal/models/dtos/responses"
	"infinite-experiment/politburo/internal/models/gorm"
)

// WorldTourService handles business logic for World Tours
type WorldTourService struct {
	worldTourRepo *repositories.WorldTourRepository
}

// NewWorldTourService creates a new World Tour service
func NewWorldTourService(worldTourRepo *repositories.WorldTourRepository) *WorldTourService {
	return &WorldTourService{
		worldTourRepo: worldTourRepo,
	}
}

// TOUR OPERATIONS

// CreateTour creates a new world tour
func (s *WorldTourService) CreateTour(ctx context.Context, vaID, createdByID string, req *requests.CreateWorldTourRequest) (*responses.WorldTourResponse, error) {
	// Create GORM model from request
	tour := &gorm.WorldTour{
		VAID:             vaID,
		Name:             req.Name,
		Description:      req.Description,
		DocumentationURL: req.DocumentationURL,
		FlightModeKey:    req.FlightModeKey,
		Status:           "draft", // Always start as draft
		CreatedByID:      &createdByID,
	}

	// Create tour in database
	err := s.worldTourRepo.Create(ctx, tour)
	if err != nil {
		return nil, fmt.Errorf("failed to create world tour: %w", err)
	}

	// Convert to response
	return s.convertTourToResponse(tour), nil
}

// UpdateTour updates an existing world tour
func (s *WorldTourService) UpdateTour(ctx context.Context, tourID string, req *requests.UpdateWorldTourRequest) (*responses.WorldTourResponse, error) {
	// Get existing tour
	tour, err := s.worldTourRepo.GetByID(ctx, tourID)
	if err != nil {
		return nil, fmt.Errorf("failed to get world tour: %w", err)
	}

	// Update fields
	tour.Name = req.Name
	tour.Description = req.Description
	tour.DocumentationURL = req.DocumentationURL
	if req.Status != "" {
		tour.Status = req.Status
	}

	// Save updates
	err = s.worldTourRepo.Update(ctx, tour)
	if err != nil {
		return nil, fmt.Errorf("failed to update world tour: %w", err)
	}

	return s.convertTourToResponse(tour), nil
}

// DeleteTour deletes a world tour
func (s *WorldTourService) DeleteTour(ctx context.Context, tourID string) error {
	return s.worldTourRepo.Delete(ctx, tourID)
}

// GetTourByID retrieves a tour by ID with its legs
func (s *WorldTourService) GetTourByID(ctx context.Context, tourID string) (*responses.WorldTourWithLegsResponse, error) {
	tour, err := s.worldTourRepo.GetByID(ctx, tourID)
	if err != nil {
		return nil, fmt.Errorf("failed to get world tour: %w", err)
	}

	return s.convertTourWithLegsToResponse(tour), nil
}

// GetToursByVA retrieves all tours for a virtual airline
func (s *WorldTourService) GetToursByVA(ctx context.Context, vaID string) ([]responses.WorldTourResponse, error) {
	tours, err := s.worldTourRepo.GetByVA(ctx, vaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tours for VA: %w", err)
	}

	responses := make([]responses.WorldTourResponse, len(tours))
	for i, tour := range tours {
		responses[i] = *s.convertTourToResponse(&tour)
	}

	return responses, nil
}

// GetActiveTour retrieves the active tour for a VA
func (s *WorldTourService) GetActiveTour(ctx context.Context, vaID string) (*responses.ActiveTourResponse, error) {
	tour, err := s.worldTourRepo.GetActiveTour(ctx, vaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active tour: %w", err)
	}

	response := &responses.ActiveTourResponse{
		HasActiveTour: tour != nil,
	}

	if tour != nil {
		response.Tour = s.convertTourWithLegsToResponse(tour)
	}

	return response, nil
}

// LEG OPERATIONS

// AddLeg adds a new leg to a world tour
func (s *WorldTourService) AddLeg(ctx context.Context, tourID, vaID string, req *requests.AddTourLegRequest) (*responses.WorldTourLegResponse, error) {
	// Create GORM model from request
	leg := &gorm.WorldTourLeg{
		LegNumber:   req.LegNumber,
		Name:        req.Name,
		RouteName:   req.RouteName,
		Description: req.Description,
	}

	// Add leg (includes route validation and resolution)
	err := s.worldTourRepo.AddLeg(ctx, tourID, vaID, leg)
	if err != nil {
		return nil, fmt.Errorf("failed to add tour leg: %w", err)
	}

	return s.convertLegToResponse(leg), nil
}

// UpdateLeg updates an existing tour leg
func (s *WorldTourService) UpdateLeg(ctx context.Context, legID, vaID string, req *requests.UpdateTourLegRequest) (*responses.WorldTourLegResponse, error) {
	// Get existing leg
	leg, err := s.worldTourRepo.GetLegByID(ctx, legID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tour leg: %w", err)
	}

	// Update fields
	leg.LegNumber = req.LegNumber
	leg.Name = req.Name
	leg.RouteName = req.RouteName
	leg.Description = req.Description

	// Update leg (includes route re-validation)
	err = s.worldTourRepo.UpdateLeg(ctx, vaID, leg)
	if err != nil {
		return nil, fmt.Errorf("failed to update tour leg: %w", err)
	}

	return s.convertLegToResponse(leg), nil
}

// DeleteLeg deletes a tour leg
func (s *WorldTourService) DeleteLeg(ctx context.Context, legID string) error {
	return s.worldTourRepo.DeleteLeg(ctx, legID)
}

// GetLegByNumber retrieves a specific leg by tour ID and leg number
func (s *WorldTourService) GetLegByNumber(ctx context.Context, tourID string, legNumber int) (*responses.WorldTourLegResponse, error) {
	leg, err := s.worldTourRepo.GetLegByNumber(ctx, tourID, legNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get tour leg: %w", err)
	}

	if leg == nil {
		return nil, fmt.Errorf("leg %d not found for tour %s", legNumber, tourID)
	}

	return s.convertLegToResponse(leg), nil
}

// PROGRESS TRACKING OPERATIONS

// GetUserProgress calculates a user's progress in a world tour
func (s *WorldTourService) GetUserProgress(ctx context.Context, tourID, userID string) (*responses.UserProgressResponse, error) {
	// Get tour details
	tour, err := s.worldTourRepo.GetByID(ctx, tourID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tour: %w", err)
	}

	// Get user's completed routes for this tour's flight mode
	completedLegs, err := s.worldTourRepo.GetUserCompletedRoutes(ctx, userID, tour.FlightModeKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get user completed routes: %w", err)
	}

	// Convert to response format
	completedLegResponses := make([]responses.CompletedLeg, len(completedLegs))
	for i, leg := range completedLegs {
		completedLegResponses[i] = responses.CompletedLeg{
			LegNumber:   leg.LegNumber,
			RouteName:   leg.RouteName,
			LegName:     leg.LegName,
			CompletedAt: leg.CompletedAt,
		}
	}

	// Determine next leg (sequential progression)
	nextLegNumber := len(completedLegs) + 1
	var nextLeg *responses.WorldTourLegResponse
	if nextLegNumber <= tour.TotalLegs {
		nextLegGorm, err := s.worldTourRepo.GetLegByNumber(ctx, tourID, nextLegNumber)
		if err == nil && nextLegGorm != nil {
			nextLeg = s.convertLegToResponse(nextLegGorm)
		}
	}

	// Calculate completion percentage
	completionPercentage := float32(0)
	if tour.TotalLegs > 0 {
		completionPercentage = float32(len(completedLegs)) / float32(tour.TotalLegs) * 100
	}

	return &responses.UserProgressResponse{
		UserID:               userID,
		TourID:               tourID,
		TourName:             tour.Name,
		CompletedLegs:        completedLegResponses,
		NextLeg:              nextLeg,
		TotalLegs:            tour.TotalLegs,
		CompletedCount:       len(completedLegs),
		CompletionPercentage: completionPercentage,
	}, nil
}

// GetTourLeaderboard generates leaderboard for a tour
func (s *WorldTourService) GetTourLeaderboard(ctx context.Context, tourID string) (*responses.LeaderboardResponse, error) {
	// Get tour details
	tour, err := s.worldTourRepo.GetByID(ctx, tourID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tour: %w", err)
	}

	// Get leaderboard data
	leaderboardData, err := s.worldTourRepo.GetTourLeaderboard(ctx, tourID, tour.FlightModeKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get tour leaderboard: %w", err)
	}

	// Convert to response format
	leaderboard := make([]responses.LeaderboardEntry, len(leaderboardData))
	for i, entry := range leaderboardData {
		leaderboard[i] = responses.LeaderboardEntry{
			UserID:         entry.UserID,
			Callsign:       entry.Callsign,
			CompletedLegs:  entry.CompletedLegs,
			LastCompletion: entry.LastCompletion,
			IsFinished:     entry.IsFinished,
			Ranking:        entry.Ranking,
		}
	}

	return &responses.LeaderboardResponse{
		TourName:    tour.Name,
		TourID:      tourID,
		TotalLegs:   tour.TotalLegs,
		Leaderboard: leaderboard,
	}, nil
}

// ROUTE VALIDATION

// ValidateRoute validates if a route matches the user's next expected leg
func (s *WorldTourService) ValidateRoute(ctx context.Context, vaID, route, userID string) (*responses.RouteValidationResponse, error) {
	// Get active tour for VA
	activeTour, err := s.worldTourRepo.GetActiveTour(ctx, vaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active tour: %w", err)
	}

	// If no active tour, route is not a world tour route
	if activeTour == nil {
		return &responses.RouteValidationResponse{
			IsWorldTourRoute: false,
			Message:          "No active world tour for this VA",
		}, nil
	}

	// Get user's progress to determine next leg
	progress, err := s.GetUserProgress(ctx, activeTour.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user progress: %w", err)
	}

	// Check if route matches next leg
	if progress.NextLeg != nil && progress.NextLeg.RouteName == route {
		return &responses.RouteValidationResponse{
			IsWorldTourRoute: true,
			TourID:           &activeTour.ID,
			Leg:              progress.NextLeg,
			IsNextLeg:        true,
			FlightModeKey:    &activeTour.FlightModeKey,
			Message:          fmt.Sprintf("This is your next World Tour leg: %s", progress.NextLeg.Name),
		}, nil
	}

	// Check if route matches any other leg in the tour (but not next)
	for _, leg := range activeTour.Legs {
		if leg.RouteName == route {
			legResponse := s.convertLegToResponse(&leg)
			return &responses.RouteValidationResponse{
				IsWorldTourRoute: true,
				TourID:           &activeTour.ID,
				Leg:              legResponse,
				IsNextLeg:        false,
				FlightModeKey:    &activeTour.FlightModeKey,
				Message:          fmt.Sprintf("This is World Tour leg %d, but you need to complete legs sequentially", leg.LegNumber),
			}, nil
		}
	}

	// Route not found in world tour
	return &responses.RouteValidationResponse{
		IsWorldTourRoute: false,
		Message:          "Route is not part of the active World Tour",
	}, nil
}

// HELPER METHODS

// convertTourToResponse converts a GORM WorldTour to response format
func (s *WorldTourService) convertTourToResponse(tour *gorm.WorldTour) *responses.WorldTourResponse {
	return &responses.WorldTourResponse{
		ID:               tour.ID,
		Name:             tour.Name,
		Description:      tour.Description,
		DocumentationURL: tour.DocumentationURL,
		Status:           tour.Status,
		FlightModeKey:    tour.FlightModeKey,
		TotalLegs:        tour.TotalLegs,
		CreatedAt:        tour.CreatedAt,
		UpdatedAt:        tour.UpdatedAt,
	}
}

// convertTourWithLegsToResponse converts a GORM WorldTour with legs to response format
func (s *WorldTourService) convertTourWithLegsToResponse(tour *gorm.WorldTour) *responses.WorldTourWithLegsResponse {
	legs := make([]responses.WorldTourLegResponse, len(tour.Legs))
	for i, leg := range tour.Legs {
		legs[i] = *s.convertLegToResponse(&leg)
	}

	return &responses.WorldTourWithLegsResponse{
		WorldTourResponse: *s.convertTourToResponse(tour),
		Legs:              legs,
	}
}

// convertLegToResponse converts a GORM WorldTourLeg to response format
func (s *WorldTourService) convertLegToResponse(leg *gorm.WorldTourLeg) *responses.WorldTourLegResponse {
	return &responses.WorldTourLegResponse{
		ID:            leg.ID,
		LegNumber:     leg.LegNumber,
		Name:          leg.Name,
		RouteName:     leg.RouteName,
		RouteATID:     leg.RouteATID,
		RouteResolved: leg.RouteATID != nil,
		Description:   leg.Description,
		CreatedAt:     leg.CreatedAt,
	}
}
