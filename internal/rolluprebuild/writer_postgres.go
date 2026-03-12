package rolluprebuild

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
)

const defaultReplaceChunkSize = 500

type PostgresWriter struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

type DeviceWindow struct {
	Provider         string
	ProviderDeviceID string
}

func NewPostgresWriter(dsn string) (*PostgresWriter, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("rollup rebuild postgres dsn is required")
	}
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, fmt.Errorf("apply rollup rebuild postgres search_path: %w", err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse rollup rebuild postgres dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("open rollup rebuild postgres pool: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping rollup rebuild postgres: %w", err)
	}
	return &PostgresWriter{pool: pool, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (w *PostgresWriter) Close() error {
	if w == nil || w.pool == nil {
		return nil
	}
	w.pool.Close()
	return nil
}

func (w *PostgresWriter) ReplaceRows(ctx context.Context, resolution Resolution, rows []BucketRow, affected []DeviceWindow, from, to time.Time, chunkSize int) (int, error) {
	if w == nil || w.pool == nil {
		return 0, fmt.Errorf("rollup rebuild postgres writer is not initialized")
	}
	if len(rows) == 0 && len(affected) == 0 {
		return 0, nil
	}
	if chunkSize <= 0 {
		chunkSize = defaultReplaceChunkSize
	}
	table, err := tableNameForResolution(resolution)
	if err != nil {
		return 0, err
	}
	rows = dedupeRows(rows)
	rowGroups := groupRowsByDevice(rows)
	affectedGroups := normalizeAffectedDevices(affected)
	total := len(rows)
	windowStart, windowEnd := replacementWindowBounds(resolution, from, to)
	for start := 0; start < len(affectedGroups); start += chunkSize {
		end := start + chunkSize
		if end > len(affectedGroups) {
			end = len(affectedGroups)
		}
		chunkDevices := affectedGroups[start:end]
		chunkRows := make([]BucketRow, 0)
		for _, device := range chunkDevices {
			key := device.Provider + "|" + device.ProviderDeviceID
			chunkRows = append(chunkRows, rowGroups[key]...)
		}
		if err := w.replaceChunk(ctx, table, chunkRows, chunkDevices, windowStart, windowEnd); err != nil {
			return total, err
		}
	}
	return total, nil
}

func (w *PostgresWriter) replaceChunk(ctx context.Context, table string, rows []BucketRow, affected []DeviceWindow, from, to time.Time) error {
	tx, err := w.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin rollup rebuild transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tempTable := "tmp_rollup_rebuild"
	createSQL := fmt.Sprintf(`CREATE TEMP TABLE %s (LIKE %s INCLUDING DEFAULTS INCLUDING CONSTRAINTS) ON COMMIT DROP`, tempTable, table)
	if _, err := tx.Exec(ctx, createSQL); err != nil {
		return fmt.Errorf("create temp rollup rebuild table: %w", err)
	}

	copyRows := make([][]any, 0, len(rows))
	now := w.now()
	for _, row := range rows {
		normalizedRow := normalizeRowForWrite(row, resolutionForTable(table))
		copyRows = append(copyRows, []any{
			normalizedRow.Provider,
			normalizedRow.ProviderDeviceID,
			normalizedRow.DeviceID,
			normalizedRow.BucketStart.UTC(),
			normalizedRow.SampleCount,
			normalizedRow.FirstTsUnixMS,
			normalizedRow.LastTsUnixMS,
			avgValue(normalizedRow.SOC),
			minValue(normalizedRow.SOC),
			maxValue(normalizedRow.SOC),
			avgValue(normalizedRow.ACIn),
			maxValue(normalizedRow.ACIn),
			avgValue(normalizedRow.PV),
			maxValue(normalizedRow.PV),
			avgValue(normalizedRow.DC),
			maxValue(normalizedRow.DC),
			avgValue(normalizedRow.Load),
			maxValue(normalizedRow.Load),
			avgValue(normalizedRow.Net),
			minValue(normalizedRow.Net),
			maxValue(normalizedRow.Net),
			avgValue(normalizedRow.Battery),
			minValue(normalizedRow.Battery),
			maxValue(normalizedRow.Battery),
			avgValue(normalizedRow.Temp),
			minValue(normalizedRow.Temp),
			maxValue(normalizedRow.Temp),
			solarGeneratedValue(normalizedRow),
			energyValue(normalizedRow.HasACInputEnergyWh, normalizedRow.ACInputEnergyWh),
			energyValue(normalizedRow.HasACOutputEnergyWh, normalizedRow.ACOutputEnergyWh),
			energyValue(normalizedRow.HasDCOutputEnergyWh, normalizedRow.DCOutputEnergyWh),
			energyValue(normalizedRow.HasLoadEnergyWh, normalizedRow.LoadEnergyWh),
			energyValue(normalizedRow.HasBatteryChargeWh, normalizedRow.BatteryChargeWh),
			energyValue(normalizedRow.HasBatteryDischargeWh, normalizedRow.BatteryDischargeWh),
			now,
			now,
		})
	}
	if _, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{tempTable},
		[]string{
			"provider",
			"provider_device_id",
			"device_id",
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
			"ac_input_energy_wh",
			"ac_output_energy_wh",
			"dc_output_energy_wh",
			"load_energy_wh",
			"battery_charge_energy_wh",
			"battery_discharge_energy_wh",
			"created_at",
			"updated_at",
		},
		pgx.CopyFromRows(copyRows),
	); err != nil {
		return fmt.Errorf("copy temp rollup rebuild rows: %w", err)
	}

	if err := deleteWindowRows(ctx, tx, table, affected, from, to); err != nil {
		return err
	}

	insertSQL := fmt.Sprintf(`
INSERT INTO %[1]s (
	provider,
	provider_device_id,
	device_id,
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
	solar_generated_wh,
	ac_input_energy_wh,
	ac_output_energy_wh,
	dc_output_energy_wh,
	load_energy_wh,
	battery_charge_energy_wh,
	battery_discharge_energy_wh,
	created_at,
	updated_at
)
SELECT
	provider,
	provider_device_id,
	device_id,
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
	solar_generated_wh,
	ac_input_energy_wh,
	ac_output_energy_wh,
	dc_output_energy_wh,
	load_energy_wh,
	battery_charge_energy_wh,
	battery_discharge_energy_wh,
	created_at,
	updated_at
FROM %[2]s
`, table, tempTable)

	if _, err := tx.Exec(ctx, insertSQL); err != nil {
		return fmt.Errorf("insert rebuilt rollup rows into %s: %w", table, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit rollup rebuild transaction: %w", err)
	}
	return nil
}

func deleteWindowRows(ctx context.Context, tx pgx.Tx, table string, affected []DeviceWindow, from, to time.Time) error {
	if len(affected) == 0 {
		return nil
	}
	devicePairs := make([][]any, 0, len(affected))
	for _, device := range affected {
		devicePairs = append(devicePairs, []any{device.Provider, device.ProviderDeviceID})
	}
	if _, err := tx.Exec(ctx, "CREATE TEMP TABLE tmp_rollup_rebuild_devices (provider text, provider_device_id text) ON COMMIT DROP"); err != nil {
		return fmt.Errorf("create temp rebuild device table: %w", err)
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"tmp_rollup_rebuild_devices"}, []string{"provider", "provider_device_id"}, pgx.CopyFromRows(devicePairs)); err != nil {
		return fmt.Errorf("copy rebuild device filters: %w", err)
	}
	deleteSQL := fmt.Sprintf(`DELETE FROM %s dst USING tmp_rollup_rebuild_devices dev WHERE dst.provider = dev.provider AND dst.provider_device_id = dev.provider_device_id AND dst.bucket_start >= $1 AND dst.bucket_start < $2`, table)
	if _, err := tx.Exec(ctx, deleteSQL, from.UTC(), to.UTC()); err != nil {
		return fmt.Errorf("delete existing rebuilt rows from %s: %w", table, err)
	}
	return nil
}

