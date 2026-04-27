package config

/*
Name: ingest/internal/config/sources.go
Description: Loads source catalog JSON, applies defaults, and validates source and buffering settings.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-14
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer source catalog load and validation comments.
- 2026-03-14, Barrett Brown: Added profile-based source enablement overrides so simulator and modbus presets can share one catalog file.
Preconditions:
- Source catalog JSON file exists and matches the expected schema closely enough to parse.
Acceptable Input Values/Types:
- Source entries for simulator and modbus services.
- Buffering config with shared units, thresholds, and per source flow control.
Unacceptable Input Values/Types:
- Duplicate source names, invalid thresholds, or unsupported overload policies.
Postconditions:
- Returns a validated SourceCatalog with defaults applied.
Return Values/Types:
- LoadSourceCatalog: (*SourceCatalog, error)
- Validation helpers return nil or descriptive errors.
Error/Exception Conditions:
- File read failures, JSON parse failures, and validation errors.
Side Effects:
- Reads the source config file from disk.
Invariants:
- Source names stay unique within one catalog.
- Defaults are applied before validation checks run.
Known Faults:
- Source catalog validation still happens entirely at startup or reload time.
*/

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultSharedUnits                = 256
	defaultSamplingSharedThreshold    = 0.80
	defaultSimulatorMachineCount      = 3
	defaultReservedUnits              = 64
	defaultSamplingEveryN             = 4
	defaultHighIntervalMultiplier     = 2
	defaultCriticalIntervalMultiplier = 4
)

type SourceCatalog struct {
	Runtime  SourceRuntimeConfig      `json:"runtime"`
	Profiles map[string]SourceProfile `json:"profiles,omitempty"`
	Sources  []SourceDefinition       `json:"sources"`
}

type SourceRuntimeConfig struct {
	Buffering BufferingConfig `json:"buffering"`
}

type SourceProfile struct {
	EnabledSources []string `json:"enabled_sources"`
}

type BufferingConfig struct {
	SharedUnits             int                `json:"shared_units"`
	SamplingSharedThreshold float64            `json:"sampling_shared_threshold"`
	Pressure                PressureThresholds `json:"pressure"`
}

type PressureThresholds struct {
	ElevatedEnter float64 `json:"elevated_enter"`
	ElevatedExit  float64 `json:"elevated_exit"`
	HighEnter     float64 `json:"high_enter"`
	HighExit      float64 `json:"high_exit"`
	CriticalEnter float64 `json:"critical_enter"`
	CriticalExit  float64 `json:"critical_exit"`
}

// SourceDefinition describes one ingest source loaded at startup.
type SourceDefinition struct {
	Name           string                   `json:"name"`
	AliasName      string                   `json:"alias_name,omitempty"`
	EventType      string                   `json:"event_type"`
	Mode           string                   `json:"mode"`
	Enabled        bool                     `json:"enabled"`
	ValidationFile string                   `json:"validation_file"`
	SQLTarget      string                   `json:"sql_target"`
	MLEnabled      bool                     `json:"ml_enabled"`
	FlowControl    FlowControlConfig        `json:"flow_control"`
	Modbus         *ModbusSourceSettings    `json:"modbus,omitempty"`
	Simulator      *SimulatorSourceSettings `json:"simulator,omitempty"`
}

type FlowControlConfig struct {
	ReservedUnits        int                `json:"reserved_units"`
	AdaptiveEnabled      *bool              `json:"adaptive_enabled,omitempty"`
	SupportsBackpressure *bool              `json:"supports_backpressure,omitempty"`
	OverloadPolicy       string             `json:"overload_policy"`
	SamplingEveryN       int                `json:"sampling_every_n"`
	Backpressure         BackpressurePolicy `json:"backpressure"`
}

type BackpressurePolicy struct {
	HighIntervalMultiplier     int `json:"high_interval_multiplier"`
	CriticalIntervalMultiplier int `json:"critical_interval_multiplier"`
}

type ModbusSourceSettings struct {
	Address       string `json:"address"`
	MWBase        uint16 `json:"mw_base"`
	RegisterCount uint16 `json:"register_count"`
	SlaveID       byte   `json:"slave_id"`
	Interval      string `json:"interval,omitempty"`
}

type SimulatorSourceSettings struct {
	Interval     string `json:"interval"`
	Seed         int64  `json:"seed"`
	Profile      string `json:"profile"`
	MachineCount int    `json:"machine_count,omitempty"`
}

func (cfg FlowControlConfig) AdaptiveEnabledValue() bool {
	if cfg.AdaptiveEnabled == nil {
		return true
	}
	return *cfg.AdaptiveEnabled
}

func (cfg FlowControlConfig) SupportsBackpressureValue(mode string) bool {
	if cfg.SupportsBackpressure != nil {
		return *cfg.SupportsBackpressure
	}

	switch mode {
	case "modbus", "simulator":
		return true
	default:
		return false
	}
}

