package ingest

/*
Name: ingest/internal/ingest/modbus_loop.go
Description: Main Modbus polling loop. Reads registers, builds a sample, validates it, then sends it to fanout sinks.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-28
Revision History:
- 2026-02-01: Barrett Brown: Created base file structure
- 2026-02-13: Barrett Brown: Expanded / Made more in line with PLC structured data
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
- 2026-02-28, Barrett Brown: Added database as a part of the modbus constructor
Preconditions:
- Modbus TCP endpoint is reachable.
- Validation rules file exists and config is set.
Acceptable Input Values/Types:
- Modbus response bytes with expected length (12 bytes for current mapping).
- Supported sensor type codes.
- Context that can be canceled to stop the loop.
Unacceptable Input Values/Types:
- Incorrect register payload size or unsupported sensor type codes.
- Bad validation file/path at startup.
Postconditions:
- Each successful tick sends one validated sample event to fanout.
- Loop terminates when context is canceled.
Return Values/Types:
- NewModbusLoop: *ModbusLoop
- Run/handleTick/dataNormalizer: error (nil = success)
- sensorTypeFromCode: (string, error)
Error/Exception Conditions:
- Modbus read/parse/validation/JSON errors.
- Startup failures in connect/rule load stop the process with log.Fatalf.
Side Effects:
- Reads from Modbus over network, updates sample state, enqueues events, writes logs.
Invariants:
- dataNormalizer expects exactly 12 bytes for current 6-register mapping.
- Sample timestamp is always written in UTC each tick.
Known Faults:
- Uses fatal exits at startup instead of retry logic.
*/

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	"github.com/goburrow/modbus"
)

// description: Polling loop that reads Modbus registers, validates samples, and dispatches them to sinks.
// input: Constructed with database queries, poll settings, Modbus settings, and validation rules.
// output: Produces validated TempSample events routed through the fan-out pipeline.
type ModbusLoop struct {
	client                modbus.Client
	db                    *sql.DB
	queries               *database.Queries
	interval              time.Duration
	mwBase                uint16
	validator             *RuleEngine
	sample                TempSample
	jsonBuf               bytes.Buffer  // Writer that stores bytes for logging
	jsonEncoder           *json.Encoder // Converts struct -> Json
	mlCh                  chan fanoutEvent
	sqlCh                 chan fanoutEvent
	wsCh                  chan fanoutEvent
	mlAPIURL              string
	mlHTTP                *http.Client
	mlBatchSize           int
	mlBatchFlushInterval  time.Duration
	mlDropOnOverload      bool
	mlLastResponse        []byte
	sqlBatchSize          int
	sqlBatchFlushInterval time.Duration
	sqlNormalSampleRate   int
	sqlNormalSampleCount  int
}

// description: ML fanout runtime settings for batching, transport, and overload behavior.
// input: Provided by config at startup with env-derived values.
// output: Consumed by NewModbusLoop to initialize ML delivery behavior.
type MLFanoutConfig struct {
	APIURL             string
	HTTPTimeout        time.Duration
	BatchSize          int
	BatchFlushInterval time.Duration
	DropOnOverload     bool
}

// description: SQL fanout runtime settings for batching and normal-event sampling.
// input: Provided by config at startup with env-derived values.
// output: Consumed by NewModbusLoop to initialize SQL delivery behavior.
type SQLFanoutConfig struct {
	BatchSize          int
	BatchFlushInterval time.Duration
	NormalSampleRate   int
}

