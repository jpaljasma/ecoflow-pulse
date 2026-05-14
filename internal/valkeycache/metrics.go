package valkeycache

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics exposes low-cardinality cache observations.
type Metrics struct {
	operations      *prometheus.CounterVec
	duration        *prometheus.HistogramVec
	payloadBytes    *prometheus.HistogramVec
	clientSideReads *prometheus.CounterVec
}

// NewMetrics registers shared cache metrics. Labels are bounded to
// namespace/operation/result and never include cache keys or user identifiers.
func NewMetrics(registerer prometheus.Registerer) *Metrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}
	return &Metrics{
		operations: registerOrReuseCounter(registerer, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_valkey_cache_operations_total",
			Help: "Total Valkey cache operations by namespace, operation, and result.",
		}, []string{"namespace", "operation", "result"})),
		duration: registerOrReuseHistogram(registerer, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_valkey_cache_operation_duration_seconds",
			Help:    "Valkey cache operation latency by namespace, operation, and result.",
			Buckets: prometheus.DefBuckets,
		}, []string{"namespace", "operation", "result"})),
		payloadBytes: registerOrReuseHistogram(registerer, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_valkey_cache_payload_bytes",
			Help:    "Valkey cache payload sizes by namespace, stage, codec, and encryption state.",
			Buckets: prometheus.ExponentialBuckets(128, 2, 18),
		}, []string{"namespace", "stage", "codec", "encrypted"})),
		clientSideReads: registerOrReuseCounter(registerer, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_valkey_cache_client_side_reads_total",
			Help: "Valkey client-side-cache read attempts and hits by namespace.",
		}, []string{"namespace", "result"})),
	}
}

func (m *Metrics) observeOperation(namespace, operation, result string, started time.Time) {
	if m == nil {
		return
	}
	m.operations.WithLabelValues(namespace, operation, result).Inc()
	m.duration.WithLabelValues(namespace, operation, result).Observe(time.Since(started).Seconds())
}

func (m *Metrics) observePayload(namespace, stage string, meta EnvelopeMeta, encrypted bool) {
	if m == nil {
		return
	}
	size := meta.StoredSize
	if stage == "raw" {
		size = meta.OriginalSize
	}
	m.payloadBytes.WithLabelValues(namespace, stage, string(meta.Codec), boolLabel(encrypted)).Observe(float64(size))
}

func (m *Metrics) observeClientSideRead(namespace, result string) {
	if m == nil {
		return
	}
	m.clientSideReads.WithLabelValues(namespace, result).Inc()
}

func registerOrReuseCounter(registerer prometheus.Registerer, collector *prometheus.CounterVec) *prometheus.CounterVec {
	if err := registerer.Register(collector); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := already.ExistingCollector.(*prometheus.CounterVec); ok {
				return existing
			}
		}
	}
	return collector
}

func registerOrReuseHistogram(registerer prometheus.Registerer, collector *prometheus.HistogramVec) *prometheus.HistogramVec {
	if err := registerer.Register(collector); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := already.ExistingCollector.(*prometheus.HistogramVec); ok {
				return existing
			}
		}
	}
	return collector
}

func boolLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
