package aircraft

import (
	"context"
	"fmt"
	"log"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/models/dtos"

	"gorm.io/gorm"
)

// Service provides aircraft livery lookup with cache-first strategy
type Service struct {
	cache *cache.CacheService
	repo  *Repository
}

// NewService creates a new aircraft livery service
func NewService(cache *cache.CacheService, repo *Repository) *Service {
	return &Service{
		cache: cache,
		repo:  repo,
	}
}

// GetAircraftLivery fetches livery data (cache-first, then DB)
func (s *Service) GetAircraftLivery(ctx context.Context, liveryID string) *dtos.AircraftLivery {
	// Try cache first
	cacheKey := string(constants.CachePrefixLiveries) + liveryID
	if val, found := s.cache.Get(cacheKey); found {
		if livery, ok := val.(dtos.AircraftLivery); ok {
			return &livery
		}
	}

	// Cache miss - try database
	dbLivery, err := s.repo.GetByLiveryID(ctx, liveryID)
	if err != nil {
		// Log warning but return nil (as per user requirement)
		if err != gorm.ErrRecordNotFound {
			log.Printf("Warning: Failed to fetch livery %s from database: %v", liveryID, err)
		}
		return nil
	}

	// Convert GORM model to DTO
	dto := dtos.AircraftLivery{
		LiveryId:     dbLivery.LiveryID,
		AircraftID:   dbLivery.AircraftID,
		LiveryName:   dbLivery.LiveryName,
		AircraftName: dbLivery.AircraftName,
	}

	// Cache the result for 24 hours
	s.cache.Set(cacheKey, dto, 24*time.Hour)

	return &dto
}

// GetAircraftName returns just the aircraft name for a livery ID
func (s *Service) GetAircraftName(ctx context.Context, liveryID string) string {
	livery := s.GetAircraftLivery(ctx, liveryID)
	if livery == nil {
		return ""
	}
	return livery.AircraftName
}

// GetLiveryName returns just the livery name for a livery ID
func (s *Service) GetLiveryName(ctx context.Context, liveryID string) string {
	livery := s.GetAircraftLivery(ctx, liveryID)
	if livery == nil {
		return ""
	}
	return livery.LiveryName
}

// WarmCache loads all active liveries into cache
func (s *Service) WarmCache(ctx context.Context) error {
	liveries, err := s.repo.GetAllActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to load liveries for cache warming: %w", err)
	}

	warmedCount := 0
	for _, livery := range liveries {
		dto := dtos.AircraftLivery{
			LiveryId:     livery.LiveryID,
			AircraftID:   livery.AircraftID,
			LiveryName:   livery.LiveryName,
			AircraftName: livery.AircraftName,
		}

		cacheKey := string(constants.CachePrefixLiveries) + livery.LiveryID
		s.cache.Set(cacheKey, dto, 24*time.Hour)
		warmedCount++
	}

	log.Printf("Cache warmed with %d active liveries", warmedCount)
	return nil
}

// ConvertAPILiveryToGORM converts IF API livery DTO to GORM entity for persistence
func ConvertAPILiveryToGORM(apiLivery dtos.AircraftLivery) AircraftLivery {
	return AircraftLivery{
		LiveryID:     apiLivery.LiveryId,
		AircraftID:   apiLivery.AircraftID,
		LiveryName:   apiLivery.LiveryName,
		AircraftName: apiLivery.AircraftName,
		IsActive:     true,
		LastSyncedAt: time.Now(),
	}
}
