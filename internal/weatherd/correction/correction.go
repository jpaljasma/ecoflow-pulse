package correction

import (
	"math"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

const (
	defaultEWMAAlpha = 2.0 / 15.0
	minRatioClamp    = 0.25
	maxRatioClamp    = 4.0
)

type BiasIndex map[weatherd.BiasMetric]map[int]weatherd.BiasState

func BuildIndex(states []weatherd.BiasState) BiasIndex {
	out := make(BiasIndex)
	for _, state := range states {
		byHour := out[state.Metric]
		if byHour == nil {
			byHour = map[int]weatherd.BiasState{}
			out[state.Metric] = byHour
		}
		byHour[state.HourOfDay] = state
	}
	return out
}

func ApplyForecast(point weatherd.HourlyForecastPoint, loc *time.Location, index BiasIndex) weatherd.HourlyForecastPoint {
	hour := point.Time.In(loc).Hour()
	out := point
	out.Corrected = weatherd.ForecastValueSet{
		Temperature:             applyAdditive(point.Raw.Temperature, lookup(index, weatherd.BiasMetricTemperature, hour).AdditiveBias),
		WindSpeed:               applyAdditive(point.Raw.WindSpeed, lookup(index, weatherd.BiasMetricWindSpeed, hour).AdditiveBias),
		WindDirectionDegrees:    point.Raw.WindDirectionDegrees,
		Precipitation:           point.Raw.Precipitation,
		CloudCover:              applyAdditive(point.Raw.CloudCover, lookup(index, weatherd.BiasMetricCloudCover, hour).AdditiveBias),
		Visibility:              applyAdditive(point.Raw.Visibility, lookup(index, weatherd.BiasMetricVisibility, hour).AdditiveBias),
		SunshineDurationSeconds: point.Raw.SunshineDurationSeconds,
		ShortwaveRadiation:      applyRatio(point.Raw.ShortwaveRadiation, lookup(index, weatherd.BiasMetricShortwaveRadiation, hour).MultiplicativeRatio),
		UVIndex:                 applyAdditive(point.Raw.UVIndex, lookup(index, weatherd.BiasMetricUVIndex, hour).AdditiveBias),
		GlobalTiltedIrradiance:  applyRatio(point.Raw.GlobalTiltedIrradiance, lookup(index, weatherd.BiasMetricGlobalTiltedIrradiance, hour).MultiplicativeRatio),
	}
	return out
}

func UpdateStates(
	now time.Time,
	canonicalLocationKey string,
	forecast weatherd.ForecastValueSet,
	actual weatherd.ForecastValueSet,
	hourOfDay int,
	existing []weatherd.BiasState,
) []weatherd.BiasState {
	index := BuildIndex(existing)
	metrics := []weatherd.BiasMetric{
		weatherd.BiasMetricTemperature,
		weatherd.BiasMetricWindSpeed,
		weatherd.BiasMetricCloudCover,
		weatherd.BiasMetricVisibility,
		weatherd.BiasMetricUVIndex,
		weatherd.BiasMetricShortwaveRadiation,
	}
	if forecast.GlobalTiltedIrradiance != nil || actual.GlobalTiltedIrradiance != nil {
		metrics = append(metrics, weatherd.BiasMetricGlobalTiltedIrradiance)
	}
	out := make([]weatherd.BiasState, 0, len(metrics))
	for _, metric := range metrics {
		state := lookup(index, metric, hourOfDay)
		state.CanonicalLocationKey = canonicalLocationKey
		state.Metric = metric
		state.HourOfDay = hourOfDay
		state.SampleCount++
		state.UpdatedAt = now.UTC()
		switch metric {
		case weatherd.BiasMetricShortwaveRadiation, weatherd.BiasMetricGlobalTiltedIrradiance:
			ratio := ratioFor(metric, forecast, actual)
			if ratio == nil {
				continue
			}
			next := ewmaRatio(state.MultiplicativeRatio, *ratio)
			state.MultiplicativeRatio = &next
			state.AdditiveBias = nil
		default:
			bias := additiveFor(metric, forecast, actual)
			if bias == nil {
				continue
			}
			next := ewmaAdditive(state.AdditiveBias, *bias)
			state.AdditiveBias = &next
			state.MultiplicativeRatio = nil
		}
		out = append(out, state)
	}
	return out
}

func CircularWindDirectionMAE(diffs []float64) *float64 {
	if len(diffs) == 0 {
		return nil
	}
	sum := 0.0
	for _, diff := range diffs {
		sum += math.Abs(normalizeDegrees(diff))
	}
	out := sum / float64(len(diffs))
	return &out
}

func CircularErrorDegrees(forecast, actual float64) float64 {
	return normalizeDegrees(actual - forecast)
}

func lookup(index BiasIndex, metric weatherd.BiasMetric, hour int) weatherd.BiasState {
	if index == nil {
		return weatherd.BiasState{}
	}
	return index[metric][hour]
}

func applyAdditive(raw *float64, bias *float64) *float64 {
	if raw == nil {
		return nil
	}
	out := *raw
	if bias != nil {
		out += *bias
	}
	return &out
}

func applyRatio(raw *float64, ratio *float64) *float64 {
	if raw == nil {
		return nil
	}
	out := *raw
	if ratio != nil {
		out *= clamp(*ratio, minRatioClamp, maxRatioClamp)
	}
	return &out
}

func ewmaAdditive(current *float64, sample float64) float64 {
	if current == nil {
		return sample
	}
	return (defaultEWMAAlpha * sample) + ((1 - defaultEWMAAlpha) * (*current))
}

func ewmaRatio(current *float64, sample float64) float64 {
	sample = clamp(sample, minRatioClamp, maxRatioClamp)
	if current == nil {
		return sample
	}
	return clamp((defaultEWMAAlpha*sample)+((1-defaultEWMAAlpha)*(*current)), minRatioClamp, maxRatioClamp)
}

func additiveFor(metric weatherd.BiasMetric, forecast weatherd.ForecastValueSet, actual weatherd.ForecastValueSet) *float64 {
	var lhs, rhs *float64
	switch metric {
	case weatherd.BiasMetricTemperature:
		lhs, rhs = forecast.Temperature, actual.Temperature
	case weatherd.BiasMetricWindSpeed:
		lhs, rhs = forecast.WindSpeed, actual.WindSpeed
	case weatherd.BiasMetricCloudCover:
		lhs, rhs = forecast.CloudCover, actual.CloudCover
	case weatherd.BiasMetricVisibility:
		lhs, rhs = forecast.Visibility, actual.Visibility
	case weatherd.BiasMetricUVIndex:
		lhs, rhs = forecast.UVIndex, actual.UVIndex
	default:
		return nil
	}
	if lhs == nil || rhs == nil {
		return nil
	}
	out := *rhs - *lhs
	return &out
}

func ratioFor(metric weatherd.BiasMetric, forecast weatherd.ForecastValueSet, actual weatherd.ForecastValueSet) *float64 {
	var lhs, rhs *float64
	switch metric {
	case weatherd.BiasMetricShortwaveRadiation:
		lhs, rhs = forecast.ShortwaveRadiation, actual.ShortwaveRadiation
	case weatherd.BiasMetricGlobalTiltedIrradiance:
		lhs, rhs = forecast.GlobalTiltedIrradiance, actual.GlobalTiltedIrradiance
	default:
		return nil
	}
	if lhs == nil || rhs == nil || *lhs == 0 {
		return nil
	}
	out := *rhs / *lhs
	return &out
}

func normalizeDegrees(v float64) float64 {
	for v <= -180 {
		v += 360
	}
	for v > 180 {
		v -= 360
	}
	return v
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
