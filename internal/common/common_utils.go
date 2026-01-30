package common

import (
	"infinite-experiment/politburo/infra/cache"
	"fmt"
	"infinite-experiment/politburo/internal/constants"
	"infinite-experiment/politburo/internal/models/dtos"
	"log"
	"strings"
	"time"
)

func GetResponseTime(init time.Time) string {
	timeDiff := time.Since(init).Milliseconds()
	return fmt.Sprintf("%dms", timeDiff)
}

func GetKeysStringMap(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k, _ := range m {
		keys = append(keys, k)
	}
	return keys

}

func GetKeysStructMap(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k, _ := range m {
		keys = append(keys, k)
	}
	return keys

}

// GetAircraftLivery is DEPRECATED
// Use AircraftLiveryService.GetAircraftLivery instead for DB-backed livery lookups
// This function remains for backwards compatibility but will be removed in a future version

func GetSessionId(c *cache.CacheService, server string) *string {
	val, found := c.Get(string(constants.CachePrefixWorldDetails))
	if !found {
		return nil
	}

	if sessions, ok := val.([]dtos.Session); ok {
		for _, session := range sessions {
			if server == session.Name {
				return &session.ID
			}
		}
	}
	return nil
}
func ContainsFlightID(summaries []dtos.FlightSummary, id string) bool {
	for _, f := range summaries {
		if f.FlightID == id {
			return true
		}
	}
	return false
}

func GetFlightFromCache(c *cache.CacheService, flightId string) *dtos.FlightInfo {

	log.Printf("\nFinding key: %s\n", string(constants.CachePrefixFlightHistory)+flightId)
	val, found := c.Get(string(constants.CachePrefixFlightHistory) + flightId)
	if !found {
		return nil
	}

	if flight, ok := val.(dtos.FlightInfo); ok {
		return &flight
	}
	return nil
}

func GetUserFlightsFromCache(c *cache.CacheService, userID string) *dtos.UserFlights {

	log.Printf("\nFinding key: %s\n", string(constants.CachePrefixUserFlights)+userID)
	val, found := c.Get(string(constants.CachePrefixUserFlights) + userID)
	if !found {
		return nil
	}
	log.Printf("\nData Found: %v\nType: %T", val, val)

	if flight, ok := val.([]dtos.FlightSummary); ok {
		return &dtos.UserFlights{
			Flights: flight,
		}
	}
	return nil
}

func GetExpertServer(c *cache.CacheService) *string {
	val, found := c.Get(string(constants.CachePrefixExpertServer))
	if !found {
		return nil
	}

	if serverID, ok := val.(string); ok {
		return &serverID
	}
	return nil
}

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

func GetShortLiveryName(name string) string {
	if code, ok := constants.LiveryShortNames[name]; ok {
		return code
	}
	// fallback
	runes := []rune(name)
	if len(runes) > 4 {
		return strings.ToUpper(string(runes[:4]))
	}
	return strings.ToUpper(name)
}

// ParseLiveAPITime converts strings like
// "2025-07-27 09:57:51Z"  →  time.Time (UTC)
func ParseLiveAPITime(s string) (time.Time, error) {
	const layout = "2006-01-02 15:04:05Z07:00" // space-separated, UTC suffix

	return time.Parse(layout, s)
}
