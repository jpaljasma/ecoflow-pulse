package ingestworker

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const defaultAutoscaleMetricsWindow = 128

type AutoscaleMetrics struct {
	registry *prometheus.Registry

	unassignedActiveDevices prometheus.Gauge
	reconcileDurationP95    prometheus.Gauge
	leaseAcquireLatencyP95  prometheus.Gauge
	pollIntervalSeconds     prometheus.Gauge

	reconcileWindow *rollingP95
	leaseWindow     *rollingP95
}

func NewAutoscaleMetrics() *AutoscaleMetrics {
	registry := prometheus.NewRegistry()
	m := &AutoscaleMetrics{
		registry: registry,
		unassignedActiveDevices: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ingest_unassigned_active_devices",
			Help: "Current observed count of active ingest assignments not running on this worker after reconcile.",
		}),
		reconcileDurationP95: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ingest_reconcile_duration_p95_seconds",
			Help: "Rolling p95 reconcile duration in seconds for the ingest worker assignment loop.",
		}),
		leaseAcquireLatencyP95: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ingest_lease_acquire_latency_p95_seconds",
			Help: "Rolling p95 lease acquire latency in seconds for ingest assignment claims.",
		}),
		pollIntervalSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ingest_poll_interval_seconds",
			Help: "Configured ingest assignment poll interval in seconds.",
		}),
		reconcileWindow: newRollingP95(defaultAutoscaleMetricsWindow),
		leaseWindow:     newRollingP95(defaultAutoscaleMetricsWindow),
	}
	registry.MustRegister(
		m.unassignedActiveDevices,
		m.reconcileDurationP95,
		m.leaseAcquireLatencyP95,
		m.pollIntervalSeconds,
	)
	return m
}

func (m *AutoscaleMetrics) Registry() *prometheus.Registry {
	if m == nil {
		return nil
	}
	return m.registry
}

func (m *AutoscaleMetrics) SetPollInterval(interval time.Duration) {
	if m == nil {
		return
	}
	m.pollIntervalSeconds.Set(interval.Seconds())
}

func (m *AutoscaleMetrics) SetUnassignedActiveDevices(count int) {
	if m == nil {
		return
	}
	if count < 0 {
		count = 0
	}
	m.unassignedActiveDevices.Set(float64(count))
}

func (m *AutoscaleMetrics) ObserveReconcileDuration(duration time.Duration) {
	if m == nil {
		return
	}
	m.reconcileDurationP95.Set(m.reconcileWindow.Record(duration.Seconds()))
}

func (m *AutoscaleMetrics) ObserveLeaseAcquireLatency(duration time.Duration) {
	if m == nil {
		return
	}
	m.leaseAcquireLatencyP95.Set(m.leaseWindow.Record(duration.Seconds()))
}

type rollingP95 struct {
	mu      sync.Mutex
	values  []float64
	next    int
	count   int
	scratch []float64
}

func newRollingP95(size int) *rollingP95 {
	if size <= 0 {
		size = defaultAutoscaleMetricsWindow
	}
	return &rollingP95{
		values:  make([]float64, size),
		scratch: make([]float64, 0, size),
	}
}

func (r *rollingP95) Record(value float64) float64 {
	if r == nil {
		return 0
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		value = 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.values[r.next] = value
	if r.count < len(r.values) {
		r.count++
	}
	r.next = (r.next + 1) % len(r.values)

	r.scratch = r.scratch[:0]
	r.scratch = append(r.scratch, r.values[:r.count]...)
	sort.Float64s(r.scratch)
	idx := int(math.Ceil(0.95*float64(len(r.scratch)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(r.scratch) {
		idx = len(r.scratch) - 1
	}
	return r.scratch[idx]
}
