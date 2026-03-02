package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHistoryLimit = 100
	maxHistoryLimit     = 1000
)

type HistoryAPI struct {
	db *sql.DB
}

func NewHistoryAPI(db *sql.DB) *HistoryAPI {
	return &HistoryAPI{db: db}
}

func (h *HistoryAPI) Register(register func(string, http.HandlerFunc)) {
	if register == nil {
		return
	}
	register("/api/history/rates", h.handleRates)
	register("/api/history/events", h.handleEvents)
}

func (h *HistoryAPI) handleRates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	eventType, tableName, _, err := parseEventType(r.URL.Query().Get("event_type"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	windowName := r.URL.Query().Get("window")
	windowDuration, defaultBucket, err := parseWindow(windowName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	bucketName := r.URL.Query().Get("bucket")
	if bucketName == "" {
		bucketName = defaultBucket
	}
	bucketDuration, err := parseBucket(windowName, bucketName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.buildRatesResponse(r.Context(), eventType, tableName, windowName, bucketName, windowDuration, bucketDuration)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *HistoryAPI) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	eventType, tableName, anomalyTable, err := parseEventType(r.URL.Query().Get("event_type"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rawCursor := r.URL.Query().Get("cursor")
	var cursor *historyCursor
	if rawCursor != "" {
		decoded, err := decodeCursor(rawCursor)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		cursor = &decoded
	}

	switch eventType {
	case "temperature":
		resp, err := h.buildTemperatureHistoryResponse(r.Context(), tableName, anomalyTable, limit, cursor)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)
	case "valve":
		resp, err := h.buildValveHistoryResponse(r.Context(), tableName, anomalyTable, limit, cursor)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)
	default:
		writeError(w, http.StatusBadRequest, "invalid event_type")
	}
}

func (h *HistoryAPI) buildRatesResponse(
	ctx context.Context,
	eventType string,
	tableName string,
	windowName string,
	bucketName string,
	windowDuration time.Duration,
	bucketDuration time.Duration,
) (ratesResponse, error) {
	now := time.Now().UTC()
	cutoff := now.Add(-windowDuration).Format(time.RFC3339Nano)
	bucketSeconds := int64(bucketDuration / time.Second)
	if bucketSeconds <= 0 {
		return ratesResponse{}, fmt.Errorf("bucket must be at least one second")
	}

	query := fmt.Sprintf(`
SELECT
	(CAST(strftime('%%s', timestamp) AS INTEGER) / ?) * ? AS bucket_start_unix,
	COUNT(*) AS bucket_count
FROM %s
WHERE timestamp >= ?
GROUP BY bucket_start_unix
ORDER BY bucket_start_unix ASC
`, tableName)

	rows, err := h.db.QueryContext(ctx, query, bucketSeconds, bucketSeconds, cutoff)
	if err != nil {
		return ratesResponse{}, fmt.Errorf("query rates: %w", err)
	}
	defer rows.Close()

	resp := ratesResponse{
		EventType:     eventType,
		Window:        windowName,
		Bucket:        bucketName,
		GeneratedAt:   now.Format(time.RFC3339Nano),
		GeneratedAtMs: now.UnixMilli(),
		Series:        make([]bucketPoint, 0),
	}

	for rows.Next() {
		var bucketStartUnix int64
		var bucketCount int64
		if err := rows.Scan(&bucketStartUnix, &bucketCount); err != nil {
			return ratesResponse{}, fmt.Errorf("scan rates row: %w", err)
		}
		resp.Series = append(resp.Series, bucketPoint{
			BucketStartMs: bucketStartUnix * 1000,
			BucketEndMs:   (bucketStartUnix + bucketSeconds) * 1000,
			Count:         bucketCount,
		})
		resp.TotalCount += bucketCount
	}
	if err := rows.Err(); err != nil {
		return ratesResponse{}, fmt.Errorf("iterate rates rows: %w", err)
	}

	return resp, nil
}

func (h *HistoryAPI) buildTemperatureHistoryResponse(
	ctx context.Context,
	tableName string,
	anomalyTable string,
	limit int,
	cursor *historyCursor,
) (historyResponse, error) {
	args := []any{}
	query := fmt.Sprintf(`
SELECT id, timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power
FROM %s
`, tableName)
	if cursor != nil {
		query += "WHERE (timestamp > ? OR (timestamp = ? AND id > ?))\n"
		args = append(args, cursor.Timestamp, cursor.Timestamp, cursor.ID)
	}
	query += "ORDER BY timestamp ASC, id ASC\nLIMIT ?"
	args = append(args, limit+1)

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return historyResponse{}, fmt.Errorf("query temperature history: %w", err)
	}
	defer rows.Close()

	items := make([]tempHistoryItem, 0, limit+1)
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var item tempHistoryItem
		if err := rows.Scan(
			&item.ID,
			&item.Timestamp,
			&item.SensorType,
			&item.SensorNumber,
			&item.FanOn,
			&item.Temperature,
			&item.HeaterPower,
		); err != nil {
			return historyResponse{}, fmt.Errorf("scan temperature history row: %w", err)
		}
		item.TimestampMs = timestampToMillis(item.Timestamp)
		item.Anomalies = []string{}
		items = append(items, item)
		ids = append(ids, item.ID)
	}
	if err := rows.Err(); err != nil {
		return historyResponse{}, fmt.Errorf("iterate temperature history rows: %w", err)
	}

	labels, err := lookupAnomalies(ctx, h.db, anomalyTable, "temp_sample_id", ids)
	if err != nil {
		return historyResponse{}, err
	}
	for index := range items {
		items[index].Anomalies = labels[items[index].ID]
		if items[index].Anomalies == nil {
			items[index].Anomalies = []string{}
		}
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor, err := nextCursorFromTemperature(items, hasMore)
	if err != nil {
		return historyResponse{}, err
	}

	return historyResponse{
		EventType:  "temperature",
		Limit:      limit,
		NextCursor: nextCursor,
		Items:      items,
	}, nil
}

func (h *HistoryAPI) buildValveHistoryResponse(
	ctx context.Context,
	tableName string,
	anomalyTable string,
	limit int,
	cursor *historyCursor,
) (historyResponse, error) {
	args := []any{}
	query := fmt.Sprintf(`
SELECT id, timestamp, sensor_type, valve_number, is_open, flow_rate
FROM %s
`, tableName)
	if cursor != nil {
		query += "WHERE (timestamp > ? OR (timestamp = ? AND id > ?))\n"
		args = append(args, cursor.Timestamp, cursor.Timestamp, cursor.ID)
	}
	query += "ORDER BY timestamp ASC, id ASC\nLIMIT ?"
	args = append(args, limit+1)

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return historyResponse{}, fmt.Errorf("query valve history: %w", err)
	}
	defer rows.Close()

	items := make([]valveHistoryItem, 0, limit+1)
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var item valveHistoryItem
		if err := rows.Scan(
			&item.ID,
			&item.Timestamp,
			&item.SensorType,
			&item.ValveNumber,
			&item.IsOpen,
			&item.FlowRate,
		); err != nil {
			return historyResponse{}, fmt.Errorf("scan valve history row: %w", err)
		}
		item.TimestampMs = timestampToMillis(item.Timestamp)
		item.Anomalies = []string{}
		items = append(items, item)
		ids = append(ids, item.ID)
	}
	if err := rows.Err(); err != nil {
		return historyResponse{}, fmt.Errorf("iterate valve history rows: %w", err)
	}

	labels, err := lookupAnomalies(ctx, h.db, anomalyTable, "valve_sample_id", ids)
	if err != nil {
		return historyResponse{}, err
	}
	for index := range items {
		items[index].Anomalies = labels[items[index].ID]
		if items[index].Anomalies == nil {
			items[index].Anomalies = []string{}
		}
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor, err := nextCursorFromValve(items, hasMore)
	if err != nil {
		return historyResponse{}, err
	}

	return historyResponse{
		EventType:  "valve",
		Limit:      limit,
		NextCursor: nextCursor,
		Items:      items,
	}, nil
}

