package telemetryquery

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNewPostgresReaderRejectsWhitespaceDSN(t *testing.T) {
	t.Parallel()

	if _, err := NewPostgresReader("   "); err == nil {
		t.Fatalf("expected whitespace dsn to fail")
	}
}

func TestPostgresReaderQueryRangeUsesHourTableAndScansMetrics(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	reader := newPostgresReader(db)
	from := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	to := from.Add(2 * time.Hour)
	rows := sqlmock.NewRows([]string{
		"bucket_start",
		"sample_count",
		"first_ts_unix_ms",
		"last_ts_unix_ms",
		"soc_avg_pct",
		"soc_min_pct",
		"soc_max_pct",
		"ac_in_avg_w",
		"ac_in_max_w",
		"ac_output_avg_w",
		"ac_output_max_w",
		"pv_avg_w",
		"pv_max_w",
		"dc_avg_w",
		"dc_max_w",
		"load_avg_w",
		"load_max_w",
		"net_avg_w",
		"net_min_w",
		"net_max_w",
		"battery_avg_w",
		"battery_min_w",
		"battery_max_w",
		"temp_avg_c",
		"temp_min_c",
		"temp_max_c",
		"solar_generated_wh",
		"ac_input_energy_wh",
		"ac_output_energy_wh",
		"dc_output_energy_wh",
		"load_energy_wh",
		"battery_charge_energy_wh",
		"battery_discharge_energy_wh",
	}).AddRow(
		from,
		42,
		from.UnixMilli(),
		from.Add(59*time.Minute).UnixMilli(),
		25.5,
		25.0,
		26.0,
		150.0,
		200.0,
		140.0,
		220.0,
		50.0,
		60.0,
		nil,
		nil,
		210.0,
		240.0,
		-60.0,
		-80.0,
		-40.0,
		35.0,
		20.0,
		50.0,
		8.5,
		7.0,
		9.0,
		0.75,
		1.5,
		2.5,
		3.5,
		4.5,
		5.5,
		6.5,
	)

	mock.ExpectQuery(regexp.QuoteMeta(buildQuery(ResolutionHour, "telemetry_rollup_hour"))).
		WithArgs("018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52", from, to, 2).
		WillReturnRows(rows)

	series, err := reader.QueryRange(context.Background(), RangeQuery{
		DeviceID:   "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		Resolution: ResolutionHour,
		From:       from,
		To:         to,
		Limit:      2,
	})
	if err != nil {
		t.Fatalf("QueryRange failed: %v", err)
	}
	if got := len(series.Points); got != 1 {
		t.Fatalf("points mismatch: got=%d want=1", got)
	}
	point := series.Points[0]
	if point.BucketEnd != from.Add(time.Hour) {
		t.Fatalf("bucket end mismatch: got=%s want=%s", point.BucketEnd, from.Add(time.Hour))
	}
	if point.Metrics.PVAvgW == nil || *point.Metrics.PVAvgW != 50 {
		t.Fatalf("pv_avg_w mismatch: got=%v", point.Metrics.PVAvgW)
	}
	if point.Metrics.DCAvgW != nil {
		t.Fatalf("expected dc_avg_w to be nil, got=%v", *point.Metrics.DCAvgW)
	}
	if point.Metrics.SolarGeneratedWh == nil || *point.Metrics.SolarGeneratedWh != 0.75 {
		t.Fatalf("solar_generated_wh mismatch: got=%v", point.Metrics.SolarGeneratedWh)
	}
	if point.Metrics.ACInputEnergyWh == nil || *point.Metrics.ACInputEnergyWh != 1.5 {
		t.Fatalf("ac_input_energy_wh mismatch: got=%v", point.Metrics.ACInputEnergyWh)
	}
	if point.Metrics.ACOutputAvgW == nil || *point.Metrics.ACOutputAvgW != 140.0 {
		t.Fatalf("ac_output_avg_w mismatch: got=%v", point.Metrics.ACOutputAvgW)
	}
	if point.Metrics.ACOutputMaxW == nil || *point.Metrics.ACOutputMaxW != 220.0 {
		t.Fatalf("ac_output_max_w mismatch: got=%v", point.Metrics.ACOutputMaxW)
	}
	if point.Metrics.ACOutputEnergyWh == nil || *point.Metrics.ACOutputEnergyWh != 2.5 {
		t.Fatalf("ac_output_energy_wh mismatch: got=%v", point.Metrics.ACOutputEnergyWh)
	}
	if point.Metrics.DCOutputEnergyWh == nil || *point.Metrics.DCOutputEnergyWh != 3.5 {
		t.Fatalf("dc_output_energy_wh mismatch: got=%v", point.Metrics.DCOutputEnergyWh)
	}
	if point.Metrics.LoadEnergyWh == nil || *point.Metrics.LoadEnergyWh != 4.5 {
		t.Fatalf("load_energy_wh mismatch: got=%v", point.Metrics.LoadEnergyWh)
	}
	if point.Metrics.BatteryChargeEnergyWh == nil || *point.Metrics.BatteryChargeEnergyWh != 5.5 {
		t.Fatalf("battery_charge_energy_wh mismatch: got=%v", point.Metrics.BatteryChargeEnergyWh)
	}
	if point.Metrics.BatteryDischargeEnergyWh == nil || *point.Metrics.BatteryDischargeEnergyWh != 6.5 {
		t.Fatalf("battery_discharge_energy_wh mismatch: got=%v", point.Metrics.BatteryDischargeEnergyWh)
	}
	if series.EnergyBucketCoverage.PointCount != 1 {
		t.Fatalf("coverage point count mismatch: got=%d want=1", series.EnergyBucketCoverage.PointCount)
	}
	if series.EnergyBucketCoverage.PersistedValueCount != 7 {
		t.Fatalf("coverage persisted values mismatch: got=%d want=7", series.EnergyBucketCoverage.PersistedValueCount)
	}
	if series.EnergyBucketCoverage.DerivedValueCount != 0 {
		t.Fatalf("coverage derived values mismatch: got=%d want=0", series.EnergyBucketCoverage.DerivedValueCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresReaderQueryRangeRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	reader := newPostgresReader(nil)
	_, err := reader.QueryRange(context.Background(), RangeQuery{
		DeviceID:   "",
		Resolution: ResolutionHour,
		From:       time.Now().UTC(),
		To:         time.Now().UTC().Add(time.Hour),
		Limit:      10,
	})
	if err == nil {
		t.Fatalf("expected invalid input error")
	}
}

func TestPostgresReaderQueryPVPortHistoryUsesHourTableAndScansRows(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	reader := newPostgresReader(db)
	from := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	to := from.Add(6 * time.Hour)
	rows := sqlmock.NewRows([]string{
		"device_id",
		"port_id",
		"port_label",
		"max_observed_volts",
		"max_observed_amps",
		"max_observed_watts",
		"last_observed_volts",
		"last_observed_amps",
		"last_observed_watts",
		"last_observed_at_unix_ms",
		"sample_count",
	}).AddRow(
		"018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		"pv-low",
		"PV Low",
		48.2,
		4.6,
		221.7,
		47.9,
		4.1,
		196.4,
		to.Add(-10*time.Minute).UnixMilli(),
		7,
	)

	sqlQuery, _ := buildPVPortHistoryQuery("telemetry_rollup_pv_port_hour", []string{"018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52"}, from, to)
	mock.ExpectQuery(regexp.QuoteMeta(sqlQuery)).
		WithArgs("018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52", from, to).
		WillReturnRows(rows)

	got, err := reader.QueryPVPortHistory(context.Background(), PVPortHistoryQuery{
		DeviceIDs:  []string{"018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52"},
		Resolution: ResolutionHour,
		From:       from,
		To:         to,
	})
	if err != nil {
		t.Fatalf("QueryPVPortHistory failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("row count mismatch: got=%d want=1", len(got))
	}
	if got[0].PortID != "pv-low" || got[0].PortLabel != "PV Low" {
		t.Fatalf("port identity mismatch: %+v", got[0])
	}
	if got[0].MaxObservedWatts != 221.7 || got[0].LastObservedWatts != 196.4 {
		t.Fatalf("watts mismatch: %+v", got[0])
	}
	if !got[0].LastObservedAt.Equal(to.Add(-10 * time.Minute)) {
		t.Fatalf("last observed mismatch: got=%s want=%s", got[0].LastObservedAt, to.Add(-10*time.Minute))
	}
	if got[0].SampleCount != 7 {
		t.Fatalf("sample count mismatch: got=%d want=7", got[0].SampleCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresReaderQueryRangeFiveMinuteAggregatesMinuteTable(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	reader := newPostgresReader(db)
	from := time.Date(2026, time.March, 12, 12, 0, 0, 0, time.UTC)
	to := from.Add(15 * time.Minute)
	rows := sqlmock.NewRows([]string{
		"bucket_start",
		"sample_count",
		"first_ts_unix_ms",
		"last_ts_unix_ms",
		"soc_avg_pct",
		"soc_min_pct",
		"soc_max_pct",
		"ac_in_avg_w",
		"ac_in_max_w",
		"ac_output_avg_w",
		"ac_output_max_w",
		"pv_avg_w",
		"pv_max_w",
		"dc_avg_w",
		"dc_max_w",
		"load_avg_w",
		"load_max_w",
		"net_avg_w",
		"net_min_w",
		"net_max_w",
		"battery_avg_w",
		"battery_min_w",
		"battery_max_w",
		"temp_avg_c",
		"temp_min_c",
		"temp_max_c",
		"solar_generated_wh",
		"ac_input_energy_wh",
		"ac_output_energy_wh",
		"dc_output_energy_wh",
		"load_energy_wh",
		"battery_charge_energy_wh",
		"battery_discharge_energy_wh",
	}).AddRow(
		from,
		250,
		from.UnixMilli(),
		from.Add(4*time.Minute+59*time.Second).UnixMilli(),
		50.0,
		49.0,
		51.0,
		120.0,
		180.0,
		80.0,
		100.0,
		60.0,
		75.0,
		10.0,
		15.0,
		130.0,
		160.0,
		-20.0,
		-25.0,
		-10.0,
		40.0,
		-5.0,
		50.0,
		21.0,
		20.0,
		22.0,
		5.0,
		10.0,
		6.5,
		0.8,
		10.8,
		1.6,
		0.9,
	)

	mock.ExpectQuery(regexp.QuoteMeta(buildQuery(ResolutionFiveMinutes, "telemetry_rollup_minute"))).
		WithArgs("018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52", from, to, 16).
		WillReturnRows(rows)

	series, err := reader.QueryRange(context.Background(), RangeQuery{
		DeviceID:   "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		Resolution: ResolutionFiveMinutes,
		From:       from,
		To:         to,
		Limit:      16,
	})
	if err != nil {
		t.Fatalf("QueryRange failed: %v", err)
	}
	if got := len(series.Points); got != 1 {
		t.Fatalf("points mismatch: got=%d want=1", got)
	}
	if got := series.Points[0].BucketEnd; got != from.Add(5*time.Minute) {
		t.Fatalf("bucket end mismatch: got=%s want=%s", got, from.Add(5*time.Minute))
	}
	if got := series.Resolution; got != ResolutionFiveMinutes {
		t.Fatalf("resolution mismatch: got=%v want=%v", got, ResolutionFiveMinutes)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresReaderQueryRangeManyAggregatesBuckets(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	reader := newPostgresReader(db)
	from := time.Date(2026, time.March, 11, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	rows := sqlmock.NewRows([]string{
		"bucket_start",
		"sample_count",
		"first_ts_unix_ms",
		"last_ts_unix_ms",
		"soc_avg_pct",
		"soc_min_pct",
		"soc_max_pct",
		"ac_in_avg_w",
		"ac_in_max_w",
		"ac_output_avg_w",
		"ac_output_max_w",
		"pv_avg_w",
		"pv_max_w",
		"dc_avg_w",
		"dc_max_w",
		"load_avg_w",
		"load_max_w",
		"net_avg_w",
		"net_min_w",
		"net_max_w",
		"battery_avg_w",
		"battery_min_w",
		"battery_max_w",
		"temp_avg_c",
		"temp_min_c",
		"temp_max_c",
		"solar_generated_wh",
		"ac_input_energy_wh",
		"ac_output_energy_wh",
		"dc_output_energy_wh",
		"load_energy_wh",
		"battery_charge_energy_wh",
		"battery_discharge_energy_wh",
	}).AddRow(
		from,
		84,
		from.UnixMilli(),
		from.Add(59*time.Minute).UnixMilli(),
		52.0,
		45.0,
		60.0,
		300.0,
		400.0,
		180.0,
		240.0,
		120.0,
		150.0,
		40.0,
		55.0,
		360.0,
		430.0,
		-20.0,
		-30.0,
		10.0,
		80.0,
		-10.0,
		95.0,
		22.5,
		20.0,
		25.0,
		720.0,
		180.0,
		150.0,
		40.0,
		360.0,
		90.0,
		70.0,
	)

	querySQL, _ := buildAggregateQuery(ResolutionDay, "telemetry_rollup_day", []string{"dev-a", "dev-b"}, from, to, 14)
	mock.ExpectQuery(regexp.QuoteMeta(querySQL)).
		WithArgs("dev-a", "dev-b", from, to, 14).
		WillReturnRows(rows)

	series, err := reader.QueryRangeMany(context.Background(), AggregateRangeQuery{
		DeviceIDs:   []string{"dev-a", "dev-b"},
		Resolution:  ResolutionDay,
		From:        from,
		To:          to,
		Limit:       14,
		AggregateID: "all",
	})
	if err != nil {
		t.Fatalf("QueryRangeMany failed: %v", err)
	}
	if got := series.DeviceID; got != "all" {
		t.Fatalf("aggregate device id mismatch: got=%s want=all", got)
	}
	if got := len(series.Points); got != 1 {
		t.Fatalf("points mismatch: got=%d want=1", got)
	}
	if series.Points[0].Metrics.LoadEnergyWh == nil || *series.Points[0].Metrics.LoadEnergyWh != 360.0 {
		t.Fatalf("load_energy_wh mismatch: got=%v", series.Points[0].Metrics.LoadEnergyWh)
	}
	if series.Points[0].Metrics.ACOutputAvgW == nil || *series.Points[0].Metrics.ACOutputAvgW != 180.0 {
		t.Fatalf("ac_output_avg_w mismatch: got=%v", series.Points[0].Metrics.ACOutputAvgW)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresReaderQueryRangeManyFiveMinuteAggregatesMinuteTable(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	reader := newPostgresReader(db)
	from := time.Date(2026, time.March, 11, 0, 0, 0, 0, time.UTC)
	to := from.Add(15 * time.Minute)
	rows := sqlmock.NewRows([]string{
		"bucket_start",
		"sample_count",
		"first_ts_unix_ms",
		"last_ts_unix_ms",
		"soc_avg_pct",
		"soc_min_pct",
		"soc_max_pct",
		"ac_in_avg_w",
		"ac_in_max_w",
		"ac_output_avg_w",
		"ac_output_max_w",
		"pv_avg_w",
		"pv_max_w",
		"dc_avg_w",
		"dc_max_w",
		"load_avg_w",
		"load_max_w",
		"net_avg_w",
		"net_min_w",
		"net_max_w",
		"battery_avg_w",
		"battery_min_w",
		"battery_max_w",
		"temp_avg_c",
		"temp_min_c",
		"temp_max_c",
		"solar_generated_wh",
		"ac_input_energy_wh",
		"ac_output_energy_wh",
		"dc_output_energy_wh",
		"load_energy_wh",
		"battery_charge_energy_wh",
		"battery_discharge_energy_wh",
	}).AddRow(
		from,
		500,
		from.UnixMilli(),
		from.Add(4*time.Minute+59*time.Second).UnixMilli(),
		52.0,
		45.0,
		60.0,
		300.0,
		400.0,
		180.0,
		240.0,
		120.0,
		150.0,
		40.0,
		55.0,
		360.0,
		430.0,
		-20.0,
		-30.0,
		10.0,
		80.0,
		-10.0,
		95.0,
		22.5,
		21.0,
		24.0,
		12.0,
		3.5,
		4.0,
		1.2,
		6.5,
		2.0,
		1.4,
	)

	querySQL, _ := buildAggregateQuery(ResolutionFiveMinutes, "telemetry_rollup_minute", []string{"dev-a", "dev-b"}, from, to, 32)
	mock.ExpectQuery(regexp.QuoteMeta(querySQL)).
		WithArgs("dev-a", "dev-b", from, to, 32).
		WillReturnRows(rows)

	series, err := reader.QueryRangeMany(context.Background(), AggregateRangeQuery{
		DeviceIDs:   []string{"dev-a", "dev-b"},
		Resolution:  ResolutionFiveMinutes,
		From:        from,
		To:          to,
		Limit:       32,
		AggregateID: "all",
	})
	if err != nil {
		t.Fatalf("QueryRangeMany failed: %v", err)
	}
	if got := len(series.Points); got != 1 {
		t.Fatalf("points mismatch: got=%d want=1", got)
	}
	if got := series.Points[0].BucketEnd; got != from.Add(5*time.Minute) {
		t.Fatalf("bucket end mismatch: got=%s want=%s", got, from.Add(5*time.Minute))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestPostgresReaderQueryRangeManyRetriesTransientPressure(t *testing.T) {
	t.Setenv("DB_READ_RETRY_MAX_ATTEMPTS", "2")
	t.Setenv("DB_READ_RETRY_INITIAL_BACKOFF", "1ms")
	t.Setenv("DB_READ_RETRY_MAX_BACKOFF", "1ms")
	t.Setenv("DB_READ_RETRY_JITTER_FACTOR", "0")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New failed: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	reader := newPostgresReader(db)
	from := time.Date(2026, time.March, 8, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	querySQL, _ := buildAggregateQuery(ResolutionDay, "telemetry_rollup_day", []string{"dev-a", "dev-b"}, from, to, 14)
	mock.ExpectQuery(regexp.QuoteMeta(querySQL)).
		WithArgs("dev-a", "dev-b", from, to, 14).
		WillReturnError(errors.New("FATAL: remaining connection slots are reserved for roles with the SUPERUSER attribute (SQLSTATE 53300)"))

	rows := sqlmock.NewRows([]string{
		"bucket_start",
		"sample_count",
		"first_ts_unix_ms",
		"last_ts_unix_ms",
		"soc_avg_pct",
		"soc_min_pct",
		"soc_max_pct",
		"ac_in_avg_w",
		"ac_in_max_w",
		"ac_output_avg_w",
		"ac_output_max_w",
		"pv_avg_w",
		"pv_max_w",
		"dc_avg_w",
		"dc_max_w",
		"load_avg_w",
		"load_max_w",
		"net_avg_w",
		"net_min_w",
		"net_max_w",
		"battery_avg_w",
		"battery_min_w",
		"battery_max_w",
		"temp_avg_c",
		"temp_min_c",
		"temp_max_c",
		"solar_generated_wh",
		"ac_input_energy_wh",
		"ac_output_energy_wh",
		"dc_output_energy_wh",
		"load_energy_wh",
		"battery_charge_energy_wh",
		"battery_discharge_energy_wh",
	}).AddRow(
		from,
		42,
		from.UnixMilli(),
		from.Add(23*time.Hour).UnixMilli(),
		50.0,
		48.0,
		52.0,
		30.0,
		40.0,
		25.0,
		35.0,
		120.0,
		150.0,
		5.0,
		8.0,
		80.0,
		100.0,
		-30.0,
		-45.0,
		-10.0,
		20.0,
		-25.0,
		35.0,
		22.0,
		20.0,
		24.0,
		720.0,
		120.0,
		80.0,
		20.0,
		300.0,
		140.0,
		90.0,
	)
	mock.ExpectQuery(regexp.QuoteMeta(querySQL)).
		WithArgs("dev-a", "dev-b", from, to, 14).
		WillReturnRows(rows)

	series, err := reader.QueryRangeMany(context.Background(), AggregateRangeQuery{
		DeviceIDs:   []string{"dev-a", "dev-b"},
		Resolution:  ResolutionDay,
		From:        from,
		To:          to,
		Limit:       14,
		AggregateID: "all",
	})
	if err != nil {
		t.Fatalf("QueryRangeMany failed: %v", err)
	}
	if got := len(series.Points); got != 1 {
		t.Fatalf("points mismatch: got=%d want=1", got)
	}
	if series.Points[0].Metrics.SolarGeneratedWh == nil || *series.Points[0].Metrics.SolarGeneratedWh != 720 {
		t.Fatalf("solar_generated_wh mismatch: got=%v", series.Points[0].Metrics.SolarGeneratedWh)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestEnrichSolarEnergyLeavesMissingMinuteEnergyBucketsUnset(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, time.March, 6, 8, 7, 0, 0, time.UTC)
	pv := 72.0
	series := enrichSolarEnergy(Series{
		DeviceID:   "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		Resolution: ResolutionMinute,
		From:       from,
		To:         from.Add(4 * time.Minute),
		Points: []Point{
			{
				BucketStart: from,
				BucketEnd:   from.Add(time.Minute),
				SampleCount: 5,
				Metrics: Metrics{
					PVAvgW: &pv,
				},
			},
		},
	})

	if got := len(series.Points); got != 1 {
		t.Fatalf("expected only stored minute points, got=%d", got)
	}
	if series.Points[0].Metrics.SolarGeneratedWh != nil {
		t.Fatalf("expected missing solar_generated_wh to remain unset, got=%v", *series.Points[0].Metrics.SolarGeneratedWh)
	}
	if series.EnergyBucketCoverage.PersistedValueCount != 0 {
		t.Fatalf("coverage persisted values mismatch: got=%d want=0", series.EnergyBucketCoverage.PersistedValueCount)
	}
	if series.EnergyBucketCoverage.DerivedValueCount != 0 {
		t.Fatalf("coverage derived values mismatch: got=%d want=0", series.EnergyBucketCoverage.DerivedValueCount)
	}
}

func TestEnrichSolarEnergyLeavesZeroPvWithoutSyntheticWh(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, time.March, 6, 8, 7, 0, 0, time.UTC)
	zero := 0.0
	series := enrichSolarEnergy(Series{
		DeviceID:   "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		Resolution: ResolutionMinute,
		From:       from,
		To:         from.Add(3 * time.Minute),
		Points: []Point{
			{
				BucketStart: from,
				BucketEnd:   from.Add(time.Minute),
				SampleCount: 1,
				Metrics: Metrics{
					PVAvgW: &zero,
				},
			},
		},
	})

	if got := len(series.Points); got != 1 {
		t.Fatalf("expected no synthetic zero-pv points, got=%d", got)
	}
	if series.Points[0].Metrics.SolarGeneratedWh != nil {
		t.Fatalf("expected no derived solar_generated_wh for zero pv, got=%v", *series.Points[0].Metrics.SolarGeneratedWh)
	}
}

func TestEnrichSolarEnergyReportsPersistedCoverageWithoutDerivingBuckets(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, time.March, 6, 8, 0, 0, 0, time.UTC)
	acInWh := 120.0
	acOutWh := 150.0
	loadWh := 180.0
	batteryChargeWh := 45.0
	batteryDischargeWh := 20.0
	series := enrichSolarEnergy(Series{
		DeviceID:   "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		Resolution: ResolutionHour,
		From:       from,
		To:         from.Add(2 * time.Hour),
		Points: []Point{
			{
				BucketStart: from,
				BucketEnd:   from.Add(time.Hour),
				SampleCount: 4,
				Metrics: Metrics{
					ACInputEnergyWh:       &acInWh,
					ACOutputEnergyWh:      &acOutWh,
					LoadEnergyWh:          &loadWh,
					BatteryChargeEnergyWh: &batteryChargeWh,
				},
			},
			{
				BucketStart: from.Add(time.Hour),
				BucketEnd:   from.Add(2 * time.Hour),
				SampleCount: 4,
				Metrics: Metrics{
					BatteryDischargeEnergyWh: &batteryDischargeWh,
				},
			},
		},
	})

	first := series.Points[0].Metrics
	if first.ACInputEnergyWh == nil || *first.ACInputEnergyWh != 120 {
		t.Fatalf("ac_input_energy_wh mismatch: got=%v want=120", first.ACInputEnergyWh)
	}
	if first.ACOutputEnergyWh == nil || *first.ACOutputEnergyWh != 150 {
		t.Fatalf("ac_output_energy_wh mismatch: got=%v want=150", first.ACOutputEnergyWh)
	}
	if first.LoadEnergyWh == nil || *first.LoadEnergyWh != 180 {
		t.Fatalf("load_energy_wh mismatch: got=%v want=180", first.LoadEnergyWh)
	}
	if first.BatteryChargeEnergyWh == nil || *first.BatteryChargeEnergyWh != 45 {
		t.Fatalf("battery_charge_energy_wh mismatch: got=%v want=45", first.BatteryChargeEnergyWh)
	}

	second := series.Points[1].Metrics
	if second.BatteryDischargeEnergyWh == nil || *second.BatteryDischargeEnergyWh != 20 {
		t.Fatalf("battery_discharge_energy_wh mismatch: got=%v want=20", second.BatteryDischargeEnergyWh)
	}
	if series.EnergyBucketCoverage.PointCount != 2 {
		t.Fatalf("coverage point count mismatch: got=%d want=2", series.EnergyBucketCoverage.PointCount)
	}
	if series.EnergyBucketCoverage.PersistedValueCount != 5 {
		t.Fatalf("coverage persisted values mismatch: got=%d want=5", series.EnergyBucketCoverage.PersistedValueCount)
	}
	if series.EnergyBucketCoverage.DerivedValueCount != 0 {
		t.Fatalf("coverage derived values mismatch: got=%d want=0", series.EnergyBucketCoverage.DerivedValueCount)
	}
}

func TestEnrichSolarEnergyLeavesSparseHourlyFallbackUnset(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, time.March, 14, 13, 0, 0, 0, time.UTC)
	pv := 300.0
	firstTs := from.Add(10 * time.Minute)
	lastTs := from.Add(30 * time.Minute)
	series := enrichSolarEnergy(Series{
		DeviceID:   "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		Resolution: ResolutionHour,
		From:       from,
		To:         from.Add(time.Hour),
		Points: []Point{
			{
				BucketStart:   from,
				BucketEnd:     from.Add(time.Hour),
				SampleCount:   3,
				FirstTsUnixMs: firstTs.UnixMilli(),
				LastTsUnixMs:  lastTs.UnixMilli(),
				Metrics: Metrics{
					PVAvgW: &pv,
				},
			},
		},
	})

	if got := len(series.Points); got != 1 {
		t.Fatalf("expected one point, got=%d", got)
	}
	if series.Points[0].Metrics.SolarGeneratedWh != nil {
		t.Fatalf("expected sparse hourly solar_generated_wh to remain unset, got=%v", *series.Points[0].Metrics.SolarGeneratedWh)
	}
	if series.EnergyBucketCoverage.DerivedValueCount != 0 {
		t.Fatalf("coverage derived values mismatch: got=%d want=0", series.EnergyBucketCoverage.DerivedValueCount)
	}
}
