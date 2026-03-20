package solarforecastd

import (
	"math"
	"time"
)

const (
	defaultCalibrationEWMAAlpha = 2.0 / 15.0
	minCalibrationRatioClamp    = 0.2
	maxCalibrationRatioClamp    = 1.8
	minCalibrationSamples       = 3
	minCalibrationForecastWh    = 50.0
)

type CalibrationIndex map[HorizonBucket]map[int]CalibrationState

func BuildCalibrationIndex(states []CalibrationState) CalibrationIndex {
	out := make(CalibrationIndex)
	for _, state := range states {
		byHour := out[state.HorizonBucket]
		if byHour == nil {
			byHour = map[int]CalibrationState{}
			out[state.HorizonBucket] = byHour
		}
		byHour[state.HourOfDay] = state
	}
	return out
}

func ApplyGenerationCalibration(value *float64, state CalibrationState) *float64 {
	if value == nil || state.MultiplicativeRatio == nil || state.SampleCount < minCalibrationSamples {
		return value
	}
	out := *value * clampCalibrationRatio(*state.MultiplicativeRatio)
	return floatPtr(round1(out))
}

func UpdateCalibrationStates(now time.Time, siteKey, forecastVersion string, rows []HourlyTrainingRecord, existing []CalibrationState) []CalibrationState {
	index := BuildCalibrationIndex(existing)
	out := make([]CalibrationState, 0, len(rows))
	for _, row := range rows {
		if row.VerificationStatus != VerificationStatusVerified || row.ActualGenerationWh == nil {
			continue
		}
		ratio := calibrationRatioForRow(row)
		if ratio == nil {
			continue
		}
		state := lookupCalibration(index, row.HorizonBucket, row.TargetLocalHour)
		state.SiteKey = siteKey
		state.ForecastVersion = forecastVersion
		state.HorizonBucket = row.HorizonBucket
		state.HourOfDay = row.TargetLocalHour
		state.SampleCount++
		next := ewmaCalibrationRatio(state.MultiplicativeRatio, *ratio)
		state.MultiplicativeRatio = &next
		state.UpdatedAt = now.UTC()
		out = append(out, state)
	}
	return out
}

func lookupCalibration(index CalibrationIndex, horizon HorizonBucket, hour int) CalibrationState {
	if index == nil {
		return CalibrationState{}
	}
	return index[horizon][hour]
}

func calibrationRatioForRow(row HourlyTrainingRecord) *float64 {
	if row.ActualGenerationWh == nil || row.ForecastGenerationWh < minCalibrationForecastWh {
		return nil
	}
	ratio := *row.ActualGenerationWh / row.ForecastGenerationWh
	return &ratio
}

func ewmaCalibrationRatio(current *float64, sample float64) float64 {
	sample = clampCalibrationRatio(sample)
	if current == nil {
		return sample
	}
	return clampCalibrationRatio((defaultCalibrationEWMAAlpha * sample) + ((1 - defaultCalibrationEWMAAlpha) * (*current)))
}

func clampCalibrationRatio(value float64) float64 {
	return math.Min(maxCalibrationRatioClamp, math.Max(minCalibrationRatioClamp, value))
}
