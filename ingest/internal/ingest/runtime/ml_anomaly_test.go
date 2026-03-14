package runtime

/*
Name: ingest/internal/ingest/runtime/ml_anomaly_test.go
Description: Tests ML anomaly normalization so frontend payloads keep a stable schema.
Programmer: Barrett Brown
Date Created: 2026-03-01
Dates Revised: 2026-03-14
Revision History:
- 2026-03-01, Barrett Brown: Created tests for ML anomaly payload normalization.
- 2026-03-14, Barrett Brown: Added coverage for service-scoped ML anomaly normalization.
Preconditions:
- Parsed ML response values are available for helper testing.
Acceptable Input Values/Types:
- Maps and arrays with common anomaly fields.
Unacceptable Input Values/Types:
- No special runtime dependencies are needed for these tests.
Postconditions:
- Confirms normalized payload shape stays consistent.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Unexpected missing anomaly flags or labels.
Side Effects:
- None.
Invariants:
- Normalized payload schema should stay ml_anomaly_v1.
Known Faults:
- Does not cover every possible ML response layout.
*/

import "testing"

func TestNormalizeMLAnomalyPayload(t *testing.T) {
	raw := []byte(`{"is_anomaly":true,"label":"overheat","score":0.97}`)
	parsed := map[string]any{
		"is_anomaly": true,
		"label":      "overheat",
		"score":      0.97,
	}

	payload := normalizeMLAnomalyPayload("svc_temp", "temperature", parsed, raw)
	if payload.Schema != "ml_anomaly_v1" {
		t.Fatalf("Schema = %q, want %q", payload.Schema, "ml_anomaly_v1")
	}
	if !payload.HasAnomaly {
		t.Fatalf("HasAnomaly = false, want true")
	}
	if len(payload.Labels) != 1 || payload.Labels[0] != "overheat" {
		t.Fatalf("Labels = %v, want [overheat]", payload.Labels)
	}
	if payload.Score == nil || *payload.Score != 0.97 {
		t.Fatalf("Score = %v, want 0.97", payload.Score)
	}
	if string(payload.RawResponse) != string(raw) {
		t.Fatalf("RawResponse = %q, want %q", string(payload.RawResponse), string(raw))
	}
	if payload.EventType != "temperature" {
		t.Fatalf("EventType = %q, want %q", payload.EventType, "temperature")
	}
	if payload.ServiceName != "svc_temp" {
		t.Fatalf("ServiceName = %q, want %q", payload.ServiceName, "svc_temp")
	}
}

func TestNormalizeMLAnomalyPayloadFallsBackToDefaultLabel(t *testing.T) {
	raw := []byte(`{"anomaly":1}`)
	parsed := map[string]any{
		"anomaly": 1.0,
	}

	payload := normalizeMLAnomalyPayload("svc_valve", "valve", parsed, raw)
	if !payload.HasAnomaly {
		t.Fatalf("HasAnomaly = false, want true")
	}
	if len(payload.Labels) != 1 || payload.Labels[0] != "ml_anomaly" {
		t.Fatalf("Labels = %v, want [ml_anomaly]", payload.Labels)
	}
	if payload.EventType != "valve" {
		t.Fatalf("EventType = %q, want %q", payload.EventType, "valve")
	}
	if payload.ServiceName != "svc_valve" {
		t.Fatalf("ServiceName = %q, want %q", payload.ServiceName, "svc_valve")
	}
}

func TestNormalizeMLAnomalyPayloadExtractsNestedLabelsAndScoreOnce(t *testing.T) {
	raw := []byte(`{"result":{"labels":[" overheat ","overheat","pressure"],"score":0.88}}`)
	parsed := map[string]any{
		"result": map[string]any{
			"labels": []any{" overheat ", "overheat", "pressure"},
			"score":  0.88,
		},
	}

	payload := normalizeMLAnomalyPayload("svc_temp", "temperature", parsed, raw)
	if !payload.HasAnomaly {
		t.Fatalf("HasAnomaly = false, want true")
	}
	if len(payload.Labels) != 2 || payload.Labels[0] != "overheat" || payload.Labels[1] != "pressure" {
		t.Fatalf("Labels = %v, want [overheat pressure]", payload.Labels)
	}
	if payload.Score == nil || *payload.Score != 0.88 {
		t.Fatalf("Score = %v, want 0.88", payload.Score)
	}
}
