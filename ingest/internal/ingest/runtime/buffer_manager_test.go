package runtime

import (
	"testing"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
	ingestevents "github.com/BarrettBr/eecs-582-capstone/internal/ingest/events"
)

func TestBufferManagerUsesReservedUnitsBeforeSharedUnits(t *testing.T) {
	manager := NewBufferManager(&Pipeline{}, config.BufferingConfig{
		SharedUnits:             1,
		SamplingSharedThreshold: 0.8,
		Pressure: config.PressureThresholds{
			ElevatedEnter: 0.70,
			ElevatedExit:  0.50,
			HighEnter:     0.85,
			HighExit:      0.65,
			CriticalEnter: 0.95,
			CriticalExit:  0.80,
		},
	}, 32)
	definition := testSourceDefinition("svc_a", true)

	_ = manager.Submit(t.Context(), definition, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 1}})
	_ = manager.Submit(t.Context(), definition, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 2}})

	manager.mu.Lock()
	service := manager.services["svc_a"]
	queueDepth := service.queueLen
	bufferedUnits := service.usedUnits
	sharedUsed := manager.sharedUsedUnits
	manager.mu.Unlock()

	if queueDepth != 2 {
		t.Fatalf("queue depth = %d, want 2", queueDepth)
	}
	if bufferedUnits != 2 {
		t.Fatalf("buffered units = %d, want 2", bufferedUnits)
	}
	if sharedUsed != 1 {
		t.Fatalf("shared used units = %d, want 1", sharedUsed)
	}
	if got := manager.Snapshot().Services["svc_a"].AdmittedEvents; got != 2 {
		t.Fatalf("admitted events = %d, want 2", got)
	}
}

func TestBufferManagerDropOldestStaysServiceLocal(t *testing.T) {
	manager := NewBufferManager(&Pipeline{}, config.BufferingConfig{
		SharedUnits:             1,
		SamplingSharedThreshold: 0.8,
		Pressure: config.PressureThresholds{
			ElevatedEnter: 0.70,
			ElevatedExit:  0.50,
			HighEnter:     0.85,
			HighExit:      0.65,
			CriticalEnter: 0.95,
			CriticalExit:  0.80,
		},
	}, 32)
	serviceA := testSourceDefinition("svc_a", true)
	serviceB := testSourceDefinition("svc_b", true)

	_ = manager.Submit(t.Context(), serviceA, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 1}})
	_ = manager.Submit(t.Context(), serviceB, IngressEvent{SourceName: "svc_b", Record: ingestevents.TempSample{Temperature: 2}})
	_ = manager.Submit(t.Context(), serviceB, IngressEvent{SourceName: "svc_b", Record: ingestevents.TempSample{Temperature: 3}})
	_ = manager.Submit(t.Context(), serviceA, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 4}})

	manager.mu.Lock()
	defer manager.mu.Unlock()

	if got := manager.services["svc_a"].evicted; got != 1 {
		t.Fatalf("svc_a evicted = %d, want 1", got)
	}
	if got := manager.services["svc_b"].evicted; got != 0 {
		t.Fatalf("svc_b evicted = %d, want 0", got)
	}
	if got := manager.services["svc_b"].queueLen; got != 2 {
		t.Fatalf("svc_b queue depth = %d, want 2", got)
	}
}

func TestBufferManagerPressureHysteresis(t *testing.T) {
	manager := NewBufferManager(&Pipeline{}, config.BufferingConfig{
		SharedUnits:             2,
		SamplingSharedThreshold: 0.8,
		Pressure: config.PressureThresholds{
			ElevatedEnter: 0.30,
			ElevatedExit:  0.25,
			HighEnter:     0.60,
			HighExit:      0.50,
			CriticalEnter: 0.95,
			CriticalExit:  0.80,
		},
	}, 32)
	definition := testSourceDefinition("svc_a", true)
	manager.UpsertService(definition)

	_ = manager.Submit(t.Context(), definition, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 1}})
	if got := manager.Snapshot().PressureState; got != PressureElevated {
		t.Fatalf("pressure after first submit = %s, want %s", got, PressureElevated)
	}

	_ = manager.Submit(t.Context(), definition, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 2}})
	if got := manager.Snapshot().PressureState; got != PressureHigh {
		t.Fatalf("pressure after second submit = %s, want %s", got, PressureHigh)
	}

	_, ok, _, _ := manager.dequeueNext()
	if !ok {
		t.Fatalf("dequeueNext() = false, want true")
	}
	if got := manager.Snapshot().PressureState; got != PressureElevated {
		t.Fatalf("pressure after one dequeue = %s, want %s", got, PressureElevated)
	}
}

