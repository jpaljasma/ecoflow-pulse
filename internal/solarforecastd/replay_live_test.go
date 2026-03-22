package solarforecastd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

type replayRun struct {
	ID                  string
	SiteKey             string
	DeviceID            *string
	Timezone            string
	IssuedAt            time.Time
	IssueLocalDate      time.Time
	ForecastVersion     string
	ForecastTotalToday  float64
	ForecastRemainToday float64
	ActualSoFar         float64
	SiteMetadataJSON    []byte
}

type replayHour struct {
	TargetTime           time.Time
	TargetLocalDate      time.Time
	ForecastGenerationWh float64
	ActualGenerationWh   *float64
	WeatherRaw           weatherd.ForecastValueSet
	WeatherCorrected     weatherd.ForecastValueSet
}

type replayResult struct {
	Date              string
	IssuedAtLocal     string
	ActualWh          float64
	OldForecastWh     float64
	OldDeltaWh        float64
	OldDeltaPct       float64
	OldDisplayedPeakW float64
	NewForecastWh     float64
	NewDeltaWh        float64
	NewDeltaPct       float64
	NewDisplayedPeakW float64
	CapacityW         float64
	CapacityMethod    string
}

func TestReplayRecentSolarRunsAgainstCurrentBranch(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("SOLAR_REPLAY_DSN"))
	if dsn == "" {
		t.Skip("set SOLAR_REPLAY_DSN to run live replay")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open replay db: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping replay db: %v", err)
	}
	queryReader, err := telemetryquery.NewPostgresReader(dsn)
	if err != nil {
		t.Fatalf("new telemetry query reader: %v", err)
	}
	defer func() { _ = queryReader.Close() }()
	runs, err := loadReplayRuns(ctx, db, 3)
	if err != nil {
		t.Fatalf("load replay runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("no completed replay runs found")
	}

	results := make([]replayResult, 0, len(runs))
	for _, run := range runs {
		result, err := replayRunWithCurrentBranch(ctx, db, queryReader, run)
		if err != nil {
			t.Fatalf("replay run %s: %v", run.ID, err)
		}
		results = append(results, result)
	}

	t.Log("| Date | Issue local | Actual kWh | Old forecast kWh | Old delta | Old peak kW | New replay kWh | New delta | New peak kW | New capacity kW | Method |")
	t.Log("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |")
	for _, row := range results {
		t.Logf(
			"| %s | %s | %.2f | %.2f | %+0.2f%% | %.2f | %.2f | %+0.2f%% | %.2f | %.2f | %s |",
			row.Date,
			row.IssuedAtLocal,
			row.ActualWh/1000,
			row.OldForecastWh/1000,
			row.OldDeltaPct,
			row.OldDisplayedPeakW/1000,
			row.NewForecastWh/1000,
			row.NewDeltaPct,
			row.NewDisplayedPeakW/1000,
			row.CapacityW/1000,
			row.CapacityMethod,
		)
	}
}

