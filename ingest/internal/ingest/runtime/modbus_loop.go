package runtime

/*
Name: ingest/internal/ingest/runtime/modbus_loop.go
Description: Normalizes simulator and Modbus source reads into typed ingest records and shared helper logic.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-03-14
Revision History:
- 2026-02-01, Barrett Brown: Added base Modbus ingest loop structure.
- 2026-02-13, Barrett Brown: Expanded normalization logic to better match PLC structured data.
- 2026-02-28, Barrett Brown: Generalized source record handling and validation-facing helpers.
- 2026-03-07, Barrett Brown: Refactored simulator and Modbus source normalization into the runtime package.
- 2026-03-13, Barrett Brown: Added clearer source normalization comments.
- 2026-03-14, Barrett Brown: Restored richer pre-refactor prologue detail from main branch history.
- 2026-03-14, Barrett Brown: Updated runtime normalization during the ingest file structure refactor to use the explicit events and validation packages.
Preconditions:
- Reachable Modbus TCP endpoints and valid source runner setup for each enabled source.
- Validation rules are loaded before records reach normalization and fanout.
Acceptable Input Values/Types:
- Simulator state and Modbus response bytes with the expected fixed layout sizes.
- Supported sensor code values recognized by the normalization helpers.
Unacceptable Input Values/Types:
- Incorrect register payload size or unsupported source modes or event types.
- Invalid per-source config that slips past startup validation.
Postconditions:
- Returns typed TempSample or ValveSample records for downstream use.
Return Values/Types:
- Source read and normalization helpers return RecordEvent values or errors.
Error/Exception Conditions:
- Modbus read failures, normalization failures, validation-facing record shape errors, and malformed register data.
Side Effects:
- Network I/O to Modbus endpoints and mutable simulator state updates.
Invariants:
- Register normalization follows the fixed sample layouts expected by the project.
- Record timestamps are always written in UTC-formatted values during normalization.
Known Faults:
- Simulator logic and register decoding remain in one file for now.
- Source setup still mixes multiple transport and normalization concerns in the same runtime area.
*/

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
	ingestevents "github.com/BarrettBr/eecs-582-capstone/internal/ingest/events"
	ingestvalidation "github.com/BarrettBr/eecs-582-capstone/internal/ingest/validation"
	"github.com/goburrow/modbus"
)

type sourceRunner struct {
	definition      config.SourceDefinition
	validator       *ingestvalidation.RuleEngine
	interval        time.Duration
	client          modbus.Client
	modbusHandler   *modbus.TCPClientHandler
	mu              sync.Mutex
	rng             *rand.Rand
	simTemp         float64
	simStep         uint16
	forceFaultTicks int
}

// description: Reads the next record using the configured source mode.
// input: None, uses the sourceRunner configuration and live state.
// output: Returns the next normalized RecordEvent or an error.
func (s *sourceRunner) nextRecord() (ingestevents.RecordEvent, error) {
	switch s.definition.Mode {
	case "simulator":
		return s.nextSimulatedRecord()
	case "modbus":
		return s.nextModbusRecord()
	default:
		return nil, fmt.Errorf("source %q unsupported mode %q", s.definition.Name, s.definition.Mode)
	}
}

