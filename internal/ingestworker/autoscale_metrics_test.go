package ingestworker

import (
	"testing"
	"time"
)

func TestRollingP95UsesLatestWindow(t *testing.T) {
	t.Parallel()

	r := newRollingP95(5)
	samples := []float64{1, 2, 3, 4, 5, 100}
	last := 0.0
	for _, sample := range samples {
		last = r.Record(sample)
	}
	// Latest window is [2,3,4,5,100], so p95 should resolve to the max sample.
	if last != 100 {
		t.Fatalf("rolling p95=%v want=100", last)
	}
}

func TestAutoscaleMetricsObserveAndClamp(t *testing.T) {
	t.Parallel()

	m := NewAutoscaleMetrics()
	m.SetPollInterval(4 * time.Second)
	m.SetUnassignedActiveDevices(-3)
	m.ObserveReconcileDuration(120 * time.Millisecond)
	m.ObserveLeaseAcquireLatency(45 * time.Millisecond)

	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	got := map[string]float64{}
	for _, family := range families {
		if len(family.GetMetric()) == 0 || family.GetMetric()[0].GetGauge() == nil {
			continue
		}
		got[family.GetName()] = family.GetMetric()[0].GetGauge().GetValue()
	}
	if got["ingest_unassigned_active_devices"] != 0 {
		t.Fatalf("unassigned active devices=%v want=0", got["ingest_unassigned_active_devices"])
	}
	if got["ingest_poll_interval_seconds"] != 4 {
		t.Fatalf("poll interval=%v want=4", got["ingest_poll_interval_seconds"])
	}
	if got["ingest_reconcile_duration_p95_seconds"] <= 0 {
		t.Fatalf("expected reconcile p95 > 0, got=%v", got["ingest_reconcile_duration_p95_seconds"])
	}
	if got["ingest_lease_acquire_latency_p95_seconds"] <= 0 {
		t.Fatalf("expected lease acquire p95 > 0, got=%v", got["ingest_lease_acquire_latency_p95_seconds"])
	}
}
