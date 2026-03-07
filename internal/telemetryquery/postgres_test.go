package telemetryquery

import (
	"context"
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
	)

	mock.ExpectQuery(regexp.QuoteMeta(buildQuery("telemetry_rollup_hour"))).
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

func TestEnrichSolarEnergyDerivesExplicitWhWithoutFillingMinuteGaps(t *testing.T) {
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
	if series.Points[0].Metrics.SolarGeneratedWh == nil {
		t.Fatalf("expected stored point to derive solar_generated_wh")
	}
	if *series.Points[0].Metrics.SolarGeneratedWh != 1.2 {
		t.Fatalf("solar_generated_wh mismatch: got=%v want=1.2", *series.Points[0].Metrics.SolarGeneratedWh)
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
