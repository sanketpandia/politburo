package repositories

import (
	"context"
	"fmt"

	"infinite-experiment/politburo/internal/models/gorm"

	gormdb "gorm.io/gorm"
)

// WorldTourRepository handles database operations for World Tours
type WorldTourRepository struct {
	db        *gormdb.DB
	routeRepo *RouteATSyncedRepo
}

// NewWorldTourRepository creates a new World Tour repository
func NewWorldTourRepository(db *gormdb.DB, routeRepo *RouteATSyncedRepo) *WorldTourRepository {
	return &WorldTourRepository{
		db:        db,
		routeRepo: routeRepo,
	}
}

// TOUR OPERATIONS

// Create creates a new world tour
func (r *WorldTourRepository) Create(ctx context.Context, tour *gorm.WorldTour) error {
	return r.db.WithContext(ctx).Create(tour).Error
}

// Update updates an existing world tour
func (r *WorldTourRepository) Update(ctx context.Context, tour *gorm.WorldTour) error {
	return r.db.WithContext(ctx).Save(tour).Error
}

// Delete deletes a world tour by ID (cascades to legs)
func (r *WorldTourRepository) Delete(ctx context.Context, tourID string) error {
	return r.db.WithContext(ctx).Delete(&gorm.WorldTour{}, "id = ?", tourID).Error
}

// GetByID retrieves a tour by ID with its legs
func (r *WorldTourRepository) GetByID(ctx context.Context, tourID string) (*gorm.WorldTour, error) {
	var tour gorm.WorldTour
	err := r.db.WithContext(ctx).
		Preload("Legs", func(db *gormdb.DB) *gormdb.DB {
			return db.Order("leg_number ASC")
		}).
		Where("id = ?", tourID).
		First(&tour).Error
	if err != nil {
		return nil, err
	}

	// Set computed fields
	tour.TotalLegs = len(tour.Legs)
	return &tour, nil
}

// GetByVA retrieves all tours for a virtual airline
func (r *WorldTourRepository) GetByVA(ctx context.Context, vaID string) ([]gorm.WorldTour, error) {
	var tours []gorm.WorldTour
	err := r.db.WithContext(ctx).
		Preload("Legs", func(db *gormdb.DB) *gormdb.DB {
			return db.Order("leg_number ASC")
		}).
		Where("va_id = ?", vaID).
		Order("created_at DESC").
		Find(&tours).Error
	if err != nil {
		return tours, err
	}

	// Set computed fields for all tours
	for i := range tours {
		tours[i].TotalLegs = len(tours[i].Legs)
	}

	return tours, nil
}

