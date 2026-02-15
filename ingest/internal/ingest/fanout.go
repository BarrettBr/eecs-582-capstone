package ingest

import (
	"context"
	"log"
)

type fanoutEvent struct {
	sample  TempSample
	payload []byte
}

func (m *ModbusLoop) startFanoutWorkers(ctx context.Context) {
	go m.runSink(ctx, "ml", m.mlCh, m.deliverToML)
	go m.runSink(ctx, "sql", m.sqlCh, m.deliverToSQL)
	go m.runSink(ctx, "websocket", m.wsCh, m.deliverToWebsocket)
}

func (m *ModbusLoop) fanOut(event fanoutEvent) {
	m.enqueue("ml", m.mlCh, event)
	m.enqueue("sql", m.sqlCh, event)
	m.enqueue("websocket", m.wsCh, event)
}

func (m *ModbusLoop) runSink(ctx context.Context, name string, sink <-chan fanoutEvent, handler func(context.Context, fanoutEvent) error) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-sink:
			if err := handler(ctx, event); err != nil {
				log.Printf("Sink=%s error: %v", name, err)
			}
		}
	}
}

func (m *ModbusLoop) enqueue(name string, sink chan<- fanoutEvent, event fanoutEvent) {
	select {
	case sink <- event:
	default:
		log.Printf("Sink=%s dropped sample id=%d", name, event.sample.ID)
	}
}

func (m *ModbusLoop) deliverToML(_ context.Context, event fanoutEvent) error {
	_ = event
	return nil
}

func (m *ModbusLoop) deliverToSQL(_ context.Context, event fanoutEvent) error {
	_ = event
	return nil
}

func (m *ModbusLoop) deliverToWebsocket(_ context.Context, event fanoutEvent) error {
	_ = event
	return nil
}