func tableNameForResolution(resolution Resolution) (string, error) {
	switch resolution {
	case ResolutionMinute:
		return "telemetry_rollup_minute", nil
	case ResolutionHour:
		return "telemetry_rollup_hour", nil
	case ResolutionDay:
		return "telemetry_rollup_day", nil
	default:
		return "", fmt.Errorf("unknown rollup rebuild resolution: %s", resolution)
	}
}

func avgValue(metric metricAccumulator) any {
	if !metric.valid || metric.count <= 0 {
		return nil
	}
	return metric.sum / float64(metric.count)
}

func minValue(metric metricAccumulator) any {
	if !metric.valid {
		return nil
	}
	return metric.min
}

func maxValue(metric metricAccumulator) any {
	if !metric.valid {
		return nil
	}
	return metric.max
}

func solarGeneratedValue(row BucketRow) any {
	if !row.HasSolarGeneratedWh {
		return nil
	}
	return row.SolarGeneratedWh
}

func energyValue(valid bool, value float64) any {
	if !valid {
		return nil
	}
	return value
}

func groupRowsByDevice(rows []BucketRow) map[string][]BucketRow {
	grouped := make(map[string][]BucketRow, len(rows))
	for _, row := range rows {
		key := row.Provider + "|" + row.ProviderDeviceID
		grouped[key] = append(grouped[key], row)
	}
	return grouped
}

