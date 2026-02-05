package aircraft

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/models/dtos"

	"gorm.io/gorm"
)

// Service provides aircraft livery lookup with cache-first strategy
type Service struct {
	cache cache.CacheInterface // Redis cache interface
	repo  *Repository
}

// NewService creates a new aircraft livery service
func NewService(cache cache.CacheInterface, repo *Repository) *Service {
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

// GetAircraftNameByID retrieves aircraft name directly from cache by aircraft ID
// This uses the cache populated by the aircraft cache_job
func (s *Service) GetAircraftNameByID(aircraftID string) string {
	if aircraftID == "" || s.cache == nil {
		return ""
	}
	aircraftKey := cache.AircraftKey(aircraftID)
	val, found := s.cache.Get(aircraftKey)
	if !found {
		return ""
	}
	// Cache stores aircraft name as a string
	if name, ok := val.(string); ok {
		return name
	}
	return ""
}

// GetLiveryNameByID retrieves livery name directly from cache by livery ID
// This uses the cache populated by the aircraft cache_job
func (s *Service) GetLiveryNameByID(liveryID string) string {
	if liveryID == "" || s.cache == nil {
		return ""
	}
	liveryKey := cache.LiveryKey(liveryID)
	val, found := s.cache.Get(liveryKey)
	if !found {
		return ""
	}
	// Cache stores livery name as a string
	if name, ok := val.(string); ok {
		return name
	}
	return ""
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

// GetShortAircraftName returns a short code for an aircraft name
func GetShortAircraftName(fullName string) string {
	if short, ok := constants.AircraftShortNames[fullName]; ok {
		return short
	}
	// fallback to first 4 uppercase characters
	runes := []rune(fullName)
	if len(runes) > 4 {
		return strings.ToUpper(string(runes[:4]))
	}
	return strings.ToUpper(fullName)
}

// GetShortLiveryName returns a short code for a livery name
func GetShortLiveryName(name string) string {
	if code, ok := constants.LiveryShortNames[name]; ok {
		return code
	}
	// fallback to first 4 uppercase characters
	runes := []rune(name)
	if len(runes) > 4 {
		return strings.ToUpper(string(runes[:4]))
	}
	return strings.ToUpper(name)
}
