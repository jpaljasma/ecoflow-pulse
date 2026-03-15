package rollupworker

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
)

type Store interface {
	ApplyEnvelope(ctx context.Context, env *envelopev1.TelemetryEnvelope) error
	Close() error
}

type PostgresStore struct {
	db                       *sql.DB
	nowFn                    func() time.Time
	dedupInsertQuery         string
	minuteUpsertQuery        string
	hourUpsertQuery          string
	dayUpsertQuery           string
	minuteSolarUpsertQuery   string
	hourSolarUpsertQuery     string
	daySolarUpsertQuery      string
	minuteEnergyUpsertQuery  string
	hourEnergyUpsertQuery    string
	dayEnergyUpsertQuery     string
	minutePVPortUpsertQuery  string
	hourPVPortUpsertQuery    string
	dayPVPortUpsertQuery     string
	mu                       sync.Mutex
	integrationStateByDevice map[string]integrationState
}

type powerChannelState struct {
	lastAt    time.Time
	hasLastAt bool
	watts     float64
	hasWatts  bool
}

type integrationState struct {
	lastEnvelopeAt    time.Time
	hasLastEnvelopeAt bool
	lastPVAt          time.Time
	hasLastPVAt       bool
	currentPV         float64
	hasPV             bool
	acIn              powerChannelState
	acOutput          powerChannelState
	dcOutput          powerChannelState
	load              powerChannelState
	batteryCharge     powerChannelState
	batteryDischarge  powerChannelState
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("rollup postgres dsn is required")
	}
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, fmt.Errorf("apply rollup postgres search_path: %w", err)
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
		db:                       db,
		nowFn:                    time.Now,
		dedupInsertQuery:         buildRollupDedupInsertQuery(),
		minuteUpsertQuery:        buildUpsertQuery("telemetry_rollup_minute"),
		hourUpsertQuery:          buildUpsertQuery("telemetry_rollup_hour"),
		dayUpsertQuery:           buildUpsertQuery("telemetry_rollup_day"),
		minuteSolarUpsertQuery:   buildSolarUpsertQuery("telemetry_rollup_minute"),
		hourSolarUpsertQuery:     buildSolarUpsertQuery("telemetry_rollup_hour"),
		daySolarUpsertQuery:      buildSolarUpsertQuery("telemetry_rollup_day"),
		minuteEnergyUpsertQuery:  buildEnergyUpsertQuery("telemetry_rollup_minute"),
		hourEnergyUpsertQuery:    buildEnergyUpsertQuery("telemetry_rollup_hour"),
		dayEnergyUpsertQuery:     buildEnergyUpsertQuery("telemetry_rollup_day"),
		minutePVPortUpsertQuery:  buildPVPortUpsertQuery("telemetry_rollup_pv_port_minute"),
		hourPVPortUpsertQuery:    buildPVPortUpsertQuery("telemetry_rollup_pv_port_hour"),
		dayPVPortUpsertQuery:     buildPVPortUpsertQuery("telemetry_rollup_pv_port_day"),
		integrationStateByDevice: make(map[string]integrationState),
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

	sampleForMetrics := *sample
	sampleForMetrics.Metrics.SolarGeneratedWh = optionalFloat{}
	now := s.nowFn().UTC()
	minuteBucket := sample.EventTime.Truncate(time.Minute)
	hourBucket := sample.EventTime.Truncate(time.Hour)
	dayBucket := time.Date(sample.EventTime.Year(), sample.EventTime.Month(), sample.EventTime.Day(), 0, 0, 0, 0, time.UTC)
	stateKey := sample.Provider + "|" + sample.ProviderDeviceID

	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.integrationStateByDevice[stateKey]
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rollup transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	claimed, err := s.claimEnvelopeDedup(ctx, tx, env, sample, now)
	if err != nil {
		return err
	}
	if !claimed {
		s.integrationStateByDevice[stateKey] = advanceDuplicateIntegrationState(state, sample)
		return nil
	}

	if err := s.execUpsert(ctx, tx, s.minuteUpsertQuery, &sampleForMetrics, minuteBucket, now); err != nil {
		return err
	}
	if err := s.execUpsert(ctx, tx, s.hourUpsertQuery, &sampleForMetrics, hourBucket, now); err != nil {
		return err
	}
	if err := s.execUpsert(ctx, tx, s.dayUpsertQuery, &sampleForMetrics, dayBucket, now); err != nil {
		return err
	}
	if err := s.execSolarUpserts(ctx, tx, sample, state, now); err != nil {
		return err
	}
	if err := s.execEnergyUpserts(ctx, tx, sample, state, now); err != nil {
		return err
	}
	if err := s.execPVPortUpserts(ctx, tx, sample, minuteBucket, hourBucket, dayBucket, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit rollup transaction: %w", err)
	}
	s.integrationStateByDevice[stateKey] = advanceIntegrationState(state, sample)
	return nil
}

