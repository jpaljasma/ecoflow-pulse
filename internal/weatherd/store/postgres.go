package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jpaljasma/ecoflow-pulse/internal/dbpool"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
)

type PostgresStore struct {
	db    *sql.DB
	nowFn func() time.Time
}

func NewPostgresStore(dsn string, nowFn func() time.Time) (*PostgresStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("weather postgres dsn is required")
	}
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, fmt.Errorf("apply weather postgres search_path: %w", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open weather postgres: %w", err)
	}
	dbpool.ConfigureSQL(db)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping weather postgres: %w", err)
	}
	if nowFn == nil {
		nowFn = time.Now
	}
	return &PostgresStore{db: db, nowFn: nowFn}, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) SaveForecastBundle(ctx context.Context, req weatherd.Request, bundle weatherd.Bundle) error {
	encodedBundle, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("marshal weather bundle: %w", err)
	}
	encodedReq, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal weather request: %w", err)
	}
	now := s.nowFn().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin weather snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var snapshotID string
	err = tx.QueryRowContext(ctx, `
INSERT INTO weather_forecast_snapshots (
	canonical_location_key,
	timezone,
	issued_at,
	source,
	model_selection,
	actual_source,
	request_json,
	bundle_json,
	created_at,
	updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING id::text;
`,
		bundle.Provenance.CanonicalLocationKey,
		bundle.Provenance.Timezone,
		bundle.Provenance.IssuedAt.UTC(),
		bundle.Provenance.Source,
		bundle.Provenance.ModelSelection,
		bundle.Provenance.ActualSource,
		encodedReq,
		encodedBundle,
		now,
	).Scan(&snapshotID)
	if err != nil {
		return fmt.Errorf("insert weather snapshot: %w", err)
	}
	for _, point := range bundle.Hourly {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO weather_forecast_points (
	snapshot_id,
	target_time,
	created_at
)
VALUES ($1::uuid, $2, $3)
ON CONFLICT (snapshot_id, target_time) DO NOTHING;
`, snapshotID, point.Time.UTC(), now); err != nil {
			return fmt.Errorf("insert weather forecast point: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit weather snapshot tx: %w", err)
	}
	return nil
}

func (s *PostgresStore) LatestBundle(ctx context.Context, canonicalLocationKey string) (*weatherd.Bundle, error) {
	return s.loadBundle(ctx, `
SELECT bundle_json
FROM weather_forecast_snapshots
WHERE canonical_location_key = $1
ORDER BY issued_at DESC
LIMIT 1;
`, canonicalLocationKey)
}

func (s *PostgresStore) LatestBundleBefore(ctx context.Context, canonicalLocationKey string, before time.Time) (*weatherd.Bundle, error) {
	return s.loadBundle(ctx, `
SELECT bundle_json
FROM weather_forecast_snapshots
WHERE canonical_location_key = $1
  AND issued_at < $2
ORDER BY issued_at DESC
LIMIT 1;
`, canonicalLocationKey, before.UTC())
}

func (s *PostgresStore) FindCanonicalLocationKeyByRequest(ctx context.Context, req weatherd.Request) (string, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal weather request for lookup: %w", err)
	}
	var key string
	err = s.db.QueryRowContext(ctx, `
SELECT canonical_location_key
FROM weather_refresh_candidates
WHERE request_json = $1
ORDER BY last_requested_at DESC
LIMIT 1;
`, raw).Scan(&key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("query weather request mapping: %w", err)
	}
	return key, nil
}

func (s *PostgresStore) loadBundle(ctx context.Context, query string, args ...any) (*weatherd.Bundle, error) {
	var raw []byte
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query weather snapshot: %w", err)
	}
	var out weatherd.Bundle
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode weather snapshot bundle: %w", err)
	}
	return &out, nil
}

func (s *PostgresStore) SaveVerification(ctx context.Context, result weatherd.VerificationResult) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal weather verification: %w", err)
	}
	now := s.nowFn().UTC()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO weather_yesterday_verifications (
	canonical_location_key,
	verification_date,
	verification_source,
	result_json,
	created_at,
	updated_at
)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (canonical_location_key, verification_date)
DO UPDATE SET
	verification_source = EXCLUDED.verification_source,
	result_json = EXCLUDED.result_json,
	updated_at = EXCLUDED.updated_at;
`,
		result.Provenance.CanonicalLocationKey,
		result.VerificationDate.UTC(),
		result.Provenance.VerificationSource,
		raw,
		now,
	)
	if err != nil {
		return fmt.Errorf("upsert weather verification: %w", err)
	}
	return nil
}

