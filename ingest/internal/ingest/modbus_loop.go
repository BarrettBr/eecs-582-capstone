package ingest

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	"github.com/BarrettBr/eecs-582-capstone/internal/stream"
	"github.com/goburrow/modbus"
)

type ModbusLoop struct {
	db                    *sql.DB
	queries               *database.Queries
	interval              time.Duration
	sources               []*sourceRunner
	sample                TempSample
	mlCh                  chan fanoutEvent
	sqlCh                 chan fanoutEvent
	wsCh                  chan fanoutEvent
	mlAPIURL              string
	mlHTTP                *http.Client
	mlBatchSize           int
	mlBatchFlushInterval  time.Duration
	mlDropOnOverload      bool
	mlLastResponse        []byte
	streamer              *stream.Server
	sqlBatchSize          int
	sqlBatchFlushInterval time.Duration
	sqlNormalSampleRate   int
	sqlNormalSampleCount  map[string]int
}

type sourceRunner struct {
	definition      SourceDefinition
	validator       *RuleEngine
	interval        time.Duration
	client          modbus.Client
	mu              sync.Mutex
	rng             *rand.Rand
	simTemp         float64
	simStep         uint16
	forceFaultTicks int
}

type MLFanoutConfig struct {
	APIURL             string
	HTTPTimeout        time.Duration
	BatchSize          int
	BatchFlushInterval time.Duration
	DropOnOverload     bool
}

type SQLFanoutConfig struct {
	BatchSize          int
	BatchFlushInterval time.Duration
	NormalSampleRate   int
}

// NewModbusLoop now builds the generalized ingest runtime from a source catalog.
func NewModbusLoop(db *sql.DB, queries *database.Queries, interval time.Duration, sourceConfigPath string, mlCfg MLFanoutConfig, sqlCfg SQLFanoutConfig, wsServer *stream.Server) *ModbusLoop {
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
	if mlCfg.BatchSize <= 0 {
		mlCfg.BatchSize = 1
	}
	if mlCfg.BatchFlushInterval <= 0 {
		mlCfg.BatchFlushInterval = 25 * time.Millisecond
	}
	if mlCfg.HTTPTimeout <= 0 {
		mlCfg.HTTPTimeout = 500 * time.Millisecond
	}

	catalog, err := LoadSourceCatalog(sourceConfigPath)
	if err != nil {
		log.Fatalf("Source config load failed (%s): %v", sourceConfigPath, err)
	}

	sources, err := buildSourceRunners(catalog, interval)
	if err != nil {
		log.Fatalf("Source initialization failed: %v", err)
	}

	return &ModbusLoop{
		db:       db,
		queries:  queries,
		interval: interval,
		sources:  sources,
		mlCh:     make(chan fanoutEvent, 128),
		sqlCh:    make(chan fanoutEvent, 128),
		wsCh:     make(chan fanoutEvent, 128),
		mlAPIURL: mlCfg.APIURL,
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
		streamer:              wsServer,
		sqlBatchSize:          sqlCfg.BatchSize,
		sqlBatchFlushInterval: sqlCfg.BatchFlushInterval,
		sqlNormalSampleRate:   sqlCfg.NormalSampleRate,
		sqlNormalSampleCount:  make(map[string]int),
	}
}

func buildSourceRunners(catalog *SourceCatalog, defaultModbusInterval time.Duration) ([]*sourceRunner, error) {
	sources := make([]*sourceRunner, 0, len(catalog.Sources))
	for _, definition := range catalog.Sources {
		validator, err := NewRuleEngine(definition.ValidationFile)
		if err != nil {
			return nil, fmt.Errorf("load validator for %q: %w", definition.Name, err)
		}
		if !definition.Enabled {
			continue
		}
		runner := &sourceRunner{
			definition: definition,
			validator:  validator,
		}
		switch definition.Mode {
		case "modbus":
			// Each Modbus source maintains its own client and register settings.
			handler := modbus.NewTCPClientHandler(definition.Modbus.Address)
			handler.Timeout = 10 * time.Second
			handler.SlaveId = definition.Modbus.SlaveID
			if err := handler.Connect(); err != nil {
				return nil, fmt.Errorf("connect source %q (%s): %w", definition.Name, definition.Modbus.Address, err)
			}
			runner.client = modbus.NewClient(handler)
			runner.interval = defaultModbusInterval
		case "simulator":
			// The simulator keeps deterministic local state for repeatable dev data.
			d, err := time.ParseDuration(definition.Simulator.Interval)
			if err != nil {
				return nil, fmt.Errorf("parse simulator interval for %q: %w", definition.Name, err)
			}
			runner.interval = d
			runner.rng = rand.New(rand.NewSource(definition.Simulator.Seed))
			runner.simTemp = 68.0
		default:
			return nil, fmt.Errorf("unsupported mode %q", definition.Mode)
		}
		sources = append(sources, runner)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no enabled sources configured")
	}
	return sources, nil
}

func (m *ModbusLoop) Run(ctx context.Context) error {
	// Start shared sinks first, then launch one worker goroutine per enabled source.
	m.startFanoutWorkers(ctx)

	for _, source := range m.sources {
		go m.runSource(ctx, source)
	}

	<-ctx.Done()
	return ctx.Err()
}

