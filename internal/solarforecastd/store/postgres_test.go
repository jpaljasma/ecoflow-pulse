package store

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd"
)

func TestActiveForecastInputFromMetadataBuildsAllScopeInput(t *testing.T) {
	t.Parallel()

	input, ok := activeForecastInputFromMetadata("America/New_York", []byte(`{
		"scope_mode":"all",
		"resolved_device_ids":["dev-b","dev-a","dev-a"],
		"request_latitude":42.61,
		"request_longitude":-77.40,
		"panel_tilt_degrees":45,
		"panel_azimuth_degrees":0
	}`))
	if !ok {
		t.Fatal("activeForecastInputFromMetadata() ok = false, want true")
	}
	if got, want := input.Scope.Mode, "all"; got != want {
		t.Fatalf("scope mode = %q, want %q", got, want)
	}
	if got, want := input.ResolvedDeviceIDs, []string{"dev-a", "dev-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("resolved device IDs = %#v, want %#v", got, want)
	}
	if got, want := input.WeatherRequest.Timezone, "America/New_York"; got != want {
		t.Fatalf("timezone = %q, want %q", got, want)
	}
	if input.WeatherRequest.PanelTiltDegrees == nil || *input.WeatherRequest.PanelTiltDegrees != 45 {
		t.Fatalf("panel tilt = %v, want 45", input.WeatherRequest.PanelTiltDegrees)
	}
}

func TestActiveForecastInputFromMetadataPreservesDeviceScopeWithSiteContext(t *testing.T) {
	t.Parallel()

	input, ok := activeForecastInputFromMetadata("America/New_York", []byte(`{
		"scope_mode":"device",
		"resolved_device_ids":["dev-b"],
		"site_resolved_device_ids":["dev-a","dev-b"],
		"request_latitude":42.61,
		"request_longitude":-77.40
	}`))
	if !ok {
		t.Fatal("activeForecastInputFromMetadata() ok = false, want true")
	}
	if got, want := input.Scope.Mode, "device"; got != want {
		t.Fatalf("scope mode = %q, want %q", got, want)
	}
	if got, want := input.Scope.DeviceID, "dev-b"; got != want {
		t.Fatalf("scope device ID = %q, want %q", got, want)
	}
	if got, want := input.ResolvedDeviceIDs, []string{"dev-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("resolved device IDs = %#v, want %#v", got, want)
	}
	if got, want := input.SiteResolvedDeviceIDs, []string{"dev-a", "dev-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("site device IDs = %#v, want %#v", got, want)
	}
}

