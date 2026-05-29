package flights

import "testing"

func TestDetermineInitialPhaseDoesNotMarkHighCruiseAsOnGround(t *testing.T) {
	trend := FlightTrend{AltitudeStable: true, SpeedStable: true}

	phase := determineInitialPhase(0, impossibleGroundAltitudeFt+1, trend, true)

	if phase != PhaseCruise {
		t.Fatalf("expected %q, got %q", PhaseCruise, phase)
	}
}

func TestDetermineInitialPhaseDoesNotMarkFastFlightAsOnGround(t *testing.T) {
	trend := FlightTrend{AltitudeStable: true, SpeedStable: true}

	phase := determineInitialPhase(impossibleGroundSpeedKt+1, 5000, trend, true)

	if phase != PhaseCruise {
		t.Fatalf("expected %q, got %q", PhaseCruise, phase)
	}
}

func TestDetermineInitialPhaseKeepsLowSlowStableOnGround(t *testing.T) {
	trend := FlightTrend{AltitudeStable: true, SpeedStable: true}

	phase := determineInitialPhase(groundSpeedThreshold, 250, trend, true)

	if phase != PhaseOnGround {
		t.Fatalf("expected %q, got %q", PhaseOnGround, phase)
	}
}

func TestUpdateFlightPhaseMovesImpossibleGroundStateToCruise(t *testing.T) {
	flight := &CompleteFlight{
		Phase:         PhaseOnGround,
		Altitude:      impossibleGroundAltitudeFt + 1000,
		Speed:         impossibleGroundSpeedKt + 20,
		VerticalSpeed: 0,
		TrendQueue: FlightTrendQueue{Items: []FlightTrendPoint{
			{Altitude: impossibleGroundAltitudeFt + 1000, Speed: impossibleGroundSpeedKt + 20},
			{Altitude: impossibleGroundAltitudeFt + 1000, Speed: impossibleGroundSpeedKt + 20},
		}},
	}

	updateFlightPhase(flight, flight.Speed, flight.Altitude)

	if flight.Phase != PhaseCruise {
		t.Fatalf("expected %q, got %q", PhaseCruise, flight.Phase)
	}
}