// description: Produces one synthetic temperature record for local development flows.
// input: None, uses the sourceRunner simulation state and random source.
// output: Returns one TempSample or an error.
func (s *sourceRunner) nextSimulatedRecord() (ingestevents.RecordEvent, error) {
	if s.definition.EventType != "temperature" {
		return nil, fmt.Errorf("source %q simulator does not support event type %q", s.definition.Name, s.definition.EventType)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Move the simulated process with bounded drift and occasional spikes.
	s.simStep++
	drift := (s.rng.Float64() - 0.5) * 2.2
	s.simTemp += drift
	if s.simTemp < 60.0 {
		s.simTemp = 60.0
	}
	if s.simTemp > 82.0 {
		s.simTemp = 82.0
	}
	if s.simStep%24 == 0 {
		s.simTemp = 77.5 + s.rng.Float64()*2.5
	}
	if s.forceFaultTicks > 0 {
		// Force a clearly anomalous sample for a short window to exercise the full flow.
		s.simTemp = 88.0 + s.rng.Float64()*4.0
		s.forceFaultTicks--
	}

	fanOn := s.simTemp >= 72.0
	heaterPower := math.Max(0, math.Min(100, (72.0-s.simTemp)*12.5))
	if fanOn {
		heaterPower = math.Max(0, heaterPower-15.0)
	}

	return ingestevents.TempSample{
		ID:           s.simStep,
		Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
		SensorType:   "temperature_control_system",
		SensorNumber: 1,
		FanOn:        fanOn,
		Temperature:  roundToTenth(s.simTemp),
		HeaterPower:  roundToHundredth(heaterPower),
	}, nil
}

// description: Reads one Modbus register block and normalizes it by event type.
// input: None, uses the sourceRunner Modbus client and source definition.
// output: Returns one normalized RecordEvent or an error.
func (s *sourceRunner) nextModbusRecord() (ingestevents.RecordEvent, error) {
	// Read the configured register block, then dispatch to a type-specific normalizer.
	results, err := s.client.ReadHoldingRegisters(s.definition.Modbus.MWBase, s.definition.Modbus.RegisterCount)
	if err != nil {
		return nil, err
	}

	switch s.definition.EventType {
	case "temperature":
		return normalizeTemperatureRegisters(results)
	case "valve":
		return normalizeValveRegisters(results)
	default:
		return nil, fmt.Errorf("source %q unsupported event type %q", s.definition.Name, s.definition.EventType)
	}
}

// description: Applies anomaly labels to one typed record event.
// input: normalized RecordEvent and anomaly label slice.
// output: Returns a record copy with anomaly labels attached.
func applyAnomalies(record ingestevents.RecordEvent, anomalies []string) (ingestevents.RecordEvent, error) {
	switch sample := record.(type) {
	case ingestevents.TempSample:
		sample.Anomalies = append([]string(nil), anomalies...)
		return sample, nil
	case *ingestevents.TempSample:
		if sample == nil {
			return nil, fmt.Errorf("temperature record is nil")
		}
		sample.Anomalies = append([]string(nil), anomalies...)
		return sample, nil
	case ingestevents.ValveSample:
		sample.Anomalies = append([]string(nil), anomalies...)
		return sample, nil
	case *ingestevents.ValveSample:
		if sample == nil {
			return nil, fmt.Errorf("valve record is nil")
		}
		sample.Anomalies = append([]string(nil), anomalies...)
		return sample, nil
	default:
		return nil, fmt.Errorf("unsupported record type %T", record)
	}
}

// description: Converts the fixed temperature register layout into a TempSample.
// input: Modbus register bytes for the temperature layout.
// output: Returns one TempSample or an error for malformed data.
func normalizeTemperatureRegisters(results []byte) (ingestevents.TempSample, error) {
	// Preserve the existing 6-register temperature layout.
	if len(results) != 12 {
		return ingestevents.TempSample{}, fmt.Errorf("invalid size: got %d bytes", len(results))
	}

	sensorCode := binary.BigEndian.Uint16(results[2:4])
	sensorType, err := sensorTypeFromCode(sensorCode)
	if err != nil {
		return ingestevents.TempSample{}, err
	}

	now := time.Now().UTC()

	return ingestevents.TempSample{
		ID:           binary.BigEndian.Uint16(results[0:2]),
		Timestamp:    now.Format(time.RFC3339Nano),
		TimestampMs:  now.UnixMilli(),
		SensorType:   sensorType,
		SensorNumber: binary.BigEndian.Uint16(results[4:6]),
		FanOn:        binary.BigEndian.Uint16(results[6:8]) == 1,
		Temperature:  float64(int16(binary.BigEndian.Uint16(results[8:10]))) / 10.0,
		HeaterPower:  float64(binary.BigEndian.Uint16(results[10:12])) / 100.0,
	}, nil
}

// description: Converts the fixed valve register layout into a ValveSample.
// input: Modbus register bytes for the valve layout.
// output: Returns one ValveSample or an error for malformed data.
func normalizeValveRegisters(results []byte) (ingestevents.ValveSample, error) {
	// Decode the locked 5-register valve layout.
	if len(results) != 10 {
		return ingestevents.ValveSample{}, fmt.Errorf("invalid size: got %d bytes", len(results))
	}

	sensorCode := binary.BigEndian.Uint16(results[2:4])
	sensorType, err := valveSensorTypeFromCode(sensorCode)
	if err != nil {
		return ingestevents.ValveSample{}, err
	}

	now := time.Now().UTC()

	return ingestevents.ValveSample{
		ID:          binary.BigEndian.Uint16(results[0:2]),
		Timestamp:   now.Format(time.RFC3339Nano),
		TimestampMs: now.UnixMilli(),
		SensorType:  sensorType,
		ValveNumber: binary.BigEndian.Uint16(results[4:6]),
		IsOpen:      binary.BigEndian.Uint16(results[6:8]) == 1,
		FlowRate:    float64(binary.BigEndian.Uint16(results[8:10])) / 100.0,
	}, nil
}

func sensorTypeFromCode(code uint16) (string, error) {
	switch code {
	case 1:
		return "temperature_control_system", nil
	default:
		return "", fmt.Errorf("invalid sensor type: %d", code)
	}
}

func valveSensorTypeFromCode(code uint16) (string, error) {
	switch code {
	case 2:
		return "valve_control_system", nil
	default:
		return "", fmt.Errorf("invalid valve sensor type: %d", code)
	}
}

func roundToTenth(value float64) float64 {
	return math.Round(value*10) / 10
}

func roundToHundredth(value float64) float64 {
	return math.Round(value*100) / 100
}

func isContextCanceled(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}
