package projectionworker

import (
	"math"
	"testing"
)

func TestExtractNumericMetrics(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"id": 12,
		"typeCode": "pdStatus",
		"params": {
			"wattsInSum": 138.5,
			"XT150Watts2": -99,
			"pvStates": [1, 0, true, false],
			"nested": {"a": 1, "b": "x"}
		},
		"ok": true
	}`)
	got := extractNumericMetrics(payload)
	if got == nil {
		t.Fatalf("expected metrics map")
	}
	expect := map[string]float64{
		"id":                 12,
		"params.wattsInSum":  138.5,
		"params.XT150Watts2": -99,
		"params.pvStates.0":  1,
		"params.pvStates.1":  0,
		"params.pvStates.2":  1,
		"params.pvStates.3":  0,
		"params.nested.a":    1,
		"ok":                 1,
	}
	for key, want := range expect {
		gotVal, ok := got[key]
		if !ok {
			t.Fatalf("missing key %q in metrics: %v", key, got)
		}
		if math.Abs(gotVal-want) > 1e-9 {
			t.Fatalf("metric mismatch for %q: got=%v want=%v", key, gotVal, want)
		}
	}
	if _, exists := got["params.nested.b"]; exists {
		t.Fatalf("string metric should not be projected")
	}
}
