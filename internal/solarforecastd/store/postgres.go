package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
	"github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd"
)

type PostgresStore struct {
	db   *sql.DB
	exec sqlExecutor
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

const pendingRunClaimCandidateLimit = 64

type runDailyVerificationRollup struct {
	RunID                    string
	SiteKey                  string
	DeviceID                 *string
	VerificationLocalDate    time.Time
	Timezone                 string
	ForecastVersion          string
	ServedVariant            string
	HorizonBucket            solarforecastd.HorizonBucket
	ForecastHours            int
	VerifiedHours            int
	MissingTruthHours        int
	MissingWeatherHours      int
	HourlyAbsErrorWhSum      float64
	HourlySqErrorWh2Sum      float64
	ForecastTotalWh          float64
	BaselineForecastTotalWh  float64
	ActualTotalWh            float64
	ForecastPeakWh           float64
	ForecastPeakTime         *time.Time
	BaselineForecastPeakWh   float64
	BaselineForecastPeakTime *time.Time
	ActualPeakWh             float64
	ActualPeakTime           *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
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
	return &PostgresStore{db: db, exec: db}, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) executor() sqlExecutor {
	if s != nil && s.exec != nil {
		return s.exec
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *PostgresStore) InsertRun(ctx context.Context, run solarforecastd.Run) (string, error) {
	siteMetadata := normalizeJSON(run.SiteMetadataJSON)
	provenance := normalizeJSON(run.ProvenanceJSON)
	var id string
	err := s.executor().QueryRowContext(ctx, `
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
)
ON CONFLICT (site_key, issued_at, forecast_version)
DO UPDATE SET
	scope_kind = EXCLUDED.scope_kind,
	device_id = EXCLUDED.device_id,
	served_variant = EXCLUDED.served_variant,
	canonical_location_key = EXCLUDED.canonical_location_key,
	timezone = EXCLUDED.timezone,
	issue_local_date = EXCLUDED.issue_local_date,
	issue_local_hour = EXCLUDED.issue_local_hour,
	issue_utc_offset_minutes = EXCLUDED.issue_utc_offset_minutes,
	feature_version = EXCLUDED.feature_version,
	weather_snapshot_id = EXCLUDED.weather_snapshot_id,
	capacity_estimate_w = EXCLUDED.capacity_estimate_w,
	actual_so_far_wh = EXCLUDED.actual_so_far_wh,
	forecast_remaining_today_wh = EXCLUDED.forecast_remaining_today_wh,
	forecast_total_today_wh = EXCLUDED.forecast_total_today_wh,
	site_metadata_json = EXCLUDED.site_metadata_json,
	provenance_json = EXCLUDED.provenance_json,
	updated_at = EXCLUDED.updated_at
RETURNING id::text;
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
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert solar forecast run: %w", err)
	}
	return id, nil
}

func (s *PostgresStore) InsertHourlyRecords(ctx context.Context, rows []solarforecastd.HourlyTrainingRecord) error {
	return s.upsertHourlyRows(ctx, rows, false)
}

func (s *PostgresStore) ListPendingHourlyRecords(ctx context.Context, before time.Time, limit int) ([]solarforecastd.HourlyTrainingRecord, error) {
	if limit <= 0 {
		limit = 256
	}
	rows, err := s.executor().QueryContext(ctx, `
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
	rows, err := s.executor().QueryContext(ctx, `
SELECT
	h.run_id::text,
	h.device_id::text,
	h.target_time,
	h.target_local_date,
	h.horizon_bucket,
	h.forecast_generation_wh,
	h.baseline_forecast_generation_wh,
	h.actual_generation_wh,
	h.verification_status,
	h.absolute_error_wh,
	h.squared_error_wh2,
	h.baseline_absolute_error_wh,
	h.baseline_squared_error_wh2,
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
	rows, err := s.executor().QueryContext(ctx, `
SELECT
	h.run_id::text,
	h.issued_at,
	h.target_time,
	h.target_local_date,
	h.forecast_generation_wh,
	h.actual_generation_wh,
	h.verification_status,
	h.updated_at,
	r.forecast_version,
	r.timezone
FROM solar_forecast_hourly_training_records h
JOIN solar_forecast_runs r ON r.id = h.run_id
WHERE h.site_key = $1
  AND r.forecast_version = $2
  AND h.verification_status = 'verified'
  AND h.actual_generation_wh IS NOT NULL
  AND h.target_local_date BETWEEN $3 AND $4
ORDER BY h.target_local_date ASC, h.target_time ASC;
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
	rows, err := s.executor().QueryContext(ctx, `
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

func (s *PostgresStore) UpsertCalibrationStates(ctx context.Context, states []solarforecastd.CalibrationState) error {
	if len(states) == 0 {
		return nil
	}
	for _, state := range states {
		_, err := s.executor().ExecContext(ctx, `
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

func (s *PostgresStore) CompleteHourlyVerification(ctx context.Context, rows []solarforecastd.HourlyTrainingRecord) error {
	return s.upsertHourlyRows(ctx, rows, true)
}

func (s *PostgresStore) UpsertDailyVerificationRollup(ctx context.Context, row solarforecastd.DailyVerificationRollup) error {
	_, err := s.executor().ExecContext(ctx, `
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
	err := s.executor().QueryRowContext(ctx, `
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
	tx, managedTx, err := s.ensureTx(ctx)
	if err != nil {
		return fmt.Errorf("begin solar forecast hourly tx: %w", err)
	}
	if managedTx {
		defer func() { _ = tx.Rollback() }()
	}
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
	if !managedTx {
		return nil
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit solar forecast hourly tx: %w", err)
	}
	return nil
}

func (s *PostgresStore) ClaimPendingRun(ctx context.Context, before time.Time) (*solarforecastd.PendingRunClaim, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin solar forecast claim tx: %w", err)
	}
	txStore := &PostgresStore{db: s.db, exec: tx}
	runID, err := txStore.claimPendingRunID(ctx, before.UTC())
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if runID == "" {
		_ = tx.Rollback()
		return nil, nil
	}
	rows, err := txStore.listPendingRowsForRun(ctx, runID, before.UTC())
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	run, err := txStore.GetRun(ctx, runID)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return solarforecastd.NewPendingRunClaim(run, rows, txStore, func(commit bool) error {
		if !commit {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				return fmt.Errorf("rollback solar forecast claim tx: %w", err)
			}
			return nil
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit solar forecast claim tx: %w", err)
		}
		return nil
	}), nil
}

func (s *PostgresStore) claimPendingRunID(ctx context.Context, before time.Time) (string, error) {
	var runID string
	err := s.executor().QueryRowContext(ctx, `
WITH candidate_runs AS (
	SELECT
		run_id,
		MIN(target_time) AS first_target_time
	FROM solar_forecast_hourly_training_records
	WHERE verification_status = 'pending'
	  AND target_time < $1
	GROUP BY run_id
	ORDER BY first_target_time ASC
	LIMIT $2
),
claimed_run AS (
	SELECT r.id::text AS run_id
	FROM candidate_runs
	JOIN solar_forecast_runs r ON r.id = candidate_runs.run_id
	ORDER BY candidate_runs.first_target_time ASC
	FOR UPDATE OF r SKIP LOCKED
	LIMIT 1
)
SELECT run_id
FROM claimed_run;
`, before.UTC(), pendingRunClaimCandidateLimit).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("claim solar forecast pending run: %w", err)
	}
	return runID, nil
}

func (s *PostgresStore) ApplyCalibrationRows(ctx context.Context, siteKey, forecastVersion string, verifiedAt time.Time, rows []solarforecastd.HourlyTrainingRecord) error {
	if len(rows) == 0 {
		return nil
	}
	if err := s.acquireAdvisoryXactLock(ctx, calibrationLockKey(siteKey, forecastVersion)); err != nil {
		return err
	}
	sortedRows := append([]solarforecastd.HourlyTrainingRecord(nil), rows...)
	sort.Slice(sortedRows, func(i, j int) bool {
		if sortedRows[i].TargetTime.Equal(sortedRows[j].TargetTime) {
			if sortedRows[i].HorizonBucket == sortedRows[j].HorizonBucket {
				return sortedRows[i].TargetLocalHour < sortedRows[j].TargetLocalHour
			}
			return sortedRows[i].HorizonBucket < sortedRows[j].HorizonBucket
		}
		return sortedRows[i].TargetTime.Before(sortedRows[j].TargetTime)
	})
	for _, row := range sortedRows {
		if row.VerificationStatus != solarforecastd.VerificationStatusVerified || row.ActualGenerationWh == nil {
			continue
		}
		if row.ForecastGenerationWh < solarforecastd.CalibrationMinForecastWh {
			continue
		}
		ratio := *row.ActualGenerationWh / row.ForecastGenerationWh
		ratio = math.Min(solarforecastd.CalibrationRatioMaxClamp, math.Max(solarforecastd.CalibrationRatioMinClamp, ratio))
		if _, err := s.executor().ExecContext(ctx, `
INSERT INTO solar_forecast_calibration_state (
	site_key,
	forecast_version,
	horizon_bucket,
	hour_of_day,
	sample_count,
	multiplicative_ratio,
	updated_at
)
VALUES ($1, $2, $3, $4, 1, $5, $6)
ON CONFLICT (site_key, forecast_version, horizon_bucket, hour_of_day)
DO UPDATE SET
	sample_count = solar_forecast_calibration_state.sample_count + 1,
	multiplicative_ratio = GREATEST(
		$7,
		LEAST(
			$8,
			($9 * EXCLUDED.multiplicative_ratio) +
			((1 - $9) * solar_forecast_calibration_state.multiplicative_ratio)
		)
	),
	updated_at = EXCLUDED.updated_at;
`, siteKey, forecastVersion, string(row.HorizonBucket), row.TargetLocalHour, ratio, verifiedAt.UTC(), solarforecastd.CalibrationRatioMinClamp, solarforecastd.CalibrationRatioMaxClamp, solarforecastd.CalibrationEWMAAlpha); err != nil {
			return fmt.Errorf("apply solar forecast calibration row: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) UpsertRunDailyVerificationRollups(ctx context.Context, run *solarforecastd.Run, rows []solarforecastd.HourlyTrainingRecord, verifiedAt time.Time) error {
	if run == nil || len(rows) == 0 {
		return nil
	}
	summaries := summarizeRunDailyVerificationRollups(run, rows, verifiedAt)
	for _, summary := range summaries {
		if _, err := s.executor().ExecContext(ctx, `
INSERT INTO solar_forecast_verification_daily_run_rollup (
	run_id,
	site_key,
	device_id,
	verification_local_date,
	timezone,
	forecast_version,
	served_variant,
	horizon_bucket,
	forecast_hours,
	verified_hours,
	missing_truth_hours,
	missing_weather_hours,
	hourly_abs_error_wh_sum,
	hourly_sq_error_wh2_sum,
	forecast_total_wh,
	baseline_forecast_total_wh,
	actual_total_wh,
	forecast_peak_wh,
	forecast_peak_time,
	baseline_forecast_peak_wh,
	baseline_forecast_peak_time,
	actual_peak_wh,
	actual_peak_time,
	created_at,
	updated_at
)
VALUES (
	$1::uuid, $2, $3::uuid, $4::date, $5, $6, $7, $8,
	$9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25
)
ON CONFLICT (run_id, verification_local_date, horizon_bucket)
DO UPDATE SET
	site_key = EXCLUDED.site_key,
	device_id = EXCLUDED.device_id,
	timezone = EXCLUDED.timezone,
	forecast_version = EXCLUDED.forecast_version,
	served_variant = EXCLUDED.served_variant,
	forecast_hours = solar_forecast_verification_daily_run_rollup.forecast_hours + EXCLUDED.forecast_hours,
	verified_hours = solar_forecast_verification_daily_run_rollup.verified_hours + EXCLUDED.verified_hours,
	missing_truth_hours = solar_forecast_verification_daily_run_rollup.missing_truth_hours + EXCLUDED.missing_truth_hours,
	missing_weather_hours = solar_forecast_verification_daily_run_rollup.missing_weather_hours + EXCLUDED.missing_weather_hours,
	hourly_abs_error_wh_sum = solar_forecast_verification_daily_run_rollup.hourly_abs_error_wh_sum + EXCLUDED.hourly_abs_error_wh_sum,
	hourly_sq_error_wh2_sum = solar_forecast_verification_daily_run_rollup.hourly_sq_error_wh2_sum + EXCLUDED.hourly_sq_error_wh2_sum,
	forecast_total_wh = solar_forecast_verification_daily_run_rollup.forecast_total_wh + EXCLUDED.forecast_total_wh,
	baseline_forecast_total_wh = solar_forecast_verification_daily_run_rollup.baseline_forecast_total_wh + EXCLUDED.baseline_forecast_total_wh,
	actual_total_wh = solar_forecast_verification_daily_run_rollup.actual_total_wh + EXCLUDED.actual_total_wh,
	forecast_peak_wh = CASE
		WHEN EXCLUDED.forecast_peak_time IS NULL THEN solar_forecast_verification_daily_run_rollup.forecast_peak_wh
		WHEN EXCLUDED.forecast_peak_wh > solar_forecast_verification_daily_run_rollup.forecast_peak_wh THEN EXCLUDED.forecast_peak_wh
		WHEN EXCLUDED.forecast_peak_wh = solar_forecast_verification_daily_run_rollup.forecast_peak_wh
		 AND (solar_forecast_verification_daily_run_rollup.forecast_peak_time IS NULL OR EXCLUDED.forecast_peak_time < solar_forecast_verification_daily_run_rollup.forecast_peak_time)
		THEN EXCLUDED.forecast_peak_wh
		ELSE solar_forecast_verification_daily_run_rollup.forecast_peak_wh
	END,
	forecast_peak_time = CASE
		WHEN EXCLUDED.forecast_peak_time IS NULL THEN solar_forecast_verification_daily_run_rollup.forecast_peak_time
		WHEN EXCLUDED.forecast_peak_wh > solar_forecast_verification_daily_run_rollup.forecast_peak_wh THEN EXCLUDED.forecast_peak_time
		WHEN EXCLUDED.forecast_peak_wh = solar_forecast_verification_daily_run_rollup.forecast_peak_wh
		 AND (solar_forecast_verification_daily_run_rollup.forecast_peak_time IS NULL OR EXCLUDED.forecast_peak_time < solar_forecast_verification_daily_run_rollup.forecast_peak_time)
		THEN EXCLUDED.forecast_peak_time
		ELSE solar_forecast_verification_daily_run_rollup.forecast_peak_time
	END,
	baseline_forecast_peak_wh = CASE
		WHEN EXCLUDED.baseline_forecast_peak_time IS NULL THEN solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_wh
		WHEN EXCLUDED.baseline_forecast_peak_wh > solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_wh THEN EXCLUDED.baseline_forecast_peak_wh
		WHEN EXCLUDED.baseline_forecast_peak_wh = solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_wh
		 AND (solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_time IS NULL OR EXCLUDED.baseline_forecast_peak_time < solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_time)
		THEN EXCLUDED.baseline_forecast_peak_wh
		ELSE solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_wh
	END,
	baseline_forecast_peak_time = CASE
		WHEN EXCLUDED.baseline_forecast_peak_time IS NULL THEN solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_time
		WHEN EXCLUDED.baseline_forecast_peak_wh > solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_wh THEN EXCLUDED.baseline_forecast_peak_time
		WHEN EXCLUDED.baseline_forecast_peak_wh = solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_wh
		 AND (solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_time IS NULL OR EXCLUDED.baseline_forecast_peak_time < solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_time)
		THEN EXCLUDED.baseline_forecast_peak_time
		ELSE solar_forecast_verification_daily_run_rollup.baseline_forecast_peak_time
	END,
	actual_peak_wh = CASE
		WHEN EXCLUDED.actual_peak_time IS NULL THEN solar_forecast_verification_daily_run_rollup.actual_peak_wh
		WHEN EXCLUDED.actual_peak_wh > solar_forecast_verification_daily_run_rollup.actual_peak_wh THEN EXCLUDED.actual_peak_wh
		WHEN EXCLUDED.actual_peak_wh = solar_forecast_verification_daily_run_rollup.actual_peak_wh
		 AND (solar_forecast_verification_daily_run_rollup.actual_peak_time IS NULL OR EXCLUDED.actual_peak_time < solar_forecast_verification_daily_run_rollup.actual_peak_time)
		THEN EXCLUDED.actual_peak_wh
		ELSE solar_forecast_verification_daily_run_rollup.actual_peak_wh
	END,
	actual_peak_time = CASE
		WHEN EXCLUDED.actual_peak_time IS NULL THEN solar_forecast_verification_daily_run_rollup.actual_peak_time
		WHEN EXCLUDED.actual_peak_wh > solar_forecast_verification_daily_run_rollup.actual_peak_wh THEN EXCLUDED.actual_peak_time
		WHEN EXCLUDED.actual_peak_wh = solar_forecast_verification_daily_run_rollup.actual_peak_wh
		 AND (solar_forecast_verification_daily_run_rollup.actual_peak_time IS NULL OR EXCLUDED.actual_peak_time < solar_forecast_verification_daily_run_rollup.actual_peak_time)
		THEN EXCLUDED.actual_peak_time
		ELSE solar_forecast_verification_daily_run_rollup.actual_peak_time
	END,
	updated_at = EXCLUDED.updated_at;
`, summary.RunID, summary.SiteKey, uuidOrNil(summary.DeviceID), summary.VerificationLocalDate.UTC(), summary.Timezone, summary.ForecastVersion, summary.ServedVariant, string(summary.HorizonBucket), summary.ForecastHours, summary.VerifiedHours, summary.MissingTruthHours, summary.MissingWeatherHours, summary.HourlyAbsErrorWhSum, summary.HourlySqErrorWh2Sum, summary.ForecastTotalWh, summary.BaselineForecastTotalWh, summary.ActualTotalWh, summary.ForecastPeakWh, nullTime(summary.ForecastPeakTime), summary.BaselineForecastPeakWh, nullTime(summary.BaselineForecastPeakTime), summary.ActualPeakWh, nullTime(summary.ActualPeakTime), summary.CreatedAt.UTC(), summary.UpdatedAt.UTC()); err != nil {
			return fmt.Errorf("upsert solar forecast run daily verification rollup: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) RebuildDailyVerificationRollups(ctx context.Context, siteKey string, affectedDates map[string]time.Time, nowUTC time.Time) ([]solarforecastd.DailyVerificationRollup, error) {
	if len(affectedDates) == 0 {
		return nil, nil
	}
	dates := make([]time.Time, 0, len(affectedDates))
	for _, date := range affectedDates {
		dates = append(dates, date.UTC())
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	out := make([]solarforecastd.DailyVerificationRollup, 0, len(dates)*4)
	for _, date := range dates {
		if err := s.acquireAdvisoryXactLock(ctx, dailyRollupLockKey(siteKey, date)); err != nil {
			return nil, err
		}
		rollups, err := s.listDailyVerificationRollups(ctx, siteKey, date, nowUTC)
		if err != nil {
			return nil, err
		}
		out = append(out, rollups...)
	}
	return out, nil
}

func (s *PostgresStore) listDailyVerificationRollups(ctx context.Context, siteKey string, verificationDate, nowUTC time.Time) ([]solarforecastd.DailyVerificationRollup, error) {
	rows, err := s.executor().QueryContext(ctx, `
WITH base_rollups AS (
	SELECT
		site_key,
		device_id::text AS device_id,
		verification_local_date,
		timezone,
		forecast_version,
		served_variant,
		horizon_bucket,
		forecast_hours,
		verified_hours,
		missing_truth_hours,
		missing_weather_hours,
		hourly_abs_error_wh_sum,
		hourly_sq_error_wh2_sum,
		forecast_total_wh,
		baseline_forecast_total_wh,
		actual_total_wh,
		forecast_peak_wh,
		forecast_peak_time,
		baseline_forecast_peak_wh,
		baseline_forecast_peak_time,
		actual_peak_wh,
		actual_peak_time
	FROM solar_forecast_verification_daily_run_rollup
	WHERE site_key = $1
	  AND verification_local_date = $2::date
),
grouped AS (
	SELECT
		site_key,
		verification_local_date,
		forecast_version,
		served_variant,
		horizon_bucket,
		CASE
			WHEN COUNT(*) FILTER (WHERE device_id IS NOT NULL) = COUNT(*)
			 AND COUNT(DISTINCT device_id) = 1
			THEN MAX(device_id)
			ELSE NULL
		END AS device_id,
		MAX(timezone) AS timezone,
		COALESCE(SUM(forecast_hours), 0)::int AS forecast_hours,
		COALESCE(SUM(verified_hours), 0)::int AS verified_hours,
		COALESCE(SUM(missing_truth_hours), 0)::int AS missing_truth_hours,
		COALESCE(SUM(missing_weather_hours), 0)::int AS missing_weather_hours,
		COALESCE(SUM(hourly_abs_error_wh_sum), 0) AS hourly_abs_error_wh_sum,
		COALESCE(SUM(hourly_sq_error_wh2_sum), 0) AS hourly_sq_error_wh2_sum,
		COALESCE(SUM(forecast_total_wh), 0) AS forecast_total_wh,
		COALESCE(SUM(baseline_forecast_total_wh), 0) AS baseline_forecast_total_wh,
		COALESCE(SUM(actual_total_wh), 0) AS actual_total_wh
	FROM base_rollups
	GROUP BY site_key, verification_local_date, forecast_version, served_variant, horizon_bucket
),
forecast_peaks AS (
	SELECT DISTINCT ON (forecast_version, served_variant, horizon_bucket)
		forecast_version,
		served_variant,
		horizon_bucket,
		forecast_peak_wh,
		forecast_peak_time
	FROM base_rollups
	WHERE forecast_peak_time IS NOT NULL
	ORDER BY forecast_version, served_variant, horizon_bucket, forecast_peak_wh DESC, forecast_peak_time ASC
),
baseline_peaks AS (
	SELECT DISTINCT ON (forecast_version, served_variant, horizon_bucket)
		forecast_version,
		served_variant,
		horizon_bucket,
		baseline_forecast_peak_wh AS baseline_peak_wh,
		baseline_forecast_peak_time AS baseline_peak_time
	FROM base_rollups
	WHERE baseline_forecast_peak_time IS NOT NULL
	ORDER BY forecast_version, served_variant, horizon_bucket, baseline_forecast_peak_wh DESC, baseline_forecast_peak_time ASC
),
actual_peaks AS (
	SELECT DISTINCT ON (forecast_version, served_variant, horizon_bucket)
		forecast_version,
		served_variant,
		horizon_bucket,
		actual_peak_wh,
		actual_peak_time
	FROM base_rollups
	WHERE actual_peak_time IS NOT NULL
	ORDER BY forecast_version, served_variant, horizon_bucket, actual_peak_wh DESC, actual_peak_time ASC
)
SELECT
	g.site_key,
	g.device_id,
	g.served_variant,
	g.verification_local_date,
	g.timezone,
	g.forecast_version,
	g.horizon_bucket,
	g.forecast_hours,
	g.verified_hours,
	g.missing_truth_hours,
	g.missing_weather_hours,
	ROUND(g.hourly_abs_error_wh_sum::numeric, 1)::double precision,
	ROUND(g.hourly_sq_error_wh2_sum::numeric, 1)::double precision,
	ROUND(ABS(g.forecast_total_wh - g.actual_total_wh)::numeric, 1)::double precision,
	ROUND(ABS(g.baseline_forecast_total_wh - g.actual_total_wh)::numeric, 1)::double precision,
	ROUND(ABS(COALESCE(fp.forecast_peak_wh, 0) - COALESCE(ap.actual_peak_wh, 0))::numeric, 1)::double precision,
	ROUND(ABS(COALESCE(bp.baseline_peak_wh, 0) - COALESCE(ap.actual_peak_wh, 0))::numeric, 1)::double precision,
	ROUND(COALESCE(ABS(EXTRACT(EPOCH FROM (fp.forecast_peak_time - ap.actual_peak_time)) / 60.0), 0)::numeric, 1)::double precision,
	ROUND(COALESCE(ABS(EXTRACT(EPOCH FROM (bp.baseline_peak_time - ap.actual_peak_time)) / 60.0), 0)::numeric, 1)::double precision,
	$3::timestamptz,
	$3::timestamptz
FROM grouped g
LEFT JOIN forecast_peaks fp
	ON fp.forecast_version = g.forecast_version
	AND fp.served_variant = g.served_variant
	AND fp.horizon_bucket = g.horizon_bucket
LEFT JOIN baseline_peaks bp
	ON bp.forecast_version = g.forecast_version
	AND bp.served_variant = g.served_variant
	AND bp.horizon_bucket = g.horizon_bucket
LEFT JOIN actual_peaks ap
	ON ap.forecast_version = g.forecast_version
	AND ap.served_variant = g.served_variant
	AND ap.horizon_bucket = g.horizon_bucket
ORDER BY g.verification_local_date ASC, g.forecast_version ASC, g.served_variant ASC, g.horizon_bucket ASC;
`, siteKey, verificationDate.UTC(), nowUTC.UTC())
	if err != nil {
		return nil, fmt.Errorf("query solar forecast verification daily rollups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]solarforecastd.DailyVerificationRollup, 0)
	for rows.Next() {
		var (
			row              solarforecastd.DailyVerificationRollup
			deviceID         sql.NullString
			horizonBucketRaw string
		)
		if err := rows.Scan(
			&row.SiteKey,
			&deviceID,
			&row.ServedVariant,
			&row.VerificationLocalDate,
			&row.Timezone,
			&row.ForecastVersion,
			&horizonBucketRaw,
			&row.ForecastHours,
			&row.VerifiedHours,
			&row.MissingTruthHours,
			&row.MissingWeatherHours,
			&row.HourlyAbsErrorWhSum,
			&row.HourlySqErrorWh2Sum,
			&row.DailyAbsErrorWhSum,
			&row.BaselineDailyAbsErrorWhSum,
			&row.PeakPowerAbsErrorWSum,
			&row.BaselinePeakPowerAbsErrorWSum,
			&row.PeakTimeAbsErrorMinutesSum,
			&row.BaselinePeakTimeAbsErrorMinutesSum,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan solar forecast verification daily rollup: %w", err)
		}
		row.HorizonBucket = solarforecastd.HorizonBucket(horizonBucketRaw)
		if deviceID.Valid {
			row.DeviceID = &deviceID.String
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate solar forecast verification daily rollups: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) acquireAdvisoryXactLock(ctx context.Context, key string) error {
	if _, err := s.executor().ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint);`, key); err != nil {
		return fmt.Errorf("acquire solar forecast advisory lock: %w", err)
	}
	return nil
}

func calibrationLockKey(siteKey, forecastVersion string) string {
	return "solar-calibration|" + siteKey + "|" + forecastVersion
}

func dailyRollupLockKey(siteKey string, verificationDate time.Time) string {
	return "solar-rollup|" + siteKey + "|" + verificationDate.UTC().Format("2006-01-02")
}

func summarizeRunDailyVerificationRollups(run *solarforecastd.Run, rows []solarforecastd.HourlyTrainingRecord, verifiedAt time.Time) []runDailyVerificationRollup {
	type summaryKey struct {
		dateISO string
		horizon solarforecastd.HorizonBucket
	}

	grouped := make(map[summaryKey]*runDailyVerificationRollup)
	for _, row := range rows {
		key := summaryKey{
			dateISO: row.TargetLocalDate.UTC().Format("2006-01-02"),
			horizon: row.HorizonBucket,
		}
		summary := grouped[key]
		if summary == nil {
			summary = &runDailyVerificationRollup{
				RunID:                 run.ID,
				SiteKey:               run.SiteKey,
				DeviceID:              run.DeviceID,
				VerificationLocalDate: row.TargetLocalDate.UTC(),
				Timezone:              run.Timezone,
				ForecastVersion:       run.ForecastVersion,
				ServedVariant:         normalizeServedVariantValue(run.ServedVariant),
				HorizonBucket:         row.HorizonBucket,
				CreatedAt:             verifiedAt,
				UpdatedAt:             verifiedAt,
			}
			grouped[key] = summary
		}

		summary.ForecastHours++
		switch row.VerificationStatus {
		case solarforecastd.VerificationStatusVerified:
			summary.VerifiedHours++
			summary.ForecastTotalWh += row.ForecastGenerationWh
			if row.BaselineForecastGenerationWh != nil {
				summary.BaselineForecastTotalWh += *row.BaselineForecastGenerationWh
				updatePeak(&summary.BaselineForecastPeakWh, &summary.BaselineForecastPeakTime, *row.BaselineForecastGenerationWh, row.TargetTime.UTC())
			}
			if row.ActualGenerationWh != nil {
				summary.ActualTotalWh += *row.ActualGenerationWh
				updatePeak(&summary.ActualPeakWh, &summary.ActualPeakTime, *row.ActualGenerationWh, row.TargetTime.UTC())
			}
			if row.AbsoluteErrorWh != nil {
				summary.HourlyAbsErrorWhSum += *row.AbsoluteErrorWh
			}
			if row.SquaredErrorWh2 != nil {
				summary.HourlySqErrorWh2Sum += *row.SquaredErrorWh2
			}
			updatePeak(&summary.ForecastPeakWh, &summary.ForecastPeakTime, row.ForecastGenerationWh, row.TargetTime.UTC())
		case solarforecastd.VerificationStatusMissingTruth:
			summary.MissingTruthHours++
		case solarforecastd.VerificationStatusMissingWeather:
			summary.MissingWeatherHours++
		}
	}

	keys := make([]summaryKey, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].dateISO == keys[j].dateISO {
			return keys[i].horizon < keys[j].horizon
		}
		return keys[i].dateISO < keys[j].dateISO
	})

	out := make([]runDailyVerificationRollup, 0, len(keys))
	for _, key := range keys {
		out = append(out, *grouped[key])
	}
	return out
}

func updatePeak(currentValue *float64, currentTime **time.Time, candidateValue float64, candidateTime time.Time) {
	if currentTime == nil {
		return
	}
	if *currentTime == nil || candidateValue > *currentValue || (candidateValue == *currentValue && candidateTime.Before((**currentTime).UTC())) {
		*currentValue = candidateValue
		timeCopy := candidateTime.UTC()
		*currentTime = &timeCopy
	}
}

func normalizeServedVariantValue(value string) string {
	switch strings.TrimSpace(value) {
	case "site_calibrated":
		return "site_calibrated"
	default:
		return "baseline"
	}
}

func (s *PostgresStore) listPendingRowsForRun(ctx context.Context, runID string, before time.Time) ([]solarforecastd.HourlyTrainingRecord, error) {
	rows, err := s.executor().QueryContext(ctx, `
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
WHERE run_id = $1::uuid
  AND verification_status = 'pending'
  AND target_time < $2
ORDER BY target_time ASC
FOR UPDATE;
`, runID, before.UTC())
	if err != nil {
		return nil, fmt.Errorf("list solar forecast claimed hourly records: %w", err)
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
		return nil, fmt.Errorf("iterate solar forecast claimed hourly records: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) ensureTx(ctx context.Context) (*sql.Tx, bool, error) {
	if tx, ok := s.executor().(*sql.Tx); ok {
		return tx, false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	return tx, true, nil
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
	var out solarforecastd.VerificationRecord
	var (
		deviceID              sql.NullString
		baselineForecast      sql.NullFloat64
		actualGeneration      sql.NullFloat64
		absoluteError         sql.NullFloat64
		squaredError          sql.NullFloat64
		baselineAbsoluteError sql.NullFloat64
		baselineSquaredError  sql.NullFloat64
		horizonBucket         string
		verificationStatus    string
	)
	if err := scanner.Scan(
		&out.RunID,
		&deviceID,
		&out.TargetTime,
		&out.TargetLocalDate,
		&horizonBucket,
		&out.ForecastGenerationWh,
		&baselineForecast,
		&actualGeneration,
		&verificationStatus,
		&absoluteError,
		&squaredError,
		&baselineAbsoluteError,
		&baselineSquaredError,
		&out.ForecastVersion,
		&out.ServedVariant,
		&out.Timezone,
	); err != nil {
		return solarforecastd.VerificationRecord{}, fmt.Errorf("scan solar forecast verification record: %w", err)
	}
	if deviceID.Valid {
		out.DeviceID = &deviceID.String
	}
	out.HorizonBucket = solarforecastd.HorizonBucket(horizonBucket)
	out.BaselineForecastGenerationWh = nullableFloatPtr(baselineForecast)
	out.ActualGenerationWh = nullableFloatPtr(actualGeneration)
	out.VerificationStatus = solarforecastd.VerificationStatus(verificationStatus)
	out.AbsoluteErrorWh = nullableFloatPtr(absoluteError)
	out.SquaredErrorWh2 = nullableFloatPtr(squaredError)
	out.BaselineAbsoluteErrorWh = nullableFloatPtr(baselineAbsoluteError)
	out.BaselineSquaredErrorWh2 = nullableFloatPtr(baselineSquaredError)
	return out, nil
}

func scanRecentCalibrationRecord(scanner rowScanner) (solarforecastd.VerificationRecord, error) {
	var out solarforecastd.VerificationRecord
	var actualGeneration sql.NullFloat64
	if err := scanner.Scan(
		&out.RunID,
		&out.IssuedAt,
		&out.TargetTime,
		&out.TargetLocalDate,
		&out.ForecastGenerationWh,
		&actualGeneration,
		&out.VerificationStatus,
		&out.UpdatedAt,
		&out.ForecastVersion,
		&out.Timezone,
	); err != nil {
		return solarforecastd.VerificationRecord{}, fmt.Errorf("scan solar forecast recent calibration record: %w", err)
	}
	out.ActualGenerationWh = nullableFloatPtr(actualGeneration)
	return out, nil
}

func nullableFloatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}