// description: Creates and initializes a ModbusLoop with Modbus client, validation engine, and fan-out channels.
// input: queries (DB query helper), interval (poll cadence), address (Modbus TCP endpoint), mwBase (register base), ValidationFilePath (rules file path).
// output: Returns a ready ModbusLoop pointer; exits process on critical startup failures.
func NewModbusLoop(db *sql.DB, queries *database.Queries, interval time.Duration, address string, mwBase uint16, ValidationFilePath string, mlCfg MLFanoutConfig, sqlCfg SQLFanoutConfig) *ModbusLoop {
	// Setup the modbus handler & start the client listening to it
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
	if sqlCfg.BatchSize <= 0 {
		sqlCfg.BatchSize = 1
	}
	if sqlCfg.BatchFlushInterval <= 0 {
		sqlCfg.BatchFlushInterval = 25 * time.Millisecond
	}
	if sqlCfg.NormalSampleRate <= 0 {
		sqlCfg.NormalSampleRate = 1
	}

	// Create the rule engine based on the validation file
	validator, err := NewRuleEngine(ValidationFilePath)
	if err != nil {
		log.Fatalf("Validation spec load failed (%s): %v", ValidationFilePath, err)
	}

	// Create the ModbusLoop struct fill out and return it for use with future validation and reading
	loop := &ModbusLoop{
		client:    client,
		db:        db,
		queries:   queries,
		interval:  interval,
		mwBase:    mwBase,
		validator: validator,
		mlCh:      make(chan fanoutEvent, 128),
		sqlCh:     make(chan fanoutEvent, 128),
		wsCh:      make(chan fanoutEvent, 128),
		mlAPIURL:  mlCfg.APIURL,
		mlHTTP: &http.Client{
			Timeout: mlCfg.HTTPTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				MaxConnsPerHost:     0,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		mlBatchSize:           mlCfg.BatchSize,
		mlBatchFlushInterval:  mlCfg.BatchFlushInterval,
		mlDropOnOverload:      mlCfg.DropOnOverload,
		sqlBatchSize:          sqlCfg.BatchSize,
		sqlBatchFlushInterval: sqlCfg.BatchFlushInterval,
		sqlNormalSampleRate:   sqlCfg.NormalSampleRate,
	}
	loop.jsonEncoder = json.NewEncoder(&loop.jsonBuf)
	loop.jsonEncoder.SetEscapeHTML(false) // Shouldn't matter atm due to us sending int / uints back but a later protection
	return loop
}

// description: Starts sink workers and continuously executes ingest ticks until context cancellation.
// input: ctx (cancellation context controlling loop lifetime).
// output: Returns context cancellation error on shutdown, otherwise runtime errors are logged per tick.
func (m *ModbusLoop) Run(ctx context.Context) error {
	// Start fanout workers to listen and consume jobs
	m.startFanoutWorkers(ctx)

	// Tick on interval for reading
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
	// Context check for graceful shutdown
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Read the holding register and store values
	results, err := m.client.ReadHoldingRegisters(m.mwBase, 6)
	if err != nil {
		return err
	}

	// Normalize data to a generic form then validate that form against our rule file
	if err := m.dataNormalizer(results); err != nil {
		return err
	}
	validation := m.validator.Validate(&m.sample)
	if !validation.Valid {
		return fmt.Errorf("Validation failed: %v", validation.Errors)
	}
	m.sample.Anomalies = validation.Anomalies

	// Reset whatever sits in the buffer and then write the encoding of the sample to the encoder
	m.jsonBuf.Reset()
	if err := m.jsonEncoder.Encode(&m.sample); err != nil {
		return fmt.Errorf("Encode sample json: %w", err)
	}

	// Copy -> Fanout as it is overwritten we don't want a pointer to it
	sample := m.sample
	sample.Anomalies = append([]string(nil), m.sample.Anomalies...)
	payload := append([]byte(nil), m.jsonBuf.Bytes()...)
	m.fanOut(ctx, fanoutEvent{record: sample, payload: payload})
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

	// Parse and then validate sensor type
	sensorCode := binary.BigEndian.Uint16(results[2:4])
	sensorType, err := sensorTypeFromCode(sensorCode)
	if err != nil {
		return err
	}

	// Later change to better fit the actual dataset properly this is mainly filler till we swap to simulink to get data flowing
	// Fill out sample data with the values read in
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
	// As register values are ints we just swap on that to get the name of the system
	switch code {
	case 1:
		return "temperature_control_system", nil
	default:
		return "", fmt.Errorf("Invalid sensor type: %d", code)
	}
}
