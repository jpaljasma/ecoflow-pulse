package rollupworker

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

type Store interface {
	ApplyEnvelope(ctx context.Context, env *envelopev1.TelemetryEnvelope) error
	Close() error
}

type PostgresStore struct {
	db                *sql.DB
	nowFn             func() time.Time
	minuteUpsertQuery string
	hourUpsertQuery   string
	dayUpsertQuery    string
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("rollup postgres dsn is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open rollup postgres connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping rollup postgres: %w", err)
	}
	return newPostgresStore(db), nil
}

func newPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{
		db:                db,
		nowFn:             time.Now,
		minuteUpsertQuery: buildUpsertQuery("telemetry_rollup_minute"),
		hourUpsertQuery:   buildUpsertQuery("telemetry_rollup_hour"),
		dayUpsertQuery:    buildUpsertQuery("telemetry_rollup_day"),
	}
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) ApplyEnvelope(ctx context.Context, env *envelopev1.TelemetryEnvelope) error {
	sample, err := SampleFromEnvelope(env)
	if err != nil {
		return err
	}
	if !sample.Metrics.HasAny() {
		return ErrNoRollupMetrics
	}

	now := s.nowFn().UTC()
	minuteBucket := sample.EventTime.Truncate(time.Minute)
	hourBucket := sample.EventTime.Truncate(time.Hour)
	dayBucket := time.Date(sample.EventTime.Year(), sample.EventTime.Month(), sample.EventTime.Day(), 0, 0, 0, 0, time.UTC)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rollup transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := s.execUpsert(ctx, tx, s.minuteUpsertQuery, sample, minuteBucket, now); err != nil {
		return err
	}
	if err := s.execUpsert(ctx, tx, s.hourUpsertQuery, sample, hourBucket, now); err != nil {
		return err
	}
	if err := s.execUpsert(ctx, tx, s.dayUpsertQuery, sample, dayBucket, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rollup transaction: %w", err)
	}
	return nil
}

func (s *PostgresStore) execUpsert(ctx context.Context, tx *sql.Tx, query string, sample *RollupSample, bucketStart time.Time, now time.Time) error {
	_, err := tx.ExecContext(ctx, query,
		sample.Provider,
		sample.ProviderDeviceID,
		sample.DeviceID,
		bucketStart,
		1,
		sample.EventUnixMs,
		sample.EventUnixMs,
		sample.Metrics.SOC.sqlValue(),
		sample.Metrics.SOC.sqlValue(),
		sample.Metrics.SOC.sqlValue(),
		sample.Metrics.ACIn.sqlValue(),
		sample.Metrics.ACIn.sqlValue(),
		sample.Metrics.PV.sqlValue(),
		sample.Metrics.PV.sqlValue(),
		sample.Metrics.DC.sqlValue(),
		sample.Metrics.DC.sqlValue(),
		sample.Metrics.Load.sqlValue(),
		sample.Metrics.Load.sqlValue(),
		sample.Metrics.Net.sqlValue(),
		sample.Metrics.Net.sqlValue(),
		sample.Metrics.Net.sqlValue(),
		sample.Metrics.Battery.sqlValue(),
		sample.Metrics.Battery.sqlValue(),
		sample.Metrics.Battery.sqlValue(),
		sample.Metrics.Temp.sqlValue(),
		sample.Metrics.Temp.sqlValue(),
		sample.Metrics.Temp.sqlValue(),
		sample.Metrics.SolarGeneratedWh.sqlValue(),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("upsert rollup bucket %s: %w", bucketStart.Format(time.RFC3339), err)
	}
	return nil
}

func buildUpsertQuery(table string) string {
	return fmt.Sprintf(`INSERT INTO %s (
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
		created_at,
		updated_at
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30
	)
	ON CONFLICT (provider, provider_device_id, bucket_start) DO UPDATE SET
		sample_count = %s.sample_count + EXCLUDED.sample_count,
		first_ts_unix_ms = LEAST(%s.first_ts_unix_ms, EXCLUDED.first_ts_unix_ms),
		last_ts_unix_ms = GREATEST(%s.last_ts_unix_ms, EXCLUDED.last_ts_unix_ms),
		soc_avg_pct = %s,
		soc_min_pct = %s,
		soc_max_pct = %s,
		ac_in_avg_w = %s,
		ac_in_max_w = %s,
		pv_avg_w = %s,
		pv_max_w = %s,
		dc_avg_w = %s,
		dc_max_w = %s,
		load_avg_w = %s,
		load_max_w = %s,
		net_avg_w = %s,
		net_min_w = %s,
		net_max_w = %s,
		battery_avg_w = %s,
		battery_min_w = %s,
		battery_max_w = %s,
		temp_avg_c = %s,
		temp_min_c = %s,
		temp_max_c = %s,
		solar_generated_wh = %s,
		updated_at = EXCLUDED.updated_at`,
		table,
		table,
		table,
		table,
		weightedAverageExpr(table, "soc_avg_pct"),
		minExpr(table, "soc_min_pct"),
		maxExpr(table, "soc_max_pct"),
		weightedAverageExpr(table, "ac_in_avg_w"),
		maxExpr(table, "ac_in_max_w"),
		weightedAverageExpr(table, "pv_avg_w"),
		maxExpr(table, "pv_max_w"),
		weightedAverageExpr(table, "dc_avg_w"),
		maxExpr(table, "dc_max_w"),
		weightedAverageExpr(table, "load_avg_w"),
		maxExpr(table, "load_max_w"),
		weightedAverageExpr(table, "net_avg_w"),
		minExpr(table, "net_min_w"),
		maxExpr(table, "net_max_w"),
		weightedAverageExpr(table, "battery_avg_w"),
		minExpr(table, "battery_min_w"),
		maxExpr(table, "battery_max_w"),
		weightedAverageExpr(table, "temp_avg_c"),
		minExpr(table, "temp_min_c"),
		maxExpr(table, "temp_max_c"),
		sumExpr(table, "solar_generated_wh"),
	)
}

func weightedAverageExpr(table, column string) string {
	return fmt.Sprintf(`CASE
		WHEN EXCLUDED.%[2]s IS NULL THEN %[1]s.%[2]s
		WHEN %[1]s.%[2]s IS NULL THEN EXCLUDED.%[2]s
		ELSE ((%[1]s.%[2]s * %[1]s.sample_count) + (EXCLUDED.%[2]s * EXCLUDED.sample_count)) / (%[1]s.sample_count + EXCLUDED.sample_count)
	END`, table, column)
}

func minExpr(table, column string) string {
	return fmt.Sprintf(`CASE
		WHEN EXCLUDED.%[2]s IS NULL THEN %[1]s.%[2]s
		WHEN %[1]s.%[2]s IS NULL THEN EXCLUDED.%[2]s
		ELSE LEAST(%[1]s.%[2]s, EXCLUDED.%[2]s)
	END`, table, column)
}

func maxExpr(table, column string) string {
	return fmt.Sprintf(`CASE
		WHEN EXCLUDED.%[2]s IS NULL THEN %[1]s.%[2]s
		WHEN %[1]s.%[2]s IS NULL THEN EXCLUDED.%[2]s
		ELSE GREATEST(%[1]s.%[2]s, EXCLUDED.%[2]s)
	END`, table, column)
}

func sumExpr(table, column string) string {
	return fmt.Sprintf(`CASE
		WHEN EXCLUDED.%[2]s IS NULL THEN %[1]s.%[2]s
		WHEN %[1]s.%[2]s IS NULL THEN EXCLUDED.%[2]s
		ELSE %[1]s.%[2]s + EXCLUDED.%[2]s
	END`, table, column)
}
