package flights

import "time"

const (
	waypointInterval     = 100 * time.Second
	maxWaypointSnapshots = 600
)

func appendWaypoint(flight *CompleteFlight) {
	now := time.Now().UTC()
	if !flight.LastUpdatedWaypoint.IsZero() && time.Since(flight.LastUpdatedWaypoint) < waypointInterval {
		return
	}

	flight.Waypoints = append(flight.Waypoints, WaypointSnapshot{
		Timestamp: now.UTC(),
		Latitude:  flight.Latitude,
		Longitude: flight.Longitude,
		Altitude:  flight.Altitude,
		Speed:     flight.Speed,
		Track:     flight.Track,
	})
	flight.LastUpdatedWaypoint = now.UTC()

	if len(flight.Waypoints) > maxWaypointSnapshots {
		flight.Waypoints = flight.Waypoints[len(flight.Waypoints)-maxWaypointSnapshots:]
	}
}