func TestPostgresStoreListActiveForecastInputsLimitsNewestSitesAfterDeduplication(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &PostgresStore{db: db}
	since := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT\s+timezone,\s+site_metadata_json\s+FROM\s+\(\s+SELECT DISTINCT ON \(site_key\)\s+site_key,\s+timezone,\s+site_metadata_json,\s+created_at\s+FROM solar_forecast_runs\s+WHERE created_at >= \$1\s+AND site_metadata_json \? 'request_latitude'\s+AND site_metadata_json \? 'request_longitude'\s+ORDER BY site_key, created_at DESC\s+\) latest\s+ORDER BY created_at DESC\s+LIMIT \$2;`).
		WithArgs(since, 2).
		WillReturnRows(sqlmock.NewRows([]string{"timezone", "site_metadata_json"}).
			AddRow("America/New_York", []byte(`{
				"scope_mode":"all",
				"resolved_device_ids":["newest-site"],
				"request_latitude":42.61,
				"request_longitude":-77.40
			}`)).
			AddRow("America/New_York", []byte(`{
				"scope_mode":"all",
				"resolved_device_ids":["second-newest-site"],
				"request_latitude":42.62,
				"request_longitude":-77.41
			}`)))

	inputs, err := store.ListActiveForecastInputs(context.Background(), since, 2)
	if err != nil {
		t.Fatalf("ListActiveForecastInputs() error = %v", err)
	}
	if len(inputs) != 2 {
		t.Fatalf("ListActiveForecastInputs() = %d inputs, want 2", len(inputs))
	}
	if got, want := inputs[0].ResolvedDeviceIDs, []string{"newest-site"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first input device IDs = %#v, want %#v", got, want)
	}
	if got, want := inputs[1].ResolvedDeviceIDs, []string{"second-newest-site"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second input device IDs = %#v, want %#v", got, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func TestPostgresStoreInsertRunUpsertsOnConflictAndReturnsCanonicalID(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &PostgresStore{db: db}
	now := time.Date(2026, 3, 19, 18, 0, 0, 0, time.UTC)
	run := solarforecastd.Run{
		ID:                       "11111111-1111-1111-1111-111111111111",
		SiteKey:                  "grid:42.61:-77.40:290|tilt:45|az:0|dev-a",
		ScopeKind:                "device",
		ServedVariant:            "baseline",
		CanonicalLocationKey:     "grid:42.61:-77.40:290|tilt:45|az:0",
		Timezone:                 "America/New_York",
		IssuedAt:                 now,
		IssueLocalDate:           now,
		IssueLocalHour:           14,
		IssueUTCOffsetMinutes:    -240,
		ForecastVersion:          "deterministic_baseline_v1",
		FeatureVersion:           "weather_v1",
		ActualSoFarWh:            123.4,
		ForecastRemainingTodayWh: 456.7,
		ForecastTotalTodayWh:     580.1,
		CreatedAt:                now,
		UpdatedAt:                now,
	}

	mock.ExpectQuery(`(?s)INSERT INTO solar_forecast_runs .* ON CONFLICT \(site_key, issued_at, forecast_version\) DO UPDATE SET .* RETURNING id::text;`).
		WithArgs(
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("canonical-run-id"))

	got, err := store.InsertRun(context.Background(), run)
	if err != nil {
		t.Fatalf("InsertRun() error = %v", err)
	}
	if got, want := got, "canonical-run-id"; got != want {
		t.Fatalf("InsertRun() returned id = %q, want %q", got, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func TestPostgresStoreListVerificationRecordsUsesRollupQueryWindow(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &PostgresStore{db: db}
	fromDate := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT\s+h.run_id::text,\s*h.device_id::text,\s*h.target_time,\s*h.target_local_date,\s*h.horizon_bucket,\s*h.forecast_generation_wh,\s*h.baseline_forecast_generation_wh,\s*h.actual_generation_wh,\s*h.verification_status,\s*h.absolute_error_wh,\s*h.squared_error_wh2,\s*h.baseline_absolute_error_wh,\s*h.baseline_squared_error_wh2,\s*r.forecast_version,\s*r.served_variant,\s*r.timezone\s+FROM solar_forecast_hourly_training_records h\s+JOIN solar_forecast_runs r ON r.id = h.run_id\s+WHERE h.site_key = \$1\s+AND h.target_local_date BETWEEN \$2 AND \$3\s+ORDER BY h.target_local_date ASC, h.target_time ASC;`).
		WithArgs("site-key", fromDate, toDate).
		WillReturnRows(sqlmock.NewRows([]string{"run_id"}))

	records, err := store.ListVerificationRecords(context.Background(), "site-key", fromDate, toDate)
	if err != nil {
		t.Fatalf("ListVerificationRecords() error = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("ListVerificationRecords() = %d records, want 0", len(records))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func TestPostgresStoreListRecentCalibrationRecordsFiltersVerifiedForecastRows(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &PostgresStore{db: db}
	fromDate := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT DISTINCT ON \(h.target_time\)\s+h.run_id::text,\s*h.issued_at,\s*h.target_time,\s*h.target_local_date,\s*h.forecast_generation_wh,\s*h.actual_generation_wh,\s*h.verification_status,\s*h.updated_at,\s*r.forecast_version,\s*r.timezone\s+FROM solar_forecast_hourly_training_records h\s+JOIN solar_forecast_runs r ON r.id = h.run_id\s+WHERE h.site_key = \$1\s+AND r.forecast_version = \$2\s+AND h.verification_status = 'verified'\s+AND h.actual_generation_wh IS NOT NULL\s+AND h.target_local_date BETWEEN \$3 AND \$4\s+ORDER BY h.target_time ASC, h.issued_at DESC;`).
		WithArgs("site-key", "deterministic_baseline_v1", fromDate, toDate).
		WillReturnRows(sqlmock.NewRows([]string{"run_id"}))

	records, err := store.ListRecentCalibrationRecords(context.Background(), "site-key", "deterministic_baseline_v1", fromDate, toDate)
	if err != nil {
		t.Fatalf("ListRecentCalibrationRecords() error = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("ListRecentCalibrationRecords() = %d records, want 0", len(records))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func TestPostgresStoreListVerificationRecordsScansRollupFieldsWithoutWeatherJSON(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &PostgresStore{db: db}
	fromDate := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
	target := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"run_id",
		"device_id",
		"target_time",
		"target_local_date",
		"horizon_bucket",
		"forecast_generation_wh",
		"baseline_forecast_generation_wh",
		"actual_generation_wh",
		"verification_status",
		"absolute_error_wh",
		"squared_error_wh2",
		"baseline_absolute_error_wh",
		"baseline_squared_error_wh2",
		"forecast_version",
		"served_variant",
		"timezone",
	}).AddRow(
		"run-1",
		"device-1",
		target,
		time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
		"day_1",
		120.0,
		100.0,
		95.0,
		"verified",
		25.0,
		625.0,
		30.0,
		900.0,
		"deterministic_baseline_v1",
		"site_calibrated",
		"America/New_York",
	)

	mock.ExpectQuery(`(?s)SELECT\s+h.run_id::text,\s*h.device_id::text,\s*h.target_time,\s*h.target_local_date,\s*h.horizon_bucket,\s*h.forecast_generation_wh,\s*h.baseline_forecast_generation_wh,\s*h.actual_generation_wh,\s*h.verification_status,\s*h.absolute_error_wh,\s*h.squared_error_wh2,\s*h.baseline_absolute_error_wh,\s*h.baseline_squared_error_wh2,\s*r.forecast_version,\s*r.served_variant,\s*r.timezone\s+FROM solar_forecast_hourly_training_records h\s+JOIN solar_forecast_runs r ON r.id = h.run_id\s+WHERE h.site_key = \$1\s+AND h.target_local_date BETWEEN \$2 AND \$3\s+ORDER BY h.target_local_date ASC, h.target_time ASC;`).
		WithArgs("site-key", fromDate, toDate).
		WillReturnRows(rows)

	records, err := store.ListVerificationRecords(context.Background(), "site-key", fromDate, toDate)
	if err != nil {
		t.Fatalf("ListVerificationRecords() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("ListVerificationRecords() = %d records, want 1", len(records))
	}
	record := records[0]
	if record.DeviceID == nil || *record.DeviceID != "device-1" {
		t.Fatalf("ListVerificationRecords() device ID = %v, want device-1", record.DeviceID)
	}
	if got, want := record.ForecastVersion, "deterministic_baseline_v1"; got != want {
		t.Fatalf("ListVerificationRecords() forecast version = %q, want %q", got, want)
	}
	if got, want := record.ServedVariant, "site_calibrated"; got != want {
		t.Fatalf("ListVerificationRecords() served variant = %q, want %q", got, want)
	}
	if got, want := record.Timezone, "America/New_York"; got != want {
		t.Fatalf("ListVerificationRecords() timezone = %q, want %q", got, want)
	}
	if record.BaselineForecastGenerationWh == nil || *record.BaselineForecastGenerationWh != 100 {
		t.Fatalf("ListVerificationRecords() baseline forecast = %v, want 100", record.BaselineForecastGenerationWh)
	}
	if record.AbsoluteErrorWh == nil || *record.AbsoluteErrorWh != 25 {
		t.Fatalf("ListVerificationRecords() absolute error = %v, want 25", record.AbsoluteErrorWh)
	}
	if record.SquaredErrorWh2 == nil || *record.SquaredErrorWh2 != 625 {
		t.Fatalf("ListVerificationRecords() squared error = %v, want 625", record.SquaredErrorWh2)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func TestPostgresStoreListRecentCalibrationRecordsScansLightweightFields(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &PostgresStore{db: db}
	fromDate := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
	issuedAt := time.Date(2026, 3, 19, 15, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 3, 19, 16, 0, 0, 0, time.UTC)
	target := time.Date(2026, 3, 18, 15, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"run_id",
		"issued_at",
		"target_time",
		"target_local_date",
		"forecast_generation_wh",
		"actual_generation_wh",
		"verification_status",
		"updated_at",
		"forecast_version",
		"timezone",
	}).AddRow(
		"run-1",
		issuedAt,
		target,
		time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
		100.0,
		80.0,
		"verified",
		updatedAt,
		"deterministic_baseline_v1",
		"America/New_York",
	)

	mock.ExpectQuery(`(?s)SELECT DISTINCT ON \(h.target_time\)\s+h.run_id::text,\s*h.issued_at,\s*h.target_time,\s*h.target_local_date,\s*h.forecast_generation_wh,\s*h.actual_generation_wh,\s*h.verification_status,\s*h.updated_at,\s*r.forecast_version,\s*r.timezone\s+FROM solar_forecast_hourly_training_records h\s+JOIN solar_forecast_runs r ON r.id = h.run_id\s+WHERE h.site_key = \$1\s+AND r.forecast_version = \$2\s+AND h.verification_status = 'verified'\s+AND h.actual_generation_wh IS NOT NULL\s+AND h.target_local_date BETWEEN \$3 AND \$4\s+ORDER BY h.target_time ASC, h.issued_at DESC;`).
		WithArgs("site-key", "deterministic_baseline_v1", fromDate, toDate).
		WillReturnRows(rows)

	records, err := store.ListRecentCalibrationRecords(context.Background(), "site-key", "deterministic_baseline_v1", fromDate, toDate)
	if err != nil {
		t.Fatalf("ListRecentCalibrationRecords() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("ListRecentCalibrationRecords() = %d records, want 1", len(records))
	}
	record := records[0]
	if got, want := record.IssuedAt.UTC(), issuedAt.UTC(); !got.Equal(want) {
		t.Fatalf("ListRecentCalibrationRecords() issued_at = %v, want %v", got, want)
	}
	if got, want := record.UpdatedAt.UTC(), updatedAt.UTC(); !got.Equal(want) {
		t.Fatalf("ListRecentCalibrationRecords() updated_at = %v, want %v", got, want)
	}
	if record.ActualGenerationWh == nil || *record.ActualGenerationWh != 80 {
		t.Fatalf("ListRecentCalibrationRecords() actual generation = %v, want 80", record.ActualGenerationWh)
	}
	if got, want := record.ForecastVersion, "deterministic_baseline_v1"; got != want {
		t.Fatalf("ListRecentCalibrationRecords() forecast version = %q, want %q", got, want)
	}
	if got, want := record.Timezone, "America/New_York"; got != want {
		t.Fatalf("ListRecentCalibrationRecords() timezone = %q, want %q", got, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func TestPostgresStoreClaimPendingRunIDLocksRunsWithSkipLocked(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &PostgresStore{db: db}
	before := time.Date(2026, 3, 27, 16, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)WITH candidate_runs AS \(\s*SELECT\s+run_id,\s+MIN\(target_time\) AS first_target_time\s+FROM solar_forecast_hourly_training_records\s+WHERE verification_status = 'pending'\s+AND target_time < \$1\s+GROUP BY run_id\s+ORDER BY first_target_time ASC\s+LIMIT \$2\s*\),\s*claimed_run AS \(\s*SELECT r.id::text AS run_id\s+FROM candidate_runs\s+JOIN solar_forecast_runs r ON r.id = candidate_runs.run_id\s+ORDER BY candidate_runs.first_target_time ASC\s+FOR UPDATE OF r SKIP LOCKED\s+LIMIT 1\s*\)\s*SELECT run_id\s+FROM claimed_run;`).
		WithArgs(before, pendingRunClaimCandidateLimit).
		WillReturnRows(sqlmock.NewRows([]string{"run_id"}).AddRow("run-123"))

	runID, err := store.claimPendingRunID(context.Background(), before)
	if err != nil {
		t.Fatalf("claimPendingRunID() error = %v", err)
	}
	if got, want := runID, "run-123"; got != want {
		t.Fatalf("claimPendingRunID() = %q, want %q", got, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func TestPostgresStoreClaimPendingRunRollsBackEmptyClaims(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &PostgresStore{db: db}
	before := time.Date(2026, 3, 27, 16, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)WITH candidate_runs AS \(\s*SELECT\s+run_id,\s+MIN\(target_time\) AS first_target_time\s+FROM solar_forecast_hourly_training_records\s+WHERE verification_status = 'pending'\s+AND target_time < \$1\s+GROUP BY run_id\s+ORDER BY first_target_time ASC\s+LIMIT \$2\s*\),\s*claimed_run AS \(\s*SELECT r.id::text AS run_id\s+FROM candidate_runs\s+JOIN solar_forecast_runs r ON r.id = candidate_runs.run_id\s+ORDER BY candidate_runs.first_target_time ASC\s+FOR UPDATE OF r SKIP LOCKED\s+LIMIT 1\s*\)\s*SELECT run_id\s+FROM claimed_run;`).
		WithArgs(before, pendingRunClaimCandidateLimit).
		WillReturnRows(sqlmock.NewRows([]string{"run_id"}).AddRow("run-123"))
	mock.ExpectQuery(`(?s)FROM solar_forecast_hourly_training_records\s+WHERE run_id = \$1::uuid\s+AND verification_status = 'pending'\s+AND target_time < \$2\s+ORDER BY target_time ASC\s+FOR UPDATE;`).
		WithArgs("run-123", before).
		WillReturnRows(sqlmock.NewRows([]string{"run_id"}))
	mock.ExpectRollback()

	claim, err := store.ClaimPendingRun(context.Background(), before)
	if err != nil {
		t.Fatalf("ClaimPendingRun() error = %v", err)
	}
	if claim != nil {
		t.Fatalf("ClaimPendingRun() = %#v, want nil", claim)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func TestPostgresStoreApplyCalibrationRowsUsesAdvisoryLockAndAtomicUpsert(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &PostgresStore{db: db}
	verifiedAt := time.Date(2026, 3, 27, 16, 0, 0, 0, time.UTC)
	rows := []solarforecastd.HourlyTrainingRecord{
		{
			HorizonBucket:        solarforecastd.HorizonBucket("day_1"),
			TargetLocalHour:      13,
			TargetTime:           verifiedAt.Add(-time.Hour),
			ForecastGenerationWh: 100,
			ActualGenerationWh:   floatPtr(80),
			VerificationStatus:   solarforecastd.VerificationStatusVerified,
		},
	}

	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtext\(\$1\)::bigint\);`).
		WithArgs(calibrationLockKey("site-key", "deterministic_baseline_v1")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO solar_forecast_calibration_state .* ON CONFLICT \(site_key, forecast_version, horizon_bucket, hour_of_day\)\s+DO UPDATE SET\s+sample_count = solar_forecast_calibration_state.sample_count \+ 1,`).
		WithArgs(
			"site-key",
			"deterministic_baseline_v1",
			"day_1",
			13,
			0.8,
			verifiedAt,
			solarforecastd.CalibrationRatioMinClamp,
			solarforecastd.CalibrationRatioMaxClamp,
			solarforecastd.CalibrationEWMAAlpha,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.ApplyCalibrationRows(context.Background(), "site-key", "deterministic_baseline_v1", verifiedAt, rows); err != nil {
		t.Fatalf("ApplyCalibrationRows() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func TestPostgresStoreRebuildDailyVerificationRollupsLocksDateAndScansAggregateRows(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &PostgresStore{db: db}
	day := time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)
	nowUTC := time.Date(2026, 3, 27, 16, 0, 0, 0, time.UTC)
	affectedDates := map[string]time.Time{"2026-03-27": day}

	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtext\(\$1\)::bigint\);`).
		WithArgs(dailyRollupLockKey("site-key", day)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)WITH base_rollups AS .* FROM grouped g .* ORDER BY g.verification_local_date ASC, g.forecast_version ASC, g.served_variant ASC, g.horizon_bucket ASC;`).
		WithArgs("site-key", day, nowUTC).
		WillReturnRows(sqlmock.NewRows([]string{
			"site_key",
			"device_id",
			"served_variant",
			"verification_local_date",
			"timezone",
			"forecast_version",
			"horizon_bucket",
			"forecast_hours",
			"verified_hours",
			"missing_truth_hours",
			"missing_weather_hours",
			"hourly_abs_error_wh_sum",
			"hourly_sq_error_wh2_sum",
			"daily_abs_error_wh_sum",
			"baseline_daily_abs_error_wh_sum",
			"peak_power_abs_error_w_sum",
			"baseline_peak_power_abs_error_w_sum",
			"peak_time_abs_error_minutes_sum",
			"baseline_peak_time_abs_error_minutes_sum",
			"created_at",
			"updated_at",
		}).AddRow(
			"site-key",
			"device-1",
			"baseline",
			day,
			"America/New_York",
			"deterministic_baseline_v1",
			"day_1",
			24,
			20,
			4,
			0,
			125.0,
			625.0,
			50.0,
			45.0,
			20.0,
			18.0,
			30.0,
			25.0,
			nowUTC,
			nowUTC,
		))

	rollups, err := store.RebuildDailyVerificationRollups(context.Background(), "site-key", affectedDates, nowUTC)
	if err != nil {
		t.Fatalf("RebuildDailyVerificationRollups() error = %v", err)
	}
	if len(rollups) != 1 {
		t.Fatalf("RebuildDailyVerificationRollups() = %d rows, want 1", len(rollups))
	}
	if got, want := rollups[0].HorizonBucket, solarforecastd.HorizonBucket("day_1"); got != want {
		t.Fatalf("RebuildDailyVerificationRollups() horizon = %q, want %q", got, want)
	}
	if rollups[0].DeviceID == nil || *rollups[0].DeviceID != "device-1" {
		t.Fatalf("RebuildDailyVerificationRollups() device ID = %v, want device-1", rollups[0].DeviceID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations were not met: %v", err)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