func (m *ModbusLoop) runSource(ctx context.Context, source *sourceRunner) {
	if err := m.handleSourceTick(ctx, source); err != nil && !isContextCanceled(err) {
		log.Printf("Source=%s initial tick failed: %v", source.definition.Name, err)
	}

	ticker := time.NewTicker(source.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.handleSourceTick(ctx, source); err != nil && !isContextCanceled(err) {
				log.Printf("Source=%s tick failed: %v", source.definition.Name, err)
			}
		}
	}
}

func (m *ModbusLoop) handleSourceTick(ctx context.Context, source *sourceRunner) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	record, err := source.nextRecord()
	if err != nil {
		return err
	}

	// Validate against the source-specific rule file loaded at startup.
	validation := source.validator.Validate(record)
	if !validation.Valid {
		return fmt.Errorf("validation failed: %v", validation.Errors)
	}

	record, err = applyAnomalies(record, validation.Anomalies)
	if err != nil {
		return err
	}

	payload, err := encodeRecordPayload(record)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", record.EventType(), err)
	}

	m.fanOut(ctx, fanoutEvent{
		sourceName: source.definition.Name,
		mlEnabled:  source.definition.MLEnabled,
		record:     record,
		payload:    payload,
	})
	return nil
}

func (s *sourceRunner) nextRecord() (RecordEvent, error) {
	switch s.definition.Mode {
	case "simulator":
		return s.nextSimulatedRecord()
	case "modbus":
		return s.nextModbusRecord()
	default:
		return nil, fmt.Errorf("source %q unsupported mode %q", s.definition.Name, s.definition.Mode)
	}
}

func (s *sourceRunner) nextSimulatedRecord() (RecordEvent, error) {
	if s.definition.EventType != "temperature" {
		return nil, fmt.Errorf("source %q simulator does not support event type %q", s.definition.Name, s.definition.EventType)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Use bounded drift plus occasional spikes so anomalies appear in local dev.
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

	return TempSample{
		ID:           s.simStep,
		Timestamp:    nowUTC(),
		SensorType:   "temperature_control_system",
		SensorNumber: 1,
		FanOn:        fanOn,
		Temperature:  roundToTenth(s.simTemp),
		HeaterPower:  roundToHundredth(heaterPower),
	}, nil
}

// TriggerFaultInjection forces the named simulator source (or the first simulator source if empty)
// to emit anomalous temperature readings for a short burst.
func (m *ModbusLoop) TriggerFaultInjection(sourceName string) error {
	for _, source := range m.sources {
		if source.definition.Mode != "simulator" {
			continue
		}
		if sourceName != "" && source.definition.Name != sourceName {
			continue
		}

		source.mu.Lock()
		source.forceFaultTicks = 10
		source.mu.Unlock()
		return nil
	}

	if sourceName == "" {
		return fmt.Errorf("no simulator source available for fault injection")
	}
	return fmt.Errorf("simulator source %q not found", sourceName)
}

func (s *sourceRunner) nextModbusRecord() (RecordEvent, error) {
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

func encodeRecordPayload(record RecordEvent) ([]byte, error) {
	payload, err := json.Marshal(record.Payload())
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), payload...), nil
}

func applyAnomalies(record RecordEvent, anomalies []string) (RecordEvent, error) {
	switch sample := record.(type) {
	case TempSample:
		sample.Anomalies = append([]string(nil), anomalies...)
		return sample, nil
	case *TempSample:
		if sample == nil {
			return nil, fmt.Errorf("temperature record is nil")
		}
		sample.Anomalies = append([]string(nil), anomalies...)
		return sample, nil
	case ValveSample:
		sample.Anomalies = append([]string(nil), anomalies...)
		return sample, nil
	case *ValveSample:
		if sample == nil {
			return nil, fmt.Errorf("valve record is nil")
		}
		sample.Anomalies = append([]string(nil), anomalies...)
		return sample, nil
	default:
		return nil, fmt.Errorf("unsupported record type %T", record)
	}
}

func (m *ModbusLoop) dataNormalizer(results []byte) error {
	sample, err := normalizeTemperatureRegisters(results)
	if err != nil {
		return err
	}
	m.sample = sample
	return nil
}

func normalizeTemperatureRegisters(results []byte) (TempSample, error) {
	// Preserve the existing 6-register temperature layout.
	if len(results) != 12 {
		return TempSample{}, fmt.Errorf("invalid size: got %d bytes", len(results))
	}

	sensorCode := binary.BigEndian.Uint16(results[2:4])
	sensorType, err := sensorTypeFromCode(sensorCode)
	if err != nil {
		return TempSample{}, err
	}

	return TempSample{
		ID:           binary.BigEndian.Uint16(results[0:2]),
		Timestamp:    nowUTC(),
		SensorType:   sensorType,
		SensorNumber: binary.BigEndian.Uint16(results[4:6]),
		FanOn:        binary.BigEndian.Uint16(results[6:8]) == 1,
		Temperature:  float64(int16(binary.BigEndian.Uint16(results[8:10]))) / 10.0,
		HeaterPower:  float64(binary.BigEndian.Uint16(results[10:12])) / 100.0,
	}, nil
}

func normalizeValveRegisters(results []byte) (ValveSample, error) {
	// Decode the locked 5-register valve layout.
	if len(results) != 10 {
		return ValveSample{}, fmt.Errorf("invalid size: got %d bytes", len(results))
	}

	sensorCode := binary.BigEndian.Uint16(results[2:4])
	sensorType, err := valveSensorTypeFromCode(sensorCode)
	if err != nil {
		return ValveSample{}, err
	}

	return ValveSample{
		ID:          binary.BigEndian.Uint16(results[0:2]),
		Timestamp:   nowUTC(),
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
