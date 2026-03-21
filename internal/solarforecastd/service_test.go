package solarforecastd

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"github.com/prometheus/client_golang/prometheus"
)

func TestGetSolarOutlookPersistsTrainingRunForAllScope(t *testing.T) {
	t.Parallel()

	nowUTC := time.Date(2026, 3, 19, 15, 0, 0, 0, time.UTC)
	loc := mustLocation(t, "America/New_York")
	weather := &stubWeatherForecaster{
		bundle: testBundle(nowUTC, loc, "grid:42.61:-77.40:290|tilt:45|az:0"),
	}
	query := &stubTelemetryReader{
		aggregateSeries: telemetryquery.Series{
			DeviceID:   "all",
			Resolution: telemetryquery.ResolutionHour,
			Points: []telemetryquery.Point{
				{
					BucketStart: nowUTC.Add(-2 * time.Hour),
					BucketEnd:   nowUTC.Add(-1 * time.Hour),
					Metrics: telemetryquery.Metrics{
						PVMaxW:           float64Ptr(1200),
						SolarGeneratedWh: float64Ptr(800),
					},
				},
				{
					BucketStart: nowUTC.Add(-1 * time.Hour),
					BucketEnd:   nowUTC,
					Metrics: telemetryquery.Metrics{
						PVMaxW:           float64Ptr(1400),
						SolarGeneratedWh: float64Ptr(900),
					},
				},
			},
		},
	}
	store := &capturingTrainingStore{}
	registry := prometheus.NewRegistry()
	svc, err := NewService(weather, query, Config{
		Log:     slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		Store:   store,
		Metrics: NewMetrics(registry),
		NowFn:   func() time.Time { return nowUTC },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	outlook, err := svc.GetSolarOutlook(context.Background(), Input{
		WeatherRequest: weatherd.Request{
			Latitude:   42.61,
			Longitude:  -77.40,
			Timezone:   "America/New_York",
			UnitSystem: weatherd.UnitSystemMetric,
		},
		Scope: Scope{
			Mode: "all",
		},
		ResolvedDeviceIDs: []string{"dev-b", "dev-a", "dev-b"},
	})
	if err != nil {
		t.Fatalf("GetSolarOutlook() error = %v", err)
	}
	if outlook == nil {
		t.Fatal("GetSolarOutlook() returned nil outlook")
	}
	if store.run == nil {
		t.Fatal("training run was not persisted")
	}
	if got, want := store.run.ScopeKind, "all"; got != want {
		t.Fatalf("run.ScopeKind = %q, want %q", got, want)
	}
	if store.run.DeviceID != nil {
		t.Fatalf("run.DeviceID = %v, want nil for all scope", *store.run.DeviceID)
	}
	if got, want := store.run.SiteKey, "grid:42.61:-77.40:290|tilt:45|az:0|dev-a,dev-b"; got != want {
		t.Fatalf("run.SiteKey = %q, want %q", got, want)
	}
	if got, want := store.run.ServedVariant, "baseline"; got != want {
		t.Fatalf("run.ServedVariant = %q, want %q", got, want)
	}
	if got := len(store.rows); got == 0 {
		t.Fatal("hourly training rows were not persisted")
	}
	for _, row := range store.rows {
		if got, want := row.SiteKey, store.run.SiteKey; got != want {
			t.Fatalf("row.SiteKey = %q, want %q", got, want)
		}
		if row.BaselineForecastGenerationWh == nil {
			t.Fatal("row.BaselineForecastGenerationWh = nil, want shadow baseline value")
		}
		if row.DeviceID != nil {
			t.Fatalf("row.DeviceID = %v, want nil for all scope", *row.DeviceID)
		}
		if row.VerificationStatus != VerificationStatusPending {
			t.Fatalf("row.VerificationStatus = %q, want %q", row.VerificationStatus, VerificationStatusPending)
		}
	}
}

func TestGetSolarOutlookTrainingStoreFailureDoesNotFailRequest(t *testing.T) {
	t.Parallel()

	nowUTC := time.Date(2026, 3, 19, 15, 0, 0, 0, time.UTC)
	loc := mustLocation(t, "America/New_York")
	weather := &stubWeatherForecaster{
		bundle: testBundle(nowUTC, loc, "grid:42.61:-77.40:290|tilt:45|az:0"),
	}
	query := &stubTelemetryReader{
		series: telemetryquery.Series{
			DeviceID:   "dev-a",
			Resolution: telemetryquery.ResolutionHour,
			Points: []telemetryquery.Point{
				{
					BucketStart: nowUTC.Add(-1 * time.Hour),
					BucketEnd:   nowUTC,
					Metrics: telemetryquery.Metrics{
						PVMaxW:           float64Ptr(900),
						SolarGeneratedWh: float64Ptr(600),
					},
				},
			},
		},
	}
	store := &capturingTrainingStore{insertRunErr: errors.New("boom")}
	svc, err := NewService(weather, query, Config{
		Log:   slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		Store: store,
		NowFn: func() time.Time { return nowUTC },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	outlook, err := svc.GetSolarOutlook(context.Background(), Input{
		WeatherRequest: weatherd.Request{
			Latitude:   42.61,
			Longitude:  -77.40,
			Timezone:   "America/New_York",
			UnitSystem: weatherd.UnitSystemMetric,
		},
		Scope: Scope{
			Mode:     "device",
			DeviceID: "dev-a",
		},
		ResolvedDeviceIDs: []string{"dev-a"},
	})
	if err != nil {
		t.Fatalf("GetSolarOutlook() error = %v", err)
	}
	if outlook == nil {
		t.Fatal("GetSolarOutlook() returned nil outlook")
	}
	if store.run == nil {
		t.Fatal("expected attempted training run insert")
	}
	if got, want := outlook.Scope.Mode, "device"; got != want {
		t.Fatalf("outlook.Scope.Mode = %q, want %q", got, want)
	}
}

func TestVerifyIssuedForecastsBackfillsActualsAndRollups(t *testing.T) {
	t.Parallel()

	nowUTC := time.Date(2026, 3, 20, 15, 0, 0, 0, time.UTC)
	targetA := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	targetB := time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC)
	siteMetadata, err := json.Marshal(map[string]any{
		"resolved_device_ids": []string{"dev-a"},
	})
	if err != nil {
		t.Fatalf("Marshal(siteMetadata) error = %v", err)
	}
	store := &capturingTrainingStore{
		runs: map[string]*Run{
			"run-1": {
				ID:               "run-1",
				SiteKey:          "grid:42.61:-77.40:290|tilt:45|az:0|dev-a",
				ScopeKind:        "device",
				DeviceID:         stringPtr("dev-a"),
				ServedVariant:    "baseline",
				Timezone:         "UTC",
				ForecastVersion:  "deterministic_baseline_v1",
				SiteMetadataJSON: siteMetadata,
			},
		},
		rows: []HourlyTrainingRecord{
			{
				RunID:                        "run-1",
				SiteKey:                      "grid:42.61:-77.40:290|tilt:45|az:0|dev-a",
				DeviceID:                     stringPtr("dev-a"),
				TargetTime:                   targetA,
				TargetLocalDate:              parseDateISO("2026-03-20"),
				HorizonBucket:                HorizonBucketSameDay,
				ForecastGenerationWh:         500,
				BaselineForecastGenerationWh: float64Ptr(500),
				VerificationStatus:           VerificationStatusPending,
			},
			{
				RunID:                        "run-1",
				SiteKey:                      "grid:42.61:-77.40:290|tilt:45|az:0|dev-a",
				DeviceID:                     stringPtr("dev-a"),
				TargetTime:                   targetB,
				TargetLocalDate:              parseDateISO("2026-03-20"),
				HorizonBucket:                HorizonBucketSameDay,
				ForecastGenerationWh:         300,
				BaselineForecastGenerationWh: float64Ptr(300),
				VerificationStatus:           VerificationStatusPending,
			},
		},
	}
	query := &stubTelemetryReader{
		series: telemetryquery.Series{
			DeviceID:   "dev-a",
			Resolution: telemetryquery.ResolutionHour,
			Points: []telemetryquery.Point{
				{
					BucketStart: targetA,
					BucketEnd:   targetA.Add(time.Hour),
					Metrics: telemetryquery.Metrics{
						SolarGeneratedWh: float64Ptr(450),
					},
				},
			},
		},
	}
	svc, err := NewService(nil, query, Config{
		Log:     slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		Store:   store,
		Metrics: NewMetrics(prometheus.NewRegistry()),
		NowFn:   func() time.Time { return nowUTC },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := svc.VerifyIssuedForecasts(context.Background(), nowUTC, 10); err != nil {
		t.Fatalf("VerifyIssuedForecasts() error = %v", err)
	}
	if got, want := len(store.completedRows), 2; got != want {
		t.Fatalf("len(completedRows) = %d, want %d", got, want)
	}
	if got, want := store.completedRows[0].VerificationStatus, VerificationStatusVerified; got != want {
		t.Fatalf("completedRows[0].VerificationStatus = %q, want %q", got, want)
	}
	if got, want := *store.completedRows[0].ActualGenerationWh, 450.0; got != want {
		t.Fatalf("completedRows[0].ActualGenerationWh = %v, want %v", got, want)
	}
	if got, want := *store.completedRows[0].AbsoluteErrorWh, 50.0; got != want {
		t.Fatalf("completedRows[0].AbsoluteErrorWh = %v, want %v", got, want)
	}
	if got, want := valueOrZero(store.completedRows[0].BaselineAbsoluteErrorWh), 50.0; got != want {
		t.Fatalf("completedRows[0].BaselineAbsoluteErrorWh = %v, want %v", got, want)
	}
	if got, want := store.completedRows[1].VerificationStatus, VerificationStatusMissingTruth; got != want {
		t.Fatalf("completedRows[1].VerificationStatus = %q, want %q", got, want)
	}
	if got, want := len(store.rollups), 1; got != want {
		t.Fatalf("len(rollups) = %d, want %d", got, want)
	}
	rollup := store.rollups[0]
	if got, want := rollup.ForecastHours, 2; got != want {
		t.Fatalf("rollup.ForecastHours = %d, want %d", got, want)
	}
	if got, want := rollup.VerifiedHours, 1; got != want {
		t.Fatalf("rollup.VerifiedHours = %d, want %d", got, want)
	}
	if got, want := rollup.MissingTruthHours, 1; got != want {
		t.Fatalf("rollup.MissingTruthHours = %d, want %d", got, want)
	}
	if got, want := rollup.DailyAbsErrorWhSum, 50.0; got != want {
		t.Fatalf("rollup.DailyAbsErrorWhSum = %v, want %v", got, want)
	}
	if got, want := rollup.ServedVariant, "baseline"; got != want {
		t.Fatalf("rollup.ServedVariant = %q, want %q", got, want)
	}
	if got, want := len(store.calibrationStates), 1; got != want {
		t.Fatalf("len(calibrationStates) = %d, want %d", got, want)
	}
	if got, want := valueOrZero(store.calibrationStates[0].MultiplicativeRatio), 0.9; got != want {
		t.Fatalf("calibrationStates[0].MultiplicativeRatio = %v, want %v", got, want)
	}
}

func TestGetSolarOutlookAppliesCalibrationRatio(t *testing.T) {
	t.Parallel()

	nowUTC := time.Date(2026, 3, 19, 15, 0, 0, 0, time.UTC)
	loc := mustLocation(t, "America/New_York")
	weather := &stubWeatherForecaster{
		bundle: testBundle(nowUTC, loc, "grid:42.61:-77.40:290|tilt:45|az:0"),
	}
	query := &stubTelemetryReader{
		series: telemetryquery.Series{
			DeviceID:   "dev-a",
			Resolution: telemetryquery.ResolutionHour,
			Points: []telemetryquery.Point{
				{
					BucketStart: nowUTC.Add(-1 * time.Hour),
					BucketEnd:   nowUTC,
					Metrics: telemetryquery.Metrics{
						PVMaxW:           float64Ptr(1000),
						SolarGeneratedWh: float64Ptr(600),
					},
				},
			},
		},
	}
	store := &capturingTrainingStore{
		calibrationStates: []CalibrationState{
			{
				SiteKey:             "grid:42.61:-77.40:290|tilt:45|az:0|dev-a",
				ForecastVersion:     "deterministic_baseline_v1",
				HorizonBucket:       HorizonBucketSameDay,
				HourOfDay:           10,
				SampleCount:         5,
				MultiplicativeRatio: float64Ptr(0.5),
			},
			{
				SiteKey:             "grid:42.61:-77.40:290|tilt:45|az:0|dev-a",
				ForecastVersion:     "deterministic_baseline_v1",
				HorizonBucket:       HorizonBucketSameDay,
				HourOfDay:           14,
				SampleCount:         5,
				MultiplicativeRatio: float64Ptr(0.5),
			},
		},
	}
	svc, err := NewService(weather, query, Config{
		Log:   slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		Store: store,
		NowFn: func() time.Time { return nowUTC },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	outlook, err := svc.GetSolarOutlook(context.Background(), Input{
		WeatherRequest: weatherd.Request{
			Latitude:   42.61,
			Longitude:  -77.40,
			Timezone:   "America/New_York",
			UnitSystem: weatherd.UnitSystemMetric,
		},
		Scope: Scope{
			Mode:     "device",
			DeviceID: "dev-a",
		},
		ResolvedDeviceIDs: []string{"dev-a"},
	})
	if err != nil {
		t.Fatalf("GetSolarOutlook() error = %v", err)
	}
	if got, want := outlook.Provenance.ForecastModel, "deterministic_baseline_v1"; got != want {
		t.Fatalf("outlook.Provenance.ForecastModel = %q, want %q", got, want)
	}
	if got, want := outlook.Provenance.ServedVariant, "site_calibrated"; got != want {
		t.Fatalf("outlook.Provenance.ServedVariant = %q, want %q", got, want)
	}
	if !outlook.Provenance.CalibrationApplied {
		t.Fatal("outlook.Provenance.CalibrationApplied = false, want true")
	}
	if got, want := outlook.Provenance.CalibrationSampleCount, 10; got != want {
		t.Fatalf("outlook.Provenance.CalibrationSampleCount = %d, want %d", got, want)
	}
	if got, want := valueOrZero(outlook.Next24Hours[0].ForecastGeneratedWh), 269.8; got != want {
		t.Fatalf("outlook.Next24Hours[0].ForecastGeneratedWh = %v, want %v", got, want)
	}
}

func TestEstimateForecastWattsUsesIrradianceWithoutSecondWeatherPenalty(t *testing.T) {
	t.Parallel()

	nowUTC := time.Date(2026, 3, 20, 15, 0, 0, 0, time.UTC)
	loc := mustLocation(t, "America/New_York")
	estimatedPeakWatts := 3100.0
	point := weatherd.HourlyForecastPoint{
		Time: time.Date(2026, 3, 20, 16, 0, 0, 0, time.UTC),
		Condition: weatherd.WeatherCondition{
			WeatherCode: 61,
		},
		Raw: weatherd.ForecastValueSet{
			Temperature:             float64Ptr(6),
			CloudCover:              float64Ptr(92),
			Precipitation:           float64Ptr(0.8),
			ShortwaveRadiation:      float64Ptr(420),
			SunshineDurationSeconds: float64Ptr(0),
		},
	}

	got := estimateForecastWatts(point, &estimatedPeakWatts, nowUTC, loc, nil, nil)
	if got == nil {
		t.Fatal("estimateForecastWatts() = nil, want value")
	}
	want := math.Min(
		estimatedPeakWatts*maxForecastPeakOutputScale,
		estimatedPeakWatts*clamp(420.0/1000.0, 0, 1.1)*baseSystemEfficiencyFactor,
	)
	if diff := math.Abs(*got - round1(want)); diff > 0.0001 {
		t.Fatalf("estimateForecastWatts() = %v, want %v", *got, want)
	}
}

func TestDeriveTodayRemainingScaleClampsLaggingDay(t *testing.T) {
	t.Parallel()

	loc := mustLocation(t, "America/New_York")
	nowUTC := time.Date(2026, 3, 20, 15, 0, 0, 0, time.UTC)
	estimatedPeakWatts := 3100.0
	todayISO := localDateISO(nowUTC, loc)
	hourly := []weatherd.HourlyForecastPoint{
		{
			Time:      time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC),
			Condition: weatherd.WeatherCondition{WeatherCode: 0},
			Raw: weatherd.ForecastValueSet{
				Temperature:             float64Ptr(11),
				CloudCover:              float64Ptr(8),
				ShortwaveRadiation:      float64Ptr(600),
				SunshineDurationSeconds: float64Ptr(3600),
			},
		},
		{
			Time:      time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC),
			Condition: weatherd.WeatherCondition{WeatherCode: 0},
			Raw: weatherd.ForecastValueSet{
				Temperature:             float64Ptr(12),
				CloudCover:              float64Ptr(10),
				ShortwaveRadiation:      float64Ptr(650),
				SunshineDurationSeconds: float64Ptr(3600),
			},
		},
		{
			Time:      time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC),
			Condition: weatherd.WeatherCondition{WeatherCode: 0},
			Raw: weatherd.ForecastValueSet{
				Temperature:             float64Ptr(13),
				CloudCover:              float64Ptr(12),
				ShortwaveRadiation:      float64Ptr(700),
				SunshineDurationSeconds: float64Ptr(3600),
			},
		},
	}

	todayStartLocal := time.Date(nowUTC.In(loc).Year(), nowUTC.In(loc).Month(), nowUTC.In(loc).Day(), 0, 0, 0, 0, loc)
	points := make([]telemetryquery.Point, 0, 11)
	for hour := 0; hour < 11; hour++ {
		bucketStart := todayStartLocal.Add(time.Duration(hour) * time.Hour).UTC()
		points = append(points, telemetryquery.Point{
			BucketStart: bucketStart,
			BucketEnd:   bucketStart.Add(time.Hour),
			Metrics: telemetryquery.Metrics{
				SolarGeneratedWh: float64Ptr(27.27),
			},
		})
	}
	history := telemetryquery.Series{
		EnergyBucketCoverage: telemetryquery.EnergyBucketCoverage{
			PointCount:          len(points),
			PersistedValueCount: len(points),
		},
		Points: points,
	}

	got := deriveTodayRemainingScale(history, hourly, &estimatedPeakWatts, 300, todayISO, nowUTC, loc, nil, nil)
	if got != 0.5 {
		t.Fatalf("deriveTodayRemainingScale() = %v, want 0.5", got)
	}
}

func TestDeriveTodayRemainingScaleSkipsIncompleteTelemetry(t *testing.T) {
	t.Parallel()

	loc := mustLocation(t, "America/New_York")
	nowUTC := time.Date(2026, 3, 20, 15, 0, 0, 0, time.UTC)
	estimatedPeakWatts := 3100.0
	todayISO := localDateISO(nowUTC, loc)
	hourly := []weatherd.HourlyForecastPoint{
		{
			Time:      time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC),
			Condition: weatherd.WeatherCondition{WeatherCode: 0},
			Raw: weatherd.ForecastValueSet{
				Temperature:             float64Ptr(11),
				CloudCover:              float64Ptr(8),
				ShortwaveRadiation:      float64Ptr(600),
				SunshineDurationSeconds: float64Ptr(3600),
			},
		},
		{
			Time:      time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC),
			Condition: weatherd.WeatherCondition{WeatherCode: 0},
			Raw: weatherd.ForecastValueSet{
				Temperature:             float64Ptr(12),
				CloudCover:              float64Ptr(10),
				ShortwaveRadiation:      float64Ptr(650),
				SunshineDurationSeconds: float64Ptr(3600),
			},
		},
		{
			Time:      time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC),
			Condition: weatherd.WeatherCondition{WeatherCode: 0},
			Raw: weatherd.ForecastValueSet{
				Temperature:             float64Ptr(13),
				CloudCover:              float64Ptr(12),
				ShortwaveRadiation:      float64Ptr(700),
				SunshineDurationSeconds: float64Ptr(3600),
			},
		},
	}
	history := telemetryquery.Series{
		EnergyBucketCoverage: telemetryquery.EnergyBucketCoverage{
			PointCount:          11,
			PersistedValueCount: 1,
		},
		Points: []telemetryquery.Point{
			{
				BucketStart: time.Date(2026, 3, 20, 4, 0, 0, 0, time.UTC),
				BucketEnd:   time.Date(2026, 3, 20, 5, 0, 0, 0, time.UTC),
				Metrics: telemetryquery.Metrics{
					SolarGeneratedWh: float64Ptr(100),
				},
			},
		},
	}

	got := deriveTodayRemainingScale(history, hourly, &estimatedPeakWatts, 300, todayISO, nowUTC, loc, nil, nil)
	if got != 1 {
		t.Fatalf("deriveTodayRemainingScale() = %v, want 1 when telemetry is incomplete", got)
	}
}

func TestBuildTrainingRowsPersistsRawForecastInsteadOfDisplayedClamp(t *testing.T) {
	t.Parallel()

	loc := mustLocation(t, "America/New_York")
	nowUTC := time.Date(2026, 3, 20, 17, 0, 0, 0, time.UTC)
	todayISO := localDateISO(nowUTC, loc)
	run := Run{
		ID:        "run-1",
		SiteKey:   "site-1",
		DeviceID:  stringPtr("dev-a"),
		CreatedAt: nowUTC,
		UpdatedAt: nowUTC,
	}
	point := weatherd.HourlyForecastPoint{
		Time:      time.Date(2026, 3, 20, 18, 0, 0, 0, time.UTC),
		Condition: weatherd.WeatherCondition{WeatherCode: 0},
		Raw: weatherd.ForecastValueSet{
			Temperature:             float64Ptr(12),
			CloudCover:              float64Ptr(10),
			ShortwaveRadiation:      float64Ptr(700),
			SunshineDurationSeconds: float64Ptr(3600),
		},
	}
	outlook := &Outlook{
		Provenance: Provenance{
			Timezone: "America/New_York",
		},
		Capacity: CapacityEstimate{
			EstimatedPeakWatts: float64Ptr(3100),
		},
		Next7Days: []GenerationDay{
			{
				Date: parseDateISO(todayISO),
			},
		},
	}
	bundle := &weatherd.Bundle{
		Hourly: []weatherd.HourlyForecastPoint{point},
	}

	rows := buildTrainingRows(run, bundle, telemetryquery.Series{}, outlook, nowUTC, nil, nil, 0.5)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	rawForecast := estimateForecastWatts(point, outlook.Capacity.EstimatedPeakWatts, nowUTC, loc, nil, nil)
	displayedForecast := estimateDisplayedForecastWatts(point, outlook.Capacity.EstimatedPeakWatts, todayISO, 0.5, nowUTC, loc, nil, nil)
	if rawForecast == nil || displayedForecast == nil {
		t.Fatal("expected non-nil raw and displayed forecasts")
	}
	if rows[0].ForecastGenerationWh != *rawForecast {
		t.Fatalf("rows[0].ForecastGenerationWh = %v, want raw %v", rows[0].ForecastGenerationWh, *rawForecast)
	}
	if rows[0].ForecastGenerationWh == *displayedForecast {
		t.Fatalf("rows[0].ForecastGenerationWh = %v, should not match displayed-clamped %v", rows[0].ForecastGenerationWh, *displayedForecast)
	}
	if rows[0].BaselineForecastGenerationWh == nil || *rows[0].BaselineForecastGenerationWh != *rawForecast {
		t.Fatalf("rows[0].BaselineForecastGenerationWh = %v, want raw %v", valueOrZero(rows[0].BaselineForecastGenerationWh), *rawForecast)
	}
}

func TestReplayValidationCalibratedRunBeatsShadowBaseline(t *testing.T) {
	t.Parallel()

	loc := mustLocation(t, "America/New_York")
	store := &capturingTrainingStore{}
	query := &stubTelemetryReader{
		queryRangeFn: func(_ context.Context, q telemetryquery.RangeQuery) (telemetryquery.Series, error) {
			points := make([]telemetryquery.Point, 0)
			for ts := q.From.UTC(); ts.Before(q.To.UTC()); ts = ts.Add(time.Hour) {
				localHour := ts.In(loc).Hour()
				if localHour != 10 && localHour != 14 {
					continue
				}
				actual := 350.0
				if localHour == 14 {
					actual = 300.0
				}
				points = append(points, telemetryquery.Point{
					BucketStart: ts.UTC(),
					BucketEnd:   ts.UTC().Add(time.Hour),
					Metrics: telemetryquery.Metrics{
						PVMaxW:           float64Ptr(600 + float64((localHour-10)*100)),
						SolarGeneratedWh: float64Ptr(actual),
					},
				})
			}
			return telemetryquery.Series{
				DeviceID:   q.DeviceID,
				Resolution: telemetryquery.ResolutionHour,
				From:       q.From,
				To:         q.To,
				Points:     points,
			}, nil
		},
	}

	for day := 0; day < 3; day++ {
		nowUTC := time.Date(2026, 3, 19+day, 8, 0, 0, 0, time.UTC)
		weather := &stubWeatherForecaster{
			bundle: testBundle(nowUTC, loc, "grid:42.61:-77.40:290|tilt:45|az:0"),
		}
		svc, err := NewService(weather, query, Config{
			Log:     slog.New(slog.NewTextHandler(testWriter{t}, nil)),
			Store:   store,
			Metrics: NewMetrics(prometheus.NewRegistry()),
			NowFn:   func() time.Time { return nowUTC },
		})
		if err != nil {
			t.Fatalf("NewService(training day %d) error = %v", day, err)
		}

		outlook, err := svc.GetSolarOutlook(context.Background(), Input{
			WeatherRequest: weatherd.Request{
				Latitude:   42.61,
				Longitude:  -77.40,
				Timezone:   "America/New_York",
				UnitSystem: weatherd.UnitSystemMetric,
			},
			Scope: Scope{
				Mode:     "device",
				DeviceID: "dev-a",
			},
			ResolvedDeviceIDs: []string{"dev-a"},
		})
		if err != nil {
			t.Fatalf("GetSolarOutlook(training day %d) error = %v", day, err)
		}
		if got, want := outlook.Provenance.ServedVariant, "baseline"; got != want {
			t.Fatalf("training day %d served variant = %q, want %q", day, got, want)
		}

		verifyAt := nowUTC.Add(40 * time.Hour)
		if err := svc.VerifyIssuedForecasts(context.Background(), verifyAt, 100); err != nil {
			t.Fatalf("VerifyIssuedForecasts(training day %d) error = %v", day, err)
		}
	}

	if got, want := len(store.calibrationStates), 2; got != want {
		t.Fatalf("len(calibrationStates) after training = %d, want %d", got, want)
	}

	nowUTC := time.Date(2026, 3, 22, 8, 0, 0, 0, time.UTC)
	weather := &stubWeatherForecaster{
		bundle: testBundle(nowUTC, loc, "grid:42.61:-77.40:290|tilt:45|az:0"),
	}
	svc, err := NewService(weather, query, Config{
		Log:     slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		Store:   store,
		Metrics: NewMetrics(prometheus.NewRegistry()),
		NowFn:   func() time.Time { return nowUTC },
	})
	if err != nil {
		t.Fatalf("NewService(calibrated day) error = %v", err)
	}

	outlook, err := svc.GetSolarOutlook(context.Background(), Input{
		WeatherRequest: weatherd.Request{
			Latitude:   42.61,
			Longitude:  -77.40,
			Timezone:   "America/New_York",
			UnitSystem: weatherd.UnitSystemMetric,
		},
		Scope: Scope{
			Mode:     "device",
			DeviceID: "dev-a",
		},
		ResolvedDeviceIDs: []string{"dev-a"},
	})
	if err != nil {
		t.Fatalf("GetSolarOutlook(calibrated day) error = %v", err)
	}
	if got, want := outlook.Provenance.ServedVariant, "site_calibrated"; got != want {
		t.Fatalf("calibrated day served variant = %q, want %q", got, want)
	}

	verifyAt := nowUTC.Add(40 * time.Hour)
	if err := svc.VerifyIssuedForecasts(context.Background(), verifyAt, 100); err != nil {
		t.Fatalf("VerifyIssuedForecasts(calibrated day) error = %v", err)
	}

	targetDate := parseDateISO("2026-03-23")
	var calibratedRollup *DailyVerificationRollup
	for idx := range store.rollups {
		rollup := &store.rollups[idx]
		if rollup.VerificationLocalDate.Equal(targetDate) && rollup.ServedVariant == "site_calibrated" {
			calibratedRollup = rollup
		}
	}
	if calibratedRollup == nil {
		t.Fatal("missing site_calibrated rollup for replay validation target day")
	}
	if calibratedRollup.BaselineDailyAbsErrorWhSum <= calibratedRollup.DailyAbsErrorWhSum {
		t.Fatalf("baseline daily abs error = %v, want greater than served %v", calibratedRollup.BaselineDailyAbsErrorWhSum, calibratedRollup.DailyAbsErrorWhSum)
	}
	if calibratedRollup.BaselinePeakPowerAbsErrorWSum <= calibratedRollup.PeakPowerAbsErrorWSum {
		t.Fatalf("baseline peak power abs error = %v, want greater than served %v", calibratedRollup.BaselinePeakPowerAbsErrorWSum, calibratedRollup.PeakPowerAbsErrorWSum)
	}
	if calibratedRollup.BaselinePeakTimeAbsErrorMinutesSum <= calibratedRollup.PeakTimeAbsErrorMinutesSum {
		t.Fatalf("baseline peak time abs error = %v, want greater than served %v", calibratedRollup.BaselinePeakTimeAbsErrorMinutesSum, calibratedRollup.PeakTimeAbsErrorMinutesSum)
	}
}

func TestBuildRecentSiteCalibrationRequiresFullDayAndDedupesLatestIssue(t *testing.T) {
	t.Parallel()

	siteKey := "grid:42.61:-77.40:290|tilt:45|az:0|dev-a"
	forecastVersion := "deterministic_baseline_v1"
	baseIssuedAt := time.Date(2026, 3, 19, 8, 0, 0, 0, time.UTC)
	records := make([]VerificationRecord, 0, 25)
	for hour := 0; hour < 24; hour++ {
		target := time.Date(2026, 3, 18, hour, 0, 0, 0, time.UTC)
		records = append(records, VerificationRecord{
			HourlyTrainingRecord: HourlyTrainingRecord{
				RunID:                "run-new",
				SiteKey:              siteKey,
				IssuedAt:             baseIssuedAt,
				TargetTime:           target,
				TargetLocalDate:      parseDateISO("2026-03-18"),
				ForecastGenerationWh: 100,
				ActualGenerationWh:   float64Ptr(80),
				VerificationStatus:   VerificationStatusVerified,
				UpdatedAt:            baseIssuedAt.Add(48 * time.Hour),
			},
			ForecastVersion: forecastVersion,
			Timezone:        "UTC",
		})
	}
	records = append(records, VerificationRecord{
		HourlyTrainingRecord: HourlyTrainingRecord{
			RunID:                "run-old",
			SiteKey:              siteKey,
			IssuedAt:             baseIssuedAt.Add(-2 * time.Hour),
			TargetTime:           time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
			TargetLocalDate:      parseDateISO("2026-03-18"),
			ForecastGenerationWh: 100,
			ActualGenerationWh:   float64Ptr(20),
			VerificationStatus:   VerificationStatusVerified,
			UpdatedAt:            baseIssuedAt.Add(47 * time.Hour),
		},
		ForecastVersion: forecastVersion,
		Timezone:        "UTC",
	})

	got := BuildRecentSiteCalibration(records, forecastVersion)
	if got.MultiplicativeRatio == nil {
		t.Fatal("BuildRecentSiteCalibration() ratio = nil, want value")
	}
	if *got.MultiplicativeRatio != 0.8 {
		t.Fatalf("BuildRecentSiteCalibration() ratio = %v, want 0.8 after deduping latest issue", *got.MultiplicativeRatio)
	}
	if got.SampleCount != 24 {
		t.Fatalf("BuildRecentSiteCalibration() sample count = %d, want 24", got.SampleCount)
	}

	got = BuildRecentSiteCalibration(records[:23], forecastVersion)
	if got.MultiplicativeRatio != nil {
		t.Fatalf("BuildRecentSiteCalibration() ratio = %v, want nil before 24 verified hours", *got.MultiplicativeRatio)
	}
}

func TestBuildRecentSiteCalibrationAcceptsDSTShortenedDay(t *testing.T) {
	t.Parallel()

	loc := mustLocation(t, "America/New_York")
	siteKey := "grid:42.61:-77.40:290|tilt:45|az:0|dev-a"
	forecastVersion := "deterministic_baseline_v1"
	issuedAt := time.Date(2026, 3, 9, 8, 0, 0, 0, time.UTC)
	startLocal := time.Date(2026, 3, 8, 0, 0, 0, 0, loc)
	records := make([]VerificationRecord, 0, 23)
	for hour := 0; hour < 23; hour++ {
		targetLocal := startLocal.Add(time.Duration(hour) * time.Hour)
		targetUTC := targetLocal.UTC()
		records = append(records, VerificationRecord{
			HourlyTrainingRecord: HourlyTrainingRecord{
				RunID:                "run-dst",
				SiteKey:              siteKey,
				IssuedAt:             issuedAt,
				TargetTime:           targetUTC,
				TargetLocalDate:      parseDateISO("2026-03-08"),
				ForecastGenerationWh: 100,
				ActualGenerationWh:   float64Ptr(80),
				VerificationStatus:   VerificationStatusVerified,
				UpdatedAt:            issuedAt.Add(24 * time.Hour),
			},
			ForecastVersion: forecastVersion,
			Timezone:        loc.String(),
		})
	}

	got := BuildRecentSiteCalibration(records, forecastVersion)
	if got.MultiplicativeRatio == nil {
		t.Fatal("BuildRecentSiteCalibration() ratio = nil, want value for complete DST-shortened day")
	}
	if *got.MultiplicativeRatio != 0.8 {
		t.Fatalf("BuildRecentSiteCalibration() ratio = %v, want 0.8", *got.MultiplicativeRatio)
	}
	if got.SampleCount != 23 {
		t.Fatalf("BuildRecentSiteCalibration() sample count = %d, want 23", got.SampleCount)
	}
}

func TestGetSolarOutlookUsesRecentSiteCalibrationForFutureForecasts(t *testing.T) {
	t.Parallel()

	nowUTC := time.Date(2026, 3, 19, 15, 0, 0, 0, time.UTC)
	loc := mustLocation(t, "America/New_York")
	weather := &stubWeatherForecaster{
		bundle: testBundle(nowUTC, loc, "grid:42.61:-77.40:290|tilt:45|az:0"),
	}
	query := &stubTelemetryReader{
		series: telemetryquery.Series{
			DeviceID:   "dev-a",
			Resolution: telemetryquery.ResolutionHour,
			Points: []telemetryquery.Point{
				{
					BucketStart: nowUTC.Add(-1 * time.Hour),
					BucketEnd:   nowUTC,
					Metrics: telemetryquery.Metrics{
						PVMaxW:           float64Ptr(1000),
						SolarGeneratedWh: float64Ptr(600),
					},
				},
			},
		},
	}
	store := &capturingTrainingStore{
		runs: map[string]*Run{
			"verified-run": {
				ID:              "verified-run",
				SiteKey:         "grid:42.61:-77.40:290|tilt:45|az:0|dev-a",
				ForecastVersion: "deterministic_baseline_v1",
				Timezone:        "America/New_York",
				IssuedAt:        nowUTC.Add(-30 * time.Hour),
			},
		},
	}
	yesterdayLocal := time.Date(nowUTC.In(loc).Year(), nowUTC.In(loc).Month(), nowUTC.In(loc).Day()-1, 0, 0, 0, 0, loc)
	for hour := 0; hour < 24; hour++ {
		target := yesterdayLocal.Add(time.Duration(hour) * time.Hour).UTC()
		store.rows = append(store.rows, HourlyTrainingRecord{
			RunID:                "verified-run",
			SiteKey:              "grid:42.61:-77.40:290|tilt:45|az:0|dev-a",
			TargetTime:           target,
			TargetLocalDate:      parseDateISO(localDateISO(target, loc)),
			ForecastGenerationWh: 100,
			ActualGenerationWh:   float64Ptr(80),
			VerificationStatus:   VerificationStatusVerified,
			IssuedAt:             nowUTC.Add(-30 * time.Hour),
			UpdatedAt:            nowUTC.Add(-6 * time.Hour),
		})
	}
	store.rows = append(store.rows, HourlyTrainingRecord{
		RunID:                "verified-run",
		SiteKey:              "grid:42.61:-77.40:290|tilt:45|az:0|dev-a",
		TargetTime:           yesterdayLocal.Add(12 * time.Hour).UTC(),
		TargetLocalDate:      parseDateISO(localDateISO(yesterdayLocal.Add(12*time.Hour).UTC(), loc)),
		ForecastGenerationWh: 100,
		ActualGenerationWh:   float64Ptr(10),
		VerificationStatus:   VerificationStatusVerified,
		IssuedAt:             nowUTC.Add(-36 * time.Hour),
		UpdatedAt:            nowUTC.Add(-7 * time.Hour),
	})

	svc, err := NewService(weather, query, Config{
		Log:   slog.New(slog.NewTextHandler(testWriter{t}, nil)),
		Store: store,
		NowFn: func() time.Time { return nowUTC },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	outlook, err := svc.GetSolarOutlook(context.Background(), Input{
		WeatherRequest: weatherd.Request{
			Latitude:   42.61,
			Longitude:  -77.40,
			Timezone:   "America/New_York",
			UnitSystem: weatherd.UnitSystemMetric,
		},
		Scope: Scope{
			Mode:     "device",
			DeviceID: "dev-a",
		},
		ResolvedDeviceIDs: []string{"dev-a"},
	})
	if err != nil {
		t.Fatalf("GetSolarOutlook() error = %v", err)
	}
	if got, want := outlook.Provenance.ServedVariant, "site_calibrated"; got != want {
		t.Fatalf("outlook.Provenance.ServedVariant = %q, want %q", got, want)
	}
	if got, want := outlook.Provenance.CalibrationSampleCount, 24; got != want {
		t.Fatalf("outlook.Provenance.CalibrationSampleCount = %d, want %d", got, want)
	}

	futurePoint := weather.bundle.Hourly[0]
	baseline := estimateForecastWatts(futurePoint, outlook.Capacity.EstimatedPeakWatts, nowUTC, loc, nil, nil)
	if baseline == nil {
		t.Fatal("estimateForecastWatts(baseline) = nil, want value")
	}
	want := round1(*baseline * 0.8)
	got := valueOrZero(outlook.Next24Hours[0].ForecastGeneratedWh)
	if got != want {
		t.Fatalf("outlook.Next24Hours[0].ForecastGeneratedWh = %v, want %v", got, want)
	}
}

type stubWeatherForecaster struct {
	bundle *weatherd.Bundle
	err    error
}

func (s *stubWeatherForecaster) Get7DayForecast(_ context.Context, _ weatherd.Request) (*weatherd.Bundle, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.bundle, nil
}

type stubTelemetryReader struct {
	series           telemetryquery.Series
	aggregateSeries  telemetryquery.Series
	queryRangeFn     func(context.Context, telemetryquery.RangeQuery) (telemetryquery.Series, error)
	queryRangeManyFn func(context.Context, telemetryquery.AggregateRangeQuery) (telemetryquery.Series, error)
	err              error
}

func (s *stubTelemetryReader) QueryRange(ctx context.Context, q telemetryquery.RangeQuery) (telemetryquery.Series, error) {
	if s.err != nil {
		return telemetryquery.Series{}, s.err
	}
	if s.queryRangeFn != nil {
		return s.queryRangeFn(ctx, q)
	}
	return s.series, nil
}

func (s *stubTelemetryReader) QueryRangeMany(ctx context.Context, q telemetryquery.AggregateRangeQuery) (telemetryquery.Series, error) {
	if s.err != nil {
		return telemetryquery.Series{}, s.err
	}
	if s.queryRangeManyFn != nil {
		return s.queryRangeManyFn(ctx, q)
	}
	return s.aggregateSeries, nil
}

func (s *stubTelemetryReader) Close() error {
	return nil
}

type capturingTrainingStore struct {
	run               *Run
	runs              map[string]*Run
	rows              []HourlyTrainingRecord
	completedRows     []HourlyTrainingRecord
	rollups           []DailyVerificationRollup
	calibrationStates []CalibrationState
	insertRunErr      error
	insertRowsErr     error
}

func (s *capturingTrainingStore) InsertRun(_ context.Context, run Run) error {
	runCopy := run
	s.run = &runCopy
	if s.runs == nil {
		s.runs = make(map[string]*Run)
	}
	s.runs[run.ID] = &runCopy
	if s.insertRunErr != nil {
		return s.insertRunErr
	}
	return nil
}

func (s *capturingTrainingStore) InsertHourlyRecords(_ context.Context, rows []HourlyTrainingRecord) error {
	s.rows = append([]HourlyTrainingRecord(nil), rows...)
	if s.insertRowsErr != nil {
		return s.insertRowsErr
	}
	return nil
}

func (s *capturingTrainingStore) ListPendingHourlyRecords(_ context.Context, before time.Time, limit int) ([]HourlyTrainingRecord, error) {
	out := make([]HourlyTrainingRecord, 0)
	for _, row := range s.rows {
		if row.VerificationStatus != VerificationStatusPending {
			continue
		}
		if !row.TargetTime.Before(before) {
			continue
		}
		out = append(out, row)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *capturingTrainingStore) ListVerificationRecords(_ context.Context, siteKey string, fromDate, toDate time.Time) ([]VerificationRecord, error) {
	out := make([]VerificationRecord, 0)
	for _, row := range s.rows {
		if row.SiteKey != siteKey {
			continue
		}
		if row.TargetLocalDate.Before(fromDate) || row.TargetLocalDate.After(toDate) {
			continue
		}
		record := VerificationRecord{HourlyTrainingRecord: row}
		if run := s.lookupRun(row.RunID); run != nil {
			record.ForecastVersion = run.ForecastVersion
			record.ServedVariant = run.ServedVariant
			record.Timezone = run.Timezone
		}
		out = append(out, record)
	}
	return out, nil
}

func (s *capturingTrainingStore) LoadCalibrationStates(_ context.Context, siteKey, forecastVersion string) ([]CalibrationState, error) {
	out := make([]CalibrationState, 0)
	for _, state := range s.calibrationStates {
		if state.SiteKey != siteKey || state.ForecastVersion != forecastVersion {
			continue
		}
		out = append(out, state)
	}
	return out, nil
}

func (s *capturingTrainingStore) UpsertCalibrationStates(_ context.Context, states []CalibrationState) error {
	s.calibrationStates = append([]CalibrationState(nil), states...)
	return nil
}

func (s *capturingTrainingStore) CompleteHourlyVerification(_ context.Context, rows []HourlyTrainingRecord) error {
	s.completedRows = append([]HourlyTrainingRecord(nil), rows...)
	index := make(map[string]HourlyTrainingRecord, len(rows))
	for _, row := range rows {
		index[row.RunID+"|"+row.TargetTime.UTC().Format(time.RFC3339)] = row
	}
	for idx := range s.rows {
		key := s.rows[idx].RunID + "|" + s.rows[idx].TargetTime.UTC().Format(time.RFC3339)
		if updated, ok := index[key]; ok {
			s.rows[idx] = updated
		}
	}
	return nil
}

func (s *capturingTrainingStore) UpsertDailyVerificationRollup(_ context.Context, row DailyVerificationRollup) error {
	s.rollups = append(s.rollups, row)
	return nil
}

func (s *capturingTrainingStore) GetRun(_ context.Context, id string) (*Run, error) {
	if run := s.lookupRun(id); run != nil {
		return run, nil
	}
	return nil, nil
}

func (s *capturingTrainingStore) Close() error {
	return nil
}

func (s *capturingTrainingStore) lookupRun(id string) *Run {
	if s.runs == nil {
		return s.run
	}
	return s.runs[id]
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", string(p))
	return len(p), nil
}

func testBundle(nowUTC time.Time, loc *time.Location, canonicalKey string) *weatherd.Bundle {
	todayLocal := time.Date(nowUTC.In(loc).Year(), nowUTC.In(loc).Month(), nowUTC.In(loc).Day(), 0, 0, 0, 0, loc)
	tomorrowLocal := todayLocal.AddDate(0, 0, 1)
	return &weatherd.Bundle{
		Provenance: weatherd.Provenance{
			Source:               "open_meteo",
			ModelSelection:       "best_match",
			ActualSource:         "past_days",
			Timezone:             loc.String(),
			CanonicalLocationKey: canonicalKey,
			IssuedAt:             nowUTC,
			Latitude:             42.61,
			Longitude:            -77.40,
			Elevation:            290,
		},
		Hourly: []weatherd.HourlyForecastPoint{
			{
				Time: tomorrowLocal.Add(10 * time.Hour).UTC(),
				Raw: weatherd.ForecastValueSet{
					ShortwaveRadiation: float64Ptr(650),
					CloudCover:         float64Ptr(20),
					Temperature:        float64Ptr(17),
				},
			},
			{
				Time: tomorrowLocal.Add(14 * time.Hour).UTC(),
				Raw: weatherd.ForecastValueSet{
					GlobalTiltedIrradiance: float64Ptr(820),
					ShortwaveRadiation:     float64Ptr(760),
					CloudCover:             float64Ptr(10),
					Temperature:            float64Ptr(21),
				},
			},
		},
		Daily: []weatherd.DailyForecastPoint{
			{
				Date: tomorrowLocal.UTC(),
			},
		},
	}
}

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q) error = %v", name, err)
	}
	return loc
}

func float64Ptr(value float64) *float64 {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
