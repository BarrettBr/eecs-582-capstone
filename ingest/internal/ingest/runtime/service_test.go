package runtime

/*
Name: ingest/internal/ingest/runtime/service_test.go
Description: Tests managed service pressure behavior, snapshots, and fault injection handling.
Programmer: Barrett Brown
Date Created: 2026-03-14
Dates Revised: 2026-03-14
Revision History:
- 2026-03-14, Barrett Brown: Added managed service behavior coverage tests.
- 2026-03-14, Barrett Brown: Added normalized fingerprint coverage for definition matching.
Preconditions:
- Test source definitions point to valid validation files.
Acceptable Input Values/Types:
- Simulator and modbus source definitions with valid flow control settings.
Unacceptable Input Values/Types:
- No special invalid runtime dependencies are required beyond unsupported mode combinations.
Postconditions:
- Confirms service pressure updates and fault injection rules behave as expected.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Unexpected interval or lifecycle values fail the tests.
Side Effects:
- Opens and closes runner resources during service construction.
Invariants:
- Backpressure support determines whether intervals change with pressure.
Known Faults:
- Does not run a full long lived service loop.
*/

import (
	"context"
	"testing"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
)

func TestManagedServiceSetPressureStateAdjustsIntervalForBackpressureService(t *testing.T) {
	service, err := newManagedService(testRegistrarSource("svc_backpressure", mustValidationPath(t, "temperature.json")), 5*time.Millisecond, noopSubmitter{})
	if err != nil {
		t.Fatalf("newManagedService() error = %v", err)
	}
	t.Cleanup(service.runner.close)

	base := service.baseInterval
	service.SetPressureState(PressureHigh)
	if got := service.interval(); got != base*2 {
		t.Fatalf("high interval = %s, want %s", got, base*2)
	}

	service.SetPressureState(PressureCritical)
	if got := service.interval(); got != base*4 {
		t.Fatalf("critical interval = %s, want %s", got, base*4)
	}

	service.SetPressureState(PressureNormal)
	if got := service.interval(); got != base {
		t.Fatalf("normal interval = %s, want %s", got, base)
	}
}

func TestManagedServiceSetPressureStateSkipsNonBackpressureService(t *testing.T) {
	definition := testRegistrarSource("svc_no_backpressure", mustValidationPath(t, "temperature.json"))
	definition.FlowControl.SupportsBackpressure = boolPointer(false)

	service, err := newManagedService(definition, 5*time.Millisecond, noopSubmitter{})
	if err != nil {
		t.Fatalf("newManagedService() error = %v", err)
	}
	t.Cleanup(service.runner.close)

	base := service.baseInterval
	service.SetPressureState(PressureCritical)
	if got := service.interval(); got != base {
		t.Fatalf("critical interval = %s, want unchanged %s", got, base)
	}
}

func TestManagedServiceTriggerFaultInjectionAndSnapshot(t *testing.T) {
	service, err := newManagedService(testRegistrarSource("svc_fault", mustValidationPath(t, "temperature.json")), 5*time.Millisecond, noopSubmitter{})
	if err != nil {
		t.Fatalf("newManagedService() error = %v", err)
	}
	t.Cleanup(service.runner.close)

	if err := service.TriggerFaultInjection(); err != nil {
		t.Fatalf("TriggerFaultInjection() error = %v", err)
	}
	if got := service.runner.forceFaultTicks; got != 10 {
		t.Fatalf("forceFaultTicks = %d, want 10", got)
	}

	service.recordError(assertErr("boom"))
	snapshot := service.snapshot()
	if snapshot.LastError != "boom" {
		t.Fatalf("snapshot.LastError = %q, want %q", snapshot.LastError, "boom")
	}
}

func TestManagedServiceTriggerFaultInjectionRejectsNonSimulator(t *testing.T) {
	service := &managedService{
		definition: config.SourceDefinition{
			Name: "svc_modbus",
			Mode: "modbus",
		},
	}

	if err := service.TriggerFaultInjection(); err == nil {
		t.Fatalf("TriggerFaultInjection() error = nil, want non-nil")
	}
}

func TestManagedServiceMatchesDefinitionUsesNormalizedFingerprint(t *testing.T) {
	definition := testRegistrarSource("svc_match", mustValidationPath(t, "temperature.json"))

	service, err := newManagedService(definition, 5*time.Millisecond, noopSubmitter{})
	if err != nil {
		t.Fatalf("newManagedService() error = %v", err)
	}
	t.Cleanup(service.runner.close)

	sameBehavior := definition
	sameBehavior.FlowControl.AdaptiveEnabled = nil
	sameBehavior.FlowControl.SupportsBackpressure = nil
	if !service.MatchesDefinition(sameBehavior) {
		t.Fatalf("MatchesDefinition() = false, want true for equivalent defaulted behavior")
	}

	changedInterval := definition
	changedInterval.Simulator = &config.SimulatorSourceSettings{
		Interval: "250ms",
		Seed:     definition.Simulator.Seed,
		Profile:  definition.Simulator.Profile,
	}
	if service.MatchesDefinition(changedInterval) {
		t.Fatalf("MatchesDefinition() = true, want false when simulator interval changes")
	}
}

type noopSubmitter struct{}

func (noopSubmitter) Submit(_ context.Context, _ config.SourceDefinition, _ IngressEvent) error {
	return nil
}

type staticError string

func (e staticError) Error() string { return string(e) }

func assertErr(msg string) error {
	return staticError(msg)
}
