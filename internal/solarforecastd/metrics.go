package solarforecastd

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requests                                    *prometheus.CounterVec
	requestDuration                             *prometheus.HistogramVec
	stageDuration                               *prometheus.HistogramVec
	trainingRuns                                *prometheus.CounterVec
	trainingHours                               *prometheus.CounterVec
	confidence                                  *prometheus.CounterVec
	verificationRuns                            *prometheus.CounterVec
	verificationHours                           *prometheus.CounterVec
	hourlySquaredErrorWh2Total                  *prometheus.CounterVec
	verificationVerifiedHoursByVariant          *prometheus.CounterVec
	hourlySquaredErrorWh2ByVariantTotal         *prometheus.CounterVec
	baselineHourlySquaredErrorWh2ByVariantTotal *prometheus.CounterVec
	hourlyErrorWh                               *prometheus.HistogramVec
	dailyErrorWh                                *prometheus.HistogramVec
	dailyErrorWhByVariant                       *prometheus.HistogramVec
	baselineDailyErrorWhByVariant               *prometheus.HistogramVec
	peakPowerErrorW                             *prometheus.HistogramVec
	peakPowerErrorWByVariant                    *prometheus.HistogramVec
	baselinePeakPowerErrorWByVariant            *prometheus.HistogramVec
	peakTimeErrorMinutes                        *prometheus.HistogramVec
	peakTimeErrorMinutesByVariant               *prometheus.HistogramVec
	baselinePeakTimeErrorMinutesByVariant       *prometheus.HistogramVec
	modelMix                                    *prometheus.CounterVec
	activeSites                                 prometheus.Gauge
	lastServedUnix                              prometheus.Gauge
	lastIssuedUnix                              prometheus.Gauge
	lastVerifiedUnix                            prometheus.Gauge
	mu                                          sync.Mutex
	activeSiteWindow                            time.Duration
	activeSiteSeen                              map[string]time.Time
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	if registerer == nil {
		return nil
	}
	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_requests_total",
			Help: "Solar forecast request outcomes by scope and result.",
		}, []string{"scope", "result"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_request_duration_seconds",
			Help:    "Solar forecast request duration by scope and result.",
			Buckets: prometheus.DefBuckets,
		}, []string{"scope", "result"}),
		stageDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_stage_duration_seconds",
			Help:    "Solar forecast stage duration by scope, stage, and result.",
			Buckets: prometheus.DefBuckets,
		}, []string{"scope", "stage", "result"}),
		trainingRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_training_runs_total",
			Help: "Solar forecast training run write outcomes.",
		}, []string{"result"}),
		trainingHours: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_training_hours_total",
			Help: "Solar forecast training hourly row write outcomes.",
		}, []string{"result"}),
		confidence: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_confidence_total",
			Help: "Solar forecast day confidence counts from served responses.",
		}, []string{"confidence"}),
		verificationRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_verification_runs_total",
			Help: "Solar forecast verification loop run outcomes.",
		}, []string{"result"}),
		verificationHours: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_verification_hours_total",
			Help: "Solar forecast verification hour outcomes by horizon and result.",
		}, []string{"horizon", "result"}),
		hourlySquaredErrorWh2Total: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_verification_hourly_squared_error_wh2_total",
			Help: "Solar forecast verified hourly squared error total in watt-hours squared by horizon.",
		}, []string{"horizon"}),
		verificationVerifiedHoursByVariant: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_verification_verified_hours_by_variant_total",
			Help: "Solar forecast verified hour count by horizon and served variant.",
		}, []string{"horizon", "variant"}),
		hourlySquaredErrorWh2ByVariantTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_verification_hourly_squared_error_wh2_by_variant_total",
			Help: "Solar forecast verified hourly squared error total in watt-hours squared by horizon and served variant.",
		}, []string{"horizon", "variant"}),
		baselineHourlySquaredErrorWh2ByVariantTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_verification_baseline_hourly_squared_error_wh2_by_variant_total",
			Help: "Shadow baseline verified hourly squared error total in watt-hours squared by horizon and served variant.",
		}, []string{"horizon", "variant"}),
		hourlyErrorWh: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_verification_hourly_error_wh",
			Help:    "Solar forecast hourly absolute error distribution in watt-hours by horizon.",
			Buckets: []float64{25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		}, []string{"horizon"}),
		dailyErrorWh: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_verification_daily_error_wh",
			Help:    "Solar forecast daily absolute error distribution in watt-hours by horizon.",
			Buckets: []float64{100, 250, 500, 1000, 2500, 5000, 10000, 20000, 50000},
		}, []string{"horizon"}),
		dailyErrorWhByVariant: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_verification_daily_error_wh_by_variant",
			Help:    "Solar forecast daily absolute error distribution in watt-hours by horizon and served variant.",
			Buckets: []float64{100, 250, 500, 1000, 2500, 5000, 10000, 20000, 50000},
		}, []string{"horizon", "variant"}),
		baselineDailyErrorWhByVariant: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_verification_baseline_daily_error_wh_by_variant",
			Help:    "Shadow baseline daily absolute error distribution in watt-hours by horizon and served variant.",
			Buckets: []float64{100, 250, 500, 1000, 2500, 5000, 10000, 20000, 50000},
		}, []string{"horizon", "variant"}),
		peakPowerErrorW: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_verification_peak_power_error_w",
			Help:    "Solar forecast peak power absolute error distribution in watts by horizon.",
			Buckets: []float64{25, 50, 100, 250, 500, 1000, 2500, 5000},
		}, []string{"horizon"}),
		peakPowerErrorWByVariant: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_verification_peak_power_error_w_by_variant",
			Help:    "Solar forecast peak power absolute error distribution in watts by horizon and served variant.",
			Buckets: []float64{25, 50, 100, 250, 500, 1000, 2500, 5000},
		}, []string{"horizon", "variant"}),
		baselinePeakPowerErrorWByVariant: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_verification_baseline_peak_power_error_w_by_variant",
			Help:    "Shadow baseline peak power absolute error distribution in watts by horizon and served variant.",
			Buckets: []float64{25, 50, 100, 250, 500, 1000, 2500, 5000},
		}, []string{"horizon", "variant"}),
		peakTimeErrorMinutes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_verification_peak_time_error_minutes",
			Help:    "Solar forecast peak timing absolute error distribution in minutes by horizon.",
			Buckets: []float64{5, 10, 15, 30, 60, 120, 180, 360},
		}, []string{"horizon"}),
		peakTimeErrorMinutesByVariant: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_verification_peak_time_error_minutes_by_variant",
			Help:    "Solar forecast peak timing absolute error distribution in minutes by horizon and served variant.",
			Buckets: []float64{5, 10, 15, 30, 60, 120, 180, 360},
		}, []string{"horizon", "variant"}),
		baselinePeakTimeErrorMinutesByVariant: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_solar_forecast_verification_baseline_peak_time_error_minutes_by_variant",
			Help:    "Shadow baseline peak timing absolute error distribution in minutes by horizon and served variant.",
			Buckets: []float64{5, 10, 15, 30, 60, 120, 180, 360},
		}, []string{"horizon", "variant"}),
		modelMix: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_solar_forecast_model_mix_total",
			Help: "Served solar forecast mode mix.",
		}, []string{"mode"}),
		activeSites: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_solar_forecast_active_sites",
			Help: "Approximate count of unique solar forecast site keys served within the recent rolling window.",
		}),
		lastServedUnix: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_solar_forecast_last_successful_serve_unixtime",
			Help: "Unix time of the latest successful served solar forecast response.",
		}),
		lastIssuedUnix: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_solar_forecast_last_issued_forecast_unixtime",
			Help: "Unix time of the latest upstream-backed solar forecast issue time seen on a successful response.",
		}),
		lastVerifiedUnix: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_solar_forecast_last_successful_verification_unixtime",
			Help: "Unix time of the latest successful solar forecast verification loop run.",
		}),
		activeSiteWindow: 24 * time.Hour,
		activeSiteSeen:   make(map[string]time.Time),
	}
	registerer.MustRegister(
		m.requests,
		m.requestDuration,
		m.stageDuration,
		m.trainingRuns,
		m.trainingHours,
		m.confidence,
		m.verificationRuns,
		m.verificationHours,
		m.hourlySquaredErrorWh2Total,
		m.verificationVerifiedHoursByVariant,
		m.hourlySquaredErrorWh2ByVariantTotal,
		m.baselineHourlySquaredErrorWh2ByVariantTotal,
		m.hourlyErrorWh,
		m.dailyErrorWh,
		m.dailyErrorWhByVariant,
		m.baselineDailyErrorWhByVariant,
		m.peakPowerErrorW,
		m.peakPowerErrorWByVariant,
		m.baselinePeakPowerErrorWByVariant,
		m.peakTimeErrorMinutes,
		m.peakTimeErrorMinutesByVariant,
		m.baselinePeakTimeErrorMinutesByVariant,
		m.modelMix,
		m.activeSites,
		m.lastServedUnix,
		m.lastIssuedUnix,
		m.lastVerifiedUnix,
	)
	return m
}

