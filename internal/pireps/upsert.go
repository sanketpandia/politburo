package pireps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"infinite-experiment/politburo/infra/logging"
	platformVA "infinite-experiment/politburo/internal/platform/va"
)

// UpsertPirepFromAirtable maps raw Airtable fields onto a PirepATSynced row and
// persists it via ON CONFLICT upsert. It performs two supplementary lookups:
//
//  1. pilot_at_synced — resolves the pilot's IFC callsign from the Airtable pilot AT-ID.
//  2. route_at_synced — resolves the route string when the route field is a linked record.
//
// Both lookups are best-effort; a miss does not fail the upsert.
func UpsertPirepFromAirtable(
	ctx context.Context,
	db *gorm.DB,
	repo *Repository,
	vaID string,
	airtableRecordID string,
	record map[string]interface{},
	createdTime string,
	schema *platformVA.EntitySchema,
) error {
	// --- field mapping resolution ---

	// "callsign" (new config) aliases "pilot_callsign" (legacy config).
	callsignField := schema.GetFieldMapping("callsign")
	pilotCallsignField := schema.GetFieldMapping("pilot_callsign")
	if callsignField != nil && pilotCallsignField == nil {
		pilotCallsignField = callsignField
	}

	// "airline" (new config) aliases "livery" (legacy config).
	airlineField := schema.GetFieldMapping("airline")
	liveryField := schema.GetFieldMapping("livery")
	if airlineField != nil && liveryField == nil {
		liveryField = airlineField
	}

	routeField := schema.GetFieldMapping("route")
	flightModeField := schema.GetFieldMapping("flight_mode")
	flightTimeField := schema.GetFieldMapping("flight_time")
	aircraftField := schema.GetFieldMapping("aircraft")
	routeATIDField := schema.GetFieldMapping("route_at_id")
	pilotATIDField := schema.GetFieldMapping("pilot_at_id")

	// --- extract route ---
	// Route can be a plain string or a linked-record array; prefer the AT-ID from
	// the linked record so we can resolve the route string from route_at_synced.
	var route string
	var routeATIDFromRoute *string
	if routeField != nil {
		if rawRoute, ok := record[routeField.AirtableName]; ok {
			if idArray, ok := rawRoute.([]interface{}); ok && len(idArray) > 0 {
				if idStr, ok := idArray[0].(string); ok {
					routeATIDFromRoute = &idStr
				}
			} else if routeStr, ok := rawRoute.(string); ok {
				route = strings.TrimSpace(routeStr)
			}
		}
	}

	// --- extract flight mode ---
	var flightMode string
	if flightModeField != nil {
		if rawMode, ok := record[flightModeField.AirtableName]; ok {
			if modeStr, ok := rawMode.(string); ok {
				flightMode = strings.TrimSpace(modeStr)
			}
		}
	}

	// --- extract flight time ---
	var flightTime *float64
	if flightTimeField != nil {
		if rawTime, ok := record[flightTimeField.AirtableName]; ok {
			switch v := rawTime.(type) {
			case float64:
				flightTime = &v
			case int:
				ft := float64(v)
				flightTime = &ft
			}
		}
	}

	// --- extract pilot AT-ID from callsign linked-record field ---
	// We do not store the callsign string here; it is resolved from pilot_at_synced.
	var pilotATIDFromCallsign *string
	if pilotCallsignField != nil {
		if rawCallsign, ok := record[pilotCallsignField.AirtableName]; ok {
			if idArray, ok := rawCallsign.([]interface{}); ok && len(idArray) > 0 {
				if idStr, ok := idArray[0].(string); ok {
					pilotATIDFromCallsign = &idStr
				}
			}
		}
	}

	// --- extract aircraft ---
	var aircraft string
	if aircraftField != nil {
		if rawAircraft, ok := record[aircraftField.AirtableName]; ok {
			if aircraftStr, ok := rawAircraft.(string); ok {
				aircraft = strings.TrimSpace(aircraftStr)
			}
		}
	}

	// --- extract livery ---
	var livery string
	if liveryField != nil {
		if rawLivery, ok := record[liveryField.AirtableName]; ok {
			if liveryStr, ok := rawLivery.(string); ok {
				livery = strings.TrimSpace(liveryStr)
			}
		}
	}

	// --- resolve route_at_id ---
	// Priority: explicit field mapping > ID extracted from Route linked-record.
	var routeATID *string
	if routeATIDField != nil {
		if rawRouteID, ok := record[routeATIDField.AirtableName]; ok {
			if idArray, ok := rawRouteID.([]interface{}); ok && len(idArray) > 0 {
				if idStr, ok := idArray[0].(string); ok {
					routeATID = &idStr
				}
			}
		}
	}
	if routeATID == nil && routeATIDFromRoute != nil {
		routeATID = routeATIDFromRoute
	}

	// --- resolve pilot_at_id ---
	// Priority: explicit field mapping > ID extracted from Callsign linked-record.
	var pilotATID *string
	if pilotATIDField != nil {
		if rawPilotID, ok := record[pilotATIDField.AirtableName]; ok {
			if idArray, ok := rawPilotID.([]interface{}); ok && len(idArray) > 0 {
				if idStr, ok := idArray[0].(string); ok {
					pilotATID = &idStr
				}
			}
		}
	}
	if pilotATID == nil && pilotATIDFromCallsign != nil {
		pilotATID = pilotATIDFromCallsign
	}

	// --- supplementary DB lookups (best-effort) ---

	var pilotCallsign string
	if pilotATID != nil && *pilotATID != "" {
		type pilotSynced struct {
			Callsign string `gorm:"column:callsign"`
		}
		var ps pilotSynced
		if err := db.WithContext(ctx).
			Table("pilot_at_synced").
			Where("at_id = ? AND server_id = ?", *pilotATID, vaID).
			First(&ps).Error; err == nil {
			pilotCallsign = ps.Callsign
		}
	}

	if routeATID != nil && *routeATID != "" && route == "" {
		type routeSynced struct {
			Route string `gorm:"column:route"`
		}
		var rs routeSynced
		if err := db.WithContext(ctx).
			Table("route_at_synced").
			Where("at_id = ? AND server_id = ?", *routeATID, vaID).
			First(&rs).Error; err == nil {
			route = rs.Route
		}
	}

	// --- parse Airtable created_time ---
	var atCreatedTime *time.Time
	if createdTime != "" {
		if t, err := time.Parse(time.RFC3339, createdTime); err == nil {
			atCreatedTime = &t
		}
	}

	pirep := &PirepATSynced{
		ATID:          airtableRecordID,
		ServerID:      vaID,
		Route:         route,
		FlightMode:    flightMode,
		FlightTime:    flightTime,
		PilotCallsign: pilotCallsign,
		Aircraft:      aircraft,
		Livery:        livery,
		RouteATID:     routeATID,
		PilotATID:     pilotATID,
		ATCreatedTime: atCreatedTime,
	}

	if err := repo.Upsert(ctx, pirep); err != nil {
		return fmt.Errorf("failed to upsert PIREP %s: %w", airtableRecordID, err)
	}

	logging.Debug("PIREP upserted",
		"record_id", airtableRecordID,
		"va_id", vaID,
		"pilot", pilotCallsign,
		"route", route,
		"aircraft", aircraft,
		"livery", livery,
		"mode", flightMode,
	)

	return nil
}
