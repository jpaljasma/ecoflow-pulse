package workermetrics

import (
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	registry *prometheus.Registry

	inflight         prometheus.Gauge
	messagesTotal    *prometheus.CounterVec
	processDuration  prometheus.Histogram
	flushTotal       *prometheus.CounterVec
	flushDuration    prometheus.Histogram
	lastFlushRecords prometheus.Gauge
	lastFlushBytes   prometheus.Gauge

	consumerRequired            atomic.Bool
	consumerReady               atomic.Bool
	consumerReadyGauge          prometheus.Gauge
	consumerSubscribeTotal      *prometheus.CounterVec
	consumerConsecutiveFailures prometheus.Gauge
}

func New(component string) *Metrics {
	registry := prometheus.NewRegistry()
	registerer := prometheus.WrapRegistererWith(prometheus.Labels{"component": component}, registry)
	m := &Metrics{
		registry: registry,
		inflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_worker_inflight_messages",
			Help: "Current number of in-flight worker message handlers.",
		}),
		messagesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_worker_messages_total",
			Help: "Total worker message outcomes by outcome.",
		}, []string{"outcome"}),
		processDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "pulse_worker_process_duration_seconds",
			Help:    "Worker message processing duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}),
		flushTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_worker_flush_total",
			Help: "Total worker flush outcomes by outcome.",
		}, []string{"outcome"}),
		flushDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "pulse_worker_flush_duration_seconds",
			Help:    "Worker flush duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}),
		lastFlushRecords: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_worker_last_flush_records",
			Help: "Number of records in the most recent flush.",
		}),
		lastFlushBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_worker_last_flush_bytes",
			Help: "Number of bytes in the most recent flush.",
		}),
		consumerReadyGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_worker_consumer_ready",
			Help: "Whether the worker's required NATS consumer subscription is currently attached.",
		}),
		consumerSubscribeTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_worker_consumer_subscribe_total",
			Help: "Total NATS consumer subscription outcomes by outcome.",
		}, []string{"outcome"}),
		consumerConsecutiveFailures: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_worker_consumer_consecutive_subscribe_failures",
			Help: "Consecutive NATS consumer subscription failures since the last successful attach.",
		}),
	}
	m.consumerReady.Store(true)
	m.consumerReadyGauge.Set(1)
	registerer.MustRegister(
		m.inflight,
		m.messagesTotal,
		m.processDuration,
		m.flushTotal,
		m.flushDuration,
		m.lastFlushRecords,
		m.lastFlushBytes,
		m.consumerReadyGauge,
		m.consumerSubscribeTotal,
		m.consumerConsecutiveFailures,
	)
	return m
}

func (m *Metrics) Registry() *prometheus.Registry {
	if m == nil {
		return nil
	}
	return m.registry
}

func (m *Metrics) StartMessage() func(string) {
	if m == nil {
		return func(string) {}
	}
	start := time.Now()
	m.inflight.Inc()
	return func(outcome string) {
		m.inflight.Dec()
		if outcome == "" {
			outcome = "unknown"
		}
		m.messagesTotal.WithLabelValues(outcome).Inc()
		m.processDuration.Observe(time.Since(start).Seconds())
	}
}

func (m *Metrics) ObserveFlush(outcome string, duration time.Duration, records int, bytes int) {
	if m == nil {
		return
	}
	if outcome == "" {
		outcome = "unknown"
	}
	m.flushTotal.WithLabelValues(outcome).Inc()
	m.flushDuration.Observe(duration.Seconds())
	m.lastFlushRecords.Set(float64(records))
	m.lastFlushBytes.Set(float64(bytes))
}

func (m *Metrics) RequireConsumerSubscription() {
	if m == nil {
		return
	}
	m.consumerRequired.Store(true)
	m.consumerReady.Store(false)
	m.consumerReadyGauge.Set(0)
}

func (m *Metrics) ObserveConsumerSubscribeFailure(string, string, error) {
	if m == nil {
		return
	}
	m.consumerRequired.Store(true)
	m.consumerReady.Store(false)
	m.consumerReadyGauge.Set(0)
	m.consumerSubscribeTotal.WithLabelValues("failure").Inc()
	m.consumerConsecutiveFailures.Inc()
}

func (m *Metrics) ObserveConsumerSubscribed(string, string) {
	if m == nil {
		return
	}
	m.consumerRequired.Store(true)
	m.consumerReady.Store(true)
	m.consumerReadyGauge.Set(1)
	m.consumerSubscribeTotal.WithLabelValues("success").Inc()
	m.consumerConsecutiveFailures.Set(0)
}

func (m *Metrics) ReadyStatus() (bool, string) {
	if m == nil {
		return true, "ok"
	}
	if m.consumerRequired.Load() && !m.consumerReady.Load() {
		return false, "consumer_not_subscribed"
	}
	return true, "ok"
}
