package ingest

/*
Name: ingest/internal/ingest/modbus_loop.go
Description: Implements the polling loop that reads Modbus registers, normalizes/validates samples, and sends events into fan-out sinks.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-15
Revision History:
- 2026-02-01: Barrett Brown: Created base file structure
- 2026-02-13: Barrett Brown: Expanded / Made more in line with PLC structured data
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
Preconditions:
- Reachable Modbus TCP endpoint and valid validation rule file.
- Poll interval and register base are configured.
Acceptable Input Values/Types:
- Modbus responses as []byte of expected length (12 bytes for current mapping).
- Supported sensor code values recognized by sensorTypeFromCode.
- Context values that may be canceled for shutdown.
Unacceptable Input Values/Types:
- Incorrect register payload size or unsupported sensor type codes.
- Invalid validation specification/path at startup.
Postconditions:
- On each successful tick, one validated event is dispatched to fan-out channels.
- Loop terminates when context is canceled.
Return Values/Types:
- NewModbusLoop: *ModbusLoop
- Run/handleTick/dataNormalizer: error (nil on success)
- sensorTypeFromCode: (string, error)
Error/Exception Conditions:
- Modbus read errors, normalization errors, validation failures, and JSON encode errors.
- Startup failures for Modbus connect/rule load terminate process via log.Fatalf.
Side Effects:
- Network I/O to Modbus endpoint, logs emitted, mutable sample buffer updated, events enqueued.
Invariants:
- dataNormalizer expects exactly 12 bytes for current 6-register mapping.
- Sample timestamp is always written in UTC string format per tick.
Known Faults:
- Startup uses fatal exits instead of retry/backoff strategies.
*/

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	"github.com/goburrow/modbus"
)

// description: Polling loop that reads Modbus registers, validates samples, and dispatches them to sinks.
// input: Constructed with database queries, poll settings, Modbus settings, and validation rules.
// output: Produces validated TempSample events routed through the fan-out pipeline.
type ModbusLoop struct {
	client      modbus.Client
	queries     *database.Queries
	interval    time.Duration
	mwBase      uint16
	validator   *RuleEngine
	sample      TempSample
	jsonBuf     bytes.Buffer  // Writer that stores bytes for logging
	jsonEncoder *json.Encoder // Converts struct -> Json
	mlCh        chan fanoutEvent
	sqlCh       chan fanoutEvent
	wsCh        chan fanoutEvent
}

// description: Creates and initializes a ModbusLoop with Modbus client, validation engine, and fan-out channels.
// input: queries (DB query helper), interval (poll cadence), address (Modbus TCP endpoint), mwBase (register base), ValidationFilePath (rules file path).
// output: Returns a ready ModbusLoop pointer; exits process on critical startup failures.
func NewModbusLoop(queries *database.Queries, interval time.Duration, address string, mwBase uint16, ValidationFilePath string) *ModbusLoop {
	handler := modbus.NewTCPClientHandler(address)
	handler.Timeout = 10 * time.Second
	handler.SlaveId = 1
	if err := handler.Connect(); err != nil {
		log.Fatalf("Modbus connect failed (%s): %v", address, err)
	}

	client := modbus.NewClient(handler)
	if interval <= 0 {
		interval = 5 * time.Second
	}
	validator, err := NewRuleEngine(ValidationFilePath)
	if err != nil {
		log.Fatalf("Validation spec load failed (%s): %v", ValidationFilePath, err)
	}

	loop := &ModbusLoop{
		client:    client,
		queries:   queries,
		interval:  interval,
		mwBase:    mwBase,
		validator: validator,
		mlCh:      make(chan fanoutEvent, 128),
		sqlCh:     make(chan fanoutEvent, 128),
		wsCh:      make(chan fanoutEvent, 128),
	}
	loop.jsonEncoder = json.NewEncoder(&loop.jsonBuf)
	loop.jsonEncoder.SetEscapeHTML(false) // Shouldn't matter atm due to us sending int / uints back but a later protection
	return loop
}

// description: Starts sink workers and continuously executes ingest ticks until context cancellation.
// input: ctx (cancellation context controlling loop lifetime).
// output: Returns context cancellation error on shutdown, otherwise runtime errors are logged per tick.
func (m *ModbusLoop) Run(ctx context.Context) error {
	m.startFanoutWorkers(ctx)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		if err := m.handleTick(ctx); err != nil {
			log.Printf("Modbus ingest tick failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// description: Executes one ingest cycle: read Modbus registers, normalize, validate, encode, and fan-out.
// input: ctx (used to abort processing when canceled).
// output: Returns error for Modbus/read/validation/encoding failures, nil on successful dispatch.
func (m *ModbusLoop) handleTick(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	results, err := m.client.ReadHoldingRegisters(m.mwBase, 6)
	if err != nil {
		return err
	}
	if err := m.dataNormalizer(results); err != nil {
		return err
	}
	validation := m.validator.Validate(&m.sample)
	if !validation.Valid {
		return fmt.Errorf("Validation failed: %v", validation.Errors)
	}
	m.sample.Anomalies = validation.Anomalies

	m.jsonBuf.Reset()
	if err := m.jsonEncoder.Encode(&m.sample); err != nil {
		return fmt.Errorf("Encode sample json: %w", err)
	}

	// Copy -> Fanout as it is overwritten we don't want a pointer to it
	sample := m.sample
	sample.Anomalies = append([]string(nil), m.sample.Anomalies...)
	payload := append([]byte(nil), m.jsonBuf.Bytes()...)
	m.fanOut(fanoutEvent{sample: sample, payload: payload})
	return nil
}

// description: Converts raw Modbus register bytes into the normalized TempSample fields.
// input: results (12-byte register payload from Modbus read).
// output: Updates m.sample in place and returns nil, or returns error on invalid payload/type.
func (m *ModbusLoop) dataNormalizer(results []byte) error {
	// 2 bytes per register * 6 registers = 12 bytes.
	if len(results) != 12 {
		return fmt.Errorf("Invalid size: got %d bytes", len(results))
	}

	sensorCode := binary.BigEndian.Uint16(results[2:4])
	sensorType, err := sensorTypeFromCode(sensorCode)
	if err != nil {
		return err
	}

	// Later change to better fit the actual dataset properly this is mainly filler till we swap to simulink to get data flowing
	m.sample.ID = binary.BigEndian.Uint16(results[0:2])
	m.sample.Timestamp = nowUTC()
	m.sample.SensorType = sensorType
	m.sample.SensorNumber = binary.BigEndian.Uint16(results[4:6])
	m.sample.FanOn = binary.BigEndian.Uint16(results[6:8]) == 1
	m.sample.Temperature = float64(int16(binary.BigEndian.Uint16(results[8:10]))) / 10.0
	m.sample.HeaterPower = float64(binary.BigEndian.Uint16(results[10:12])) / 100.0

	return nil
}

// description: Maps numeric sensor type codes to human-readable sensor type names.
// input: code (raw sensor type numeric code from Modbus data).
// output: Returns mapped sensor type string or an error for unsupported codes.
func sensorTypeFromCode(code uint16) (string, error) {
	switch code {
	case 1:
		return "temperature_control_system", nil
	default:
		return "", fmt.Errorf("Invalid sensor type: %d", code)
	}
}
