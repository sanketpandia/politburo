package pilots

import (
	"fmt"
	"math"

	platformVA "infinite-experiment/politburo/internal/platform/va"
)

func formatTimeSeconds(seconds interface{}) string {
	var secs int
	switch v := seconds.(type) {
	case float64:
		secs = int(math.Round(v))
	case int:
		secs = v
	case int64:
		secs = int(v)
	default:
		return fmt.Sprintf("%v", seconds)
	}
	hours := secs / 3600
	mins := (secs % 3600) / 60
	return fmt.Sprintf("%02d:%02d", hours, mins)
}

func isTimeField(field platformVA.FieldMapping) bool {
	return field.DataType == "time" || (field.DisplayFormat != nil && *field.DisplayFormat == "duration")
}

type statsFieldMapper struct{}

func newStatsFieldMapper() *statsFieldMapper { return &statsFieldMapper{} }

func (m *statsFieldMapper) TransformProviderFields(rawFields map[string]interface{}, schema *platformVA.EntitySchema) *ProviderPilotData {
	data := &ProviderPilotData{AdditionalFields: make(map[string]interface{})}
	for _, field := range schema.Fields {
		if !field.IsUserVisible {
			continue
		}
		value, exists := rawFields[field.AirtableName]
		if !exists {
			continue
		}
		if isTimeField(field) {
			value = formatTimeSeconds(value)
		}
		switch field.InternalName {
		case "flight_hours":
			var iface interface{}
			if isTimeField(field) {
				iface = value
			} else if v, ok := value.(float64); ok {
				iface = v
			} else if v, ok := value.(int); ok {
				iface = float64(v)
			} else {
				iface = value
			}
			data.FlightHours = &iface
		case "rank":
			if v, ok := value.(string); ok {
				data.Rank = &v
			}
		case "join_date":
			if v, ok := value.(string); ok {
				data.JoinDate = &v
			}
		case "last_activity":
			if v, ok := value.(string); ok {
				data.LastActivity = &v
			}
		case "last_flight":
			if v, ok := value.(string); ok {
				data.LastFlight = &v
			}
		case "region":
			if v, ok := value.(string); ok {
				data.Region = &v
			}
		case "total_flights":
			if v, ok := value.(float64); ok {
				intVal := int(v)
				data.TotalFlights = &intVal
			} else if v, ok := value.(int); ok {
				data.TotalFlights = &v
			}
		case "status":
			if v, ok := value.(string); ok {
				data.Status = &v
			}
		default:
			fieldKey := field.DisplayName
			if fieldKey == "" {
				fieldKey = field.InternalName
			}
			data.AdditionalFields[fieldKey] = value
		}
	}
	return data
}

func (m *statsFieldMapper) TransformCareerModeFields(rawFields map[string]interface{}, schema *platformVA.EntitySchema) *CareerModeData {
	data := &CareerModeData{AdditionalFields: make(map[string]interface{})}
	for _, field := range schema.Fields {
		if !field.IsUserVisible {
			continue
		}
		value, exists := rawFields[field.AirtableName]
		if !exists {
			continue
		}
		if isTimeField(field) {
			value = formatTimeSeconds(value)
		}
		switch field.InternalName {
		case "total_cm_hours":
			var iface interface{}
			if isTimeField(field) {
				iface = value
			} else {
				iface = value
			}
			data.TotalCMHours = &iface
		case "required_hours_to_next":
			var iface interface{}
			if isTimeField(field) {
				iface = value
			} else {
				iface = value
			}
			data.RequiredHoursToNext = &iface
		case "last_activity_cm":
			if v, ok := value.(string); ok {
				data.LastActivityCM = &v
			}
		case "assigned_routes":
			data.AssignedRoutes = &value
		case "aircraft":
			if v, ok := value.(string); ok {
				data.Aircraft = &v
			}
		case "airline":
			if v, ok := value.(string); ok {
				data.Airline = &v
			}
		case "last_career_mode_pirep":
			data.LastCareerModePIREP = &value
		case "last_flown_route":
			if str := fmt.Sprintf("%v", value); str != "" {
				data.LastCareerModeFlight = &str
			}
		default:
			fieldKey := field.DisplayName
			if fieldKey == "" {
				fieldKey = field.InternalName
			}
			data.AdditionalFields[fieldKey] = value
		}
	}
	return data
}
