package ingest

/*
Name: ingest/internal/ingest/ml_anomaly.go
Description: Defines a stable ML anomaly payload shape and helpers to normalize ML responses into it.
Programmer: Barrett Brown
Date Created: 2026-03-01
Dates Revised: 2026-03-01
Revision History:
- 2026-03-01, Barrett Brown: Created consistent ML anomaly payload helpers for frontend forwarding.
Preconditions:
- Raw ML response is valid JSON after HTTP success.
Acceptable Input Values/Types:
- Parsed JSON objects, arrays, or scalar values.
Unacceptable Input Values/Types:
- Nil values are allowed but will produce a default non anomalous payload.
Postconditions:
- Produces a stable ML anomaly payload shape for the stream and frontend.
Return Values/Types:
- normalizeMLAnomalyPayload: MLAnomalyPayload
Error/Exception Conditions:
- No direct errors from normalization helpers.
Side Effects:
- None.
Invariants:
- Output payload always uses the same top level schema.
Known Faults:
- Heuristic field detection may not capture every ML model response format.
*/

import (
	"encoding/json"
	"strings"
	"time"
)

// MLAnomalyPayload is the stable payload shape sent to the frontend for ML results.
type MLAnomalyPayload struct {
	Schema      string          `json:"schema"`
	EventType   string          `json:"event_type"`
	GeneratedAt string          `json:"generated_at"`
	HasAnomaly  bool            `json:"has_anomaly"`
	Labels      []string        `json:"labels"`
	Score       *float64        `json:"score,omitempty"`
	RawResponse json.RawMessage `json:"raw_response"`
}

func normalizeMLAnomalyPayload(parsed any, raw []byte) MLAnomalyPayload {
	payload := MLAnomalyPayload{
		Schema:      "ml_anomaly_v1",
		EventType:   "temperature",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Labels:      make([]string, 0),
		RawResponse: append([]byte(nil), raw...),
	}

	score := extractScore(parsed)
	if score != nil {
		payload.Score = score
	}

	payload.Labels = extractLabels(parsed)
	if len(payload.Labels) > 0 {
		payload.HasAnomaly = true
		return payload
	}

	payload.HasAnomaly = extractAnomalyFlag(parsed)
	if payload.HasAnomaly && len(payload.Labels) == 0 {
		payload.Labels = []string{"ml_anomaly"}
	}

	return payload
}

func extractLabels(value any) []string {
	switch v := value.(type) {
	case map[string]any:
		var labels []string
		for key, item := range v {
			switch strings.ToLower(key) {
			case "label":
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					labels = append(labels, s)
				}
			case "labels", "anomalies", "anomaly_labels":
				if values, ok := item.([]any); ok {
					for _, rawLabel := range values {
						if s, ok := rawLabel.(string); ok && strings.TrimSpace(s) != "" {
							labels = append(labels, s)
						}
					}
				}
			default:
				labels = append(labels, extractLabels(item)...)
			}
		}
		return dedupeStrings(labels)
	case []any:
		var labels []string
		for _, item := range v {
			labels = append(labels, extractLabels(item)...)
		}
		return dedupeStrings(labels)
	default:
		return nil
	}
}

func extractAnomalyFlag(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			switch strings.ToLower(key) {
			case "has_anomaly", "is_anomaly", "anomaly", "anomalous":
				if b, ok := item.(bool); ok {
					return b
				}
				if num, ok := asFloat64(item); ok {
					return num != 0
				}
			}
			if extractAnomalyFlag(item) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if extractAnomalyFlag(item) {
				return true
			}
		}
	}
	return false
}

func extractScore(value any) *float64 {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			switch strings.ToLower(key) {
			case "score", "anomaly_score", "confidence":
				if num, ok := asFloat64(item); ok {
					score := num
					return &score
				}
			default:
				if nested := extractScore(item); nested != nil {
					return nested
				}
			}
		}
	case []any:
		for _, item := range v {
			if nested := extractScore(item); nested != nil {
				return nested
			}
		}
	}
	return nil
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