// description: Reads, defaults, and validates the source catalog JSON file.
// input: path to the source catalog file on disk.
// output: Returns a fully validated SourceCatalog or an error.
func LoadSourceCatalog(path string) (*SourceCatalog, error) {
	return LoadSourceCatalogWithProfile(path, "")
}

// description: Reads, defaults, validates, and optionally applies one named profile from the source catalog JSON file.
// input: path to the source catalog file on disk plus an optional profile name.
// output: Returns a fully validated SourceCatalog or an error.
func LoadSourceCatalogWithProfile(path string, profileName string) (*SourceCatalog, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read source config %s: %w", path, err)
	}

	return ParseSourceCatalogWithProfile(content, profileName)
}

// description: Parses, defaults, validates, and optionally applies one named profile from in-memory JSON.
// input: raw source catalog JSON bytes plus an optional profile name.
// output: Returns a fully validated SourceCatalog or an error.
func ParseSourceCatalogWithProfile(content []byte, profileName string) (*SourceCatalog, error) {
	var catalog SourceCatalog
	if err := json.Unmarshal(content, &catalog); err != nil {
		return nil, fmt.Errorf("parse source config: %w", err)
	}
	if err := ValidateSourceCatalogWithProfile(&catalog, profileName); err != nil {
		return nil, err
	}
	return &catalog, nil
}

// description: Applies defaults and validates one parsed source catalog in place.
// input: parsed SourceCatalog plus an optional profile name.
// output: Returns nil when the catalog is ready for use.
func ValidateSourceCatalogWithProfile(catalog *SourceCatalog, profileName string) error {
	if catalog == nil {
		return fmt.Errorf("source catalog is required")
	}
	applyCatalogDefaults(catalog)
	if len(catalog.Sources) == 0 {
		return fmt.Errorf("source config defines no sources")
	}
	if err := validateBufferingConfig(catalog.Runtime.Buffering); err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(catalog.Sources))
	for i := range catalog.Sources {
		if err := validateSourceDefinition(&catalog.Sources[i], seen); err != nil {
			return err
		}
	}
	if err := applyCatalogProfile(catalog, profileName, seen); err != nil {
		return err
	}

	return nil
}

// description: Fills in default buffering and per source flow control values.
// input: pointer to a parsed SourceCatalog.
// output: Mutates the catalog in place to include default values.
func applyCatalogDefaults(catalog *SourceCatalog) {
	if catalog.Runtime.Buffering.SharedUnits <= 0 {
		catalog.Runtime.Buffering.SharedUnits = defaultSharedUnits
	}
	if catalog.Runtime.Buffering.SamplingSharedThreshold <= 0 {
		catalog.Runtime.Buffering.SamplingSharedThreshold = defaultSamplingSharedThreshold
	}

	pressure := &catalog.Runtime.Buffering.Pressure
	if pressure.ElevatedEnter <= 0 {
		pressure.ElevatedEnter = 0.70
	}
	if pressure.ElevatedExit <= 0 {
		pressure.ElevatedExit = 0.50
	}
	if pressure.HighEnter <= 0 {
		pressure.HighEnter = 0.85
	}
	if pressure.HighExit <= 0 {
		pressure.HighExit = 0.65
	}
	if pressure.CriticalEnter <= 0 {
		pressure.CriticalEnter = 0.95
	}
	if pressure.CriticalExit <= 0 {
		pressure.CriticalExit = 0.80
	}

	for i := range catalog.Sources {
		flow := &catalog.Sources[i].FlowControl
		if flow.ReservedUnits <= 0 {
			flow.ReservedUnits = defaultReservedUnits
		}
		if flow.AdaptiveEnabled == nil {
			flow.AdaptiveEnabled = boolPtr(true)
		}
		if flow.SupportsBackpressure == nil {
			flow.SupportsBackpressure = boolPtr(flow.SupportsBackpressureValue(catalog.Sources[i].Mode))
		}
		if flow.OverloadPolicy == "" {
			flow.OverloadPolicy = "drop_oldest"
		}
		if flow.SamplingEveryN <= 0 {
			flow.SamplingEveryN = defaultSamplingEveryN
		}
		if flow.Backpressure.HighIntervalMultiplier <= 0 {
			flow.Backpressure.HighIntervalMultiplier = defaultHighIntervalMultiplier
		}
		if flow.Backpressure.CriticalIntervalMultiplier <= 0 {
			flow.Backpressure.CriticalIntervalMultiplier = defaultCriticalIntervalMultiplier
		}
		if catalog.Sources[i].Simulator != nil && catalog.Sources[i].Simulator.MachineCount == 0 {
			catalog.Sources[i].Simulator.MachineCount = defaultSimulatorMachineCount
		}
	}
}

