package pilots

import (
	"context"
	"fmt"
	"infinite-experiment/politburo/infra/cache"
	"infinite-experiment/politburo/infra/liveapi"
	"infinite-experiment/politburo/infra/logging"
	"infinite-experiment/politburo/internal/models/dtos"
	platformAircraft "infinite-experiment/politburo/internal/platform/aircraft"
	platformMemberships "infinite-experiment/politburo/internal/platform/memberships"

	"gorm.io/gorm"
)

type Service struct {
	cache  *cache.CacheService
	gormDB *gorm.DB
	repo   *platformMemberships.Repository
}

// LogbookService provides logbook functionality using liveapi directly
type LogbookService struct {
	liveAPIClient *liveapi.Client
	aircraftSvc   *platformAircraft.Service
}

// NewLogbookService creates a new logbook service
func NewLogbookService(liveAPIClient *liveapi.Client, aircraftSvc *platformAircraft.Service) *LogbookService {
	return &LogbookService{
		liveAPIClient: liveAPIClient,
		aircraftSvc:  aircraftSvc,
	}
}

// GetUserLogbook fetches flight history for a user by IFC ID
func (s *LogbookService) GetUserLogbook(ifcID string, page int) (*dtos.FlightHistoryDto, error) {
	response := &dtos.FlightHistoryDto{
		PageNo:      page,
		Error:       "",
		Records:     nil,
		HasNext:     false,
		HasPrevious: false,
		TotalPages:  0,
		TotalCount:  0,
	}

	// Step 1: Get user by IFC ID to get userID
	userStatsResp, statusCode, err := s.liveAPIClient.GetUserByIfcId(ifcID)
	if err != nil {
		logging.Warn("Failed to fetch user by IFC ID", "ifc_id", ifcID, "status", statusCode, "error", err)
		response.Error = "Unable to fetch user"
		return response, err
	}

	if userStatsResp == nil || len(userStatsResp.Result) < 1 {
		response.Error = "User not found"
		return response, fmt.Errorf("user not found for IFC ID: %s", ifcID)
	}

	// The first result is the user we're looking for
	userStats := userStatsResp.Result[0]
	userID := userStats.UserID
	username := ifcID // Use the IFC ID as the display name
	if userStats.DiscourseUsername != nil {
		username = *userStats.DiscourseUsername
	}

	logging.Info("Fetching logbook flights", "ifc_id", ifcID, "user_id", userID, "page", page)

	// Step 2: Get flights for the user
	flightsResp, statusCode, err := s.liveAPIClient.GetUserFlights(userID, page)
	if err != nil {
		logging.Warn("Failed to fetch user flights", "user_id", userID, "page", page, "status", statusCode, "error", err)
		response.Error = "Unable to fetch flights from Live API"
		return response, err
	}

	if flightsResp == nil || len(flightsResp.Flights) < 1 {
		response.Error = "No flights"
		return response, fmt.Errorf("empty result")
	}

	// Step 3: Get sessions to map server names to session IDs
	sessionsResp, err := s.liveAPIClient.GetSessions()
	serverSessionMap := make(map[string]string) // serverName → sessionID
	if err == nil && sessionsResp != nil {
		for _, session := range sessionsResp.Result {
			serverSessionMap[session.Name] = session.ID
			logging.Debug("Mapped session", "server", session.Name, "session_id", session.ID)
		}
	} else {
		logging.Warn("Could not fetch sessions for server mapping", "error", err)
	}

	// Step 4: Populate pagination metadata from Live API response
	response.HasNext = flightsResp.HasNext
	response.HasPrevious = flightsResp.HasPrevious
	response.TotalPages = flightsResp.TotalPages
	response.TotalCount = flightsResp.TotalCount

	// Step 5: Transform flights to HistoryRecord
	for _, rec := range flightsResp.Flights {
		// Get aircraft and livery names
		aircraftName := ""
		liveryName := ""
		if liveryData := s.aircraftSvc.GetAircraftLivery(context.Background(), rec.LiveryID); liveryData != nil {
			aircraftName = liveryData.AircraftName
			liveryName = liveryData.LiveryName
		}

		// Format duration from TotalTime (minutes) to "HH:MM"
		totalMinutes := int(rec.TotalTime)
		hours := totalMinutes / 60
		minutes := totalMinutes % 60
		dur := fmt.Sprintf("%02d:%02d", hours, minutes)

		// Map server name to session ID
		sessionID := serverSessionMap[rec.Server]

		// Create HistoryRecord
		dto := dtos.HistoryRecord{
			FlightID:   rec.ID,
			Origin:     rec.OriginAirport,
			Dest:       rec.DestinationAirport,
			TimeStamp:  rec.Created.UTC(),
			Landings:   rec.LandingCount,
			Server:     rec.Server,
			SessionID:  sessionID,
			Equipment:  fmt.Sprintf("%s %s", aircraftName, liveryName),
			Livery:     liveryName,
			Callsign:   rec.Callsign,
			Violations: len(rec.Violations),
			Duration:   dur,
			Aircraft:   aircraftName,
			DayTime:    rec.DayTime,
			NightTime:  rec.NightTime,
			XP:         rec.XP,
			WorldType:  rec.WorldType,
			Username:   username,
			MapUrl:     "", // Will be populated if route is available
		}

		response.Records = append(response.Records, dto)
	}

	return response, nil
}
