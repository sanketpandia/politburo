package flights

import "time"

func updateFlightPhase(flight *CompleteFlight, speed int, altitude int) {
	now := time.Now().UTC()
	prevPhase := flight.Phase
	trend := calculateTrendFromQueue(flight.TrendQueue)

	if prevPhase == PhaseUnknown || prevPhase == "" {
		flight.Phase = determineInitialPhase(speed, altitude)
		if flight.Phase != PhaseOnGround && flight.Phase != PhaseUnknown && flight.TakeoffTime == nil {
			takeoffTime := now.UTC()
			flight.TakeoffTime = &takeoffTime
		}
		return
	}

	switch {
	case speed < 50:
		if prevPhase == PhaseDescent || prevPhase == PhaseCruise || prevPhase == PhaseClimb {
			flight.Phase = PhaseLanded
			if flight.LandingTime == nil {
				landingTime := now.UTC()
				flight.LandingTime = &landingTime
			}
			return
		}
		flight.Phase = PhaseOnGround

	case (prevPhase == PhaseOnGround || prevPhase == PhaseLanded) && speed > 80 && (trend.AltitudeRising || trend.SpeedIncreasing):
		flight.Phase = PhaseTakeoff
		if flight.TakeoffTime == nil {
			takeoffTime := now.UTC()
			flight.TakeoffTime = &takeoffTime
		}

	case (prevPhase == PhaseTakeoff || prevPhase == PhaseOnGround) && trend.AltitudeRising && altitude > 1000:
		flight.Phase = PhaseClimb

	case (prevPhase == PhaseClimb || prevPhase == PhaseTakeoff) && trend.AltitudeStable && (altitude > 8000 || speed > 250):
		flight.Phase = PhaseCruise

	case (prevPhase == PhaseCruise || prevPhase == PhaseClimb) && trend.AltitudeFalling:
		flight.Phase = PhaseDescent

	default:
		flight.Phase = prevPhase
		if flight.Phase == "" {
			flight.Phase = PhaseUnknown
		}
	}
}

func determineInitialPhase(speed int, altitude int) FlightPhase {
	switch {
	case speed < 50:
		return PhaseOnGround
	case altitude > 30000 || speed > 300:
		return PhaseCruise
	case altitude > 8000 && speed > 80:
		return PhaseClimb
	case altitude < 15000 && speed > 50 && altitude > 1000:
		return PhaseDescent
	case speed > 80 && altitude < 1000:
		return PhaseTakeoff
	case speed > 80:
		return PhaseTakeoff
	default:
		return PhaseUnknown
	}
}