func (s *PostgresStore) claimEnvelopeDedup(ctx context.Context, tx *sql.Tx, env *envelopev1.TelemetryEnvelope, sample *RollupSample, now time.Time) (bool, error) {
	if tx == nil || sample == nil {
		return false, nil
	}
	dedupKey := rollupDedupKey(env)
	if dedupKey == "" {
		return true, nil
	}
	result, err := tx.ExecContext(
		ctx,
		s.dedupInsertQuery,
		dedupKey,
		nullableTrimmed(env.GetEnvelopeId()),
		nullableTrimmed(env.GetMessageId()),
		sample.Provider,
		sample.ProviderDeviceID,
		sample.DeviceID,
		sample.EventTime.UTC(),
		now,
	)
	if err != nil {
		return false, fmt.Errorf("claim rollup envelope dedup: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read rollup envelope dedup result: %w", err)
	}
	return rowsAffected > 0, nil
}

func (s *PostgresStore) execSolarUpserts(ctx context.Context, tx *sql.Tx, sample *RollupSample, state integrationState, now time.Time) error {
	if sample == nil || !state.hasLastEnvelopeAt || !sample.EventTime.After(state.lastEnvelopeAt) || !state.hasPV {
		return nil
	}
	if err := s.execSolarUpsert(ctx, tx, s.minuteSolarUpsertQuery, sample, state, now, time.Minute, false); err != nil {
		return err
	}
	if err := s.execSolarUpsert(ctx, tx, s.hourSolarUpsertQuery, sample, state, now, time.Hour, false); err != nil {
		return err
	}
	if err := s.execSolarUpsert(ctx, tx, s.daySolarUpsertQuery, sample, state, now, 24*time.Hour, true); err != nil {
		return err
	}
	return nil
}

func (s *PostgresStore) execSolarUpsert(ctx context.Context, tx *sql.Tx, query string, sample *RollupSample, state integrationState, now time.Time, bucketWidth time.Duration, dayBucket bool) error {
	var firstErr error
	IntegrateSolarWindow(state.lastEnvelopeAt, sample.EventTime, state.lastPVAt, state.currentPV, DefaultSolarCarryForwardMaxGap, bucketWidth, dayBucket, func(bucketStart time.Time, segmentStart time.Time, segmentEnd time.Time, wattHours float64) {
		if firstErr != nil {
			return
		}
		firstTS := segmentStart.UnixMilli()
		lastTS := segmentEnd.Add(-time.Millisecond).UnixMilli()
		if lastTS < firstTS {
			lastTS = firstTS
		}
		if _, err := tx.ExecContext(ctx, query,
			sample.Provider,
			sample.ProviderDeviceID,
			sample.DeviceID,
			bucketStart,
			0,
			firstTS,
			lastTS,
			wattHours,
			now,
			now,
		); err != nil {
			firstErr = fmt.Errorf("upsert solar rollup bucket %s: %w", bucketStart.Format(time.RFC3339), err)
		}
	})
	return firstErr
}

type energyDelta struct {
	firstTSUnixMs            int64
	lastTSUnixMs             int64
	hasTimeRange             bool
	acInputEnergyWh          float64
	acOutputEnergyWh         float64
	dcOutputEnergyWh         float64
	loadEnergyWh             float64
	batteryChargeEnergyWh    float64
	batteryDischargeEnergyWh float64
}

func (s *PostgresStore) execEnergyUpserts(ctx context.Context, tx *sql.Tx, sample *RollupSample, state integrationState, now time.Time) error {
	if sample == nil || !state.hasLastEnvelopeAt || !sample.EventTime.After(state.lastEnvelopeAt) {
		return nil
	}
	if err := s.execEnergyUpsert(ctx, tx, s.minuteEnergyUpsertQuery, sample, state, now, time.Minute, false); err != nil {
		return err
	}
	if err := s.execEnergyUpsert(ctx, tx, s.hourEnergyUpsertQuery, sample, state, now, time.Hour, false); err != nil {
		return err
	}
	if err := s.execEnergyUpsert(ctx, tx, s.dayEnergyUpsertQuery, sample, state, now, 24*time.Hour, true); err != nil {
		return err
	}
	return nil
}