func loadReplayRuns(ctx context.Context, db *sql.DB, dayCount int) ([]replayRun, error) {
	if dayCount <= 0 {
		dayCount = 3
	}
	rows, err := db.QueryContext(ctx, `
WITH completed_days AS (
  SELECT DISTINCT issue_local_date
  FROM solar_forecast_runs
  WHERE issue_local_date < CURRENT_DATE
  ORDER BY issue_local_date DESC
  LIMIT $1
),
ranked AS (
  SELECT
    r.id::text,
    r.site_key,
    r.device_id::text,
    r.timezone,
    r.issued_at,
    r.issue_local_date,
    r.forecast_version,
    r.forecast_total_today_wh,
    r.forecast_remaining_today_wh,
    r.actual_so_far_wh,
    r.site_metadata_json,
    ROW_NUMBER() OVER (
      PARTITION BY r.issue_local_date
      ORDER BY ABS(r.issue_local_hour - 14) ASC, r.issued_at DESC
    ) AS rn
  FROM solar_forecast_runs r
  JOIN completed_days d ON d.issue_local_date = r.issue_local_date
)
SELECT
  id,
  site_key,
  device_id,
  timezone,
  issued_at,
  issue_local_date,
  forecast_version,
  forecast_total_today_wh,
  forecast_remaining_today_wh,
  actual_so_far_wh,
  site_metadata_json
FROM ranked
WHERE rn = 1
ORDER BY issue_local_date DESC;
`, dayCount)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]replayRun, 0, dayCount)
	for rows.Next() {
		var (
			row      replayRun
			deviceID sql.NullString
		)
		if err := rows.Scan(
			&row.ID,
			&row.SiteKey,
			&deviceID,
			&row.Timezone,
			&row.IssuedAt,
			&row.IssueLocalDate,
			&row.ForecastVersion,
			&row.ForecastTotalToday,
			&row.ForecastRemainToday,
			&row.ActualSoFar,
			&row.SiteMetadataJSON,
		); err != nil {
			return nil, err
		}
		if deviceID.Valid && strings.TrimSpace(deviceID.String) != "" {
			id := strings.TrimSpace(deviceID.String)
			row.DeviceID = &id
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func replayRunWithCurrentBranch(
	ctx context.Context,
	db *sql.DB,
	queryReader telemetryquery.Reader,
	run replayRun,
) (replayResult, error) {
	loc := loadLocation(run.Timezone)
	issuedAtUTC := run.IssuedAt.UTC()
	nowLocal := issuedAtUTC.In(loc)
	deviceIDs := replayResolvedDeviceIDs(run)
	if len(deviceIDs) == 0 && run.DeviceID != nil {
		deviceIDs = []string{*run.DeviceID}
	}
	if len(deviceIDs) == 0 {
		return replayResult{}, fmt.Errorf("run %s has no resolved device ids", run.ID)
	}

	hours, err := loadReplayHours(ctx, db, run.ID)
	if err != nil {
		return replayResult{}, err
	}
	if len(hours) == 0 {
		return replayResult{}, fmt.Errorf("run %s has no hourly rows", run.ID)
	}
	hourlyPoints := make([]weatherd.HourlyForecastPoint, 0, len(hours))
	uniqueDates := make(map[string]struct{}, len(hours))
	rawFutureWh := 0.0
	oldDisplayedPeakWh := 0.0
	todayISO := localDateISO(nowLocal, loc)
	for _, row := range hours {
		hourlyPoints = append(hourlyPoints, weatherd.HourlyForecastPoint{
			Time:      row.TargetTime.UTC(),
			Raw:       row.WeatherRaw,
			Corrected: row.WeatherCorrected,
		})
		dateISO := row.TargetLocalDate.UTC().Format("2006-01-02")
		uniqueDates[dateISO] = struct{}{}
		if dateISO == todayISO {
			rawFutureWh += row.ForecastGenerationWh
		}
	}
	oldScale := 1.0
	if rawFutureWh > 0 && run.ForecastRemainToday > 0 {
		oldScale = run.ForecastRemainToday / rawFutureWh
	}
	for _, row := range hours {
		if row.TargetLocalDate.UTC().Format("2006-01-02") != todayISO {
			continue
		}
		displayed := row.ForecastGenerationWh * oldScale
		if displayed > oldDisplayedPeakWh {
			oldDisplayedPeakWh = displayed
		}
	}
	dailyPoints := make([]weatherd.DailyForecastPoint, 0, len(uniqueDates))
	dateISOs := make([]string, 0, len(uniqueDates))
	for dateISO := range uniqueDates {
		dateISOs = append(dateISOs, dateISO)
	}
	sort.Strings(dateISOs)
	for _, dateISO := range dateISOs {
		dailyPoints = append(dailyPoints, weatherd.DailyForecastPoint{
			Date: parseDateISO(dateISO),
		})
	}

	todayStartLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	lookbackStartLocal := todayStartLocal.AddDate(0, 0, -6)
	svc := &Service{query: queryReader}
	history, err := svc.querySeries(ctx, deviceIDs, lookbackStartLocal.UTC(), issuedAtUTC, 24*8)
	if err != nil {
		return replayResult{}, err
	}
	capacity := inferCapacityEstimate(history.Points, currentWeatherPoint(hourlyPoints, issuedAtUTC))

	calibrationStates, recentSiteCalibration, err := reconstructCalibrationAsOf(ctx, db, run, issuedAtUTC)
	if err != nil {
		return replayResult{}, err
	}
	calibrationIndex := BuildCalibrationIndex(calibrationStates)
	todayRemainingScale := deriveTodayRemainingScale(
		history,
		hourlyPoints,
		capacity.EstimatedPeakWatts,
		run.ActualSoFar,
		todayISO,
		issuedAtUTC,
		loc,
		calibrationIndex,
		recentSiteCalibration.MultiplicativeRatio,
	)
	daily := summarizeDailyOutlook(
		hourlyPoints,
		dailyPoints,
		capacity.EstimatedPeakWatts,
		run.ActualSoFar,
		todayISO,
		todayRemainingScale,
		issuedAtUTC,
		loc,
		calibrationIndex,
		recentSiteCalibration.MultiplicativeRatio,
	)
	today := firstDayForISO(daily, todayISO)
	newForecastWh := kwhToWh(today.ForecastTotalKWh)
	newPeakWh := valueOrZero(today.EstimatedPeakWatts)

	oldDeltaWh := run.ForecastTotalToday - run.ActualSoFar
	newDeltaWh := newForecastWh - run.ActualSoFar

	return replayResult{
		Date:              todayISO,
		IssuedAtLocal:     issuedAtUTC.In(loc).Format("15:04"),
		ActualWh:          run.ActualSoFar,
		OldForecastWh:     run.ForecastTotalToday,
		OldDeltaWh:        oldDeltaWh,
		OldDeltaPct:       pctDelta(oldDeltaWh, run.ActualSoFar),
		OldDisplayedPeakW: oldDisplayedPeakWh,
		NewForecastWh:     newForecastWh,
		NewDeltaWh:        newDeltaWh,
		NewDeltaPct:       pctDelta(newDeltaWh, run.ActualSoFar),
		NewDisplayedPeakW: newPeakWh,
		CapacityW:         valueOrZero(capacity.EstimatedPeakWatts),
		CapacityMethod:    capacity.Method,
	}, nil
}

func loadReplayHours(ctx context.Context, db *sql.DB, runID string) ([]replayHour, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	target_time,
	target_local_date,
	forecast_generation_wh,
	actual_generation_wh,
	weather_raw_json,
	weather_corrected_json
FROM solar_forecast_hourly_training_records
WHERE run_id = $1::uuid
ORDER BY target_time ASC;
`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]replayHour, 0)
	for rows.Next() {
		var (
			row           replayHour
			actual        sql.NullFloat64
			rawJSON       []byte
			correctedJSON []byte
		)
		if err := rows.Scan(&row.TargetTime, &row.TargetLocalDate, &row.ForecastGenerationWh, &actual, &rawJSON, &correctedJSON); err != nil {
			return nil, err
		}
		if actual.Valid {
			value := actual.Float64
			row.ActualGenerationWh = &value
		}
		if len(rawJSON) > 0 {
			if err := json.Unmarshal(rawJSON, &row.WeatherRaw); err != nil {
				return nil, err
			}
		}
		if len(correctedJSON) > 0 {
			if err := json.Unmarshal(correctedJSON, &row.WeatherCorrected); err != nil {
				return nil, err
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func reconstructCalibrationAsOf(ctx context.Context, db *sql.DB, run replayRun, issuedAt time.Time) ([]CalibrationState, RecentSiteCalibration, error) {
	fromDate := run.IssueLocalDate.AddDate(0, 0, -14)
	records, err := loadReplayVerificationRecords(ctx, db, run.SiteKey, fromDate, run.IssueLocalDate)
	if err != nil {
		return nil, RecentSiteCalibration{}, err
	}
	filtered := make([]VerificationRecord, 0, len(records))
	for _, record := range records {
		if record.ForecastVersion != run.ForecastVersion {
			continue
		}
		if record.UpdatedAt.After(issuedAt) {
			continue
		}
		filtered = append(filtered, record)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].UpdatedAt.Equal(filtered[j].UpdatedAt) {
			return filtered[i].TargetTime.Before(filtered[j].TargetTime)
		}
		return filtered[i].UpdatedAt.Before(filtered[j].UpdatedAt)
	})
	index := make(CalibrationIndex)
	for _, record := range filtered {
		if record.VerificationStatus != VerificationStatusVerified || record.ActualGenerationWh == nil {
			continue
		}
		ratio := calibrationRatioForRow(record.HourlyTrainingRecord)
		if ratio == nil {
			continue
		}
		state := lookupCalibration(index, record.HorizonBucket, record.TargetLocalHour)
		state.SiteKey = run.SiteKey
		state.ForecastVersion = run.ForecastVersion
		state.HorizonBucket = record.HorizonBucket
		state.HourOfDay = record.TargetLocalHour
		state.SampleCount++
		next := ewmaCalibrationRatio(state.MultiplicativeRatio, *ratio)
		state.MultiplicativeRatio = &next
		state.UpdatedAt = record.UpdatedAt.UTC()
		byHour := index[state.HorizonBucket]
		if byHour == nil {
			byHour = map[int]CalibrationState{}
			index[state.HorizonBucket] = byHour
		}
		byHour[state.HourOfDay] = state
	}
	states := flattenCalibrationIndex(index)
	recent := BuildRecentSiteCalibration(filtered, run.ForecastVersion)
	return states, recent, nil
}

func flattenCalibrationIndex(index CalibrationIndex) []CalibrationState {
	out := make([]CalibrationState, 0)
	for _, byHour := range index {
		for _, state := range byHour {
			out = append(out, state)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].HorizonBucket == out[j].HorizonBucket {
			return out[i].HourOfDay < out[j].HourOfDay
		}
		return out[i].HorizonBucket < out[j].HorizonBucket
	})
	return out
}

func replayResolvedDeviceIDs(run replayRun) []string {
	var metadata struct {
		ResolvedDeviceIDs []string `json:"resolved_device_ids"`
	}
	if len(run.SiteMetadataJSON) == 0 {
		return nil
	}
	if err := json.Unmarshal(run.SiteMetadataJSON, &metadata); err != nil {
		return nil
	}
	return normalizedDeviceIDs(metadata.ResolvedDeviceIDs)
}

func pctDelta(delta, actual float64) float64 {
	if actual == 0 {
		return 0
	}
	return round1((delta / actual) * 100)
}

func loadReplayVerificationRecords(ctx context.Context, db *sql.DB, siteKey string, fromDate, toDate time.Time) ([]VerificationRecord, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
	h.run_id::text,
	h.site_key,
	h.device_id::text,
	h.issued_at,
	h.target_time,
	h.target_local_date,
	h.target_local_hour,
	h.target_utc_offset_minutes,
	h.horizon_hours,
	h.horizon_bucket,
	h.forecast_generation_wh,
	h.baseline_forecast_generation_wh,
	h.actual_generation_wh,
	h.verification_status,
	h.updated_at,
	r.forecast_version,
	r.served_variant,
	r.timezone
FROM solar_forecast_hourly_training_records h
JOIN solar_forecast_runs r ON r.id = h.run_id
WHERE h.site_key = $1
  AND h.target_local_date BETWEEN $2 AND $3
ORDER BY h.target_local_date ASC, h.target_time ASC;
`, siteKey, fromDate.UTC(), toDate.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]VerificationRecord, 0)
	for rows.Next() {
		var (
			record             VerificationRecord
			deviceID           sql.NullString
			horizonBucket      string
			verificationStatus string
			baselineForecastWh sql.NullFloat64
			actualGenerationWh sql.NullFloat64
		)
		if err := rows.Scan(
			&record.RunID,
			&record.SiteKey,
			&deviceID,
			&record.IssuedAt,
			&record.TargetTime,
			&record.TargetLocalDate,
			&record.TargetLocalHour,
			&record.TargetUTCOffsetMinutes,
			&record.HorizonHours,
			&horizonBucket,
			&record.ForecastGenerationWh,
			&baselineForecastWh,
			&actualGenerationWh,
			&verificationStatus,
			&record.UpdatedAt,
			&record.ForecastVersion,
			&record.ServedVariant,
			&record.Timezone,
		); err != nil {
			return nil, err
		}
		if deviceID.Valid && strings.TrimSpace(deviceID.String) != "" {
			id := strings.TrimSpace(deviceID.String)
			record.DeviceID = &id
		}
		record.HorizonBucket = HorizonBucket(horizonBucket)
		record.VerificationStatus = VerificationStatus(verificationStatus)
		if baselineForecastWh.Valid {
			value := baselineForecastWh.Float64
			record.BaselineForecastGenerationWh = &value
		}
		if actualGenerationWh.Valid {
			value := actualGenerationWh.Float64
			record.ActualGenerationWh = &value
		}
		out = append(out, record)
	}
	return out, rows.Err()
}
