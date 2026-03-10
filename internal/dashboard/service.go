package dashboard

import (
	"context"
	"fmt"
	"time"

	"infinite-experiment/politburo/internal/events"
	"infinite-experiment/politburo/internal/pilots"
	"infinite-experiment/politburo/internal/pireps"

	"gorm.io/gorm"
)

// Service handles dashboard business logic
type Service struct {
	eventSvc   *events.Service
	pirepRepo  *pireps.Repository
	statsSvc   *pilots.StatsService
	db         *gorm.DB
}

// NewService creates a new dashboard service
func NewService(eventSvc *events.Service, pirepRepo *pireps.Repository, statsSvc *pilots.StatsService, db *gorm.DB) *Service {
	return &Service{
		eventSvc:  eventSvc,
		pirepRepo: pirepRepo,
		statsSvc:  statsSvc,
		db:        db,
	}
}

// GetPilotStats fetches pilot statistics for the current user
func (s *Service) GetPilotStats(ctx context.Context, userDiscordID, vaID string) (*pilots.StatsResponse, error) {
	return s.statsSvc.GetPilotStats(ctx, userDiscordID, vaID)
}

// LeaderboardEntry represents a single leaderboard entry
type LeaderboardEntry struct {
	PilotATID     string `json:"pilot_at_id"`
	PilotCallsign string `json:"pilot_callsign"`
	IFCommunityID string `json:"if_community_id"` // IFC ID from users table
	PirepCount    int    `json:"pirep_count"`
	Rank          int    `json:"rank"`
}

// PilotPirepLog represents a single PIREP log entry for a pilot
type PilotPirepLog struct {
	ATID          string  `json:"at_id"`
	Route         string  `json:"route"`
	LegNumber     int     `json:"leg_number"`
	FlightMode    string  `json:"flight_mode"`
	FlightTime    *float64 `json:"flight_time"`
	Aircraft      string  `json:"aircraft"`
	Livery        string  `json:"livery"`
	ATCreatedTime string  `json:"at_created_time"` // ISO 8601 timestamp
}

// GetEventLeaderboard retrieves the leaderboard for the active multi-leg event
// Returns leaderboard entries sorted by pirep count (desc) and then by max leg number (desc)
func (s *Service) GetEventLeaderboard(ctx context.Context, vaID string) ([]LeaderboardEntry, *events.Event, error) {
	// Get active multi-leg event
	activeEvent, err := s.eventSvc.GetActiveMultiLegEvent(ctx, vaID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get active multi-leg event: %w", err)
	}

	// If no active event, return empty leaderboard
	if activeEvent == nil {
		return []LeaderboardEntry{}, nil, nil
	}

	// Get flight mode from event
	if activeEvent.FlightMode == nil || *activeEvent.FlightMode == "" {
		return []LeaderboardEntry{}, activeEvent, nil
	}
	flightMode := *activeEvent.FlightMode

	// Build a map of route_at_id -> leg_number for quick lookup
	routeToLegMap := make(map[string]int)
	for _, leg := range activeEvent.Legs {
		if leg.RouteATID != nil && *leg.RouteATID != "" {
			routeToLegMap[*leg.RouteATID] = leg.LegNumber
		}
	}

	// Query pireps for this VA and flight mode
	// We'll need to do a raw SQL query to group by pilot and calculate max leg number
	// Also join with pilot_at_synced and va_user_roles/users to get IFC ID
	type LeaderboardResult struct {
		PilotATID         string `gorm:"column:pilot_at_id"`
		PilotCallsign     string `gorm:"column:pilot_callsign"`
		IFCommunityID     string `gorm:"column:if_community_id"`
		PirepCount        int64  `gorm:"column:pirep_count"`
		MaxLegNumber      *int   `gorm:"column:max_leg_number"`
		LatestLegTime     *string `gorm:"column:latest_leg_time"` // Earliest at_created_time for the latest leg
	}

	var results []LeaderboardResult

	// Build SQL query:
	// 1. Filter by server_id and flight_mode
	// 2. Group by pilot_at_id
	// 3. Count pireps per pilot
	// 4. Get max leg_number by matching route_at_id to event legs
	// 5. Get earliest at_created_time for PIREPs matching the max leg number
	// 6. Join with pilot_at_synced to get callsign, then va_user_roles/users to get IFC ID
	// Sorting: pirep_count DESC, max_leg_number DESC, latest_leg_time ASC (earliest first)
	query := `
		WITH pilot_legs AS (
			SELECT 
				p.pilot_at_id,
				p.route_at_id,
				p.at_created_time,
				CASE 
					WHEN p.route_at_id IS NOT NULL AND p.route_at_id != '' THEN
						COALESCE(
							(SELECT el.leg_number 
							 FROM event_legs el 
							 WHERE el.event_id = ? 
							   AND el.route_at_id = p.route_at_id 
							 LIMIT 1), 0)
					ELSE 0
				END as leg_number
			FROM pirep_at_synced p
			WHERE p.server_id = ? 
			  AND p.flight_mode = ?
			  AND p.pilot_at_id IS NOT NULL
			  AND p.pilot_at_id != ''
		),
		pilot_stats AS (
			SELECT 
				pilot_at_id,
				COUNT(*) as pirep_count,
				MAX(leg_number) as max_leg_number
			FROM pilot_legs
			GROUP BY pilot_at_id
		),
		latest_leg_times AS (
			SELECT 
				pl.pilot_at_id,
				MIN(pl.at_created_time) as latest_leg_time
			FROM pilot_legs pl
			INNER JOIN pilot_stats ps ON ps.pilot_at_id = pl.pilot_at_id
			WHERE pl.leg_number = ps.max_leg_number
			GROUP BY pl.pilot_at_id
		)
		SELECT 
			ps.pilot_at_id,
			MAX(pas.callsign) as pilot_callsign,
			MAX(u.if_community_id) as if_community_id,
			ps.pirep_count,
			llt.latest_leg_time
		FROM pilot_stats ps
		LEFT JOIN latest_leg_times llt ON llt.pilot_at_id = ps.pilot_at_id
		LEFT JOIN pilot_at_synced pas ON pas.at_id = ps.pilot_at_id AND pas.server_id = ?
		LEFT JOIN va_user_roles vur ON vur.va_id = ? 
			AND vur.callsign = pas.callsign 
			AND vur.is_active = true
		LEFT JOIN users u ON u.id = vur.user_id
		GROUP BY ps.pilot_at_id, ps.pirep_count, llt.latest_leg_time
		ORDER BY ps.pirep_count DESC, llt.latest_leg_time ASC NULLS LAST
	`

	err = s.db.WithContext(ctx).Raw(query, activeEvent.ID, vaID, flightMode, vaID, vaID).Scan(&results).Error
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query leaderboard: %w", err)
	}

	// Convert results to leaderboard entries
	entries := make([]LeaderboardEntry, 0, len(results))
	for i, result := range results {
		entries = append(entries, LeaderboardEntry{
			PilotATID:     result.PilotATID,
			PilotCallsign: result.PilotCallsign,
			IFCommunityID: result.IFCommunityID,
			PirepCount:    int(result.PirepCount),
			Rank:          i + 1,
		})
	}

	return entries, activeEvent, nil
}

