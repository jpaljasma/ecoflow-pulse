package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
	"github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("solar forecast postgres dsn is required")
	}
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, fmt.Errorf("apply solar forecast postgres search_path: %w", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open solar forecast postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping solar forecast postgres: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) InsertRun(ctx context.Context, run solarforecastd.Run) error {
	siteMetadata := normalizeJSON(run.SiteMetadataJSON)
	provenance := normalizeJSON(run.ProvenanceJSON)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO solar_forecast_runs (
	id,
	site_key,
	scope_kind,
	device_id,
	served_variant,
	canonical_location_key,
	timezone,
	issued_at,
	issue_local_date,
	issue_local_hour,
	issue_utc_offset_minutes,
	forecast_version,
	feature_version,
	weather_snapshot_id,
	capacity_estimate_w,
	actual_so_far_wh,
	forecast_remaining_today_wh,
	forecast_total_today_wh,
	site_metadata_json,
	provenance_json,
	created_at,
	updated_at
)
VALUES (
	$1::uuid, $2, $3, $4::uuid, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14::uuid, $15, $16, $17, $18, $19, $20, $21, $22
);
`,
		run.ID,
		run.SiteKey,
		run.ScopeKind,
		uuidOrNil(run.DeviceID),
		run.ServedVariant,
		run.CanonicalLocationKey,
		run.Timezone,
		run.IssuedAt.UTC(),
		run.IssueLocalDate.UTC(),
		run.IssueLocalHour,
		run.IssueUTCOffsetMinutes,
		run.ForecastVersion,
		run.FeatureVersion,
		stringOrNil(run.WeatherSnapshotID),
		run.CapacityEstimateW,
		run.ActualSoFarWh,
		run.ForecastRemainingTodayWh,
		run.ForecastTotalTodayWh,
		siteMetadata,
		provenance,
		run.CreatedAt.UTC(),
		run.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert solar forecast run: %w", err)
	}
	return nil
}

func (s *PostgresStore) InsertHourlyRecords(ctx context.Context, rows []solarforecastd.HourlyTrainingRecord) error {
	return s.upsertHourlyRows(ctx, rows, false)
}

func (s *PostgresStore) ListPendingHourlyRecords(ctx context.Context, before time.Time, limit int) ([]solarforecastd.HourlyTrainingRecord, error) {
	if limit <= 0 {
		limit = 256
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT
	run_id::text,
	site_key,
	device_id::text,
	issued_at,
	target_time,
	target_local_date,
	target_local_hour,
	target_utc_offset_minutes,
	horizon_hours,
	horizon_bucket,
	forecast_generation_wh,
	baseline_forecast_generation_wh,
	forecast_gti_wm2,
	forecast_shortwave_wm2,
	forecast_temperature_c,
	forecast_cloud_cover_pct,
	forecast_irradiance_source,
	actual_generation_wh,
	actual_gti_wm2,
	actual_shortwave_wm2,
	actual_temperature_c,
	actual_cloud_cover_pct,
	verification_status,
	signed_error_wh,
	absolute_error_wh,
	squared_error_wh2,
	baseline_absolute_error_wh,
	baseline_squared_error_wh2,
	verified_at,
	feature_snapshot_json,
	weather_raw_json,
	weather_corrected_json,
	created_at,
	updated_at
FROM solar_forecast_hourly_training_records
WHERE verification_status = 'pending'
  AND target_time < $1
ORDER BY target_time ASC
LIMIT $2;
`, before.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list pending solar forecast hourly records: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]solarforecastd.HourlyTrainingRecord, 0)
	for rows.Next() {
		record, err := scanHourlyTrainingRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending solar forecast hourly records: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) ListVerificationRecords(ctx context.Context, siteKey string, fromDate, toDate time.Time) ([]solarforecastd.VerificationRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
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
	h.forecast_gti_wm2,
	h.forecast_shortwave_wm2,
	h.forecast_temperature_c,
	h.forecast_cloud_cover_pct,
	h.forecast_irradiance_source,
	h.actual_generation_wh,
	h.actual_gti_wm2,
	h.actual_shortwave_wm2,
	h.actual_temperature_c,
	h.actual_cloud_cover_pct,
	h.verification_status,
	h.signed_error_wh,
	h.absolute_error_wh,
	h.squared_error_wh2,
	h.baseline_absolute_error_wh,
	h.baseline_squared_error_wh2,
	h.verified_at,
	h.feature_snapshot_json,
	h.weather_raw_json,
	h.weather_corrected_json,
	h.created_at,
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
		return nil, fmt.Errorf("list solar forecast verification records: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]solarforecastd.VerificationRecord, 0)
	for rows.Next() {
		record, err := scanVerificationRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate solar forecast verification records: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) ListRecentCalibrationRecords(ctx context.Context, siteKey, forecastVersion string, fromDate, toDate time.Time) ([]solarforecastd.VerificationRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT ON (h.target_time)
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
	h.actual_generation_wh,
	h.verification_status,
	h.updated_at,
	r.forecast_version,
	r.served_variant,
	r.timezone
FROM solar_forecast_hourly_training_records h
JOIN solar_forecast_runs r ON r.id = h.run_id
WHERE h.site_key = $1
  AND r.forecast_version = $2
  AND h.verification_status = 'verified'
  AND h.actual_generation_wh IS NOT NULL
  AND h.target_local_date BETWEEN $3 AND $4
ORDER BY h.target_time ASC, h.issued_at DESC;
`, siteKey, forecastVersion, fromDate.UTC(), toDate.UTC())
	if err != nil {
		return nil, fmt.Errorf("list recent solar forecast calibration records: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]solarforecastd.VerificationRecord, 0)
	for rows.Next() {
		record, err := scanRecentCalibrationRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent solar forecast calibration records: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) LoadCalibrationStates(ctx context.Context, siteKey, forecastVersion string) ([]solarforecastd.CalibrationState, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT site_key, forecast_version, horizon_bucket, hour_of_day, sample_count, multiplicative_ratio, updated_at
FROM solar_forecast_calibration_state
WHERE site_key = $1
  AND forecast_version = $2
ORDER BY horizon_bucket, hour_of_day;
`, siteKey, forecastVersion)
	if err != nil {
		return nil, fmt.Errorf("query solar forecast calibration state: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]solarforecastd.CalibrationState, 0)
	for rows.Next() {
		var (
			row     solarforecastd.CalibrationState
			horizon string
			ratio   float64
		)
		if err := rows.Scan(
			&row.SiteKey,
			&row.ForecastVersion,
			&horizon,
			&row.HourOfDay,
			&row.SampleCount,
			&ratio,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan solar forecast calibration state: %w", err)
		}
		row.HorizonBucket = solarforecastd.HorizonBucket(horizon)
		row.MultiplicativeRatio = &ratio
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate solar forecast calibration state: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) LoadServingState(ctx context.Context, siteKey, forecastVersion string) (*solarforecastd.ServingState, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
	site_key,
	forecast_version,
	timezone,
	recent_site_ratio,
	recent_site_sample_count,
	recent_site_updated_at,
	potential_base_envelope_w,
	potential_saturated_envelope_w,
	potential_final_envelope_w,
	qualified_saturated_days,
	qualified_saturated_hours,
	history_from,
	history_to,
	updated_at
FROM solar_forecast_site_serving_state
WHERE site_key = $1
  AND forecast_version = $2;
`, siteKey, forecastVersion)
	var (
		out                     solarforecastd.ServingState
		recentRatio             sql.NullFloat64
		recentUpdatedAt         sql.NullTime
		potentialBaseEnvelopeW  sql.NullFloat64
		potentialSaturatedW     sql.NullFloat64
		potentialFinalEnvelopeW sql.NullFloat64
		historyFrom             sql.NullTime
		historyTo               sql.NullTime
	)
	if err := row.Scan(
		&out.SiteKey,
		&out.ForecastVersion,
		&out.Timezone,
		&recentRatio,
		&out.RecentSiteSampleCount,
		&recentUpdatedAt,
		&potentialBaseEnvelopeW,
		&potentialSaturatedW,
		&potentialFinalEnvelopeW,
		&out.QualifiedSaturatedDays,
		&out.QualifiedSaturatedHours,
		&historyFrom,
		&historyTo,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load solar forecast serving state: %w", err)
	}
	out.RecentSiteRatio = nullableFloatPtr(recentRatio)
	if recentUpdatedAt.Valid {
		value := recentUpdatedAt.Time.UTC()
		out.RecentSiteUpdatedAt = &value
	}
	out.PotentialBaseEnvelopeW = nullableFloatPtr(potentialBaseEnvelopeW)
	out.PotentialSaturatedW = nullableFloatPtr(potentialSaturatedW)
	out.PotentialFinalEnvelopeW = nullableFloatPtr(potentialFinalEnvelopeW)
	if historyFrom.Valid {
		value := historyFrom.Time.UTC()
		out.HistoryFrom = &value
	}
	if historyTo.Valid {
		value := historyTo.Time.UTC()
		out.HistoryTo = &value
	}
	return &out, nil
}

func (s *PostgresStore) UpsertCalibrationStates(ctx context.Context, states []solarforecastd.CalibrationState) error {
	if len(states) == 0 {
		return nil
	}
	for _, state := range states {
		_, err := s.db.ExecContext(ctx, `
INSERT INTO solar_forecast_calibration_state (
	site_key,
	forecast_version,
	horizon_bucket,
	hour_of_day,
	sample_count,
	multiplicative_ratio,
	updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (site_key, forecast_version, horizon_bucket, hour_of_day)
DO UPDATE SET
	sample_count = EXCLUDED.sample_count,
	multiplicative_ratio = EXCLUDED.multiplicative_ratio,
	updated_at = EXCLUDED.updated_at;
`, state.SiteKey, state.ForecastVersion, string(state.HorizonBucket), state.HourOfDay, state.SampleCount, state.MultiplicativeRatio, state.UpdatedAt.UTC())
		if err != nil {
			return fmt.Errorf("upsert solar forecast calibration state: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) UpsertServingState(ctx context.Context, state solarforecastd.ServingState) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO solar_forecast_site_serving_state (
	site_key,
	forecast_version,
	timezone,
	recent_site_ratio,
	recent_site_sample_count,
	recent_site_updated_at,
	potential_base_envelope_w,
	potential_saturated_envelope_w,
	potential_final_envelope_w,
	qualified_saturated_days,
	qualified_saturated_hours,
	history_from,
	history_to,
	updated_at
)
VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
ON CONFLICT (site_key, forecast_version) DO UPDATE
SET timezone = EXCLUDED.timezone,
	recent_site_ratio = EXCLUDED.recent_site_ratio,
	recent_site_sample_count = EXCLUDED.recent_site_sample_count,
	recent_site_updated_at = EXCLUDED.recent_site_updated_at,
	potential_base_envelope_w = EXCLUDED.potential_base_envelope_w,
	potential_saturated_envelope_w = EXCLUDED.potential_saturated_envelope_w,
	potential_final_envelope_w = EXCLUDED.potential_final_envelope_w,
	qualified_saturated_days = EXCLUDED.qualified_saturated_days,
	qualified_saturated_hours = EXCLUDED.qualified_saturated_hours,
	history_from = EXCLUDED.history_from,
	history_to = EXCLUDED.history_to,
	updated_at = EXCLUDED.updated_at;
`,
		state.SiteKey,
		state.ForecastVersion,
		state.Timezone,
		floatOrNil(state.RecentSiteRatio),
		state.RecentSiteSampleCount,
		timeOrNil(state.RecentSiteUpdatedAt),
		floatOrNil(state.PotentialBaseEnvelopeW),
		floatOrNil(state.PotentialSaturatedW),
		floatOrNil(state.PotentialFinalEnvelopeW),
		state.QualifiedSaturatedDays,
		state.QualifiedSaturatedHours,
		timeOrNil(state.HistoryFrom),
		timeOrNil(state.HistoryTo),
		state.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert solar forecast serving state: %w", err)
	}
	return nil
}

func (s *PostgresStore) CompleteHourlyVerification(ctx context.Context, rows []solarforecastd.HourlyTrainingRecord) error {
	return s.upsertHourlyRows(ctx, rows, true)
}

func (s *PostgresStore) UpsertDailyVerificationRollup(ctx context.Context, row solarforecastd.DailyVerificationRollup) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO solar_forecast_verification_daily (
	site_key,
	device_id,
	served_variant,
	verification_local_date,
	timezone,
	forecast_version,
	horizon_bucket,
	forecast_hours,
	verified_hours,
	missing_truth_hours,
	missing_weather_hours,
	hourly_abs_error_wh_sum,
	hourly_sq_error_wh2_sum,
	daily_abs_error_wh_sum,
	baseline_daily_abs_error_wh_sum,
	peak_power_abs_error_w_sum,
	baseline_peak_power_abs_error_w_sum,
	peak_time_abs_error_minutes_sum,
	baseline_peak_time_abs_error_minutes_sum,
	created_at,
	updated_at
)
VALUES (
	$1, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
)
ON CONFLICT (site_key, verification_local_date, forecast_version, served_variant, horizon_bucket)
DO UPDATE SET
	device_id = EXCLUDED.device_id,
	served_variant = EXCLUDED.served_variant,
	timezone = EXCLUDED.timezone,
	forecast_hours = EXCLUDED.forecast_hours,
	verified_hours = EXCLUDED.verified_hours,
	missing_truth_hours = EXCLUDED.missing_truth_hours,
	missing_weather_hours = EXCLUDED.missing_weather_hours,
	hourly_abs_error_wh_sum = EXCLUDED.hourly_abs_error_wh_sum,
	hourly_sq_error_wh2_sum = EXCLUDED.hourly_sq_error_wh2_sum,
	daily_abs_error_wh_sum = EXCLUDED.daily_abs_error_wh_sum,
	baseline_daily_abs_error_wh_sum = EXCLUDED.baseline_daily_abs_error_wh_sum,
	peak_power_abs_error_w_sum = EXCLUDED.peak_power_abs_error_w_sum,
	baseline_peak_power_abs_error_w_sum = EXCLUDED.baseline_peak_power_abs_error_w_sum,
	peak_time_abs_error_minutes_sum = EXCLUDED.peak_time_abs_error_minutes_sum,
	baseline_peak_time_abs_error_minutes_sum = EXCLUDED.baseline_peak_time_abs_error_minutes_sum,
	updated_at = EXCLUDED.updated_at;
`,
		row.SiteKey,
		uuidOrNil(row.DeviceID),
		row.ServedVariant,
		row.VerificationLocalDate.UTC(),
		row.Timezone,
		row.ForecastVersion,
		string(row.HorizonBucket),
		row.ForecastHours,
		row.VerifiedHours,
		row.MissingTruthHours,
		row.MissingWeatherHours,
		row.HourlyAbsErrorWhSum,
		row.HourlySqErrorWh2Sum,
		row.DailyAbsErrorWhSum,
		row.BaselineDailyAbsErrorWhSum,
		row.PeakPowerAbsErrorWSum,
		row.BaselinePeakPowerAbsErrorWSum,
		row.PeakTimeAbsErrorMinutesSum,
		row.BaselinePeakTimeAbsErrorMinutesSum,
		row.CreatedAt.UTC(),
		row.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert solar forecast verification daily: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetRun(ctx context.Context, id string) (*solarforecastd.Run, error) {
	var out solarforecastd.Run
	var deviceID sql.NullString
	var capacity sql.NullFloat64
	var weatherSnapshotID sql.NullString
	var siteMetadata []byte
	var provenance []byte
	err := s.db.QueryRowContext(ctx, `
SELECT
	id::text,
	site_key,
	scope_kind,
	device_id::text,
	served_variant,
	canonical_location_key,
	timezone,
	issued_at,
	issue_local_date,
	issue_local_hour,
	issue_utc_offset_minutes,
	forecast_version,
	feature_version,
	weather_snapshot_id::text,
	capacity_estimate_w,
	actual_so_far_wh,
	forecast_remaining_today_wh,
	forecast_total_today_wh,
	site_metadata_json,
	provenance_json,
	created_at,
	updated_at
FROM solar_forecast_runs
WHERE id = $1::uuid
LIMIT 1;
`, id).Scan(
		&out.ID,
		&out.SiteKey,
		&out.ScopeKind,
		&deviceID,
		&out.ServedVariant,
		&out.CanonicalLocationKey,
		&out.Timezone,
		&out.IssuedAt,
		&out.IssueLocalDate,
		&out.IssueLocalHour,
		&out.IssueUTCOffsetMinutes,
		&out.ForecastVersion,
		&out.FeatureVersion,
		&weatherSnapshotID,
		&capacity,
		&out.ActualSoFarWh,
		&out.ForecastRemainingTodayWh,
		&out.ForecastTotalTodayWh,
		&siteMetadata,
		&provenance,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query solar forecast run: %w", err)
	}
	if deviceID.Valid {
		out.DeviceID = &deviceID.String
	}
	if weatherSnapshotID.Valid {
		out.WeatherSnapshotID = weatherSnapshotID.String
	}
	if capacity.Valid {
		out.CapacityEstimateW = &capacity.Float64
	}
	out.SiteMetadataJSON = append([]byte(nil), siteMetadata...)
	out.ProvenanceJSON = append([]byte(nil), provenance...)
	return &out, nil
}

func (s *PostgresStore) upsertHourlyRows(ctx context.Context, rows []solarforecastd.HourlyTrainingRecord, updateVerification bool) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin solar forecast hourly tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, row := range rows {
		weatherRaw, err := json.Marshal(row.WeatherRaw)
		if err != nil {
			return fmt.Errorf("marshal solar forecast weather raw: %w", err)
		}
		weatherCorrected, err := json.Marshal(row.WeatherCorrected)
		if err != nil {
			return fmt.Errorf("marshal solar forecast weather corrected: %w", err)
		}
		featureSnapshot := normalizeJSON(row.FeatureSnapshotJSON)
		query := `
INSERT INTO solar_forecast_hourly_training_records (
	run_id,
	site_key,
	device_id,
	issued_at,
	target_time,
	target_local_date,
	target_local_hour,
	target_utc_offset_minutes,
	horizon_hours,
	horizon_bucket,
	forecast_generation_wh,
	baseline_forecast_generation_wh,
	forecast_gti_wm2,
	forecast_shortwave_wm2,
	forecast_temperature_c,
	forecast_cloud_cover_pct,
	forecast_irradiance_source,
	actual_generation_wh,
	actual_gti_wm2,
	actual_shortwave_wm2,
	actual_temperature_c,
	actual_cloud_cover_pct,
	verification_status,
	signed_error_wh,
	absolute_error_wh,
	squared_error_wh2,
	baseline_absolute_error_wh,
	baseline_squared_error_wh2,
	verified_at,
	feature_snapshot_json,
	weather_raw_json,
	weather_corrected_json,
	created_at,
	updated_at
)
VALUES (
	$1::uuid, $2, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34
)
ON CONFLICT (run_id, target_time) DO UPDATE SET
	verification_status = EXCLUDED.verification_status,
	actual_generation_wh = EXCLUDED.actual_generation_wh,
	actual_gti_wm2 = EXCLUDED.actual_gti_wm2,
	actual_shortwave_wm2 = EXCLUDED.actual_shortwave_wm2,
	actual_temperature_c = EXCLUDED.actual_temperature_c,
	actual_cloud_cover_pct = EXCLUDED.actual_cloud_cover_pct,
	signed_error_wh = EXCLUDED.signed_error_wh,
	absolute_error_wh = EXCLUDED.absolute_error_wh,
	squared_error_wh2 = EXCLUDED.squared_error_wh2,
	baseline_absolute_error_wh = EXCLUDED.baseline_absolute_error_wh,
	baseline_squared_error_wh2 = EXCLUDED.baseline_squared_error_wh2,
	verified_at = EXCLUDED.verified_at,
	feature_snapshot_json = EXCLUDED.feature_snapshot_json,
	updated_at = EXCLUDED.updated_at`
		if !updateVerification {
			query = `
INSERT INTO solar_forecast_hourly_training_records (
	run_id,
	site_key,
	device_id,
	issued_at,
	target_time,
	target_local_date,
	target_local_hour,
	target_utc_offset_minutes,
	horizon_hours,
	horizon_bucket,
	forecast_generation_wh,
	baseline_forecast_generation_wh,
	forecast_gti_wm2,
	forecast_shortwave_wm2,
	forecast_temperature_c,
	forecast_cloud_cover_pct,
	forecast_irradiance_source,
	actual_generation_wh,
	actual_gti_wm2,
	actual_shortwave_wm2,
	actual_temperature_c,
	actual_cloud_cover_pct,
	verification_status,
	signed_error_wh,
	absolute_error_wh,
	squared_error_wh2,
	baseline_absolute_error_wh,
	baseline_squared_error_wh2,
	verified_at,
	feature_snapshot_json,
	weather_raw_json,
	weather_corrected_json,
	created_at,
	updated_at
)
VALUES (
	$1::uuid, $2, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34
)
ON CONFLICT (run_id, target_time) DO NOTHING`
		}
		if _, err := tx.ExecContext(ctx, query,
			row.RunID,
			row.SiteKey,
			uuidOrNil(row.DeviceID),
			row.IssuedAt.UTC(),
			row.TargetTime.UTC(),
			row.TargetLocalDate.UTC(),
			row.TargetLocalHour,
			row.TargetUTCOffsetMinutes,
			row.HorizonHours,
			string(row.HorizonBucket),
			row.ForecastGenerationWh,
			row.BaselineForecastGenerationWh,
			row.ForecastGTIWm2,
			row.ForecastShortwaveWm2,
			row.ForecastTemperatureC,
			row.ForecastCloudCoverPct,
			string(row.ForecastIrradianceSource),
			row.ActualGenerationWh,
			row.ActualGTIWm2,
			row.ActualShortwaveWm2,
			row.ActualTemperatureC,
			row.ActualCloudCoverPct,
			string(row.VerificationStatus),
			row.SignedErrorWh,
			row.AbsoluteErrorWh,
			row.SquaredErrorWh2,
			row.BaselineAbsoluteErrorWh,
			row.BaselineSquaredErrorWh2,
			nullTime(row.VerifiedAt),
			featureSnapshot,
			weatherRaw,
			weatherCorrected,
			row.CreatedAt.UTC(),
			row.UpdatedAt.UTC(),
		); err != nil {
			return fmt.Errorf("upsert solar forecast hourly record: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit solar forecast hourly tx: %w", err)
	}
	return nil
}

func normalizeJSON(raw []byte) []byte {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return []byte(`{}`)
	}
	return raw
}

func uuidOrNil(value *string) any {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}

func stringOrNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHourlyTrainingRecord(scanner rowScanner) (solarforecastd.HourlyTrainingRecord, error) {
	var out solarforecastd.HourlyTrainingRecord
	var (
		deviceID                 sql.NullString
		baselineForecast         sql.NullFloat64
		forecastGTI              sql.NullFloat64
		forecastShortwave        sql.NullFloat64
		forecastTemperature      sql.NullFloat64
		forecastCloudCover       sql.NullFloat64
		actualGeneration         sql.NullFloat64
		actualGTI                sql.NullFloat64
		actualShortwave          sql.NullFloat64
		actualTemperature        sql.NullFloat64
		actualCloudCover         sql.NullFloat64
		signedError              sql.NullFloat64
		absoluteError            sql.NullFloat64
		squaredError             sql.NullFloat64
		baselineAbsoluteError    sql.NullFloat64
		baselineSquaredError     sql.NullFloat64
		verifiedAt               sql.NullTime
		featureSnapshotJSON      []byte
		weatherRawJSON           []byte
		weatherCorrectedJSON     []byte
		forecastIrradianceSource string
		verificationStatus       string
		horizonBucket            string
	)
	err := scanner.Scan(
		&out.RunID,
		&out.SiteKey,
		&deviceID,
		&out.IssuedAt,
		&out.TargetTime,
		&out.TargetLocalDate,
		&out.TargetLocalHour,
		&out.TargetUTCOffsetMinutes,
		&out.HorizonHours,
		&horizonBucket,
		&out.ForecastGenerationWh,
		&baselineForecast,
		&forecastGTI,
		&forecastShortwave,
		&forecastTemperature,
		&forecastCloudCover,
		&forecastIrradianceSource,
		&actualGeneration,
		&actualGTI,
		&actualShortwave,
		&actualTemperature,
		&actualCloudCover,
		&verificationStatus,
		&signedError,
		&absoluteError,
		&squaredError,
		&baselineAbsoluteError,
		&baselineSquaredError,
		&verifiedAt,
		&featureSnapshotJSON,
		&weatherRawJSON,
		&weatherCorrectedJSON,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return solarforecastd.HourlyTrainingRecord{}, fmt.Errorf("scan solar forecast hourly record: %w", err)
	}
	if deviceID.Valid {
		out.DeviceID = &deviceID.String
	}
	out.HorizonBucket = solarforecastd.HorizonBucket(horizonBucket)
	out.ForecastIrradianceSource = solarforecastd.IrradianceSource(forecastIrradianceSource)
	out.VerificationStatus = solarforecastd.VerificationStatus(verificationStatus)
	out.BaselineForecastGenerationWh = nullableFloatPtr(baselineForecast)
	out.ForecastGTIWm2 = nullableFloatPtr(forecastGTI)
	out.ForecastShortwaveWm2 = nullableFloatPtr(forecastShortwave)
	out.ForecastTemperatureC = nullableFloatPtr(forecastTemperature)
	out.ForecastCloudCoverPct = nullableFloatPtr(forecastCloudCover)
	out.ActualGenerationWh = nullableFloatPtr(actualGeneration)
	out.ActualGTIWm2 = nullableFloatPtr(actualGTI)
	out.ActualShortwaveWm2 = nullableFloatPtr(actualShortwave)
	out.ActualTemperatureC = nullableFloatPtr(actualTemperature)
	out.ActualCloudCoverPct = nullableFloatPtr(actualCloudCover)
	out.SignedErrorWh = nullableFloatPtr(signedError)
	out.AbsoluteErrorWh = nullableFloatPtr(absoluteError)
	out.SquaredErrorWh2 = nullableFloatPtr(squaredError)
	out.BaselineAbsoluteErrorWh = nullableFloatPtr(baselineAbsoluteError)
	out.BaselineSquaredErrorWh2 = nullableFloatPtr(baselineSquaredError)
	if verifiedAt.Valid {
		value := verifiedAt.Time.UTC()
		out.VerifiedAt = &value
	}
	out.FeatureSnapshotJSON = append([]byte(nil), featureSnapshotJSON...)
	if err := json.Unmarshal(normalizeJSON(weatherRawJSON), &out.WeatherRaw); err != nil {
		return solarforecastd.HourlyTrainingRecord{}, fmt.Errorf("unmarshal solar forecast weather raw: %w", err)
	}
	if err := json.Unmarshal(normalizeJSON(weatherCorrectedJSON), &out.WeatherCorrected); err != nil {
		return solarforecastd.HourlyTrainingRecord{}, fmt.Errorf("unmarshal solar forecast weather corrected: %w", err)
	}
	return out, nil
}

func scanVerificationRecord(scanner rowScanner) (solarforecastd.VerificationRecord, error) {
	record, err := scanHourlyTrainingRecordWithExtras(scanner)
	if err != nil {
		return solarforecastd.VerificationRecord{}, err
	}
	return record, nil
}

func scanRecentCalibrationRecord(scanner rowScanner) (solarforecastd.VerificationRecord, error) {
	var (
		out                solarforecastd.VerificationRecord
		deviceID           sql.NullString
		actualGeneration   sql.NullFloat64
		updatedAt          time.Time
		horizonBucket      string
		verificationStatus string
	)
	if err := scanner.Scan(
		&out.RunID,
		&out.SiteKey,
		&deviceID,
		&out.IssuedAt,
		&out.TargetTime,
		&out.TargetLocalDate,
		&out.TargetLocalHour,
		&out.TargetUTCOffsetMinutes,
		&out.HorizonHours,
		&horizonBucket,
		&out.ForecastGenerationWh,
		&actualGeneration,
		&verificationStatus,
		&updatedAt,
		&out.ForecastVersion,
		&out.ServedVariant,
		&out.Timezone,
	); err != nil {
		return solarforecastd.VerificationRecord{}, fmt.Errorf("scan recent solar forecast calibration record: %w", err)
	}
	if deviceID.Valid {
		out.DeviceID = &deviceID.String
	}
	out.HorizonBucket = solarforecastd.HorizonBucket(horizonBucket)
	out.VerificationStatus = solarforecastd.VerificationStatus(verificationStatus)
	out.ActualGenerationWh = nullableFloatPtr(actualGeneration)
	out.UpdatedAt = updatedAt.UTC()
	return out, nil
}

func scanHourlyTrainingRecordWithExtras(scanner rowScanner) (solarforecastd.VerificationRecord, error) {
	var out solarforecastd.VerificationRecord
	record, err := scanHourlyTrainingRecord(scanAdapter{
		scanner: scanner,
		extra:   []any{&out.ForecastVersion, &out.ServedVariant, &out.Timezone},
	})
	if err != nil {
		return solarforecastd.VerificationRecord{}, err
	}
	out.HourlyTrainingRecord = record
	return out, nil
}

type scanAdapter struct {
	scanner rowScanner
	extra   []any
}

func (s scanAdapter) Scan(dest ...any) error {
	dest = append(dest, s.extra...)
	return s.scanner.Scan(dest...)
}

func nullableFloatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func floatOrNil(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func timeOrNil(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