func lookupAnomalies(ctx context.Context, db *sql.DB, tableName string, sampleColumn string, ids []int64) (map[int64][]string, error) {
	result := make(map[int64][]string)
	if len(ids) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids))
	for index, id := range ids {
		placeholders[index] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf(`
SELECT %s, anomaly_label
FROM %s
WHERE %s IN (%s)
ORDER BY id ASC
`, sampleColumn, tableName, sampleColumn, strings.Join(placeholders, ","))

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query anomalies: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sampleID int64
		var label string
		if err := rows.Scan(&sampleID, &label); err != nil {
			return nil, fmt.Errorf("scan anomalies row: %w", err)
		}
		result[sampleID] = append(result[sampleID], label)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate anomalies rows: %w", err)
	}

	return result, nil
}

func nextCursorFromTemperature(items []tempHistoryItem, hasMore bool) (*string, error) {
	if len(items) == 0 || !hasMore {
		return nil, nil
	}
	last := items[len(items)-1]
	encoded, err := encodeCursor(historyCursor{
		Timestamp: last.Timestamp,
		ID:        last.ID,
	})
	if err != nil {
		return nil, err
	}
	return &encoded, nil
}

func nextCursorFromValve(items []valveHistoryItem, hasMore bool) (*string, error) {
	if len(items) == 0 || !hasMore {
		return nil, nil
	}
	last := items[len(items)-1]
	encoded, err := encodeCursor(historyCursor{
		Timestamp: last.Timestamp,
		ID:        last.ID,
	})
	if err != nil {
		return nil, err
	}
	return &encoded, nil
}

