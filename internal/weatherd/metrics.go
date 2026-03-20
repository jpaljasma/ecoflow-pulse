package weatherd

import (
	"context"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/budget"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requests             *prometheus.CounterVec
	requestDuration      *prometheus.HistogramVec
	upstreamCalls        *prometheus.CounterVec
	upstreamDuration     *prometheus.HistogramVec
	budgetDenied         *prometheus.CounterVec
	budgetUnitsConsumed  *prometheus.CounterVec
	activeLocations      prometheus.Gauge
	dailyLimit           prometheus.Gauge
	dailyUsed            prometheus.Gauge
	dailyRemainingRatio  prometheus.Gauge
	minuteLimit          prometheus.Gauge
	minuteUsed           prometheus.Gauge
	minuteRemainingRatio prometheus.Gauge
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	if registerer == nil {
		return nil
	}
	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_weather_requests_total",
			Help: "Weather service request outcomes by method, source path, and result.",
		}, []string{"method", "source", "result"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_weather_request_duration_seconds",
			Help:    "Weather service request duration by method, source path, and result.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "source", "result"}),
		upstreamCalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_weather_upstream_calls_total",
			Help: "Weather upstream call outcomes by operation and result.",
		}, []string{"operation", "result"}),
		upstreamDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_weather_upstream_call_duration_seconds",
			Help:    "Weather upstream call duration by operation and result.",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation", "result"}),
		budgetDenied: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_weather_budget_denials_total",
			Help: "Weather upstream budget denials by window.",
		}, []string{"window"}),
		budgetUnitsConsumed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_weather_upstream_budget_units_consumed_total",
			Help: "Weather upstream budget units consumed by operation.",
		}, []string{"operation"}),
		activeLocations: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_weather_active_locations",
			Help: "Distinct active weather locations requested within the recent-active window.",
		}),
		dailyLimit: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_weather_upstream_budget_daily_limit_units",
			Help: "Configured daily upstream budget limit in weighted units.",
		}),
		dailyUsed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_weather_upstream_budget_daily_used_units",
			Help: "Current daily upstream budget usage in weighted units.",
		}),
		dailyRemainingRatio: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_weather_upstream_budget_daily_remaining_ratio",
			Help: "Remaining fraction of the daily upstream budget.",
		}),
		minuteLimit: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_weather_upstream_budget_minute_limit_units",
			Help: "Configured per-minute upstream budget limit in weighted units.",
		}),
		minuteUsed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_weather_upstream_budget_minute_used_units",
			Help: "Current per-minute upstream budget usage in weighted units.",
		}),
		minuteRemainingRatio: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_weather_upstream_budget_minute_remaining_ratio",
			Help: "Remaining fraction of the per-minute upstream budget.",
		}),
	}
	registerer.MustRegister(
		m.requests,
		m.requestDuration,
		m.upstreamCalls,
		m.upstreamDuration,
		m.budgetDenied,
		m.budgetUnitsConsumed,
		m.activeLocations,
		m.dailyLimit,
		m.dailyUsed,
		m.dailyRemainingRatio,
		m.minuteLimit,
		m.minuteUsed,
		m.minuteRemainingRatio,
	)
	return m
}

func (m *Metrics) ObserveRequest(method, source string, err error, duration time.Duration) {
	if m == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	m.requests.WithLabelValues(method, source, result).Inc()
	m.requestDuration.WithLabelValues(method, source, result).Observe(duration.Seconds())
}

func (m *Metrics) ObserveUpstreamCall(operation string, err error, duration time.Duration) {
	if m == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	m.upstreamCalls.WithLabelValues(operation, result).Inc()
	m.upstreamDuration.WithLabelValues(operation, result).Observe(duration.Seconds())
}

func (m *Metrics) ObserveBudgetDenied(window string) {
	if m == nil {
		return
	}
	if window == "" {
		window = "unknown"
	}
	m.budgetDenied.WithLabelValues(window).Inc()
}

func (m *Metrics) ObserveBudgetUnitsConsumed(operation string, units int) {
	if m == nil || units <= 0 {
		return
	}
	m.budgetUnitsConsumed.WithLabelValues(operation).Add(float64(units))
}

func (m *Metrics) ObserveBudgetSnapshot(snapshot budget.Snapshot) {
	if m == nil {
		return
	}
	m.dailyLimit.Set(float64(snapshot.DailyLimit))
	m.dailyUsed.Set(float64(snapshot.DailyUsed))
	m.dailyRemainingRatio.Set(remainingRatio(snapshot.DailyUsed, snapshot.DailyLimit))
	m.minuteLimit.Set(float64(snapshot.PerMinuteLimit))
	m.minuteUsed.Set(float64(snapshot.MinuteUsed))
	m.minuteRemainingRatio.Set(remainingRatio(snapshot.MinuteUsed, snapshot.PerMinuteLimit))
}

func (m *Metrics) ObserveActiveLocations(count int) {
	if m == nil {
		return
	}
	m.activeLocations.Set(float64(count))
}

func remainingRatio(used, limit int) float64 {
	if limit <= 0 {
		return 1
	}
	remaining := float64(limit-used) / float64(limit)
	switch {
	case remaining < 0:
		return 0
	case remaining > 1:
		return 1
	default:
		return remaining
	}
}

func (s *Service) UpdateMetrics(ctx context.Context) error {
	if s == nil || s.metrics == nil {
		return nil
	}
	s.metrics.ObserveBudgetSnapshot(s.budget.Snapshot())
	candidates, err := s.snapshots.ListRecentRefreshCandidates(ctx, s.nowFn().UTC().Add(-s.activeWindow))
	if err != nil {
		return err
	}
	s.metrics.ObserveActiveLocations(len(candidates))
	return nil
}
