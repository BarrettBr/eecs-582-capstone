package runtime

/*
Name: ingest/internal/ingest/runtime/ml_anomaly.go
Description: Defines a stable ML anomaly payload shape and helpers to normalize ML responses into it.
Programmer: Barrett Brown
Date Created: 2026-03-01
Dates Revised: 2026-03-14
Revision History:
- 2026-03-01, Barrett Brown: Created consistent ML anomaly payload helpers for frontend forwarding.
- 2026-03-14, Barrett Brown: Added service identity to normalized ML anomaly payloads for service-room routing.
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
	Schema            string          `json:"schema"`
	ServiceName       string          `json:"service_name,omitempty"`
	EventType         string          `json:"event_type"`
	Model             string          `json:"model,omitempty"`
	GeneratedAt       string          `json:"generated_at"`
	GeneratedAtMs     int64           `json:"generated_at_ms"`
	HasAnomaly        bool            `json:"has_anomaly"`
	Severity          string          `json:"severity,omitempty"`
	Labels            []string        `json:"labels"`
	Confidence        *float64        `json:"confidence,omitempty"`
	ProbableCause     string          `json:"probable_cause,omitempty"`
	RecommendedAction string          `json:"recommended_action,omitempty"`
	Score             *float64        `json:"score,omitempty"`
	RawResponse       json.RawMessage `json:"raw_response"`
}

func normalizeMLAnomalyPayload(serviceName, eventType string, parsed any, raw []byte) MLAnomalyPayload {
	now := time.Now().UTC()

	payload := MLAnomalyPayload{
		Schema:        "ml_anomaly_v1",
		ServiceName:   serviceName,
		EventType:     eventType,
		GeneratedAt:   now.Format(time.RFC3339Nano),
		GeneratedAtMs: now.UnixMilli(),
		Labels:        make([]string, 0),
		RawResponse:   append([]byte(nil), raw...),
	}

	score := extractScore(parsed)
	if score != nil {
		payload.Score = score
	}
	confidence := extractConfidence(parsed)
	if confidence != nil {
		payload.Confidence = confidence
	}
	payload.Severity = extractSeverity(parsed)
	payload.ProbableCause = extractStringField(parsed, "probable_cause", "cause", "summary")
	payload.RecommendedAction = extractStringField(parsed, "recommended_action", "action", "recommendation")
	payload.Model = extractStringField(parsed, "model", "model_name", "type")
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
			case "score", "anomaly_score":
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

func extractConfidence(value any) *float64 {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			switch strings.ToLower(key) {
			case "confidence":
				if num, ok := asFloat64(item); ok {
					confidence := num
					return &confidence
				}
			default:
				if nested := extractConfidence(item); nested != nil {
					return nested
				}
			}
		}
	case []any:
		for _, item := range v {
			if nested := extractConfidence(item); nested != nil {
				return nested
			}
		}
	}
	return nil
}

func extractSeverity(value any) string {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			switch strings.ToLower(key) {
			case "severity":
				if s, ok := item.(string); ok {
					return strings.TrimSpace(s)
				}
			default:
				if nested := extractSeverity(item); nested != "" {
					return nested
				}
			}
		}
	case []any:
		for _, item := range v {
			if nested := extractSeverity(item); nested != "" {
				return nested
			}
		}
	}
	return ""
}

func extractStringField(value any, names ...string) string {
	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameSet[strings.ToLower(name)] = struct{}{}
	}
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if _, ok := nameSet[strings.ToLower(key)]; ok {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
			if nested := extractStringField(item, names...); nested != "" {
				return nested
			}
		}
	case []any:
		for _, item := range v {
			if nested := extractStringField(item, names...); nested != "" {
				return nested
			}
		}
	}
	return ""
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

func asFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}
