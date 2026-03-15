package main

import (
	"testing"
	"time"

	ingestruntime "github.com/BarrettBr/eecs-582-capstone/internal/ingest/runtime"
)

func TestBuildServiceRatesPayloadComputesPerServiceDeltaRate(t *testing.T) {
	snapshot := ingestruntime.SystemStatusSnapshot{
		Services: []ingestruntime.ServiceStatus{
			{Name: "svc_a", AdmittedEvents: 50},
			{Name: "svc_b", AdmittedEvents: 5},
		},
	}

	payload := buildServiceRatesPayload(snapshot, map[string]uint64{
		"svc_a": 0,
		"svc_b": 0,
	}, 5*time.Second)

	if payload.IntervalSeconds != 5 {
		t.Fatalf("IntervalSeconds = %d, want 5", payload.IntervalSeconds)
	}
	if len(payload.Services) != 2 {
		t.Fatalf("len(payload.Services) = %d, want 2", len(payload.Services))
	}
	if payload.Services[0].Name != "svc_a" || payload.Services[0].AdmittedEPS5s != 10 {
		t.Fatalf("payload.Services[0] = %+v, want svc_a at 10 eps", payload.Services[0])
	}
	if payload.Services[1].Name != "svc_b" || payload.Services[1].AdmittedEPS5s != 1 {
		t.Fatalf("payload.Services[1] = %+v, want svc_b at 1 eps", payload.Services[1])
	}
}