func (s *PostgresStore) execEnergyUpsert(ctx context.Context, tx *sql.Tx, query string, sample *RollupSample, state integrationState, now time.Time, bucketWidth time.Duration, dayBucket bool) error {
	accumulated := make(map[time.Time]*energyDelta)
	integrateEnergyChannel(accumulated, state.lastEnvelopeAt, sample.EventTime, state.acIn, bucketWidth, dayBucket, func(delta *energyDelta, wattHours float64) {
		delta.acInputEnergyWh += wattHours
	})
	integrateEnergyChannel(accumulated, state.lastEnvelopeAt, sample.EventTime, state.acOutput, bucketWidth, dayBucket, func(delta *energyDelta, wattHours float64) {
		delta.acOutputEnergyWh += wattHours
	})
	integrateEnergyChannel(accumulated, state.lastEnvelopeAt, sample.EventTime, state.dcOutput, bucketWidth, dayBucket, func(delta *energyDelta, wattHours float64) {
		delta.dcOutputEnergyWh += wattHours
	})
	integrateEnergyChannel(accumulated, state.lastEnvelopeAt, sample.EventTime, state.load, bucketWidth, dayBucket, func(delta *energyDelta, wattHours float64) {
		delta.loadEnergyWh += wattHours
	})
	integrateEnergyChannel(accumulated, state.lastEnvelopeAt, sample.EventTime, state.batteryCharge, bucketWidth, dayBucket, func(delta *energyDelta, wattHours float64) {
		delta.batteryChargeEnergyWh += wattHours
	})
	integrateEnergyChannel(accumulated, state.lastEnvelopeAt, sample.EventTime, state.batteryDischarge, bucketWidth, dayBucket, func(delta *energyDelta, wattHours float64) {
		delta.batteryDischargeEnergyWh += wattHours
	})
	if len(accumulated) == 0 {
		return nil
	}

	bucketStarts := make([]time.Time, 0, len(accumulated))
	for bucketStart := range accumulated {
		bucketStarts = append(bucketStarts, bucketStart)
	}
	sort.Slice(bucketStarts, func(i, j int) bool {
		return bucketStarts[i].Before(bucketStarts[j])
	})

	for _, bucketStart := range bucketStarts {
		delta := accumulated[bucketStart]
		if delta == nil || !delta.hasTimeRange {
			continue
		}
		if _, err := tx.ExecContext(ctx, query,
			sample.Provider,
			sample.ProviderDeviceID,
			sample.DeviceID,
			bucketStart,
			0,
			delta.firstTSUnixMs,
			delta.lastTSUnixMs,
			nullableEnergyValue(delta.acInputEnergyWh),
			nullableEnergyValue(delta.acOutputEnergyWh),
			nullableEnergyValue(delta.dcOutputEnergyWh),
			nullableEnergyValue(delta.loadEnergyWh),
			nullableEnergyValue(delta.batteryChargeEnergyWh),
			nullableEnergyValue(delta.batteryDischargeEnergyWh),
			now,
			now,
		); err != nil {
			return fmt.Errorf("upsert energy rollup bucket %s: %w", bucketStart.Format(time.RFC3339), err)
		}
	}
	return nil
}

func integrateEnergyChannel(accumulated map[time.Time]*energyDelta, start time.Time, end time.Time, channel powerChannelState, bucketWidth time.Duration, dayBucket bool, apply func(delta *energyDelta, wattHours float64)) {
	if accumulated == nil || apply == nil || !channel.hasWatts || !channel.hasLastAt {
		return
	}
	IntegratePowerWindow(start, end, channel.lastAt, channel.watts, DefaultSolarCarryForwardMaxGap, bucketWidth, dayBucket, func(bucketStart time.Time, segmentStart time.Time, segmentEnd time.Time, wattHours float64) {
		delta := accumulated[bucketStart]
		if delta == nil {
			delta = &energyDelta{}
			accumulated[bucketStart] = delta
		}
		firstTS := segmentStart.UnixMilli()
		lastTS := segmentEnd.Add(-time.Millisecond).UnixMilli()
		if lastTS < firstTS {
			lastTS = firstTS
		}
		if !delta.hasTimeRange || firstTS < delta.firstTSUnixMs {
			delta.firstTSUnixMs = firstTS
		}
		if !delta.hasTimeRange || lastTS > delta.lastTSUnixMs {
			delta.lastTSUnixMs = lastTS
		}
		delta.hasTimeRange = true
		apply(delta, wattHours)
	})
}