// GetPilotPirepLogs retrieves all PIREP logs for a specific pilot in the active event
// Returns logs sorted by at_created_time ascending
func (s *Service) GetPilotPirepLogs(ctx context.Context, vaID string, pilotATID string) ([]PilotPirepLog, *events.Event, error) {
	// Get active multi-leg event
	activeEvent, err := s.eventSvc.GetActiveMultiLegEvent(ctx, vaID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get active multi-leg event: %w", err)
	}

	// If no active event, return empty
	if activeEvent == nil {
		return []PilotPirepLog{}, nil, nil
	}

	// Get flight mode from event
	if activeEvent.FlightMode == nil || *activeEvent.FlightMode == "" {
		return []PilotPirepLog{}, activeEvent, nil
	}
	flightMode := *activeEvent.FlightMode

	// Build a map of route_at_id -> leg_number for quick lookup
	routeToLegMap := make(map[string]int)
	for _, leg := range activeEvent.Legs {
		if leg.RouteATID != nil && *leg.RouteATID != "" {
			routeToLegMap[*leg.RouteATID] = leg.LegNumber
		}
	}

	// Query PIREPs for this pilot
	type PirepLogResult struct {
		ATID          string     `gorm:"column:at_id"`
		Route         string     `gorm:"column:route"`
		RouteATID     *string    `gorm:"column:route_at_id"`
		FlightMode    string     `gorm:"column:flight_mode"`
		FlightTime    *float64   `gorm:"column:flight_time"`
		Aircraft      string     `gorm:"column:aircraft"`
		Livery        string     `gorm:"column:livery"`
		ATCreatedTime *time.Time `gorm:"column:at_created_time"`
	}

	var results []PirepLogResult

	query := `
		SELECT 
			p.at_id,
			p.route,
			p.route_at_id,
			p.flight_mode,
			p.flight_time,
			p.aircraft,
			p.livery,
			p.at_created_time
		FROM pirep_at_synced p
		WHERE p.server_id = ? 
		  AND p.flight_mode = ?
		  AND p.pilot_at_id = ?
		  AND p.pilot_at_id IS NOT NULL
		  AND p.pilot_at_id != ''
		ORDER BY p.at_created_time ASC NULLS LAST
	`

	err = s.db.WithContext(ctx).Raw(query, vaID, flightMode, pilotATID).Scan(&results).Error
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query pilot PIREP logs: %w", err)
	}

	// Convert results to log entries
	logs := make([]PilotPirepLog, 0, len(results))
	for _, result := range results {
		legNumber := 0
		if result.RouteATID != nil && *result.RouteATID != "" {
			if legNum, ok := routeToLegMap[*result.RouteATID]; ok {
				legNumber = legNum
			}
		}

		atCreatedTimeStr := ""
		if result.ATCreatedTime != nil {
			atCreatedTimeStr = result.ATCreatedTime.Format(time.RFC3339)
		}

		logs = append(logs, PilotPirepLog{
			ATID:          result.ATID,
			Route:         result.Route,
			LegNumber:     legNumber,
			FlightMode:    result.FlightMode,
			FlightTime:    result.FlightTime,
			Aircraft:      result.Aircraft,
			Livery:        result.Livery,
			ATCreatedTime: atCreatedTimeStr,
		})
	}

	return logs, activeEvent, nil
}
