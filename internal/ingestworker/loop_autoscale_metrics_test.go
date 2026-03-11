package ingestworker

import (
	"context"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

func TestLoopReconcileUpdatesAutoscaleMetrics(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	store.set([]controlplane.IngestAssignment{
		{
			Provider:           controlplane.ProviderEcoFlow,
			ProviderDeviceID:   "DEMOD2M00001057",
			DeviceIsActive:     true,
			CredentialIsActive: true,
			IngestDesiredState: "active",
		},
	})
	metrics := NewAutoscaleMetrics()
	loop, err := NewLoop(testLogger(), store, &fakeLeaseManager{denyAcquire: true}, &fakeSessionRunner{}, Config{
		WorkerID:         "worker-test",
		PollInterval:     4 * time.Second,
		PollJitter:       0,
		StopTimeout:      2 * time.Second,
		StartWorkers:     1,
		AutoscaleMetrics: metrics,
	})
	if err != nil {
		t.Fatalf("NewLoop error: %v", err)
	}
	metrics.SetPollInterval(loop.cfg.PollInterval)

	if err := loop.reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile() error = %v", err)
	}

	families, err := metrics.Registry().Gather()
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
	if got["ingest_unassigned_active_devices"] != 1 {
		t.Fatalf("ingest_unassigned_active_devices=%v want=1", got["ingest_unassigned_active_devices"])
	}
	if got["ingest_poll_interval_seconds"] != 4 {
		t.Fatalf("ingest_poll_interval_seconds=%v want=4", got["ingest_poll_interval_seconds"])
	}
	if got["ingest_reconcile_duration_p95_seconds"] <= 0 {
		t.Fatalf("expected reconcile p95 > 0, got=%v", got["ingest_reconcile_duration_p95_seconds"])
	}
	if got["ingest_lease_acquire_latency_p95_seconds"] <= 0 {
		t.Fatalf("expected lease acquire p95 > 0, got=%v", got["ingest_lease_acquire_latency_p95_seconds"])
	}
}