func (m *Metrics) ObserveRequest(scope string, err error, duration time.Duration) {
	if m == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	m.requests.WithLabelValues(scopeLabel(scope), result).Inc()
	m.requestDuration.WithLabelValues(scopeLabel(scope), result).Observe(duration.Seconds())
}

func (m *Metrics) ObserveStageTiming(scope, stage string, err error, duration time.Duration) {
	if m == nil {
		return
	}
	m.stageDuration.WithLabelValues(scopeLabel(scope), stageLabel(stage), resultLabel(err)).Observe(duration.Seconds())
}

func (m *Metrics) ObserveTrainingRun(err error) {
	if m == nil {
		return
	}
	m.trainingRuns.WithLabelValues(resultLabel(err)).Inc()
}

func (m *Metrics) ObserveTrainingHours(count int, err error) {
	if m == nil || count <= 0 {
		return
	}
	m.trainingHours.WithLabelValues(resultLabel(err)).Add(float64(count))
}

func (m *Metrics) ObserveConfidence(outlook *Outlook) {
	if m == nil || outlook == nil {
		return
	}
	if outlook.Today.Confidence != "" {
		m.confidence.WithLabelValues(string(outlook.Today.Confidence)).Inc()
	}
	for _, day := range outlook.Next7Days {
		if day.Confidence == "" {
			continue
		}
		m.confidence.WithLabelValues(string(day.Confidence)).Inc()
	}
}

