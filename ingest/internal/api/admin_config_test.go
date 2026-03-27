package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
	ingestvalidation "github.com/BarrettBr/eecs-582-capstone/internal/ingest/validation"
)

func TestRegisterRoutesAddsAdminConfigEndpoint(t *testing.T) {
	paths := writeAdminConfigFixture(t)

	mux := http.NewServeMux()
	registration := RegisterRoutes(muxRegistrar{mux: mux}, Config{
		BasePath:         "/api/v1",
		SourceConfigPath: paths.sourceConfigPath,
	})

	if registration.AdminConfigPath != "/api/v1/admin/config" {
		t.Fatalf("registration.AdminConfigPath = %q, want %q", registration.AdminConfigPath, "/api/v1/admin/config")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAdminConfigHandlerReturnsStructuredConfig(t *testing.T) {
	paths := writeAdminConfigFixture(t)

	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{
		BasePath:         "/api/v1",
		SourceConfigPath: paths.sourceConfigPath,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var doc adminConfigDocument
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if doc.SourceConfigPath != filepath.Clean(paths.sourceConfigPath) {
		t.Fatalf("doc.SourceConfigPath = %q, want %q", doc.SourceConfigPath, filepath.Clean(paths.sourceConfigPath))
	}
	if len(doc.Catalog.Sources) != 2 {
		t.Fatalf("len(doc.Catalog.Sources) = %d, want 2", len(doc.Catalog.Sources))
	}
	if _, ok := doc.ValidationSpecs[filepath.Clean(paths.tempValidationPath)]; !ok {
		t.Fatalf("validation specs missing %q", filepath.Clean(paths.tempValidationPath))
	}
	if _, ok := doc.ValidationSpecs[filepath.Clean(paths.valveValidationPath)]; !ok {
		t.Fatalf("validation specs missing %q", filepath.Clean(paths.valveValidationPath))
	}
}

func TestAdminConfigHandlerSavesCatalogAndValidationSpecs(t *testing.T) {
	paths := writeAdminConfigFixture(t)

	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{
		BasePath:         "/api/v1",
		SourceConfigPath: paths.sourceConfigPath,
	})

	updatedRulePath := filepath.Join(filepath.Dir(paths.tempValidationPath), "pressure.json")
	payload := adminConfigDocument{
		Catalog: config.SourceCatalog{
			Runtime: config.SourceRuntimeConfig{
				Buffering: config.BufferingConfig{
					SharedUnits:             300,
					SamplingSharedThreshold: 0.82,
					Pressure: config.PressureThresholds{
						ElevatedEnter: 0.70,
						ElevatedExit:  0.50,
						HighEnter:     0.86,
						HighExit:      0.66,
						CriticalEnter: 0.96,
						CriticalExit:  0.81,
					},
				},
			},
			Profiles: map[string]config.SourceProfile{
				"modbus": {
					EnabledSources: []string{"valve_plc"},
				},
			},
			Sources: []config.SourceDefinition{
				{
					Name:           "valve_plc",
					AliasName:      "Primary Valve PLC",
					EventType:      "valve",
					Mode:           "modbus",
					Enabled:        true,
					ValidationFile: updatedRulePath,
					SQLTarget:      "valve_samples",
					MLEnabled:      true,
					FlowControl: config.FlowControlConfig{
						ReservedUnits:  64,
						OverloadPolicy: "drop_oldest",
						SamplingEveryN: 4,
						Backpressure: config.BackpressurePolicy{
							HighIntervalMultiplier:     2,
							CriticalIntervalMultiplier: 4,
						},
					},
					Modbus: &config.ModbusSourceSettings{
						Address:       "127.0.0.1:1502",
						MWBase:        2200,
						RegisterCount: 5,
						SlaveID:       1,
					},
				},
			},
		},
		ValidationSpecs: map[string]ingestvalidation.ValidationSpec{
			updatedRulePath: {
				RequiredFields: []string{"id", "timestamp", "flow_rate"},
				Bounds: map[string]ingestvalidation.Bounds{
					"flow_rate": {
						Min: float64Ptr(0),
						Max: float64Ptr(900),
					},
				},
				AnomalyRules: []ingestvalidation.AnomalyRule{
					{
						Field: "flow_rate",
						Op:    ">",
						Value: 650,
						Label: "pressure_spike",
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	savedCatalog, err := config.LoadSourceCatalog(paths.sourceConfigPath)
	if err != nil {
		t.Fatalf("LoadSourceCatalog() error = %v", err)
	}
	if len(savedCatalog.Sources) != 1 || savedCatalog.Sources[0].Name != "valve_plc" {
		t.Fatalf("saved sources = %+v, want only valve_plc", savedCatalog.Sources)
	}
	if savedCatalog.Sources[0].AliasName != "Primary Valve PLC" {
		t.Fatalf("saved alias = %q, want %q", savedCatalog.Sources[0].AliasName, "Primary Valve PLC")
	}

	if _, err := os.Stat(paths.tempValidationPath); !os.IsNotExist(err) {
		t.Fatalf("temp validation file still exists, err=%v", err)
	}

	specContent, err := os.ReadFile(updatedRulePath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	spec, err := ingestvalidation.ParseValidationSpec(specContent)
	if err != nil {
		t.Fatalf("ParseValidationSpec() error = %v", err)
	}
	if len(spec.AnomalyRules) != 1 || spec.AnomalyRules[0].Label != "pressure_spike" {
		t.Fatalf("saved anomaly rules = %+v, want pressure_spike", spec.AnomalyRules)
	}
}

type adminConfigPaths struct {
	sourceConfigPath    string
	tempValidationPath  string
	valveValidationPath string
}

func writeAdminConfigFixture(t *testing.T) adminConfigPaths {
	t.Helper()

	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	validationDir := filepath.Join(configDir, "validation")
	if err := os.MkdirAll(validationDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	tempValidationPath := filepath.Join(validationDir, "temperature.json")
	valveValidationPath := filepath.Join(validationDir, "valve.json")
	writeTestJSON(t, tempValidationPath, ingestvalidation.ValidationSpec{
		RequiredFields: []string{"id", "timestamp", "temperature"},
		Bounds: map[string]ingestvalidation.Bounds{
			"temperature": {Min: float64Ptr(-100), Max: float64Ptr(200)},
		},
		AnomalyRules: []ingestvalidation.AnomalyRule{
			{Field: "temperature", Op: ">", Value: 80, Label: "temperature_high"},
		},
	})
	writeTestJSON(t, valveValidationPath, ingestvalidation.ValidationSpec{
		RequiredFields: []string{"id", "timestamp", "flow_rate"},
		Bounds: map[string]ingestvalidation.Bounds{
			"flow_rate": {Min: float64Ptr(0), Max: float64Ptr(1000)},
		},
		AnomalyRules: []ingestvalidation.AnomalyRule{
			{Field: "flow_rate", Op: ">", Value: 500, Label: "valve_flow_high"},
		},
	})

	sourceConfigPath := filepath.Join(configDir, "sources.json")
	writeTestJSON(t, sourceConfigPath, config.SourceCatalog{
		Runtime: config.SourceRuntimeConfig{
			Buffering: config.BufferingConfig{
				SharedUnits:             256,
				SamplingSharedThreshold: 0.8,
				Pressure: config.PressureThresholds{
					ElevatedEnter: 0.7,
					ElevatedExit:  0.5,
					HighEnter:     0.85,
					HighExit:      0.65,
					CriticalEnter: 0.95,
					CriticalExit:  0.8,
				},
			},
		},
		Profiles: map[string]config.SourceProfile{
			"modbus": {
				EnabledSources: []string{"temp_plc", "valve_plc"},
			},
		},
		Sources: []config.SourceDefinition{
			{
				Name:           "temp_dev",
				AliasName:      "Temperature Simulator",
				EventType:      "temperature",
				Mode:           "simulator",
				Enabled:        true,
				ValidationFile: tempValidationPath,
				SQLTarget:      "temp_samples",
				MLEnabled:      true,
				FlowControl: config.FlowControlConfig{
					ReservedUnits:  64,
					OverloadPolicy: "drop_oldest",
					SamplingEveryN: 4,
					Backpressure: config.BackpressurePolicy{
						HighIntervalMultiplier:     2,
						CriticalIntervalMultiplier: 4,
					},
				},
				Simulator: &config.SimulatorSourceSettings{
					Interval: "100ms",
					Seed:     42,
					Profile:  "temperature_default",
				},
			},
			{
				Name:           "valve_dev",
				AliasName:      "Valve Simulator",
				EventType:      "valve",
				Mode:           "simulator",
				Enabled:        true,
				ValidationFile: valveValidationPath,
				SQLTarget:      "valve_samples",
				MLEnabled:      true,
				FlowControl: config.FlowControlConfig{
					ReservedUnits:  64,
					OverloadPolicy: "drop_oldest",
					SamplingEveryN: 4,
					Backpressure: config.BackpressurePolicy{
						HighIntervalMultiplier:     2,
						CriticalIntervalMultiplier: 4,
					},
				},
				Simulator: &config.SimulatorSourceSettings{
					Interval: "100ms",
					Seed:     84,
					Profile:  "valve_default",
				},
			},
		},
	})

	return adminConfigPaths{
		sourceConfigPath:    sourceConfigPath,
		tempValidationPath:  tempValidationPath,
		valveValidationPath: valveValidationPath,
	}
}

func writeTestJSON(t *testing.T, path string, payload any) {
	t.Helper()

	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
}

func float64Ptr(value float64) *float64 {
	return &value
}
