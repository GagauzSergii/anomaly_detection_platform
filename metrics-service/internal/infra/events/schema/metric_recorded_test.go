package schema

import (
	"encoding/json"
	"testing"
)

func TestMetricRecorded_JSONContract(t *testing.T) {
	dto := MetricRecordedDto{
		Source:     "orders-service",
		MetricName: "latency_p95",
		Value:      350.5,
		Env:        "prod",
		Region:     "eu-central-1",
		InstanceID: "pod-123",
		Timestamp:  1736448000,
	}

	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// 1) Checking, if keys are snake_case, not like MetricName/InstanceId etc.
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal(map) failed: %v", err)
	}

	wantKeys := []string{
		"source",
		"metric_name",
		"value",
		"env",
		"region",
		"instance_id",
		"timestamp",
	}

	for _, k := range wantKeys {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing json key %q in payload: %s", k, string(raw))
		}
	}

	// 2) Check round-trip: marshal -> unmarshal -> fields are not loss
	var got MetricRecordedDto
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal(struct) failed: %v", err)
	}

	if got != dto {
		t.Fatalf("round-trip mismatch:\nwant: %+v\ngot:  %+v\nraw:  %s", dto, got, string(raw))
	}

	// 3) Additional typing check (json.Unmarshal(map[string]any) float64 for numbers)
	if _, ok := m["timestamp"].(float64); !ok {
		t.Fatalf("timestamp should be number in JSON, got %T (%v)", m["timestamp"], m["timestamp"])
	}
	if _, ok := m["value"].(float64); !ok {
		t.Fatalf("value should be number in JSON, got %T (%v)", m["value"], m["value"])
	}
}