func (m *Metrics) ObserveVerificationRun(err error) {
	if m == nil {
		return
	}
	m.verificationRuns.WithLabelValues(resultLabel(err)).Inc()
	if err == nil {
		m.lastVerifiedUnix.Set(float64(time.Now().UTC().Unix()))
	}
}

func (m *Metrics) ObserveVerificationRows(rows []HourlyTrainingRecord, servedVariant string) {
	if m == nil {
		return
	}
	variant := normalizedServedVariant(servedVariant)
	for _, row := range rows {
		horizon := horizonLabel(row.HorizonBucket)
		result := verificationResultLabel(row.VerificationStatus)
		m.verificationHours.WithLabelValues(horizon, result).Inc()
		if row.AbsoluteErrorWh != nil && row.VerificationStatus == VerificationStatusVerified {
			m.hourlyErrorWh.WithLabelValues(horizon).Observe(*row.AbsoluteErrorWh)
		}
		if row.SquaredErrorWh2 != nil && row.VerificationStatus == VerificationStatusVerified {
			m.hourlySquaredErrorWh2Total.WithLabelValues(horizon).Add(*row.SquaredErrorWh2)
			m.hourlySquaredErrorWh2ByVariantTotal.WithLabelValues(horizon, variant).Add(*row.SquaredErrorWh2)
		}
		if row.BaselineSquaredErrorWh2 != nil && row.VerificationStatus == VerificationStatusVerified {
			m.baselineHourlySquaredErrorWh2ByVariantTotal.WithLabelValues(horizon, variant).Add(*row.BaselineSquaredErrorWh2)
		}
		if row.VerificationStatus == VerificationStatusVerified {
			m.verificationVerifiedHoursByVariant.WithLabelValues(horizon, variant).Inc()
		}
	}
}

