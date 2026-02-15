package ingest

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

type ModbusLoop struct {
	client      modbus.Client
	queries     *database.Queries
	interval    time.Duration
	mwBase      uint16
	sample      TempSample
	jsonBuf     bytes.Buffer  // Writer that stores bytes for logging
	jsonEncoder *json.Encoder // Converts struct -> Json
}

func NewModbusLoop(queries *database.Queries, interval time.Duration, address string, mwBase uint16) *ModbusLoop {
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

	loop := &ModbusLoop{
		client:   client,
		queries:  queries,
		interval: interval,
		mwBase:   mwBase,
	}
	loop.jsonEncoder = json.NewEncoder(&loop.jsonBuf)
	loop.jsonEncoder.SetEscapeHTML(false) // Shouldn't matter atm due to us sending int / uints back but a later protection
	return loop
}

func (m *ModbusLoop) Run(ctx context.Context) error {
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
	if err := m.dataValidaterAndNormalizer(results); err != nil {
		return err
	}

	m.jsonBuf.Reset()
	if err := m.jsonEncoder.Encode(&m.sample); err != nil {
		return fmt.Errorf("Encode sample json: %w", err)
	}

	// Print for now
	payload := m.jsonBuf.Bytes()
	log.Printf("Sample=%s", payload)
	return nil
}

func (m *ModbusLoop) dataValidaterAndNormalizer(results []byte) error {
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
	m.sample.SensorType = sensorType
	m.sample.SensorNumber = binary.BigEndian.Uint16(results[4:6])
	m.sample.FanOn = binary.BigEndian.Uint16(results[6:8]) == 1
	m.sample.Temperature = float64(int16(binary.BigEndian.Uint16(results[8:10]))) / 10.0
	m.sample.HeaterPower = float64(binary.BigEndian.Uint16(results[10:12])) / 100.0

	return nil
}

func sensorTypeFromCode(code uint16) (string, error) {
	switch code {
	case 1:
		return "temperature_control_system", nil
	default:
		return "", fmt.Errorf("Invalid sensor type: %d", code)
	}
}
