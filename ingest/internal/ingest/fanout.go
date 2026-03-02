package ingest

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

type fanoutEvent struct {
	sourceName string
	mlEnabled  bool
	record     RecordEvent
	payload    []byte
}

type mlBatchRequest struct {
	EventType string `json:"event_type"`
	Samples   []any  `json:"samples"`
}

func (m *ModbusLoop) startFanoutWorkers(ctx context.Context) {
	if m.mlAPIURL != "" {
		go m.runMLSink(ctx)
	} else {
		log.Printf("Sink=ml disabled: ML_API_URL is empty")
	}
	go m.runSQLSink(ctx)
	go m.runSink(ctx, "websocket", m.wsCh, m.deliverToWebsocket)
}

func (m *ModbusLoop) fanOut(ctx context.Context, event fanoutEvent) {
	m.enqueueML(ctx, event)
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
				log.Printf("Sink=%s error source=%s: %v", name, event.sourceName, err)
			}
		}
	}
}

func (m *ModbusLoop) enqueue(name string, sink chan<- fanoutEvent, event fanoutEvent) {
	select {
	case sink <- event:
	default:
		log.Printf("Sink=%s dropped event type=%s source=%s", name, event.record.EventType(), event.sourceName)
	}
}

func (m *ModbusLoop) enqueueML(ctx context.Context, event fanoutEvent) {
	if m.mlAPIURL == "" || !event.mlEnabled {
		return
	}
	if m.mlDropOnOverload {
		m.enqueue("ml", m.mlCh, event)
		return
	}

	select {
	case <-ctx.Done():
		log.Printf("Sink=ml canceled before enqueue event type=%s source=%s", event.record.EventType(), event.sourceName)
	case m.mlCh <- event:
	}
}

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

func (m *ModbusLoop) deliverMLBatch(ctx context.Context, batch []fanoutEvent) error {
	if len(batch) == 0 || m.mlAPIURL == "" {
		return nil
	}

	grouped := make(map[string][]any)
	for _, event := range batch {
		if !event.mlEnabled {
			continue
		}
		grouped[event.record.EventType()] = append(grouped[event.record.EventType()], event.record.Payload())
	}

	for eventType, samples := range grouped {
		if len(samples) == 0 {
			continue
		}
		if err := m.deliverMLTypeBatch(ctx, eventType, samples); err != nil {
			return err
		}
	}

	return nil
}

func (m *ModbusLoop) deliverMLTypeBatch(ctx context.Context, eventType string, samples []any) error {
	body, err := json.Marshal(mlBatchRequest{
		EventType: eventType,
		Samples:   samples,
	})
	if err != nil {
		return fmt.Errorf("ML request encode (%s): %w", eventType, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.mlAPIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ML request build (%s): %w", eventType, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.mlHTTP.Do(req)
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
		m.mlLastResponse = m.mlLastResponse[:0]
		return nil
	}

	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return fmt.Errorf("ML response json parse (%s): %w", eventType, err)
	}

	normalized := normalizeMLAnomalyPayload(eventType, parsed, trimmed)
	normalizedBody, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("ML normalized response encode (%s): %w", eventType, err)
	}
	m.mlLastResponse = append(m.mlLastResponse[:0], normalizedBody...)
	if !normalized.HasAnomaly {
		return nil
	}
	if m.streamer != nil {
		m.streamer.PublishMLResult(eventType, normalizedBody)
	}
	return nil
}

func (m *ModbusLoop) deliverToSQL(ctx context.Context, event fanoutEvent) error {
	return m.deliverSQLBatch(ctx, []fanoutEvent{event})
}

func (m *ModbusLoop) deliverSQLBatch(ctx context.Context, batch []fanoutEvent) error {
	if len(batch) == 0 {
		return nil
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQL batch transaction: %w", err)
	}
	defer tx.Rollback()
	qtx := m.queries.WithTx(tx)

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
			if err := persistTempSample(ctx, qtx, sample); err != nil {
				return err
			}
		case "valve":
			sample, err := eventValveSample(event)
			if err != nil {
				return err
			}
			if err := persistValveSample(ctx, qtx, sample); err != nil {
				return err
			}
		default:
			log.Printf("Sink=sql skipped unsupported event type=%s", event.record.EventType())
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQL batch transaction: %w", err)
	}
	return nil
}

func persistTempSample(ctx context.Context, qtx *database.Queries, sample TempSample) error {
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
	return nil
}

func persistValveSample(ctx context.Context, qtx *database.Queries, sample ValveSample) error {
	result, err := qtx.InsertValveSample(ctx, database.InsertValveSampleParams{
		Timestamp:   sample.Timestamp,
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

func (m *ModbusLoop) shouldPersistNormalSample(sample TempSample) bool {
	if len(sample.AnomalyLabels()) > 0 {
		return true
	}
	if m.sqlNormalSampleRate <= 1 {
		return true
	}
	if m.sqlNormalSampleCount == nil {
		m.sqlNormalSampleCount = make(map[string]int)
	}

	m.sqlNormalSampleCount[sample.EventType()]++
	return m.sqlNormalSampleCount[sample.EventType()]%m.sqlNormalSampleRate == 0
}

func eventTempSample(event fanoutEvent) (TempSample, error) {
	switch sample := event.record.Payload().(type) {
	case TempSample:
		return sample, nil
	case *TempSample:
		if sample == nil {
			return TempSample{}, fmt.Errorf("temperature payload is nil")
		}
		return *sample, nil
	default:
		return TempSample{}, fmt.Errorf("temperature payload has unexpected type %T", event.record.Payload())
	}
}

func eventValveSample(event fanoutEvent) (ValveSample, error) {
	switch sample := event.record.Payload().(type) {
	case ValveSample:
		return sample, nil
	case *ValveSample:
		if sample == nil {
			return ValveSample{}, fmt.Errorf("valve payload is nil")
		}
		return *sample, nil
	default:
		return ValveSample{}, fmt.Errorf("valve payload has unexpected type %T", event.record.Payload())
	}
}

func (m *ModbusLoop) deliverToWebsocket(_ context.Context, event fanoutEvent) error {
	if m.streamer == nil {
		return nil
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	switch event.record.EventType() {
	case "temperature":
		if sample, err := eventTempSample(event); err == nil {
			timestamp = sample.Timestamp
		}
	case "valve":
		if sample, err := eventValveSample(event); err == nil {
			timestamp = sample.Timestamp
		}
	}

	m.streamer.PublishEvent(event.record.EventType(), event.payload, timestamp)
	return nil
}
