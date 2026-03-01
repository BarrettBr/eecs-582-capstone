package ingest

/*
Name: ingest/internal/ingest/fanout.go
Description: Handles fanout so one sample can be sent to ML, SQL, and websocket workers.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-28
Revision History:
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
- 2026-02-28, Barrett Brown:
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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
)

// description: Immutable event passed to each downstream sink worker.
// input: sample (normalized struct), payload (JSON-encoded bytes for wire/storage use).
// output: Consumed by sink handlers for ML, SQL, and websocket delivery.
type fanoutEvent struct {
	record  RecordEvent
	payload []byte
}

type mlBatchRequest struct {
	Samples []TempSample `json:"samples"`
}

// description: Starts one worker goroutine per sink channel.
// input: ctx (shared cancellation context for all workers).
// output: None; workers run asynchronously until ctx is canceled.
func (m *ModbusLoop) startFanoutWorkers(ctx context.Context) {
	// Start a goroutine for each consumer to allow each to consume tasks
	if m.mlAPIURL != "" {
		go m.runMLSink(ctx)
	} else {
		log.Printf("Sink=ml disabled: ML_API_URL is empty")
	}
	go m.runSQLSink(ctx)
	go m.runSink(ctx, "websocket", m.wsCh, m.deliverToWebsocket)
}

// description: Dispatches one event to all configured sinks.
// input: event (sample and payload snapshot to deliver).
// output: None; each sink enqueue may accept or drop independently.
func (m *ModbusLoop) fanOut(ctx context.Context, event fanoutEvent) {
	// Enque the event to each service to be consumed on it's own time
	m.enqueueML(ctx, event)
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
		log.Printf("Sink=%s dropped event type=%s", name, event.record.EventType())
	}
}

// description: Sends one event to the ML sink with either drop-on-full or backpressure behavior.
// input: ctx (for canceling blocking sends) and event (fanout payload).
// output: None; logs drops or canceled backpressure sends.
func (m *ModbusLoop) enqueueML(ctx context.Context, event fanoutEvent) {
	if m.mlAPIURL == "" {
		return
	}
	if m.mlDropOnOverload {
		m.enqueue("ml", m.mlCh, event)
		return
	}

	select {
	case <-ctx.Done():
		log.Printf("Sink=ml canceled before enqueue event type=%s", event.record.EventType())
	case m.mlCh <- event:
	}
}

// description: Drains ML events, batches them, and flushes to the ML API on size or interval.
// input: ctx (cancellation context for worker lifecycle).
// output: None; logs batch delivery errors and exits on cancellation.
func (m *ModbusLoop) runMLSink(ctx context.Context) {
	batch := make([]fanoutEvent, 0, m.mlBatchSize)
	ticker := time.NewTicker(m.mlBatchFlushInterval)
	defer ticker.Stop()

	flush := func(flushCtx context.Context) {
		if len(batch) == 0 {
			return
		}
		if err := m.deliverMLBatch(flushCtx, batch); err != nil {
			log.Printf("Sink=ml error: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush(context.Background())
			return
		case event := <-m.mlCh:
			batch = append(batch, event)
			if len(batch) >= m.mlBatchSize {
				flush(ctx)
			}
		case <-ticker.C:
			flush(ctx)
		}
	}
}

// description: Drains SQL events, batches them, and flushes to SQLite on size or interval.
// input: ctx (cancellation context for worker lifecycle).
// output: None; logs batch delivery errors and exits on cancellation.
func (m *ModbusLoop) runSQLSink(ctx context.Context) {
	batch := make([]fanoutEvent, 0, m.sqlBatchSize)
	ticker := time.NewTicker(m.sqlBatchFlushInterval)
	defer ticker.Stop()

	flush := func(flushCtx context.Context) {
		if len(batch) == 0 {
			return
		}
		if err := m.deliverSQLBatch(flushCtx, batch); err != nil {
			log.Printf("Sink=sql error: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush(context.Background())
			return
		case event := <-m.sqlCh:
			batch = append(batch, event)
			if len(batch) >= m.sqlBatchSize {
				flush(ctx)
			}
		case <-ticker.C:
			flush(ctx)
		}
	}
}

// description: Encodes and posts a batch to the ML API, then stores the last valid JSON response body.
// input: ctx (request lifecycle) and batch (one or more fanout events).
// output: Returns error on request, non-2xx status, or invalid JSON response.
func (m *ModbusLoop) deliverMLBatch(ctx context.Context, batch []fanoutEvent) error {
	if len(batch) == 0 {
		return nil
	}
	if m.mlAPIURL == "" {
		return nil
	}

	// Append all samples for marshaling
	samples := make([]TempSample, 0, len(batch))
	for _, event := range batch {
		if event.record.EventType() != "temperature" {
			log.Printf("Sink=ml skipped unsupported event type=%s", event.record.EventType())
			continue
		}

		sample, ok := event.record.Payload().(TempSample)
		if !ok {
			if samplePtr, ok := event.record.Payload().(*TempSample); ok && samplePtr != nil {
				samples = append(samples, *samplePtr)
				continue
			}
			log.Printf("Sink=ml skipped malformed temperature payload type=%T", event.record.Payload())
			continue
		}
		samples = append(samples, sample)
	}
	if len(samples) == 0 {
		return nil
	}

	body, err := json.Marshal(mlBatchRequest{Samples: samples})
	if err != nil {
		return fmt.Errorf("ML request encode: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.mlAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ML request build: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.mlHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("ML request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ML response read: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return fmt.Errorf("ML response status=%d body=%q", resp.StatusCode, msg)
	}

	if trimmed := bytes.TrimSpace(respBody); len(trimmed) > 0 {
		var parsed any
		if err := json.Unmarshal(trimmed, &parsed); err != nil {
			return fmt.Errorf("ML response json parse: %w", err)
		}
		normalized := normalizeMLAnomalyPayload(parsed, trimmed)
		normalizedBody, err := json.Marshal(normalized)
		if err != nil {
			return fmt.Errorf("ML normalized response encode: %w", err)
		}
		m.mlLastResponse = append(m.mlLastResponse[:0], normalizedBody...)
		if m.streamer != nil {
			m.streamer.PublishMLResult(normalizedBody)
		}
	} else {
		m.mlLastResponse = m.mlLastResponse[:0]
	}

	return nil
}

// description: SQL sink handler compatibility wrapper for one-off writes.
// input: context and fanout event.
// output: Persists a single event by delegating to the batch path.
func (m *ModbusLoop) deliverToSQL(ctx context.Context, event fanoutEvent) error {
	return m.deliverSQLBatch(ctx, []fanoutEvent{event})
}

// description: Persists a batch of supported events in one transaction using sqlc-generated queries.
// input: ctx (request lifecycle) and batch (one or more fanout events).
// output: Returns error on transaction or insert failure.
func (m *ModbusLoop) deliverSQLBatch(ctx context.Context, batch []fanoutEvent) error {
	samples := make([]TempSample, 0, len(batch))
	for _, event := range batch {
		switch event.record.EventType() {
		case "temperature":
			sample, err := eventTempSample(event)
			if err != nil {
				return err
			}
			if !m.shouldPersistNormalSample(sample) {
				continue
			}
			samples = append(samples, sample)
		default:
			log.Printf("Sink=sql skipped unsupported event type=%s", event.record.EventType())
		}
	}
	if len(samples) == 0 {
		return nil
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQL batch transaction: %w", err)
	}
	defer tx.Rollback()
	qtx := m.queries.WithTx(tx)

	for _, sample := range samples {
		result, err := qtx.InsertTempSample(ctx, database.InsertTempSampleParams{
			Timestamp:    sample.Timestamp,
			SensorType:   sample.SensorType,
			SensorNumber: int64(sample.SensorNumber),
			FanOn:        sample.FanOn,
			Temperature:  sample.Temperature,
			HeaterPower:  sample.HeaterPower,
		})
		if err != nil {
			return fmt.Errorf("insert temperature sample: %w", err)
		}

		tempSampleID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("read temperature sample id: %w", err)
		}

		for _, label := range sample.AnomalyLabels() {
			if err := qtx.InsertTempSampleAnomaly(ctx, database.InsertTempSampleAnomalyParams{
				TempSampleID: tempSampleID,
				AnomalyLabel: label,
				CreatedAt:    sample.Timestamp,
			}); err != nil {
				return fmt.Errorf("insert temperature anomaly %q: %w", label, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQL batch transaction: %w", err)
	}

	return nil
}

// description: Applies the configured sampling policy to normal events while always keeping anomalies.
// input: sample (temperature sample being considered for SQL persistence).
// output: Returns true when the sample should be written to SQL.
func (m *ModbusLoop) shouldPersistNormalSample(sample TempSample) bool {
	if len(sample.AnomalyLabels()) > 0 {
		return true
	}
	if m.sqlNormalSampleRate <= 1 {
		return true
	}

	m.sqlNormalSampleCount++
	return m.sqlNormalSampleCount%m.sqlNormalSampleRate == 0
}

func eventTempSample(event fanoutEvent) (TempSample, error) {
	payload := event.record.Payload()
	switch sample := payload.(type) {
	case TempSample:
		return sample, nil
	case *TempSample:
		if sample == nil {
			return TempSample{}, fmt.Errorf("temperature payload is nil")
		}
		return *sample, nil
	default:
		return TempSample{}, fmt.Errorf("temperature payload has unexpected type %T", payload)
	}
}

// description: Publishes one event to the websocket stream server for frontend display.
// input: context and fanout event.
// output: Returns nil after enqueueing to the stream server when configured.
func (m *ModbusLoop) deliverToWebsocket(_ context.Context, event fanoutEvent) error {
	if m.streamer == nil {
		return nil
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	if sample, err := eventTempSample(event); err == nil {
		timestamp = sample.Timestamp
	}
	m.streamer.PublishEvent(event.record.EventType(), event.payload, timestamp)
	return nil
}
