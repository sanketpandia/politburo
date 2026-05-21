package flights

import (
	"strings"

	"infinite-experiment/politburo/infra/cache"
)

type FlightAggregation struct {
	SessionFlights    map[string][]string
	VAFlights         map[string][]string
	TotalFlights      int
	ProcessedSessions int
}

func NewFlightAggregation() FlightAggregation {
	return FlightAggregation{
		SessionFlights: make(map[string][]string),
		VAFlights:      make(map[string][]string),
	}
}

func (a *FlightAggregation) AddFlight(sessionID string, flightID string, vaIDs []string) {
	a.SessionFlights[sessionID] = append(a.SessionFlights[sessionID], flightID)
	for _, vaID := range vaIDs {
		a.VAFlights[vaID] = append(a.VAFlights[vaID], flightID)
	}
	a.TotalFlights++
}

func (a *FlightAggregation) Merge(other FlightAggregation) {
	for sessionID, flightIDs := range other.SessionFlights {
		a.SessionFlights[sessionID] = append(a.SessionFlights[sessionID], flightIDs...)
	}
	for vaID, flightIDs := range other.VAFlights {
		a.VAFlights[vaID] = append(a.VAFlights[vaID], flightIDs...)
	}
	a.TotalFlights += other.TotalFlights
	a.ProcessedSessions += other.ProcessedSessions
}

func (j *CacheJob) storeSessionFlightIndexes(sessionFlights map[string][]string) {
	for sessionID, flightIDs := range sessionFlights {
		j.redisCache.Set(cache.LiveFlightsKey(sessionID), strings.Join(flightIDs, "|"), cache.LiveFlightListTTL)
	}
}

func (j *CacheJob) storeVAFlightIndexes(vaFlights map[string][]string) {
	for vaID, flightIDs := range vaFlights {
		j.redisCache.Set(cache.LiveVAFlightsKey(vaID), strings.Join(flightIDs, "|"), cache.LiveFlightListTTL)
	}
}
