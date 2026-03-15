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

	rows, err := r.db.QueryContext(ctx, buildQuery(query.Resolution, table), query.DeviceID, from, to, query.Limit)
	if err != nil {
		return Series{}, fmt.Errorf("query telemetry rollup range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSeriesRows(rows, query.DeviceID, query.Resolution, from, to)
}

func (r *PostgresReader) QueryRangeMany(ctx context.Context, query AggregateRangeQuery) (Series, error) {
	table, err := query.Resolution.TableName()
	if err != nil {
		return Series{}, err
	}
	deviceIDs := normalizeAggregateDeviceIDs(query.DeviceIDs)
	if len(deviceIDs) == 0 {
		return Series{}, ErrInvalidRange
	}
	from := query.From.UTC()
	to := query.To.UTC()
	if !from.Before(to) || query.Limit <= 0 {
		return Series{}, ErrInvalidRange
	}

	sqlQuery, args := buildAggregateQuery(query.Resolution, table, deviceIDs, from, to, query.Limit)
	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return Series{}, fmt.Errorf("query aggregated telemetry rollup range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	aggregateID := strings.TrimSpace(query.AggregateID)
	if aggregateID == "" {
		aggregateID = "all"
	}
	return scanSeriesRows(rows, aggregateID, query.Resolution, from, to)
}

func (r *PostgresReader) QueryPVPortHistory(ctx context.Context, query PVPortHistoryQuery) ([]PVPortHistory, error) {
	table, err := pvPortTableName(query.Resolution)
	if err != nil {
		return nil, err
	}
	deviceIDs := normalizeAggregateDeviceIDs(query.DeviceIDs)
	if len(deviceIDs) == 0 {
		return nil, ErrInvalidRange
	}
	from := query.From.UTC()
	to := query.To.UTC()
	if !from.Before(to) {
		return nil, ErrInvalidRange
	}
	sqlQuery, args := buildPVPortHistoryQuery(table, deviceIDs, from, to)
	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query pv-port history rollups: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]PVPortHistory, 0, 16)
	for rows.Next() {
		var (
			row                  PVPortHistory
			lastObservedAtUnixMS sql.NullInt64
		)
		if err := rows.Scan(
			&row.DeviceID,
			&row.PortID,
			&row.PortLabel,
			&row.MaxObservedVolts,
			&row.MaxObservedAmps,
			&row.MaxObservedWatts,
			&row.LastObservedVolts,
			&row.LastObservedAmps,
			&row.LastObservedWatts,
			&lastObservedAtUnixMS,
			&row.SampleCount,
		); err != nil {
			return nil, fmt.Errorf("scan pv-port history row: %w", err)
		}
		if lastObservedAtUnixMS.Valid && lastObservedAtUnixMS.Int64 > 0 {
			row.LastObservedAt = time.UnixMilli(lastObservedAtUnixMS.Int64).UTC()
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pv-port history rows: %w", err)
	}
	return out, nil
}

func scanSeriesRows(rows *sql.Rows, deviceID string, resolution Resolution, from, to time.Time) (Series, error) {
	points := make([]Point, 0, 64)
	bucketWidth := resolution.BucketDuration()
	for rows.Next() {
		var (
			point Point

			socAvgPct          sql.NullFloat64
			socMinPct          sql.NullFloat64
			socMaxPct          sql.NullFloat64
			acInAvgW           sql.NullFloat64
			acInMaxW           sql.NullFloat64
			acOutputAvgW       sql.NullFloat64
			acOutputMaxW       sql.NullFloat64
			pvAvgW             sql.NullFloat64
			pvMaxW             sql.NullFloat64
			dcAvgW             sql.NullFloat64
			dcMaxW             sql.NullFloat64
			loadAvgW           sql.NullFloat64
			loadMaxW           sql.NullFloat64
			netAvgW            sql.NullFloat64
			netMinW            sql.NullFloat64
			netMaxW            sql.NullFloat64
			batteryAvgW        sql.NullFloat64
			batteryMinW        sql.NullFloat64
			batteryMaxW        sql.NullFloat64
			tempAvgC           sql.NullFloat64
			tempMinC           sql.NullFloat64
			tempMaxC           sql.NullFloat64
			solarGeneratedWh   sql.NullFloat64
			acInputEnergyWh    sql.NullFloat64
			acOutputEnergyWh   sql.NullFloat64
			dcOutputEnergyWh   sql.NullFloat64
			loadEnergyWh       sql.NullFloat64
			batteryChargeWh    sql.NullFloat64
			batteryDischargeWh sql.NullFloat64
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
			&acOutputAvgW,
			&acOutputMaxW,
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
			&acInputEnergyWh,
			&acOutputEnergyWh,
			&dcOutputEnergyWh,
			&loadEnergyWh,
			&batteryChargeWh,
			&batteryDischargeWh,
		); err != nil {
			return Series{}, fmt.Errorf("scan telemetry rollup row: %w", err)
		}

		point.BucketStart = point.BucketStart.UTC()
		point.BucketEnd = point.BucketStart.Add(bucketWidth)
		point.Metrics = Metrics{
			SOCAvgPct:                nullableFloat64(socAvgPct),
			SOCMinPct:                nullableFloat64(socMinPct),
			SOCMaxPct:                nullableFloat64(socMaxPct),
			ACInAvgW:                 nullableFloat64(acInAvgW),
			ACInMaxW:                 nullableFloat64(acInMaxW),
			ACOutputAvgW:             nullableFloat64(acOutputAvgW),
			ACOutputMaxW:             nullableFloat64(acOutputMaxW),
			PVAvgW:                   nullableFloat64(pvAvgW),
			PVMaxW:                   nullableFloat64(pvMaxW),
			DCAvgW:                   nullableFloat64(dcAvgW),
			DCMaxW:                   nullableFloat64(dcMaxW),
			LoadAvgW:                 nullableFloat64(loadAvgW),
			LoadMaxW:                 nullableFloat64(loadMaxW),
			NetAvgW:                  nullableFloat64(netAvgW),
			NetMinW:                  nullableFloat64(netMinW),
			NetMaxW:                  nullableFloat64(netMaxW),
			BatteryAvgW:              nullableFloat64(batteryAvgW),
			BatteryMinW:              nullableFloat64(batteryMinW),
			BatteryMaxW:              nullableFloat64(batteryMaxW),
			TempAvgC:                 nullableFloat64(tempAvgC),
			TempMinC:                 nullableFloat64(tempMinC),
			TempMaxC:                 nullableFloat64(tempMaxC),
			SolarGeneratedWh:         nullableFloat64(solarGeneratedWh),
			ACInputEnergyWh:          nullableFloat64(acInputEnergyWh),
			ACOutputEnergyWh:         nullableFloat64(acOutputEnergyWh),
			DCOutputEnergyWh:         nullableFloat64(dcOutputEnergyWh),
			LoadEnergyWh:             nullableFloat64(loadEnergyWh),
			BatteryChargeEnergyWh:    nullableFloat64(batteryChargeWh),
			BatteryDischargeEnergyWh: nullableFloat64(batteryDischargeWh),
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return Series{}, fmt.Errorf("iterate telemetry rollup rows: %w", err)
	}

	series := Series{
		DeviceID:   deviceID,
		Resolution: resolution,
		From:       from,
		To:         to,
		Points:     points,
	}
	return enrichSolarEnergy(series), nil
}

func buildQuery(resolution Resolution, table string) string {
	if resolution == ResolutionFiveMinutes {
		return fmt.Sprintf(`WITH grouped AS (
	SELECT
		date_bin('5 minutes', bucket_start, TIMESTAMPTZ '2001-01-01 00:00:00+00') AS bucket_start,
		SUM(sample_count) AS sample_count,
		MIN(first_ts_unix_ms) AS first_ts_unix_ms,
		MAX(last_ts_unix_ms) AS last_ts_unix_ms,
		AVG(soc_avg_pct) AS soc_avg_pct,
		MIN(soc_min_pct) AS soc_min_pct,
		MAX(soc_max_pct) AS soc_max_pct,
		AVG(ac_in_avg_w) AS ac_in_avg_w,
		MAX(ac_in_max_w) AS ac_in_max_w,
		AVG(ac_output_avg_w) AS ac_output_avg_w,
		MAX(ac_output_max_w) AS ac_output_max_w,
		AVG(pv_avg_w) AS pv_avg_w,
		MAX(pv_max_w) AS pv_max_w,
		AVG(dc_avg_w) AS dc_avg_w,
		MAX(dc_max_w) AS dc_max_w,
		AVG(load_avg_w) AS load_avg_w,
		MAX(load_max_w) AS load_max_w,
		AVG(net_avg_w) AS net_avg_w,
		MIN(net_min_w) AS net_min_w,
		MAX(net_max_w) AS net_max_w,
		AVG(battery_avg_w) AS battery_avg_w,
		MIN(battery_min_w) AS battery_min_w,
		MAX(battery_max_w) AS battery_max_w,
		AVG(temp_avg_c) AS temp_avg_c,
		MIN(temp_min_c) AS temp_min_c,
		MAX(temp_max_c) AS temp_max_c,
		SUM(solar_generated_wh) AS solar_generated_wh,
		SUM(ac_input_energy_wh) AS ac_input_energy_wh,
		SUM(ac_output_energy_wh) AS ac_output_energy_wh,
		SUM(dc_output_energy_wh) AS dc_output_energy_wh,
		SUM(load_energy_wh) AS load_energy_wh,
		SUM(battery_charge_energy_wh) AS battery_charge_energy_wh,
		SUM(battery_discharge_energy_wh) AS battery_discharge_energy_wh
	FROM %s
	WHERE device_id = $1::uuid
	  AND bucket_start >= $2
	  AND bucket_start < $3
	GROUP BY 1
)
SELECT
	bucket_start,
	sample_count,
	first_ts_unix_ms,
	last_ts_unix_ms,
	soc_avg_pct,
	soc_min_pct,
	soc_max_pct,
	ac_in_avg_w,
	ac_in_max_w,
	ac_output_avg_w,
	ac_output_max_w,
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
	solar_generated_wh,
	ac_input_energy_wh,
	ac_output_energy_wh,
	dc_output_energy_wh,
	load_energy_wh,
	battery_charge_energy_wh,
	battery_discharge_energy_wh
FROM grouped
ORDER BY bucket_start ASC
LIMIT $4;`, table)
	}
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
	ac_output_avg_w,
	ac_output_max_w,
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
	solar_generated_wh,
	ac_input_energy_wh,
	ac_output_energy_wh,
	dc_output_energy_wh,
	load_energy_wh,
	battery_charge_energy_wh,
	battery_discharge_energy_wh
FROM %s
WHERE device_id = $1::uuid
  AND bucket_start >= $2
  AND bucket_start < $3
ORDER BY bucket_start ASC
LIMIT $4;`, table)
}

func pvPortTableName(resolution Resolution) (string, error) {
	switch resolution {
	case ResolutionMinute:
		return "telemetry_rollup_pv_port_minute", nil
	case ResolutionHour:
		return "telemetry_rollup_pv_port_hour", nil
	case ResolutionDay:
		return "telemetry_rollup_pv_port_day", nil
	default:
		return "", ErrInvalidResolution
	}
}

func buildPVPortHistoryQuery(table string, deviceIDs []string, from, to time.Time) (string, []any) {
	placeholders := make([]string, 0, len(deviceIDs))
	args := make([]any, 0, len(deviceIDs)+2)
	for idx, deviceID := range deviceIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d::uuid", idx+1))
		args = append(args, deviceID)
	}
	fromIdx := len(args) + 1
	toIdx := len(args) + 2
	args = append(args, from, to)
	sqlQuery := fmt.Sprintf(`WITH windowed AS (
	SELECT
		device_id::text AS device_id,
		port_id,
		port_label,
		max_observed_volts,
		max_observed_amps,
		max_observed_watts,
		last_observed_volts,
		last_observed_amps,
		last_observed_watts,
		last_observed_at_unix_ms,
		sample_count,
		bucket_start
	FROM %s
	WHERE device_id IN (%s)
	  AND bucket_start >= $%d
	  AND bucket_start < $%d
), aggregate_rows AS (
	SELECT
		device_id,
		port_id,
		MAX(max_observed_volts) AS max_observed_volts,
		MAX(max_observed_amps) AS max_observed_amps,
		MAX(max_observed_watts) AS max_observed_watts,
		SUM(sample_count) AS sample_count
	FROM windowed
	GROUP BY device_id, port_id
), latest_rows AS (
	SELECT DISTINCT ON (device_id, port_id)
		device_id,
		port_id,
		port_label,
		last_observed_volts,
		last_observed_amps,
		last_observed_watts,
		last_observed_at_unix_ms
	FROM windowed
	ORDER BY device_id, port_id, last_observed_at_unix_ms DESC NULLS LAST, bucket_start DESC
)
SELECT
	aggregate_rows.device_id,
	aggregate_rows.port_id,
	latest_rows.port_label,
	aggregate_rows.max_observed_volts,
	aggregate_rows.max_observed_amps,
	aggregate_rows.max_observed_watts,
	latest_rows.last_observed_volts,
	latest_rows.last_observed_amps,
	latest_rows.last_observed_watts,
	latest_rows.last_observed_at_unix_ms,
	aggregate_rows.sample_count
FROM aggregate_rows
JOIN latest_rows USING (device_id, port_id)
ORDER BY aggregate_rows.device_id, aggregate_rows.port_id`, table, strings.Join(placeholders, ", "), fromIdx, toIdx)
	return sqlQuery, args
}

func buildAggregateQuery(resolution Resolution, table string, deviceIDs []string, from, to time.Time, limit int) (string, []any) {
	placeholders := make([]string, 0, len(deviceIDs))
	args := make([]any, 0, len(deviceIDs)+3)
	for idx, deviceID := range deviceIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d::uuid", idx+1))
		args = append(args, deviceID)
	}
	fromIdx := len(args) + 1
	toIdx := len(args) + 2
	limitIdx := len(args) + 3
	args = append(args, from, to, limit)
	if resolution == ResolutionFiveMinutes {
		return fmt.Sprintf(`WITH device_grouped AS (
	SELECT
		device_id,
		date_bin('5 minutes', bucket_start, TIMESTAMPTZ '2001-01-01 00:00:00+00') AS bucket_start,
		SUM(sample_count) AS sample_count,
		MIN(first_ts_unix_ms) AS first_ts_unix_ms,
		MAX(last_ts_unix_ms) AS last_ts_unix_ms,
		AVG(soc_avg_pct) AS soc_avg_pct,
		MIN(soc_min_pct) AS soc_min_pct,
		MAX(soc_max_pct) AS soc_max_pct,
		AVG(ac_in_avg_w) AS ac_in_avg_w,
		MAX(ac_in_max_w) AS ac_in_max_w,
		AVG(ac_output_avg_w) AS ac_output_avg_w,
		MAX(ac_output_max_w) AS ac_output_max_w,
		AVG(pv_avg_w) AS pv_avg_w,
		MAX(pv_max_w) AS pv_max_w,
		AVG(dc_avg_w) AS dc_avg_w,
		MAX(dc_max_w) AS dc_max_w,
		AVG(load_avg_w) AS load_avg_w,
		MAX(load_max_w) AS load_max_w,
		AVG(net_avg_w) AS net_avg_w,
		MIN(net_min_w) AS net_min_w,
		MAX(net_max_w) AS net_max_w,
		AVG(battery_avg_w) AS battery_avg_w,
		MIN(battery_min_w) AS battery_min_w,
		MAX(battery_max_w) AS battery_max_w,
		AVG(temp_avg_c) AS temp_avg_c,
		MIN(temp_min_c) AS temp_min_c,
		MAX(temp_max_c) AS temp_max_c,
		SUM(solar_generated_wh) AS solar_generated_wh,
		SUM(ac_input_energy_wh) AS ac_input_energy_wh,
		SUM(ac_output_energy_wh) AS ac_output_energy_wh,
		SUM(dc_output_energy_wh) AS dc_output_energy_wh,
		SUM(load_energy_wh) AS load_energy_wh,
		SUM(battery_charge_energy_wh) AS battery_charge_energy_wh,
		SUM(battery_discharge_energy_wh) AS battery_discharge_energy_wh
	FROM %s
	WHERE device_id IN (%s)
	  AND bucket_start >= $%d
	  AND bucket_start < $%d
	GROUP BY device_id, 2
)
SELECT
	bucket_start,
	SUM(sample_count) AS sample_count,
	MIN(first_ts_unix_ms) AS first_ts_unix_ms,
	MAX(last_ts_unix_ms) AS last_ts_unix_ms,
	AVG(soc_avg_pct) AS soc_avg_pct,
	MIN(soc_min_pct) AS soc_min_pct,
	MAX(soc_max_pct) AS soc_max_pct,
	SUM(ac_in_avg_w) AS ac_in_avg_w,
	SUM(ac_in_max_w) AS ac_in_max_w,
	SUM(ac_output_avg_w) AS ac_output_avg_w,
	SUM(ac_output_max_w) AS ac_output_max_w,
	SUM(pv_avg_w) AS pv_avg_w,
	SUM(pv_max_w) AS pv_max_w,
	SUM(dc_avg_w) AS dc_avg_w,
	SUM(dc_max_w) AS dc_max_w,
	SUM(load_avg_w) AS load_avg_w,
	SUM(load_max_w) AS load_max_w,
	SUM(net_avg_w) AS net_avg_w,
	SUM(net_min_w) AS net_min_w,
	SUM(net_max_w) AS net_max_w,
	SUM(battery_avg_w) AS battery_avg_w,
	SUM(battery_min_w) AS battery_min_w,
	SUM(battery_max_w) AS battery_max_w,
	AVG(temp_avg_c) AS temp_avg_c,
	MIN(temp_min_c) AS temp_min_c,
	MAX(temp_max_c) AS temp_max_c,
	SUM(solar_generated_wh) AS solar_generated_wh,
	SUM(ac_input_energy_wh) AS ac_input_energy_wh,
	SUM(ac_output_energy_wh) AS ac_output_energy_wh,
	SUM(dc_output_energy_wh) AS dc_output_energy_wh,
	SUM(load_energy_wh) AS load_energy_wh,
	SUM(battery_charge_energy_wh) AS battery_charge_energy_wh,
	SUM(battery_discharge_energy_wh) AS battery_discharge_energy_wh
FROM device_grouped
GROUP BY bucket_start
ORDER BY bucket_start ASC
LIMIT $%d;`, table, strings.Join(placeholders, ", "), fromIdx, toIdx, limitIdx), args
	}
	return fmt.Sprintf(`SELECT
	bucket_start,
	SUM(sample_count) AS sample_count,
	MIN(first_ts_unix_ms) AS first_ts_unix_ms,
	MAX(last_ts_unix_ms) AS last_ts_unix_ms,
	AVG(soc_avg_pct) AS soc_avg_pct,
	MIN(soc_min_pct) AS soc_min_pct,
	MAX(soc_max_pct) AS soc_max_pct,
	SUM(ac_in_avg_w) AS ac_in_avg_w,
	SUM(ac_in_max_w) AS ac_in_max_w,
	SUM(ac_output_avg_w) AS ac_output_avg_w,
	SUM(ac_output_max_w) AS ac_output_max_w,
	SUM(pv_avg_w) AS pv_avg_w,
	SUM(pv_max_w) AS pv_max_w,
	SUM(dc_avg_w) AS dc_avg_w,
	SUM(dc_max_w) AS dc_max_w,
	SUM(load_avg_w) AS load_avg_w,
	SUM(load_max_w) AS load_max_w,
	SUM(net_avg_w) AS net_avg_w,
	SUM(net_min_w) AS net_min_w,
	SUM(net_max_w) AS net_max_w,
	SUM(battery_avg_w) AS battery_avg_w,
	SUM(battery_min_w) AS battery_min_w,
	SUM(battery_max_w) AS battery_max_w,
	AVG(temp_avg_c) AS temp_avg_c,
	MIN(temp_min_c) AS temp_min_c,
	MAX(temp_max_c) AS temp_max_c,
	SUM(solar_generated_wh) AS solar_generated_wh,
	SUM(ac_input_energy_wh) AS ac_input_energy_wh,
	SUM(ac_output_energy_wh) AS ac_output_energy_wh,
	SUM(dc_output_energy_wh) AS dc_output_energy_wh,
	SUM(load_energy_wh) AS load_energy_wh,
	SUM(battery_charge_energy_wh) AS battery_charge_energy_wh,
	SUM(battery_discharge_energy_wh) AS battery_discharge_energy_wh
FROM %s
WHERE device_id IN (%s)
  AND bucket_start >= $%d
  AND bucket_start < $%d
GROUP BY bucket_start
ORDER BY bucket_start ASC
LIMIT $%d;`, table, strings.Join(placeholders, ", "), fromIdx, toIdx, limitIdx), args
}

func normalizeAggregateDeviceIDs(deviceIDs []string) []string {
	out := make([]string, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		trimmed := strings.TrimSpace(deviceID)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
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

	coverage := EnergyBucketCoverage{PointCount: len(series.Points)}
	for _, point := range series.Points {
		coverage.PersistedValueCount += storedEnergyValueCount(point.Metrics)
	}
	series.EnergyBucketCoverage = coverage
	return series
}

func storedEnergyValueCount(metrics Metrics) int {
	count := 0
	for _, value := range []*float64{
		metrics.SolarGeneratedWh,
		metrics.ACInputEnergyWh,
		metrics.ACOutputEnergyWh,
		metrics.DCOutputEnergyWh,
		metrics.LoadEnergyWh,
		metrics.BatteryChargeEnergyWh,
		metrics.BatteryDischargeEnergyWh,
	} {
		if value != nil {
			count++
		}
	}
	return count
}