func advanceIntegrationState(state integrationState, sample *RollupSample) integrationState {
	if sample == nil {
		return state
	}
	if !state.hasLastEnvelopeAt || sample.EventTime.After(state.lastEnvelopeAt) {
		state.lastEnvelopeAt = sample.EventTime
		state.hasLastEnvelopeAt = true
	}
	if sample.Metrics.PV.Valid && (!state.hasLastPVAt || !sample.EventTime.Before(state.lastPVAt)) {
		state.lastPVAt = sample.EventTime
		state.hasLastPVAt = true
		if sample.Metrics.PV.Value > 0 {
			state.currentPV = sample.Metrics.PV.Value
			state.hasPV = true
		} else {
			state.currentPV = 0
			state.hasPV = false
		}
	}
	state.acIn = advancePowerChannelState(state.acIn, sample.EventTime, sample.Metrics.ACIn)
	state.acOutput = advancePowerChannelState(state.acOutput, sample.EventTime, sample.Metrics.ACOutput)
	state.dcOutput = advancePowerChannelState(state.dcOutput, sample.EventTime, sample.Metrics.DC)
	state.load = advancePowerChannelState(state.load, sample.EventTime, sample.Metrics.Load)
	state.batteryCharge = advancePowerChannelState(state.batteryCharge, sample.EventTime, positiveOptionalFloat(sample.Metrics.Battery))
	state.batteryDischarge = advancePowerChannelState(state.batteryDischarge, sample.EventTime, negativeOptionalFloat(sample.Metrics.Battery))
	return state
}

func advanceDuplicateIntegrationState(state integrationState, sample *RollupSample) integrationState {
	if sample == nil {
		return state
	}
	if state.hasLastEnvelopeAt && !sample.EventTime.After(state.lastEnvelopeAt) {
		return state
	}
	return advanceIntegrationState(state, sample)
}

func advancePowerChannelState(state powerChannelState, at time.Time, metric optionalFloat) powerChannelState {
	if !metric.Valid {
		return state
	}
	if state.hasLastAt && at.Before(state.lastAt) {
		return state
	}
	state.lastAt = at
	state.hasLastAt = true
	if metric.Value > 0 {
		state.watts = metric.Value
		state.hasWatts = true
	} else {
		state.watts = 0
		state.hasWatts = false
	}
	return state
}