func applyCatalogProfile(catalog *SourceCatalog, profileName string, seen map[string]struct{}) error {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return nil
	}

	profile, ok := catalog.Profiles[profileName]
	if !ok {
		return fmt.Errorf("source config profile %q not found", profileName)
	}

	enabled := make(map[string]struct{}, len(profile.EnabledSources))
	for _, name := range profile.EnabledSources {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; !exists {
			return fmt.Errorf("source config profile %q references unknown source %q", profileName, trimmed)
		}
		enabled[trimmed] = struct{}{}
	}

	for i := range catalog.Sources {
		_, shouldEnable := enabled[catalog.Sources[i].Name]
		catalog.Sources[i].Enabled = shouldEnable
	}

	return nil
}

// description: Checks top level buffering limits and hysteresis threshold ordering.
// input: BufferingConfig from the parsed source catalog.
// output: Returns nil when the buffering config is valid.
func validateBufferingConfig(cfg BufferingConfig) error {
	if cfg.SharedUnits <= 0 {
		return fmt.Errorf("runtime buffering shared_units must be > 0")
	}
	if err := validateUnitFraction("runtime buffering sampling_shared_threshold", cfg.SamplingSharedThreshold); err != nil {
		return err
	}
	if err := validateUnitFraction("runtime buffering pressure elevated_enter", cfg.Pressure.ElevatedEnter); err != nil {
		return err
	}
	if err := validateUnitFraction("runtime buffering pressure elevated_exit", cfg.Pressure.ElevatedExit); err != nil {
		return err
	}
	if err := validateUnitFraction("runtime buffering pressure high_enter", cfg.Pressure.HighEnter); err != nil {
		return err
	}
	if err := validateUnitFraction("runtime buffering pressure high_exit", cfg.Pressure.HighExit); err != nil {
		return err
	}
	if err := validateUnitFraction("runtime buffering pressure critical_enter", cfg.Pressure.CriticalEnter); err != nil {
		return err
	}
	if err := validateUnitFraction("runtime buffering pressure critical_exit", cfg.Pressure.CriticalExit); err != nil {
		return err
	}
	if !(cfg.Pressure.ElevatedExit < cfg.Pressure.ElevatedEnter &&
		cfg.Pressure.ElevatedEnter < cfg.Pressure.HighEnter &&
		cfg.Pressure.HighExit < cfg.Pressure.HighEnter &&
		cfg.Pressure.HighEnter < cfg.Pressure.CriticalEnter &&
		cfg.Pressure.CriticalExit < cfg.Pressure.CriticalEnter) {
		return fmt.Errorf("runtime buffering pressure thresholds must be ordered with exit < enter for each state")
	}
	return nil
}

// description: Validates one source definition after defaults are applied.
// input: source definition pointer and the shared map of already seen names.
// output: Returns nil when the source is valid and unique.
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

	if source.FlowControl.ReservedUnits <= 0 {
		return fmt.Errorf("source %q flow_control reserved_units must be > 0", source.Name)
	}
	if source.FlowControl.OverloadPolicy != "drop_oldest" {
		return fmt.Errorf("source %q unsupported overload_policy %q", source.Name, source.FlowControl.OverloadPolicy)
	}
	if source.FlowControl.SamplingEveryN <= 0 {
		return fmt.Errorf("source %q flow_control sampling_every_n must be > 0", source.Name)
	}
	if source.FlowControl.Backpressure.HighIntervalMultiplier <= 0 {
		return fmt.Errorf("source %q flow_control backpressure high_interval_multiplier must be > 0", source.Name)
	}
	if source.FlowControl.Backpressure.CriticalIntervalMultiplier < source.FlowControl.Backpressure.HighIntervalMultiplier {
		return fmt.Errorf("source %q flow_control critical_interval_multiplier must be >= high_interval_multiplier", source.Name)
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
		if source.Modbus.Interval != "" {
			if _, err := time.ParseDuration(source.Modbus.Interval); err != nil {
				return fmt.Errorf("source %q invalid modbus interval %q: %w", source.Name, source.Modbus.Interval, err)
			}
		}
	case "simulator":
		if source.Simulator == nil {
			return fmt.Errorf("source %q missing simulator settings", source.Name)
		}
		if _, err := time.ParseDuration(source.Simulator.Interval); err != nil {
			return fmt.Errorf("source %q invalid simulator interval %q: %w", source.Name, source.Simulator.Interval, err)
		}
		if source.Simulator.MachineCount <= 0 {
			return fmt.Errorf("source %q simulator machine_count must be > 0", source.Name)
		}
	default:
		return fmt.Errorf("source %q unsupported mode %q", source.Name, source.Mode)
	}

	return nil
}

func validateUnitFraction(name string, value float64) error {
	if value <= 0 || value >= 1 {
		return fmt.Errorf("%s must be between 0 and 1", name)
	}
	return nil
}

func boolPtr(value bool) *bool {
	return &value
}