func TestBufferManagerSamplingActivatesOnlyForBorrowingNonBackpressureService(t *testing.T) {
	manager := NewBufferManager(&Pipeline{}, config.BufferingConfig{
		SharedUnits:             2,
		SamplingSharedThreshold: 0.5,
		Pressure: config.PressureThresholds{
			ElevatedEnter: 0.20,
			ElevatedExit:  0.10,
			HighEnter:     0.30,
			HighExit:      0.20,
			CriticalEnter: 0.95,
			CriticalExit:  0.80,
		},
	}, 32)
	definition := testSourceDefinition("svc_a", false)
	definition.FlowControl.SamplingEveryN = 2

	_ = manager.Submit(t.Context(), definition, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 1}})
	_ = manager.Submit(t.Context(), definition, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 2}})
	_ = manager.Submit(t.Context(), definition, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 3}})

	snapshot := manager.Snapshot()
	if got := snapshot.PressureState; got != PressureHigh {
		t.Fatalf("pressure state = %s, want %s", got, PressureHigh)
	}
	if !snapshot.Services["svc_a"].Sampling {
		t.Fatalf("sampling = false, want true")
	}

	_ = manager.Submit(t.Context(), definition, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 4}})
	updated := manager.Snapshot()
	if updated.Services["svc_a"].SampledEvents == 0 {
		t.Fatalf("sampled events = 0, want > 0")
	}
}

func TestBufferManagerRoundRobinDequeueAcrossActiveServices(t *testing.T) {
	manager := NewBufferManager(&Pipeline{}, config.BufferingConfig{
		SharedUnits:             4,
		SamplingSharedThreshold: 0.8,
		Pressure: config.PressureThresholds{
			ElevatedEnter: 0.70,
			ElevatedExit:  0.50,
			HighEnter:     0.85,
			HighExit:      0.65,
			CriticalEnter: 0.95,
			CriticalExit:  0.80,
		},
	}, 32)
	serviceA := testSourceDefinition("svc_a", true)
	serviceB := testSourceDefinition("svc_b", true)

	_ = manager.Submit(t.Context(), serviceA, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 1}})
	_ = manager.Submit(t.Context(), serviceA, IngressEvent{SourceName: "svc_a", Record: ingestevents.TempSample{Temperature: 2}})
	_ = manager.Submit(t.Context(), serviceB, IngressEvent{SourceName: "svc_b", Record: ingestevents.TempSample{Temperature: 3}})

	first, ok, _, _ := manager.dequeueNext()
	if !ok {
		t.Fatalf("first dequeueNext() = false, want true")
	}
	second, ok, _, _ := manager.dequeueNext()
	if !ok {
		t.Fatalf("second dequeueNext() = false, want true")
	}

	if first.event.SourceName != "svc_a" || second.event.SourceName != "svc_b" {
		t.Fatalf("dequeue order = [%s %s], want [svc_a svc_b]", first.event.SourceName, second.event.SourceName)
	}
}

func testSourceDefinition(name string, supportsBackpressure bool) config.SourceDefinition {
	return config.SourceDefinition{
		Name:      name,
		EventType: "temperature",
		Mode:      "simulator",
		Enabled:   true,
		FlowControl: config.FlowControlConfig{
			ReservedUnits:        1,
			AdaptiveEnabled:      boolPointer(true),
			SupportsBackpressure: boolPointer(supportsBackpressure),
			OverloadPolicy:       "drop_oldest",
			SamplingEveryN:       2,
			Backpressure: config.BackpressurePolicy{
				HighIntervalMultiplier:     2,
				CriticalIntervalMultiplier: 4,
			},
		},
		Simulator: &config.SimulatorSourceSettings{
			Interval: "100ms",
			Seed:     1,
			Profile:  "temperature_default",
		},
	}
}

func boolPointer(value bool) *bool {
	return &value
}
