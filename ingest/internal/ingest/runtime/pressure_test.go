package runtime

/*
Name: ingest/internal/ingest/runtime/pressure_test.go
Description: Tests pressure state hysteresis transitions across normal, elevated, high, and critical states.
Programmer: Barrett Brown
Date Created: 2026-03-14
Dates Revised: 2026-03-14
Revision History:
- 2026-03-14, Barrett Brown: Added pressure hysteresis coverage tests.
Preconditions:
- Pressure threshold config is valid for hysteresis checks.
Acceptable Input Values/Types:
- Occupancy ratios within and around threshold boundaries.
Unacceptable Input Values/Types:
- No special invalid runtime dependencies are required.
Postconditions:
- Confirms pressure transitions follow the configured hysteresis rules.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Unexpected pressure transitions fail the tests.
Side Effects:
- None.
Invariants:
- nextPressureState should preserve hysteresis and avoid flapping.
Known Faults:
- Does not benchmark transition cost.
*/

import (
	"testing"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
)

func TestNextPressureStateTransitionsAcrossThresholds(t *testing.T) {
	cfg := config.PressureThresholds{
		ElevatedEnter: 0.70,
		ElevatedExit:  0.50,
		HighEnter:     0.85,
		HighExit:      0.65,
		CriticalEnter: 0.95,
		CriticalExit:  0.80,
	}

	if got := nextPressureState(PressureNormal, 0.71, cfg); got != PressureElevated {
		t.Fatalf("normal -> %s, want elevated", got)
	}
	if got := nextPressureState(PressureElevated, 0.86, cfg); got != PressureHigh {
		t.Fatalf("elevated -> %s, want high", got)
	}
	if got := nextPressureState(PressureHigh, 0.96, cfg); got != PressureCritical {
		t.Fatalf("high -> %s, want critical", got)
	}
	if got := nextPressureState(PressureCritical, 0.81, cfg); got != PressureCritical {
		t.Fatalf("critical hysteresis hold -> %s, want critical", got)
	}
	if got := nextPressureState(PressureCritical, 0.79, cfg); got != PressureElevated {
		t.Fatalf("critical recovery to elevated -> %s, want elevated", got)
	}
	if got := nextPressureState(PressureHigh, 0.66, cfg); got != PressureHigh {
		t.Fatalf("high hysteresis hold -> %s, want high", got)
	}
	if got := nextPressureState(PressureHigh, 0.64, cfg); got != PressureNormal {
		t.Fatalf("high exit -> %s, want normal", got)
	}
	if got := nextPressureState(PressureElevated, 0.51, cfg); got != PressureElevated {
		t.Fatalf("elevated hysteresis hold -> %s, want elevated", got)
	}
	if got := nextPressureState(PressureElevated, 0.49, cfg); got != PressureNormal {
		t.Fatalf("elevated exit -> %s, want normal", got)
	}
}