func (m *Metrics) ObserveDailyRollup(row DailyVerificationRollup) {
	if m == nil {
		return
	}
	horizon := horizonLabel(row.HorizonBucket)
	variant := normalizedServedVariant(row.ServedVariant)
	m.dailyErrorWh.WithLabelValues(horizon).Observe(row.DailyAbsErrorWhSum)
	m.dailyErrorWhByVariant.WithLabelValues(horizon, variant).Observe(row.DailyAbsErrorWhSum)
	m.baselineDailyErrorWhByVariant.WithLabelValues(horizon, variant).Observe(row.BaselineDailyAbsErrorWhSum)
	m.peakPowerErrorW.WithLabelValues(horizon).Observe(row.PeakPowerAbsErrorWSum)
	m.peakPowerErrorWByVariant.WithLabelValues(horizon, variant).Observe(row.PeakPowerAbsErrorWSum)
	m.baselinePeakPowerErrorWByVariant.WithLabelValues(horizon, variant).Observe(row.BaselinePeakPowerAbsErrorWSum)
	m.peakTimeErrorMinutes.WithLabelValues(horizon).Observe(row.PeakTimeAbsErrorMinutesSum)
	m.peakTimeErrorMinutesByVariant.WithLabelValues(horizon, variant).Observe(row.PeakTimeAbsErrorMinutesSum)
	m.baselinePeakTimeErrorMinutesByVariant.WithLabelValues(horizon, variant).Observe(row.BaselinePeakTimeAbsErrorMinutesSum)
}

func (m *Metrics) ObserveModel(mode string) {
	if m == nil {
		return
	}
	m.modelMix.WithLabelValues(mode).Inc()
}

func (m *Metrics) ObserveServedOutlook(siteKey string, outlook *Outlook, servedAt time.Time) {
	if m == nil || outlook == nil {
		return
	}
	nowUTC := servedAt.UTC()
	if nowUTC.IsZero() {
		nowUTC = time.Now().UTC()
	}
	m.lastServedUnix.Set(float64(nowUTC.Unix()))
	if !outlook.Provenance.IssuedAt.IsZero() {
		m.lastIssuedUnix.Set(float64(outlook.Provenance.IssuedAt.UTC().Unix()))
	}
	if siteKey == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := nowUTC.Add(-m.activeSiteWindow)
	for key, seenAt := range m.activeSiteSeen {
		if seenAt.Before(cutoff) {
			delete(m.activeSiteSeen, key)
		}
	}
	m.activeSiteSeen[siteKey] = nowUTC
	m.activeSites.Set(float64(len(m.activeSiteSeen)))
}

func resultLabel(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func scopeLabel(scope string) string {
	switch scope {
	case "device", "all":
		return scope
	default:
		return "unknown"
	}
}

func stageLabel(stage string) string {
	switch stage {
	case solarForecastStageWeatherFetch,
		solarForecastStageTelemetryLookback,
		solarForecastStageCalibrationLoads,
		solarForecastStageSummarization,
		solarForecastStageTrainingKickoff:
		return stage
	default:
		return "unknown"
	}
}

func horizonLabel(horizon HorizonBucket) string {
	switch horizon {
	case HorizonBucketSameDay, HorizonBucketDay1, HorizonBucketDay3, HorizonBucketDay7:
		return string(horizon)
	default:
		return "unknown"
	}
}

func verificationResultLabel(status VerificationStatus) string {
	switch status {
	case VerificationStatusVerified, VerificationStatusMissingTruth, VerificationStatusMissingWeather:
		return string(status)
	default:
		return "unknown"
	}
}

func calibrationModeLabel(calibrated bool) string {
	if calibrated {
		return "site_calibrated"
	}
	return "baseline"
}
