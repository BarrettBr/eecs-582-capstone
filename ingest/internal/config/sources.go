package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type SourceCatalog struct {
	Sources []SourceDefinition `json:"sources"`
}

// SourceDefinition describes one ingest source loaded at startup.
type SourceDefinition struct {
	Name           string                   `json:"name"`
	EventType      string                   `json:"event_type"`
	Mode           string                   `json:"mode"`
	Enabled        bool                     `json:"enabled"`
	ValidationFile string                   `json:"validation_file"`
	SQLTarget      string                   `json:"sql_target"`
	MLEnabled      bool                     `json:"ml_enabled"`
	Modbus         *ModbusSourceSettings    `json:"modbus,omitempty"`
	Simulator      *SimulatorSourceSettings `json:"simulator,omitempty"`
}

type ModbusSourceSettings struct {
	Address       string `json:"address"`
	MWBase        uint16 `json:"mw_base"`
	RegisterCount uint16 `json:"register_count"`
	SlaveID       byte   `json:"slave_id"`
}

type SimulatorSourceSettings struct {
	Interval string `json:"interval"`
	Seed     int64  `json:"seed"`
	Profile  string `json:"profile"`
}

func LoadSourceCatalog(path string) (*SourceCatalog, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read source config %s: %w", path, err)
	}

	var catalog SourceCatalog
	if err := json.Unmarshal(content, &catalog); err != nil {
		return nil, fmt.Errorf("parse source config %s: %w", path, err)
	}
	if len(catalog.Sources) == 0 {
		return nil, fmt.Errorf("source config %s defines no sources", path)
	}

	seen := make(map[string]struct{}, len(catalog.Sources))
	for i := range catalog.Sources {
		if err := validateSourceDefinition(&catalog.Sources[i], seen); err != nil {
			return nil, err
		}
	}

	return &catalog, nil
}

func validateSourceDefinition(source *SourceDefinition, seen map[string]struct{}) error {
	if source.Name == "" {
		return fmt.Errorf("source name is required")
	}
	if _, exists := seen[source.Name]; exists {
		return fmt.Errorf("duplicate source name %q", source.Name)
	}
	seen[source.Name] = struct{}{}

	if source.ValidationFile == "" {
		return fmt.Errorf("source %q missing validation_file", source.Name)
	}
	switch source.EventType {
	case "temperature":
		if source.SQLTarget == "" {
			source.SQLTarget = "temp_samples"
		}
		if source.SQLTarget != "temp_samples" {
			return fmt.Errorf("source %q invalid sql_target %q for temperature", source.Name, source.SQLTarget)
		}
	case "valve":
		if source.SQLTarget == "" {
			source.SQLTarget = "valve_samples"
		}
		if source.SQLTarget != "valve_samples" {
			return fmt.Errorf("source %q invalid sql_target %q for valve", source.Name, source.SQLTarget)
		}
	default:
		return fmt.Errorf("source %q unsupported event_type %q", source.Name, source.EventType)
	}

	switch source.Mode {
	case "modbus":
		if source.Modbus == nil {
			return fmt.Errorf("source %q missing modbus settings", source.Name)
		}
		if source.Modbus.Address == "" {
			return fmt.Errorf("source %q missing modbus address", source.Name)
		}
		if source.Modbus.RegisterCount == 0 {
			return fmt.Errorf("source %q modbus register_count must be > 0", source.Name)
		}
		if source.Modbus.SlaveID == 0 {
			source.Modbus.SlaveID = 1
		}
	case "simulator":
		if source.Simulator == nil {
			return fmt.Errorf("source %q missing simulator settings", source.Name)
		}
		if _, err := time.ParseDuration(source.Simulator.Interval); err != nil {
			return fmt.Errorf("source %q invalid simulator interval %q: %w", source.Name, source.Simulator.Interval, err)
		}
	default:
		return fmt.Errorf("source %q unsupported mode %q", source.Name, source.Mode)
	}

	return nil
}