func (s *PostgresStore) LoadVerification(ctx context.Context, canonicalLocationKey string, verificationDate time.Time) (*weatherd.VerificationResult, error) {
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
SELECT result_json
FROM weather_yesterday_verifications
WHERE canonical_location_key = $1
  AND verification_date = $2
LIMIT 1;
`, canonicalLocationKey, verificationDate.UTC()).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query weather verification: %w", err)
	}
	var out weatherd.VerificationResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode weather verification: %w", err)
	}
	return &out, nil
}

func (s *PostgresStore) LoadBiasStates(ctx context.Context, canonicalLocationKey string) ([]weatherd.BiasState, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT canonical_location_key, metric_key, hour_of_day, sample_count, additive_bias, multiplicative_ratio, updated_at
FROM weather_bias_state
WHERE canonical_location_key = $1;
`, canonicalLocationKey)
	if err != nil {
		return nil, fmt.Errorf("query weather bias state: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]weatherd.BiasState, 0, 32)
	for rows.Next() {
		var row weatherd.BiasState
		var metric string
		var additive sql.NullFloat64
		var ratio sql.NullFloat64
		if err := rows.Scan(
			&row.CanonicalLocationKey,
			&metric,
			&row.HourOfDay,
			&row.SampleCount,
			&additive,
			&ratio,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan weather bias state: %w", err)
		}
		row.Metric = weatherd.BiasMetric(metric)
		if additive.Valid {
			v := additive.Float64
			row.AdditiveBias = &v
		}
		if ratio.Valid {
			v := ratio.Float64
			row.MultiplicativeRatio = &v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate weather bias state: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) UpsertBiasStates(ctx context.Context, states []weatherd.BiasState) error {
	now := s.nowFn().UTC()
	for _, state := range states {
		_, err := s.db.ExecContext(ctx, `
INSERT INTO weather_bias_state (
	canonical_location_key,
	metric_key,
	hour_of_day,
	sample_count,
	additive_bias,
	multiplicative_ratio,
	updated_at,
	created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
ON CONFLICT (canonical_location_key, metric_key, hour_of_day)
DO UPDATE SET
	sample_count = EXCLUDED.sample_count,
	additive_bias = EXCLUDED.additive_bias,
	multiplicative_ratio = EXCLUDED.multiplicative_ratio,
	updated_at = EXCLUDED.updated_at;
`,
			state.CanonicalLocationKey,
			string(state.Metric),
			state.HourOfDay,
			state.SampleCount,
			state.AdditiveBias,
			state.MultiplicativeRatio,
			chooseTime(state.UpdatedAt, now),
		)
		if err != nil {
			return fmt.Errorf("upsert weather bias state: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) TouchRefreshCandidate(ctx context.Context, canonicalLocationKey string, req weatherd.Request, requestedAt time.Time) error {
	raw, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal weather refresh candidate request: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO weather_refresh_candidates (
	canonical_location_key,
	request_json,
	last_requested_at,
	last_refreshed_at,
	created_at,
	updated_at
)
VALUES ($1, $2, $3, NULL, $3, $3)
ON CONFLICT (canonical_location_key)
DO UPDATE SET
	request_json = EXCLUDED.request_json,
	last_requested_at = EXCLUDED.last_requested_at,
	updated_at = EXCLUDED.updated_at;
`, canonicalLocationKey, raw, requestedAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert weather refresh candidate: %w", err)
	}
	return nil
}

func (s *PostgresStore) ListRecentRefreshCandidates(ctx context.Context, since time.Time) ([]weatherd.RefreshCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT canonical_location_key, request_json, last_requested_at, last_refreshed_at
FROM weather_refresh_candidates
WHERE last_requested_at >= $1
ORDER BY last_requested_at DESC;
`, since.UTC())
	if err != nil {
		return nil, fmt.Errorf("query weather refresh candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]weatherd.RefreshCandidate, 0, 16)
	for rows.Next() {
		var row weatherd.RefreshCandidate
		var raw []byte
		var lastRefreshed sql.NullTime
		if err := rows.Scan(&row.CanonicalLocationKey, &raw, &row.LastRequestedAt, &lastRefreshed); err != nil {
			return nil, fmt.Errorf("scan weather refresh candidate: %w", err)
		}
		if err := json.Unmarshal(raw, &row.Request); err != nil {
			return nil, fmt.Errorf("decode weather refresh candidate request: %w", err)
		}
		if lastRefreshed.Valid {
			v := lastRefreshed.Time.UTC()
			row.LastRefreshedAt = &v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate weather refresh candidates: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) MarkRefreshCandidateRefreshed(ctx context.Context, canonicalLocationKey string, refreshedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE weather_refresh_candidates
SET last_refreshed_at = $2,
	updated_at = $2
WHERE canonical_location_key = $1;
`, canonicalLocationKey, refreshedAt.UTC())
	if err != nil {
		return fmt.Errorf("mark weather refresh candidate refreshed: %w", err)
	}
	return nil
}

func chooseTime(primary, fallback time.Time) time.Time {
	if !primary.IsZero() {
		return primary.UTC()
	}
	return fallback.UTC()
}
