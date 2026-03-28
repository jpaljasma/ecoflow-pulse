package solarforecastd

import (
	"context"
	"log/slog"
	"time"
)

const (
	solarForecastStageWeatherFetch      = "weather_fetch"
	solarForecastStageTelemetryLookback = "telemetry_lookback"
	solarForecastStageCalibrationLoads  = "calibration_loads"
	solarForecastStageSummarization     = "summarization"
	solarForecastStageTrainingKickoff   = "training_kickoff"
)

type solarForecastStageTimings struct {
	weatherFetch      time.Duration
	telemetryLookback time.Duration
	calibrationLoads  time.Duration
	summarization     time.Duration
	trainingKickoff   time.Duration
}

func (s *Service) observeSolarStageTiming(scope, stage string, err error, duration time.Duration) {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.ObserveStageTiming(scope, stage, err, duration)
}

func (s *Service) logSolarStageTimings(ctx context.Context, scope string, err error, timings solarForecastStageTimings, total time.Duration) {
	if s == nil || s.log == nil || !s.log.Enabled(ctx, slog.LevelDebug) {
		return
	}
	s.log.DebugContext(ctx, "solar forecast stage timings",
		"scope", scopeLabel(scope),
		"result", resultLabel(err),
		"total_ms", total.Milliseconds(),
		"weather_fetch_ms", timings.weatherFetch.Milliseconds(),
		"telemetry_lookback_ms", timings.telemetryLookback.Milliseconds(),
		"calibration_loads_ms", timings.calibrationLoads.Milliseconds(),
		"summarization_ms", timings.summarization.Milliseconds(),
		"training_kickoff_ms", timings.trainingKickoff.Milliseconds(),
	)
}
