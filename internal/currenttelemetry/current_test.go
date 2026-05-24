package currenttelemetry

import "testing"

func TestIdleStaleDetectsImpossibleEcoFlowIdleCurrent(t *testing.T) {
	metrics := map[string]float64{
		"params.wattsInSum":    46,
		"params.wattsOutSum":   0,
		"params.bmsInputWatts": 0,
		"params.chgDsgState":   2,
		"params.remainTime":    5999,
	}

	if !IdleStale(metrics) {
		t.Fatal("expected idle stale current telemetry")
	}
}

func TestIdleStaleKeepsLiveTrickleWithBatterySink(t *testing.T) {
	metrics := map[string]float64{
		"params.wattsInSum":    8,
		"params.wattsOutSum":   0,
		"params.bmsInputWatts": 4,
		"params.chgDsgState":   2,
		"params.remainTime":    5999,
	}

	if IdleStale(metrics) {
		t.Fatal("expected live trickle telemetry to remain fresh")
	}
}

func TestIdleStaleRequiresObservedSinkMetrics(t *testing.T) {
	metrics := map[string]float64{
		"params.wattsInSum":  46,
		"params.chgDsgState": 2,
		"params.remainTime":  5999,
	}

	if IdleStale(metrics) {
		t.Fatal("expected classifier to avoid stale decision without load or battery sink evidence")
	}
}

func TestExtractNumericMetricsWalksNestedPayload(t *testing.T) {
	metrics := ExtractNumericMetrics([]byte(`{"params":{"wattsInSum":46,"enabled":true,"nested":{"remainTime":5999}}}`))

	if got := metrics["params.wattsInSum"]; got != 46 {
		t.Fatalf("params.wattsInSum=%v, want 46", got)
	}
	if got := metrics["params.enabled"]; got != 1 {
		t.Fatalf("params.enabled=%v, want 1", got)
	}
	if got := metrics["params.nested.remainTime"]; got != 5999 {
		t.Fatalf("params.nested.remainTime=%v, want 5999", got)
	}
}
