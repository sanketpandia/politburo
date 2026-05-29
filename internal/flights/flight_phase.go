package flights

import "time"

const (
	groundSpeedThreshold       = 50
	flightSpeedThreshold       = 120
	landingSpeedThreshold      = 160
	impossibleGroundAltitudeFt = 15000
	impossibleGroundSpeedKt    = 150
)

func updateFlightPhase(flight *CompleteFlight, speed int, altitude int) {
	now := time.Now().UTC()
	prevPhase := flight.Phase
	trend := calculateTrendFromQueue(flight.TrendQueue)
	nearZeroVerticalSpeed := flight.VerticalSpeed > -150 && flight.VerticalSpeed < 150
	impossibleOnGround := altitude > impossibleGroundAltitudeFt || speed > impossibleGroundSpeedKt

	if prevPhase == PhaseUnknown || prevPhase == "" {
		flight.Phase = determineInitialPhase(speed, altitude, trend, nearZeroVerticalSpeed)
		markPhaseChange(flight, prevPhase, now)
		if flight.Phase != PhaseOnGround && flight.Phase != PhaseUnknown && flight.TakeoffTime == nil {
			takeoffTime := now.UTC()
			flight.TakeoffTime = &takeoffTime
		}
		return
	}

	nextPhase := prevPhase
	switch prevPhase {
	case PhaseOnGround:
		if impossibleOnGround && trend.AltitudeFalling {
			nextPhase = PhaseDescent
		} else if impossibleOnGround && trend.AltitudeStable {
			nextPhase = PhaseCruise
		} else if impossibleOnGround || trend.SpeedIncreasing && trend.AltitudeRising {
			nextPhase = PhaseTakeoff
		}
	case PhaseTakeoff:
		if speed >= flightSpeedThreshold && trend.AltitudeRising {
			nextPhase = PhaseClimb
		} else if trend.SpeedDecreasing && trend.AltitudeStable && nearZeroVerticalSpeed {
			nextPhase = PhaseOnGround
		}
	case PhaseClimb:
		if trend.AltitudeStable && trend.SpeedStable {
			nextPhase = PhaseCruise
		} else if trend.AltitudeFalling {
			nextPhase = PhaseDescent
		}
	case PhaseCruise:
		if trend.AltitudeFalling {
			nextPhase = PhaseDescent
		} else if trend.AltitudeRising {
			nextPhase = PhaseClimb
		}
	case PhaseDescent:
		if trend.AltitudeRising {
			nextPhase = PhaseClimb
		} else if trend.SpeedDecreasing && trend.AltitudeFalling && speed <= landingSpeedThreshold {
			nextPhase = PhaseLanded
		}
	case PhaseLanded:
		if impossibleOnGround && trend.AltitudeFalling {
			nextPhase = PhaseDescent
		} else if impossibleOnGround && trend.AltitudeStable {
			nextPhase = PhaseCruise
		} else if speed <= groundSpeedThreshold && trend.AltitudeStable && nearZeroVerticalSpeed {
			nextPhase = PhaseOnGround
		} else if impossibleOnGround || trend.AltitudeRising && speed >= flightSpeedThreshold {
			nextPhase = PhaseClimb
		}
	}

	flight.Phase = nextPhase
	markPhaseChange(flight, prevPhase, now)
	if flight.Phase == PhaseTakeoff && flight.TakeoffTime == nil {
		flight.Phase = PhaseTakeoff
		takeoffTime := now.UTC()
		flight.TakeoffTime = &takeoffTime
	}
	if flight.Phase == PhaseLanded && flight.LandingTime == nil {
		landingTime := now.UTC()
		flight.LandingTime = &landingTime
	}
}

func determineInitialPhase(speed int, altitude int, trend FlightTrend, nearZeroVerticalSpeed bool) FlightPhase {
	if altitude > impossibleGroundAltitudeFt || speed > impossibleGroundSpeedKt {
		switch {
		case trend.AltitudeFalling:
			return PhaseDescent
		case trend.AltitudeRising:
			return PhaseClimb
		default:
			return PhaseCruise
		}
	}

	switch {
	case speed <= groundSpeedThreshold && trend.AltitudeStable && nearZeroVerticalSpeed:
		return PhaseOnGround
	case speed >= flightSpeedThreshold && trend.AltitudeRising:
		return PhaseClimb
	case speed >= flightSpeedThreshold && trend.AltitudeStable:
		return PhaseCruise
	case speed >= flightSpeedThreshold && trend.AltitudeFalling:
		return PhaseDescent
	default:
		return PhaseUnknown
	}
}

func markPhaseChange(flight *CompleteFlight, previous FlightPhase, changedAt time.Time) {
	if flight.Phase == "" {
		flight.Phase = PhaseUnknown
	}
	if flight.Phase == previous {
		if len(flight.PhaseHistory) == 0 && flight.Phase != PhaseUnknown {
			flight.PhaseHistory = append(flight.PhaseHistory, FlightPhaseHistoryEntry{Phase: flight.Phase, ChangedAt: changedAt.UTC()})
		}
		return
	}
	flight.PhaseHistory = append(flight.PhaseHistory, FlightPhaseHistoryEntry{Phase: flight.Phase, ChangedAt: changedAt.UTC()})
}
