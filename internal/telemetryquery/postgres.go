package telemetryquery

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
)

type PostgresReader struct {
	db *sql.DB
}

func NewPostgresReader(dsn string) (*PostgresReader, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("telemetry query postgres dsn is required")
	}
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, fmt.Errorf("apply telemetry query postgres search_path: %w", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open telemetry query postgres connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping telemetry query postgres: %w", err)
	}
	return newPostgresReader(db), nil
}

func newPostgresReader(db *sql.DB) *PostgresReader {
	return &PostgresReader{db: db}
}

func (r *PostgresReader) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *PostgresReader) QueryRange(ctx context.Context, query RangeQuery) (Series, error) {
	table, err := query.Resolution.TableName()
	if err != nil {
		return Series{}, err
	}
	if strings.TrimSpace(query.DeviceID) == "" {
		return Series{}, ErrInvalidRange
	}
	from := query.From.UTC()
	to := query.To.UTC()
	if !from.Before(to) {
		return Series{}, ErrInvalidRange
	}
	if query.Limit <= 0 {
		return Series{}, ErrInvalidRange
	}

	rows, err := r.db.QueryContext(ctx, buildQuery(table), query.DeviceID, from, to, query.Limit)
	if err != nil {
		return Series{}, fmt.Errorf("query telemetry rollup range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	points := make([]Point, 0, 64)
	bucketWidth := query.Resolution.BucketDuration()
	for rows.Next() {
		var (
			point Point

			socAvgPct        sql.NullFloat64
			socMinPct        sql.NullFloat64
			socMaxPct        sql.NullFloat64
			acInAvgW         sql.NullFloat64
			acInMaxW         sql.NullFloat64
			pvAvgW           sql.NullFloat64
			pvMaxW           sql.NullFloat64
			dcAvgW           sql.NullFloat64
			dcMaxW           sql.NullFloat64
			loadAvgW         sql.NullFloat64
			loadMaxW         sql.NullFloat64
			netAvgW          sql.NullFloat64
			netMinW          sql.NullFloat64
			netMaxW          sql.NullFloat64
			batteryAvgW      sql.NullFloat64
			batteryMinW      sql.NullFloat64
			batteryMaxW      sql.NullFloat64
			tempAvgC         sql.NullFloat64
			tempMinC         sql.NullFloat64
			tempMaxC         sql.NullFloat64
			solarGeneratedWh sql.NullFloat64
		)

		if err := rows.Scan(
			&point.BucketStart,
			&point.SampleCount,
			&point.FirstTsUnixMs,
			&point.LastTsUnixMs,
			&socAvgPct,
			&socMinPct,
			&socMaxPct,
			&acInAvgW,
			&acInMaxW,
			&pvAvgW,
			&pvMaxW,
			&dcAvgW,
			&dcMaxW,
			&loadAvgW,
			&loadMaxW,
			&netAvgW,
			&netMinW,
			&netMaxW,
			&batteryAvgW,
			&batteryMinW,
			&batteryMaxW,
			&tempAvgC,
			&tempMinC,
			&tempMaxC,
			&solarGeneratedWh,
		); err != nil {
			return Series{}, fmt.Errorf("scan telemetry rollup row: %w", err)
		}

		point.BucketStart = point.BucketStart.UTC()
		point.BucketEnd = point.BucketStart.Add(bucketWidth)
		point.Metrics = Metrics{
			SOCAvgPct:        nullableFloat64(socAvgPct),
			SOCMinPct:        nullableFloat64(socMinPct),
			SOCMaxPct:        nullableFloat64(socMaxPct),
			ACInAvgW:         nullableFloat64(acInAvgW),
			ACInMaxW:         nullableFloat64(acInMaxW),
			PVAvgW:           nullableFloat64(pvAvgW),
			PVMaxW:           nullableFloat64(pvMaxW),
			DCAvgW:           nullableFloat64(dcAvgW),
			DCMaxW:           nullableFloat64(dcMaxW),
			LoadAvgW:         nullableFloat64(loadAvgW),
			LoadMaxW:         nullableFloat64(loadMaxW),
			NetAvgW:          nullableFloat64(netAvgW),
			NetMinW:          nullableFloat64(netMinW),
			NetMaxW:          nullableFloat64(netMaxW),
			BatteryAvgW:      nullableFloat64(batteryAvgW),
			BatteryMinW:      nullableFloat64(batteryMinW),
			BatteryMaxW:      nullableFloat64(batteryMaxW),
			TempAvgC:         nullableFloat64(tempAvgC),
			TempMinC:         nullableFloat64(tempMinC),
			TempMaxC:         nullableFloat64(tempMaxC),
			SolarGeneratedWh: nullableFloat64(solarGeneratedWh),
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return Series{}, fmt.Errorf("iterate telemetry rollup rows: %w", err)
	}

	series := Series{
		DeviceID:   query.DeviceID,
		Resolution: query.Resolution,
		From:       from,
		To:         to,
		Points:     points,
	}
	return enrichSolarEnergy(series), nil
}

func buildQuery(table string) string {
	return fmt.Sprintf(`SELECT
	bucket_start,
	sample_count,
	first_ts_unix_ms,
	last_ts_unix_ms,
	soc_avg_pct,
	soc_min_pct,
	soc_max_pct,
	ac_in_avg_w,
	ac_in_max_w,
	pv_avg_w,
	pv_max_w,
	dc_avg_w,
	dc_max_w,
	load_avg_w,
	load_max_w,
	net_avg_w,
	net_min_w,
	net_max_w,
	battery_avg_w,
	battery_min_w,
	battery_max_w,
	temp_avg_c,
	temp_min_c,
	temp_max_c,
	solar_generated_wh
FROM %s
WHERE device_id = $1::uuid
  AND bucket_start >= $2
  AND bucket_start < $3
ORDER BY bucket_start ASC
LIMIT $4;`, table)
}

func nullableFloat64(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}

func enrichSolarEnergy(series Series) Series {
	if len(series.Points) == 0 {
		return series
	}

	points := make([]Point, 0, len(series.Points))
	for _, point := range series.Points {
		points = append(points, withDerivedSolarEnergy(point, series.Resolution))
	}
	series.Points = points
	return series
}

func withDerivedSolarEnergy(point Point, resolution Resolution) Point {
	if point.Metrics.SolarGeneratedWh != nil {
		return point
	}
	pvAvgW, ok := positiveMetricValue(point.Metrics.PVAvgW)
	if !ok {
		return point
	}
	durationHours := point.BucketEnd.Sub(point.BucketStart).Hours()
	if resolution == ResolutionMinute && durationHours <= 0 {
		durationHours = time.Minute.Hours()
	}
	if durationHours <= 0 {
		return point
	}
	solarWh := pvAvgW * durationHours
	point.Metrics.SolarGeneratedWh = floatPtr(solarWh)
	return point
}

func positiveMetricValue(value *float64) (float64, bool) {
	if value == nil || *value <= 0 {
		return 0, false
	}
	return *value, true
}

func floatPtr(value float64) *float64 {
	v := value
	return &v
}