func nullableTrimmed(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func positiveOptionalFloat(metric optionalFloat) optionalFloat {
	if !metric.Valid || metric.Value <= 0 {
		return optionalFloat{Valid: true}
	}
	return optionalFloat{Value: metric.Value, Valid: true}
}

func negativeOptionalFloat(metric optionalFloat) optionalFloat {
	if !metric.Valid || metric.Value >= 0 {
		return optionalFloat{Valid: true}
	}
	return optionalFloat{Value: -metric.Value, Valid: true}
}

func nullableEnergyValue(value float64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func (s *PostgresStore) execPVPortUpserts(ctx context.Context, tx *sql.Tx, sample *RollupSample, minuteBucket, hourBucket, dayBucket, now time.Time) error {
	if sample == nil || len(sample.PVPorts) == 0 {
		return nil
	}
	for _, observation := range sample.PVPorts {
		if err := s.execPVPortUpsert(ctx, tx, s.minutePVPortUpsertQuery, sample, observation, minuteBucket, now); err != nil {
			return err
		}
		if err := s.execPVPortUpsert(ctx, tx, s.hourPVPortUpsertQuery, sample, observation, hourBucket, now); err != nil {
			return err
		}
		if err := s.execPVPortUpsert(ctx, tx, s.dayPVPortUpsertQuery, sample, observation, dayBucket, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) execPVPortUpsert(ctx context.Context, tx *sql.Tx, query string, sample *RollupSample, observation PVPortObservation, bucketStart, now time.Time) error {
	if _, err := tx.ExecContext(ctx, query,
		sample.Provider,
		sample.ProviderDeviceID,
		sample.DeviceID,
		observation.PortID,
		observation.PortLabel,
		bucketStart,
		1,
		sample.EventUnixMs,
		sample.EventUnixMs,
		observation.Volts,
		observation.Amps,
		observation.Watts,
		observation.Volts,
		observation.Amps,
		observation.Watts,
		sample.EventUnixMs,
		now,
		now,
	); err != nil {
		return fmt.Errorf("upsert pv-port rollup bucket %s/%s: %w", observation.PortID, bucketStart.Format(time.RFC3339), err)
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
		sample.Metrics.ACOutput.sqlValue(),
		sample.Metrics.ACOutput.sqlValue(),
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
		created_at,
		updated_at
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32
	)
	ON CONFLICT (provider, provider_device_id, bucket_start) DO UPDATE SET
		sample_count = %s.sample_count + EXCLUDED.sample_count,
		first_ts_unix_ms = CASE
			WHEN %s.sample_count = 0 THEN EXCLUDED.first_ts_unix_ms
			ELSE LEAST(%s.first_ts_unix_ms, EXCLUDED.first_ts_unix_ms)
		END,
		last_ts_unix_ms = CASE
			WHEN %s.sample_count = 0 THEN EXCLUDED.last_ts_unix_ms
			ELSE GREATEST(%s.last_ts_unix_ms, EXCLUDED.last_ts_unix_ms)
		END,
		soc_avg_pct = %s,
		soc_min_pct = %s,
		soc_max_pct = %s,
		ac_in_avg_w = %s,
		ac_in_max_w = %s,
		ac_output_avg_w = %s,
		ac_output_max_w = %s,
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
		table,
		table,
		weightedAverageExpr(table, "soc_avg_pct"),
		minExpr(table, "soc_min_pct"),
		maxExpr(table, "soc_max_pct"),
		weightedAverageExpr(table, "ac_in_avg_w"),
		maxExpr(table, "ac_in_max_w"),
		weightedAverageExpr(table, "ac_output_avg_w"),
		maxExpr(table, "ac_output_max_w"),
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

func buildRollupDedupInsertQuery() string {
	return `INSERT INTO telemetry_rollup_applied_envelopes (
		dedup_key,
		envelope_id,
		message_id,
		provider,
		provider_device_id,
		device_id,
		event_time,
		applied_at
	) VALUES ($1, $2, $3, $4, $5, $6::uuid, $7, $8)
	ON CONFLICT (dedup_key) DO NOTHING`
}

func buildSolarUpsertQuery(table string) string {
	return fmt.Sprintf(`INSERT INTO %s (
		provider,
		provider_device_id,
		device_id,
		bucket_start,
		sample_count,
		first_ts_unix_ms,
		last_ts_unix_ms,
		solar_generated_wh,
		created_at,
		updated_at
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10
	)
	ON CONFLICT (provider, provider_device_id, bucket_start) DO UPDATE SET
		device_id = EXCLUDED.device_id,
		first_ts_unix_ms = CASE
			WHEN %[1]s.sample_count = 0 THEN LEAST(%[1]s.first_ts_unix_ms, EXCLUDED.first_ts_unix_ms)
			ELSE %[1]s.first_ts_unix_ms
		END,
		last_ts_unix_ms = CASE
			WHEN %[1]s.sample_count = 0 THEN GREATEST(%[1]s.last_ts_unix_ms, EXCLUDED.last_ts_unix_ms)
			ELSE %[1]s.last_ts_unix_ms
		END,
		solar_generated_wh = CASE
			WHEN %[1]s.solar_generated_wh IS NULL THEN EXCLUDED.solar_generated_wh
			WHEN EXCLUDED.solar_generated_wh IS NULL THEN %[1]s.solar_generated_wh
			ELSE %[1]s.solar_generated_wh + EXCLUDED.solar_generated_wh
		END,
		updated_at = EXCLUDED.updated_at`, table)
}

func buildEnergyUpsertQuery(table string) string {
	return fmt.Sprintf(`INSERT INTO %s (
		provider,
		provider_device_id,
		device_id,
		bucket_start,
		sample_count,
		first_ts_unix_ms,
		last_ts_unix_ms,
		ac_input_energy_wh,
		ac_output_energy_wh,
		dc_output_energy_wh,
		load_energy_wh,
		battery_charge_energy_wh,
		battery_discharge_energy_wh,
		created_at,
		updated_at
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15
	)
	ON CONFLICT (provider, provider_device_id, bucket_start) DO UPDATE SET
		device_id = EXCLUDED.device_id,
		first_ts_unix_ms = CASE
			WHEN %[1]s.sample_count = 0 THEN LEAST(%[1]s.first_ts_unix_ms, EXCLUDED.first_ts_unix_ms)
			ELSE %[1]s.first_ts_unix_ms
		END,
		last_ts_unix_ms = CASE
			WHEN %[1]s.sample_count = 0 THEN GREATEST(%[1]s.last_ts_unix_ms, EXCLUDED.last_ts_unix_ms)
			ELSE %[1]s.last_ts_unix_ms
		END,
		ac_input_energy_wh = %[2]s,
		ac_output_energy_wh = %[3]s,
		dc_output_energy_wh = %[4]s,
		load_energy_wh = %[5]s,
		battery_charge_energy_wh = %[6]s,
		battery_discharge_energy_wh = %[7]s,
		updated_at = EXCLUDED.updated_at`, table,
		sumExpr(table, "ac_input_energy_wh"),
		sumExpr(table, "ac_output_energy_wh"),
		sumExpr(table, "dc_output_energy_wh"),
		sumExpr(table, "load_energy_wh"),
		sumExpr(table, "battery_charge_energy_wh"),
		sumExpr(table, "battery_discharge_energy_wh"),
	)
}

func buildPVPortUpsertQuery(table string) string {
	return fmt.Sprintf(`INSERT INTO %s (
		provider,
		provider_device_id,
		device_id,
		port_id,
		port_label,
		bucket_start,
		sample_count,
		first_ts_unix_ms,
		last_ts_unix_ms,
		max_observed_volts,
		max_observed_amps,
		max_observed_watts,
		last_observed_volts,
		last_observed_amps,
		last_observed_watts,
		last_observed_at_unix_ms,
		created_at,
		updated_at
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18
	)
	ON CONFLICT (provider, provider_device_id, port_id, bucket_start) DO UPDATE SET
		device_id = EXCLUDED.device_id,
		port_label = CASE
			WHEN EXCLUDED.last_observed_at_unix_ms IS NULL THEN %[1]s.port_label
			WHEN %[1]s.last_observed_at_unix_ms IS NULL THEN EXCLUDED.port_label
			WHEN EXCLUDED.last_observed_at_unix_ms >= %[1]s.last_observed_at_unix_ms THEN EXCLUDED.port_label
			ELSE %[1]s.port_label
		END,
		sample_count = %[1]s.sample_count + EXCLUDED.sample_count,
		first_ts_unix_ms = LEAST(%[1]s.first_ts_unix_ms, EXCLUDED.first_ts_unix_ms),
		last_ts_unix_ms = GREATEST(%[1]s.last_ts_unix_ms, EXCLUDED.last_ts_unix_ms),
		max_observed_volts = %[2]s,
		max_observed_amps = %[3]s,
		max_observed_watts = %[4]s,
		last_observed_volts = CASE
			WHEN EXCLUDED.last_observed_at_unix_ms IS NULL THEN %[1]s.last_observed_volts
			WHEN %[1]s.last_observed_at_unix_ms IS NULL THEN EXCLUDED.last_observed_volts
			WHEN EXCLUDED.last_observed_at_unix_ms >= %[1]s.last_observed_at_unix_ms THEN EXCLUDED.last_observed_volts
			ELSE %[1]s.last_observed_volts
		END,
		last_observed_amps = CASE
			WHEN EXCLUDED.last_observed_at_unix_ms IS NULL THEN %[1]s.last_observed_amps
			WHEN %[1]s.last_observed_at_unix_ms IS NULL THEN EXCLUDED.last_observed_amps
			WHEN EXCLUDED.last_observed_at_unix_ms >= %[1]s.last_observed_at_unix_ms THEN EXCLUDED.last_observed_amps
			ELSE %[1]s.last_observed_amps
		END,
		last_observed_watts = CASE
			WHEN EXCLUDED.last_observed_at_unix_ms IS NULL THEN %[1]s.last_observed_watts
			WHEN %[1]s.last_observed_at_unix_ms IS NULL THEN EXCLUDED.last_observed_watts
			WHEN EXCLUDED.last_observed_at_unix_ms >= %[1]s.last_observed_at_unix_ms THEN EXCLUDED.last_observed_watts
			ELSE %[1]s.last_observed_watts
		END,
		last_observed_at_unix_ms = CASE
			WHEN EXCLUDED.last_observed_at_unix_ms IS NULL THEN %[1]s.last_observed_at_unix_ms
			WHEN %[1]s.last_observed_at_unix_ms IS NULL THEN EXCLUDED.last_observed_at_unix_ms
			ELSE GREATEST(%[1]s.last_observed_at_unix_ms, EXCLUDED.last_observed_at_unix_ms)
		END,
		updated_at = EXCLUDED.updated_at`, table,
		maxExpr(table, "max_observed_volts"),
		maxExpr(table, "max_observed_amps"),
		maxExpr(table, "max_observed_watts"),
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
