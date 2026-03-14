package runtime

/*
Name: ingest/internal/ingest/runtime/registrar_test.go
Description: Tests registrar catalog apply, hot reload, and catalog callback behavior.
Programmer: Barrett Brown
Date Created: 2026-03-14
Dates Revised: 2026-03-14
Revision History:
- 2026-03-14, Barrett Brown: Added registrar coverage for apply, reload, and service catalog callback behavior.
Preconditions:
- Test source definitions point to valid validation files.
Acceptable Input Values/Types:
- Valid source catalogs, buffering configs, and temporary source files.
Unacceptable Input Values/Types:
- No special invalid runtime dependencies are required beyond malformed test catalogs.
Postconditions:
- Confirms registrar service lifecycle and callback behavior stay aligned with source catalog changes.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Unexpected missing services, reloads, or callback payloads fail the tests.
Side Effects:
- Starts local registrar goroutines and writes temporary source config files.
Invariants:
- Registrar snapshots should reflect the currently applied catalog.
Known Faults:
- Does not exercise every possible reload error branch.
*/

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
)

func TestRegistrarApplyCatalogAddsAndRemovesServices(t *testing.T) {
	validationPath := mustValidationPath(t, "temperature.json")
	bufferManager := NewBufferManager(&Pipeline{}, config.BufferingConfig{
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
	initialCatalog := &config.SourceCatalog{
		Runtime: config.SourceRuntimeConfig{Buffering: bufferManager.runtime},
		Sources: []config.SourceDefinition{
			testRegistrarSource("svc_a", validationPath),
		},
	}
	updatedCatalog := &config.SourceCatalog{
		Runtime: config.SourceRuntimeConfig{Buffering: bufferManager.runtime},
		Sources: []config.SourceDefinition{
			func() config.SourceDefinition {
				source := testRegistrarSource("svc_a", validationPath)
				source.Enabled = false
				return source
			}(),
			testRegistrarSource("svc_b", validationPath),
		},
	}

	registrar, err := NewRegistrar(RegistrarConfig{
		SourceConfigPath:      "config/sources.json",
		DefaultModbusInterval: 5 * time.Millisecond,
		InitialCatalog:        initialCatalog,
		BufferManager:         bufferManager,
	})
	if err != nil {
		t.Fatalf("NewRegistrar() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := registrar.applyCatalog(ctx, initialCatalog, true); err != nil {
		t.Fatalf("applyCatalog(initial) error = %v", err)
	}
	if _, ok := registrar.services["svc_a"]; !ok {
		t.Fatalf("svc_a missing after initial apply")
	}

	if err := registrar.applyCatalog(ctx, updatedCatalog, false); err != nil {
		t.Fatalf("applyCatalog(updated) error = %v", err)
	}
	if _, ok := registrar.services["svc_a"]; ok {
		t.Fatalf("svc_a still present after removal")
	}
	if _, ok := registrar.services["svc_b"]; !ok {
		t.Fatalf("svc_b missing after add")
	}
}

func TestRegistrarApplyCatalogNotifiesCatalogCallbackWithRemovedServices(t *testing.T) {
	validationPath := mustValidationPath(t, "temperature.json")
	bufferManager := NewBufferManager(&Pipeline{}, config.BufferingConfig{
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
	initialCatalog := &config.SourceCatalog{
		Runtime: config.SourceRuntimeConfig{Buffering: bufferManager.runtime},
		Sources: []config.SourceDefinition{
			testRegistrarSource("svc_a", validationPath),
		},
	}
	updatedCatalog := &config.SourceCatalog{
		Runtime: config.SourceRuntimeConfig{Buffering: bufferManager.runtime},
		Sources: []config.SourceDefinition{
			testRegistrarSource("svc_b", validationPath),
		},
	}

	type callbackState struct {
		snapshot SystemStatusSnapshot
		removed  []string
	}
	callbacks := make(chan callbackState, 2)
	registrar, err := NewRegistrar(RegistrarConfig{
		SourceConfigPath:      "config/sources.json",
		DefaultModbusInterval: 5 * time.Millisecond,
		InitialCatalog:        initialCatalog,
		BufferManager:         bufferManager,
		OnCatalogApplied: func(snapshot SystemStatusSnapshot, removed []string) {
			callbacks <- callbackState{
				snapshot: snapshot,
				removed:  append([]string(nil), removed...),
			}
		},
	})
	if err != nil {
		t.Fatalf("NewRegistrar() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := registrar.applyCatalog(ctx, initialCatalog, true); err != nil {
		t.Fatalf("applyCatalog(initial) error = %v", err)
	}
	<-callbacks

	if err := registrar.applyCatalog(ctx, updatedCatalog, false); err != nil {
		t.Fatalf("applyCatalog(updated) error = %v", err)
	}

	state := <-callbacks
	if len(state.removed) != 1 || state.removed[0] != "svc_a" {
		t.Fatalf("removed = %v, want [svc_a]", state.removed)
	}
	if len(state.snapshot.Services) != 1 || state.snapshot.Services[0].Name != "svc_b" {
		t.Fatalf("snapshot.Services = %+v, want only svc_b", state.snapshot.Services)
	}
	if state.snapshot.LastReloadAt == "" {
		t.Fatalf("snapshot.LastReloadAt = empty, want populated reload timestamp")
	}
}

func TestRegistrarRunHotReloadsSourceFile(t *testing.T) {
	validationPath := mustValidationPath(t, "temperature.json")
	sourcePath := filepath.Join(t.TempDir(), "sources.json")
	bufferingCfg := config.BufferingConfig{
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
	}

	initialCatalog := &config.SourceCatalog{
		Runtime: config.SourceRuntimeConfig{Buffering: bufferingCfg},
		Sources: []config.SourceDefinition{
			testRegistrarSource("svc_a", validationPath),
		},
	}
	if err := writeSourceCatalog(sourcePath, initialCatalog); err != nil {
		t.Fatalf("writeSourceCatalog(initial) error = %v", err)
	}

	loadedCatalog, err := config.LoadSourceCatalog(sourcePath)
	if err != nil {
		t.Fatalf("LoadSourceCatalog() error = %v", err)
	}

	bufferManager := NewBufferManager(&Pipeline{}, loadedCatalog.Runtime.Buffering, 32)
	registrar, err := NewRegistrar(RegistrarConfig{
		SourceConfigPath:      sourcePath,
		DefaultModbusInterval: 5 * time.Millisecond,
		InitialCatalog:        loadedCatalog,
		BufferManager:         bufferManager,
	})
	if err != nil {
		t.Fatalf("NewRegistrar() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runDone := make(chan error, 1)
	go func() {
		runDone <- registrar.Run(ctx)
	}()

	waitForStatus(t, 3*time.Second, registrar.StatusSnapshot, func(snapshot SystemStatusSnapshot) bool {
		return len(snapshot.Services) == 1 &&
			snapshot.Services[0].Name == "svc_a" &&
			snapshot.Services[0].CurrentInterval == 100*time.Millisecond
	})

	updatedA := testRegistrarSource("svc_a", validationPath)
	updatedA.Simulator.Interval = "250ms"
	updatedCatalog := &config.SourceCatalog{
		Runtime: config.SourceRuntimeConfig{Buffering: bufferingCfg},
		Sources: []config.SourceDefinition{
			updatedA,
			testRegistrarSource("svc_b", validationPath),
		},
	}
	if err := writeSourceCatalog(sourcePath, updatedCatalog); err != nil {
		t.Fatalf("writeSourceCatalog(updated) error = %v", err)
	}

	waitForStatus(t, 4*time.Second, registrar.StatusSnapshot, func(snapshot SystemStatusSnapshot) bool {
		if len(snapshot.Services) != 2 {
			return false
		}
		services := serviceStatusByName(snapshot)
		return services["svc_a"].CurrentInterval == 250*time.Millisecond &&
			services["svc_b"].LifecycleState == "running" &&
			snapshot.LastReloadAt != ""
	})

	removedCatalog := &config.SourceCatalog{
		Runtime: config.SourceRuntimeConfig{Buffering: bufferingCfg},
		Sources: []config.SourceDefinition{
			testRegistrarSource("svc_b", validationPath),
		},
	}
	if err := writeSourceCatalog(sourcePath, removedCatalog); err != nil {
		t.Fatalf("writeSourceCatalog(removed) error = %v", err)
	}

	waitForStatus(t, 4*time.Second, registrar.StatusSnapshot, func(snapshot SystemStatusSnapshot) bool {
		if len(snapshot.Services) != 1 {
			return false
		}
		return snapshot.Services[0].Name == "svc_b"
	})

	cancel()
	if err := <-runDone; err != nil && err != context.Canceled {
		t.Fatalf("registrar.Run() error = %v", err)
	}
}

func testRegistrarSource(name, validationPath string) config.SourceDefinition {
	return config.SourceDefinition{
		Name:           name,
		EventType:      "temperature",
		Mode:           "simulator",
		Enabled:        true,
		ValidationFile: validationPath,
		SQLTarget:      "temp_samples",
		FlowControl: config.FlowControlConfig{
			ReservedUnits:        4,
			AdaptiveEnabled:      boolPointer(true),
			SupportsBackpressure: boolPointer(true),
			OverloadPolicy:       "drop_oldest",
			SamplingEveryN:       4,
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

func writeSourceCatalog(path string, catalog *config.SourceCatalog) error {
	content, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func waitForStatus(t *testing.T, timeout time.Duration, provider func() SystemStatusSnapshot, predicate func(SystemStatusSnapshot) bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate(provider()) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	snapshot := provider()
	t.Fatalf("timed out waiting for registrar status, last snapshot = %+v", snapshot)
}

func serviceStatusByName(snapshot SystemStatusSnapshot) map[string]ServiceStatus {
	services := make(map[string]ServiceStatus, len(snapshot.Services))
	for _, service := range snapshot.Services {
		services[service.Name] = service
	}
	return services
}

func mustValidationPath(t *testing.T, name string) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	return filepath.Join(wd, "..", "..", "..", "config", "validation", name)
}
