package solarforecastd

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMetricsObserveServedOutlookTracksActiveSitesAndFreshness(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	metrics.activeSiteWindow = 24 * time.Hour

	issuedAt := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	servedAt := time.Date(2026, 3, 19, 13, 0, 0, 0, time.UTC)
	outlook := &Outlook{
		Provenance: Provenance{
			IssuedAt: issuedAt,
		},
	}

	metrics.ObserveServedOutlook("site-a", outlook, servedAt)
	if got := gatherGaugeValue(t, registry, "pulse_solar_forecast_active_sites"); got != 1 {
		t.Fatalf("activeSites after first serve = %v, want 1", got)
	}
	if got := gatherGaugeValue(t, registry, "pulse_solar_forecast_last_successful_serve_unixtime"); got != float64(servedAt.Unix()) {
		t.Fatalf("lastServedUnix = %v, want %v", got, servedAt.Unix())
	}
	if got := gatherGaugeValue(t, registry, "pulse_solar_forecast_last_issued_forecast_unixtime"); got != float64(issuedAt.Unix()) {
		t.Fatalf("lastIssuedUnix = %v, want %v", got, issuedAt.Unix())
	}

	metrics.ObserveServedOutlook("site-b", outlook, servedAt.Add(2*time.Hour))
	if got := gatherGaugeValue(t, registry, "pulse_solar_forecast_active_sites"); got != 2 {
		t.Fatalf("activeSites after second site = %v, want 2", got)
	}

	metrics.ObserveServedOutlook("site-c", outlook, servedAt.Add(26*time.Hour))
	if got := gatherGaugeValue(t, registry, "pulse_solar_forecast_active_sites"); got != 2 {
		t.Fatalf("activeSites after pruning = %v, want 2", got)
	}
}

func TestMetricsObserveVerificationRowsTracksSquaredErrorTotals(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	metrics.ObserveVerificationRows([]HourlyTrainingRecord{
		{
			HorizonBucket:           HorizonBucketDay1,
			VerificationStatus:      VerificationStatusVerified,
			AbsoluteErrorWh:         float64Ptr(20),
			SquaredErrorWh2:         float64Ptr(400),
			BaselineSquaredErrorWh2: float64Ptr(625),
		},
		{
			HorizonBucket:      HorizonBucketDay1,
			VerificationStatus: VerificationStatusMissingTruth,
			SquaredErrorWh2:    float64Ptr(999),
		},
	}, "site_calibrated")

	if got := gatherCounterValue(t, registry, "pulse_solar_forecast_verification_hourly_squared_error_wh2_total", "horizon", "day_1"); got != 400 {
		t.Fatalf("hourlySquaredErrorWh2Total = %v, want 400", got)
	}
	if got := gatherCounterValue2(t, registry, "pulse_solar_forecast_verification_hourly_squared_error_wh2_by_variant_total", "horizon", "day_1", "variant", "site_calibrated"); got != 400 {
		t.Fatalf("hourlySquaredErrorWh2ByVariantTotal = %v, want 400", got)
	}
	if got := gatherCounterValue2(t, registry, "pulse_solar_forecast_verification_verified_hours_by_variant_total", "horizon", "day_1", "variant", "site_calibrated"); got != 1 {
		t.Fatalf("verificationVerifiedHoursByVariant = %v, want 1", got)
	}
	if got := gatherCounterValue2(t, registry, "pulse_solar_forecast_verification_baseline_hourly_squared_error_wh2_by_variant_total", "horizon", "day_1", "variant", "site_calibrated"); got != 625 {
		t.Fatalf("baselineHourlySquaredErrorWh2ByVariantTotal = %v, want 625", got)
	}
}

