package ingest

/*
Name: ingest/internal/ingest/fanout.go
Description: Handles fanout so one sample can be sent to ML, SQL, and websocket workers.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-15
Revision History:
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
Preconditions:
- ModbusLoop channels are initialized before worker start.
- Event data should be copied before sharing across workers.
Acceptable Input Values/Types:
- fanoutEvent containing TempSample and payload []byte.
- Non-nil context for worker lifecycle.
Unacceptable Input Values/Types:
- Nil/uninitialized channels (can block or panic).
- Reusing the same payload data in multiple places without making a copy first.
Postconditions:
- Tries to send each event to all sinks; a sink can still drop if full.
- Workers stop when context is canceled.
Return Values/Types:
- Most functions return no value.
- Sink handlers return error (currently nil in placeholders).
Error/Exception Conditions:
- Handler errors are logged by runSink.
- Dropped events are logged when sink channel is full.
Side Effects:
- Starts goroutines, uses channels, writes logs.
Invariants:
- enqueue is non-blocking and does not wait on full channels.
- Each sink has a dedicated worker loop.
Known Faults:
- Sink handlers are placeholders right now.
*/

import (
	"context"
	"log"
)

// description: Immutable event passed to each downstream sink worker.
// input: sample (normalized struct), payload (JSON-encoded bytes for wire/storage use).
// output: Consumed by sink handlers for ML, SQL, and websocket delivery.
type fanoutEvent struct {
	sample  TempSample
	payload []byte
}

// description: Starts one worker goroutine per sink channel.
// input: ctx (shared cancellation context for all workers).
// output: None; workers run asynchronously until ctx is canceled.
func (m *ModbusLoop) startFanoutWorkers(ctx context.Context) {
	// Start a goroutine for each consumer to allow each to consume tasks
	go m.runSink(ctx, "ml", m.mlCh, m.deliverToML)
	go m.runSink(ctx, "sql", m.sqlCh, m.deliverToSQL)
	go m.runSink(ctx, "websocket", m.wsCh, m.deliverToWebsocket)
}

// description: Dispatches one event to all configured sinks.
// input: event (sample and payload snapshot to deliver).
// output: None; each sink enqueue may accept or drop independently.
func (m *ModbusLoop) fanOut(event fanoutEvent) {
	// Enque the event to each service to be consumed on it's own time
	m.enqueue("ml", m.mlCh, event)
	m.enqueue("sql", m.sqlCh, event)
	m.enqueue("websocket", m.wsCh, event)
}

// description: Worker loop that drains a sink channel and executes its handler.
// input: ctx (cancellation), name (sink label), sink (event channel), handler (sink-specific processor).
// output: None; logs sink handler errors and exits on context cancellation.
func (m *ModbusLoop) runSink(ctx context.Context, name string, sink <-chan fanoutEvent, handler func(context.Context, fanoutEvent) error) {
	// Pass event to the handler function of that sink
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-sink:
			// Currently print error, swap to logging or something down the line
			if err := handler(ctx, event); err != nil {
				log.Printf("Sink=%s error: %v", name, err)
			}
		}
	}
}

// description: Attempts a non-blocking enqueue into a sink channel.
// input: name (sink label), sink (target channel), event (fanout payload).
// output: None; logs when the sink is full and event is dropped.
func (m *ModbusLoop) enqueue(name string, sink chan<- fanoutEvent, event fanoutEvent) {
	// Give event to the sink itself to handle, eventually update default to log failures
	select {
	case sink <- event:
	default:
		log.Printf("Sink=%s dropped sample id=%d", name, event.sample.ID)
	}
}

// description: Placeholder ML sink handler for future inference/publishing logic.
// input: context and fanout event.
// output: Returns nil; no-op skeleton.
func (m *ModbusLoop) deliverToML(_ context.Context, event fanoutEvent) error {
	_ = event
	return nil
}

// description: Placeholder SQL sink handler for future persistence logic.
// input: context and fanout event.
// output: Returns nil; no-op skeleton.
func (m *ModbusLoop) deliverToSQL(_ context.Context, event fanoutEvent) error {
	_ = event
	return nil
}

// description: Placeholder websocket sink handler for future broadcast logic.
// input: context and fanout event.
// output: Returns nil; no-op skeleton.
func (m *ModbusLoop) deliverToWebsocket(_ context.Context, event fanoutEvent) error {
	_ = event
	return nil
}
