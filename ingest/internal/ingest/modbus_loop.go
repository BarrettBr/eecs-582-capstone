package ingest

// This file is mainlyy placeholder for now, it will probably change dramatically as we add in the actual modbus data

import (
	"context"
	"log"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
)

type ModbusLoop struct {
	queries  *database.Queries
	interval time.Duration
}

func NewModbusLoop(queries *database.Queries, interval time.Duration) *ModbusLoop {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &ModbusLoop{
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
	// Basicallly all placeholder for now, eventually we will want to actually read in that data
	_ = ctx
	_ = m.queries
	log.Println("modbus ingest tick")
	return nil
}
