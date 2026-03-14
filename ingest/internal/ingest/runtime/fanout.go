package runtime

/*
Name: ingest/internal/ingest/runtime/fanout.go
Description: Runs the shared ML, SQL, and websocket sink workers behind the Pipeline.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-03-14
Revision History:
- 2026-02-15, Barrett Brown: Added base fanout worker skeleton for ML, SQL, and websocket sinks.
- 2026-02-24, Barrett Brown: Added ML fanout batching and request handling.
- 2026-02-28, Barrett Brown: Generalized fanout events and added SQL batching support.
- 2026-03-07, Barrett Brown: Refactored shared sink workers behind the runtime Pipeline.
- 2026-03-13, Barrett Brown: Added clearer sink worker and delivery comments.
- 2026-03-14, Barrett Brown: Restored richer pre-refactor prologue detail from main branch history.
- 2026-03-14, Barrett Brown: Updated runtime fanout during the ingest file structure refactor to use explicit events package imports.
- 2026-03-14, Barrett Brown: Routed websocket and ML fanout through service-scoped delivery paths.
Preconditions:
- Pipeline is initialized with its downstream channels and dependencies.
Acceptable Input Values/Types:
- Ingress events, ML API responses, and SQL persistence inputs.
Unacceptable Input Values/Types:
- Nil or uninitialized sink dependencies for active delivery paths.
- Reusing mutable payload data in multiple places without copy discipline.
Postconditions:
- Tries to deliver each accepted event to all configured sinks.
- Workers stop when context is canceled.
Return Values/Types:
- Sink worker methods return errors to callers or log them internally.
Error/Exception Conditions:
- Downstream HTTP, SQL, and JSON errors are surfaced or logged.
Side Effects:
- Starts goroutines, uses channels, writes to the database, ML API, and websocket stream.
Invariants:
- All sinks consume from the shared pipeline channel set.
- Sink enqueue and worker ownership remain separated by downstream destination.
Known Faults:
- SQL and ML batching still rebuild temporary slices and maps each flush.
- Some websocket and ML payload paths still do extra copying for safety.
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
	ingestevents "github.com/BarrettBr/eecs-582-capstone/internal/ingest/events"
)

type mlBatchRequest struct {
	EventType string `json:"event_type"`
	Samples   []any  `json:"samples"`
}

type mlBatchKey struct {
	ServiceName string
	EventType   string
}

// description: Starts the downstream sink workers owned by the shared pipeline.
// input: context used to stop the workers on shutdown.
// output: Starts goroutines for enabled sinks.
func (p *Pipeline) startWorkers(ctx context.Context) {
	if p.mlAPIURL != "" {
		go p.runMLSink(ctx)
	} else {
		log.Printf("Sink=ml disabled: ML_API_URL is empty")
	}
	go p.runSQLSink(ctx)
	go p.runSink(ctx, "websocket", p.wsCh, p.deliverToWebsocket)
}

// description: Runs one simple sink loop that handles events one at a time.
// input: context, sink name, source channel, and delivery handler.
// output: Returns when the context is canceled.
func (p *Pipeline) runSink(ctx context.Context, name string, sink <-chan IngressEvent, handler func(context.Context, IngressEvent) error) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-sink:
			if err := handler(ctx, event); err != nil {
				log.Printf("Sink=%s error source=%s: %v", name, event.SourceName, err)
			}
		}
	}
}

func (p *Pipeline) enqueue(ctx context.Context, name string, sink chan<- IngressEvent, event IngressEvent) error {
	select {
	case <-ctx.Done():
		log.Printf("Sink=%s canceled before enqueue event type=%s source=%s", name, event.Record.EventType(), event.SourceName)
		return ctx.Err()
	case sink <- event:
		return nil
	}
}

// description: Runs the ML batching worker for grouped event delivery.
// input: context used to stop the worker and flush remaining work.
// output: Returns when the context is canceled.
func (p *Pipeline) runMLSink(ctx context.Context) {
	batch := make([]IngressEvent, 0, p.mlBatchSize)
	ticker := time.NewTicker(p.mlBatchFlushInterval)
	defer ticker.Stop()

	flush := func(flushCtx context.Context) {
		if len(batch) == 0 {
			return
		}
		if err := p.deliverMLBatch(flushCtx, batch); err != nil {
			log.Printf("Sink=ml error: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush(context.Background())
			return
		case event := <-p.mlCh:
			batch = append(batch, event)
			if len(batch) >= p.mlBatchSize {
				flush(ctx)
			}
		case <-ticker.C:
			flush(ctx)
		}
	}
}

// description: Runs the SQL batching worker for grouped database writes.
// input: context used to stop the worker and flush remaining work.
// output: Returns when the context is canceled.
func (p *Pipeline) runSQLSink(ctx context.Context) {
	batch := make([]IngressEvent, 0, p.sqlBatchSize)
	ticker := time.NewTicker(p.sqlBatchFlushInterval)
	defer ticker.Stop()

	flush := func(flushCtx context.Context) {
		if len(batch) == 0 {
			return
		}
		if err := p.deliverSQLBatch(flushCtx, batch); err != nil {
			log.Printf("Sink=sql error: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush(context.Background())
			return
		case event := <-p.sqlCh:
			batch = append(batch, event)
			if len(batch) >= p.sqlBatchSize {
				flush(ctx)
			}
		case <-ticker.C:
			flush(ctx)
		}
	}
}

// description: Groups ML enabled events by service and type and posts each batch.
// input: context plus one batch of ingress events.
// output: Returns nil when all type batches are delivered successfully.
func (p *Pipeline) deliverMLBatch(ctx context.Context, batch []IngressEvent) error {
	if len(batch) == 0 || p.mlAPIURL == "" {
		return nil
	}

	grouped := make(map[mlBatchKey][]any)
	for _, event := range batch {
		if !event.MLEnabled || event.SourceName == "" {
			continue
		}
		key := mlBatchKey{
			ServiceName: event.SourceName,
			EventType:   event.Record.EventType(),
		}
		grouped[key] = append(grouped[key], event.Record.Payload())
	}

	for key, samples := range grouped {
		if len(samples) == 0 {
			continue
		}
		if err := p.deliverMLTypeBatch(ctx, key.ServiceName, key.EventType, samples); err != nil {
			return err
		}
	}

	return nil
}

// description: Sends one service scoped typed ML batch to the configured ML service and normalizes the response.
// input: context, service name, event type name, and typed sample payloads.
// output: Returns nil when the ML request and response handling succeed.
func (p *Pipeline) deliverMLTypeBatch(ctx context.Context, serviceName, eventType string, samples []any) error {
	body, err := json.Marshal(mlBatchRequest{
		EventType: eventType,
		Samples:   samples,
	})
	if err != nil {
		return fmt.Errorf("ML request encode (%s): %w", eventType, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.mlAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ML request build (%s): %w", eventType, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.mlHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("ML request failed (%s): %w", eventType, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ML response read (%s): %w", eventType, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return fmt.Errorf("ML response status=%d event_type=%s body=%q", resp.StatusCode, eventType, msg)
	}

	trimmed := bytes.TrimSpace(respBody)
	if len(trimmed) == 0 {
		p.mlLastResponse = p.mlLastResponse[:0]
		return nil
	}

	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return fmt.Errorf("ML response json parse (%s): %w", eventType, err)
	}

	normalized := normalizeMLAnomalyPayload(serviceName, eventType, parsed, trimmed)
	normalizedBody, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("ML normalized response encode (%s): %w", eventType, err)
	}
	p.mlLastResponse = append(p.mlLastResponse[:0], normalizedBody...)
	if !normalized.HasAnomaly {
		return nil
	}
	if p.streamer != nil {
		p.streamer.PublishScopedMLResult(eventType, serviceName, normalizedBody)
	}
	return nil
}

// description: Writes one batch of ingest events into the SQL database inside one transaction.
// input: context and one slice of ingress events.
// output: Returns nil when the transaction commits successfully.
func (p *Pipeline) deliverSQLBatch(ctx context.Context, batch []IngressEvent) error {
	if len(batch) == 0 {
		return nil
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQL batch transaction: %w", err)
	}
	defer tx.Rollback()
	qtx := p.queries.WithTx(tx)

	// Persist each event type into the matching database tables.
	for _, event := range batch {
		switch event.Record.EventType() {
		case "temperature":
			sample, err := ingressEventTempSample(event)
			if err != nil {
				return err
			}
			if !p.shouldPersistNormalSample(sample) {
				continue
			}
			if err := persistTempSample(ctx, qtx, sample); err != nil {
				return err
			}
		case "valve":
			sample, err := ingressEventValveSample(event)
			if err != nil {
				return err
			}
			if err := persistValveSample(ctx, qtx, sample); err != nil {
				return err
			}
		default:
			log.Printf("Sink=sql skipped unsupported event type=%s", event.Record.EventType())
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQL batch transaction: %w", err)
	}
	return nil
}

func persistTempSample(ctx context.Context, qtx *database.Queries, sample ingestevents.TempSample) error {
	timestampUnix := sampleUnixTimestamp(sample.TimestampMs, sample.Timestamp)
	result, err := qtx.InsertTempSample(ctx, database.InsertTempSampleParams{
		Timestamp:    timestampUnix,
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
	return nil
}

func persistValveSample(ctx context.Context, qtx *database.Queries, sample ingestevents.ValveSample) error {
	timestampUnix := sampleUnixTimestamp(sample.TimestampMs, sample.Timestamp)
	result, err := qtx.InsertValveSample(ctx, database.InsertValveSampleParams{
		Timestamp:   timestampUnix,
		SensorType:  sample.SensorType,
		ValveNumber: int64(sample.ValveNumber),
		IsOpen:      sample.IsOpen,
		FlowRate:    sample.FlowRate,
	})
	if err != nil {
		return fmt.Errorf("insert valve sample: %w", err)
	}

	valveSampleID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read valve sample id: %w", err)
	}

	for _, label := range sample.AnomalyLabels() {
		if err := qtx.InsertValveSampleAnomaly(ctx, database.InsertValveSampleAnomalyParams{
			ValveSampleID: valveSampleID,
			AnomalyLabel:  label,
			CreatedAt:     sample.Timestamp,
		}); err != nil {
			return fmt.Errorf("insert valve anomaly %q: %w", label, err)
		}
	}
	return nil
}

func (p *Pipeline) shouldPersistNormalSample(sample ingestevents.TempSample) bool {
	if len(sample.AnomalyLabels()) > 0 {
		return true
	}
	if p.sqlNormalSampleRate <= 1 {
		return true
	}
	if p.sqlNormalSampleCount == nil {
		p.sqlNormalSampleCount = make(map[string]int)
	}

	p.sqlNormalSampleCount[sample.EventType()]++
	return p.sqlNormalSampleCount[sample.EventType()]%p.sqlNormalSampleRate == 0
}

func ingressEventTempSample(event IngressEvent) (ingestevents.TempSample, error) {
	switch sample := event.Record.Payload().(type) {
	case ingestevents.TempSample:
		return sample, nil
	case *ingestevents.TempSample:
		if sample == nil {
			return ingestevents.TempSample{}, fmt.Errorf("temperature payload is nil")
		}
		return *sample, nil
	default:
		return ingestevents.TempSample{}, fmt.Errorf("temperature payload has unexpected type %T", event.Record.Payload())
	}
}

func ingressEventValveSample(event IngressEvent) (ingestevents.ValveSample, error) {
	switch sample := event.Record.Payload().(type) {
	case ingestevents.ValveSample:
		return sample, nil
	case *ingestevents.ValveSample:
		if sample == nil {
			return ingestevents.ValveSample{}, fmt.Errorf("valve payload is nil")
		}
		return *sample, nil
	default:
		return ingestevents.ValveSample{}, fmt.Errorf("valve payload has unexpected type %T", event.Record.Payload())
	}
}

// description: Marshals one event payload for websocket delivery and publishes it to the stream server.
// input: context and one normalized ingress event.
// output: Returns nil when the event is accepted by the stream publisher.
func (p *Pipeline) deliverToWebsocket(_ context.Context, event IngressEvent) error {
	if p.streamer == nil {
		return nil
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	switch event.Record.EventType() {
	case "temperature":
		if sample, err := ingressEventTempSample(event); err == nil {
			timestamp = sample.Timestamp
		}
	case "valve":
		if sample, err := ingressEventValveSample(event); err == nil {
			timestamp = sample.Timestamp
		}
	}

	payload, err := json.Marshal(event.Record.Payload())
	if err != nil {
		return fmt.Errorf("encode websocket payload: %w", err)
	}

	p.streamer.PublishScopedEvent(event.Record.EventType(), event.SourceName, payload, timestamp)
	return nil
}

func sampleUnixTimestamp(timestampMs int64, timestampRFC3339 string) int64 {
	if timestampMs > 0 {
		return timestampMs / 1000
	}
	if timestampRFC3339 != "" {
		parsed, err := time.Parse(time.RFC3339Nano, timestampRFC3339)
		if err == nil {
			return parsed.Unix()
		}
	}
	return time.Now().UTC().Unix()
}
