package flights

const (
	PilotStateActive       = 0
	PilotStateAwayInFlight = 1
	PilotStateAwayParked   = 2
	PilotStateInBackground = 3

	PilotStateNameActive       = "active"
	PilotStateNameAwayInFlight = "away_in_flight"
	PilotStateNameAwayParked   = "away_parked"
	PilotStateNameInBackground = "in_background"
)

var pilotStateNames = []string{
	PilotStateNameActive,
	PilotStateNameAwayInFlight,
	PilotStateNameAwayParked,
	PilotStateNameInBackground,
}

func PilotStateNames() []string {
	names := make([]string, len(pilotStateNames))
	copy(names, pilotStateNames)
	return names
}

func NameForPilotState(code int) string {
	switch code {
	case PilotStateActive:
		return PilotStateNameActive
	case PilotStateAwayInFlight:
		return PilotStateNameAwayInFlight
	case PilotStateAwayParked:
		return PilotStateNameAwayParked
	case PilotStateInBackground:
		return PilotStateNameInBackground
	default:
		return PilotStateNameActive
	}
}

func ParsePilotStateName(name string) (string, bool) {
	switch name {
	case PilotStateNameActive, PilotStateNameAwayInFlight, PilotStateNameAwayParked, PilotStateNameInBackground:
		return name, true
	default:
		return "", false
	}
}
