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
