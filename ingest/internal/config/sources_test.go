package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSourceCatalogAppliesDefaults(t *testing.T) {
	baseDir := t.TempDir()
	path := filepath.Join(baseDir, "sources.json")

	content := `{
  "sources": [
    {
      "name": "temp_dev",
      "event_type": "temperature",
      "mode": "simulator",
      "enabled": true,
      "validation_file": "config/validation/temperature.json",
      "ml_enabled": true,
      "simulator": {
        "interval": "100ms",
        "seed": 42,
        "profile": "temperature_default"
      }
    },
    {
      "name": "temp_plc",
      "event_type": "temperature",
      "mode": "modbus",
      "enabled": true,
      "validation_file": "config/validation/temperature.json",
      "modbus": {
        "address": "127.0.0.1:1502",
        "mw_base": 1024,
        "register_count": 6
      }
    }
  ]
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	catalog, err := LoadSourceCatalog(path)
	if err != nil {
		t.Fatalf("LoadSourceCatalog() error = %v", err)
	}

	if got := catalog.Sources[0].SQLTarget; got != "temp_samples" {
		t.Fatalf("temperature SQLTarget = %q, want %q", got, "temp_samples")
	}
	if got := catalog.Sources[1].Modbus.SlaveID; got != 1 {
		t.Fatalf("modbus SlaveID = %d, want 1", got)
	}
	if got := catalog.Runtime.Buffering.SharedUnits; got != defaultSharedUnits {
		t.Fatalf("runtime buffering shared units = %d, want %d", got, defaultSharedUnits)
	}
	if got := catalog.Sources[0].FlowControl.ReservedUnits; got != defaultReservedUnits {
		t.Fatalf("source reserved units = %d, want %d", got, defaultReservedUnits)
	}
	if got := catalog.Sources[1].FlowControl.SupportsBackpressureValue(catalog.Sources[1].Mode); !got {
		t.Fatalf("modbus source supports backpressure = false, want true")
	}
}

func TestLoadSourceCatalogReadsCurrentBufferFields(t *testing.T) {
	baseDir := t.TempDir()
	path := filepath.Join(baseDir, "sources.json")

	content := `{
  "runtime": {
    "buffering": {
      "shared_units": 99,
      "sampling_shared_threshold": 0.6,
      "pressure": {
        "elevated_enter": 0.7,
        "elevated_exit": 0.5,
        "high_enter": 0.85,
        "high_exit": 0.65,
        "critical_enter": 0.95,
        "critical_exit": 0.8
      }
    }
  },
  "sources": [
    {
      "name": "temp_dev",
      "event_type": "temperature",
      "mode": "simulator",
      "enabled": true,
      "validation_file": "config/validation/temperature.json",
      "flow_control": {
        "reserved_units": 7,
        "overload_policy": "drop_oldest",
        "sampling_every_n": 2
      },
      "simulator": {
        "interval": "100ms",
        "seed": 42,
        "profile": "temperature_default"
      }
    }
  ]
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	catalog, err := LoadSourceCatalog(path)
	if err != nil {
		t.Fatalf("LoadSourceCatalog() error = %v", err)
	}

	if got := catalog.Runtime.Buffering.SharedUnits; got != 99 {
		t.Fatalf("runtime buffering shared units = %d, want 99", got)
	}
	if got := catalog.Runtime.Buffering.SamplingSharedThreshold; got != 0.6 {
		t.Fatalf("runtime buffering sampling threshold = %v, want 0.6", got)
	}
	if got := catalog.Sources[0].FlowControl.ReservedUnits; got != 7 {
		t.Fatalf("source reserved units = %d, want 7", got)
	}
}

func TestLoadSourceCatalogRejectsDuplicateNames(t *testing.T) {
	baseDir := t.TempDir()
	path := filepath.Join(baseDir, "sources.json")

	content := `{
  "sources": [
    {
      "name": "dup",
      "event_type": "temperature",
      "mode": "simulator",
      "enabled": true,
      "validation_file": "config/validation/temperature.json",
      "simulator": {
        "interval": "100ms",
        "seed": 42,
        "profile": "temperature_default"
      }
    },
    {
      "name": "dup",
      "event_type": "temperature",
      "mode": "simulator",
      "enabled": true,
      "validation_file": "config/validation/temperature.json",
      "simulator": {
        "interval": "100ms",
        "seed": 7,
        "profile": "temperature_default"
      }
    }
  ]
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	_, err := LoadSourceCatalog(path)
	if err == nil {
		t.Fatalf("LoadSourceCatalog() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `duplicate source name "dup"`) {
		t.Fatalf("LoadSourceCatalog() error = %v, want duplicate source name", err)
	}
}

func TestLoadSourceCatalogWithProfileOverridesEnabledSources(t *testing.T) {
	baseDir := t.TempDir()
	path := filepath.Join(baseDir, "sources.json")

	content := `{
  "profiles": {
    "modbus": {
      "enabled_sources": ["temp_plc"]
    }
  },
  "sources": [
    {
      "name": "temp_dev",
      "event_type": "temperature",
      "mode": "simulator",
      "enabled": true,
      "validation_file": "config/validation/temperature.json",
      "simulator": {
        "interval": "100ms",
        "seed": 42,
        "profile": "temperature_default"
      }
    },
    {
      "name": "temp_plc",
      "event_type": "temperature",
      "mode": "modbus",
      "enabled": false,
      "validation_file": "config/validation/temperature.json",
      "modbus": {
        "address": "127.0.0.1:1502",
        "mw_base": 1024,
        "register_count": 6
      }
    }
  ]
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	catalog, err := LoadSourceCatalogWithProfile(path, "modbus")
	if err != nil {
		t.Fatalf("LoadSourceCatalogWithProfile() error = %v", err)
	}

	if catalog.Sources[0].Enabled {
		t.Fatalf("temp_dev enabled = true, want false under modbus profile")
	}
	if !catalog.Sources[1].Enabled {
		t.Fatalf("temp_plc enabled = false, want true under modbus profile")
	}
}

func TestLoadSourceCatalogWithProfileRejectsUnknownSource(t *testing.T) {
	baseDir := t.TempDir()
	path := filepath.Join(baseDir, "sources.json")

	content := `{
  "profiles": {
    "modbus": {
      "enabled_sources": ["missing_service"]
    }
  },
  "sources": [
    {
      "name": "temp_dev",
      "event_type": "temperature",
      "mode": "simulator",
      "enabled": true,
      "validation_file": "config/validation/temperature.json",
      "simulator": {
        "interval": "100ms",
        "seed": 42,
        "profile": "temperature_default"
      }
    }
  ]
}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	_, err := LoadSourceCatalogWithProfile(path, "modbus")
	if err == nil {
		t.Fatalf("LoadSourceCatalogWithProfile() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `references unknown source "missing_service"`) {
		t.Fatalf("LoadSourceCatalogWithProfile() error = %v, want unknown source reference", err)
	}
}
