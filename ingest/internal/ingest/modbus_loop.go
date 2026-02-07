package ingest

// This file is mainlyy placeholder for now, it will probably change dramatically as we add in the actual modbus data

import (
	"context"
	"encoding/binary"
	"log"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	"github.com/goburrow/modbus"
)

type ModbusLoop struct {
	client modbus.Client
    queries  *database.Queries
	interval time.Duration
}

func NewModbusLoop(queries *database.Queries, interval time.Duration, address string) *ModbusLoop {
	handler := modbus.NewTCPClientHandler(address) // We could use NewRTUClientHandler instead if connecting over serial. Maybe some way to tell?
    handler.Timeout = 10 * time.Second
    client := modbus.NewClient(handler)
    if interval <= 0 {
		interval = 5 * time.Second
	}
	return &ModbusLoop{
		client: client,
        queries:  queries,
		interval: interval,
	}
}

func (m *ModbusLoop) Run(ctx context.Context) error {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		if err := m.handleTick(ctx); err != nil {
			log.Printf("modbus ingest tick failed: %v", err)
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
    results, err := m.client.ReadHoldingRegisters(0, 1)
    if err != nil {
        return err
    }
    counter := binary.BigEndian.Uint16(results)
	log.Printf("Modbus Results: %v\n", counter)
	return nil
}