func normalizeAffectedDevices(devices []DeviceWindow) []DeviceWindow {
	seen := make(map[string]struct{}, len(devices))
	out := make([]DeviceWindow, 0, len(devices))
	for _, device := range devices {
		if strings.TrimSpace(device.Provider) == "" || strings.TrimSpace(device.ProviderDeviceID) == "" {
			continue
		}
		key := device.Provider + "|" + device.ProviderDeviceID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, DeviceWindow{
			Provider:         device.Provider,
			ProviderDeviceID: device.ProviderDeviceID,
		})
	}
	return out
}

func dedupeRows(rows []BucketRow) []BucketRow {
	if len(rows) <= 1 {
		return rows
	}
	byKey := make(map[string]BucketRow, len(rows))
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		key := bucketMapKey(row.Provider, row.ProviderDeviceID, row.BucketStart)
		if _, exists := byKey[key]; !exists {
			keys = append(keys, key)
		}
		byKey[key] = row
	}
	sort.Strings(keys)
	out := make([]BucketRow, 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}
	return out
}

func normalizeRowForWrite(row BucketRow, resolution Resolution) BucketRow {
	if row.SampleCount > 0 {
		return row
	}
	if !row.HasSolarGeneratedWh {
		return row
	}
	bucketDuration := bucketDurationForResolution(resolution)
	firstTs := row.BucketStart.UTC().UnixMilli()
	lastTs := firstTs
	if bucketDuration > 0 {
		lastTs = row.BucketStart.Add(bucketDuration).Add(-time.Millisecond).UTC().UnixMilli()
	}
	row.SampleCount = 0
	row.FirstTsUnixMS = firstTs
	row.LastTsUnixMS = lastTs
	return row
}

func resolutionForTable(table string) Resolution {
	switch table {
	case "telemetry_rollup_minute":
		return ResolutionMinute
	case "telemetry_rollup_hour":
		return ResolutionHour
	case "telemetry_rollup_day":
		return ResolutionDay
	default:
		return ""
	}
}

func bucketDurationForResolution(resolution Resolution) time.Duration {
	switch resolution {
	case ResolutionMinute:
		return time.Minute
	case ResolutionHour:
		return time.Hour
	case ResolutionDay:
		return 24 * time.Hour
	default:
		return 0
	}
}