func TestMetricsObserveDailyRollupTracksVariantHistograms(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	metrics.ObserveDailyRollup(DailyVerificationRollup{
		HorizonBucket:                      HorizonBucketDay3,
		ServedVariant:                      "site_calibrated",
		DailyAbsErrorWhSum:                 120,
		BaselineDailyAbsErrorWhSum:         180,
		PeakPowerAbsErrorWSum:              80,
		BaselinePeakPowerAbsErrorWSum:      105,
		PeakTimeAbsErrorMinutesSum:         15,
		BaselinePeakTimeAbsErrorMinutesSum: 35,
	})

	if got := gatherHistogramSampleCount2(t, registry, "pulse_solar_forecast_verification_daily_error_wh_by_variant", "horizon", "day_3", "variant", "site_calibrated"); got != 1 {
		t.Fatalf("dailyErrorWhByVariant sample count = %v, want 1", got)
	}
	if got := gatherHistogramSampleCount2(t, registry, "pulse_solar_forecast_verification_peak_power_error_w_by_variant", "horizon", "day_3", "variant", "site_calibrated"); got != 1 {
		t.Fatalf("peakPowerErrorWByVariant sample count = %v, want 1", got)
	}
	if got := gatherHistogramSampleCount2(t, registry, "pulse_solar_forecast_verification_peak_time_error_minutes_by_variant", "horizon", "day_3", "variant", "site_calibrated"); got != 1 {
		t.Fatalf("peakTimeErrorMinutesByVariant sample count = %v, want 1", got)
	}
	if got := gatherHistogramSampleCount2(t, registry, "pulse_solar_forecast_verification_baseline_daily_error_wh_by_variant", "horizon", "day_3", "variant", "site_calibrated"); got != 1 {
		t.Fatalf("baselineDailyErrorWhByVariant sample count = %v, want 1", got)
	}
	if got := gatherHistogramSampleCount2(t, registry, "pulse_solar_forecast_verification_baseline_peak_power_error_w_by_variant", "horizon", "day_3", "variant", "site_calibrated"); got != 1 {
		t.Fatalf("baselinePeakPowerErrorWByVariant sample count = %v, want 1", got)
	}
	if got := gatherHistogramSampleCount2(t, registry, "pulse_solar_forecast_verification_baseline_peak_time_error_minutes_by_variant", "horizon", "day_3", "variant", "site_calibrated"); got != 1 {
		t.Fatalf("baselinePeakTimeErrorMinutesByVariant sample count = %v, want 1", got)
	}
}

func gatherGaugeValue(t *testing.T, registry *prometheus.Registry, name string) float64 {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		if len(family.Metric) == 0 || family.Metric[0].Gauge == nil {
			t.Fatalf("metric %s missing gauge value", name)
		}
		return family.Metric[0].Gauge.GetValue()
	}
	t.Fatalf("metric %s not found", name)
	return 0
}

func gatherCounterValue(t *testing.T, registry *prometheus.Registry, name, labelName, labelValue string) float64 {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			if hasMetricLabel(metric, labelName, labelValue) {
				if metric.Counter == nil {
					t.Fatalf("metric %s missing counter value", name)
				}
				return metric.Counter.GetValue()
			}
		}
	}
	t.Fatalf("metric %s with %s=%s not found", name, labelName, labelValue)
	return 0
}

func gatherCounterValue2(t *testing.T, registry *prometheus.Registry, name, labelNameA, labelValueA, labelNameB, labelValueB string) float64 {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			if hasMetricLabel(metric, labelNameA, labelValueA) && hasMetricLabel(metric, labelNameB, labelValueB) {
				if metric.Counter == nil {
					t.Fatalf("metric %s missing counter value", name)
				}
				return metric.Counter.GetValue()
			}
		}
	}
	t.Fatalf("metric %s with %s=%s and %s=%s not found", name, labelNameA, labelValueA, labelNameB, labelValueB)
	return 0
}

func gatherHistogramSampleCount2(t *testing.T, registry *prometheus.Registry, name, labelNameA, labelValueA, labelNameB, labelValueB string) uint64 {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			if hasMetricLabel(metric, labelNameA, labelValueA) && hasMetricLabel(metric, labelNameB, labelValueB) {
				if metric.Histogram == nil {
					t.Fatalf("metric %s missing histogram value", name)
				}
				return metric.Histogram.GetSampleCount()
			}
		}
	}
	t.Fatalf("metric %s with %s=%s and %s=%s not found", name, labelNameA, labelValueA, labelNameB, labelValueB)
	return 0
}

func hasMetricLabel(metric *dto.Metric, labelName, labelValue string) bool {
	for _, label := range metric.Label {
		if label.GetName() == labelName && label.GetValue() == labelValue {
			return true
		}
	}
	return false
}
