package store

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd"
)

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

	mock.ExpectQuery(`(?s)FROM solar_forecast_hourly_training_records h JOIN solar_forecast_runs r ON r.id = h.run_id WHERE h.site_key = \$1.*h.target_local_date BETWEEN \$2 AND \$3.*ORDER BY h.target_local_date ASC, h.target_time ASC;`).
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

	mock.ExpectQuery(`(?s)FROM solar_forecast_hourly_training_records h JOIN solar_forecast_runs r ON r.id = h.run_id WHERE h.site_key = \$1.*r.forecast_version = \$2.*h.verification_status = 'verified'.*h.actual_generation_wh IS NOT NULL.*h.target_local_date BETWEEN \$3 AND \$4.*ORDER BY h.target_local_date ASC, h.target_time ASC;`).
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
