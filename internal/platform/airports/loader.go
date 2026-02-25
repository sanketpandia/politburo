package airports

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"gorm.io/gorm"
)

// LoaderService handles loading airport data from JSON
type LoaderService struct {
	repo *Repository
}

// RawAirportData represents the structure of airport data from JSON
type RawAirportData struct {
	ICAO      string  `json:"icao"`
	IATA      string  `json:"iata"`
	Name      string  `json:"name"`
	City      string  `json:"city"`
	State     string  `json:"state"`
	Country   string  `json:"country"`
	Elevation int     `json:"elevation"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	TZ        string  `json:"tz"`
}

// NewLoaderService creates a new airport loader service
func NewLoaderService(db *gorm.DB) *LoaderService {
	return &LoaderService{
		repo: NewRepository(db),
	}
}

// LoadFromJSON loads airports from a JSON reader
// Expected format: object with airport data as values
// Example: {"KJFK": {"icao": "KJFK", "name": "John F. Kennedy...", ...}}
func (s *LoaderService) LoadFromJSON(ctx context.Context, reader io.Reader) (int, error) {
	// Parse the JSON data
	var rawData map[string]RawAirportData
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&rawData); err != nil {
		return 0, fmt.Errorf("failed to decode JSON: %w", err)
	}

	if len(rawData) == 0 {
		return 0, fmt.Errorf("no airport data found in JSON")
	}

	log.Printf("[AirportLoader] Loaded %d airports from JSON", len(rawData))

	// Convert to GORM models
	airports := make([]Airport, 0, len(rawData))
	for _, rawAirport := range rawData {
		// Build timezone string
		timezone := rawAirport.TZ
		if timezone == "" && rawAirport.State != "" {
			// Fallback to state if timezone not provided
			timezone = rawAirport.State
		}

		// Parse elevation
		var elevation sql.NullInt64
		if rawAirport.Elevation > 0 {
			elevation = sql.NullInt64{Int64: int64(rawAirport.Elevation), Valid: true}
		}

		airport := Airport{
			ICAO:      strings.ToUpper(strings.TrimSpace(rawAirport.ICAO)),
			IATA:      strings.ToUpper(strings.TrimSpace(rawAirport.IATA)),
			Name:      strings.TrimSpace(rawAirport.Name),
			City:      strings.TrimSpace(rawAirport.City),
			Country:   strings.TrimSpace(rawAirport.Country),
			Elevation: elevation,
			Latitude:  rawAirport.Lat,
			Longitude: rawAirport.Lon,
			Timezone:  timezone,
		}

		// Validate required fields
		if airport.ICAO == "" || airport.Name == "" {
			continue // Skip invalid records
		}

		airports = append(airports, airport)
	}

	if len(airports) == 0 {
		return 0, fmt.Errorf("no valid airports found after parsing")
	}

	log.Printf("[AirportLoader] Parsed %d valid airports", len(airports))

	// Delete existing airports before importing
	if err := s.repo.DeleteAll(ctx); err != nil {
		return 0, fmt.Errorf("failed to delete existing airports: %w", err)
	}

	log.Printf("[AirportLoader] Deleted existing airports")

	// Batch insert
	if err := s.repo.BatchInsert(ctx, airports); err != nil {
		return 0, fmt.Errorf("failed to insert airports: %w", err)
	}

	log.Printf("[AirportLoader] Successfully imported %d airports", len(airports))

	return len(airports), nil
}

// GetStats returns statistics about loaded airports
func (s *LoaderService) GetStats(ctx context.Context) (map[string]interface{}, error) {
	count, err := s.repo.Count(ctx)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_airports": count,
	}

	return stats, nil
}

// LoadAirportsFromEmbedded loads airports from the mwgg/Airports GitHub repository
// This is used for the initial setup via API endpoint
func (s *LoaderService) LoadAirportsFromEmbedded(ctx context.Context) (int, error) {
	log.Printf("[AirportLoader] Fetching airports from GitHub...")

	// Fetch from the mwgg/Airports repository
	url := "https://raw.githubusercontent.com/mwgg/Airports/refs/heads/master/airports.json"
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch airports from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to fetch airports: HTTP %d", resp.StatusCode)
	}

	return s.LoadFromJSON(ctx, resp.Body)
}
