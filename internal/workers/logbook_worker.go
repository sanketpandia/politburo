package workers

import (
	"infinite-experiment/politburo/infra/cache"
	"context"
	"fmt"
	"infinite-experiment/politburo/internal/common"
	"infinite-experiment/politburo/internal/models/dtos"
	"infinite-experiment/politburo/internal/platform/aircraft"
	"log"
	"math"
	"time"
)

type LogbookRequest struct {
	FlightId  string
	Flight    dtos.UserFlightEntry
	SessionId string
	CacheKey  string
}

var LogbookQueue = make(chan LogbookRequest, 100)

// getAltitudeColor calculates a hex color based on altitude (green -> yellow -> red gradient)
// 0 ft = green (#A3BE8C), 22.5k ft = yellow (#EBCB8B), 45k ft = red (#BF616A)
func getAltitudeColor(altitude int, maxAltitude int) string {
	if maxAltitude == 0 {
		maxAltitude = 45000
	}

	ratio := math.Min(float64(altitude)/float64(maxAltitude), 1.0)

	var r, g, b int
	if ratio <= 0.5 {
		// Green to Yellow: interpolate between #A3BE8C and #EBCB8B
		t := ratio * 2
		r = int(163 + (235-163)*t)
		g = int(190 + (203-190)*t)
		b = int(140 + (139-140)*t)
	} else {
		// Yellow to Red: interpolate between #EBCB8B and #BF616A
		t := (ratio - 0.5) * 2
		r = int(235 + (191-235)*t)
		g = int(203 + (97-203)*t)
		b = int(139 + (106-139)*t)
	}

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// formatDuration converts seconds to HH:MM format
func formatDuration(seconds int) string {
	hours := seconds / 3600
	mins := (seconds % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, mins)
}

func LogbookWorker(cache cache.CacheInterface, liveApiService *common.LiveAPIService, liverySvc *aircraft.Service) {
	log.Printf("[DEBUG] LogbookWorker started, queue_addr=%p", LogbookQueue)
	for req := range LogbookQueue {

		fmt.Printf("✅ Logbook queue called for %s points for flight %s", req.SessionId, req.FlightId)

		cacheKey := req.CacheKey

		if req.Flight.DestinationAirport == "" || req.Flight.OriginAirport == "" {
			log.Println("❌ Skipping: missing origin or destination")
			continue
		}
		if val, found := cache.Get(cacheKey); found && val != nil {
			log.Printf("⚠️  Flight %s already cached, skipping\n", req.FlightId)
			continue
		}

		session := req.SessionId
		// Call the Live API
		data, _, err := liveApiService.GetFlightRoute(req.FlightId, session)
		if err != nil {
			fmt.Printf("❌ Error fetching flight path: %v\n", err)
			continue
		}

		var (
			maxGS  int
			maxAlt int
		)

		var waypoints []dtos.RouteWaypoint
		for _, pos := range data.Result {
			gs := int(pos.GroundSpeed)
			alt := int(pos.Altitude)
			waypoints = append(waypoints, dtos.RouteWaypoint{
				Lat:         fmt.Sprintf("%f", pos.Latitude),
				Long:        fmt.Sprintf("%f", pos.Longitude),
				Altitude:    int(pos.Altitude),
				Timestamp:   pos.Date,
				GroundSpeed: gs,
			})

			if gs > maxGS {
				maxGS = gs
			}

			if alt > maxAlt {
				maxAlt = alt
			}
		}

		originNode := dtos.RouteNode{
			Name: req.Flight.OriginAirport,
			Lat:  "",
			Long: "",
		}
		destNode := dtos.RouteNode{
			Name: req.Flight.DestinationAirport,
			Lat:  "",
			Long: "",
		}

		if len(waypoints) > 0 {
			originNode.Lat = waypoints[0].Lat
			originNode.Long = waypoints[0].Long
			destNode.Lat = waypoints[len(waypoints)-1].Lat
			destNode.Long = waypoints[len(waypoints)-1].Long
		}
		totalMinutes := int(req.Flight.TotalTime)

		// Use new livery service (cache-first, then DB)
		aircraftName := ""
		liveryName := ""
		if liveryData := liverySvc.GetAircraftLivery(context.Background(), req.Flight.LiveryID); liveryData != nil {
			aircraftName = liveryData.AircraftName
			liveryName = liveryData.LiveryName
		}

		flightInfo := dtos.FlightInfo{
			Meta: dtos.FlightMeta{
				Aircraft:   aircraftName,
				Livery:     liveryName,
				MaxSpeed:   maxGS,
				MaxAlt:     maxAlt,
				Violations: len(req.Flight.Violations),
				Landings:   req.Flight.LandingCount,
				Duration:   totalMinutes,
				StartedAt:  req.Flight.Created,
			},
			Route:     waypoints,
			Origin:    originNode,
			Dest:      destNode,
			SessionID: req.SessionId,
			Callsign:  req.Flight.Callsign,
			DayTime:   req.Flight.DayTime,
			NightTime: req.Flight.NightTime,
			XP:        req.Flight.XP,
			WorldType: req.Flight.WorldType,
		}

		// Cache for 7 days (604800 seconds)
		cache.Set(cacheKey, flightInfo, 7*24*time.Hour)
		log.Printf("✅ Cached %d points for flight %s\nCACHE_KEY=%s", len(data.Result), req.FlightId, cacheKey)

	}
}