func parseEventType(raw string) (string, string, string, error) {
	switch raw {
	case "temperature":
		return "temperature", "temp_samples", "temp_sample_anomalies", nil
	case "valve":
		return "valve", "valve_samples", "valve_sample_anomalies", nil
	default:
		return "", "", "", fmt.Errorf("invalid event_type")
	}
}

func parseWindow(raw string) (time.Duration, string, error) {
	switch raw {
	case "1h":
		return time.Hour, "5m", nil
	case "24h":
		return 24 * time.Hour, "1h", nil
	case "7d":
		return 7 * 24 * time.Hour, "6h", nil
	default:
		return 0, "", fmt.Errorf("invalid window")
	}
}

func parseBucket(windowRaw string, bucketRaw string) (time.Duration, error) {
	allowed := map[string]map[string]time.Duration{
		"1h": {
			"1m":  time.Minute,
			"5m":  5 * time.Minute,
			"15m": 15 * time.Minute,
		},
		"24h": {
			"15m": 15 * time.Minute,
			"1h":  time.Hour,
			"6h":  6 * time.Hour,
		},
		"7d": {
			"1h":  time.Hour,
			"6h":  6 * time.Hour,
			"24h": 24 * time.Hour,
		},
	}
	options, ok := allowed[windowRaw]
	if !ok {
		return 0, fmt.Errorf("invalid window")
	}
	duration, ok := options[bucketRaw]
	if !ok {
		return 0, fmt.Errorf("invalid bucket")
	}
	return duration, nil
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return defaultHistoryLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid limit")
	}
	if limit <= 0 {
		return 0, fmt.Errorf("invalid limit")
	}
	if limit > maxHistoryLimit {
		return maxHistoryLimit, nil
	}
	return limit, nil
}

func timestampToMillis(timestamp string) int64 {
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return 0
	}
	return parsed.UnixMilli()
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, `{"error":"encode response"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
