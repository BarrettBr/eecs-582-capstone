package runtime

/*
Name: ingest/internal/ingest/runtime/pressure.go
Description: Defines pressure states and hysteresis transitions for buffering decisions.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-13
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer pressure transition comments.
Preconditions:
- Pressure threshold config is validated before use.
Acceptable Input Values/Types:
- Current pressure state, occupancy ratio, and threshold config.
Unacceptable Input Values/Types:
- Invalid threshold ordering supplied from config validation bypasses.
Postconditions:
- Returns the next pressure state using hysteresis rules.
Return Values/Types:
- nextPressureState: PressureState
Error/Exception Conditions:
- None in this file directly.
Side Effects:
- None.
Invariants:
- Pressure transitions use the validated threshold ordering.
Known Faults:
- Pressure still only reflects buffer occupancy and not sink specific lag.
*/

import "github.com/BarrettBr/eecs-582-capstone/internal/config"

type PressureState string

const (
	PressureNormal   PressureState = "normal"
	PressureElevated PressureState = "elevated"
	PressureHigh     PressureState = "high"
	PressureCritical PressureState = "critical"
)

// description: Applies hysteresis rules to one occupancy ratio and current pressure state.
// input: current PressureState, occupancy ratio, and validated threshold config.
// output: Returns the next PressureState for the BufferManager.
func nextPressureState(current PressureState, occupancyRatio float64, cfg config.PressureThresholds) PressureState {
	switch current {
	case PressureCritical:
		if occupancyRatio >= cfg.CriticalExit {
			return PressureCritical
		}
		if occupancyRatio >= cfg.HighEnter {
			return PressureHigh
		}
		if occupancyRatio >= cfg.ElevatedEnter {
			return PressureElevated
		}
		return PressureNormal
	case PressureHigh:
		if occupancyRatio >= cfg.CriticalEnter {
			return PressureCritical
		}
		if occupancyRatio >= cfg.HighExit {
			return PressureHigh
		}
		if occupancyRatio >= cfg.ElevatedEnter {
			return PressureElevated
		}
		return PressureNormal
	case PressureElevated:
		if occupancyRatio >= cfg.CriticalEnter {
			return PressureCritical
		}
		if occupancyRatio >= cfg.HighEnter {
			return PressureHigh
		}
		if occupancyRatio >= cfg.ElevatedExit {
			return PressureElevated
		}
		return PressureNormal
	default:
		if occupancyRatio >= cfg.CriticalEnter {
			return PressureCritical
		}
		if occupancyRatio >= cfg.HighEnter {
			return PressureHigh
		}
		if occupancyRatio >= cfg.ElevatedEnter {
			return PressureElevated
		}
		return PressureNormal
	}
}
