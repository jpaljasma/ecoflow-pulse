package solarforecastd

import (
	"math"
	"sort"
	"time"
)

const (
	defaultCalibrationEWMAAlpha           = 2.0 / 15.0
	minCalibrationRatioClamp              = 0.2
	maxCalibrationRatioClamp              = 1.8
	minCalibrationSamples                 = 3
	minCalibrationForecastWh              = 50.0
	minRecentSiteCalibrationSignalHours   = 6
	minRecentSiteCalibrationForecastWhSum = 400.0
)

type CalibrationIndex map[HorizonBucket]map[int]CalibrationState

type RecentSiteCalibration struct {
	MultiplicativeRatio *float64
	SampleCount         int
	UpdatedAt           *time.Time
}

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

func hasUsableCalibrationState(state CalibrationState) bool {
	return state.MultiplicativeRatio != nil && state.SampleCount >= minCalibrationSamples
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

func BuildRecentSiteCalibration(records []VerificationRecord, forecastVersion string) RecentSiteCalibration {
	if len(records) == 0 {
		return RecentSiteCalibration{}
	}
	latestByTarget := make(map[time.Time]VerificationRecord, len(records))
	for _, record := range records {
		if record.ForecastVersion != forecastVersion {
			continue
		}
		if record.VerificationStatus != VerificationStatusVerified || record.ActualGenerationWh == nil {
			continue
		}
		key := record.TargetTime.UTC()
		existing, ok := latestByTarget[key]
		if ok && !record.IssuedAt.After(existing.IssuedAt) {
			continue
		}
		latestByTarget[key] = record
	}
	if !hasCompleteRecentCalibrationDay(latestByTarget) {
		return RecentSiteCalibration{}
	}
	keys := make([]time.Time, 0, len(latestByTarget))
	for key := range latestByTarget {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })

	sampleCount := len(keys)
	signalHours := 0
	forecastWhSum := 0.0
	actualWhSum := 0.0
	var latest time.Time
	for _, key := range keys {
		record := latestByTarget[key]
		if record.ForecastGenerationWh < minCalibrationForecastWh {
			continue
		}
		signalHours++
		forecastWhSum += record.ForecastGenerationWh
		actualWhSum += *record.ActualGenerationWh
		if latest.IsZero() || record.UpdatedAt.After(latest) {
			latest = record.UpdatedAt.UTC()
		}
	}
	if signalHours < minRecentSiteCalibrationSignalHours || forecastWhSum < minRecentSiteCalibrationForecastWhSum {
		return RecentSiteCalibration{}
	}
	ratio := clampCalibrationRatio(actualWhSum / forecastWhSum)
	var updatedAt *time.Time
	if !latest.IsZero() {
		updatedAt = &latest
	}
	return RecentSiteCalibration{
		MultiplicativeRatio: &ratio,
		SampleCount:         sampleCount,
		UpdatedAt:           updatedAt,
	}
}

func hasCompleteRecentCalibrationDay(records map[time.Time]VerificationRecord) bool {
	if len(records) == 0 {
		return false
	}
	hoursByDate := make(map[string]map[time.Time]struct{})
	timezoneByDate := make(map[string]string)
	for _, record := range records {
		dateISO := record.TargetLocalDate.Format("2006-01-02")
		if hoursByDate[dateISO] == nil {
			hoursByDate[dateISO] = make(map[time.Time]struct{})
		}
		hoursByDate[dateISO][record.TargetTime.UTC()] = struct{}{}
		timezoneByDate[dateISO] = record.Timezone
	}
	for dateISO, hours := range hoursByDate {
		if len(hours) >= expectedLocalDayHours(dateISO, timezoneByDate[dateISO]) {
			return true
		}
	}
	return false
}

func expectedLocalDayHours(dateISO string, timezone string) int {
	loc := time.UTC
	trimmed := timezone
	if trimmed != "" {
		if loaded, err := time.LoadLocation(trimmed); err == nil {
			loc = loaded
		}
	}
	day, err := time.Parse("2006-01-02", dateISO)
	if err != nil {
		return 24
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	return int(start.AddDate(0, 0, 1).Sub(start).Hours())
}