// GetActiveTour retrieves the active tour for a VA (status = 'active')
func (r *WorldTourRepository) GetActiveTour(ctx context.Context, vaID string) (*gorm.WorldTour, error) {
	var tour gorm.WorldTour
	err := r.db.WithContext(ctx).
		Preload("Legs", func(db *gormdb.DB) *gormdb.DB {
			return db.Order("leg_number ASC")
		}).
		Where("va_id = ? AND status = 'active'", vaID).
		First(&tour).Error
	if err != nil {
		if err == gormdb.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	tour.TotalLegs = len(tour.Legs)
	return &tour, nil
}

// LEG OPERATIONS

// AddLeg adds a new leg to a world tour with route validation
func (r *WorldTourRepository) AddLeg(ctx context.Context, tourID, vaID string, leg *gorm.WorldTourLeg) error {
	// First, try to resolve the route name to an Airtable ID
	route, err := r.routeRepo.FindByName(ctx, vaID, leg.RouteName)
	if err != nil {
		return fmt.Errorf("error checking route: %w", err)
	}

	// If route found, set the resolved Airtable ID
	if route != nil {
		leg.RouteATID = &route.ATID
	}
	// If route not found, RouteATID remains nil (which is allowed)

	leg.WorldTourID = tourID
	return r.db.WithContext(ctx).Create(leg).Error
}

// UpdateLeg updates an existing leg with route re-validation
func (r *WorldTourRepository) UpdateLeg(ctx context.Context, vaID string, leg *gorm.WorldTourLeg) error {
	// Re-validate route if route name changed
	route, err := r.routeRepo.FindByName(ctx, vaID, leg.RouteName)
	if err != nil {
		return fmt.Errorf("error checking route: %w", err)
	}

	// Update resolved Airtable ID
	if route != nil {
		leg.RouteATID = &route.ATID
	} else {
		leg.RouteATID = nil
	}

	return r.db.WithContext(ctx).Save(leg).Error
}

// DeleteLeg deletes a leg by ID
func (r *WorldTourRepository) DeleteLeg(ctx context.Context, legID string) error {
	return r.db.WithContext(ctx).Delete(&gorm.WorldTourLeg{}, "id = ?", legID).Error
}

// GetLegByID retrieves a leg by ID
func (r *WorldTourRepository) GetLegByID(ctx context.Context, legID string) (*gorm.WorldTourLeg, error) {
	var leg gorm.WorldTourLeg
	err := r.db.WithContext(ctx).Where("id = ?", legID).First(&leg).Error
	if err != nil {
		return nil, err
	}
	return &leg, nil
}

// GetLegByNumber retrieves a specific leg by tour ID and leg number
func (r *WorldTourRepository) GetLegByNumber(ctx context.Context, tourID string, legNumber int) (*gorm.WorldTourLeg, error) {
	var leg gorm.WorldTourLeg
	err := r.db.WithContext(ctx).
		Where("world_tour_id = ? AND leg_number = ?", tourID, legNumber).
		First(&leg).Error
	if err != nil {
		if err == gormdb.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &leg, nil
}

// GetAllLegs retrieves all legs for a tour ordered by leg number
func (r *WorldTourRepository) GetAllLegs(ctx context.Context, tourID string) ([]gorm.WorldTourLeg, error) {
	var legs []gorm.WorldTourLeg
	err := r.db.WithContext(ctx).
		Where("world_tour_id = ?", tourID).
		Order("leg_number ASC").
		Find(&legs).Error
	return legs, err
}

// PROGRESS TRACKING OPERATIONS

// CompletedLeg represents a completed tour leg
type CompletedLeg struct {
	LegNumber   int    `json:"leg_number"`
	RouteName   string `json:"route_name"`
	LegName     string `json:"leg_name"`
	CompletedAt string `json:"completed_at"`
}

// GetUserCompletedRoutes retrieves completed routes for a user in a specific flight mode
func (r *WorldTourRepository) GetUserCompletedRoutes(ctx context.Context, userID, flightModeKey string) ([]CompletedLeg, error) {
	var completed []CompletedLeg

	// Query PIREPs joined with tour legs to get completion data
	err := r.db.WithContext(ctx).Raw(`
		SELECT 
			wtl.leg_number,
			wtl.route_name,
			wtl.name as leg_name,
			p.created_at as completed_at
		FROM pirep_at_synced p
		JOIN world_tour_legs wtl ON wtl.route_name = p.route
		JOIN world_tours wt ON wt.id = wtl.world_tour_id
		WHERE p.user_id = ? AND wt.flight_mode_key = ?
		ORDER BY wtl.leg_number ASC
	`, userID, flightModeKey).Scan(&completed).Error

	return completed, err
}

// LEADERBOARD OPERATIONS

// LeaderboardEntry represents a user's position in the tour leaderboard
type LeaderboardEntry struct {
	UserID         string `json:"user_id"`
	Callsign       string `json:"callsign"`
	CompletedLegs  int    `json:"completed_legs"`
	LastCompletion string `json:"last_completion,omitempty"`
	IsFinished     bool   `json:"is_finished"`
	Ranking        int    `json:"ranking"`
}

// GetTourLeaderboard generates leaderboard for a tour
func (r *WorldTourRepository) GetTourLeaderboard(ctx context.Context, tourID, flightModeKey string) ([]LeaderboardEntry, error) {
	var leaderboard []LeaderboardEntry

	// Get total leg count for the tour
	var totalLegs int64
	err := r.db.WithContext(ctx).
		Model(&gorm.WorldTourLeg{}).
		Where("world_tour_id = ?", tourID).
		Count(&totalLegs).Error
	if err != nil {
		return nil, err
	}

	// Query with leaderboard ranking logic
	err = r.db.WithContext(ctx).Raw(`
		WITH user_progress AS (
			SELECT 
				p.user_id,
				u.callsign,
				COUNT(*) as completed_legs,
				MAX(p.created_at) as last_completion
			FROM pirep_at_synced p
			JOIN users u ON u.id = p.user_id
			JOIN world_tour_legs wtl ON wtl.route_name = p.route
			JOIN world_tours wt ON wt.id = wtl.world_tour_id
			WHERE wt.flight_mode_key = ? AND wt.id = ?
			GROUP BY p.user_id, u.callsign
		)
		SELECT 
			user_id,
			callsign,
			completed_legs,
			last_completion,
			(completed_legs = ?) as is_finished,
			ROW_NUMBER() OVER (ORDER BY completed_legs DESC, last_completion ASC) as ranking
		FROM user_progress
		ORDER BY ranking
		LIMIT 50
	`, flightModeKey, tourID, totalLegs).Scan(&leaderboard).Error

	return leaderboard, err
}

// ROUTE VALIDATION

// ValidateRoute checks if a route exists in the VA's route database
func (r *WorldTourRepository) ValidateRoute(ctx context.Context, vaID, routeName string) (*gorm.RouteATSynced, error) {
	return r.routeRepo.FindByName(ctx, vaID, routeName)
}

// UTILITY OPERATIONS

// Exists checks if a tour exists
func (r *WorldTourRepository) Exists(ctx context.Context, tourID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&gorm.WorldTour{}).
		Where("id = ?", tourID).
		Count(&count).Error
	return count > 0, err
}

// GetCountByVA gets the total count of tours for a VA
func (r *WorldTourRepository) GetCountByVA(ctx context.Context, vaID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&gorm.WorldTour{}).
		Where("va_id = ?", vaID).
		Count(&count).Error
	return count, err
}
