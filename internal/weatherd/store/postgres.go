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

type WeatherPruneStats struct {
	CompactedSnapshots  int64
	PrunedVerifications int64
	PrunedCandidates    int64
}

type WeatherRetainedCounts struct {
	Snapshots            int64
	VerificationAnchors  int64
	Verifications        int64
	RefreshCandidates    int64
	DueRefreshCandidates int64
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
	var existingIssuedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
SELECT issued_at
FROM weather_forecast_snapshots
WHERE canonical_location_key = $1
ORDER BY issued_at DESC
LIMIT 1;
`, bundle.Provenance.CanonicalLocationKey).Scan(&existingIssuedAt); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query existing weather snapshot: %w", err)
	}
	if existingIssuedAt.Valid && existingIssuedAt.Time.After(bundle.Provenance.IssuedAt.UTC()) {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM weather_forecast_snapshots
WHERE canonical_location_key = $1;
`, bundle.Provenance.CanonicalLocationKey); err != nil {
		return fmt.Errorf("delete existing weather snapshots: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
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
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9);
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
	)
	if err != nil {
		return fmt.Errorf("insert weather snapshot: %w", err)
	}
	verificationDate := nextLocalDayStartUTCForPostgres(bundle)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO weather_verification_forecast_anchors (
	canonical_location_key,
	verification_date,
	issued_at,
	bundle_json,
	created_at,
	updated_at
)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (canonical_location_key, verification_date)
DO UPDATE SET
	issued_at = EXCLUDED.issued_at,
	bundle_json = EXCLUDED.bundle_json,
	updated_at = EXCLUDED.updated_at
WHERE EXCLUDED.issued_at >= weather_verification_forecast_anchors.issued_at;
`,
		bundle.Provenance.CanonicalLocationKey,
		verificationDate.UTC(),
		bundle.Provenance.IssuedAt.UTC(),
		encodedBundle,
		now,
	); err != nil {
		return fmt.Errorf("upsert weather verification forecast anchor: %w", err)
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

func (s *PostgresStore) LoadVerificationForecastAnchor(ctx context.Context, canonicalLocationKey string, verificationDate time.Time) (*weatherd.Bundle, error) {
	return s.loadBundle(ctx, `
SELECT bundle_json
FROM weather_verification_forecast_anchors
WHERE canonical_location_key = $1
  AND verification_date = $2
LIMIT 1;
`, canonicalLocationKey, verificationDate.UTC())
}

func (s *PostgresStore) UpsertVerificationForecastAnchor(ctx context.Context, canonicalLocationKey string, verificationDate time.Time, bundle weatherd.Bundle) error {
	raw, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("marshal weather verification forecast anchor bundle: %w", err)
	}
	now := s.nowFn().UTC()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO weather_verification_forecast_anchors (
	canonical_location_key,
	verification_date,
	issued_at,
	bundle_json,
	created_at,
	updated_at
)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (canonical_location_key, verification_date)
DO UPDATE SET
	issued_at = EXCLUDED.issued_at,
	bundle_json = EXCLUDED.bundle_json,
	updated_at = EXCLUDED.updated_at
WHERE EXCLUDED.issued_at >= weather_verification_forecast_anchors.issued_at;
`,
		canonicalLocationKey,
		verificationDate.UTC(),
		bundle.Provenance.IssuedAt.UTC(),
		raw,
		now,
	)
	if err != nil {
		return fmt.Errorf("upsert weather verification forecast anchor: %w", err)
	}
	return nil
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
	next_refresh_at = COALESCE(weather_refresh_candidates.next_refresh_at, EXCLUDED.last_requested_at),
	updated_at = EXCLUDED.updated_at;
`, canonicalLocationKey, raw, requestedAt.UTC())
	if err != nil {
		return fmt.Errorf("upsert weather refresh candidate: %w", err)
	}
	return nil
}

func (s *PostgresStore) ListRecentRefreshCandidates(ctx context.Context, since time.Time) ([]weatherd.RefreshCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT canonical_location_key, request_json, last_requested_at, last_refreshed_at, next_refresh_at
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
		var nextRefresh sql.NullTime
		if err := rows.Scan(&row.CanonicalLocationKey, &raw, &row.LastRequestedAt, &lastRefreshed, &nextRefresh); err != nil {
			return nil, fmt.Errorf("scan weather refresh candidate: %w", err)
		}
		if err := json.Unmarshal(raw, &row.Request); err != nil {
			return nil, fmt.Errorf("decode weather refresh candidate request: %w", err)
		}
		if lastRefreshed.Valid {
			v := lastRefreshed.Time.UTC()
			row.LastRefreshedAt = &v
		}
		if nextRefresh.Valid {
			v := nextRefresh.Time.UTC()
			row.NextRefreshAt = &v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate weather refresh candidates: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) ListDueRefreshCandidates(ctx context.Context, since, dueBefore time.Time) ([]weatherd.RefreshCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT canonical_location_key, request_json, last_requested_at, last_refreshed_at, next_refresh_at
FROM weather_refresh_candidates
WHERE last_requested_at >= $1
  AND (next_refresh_at IS NULL OR next_refresh_at <= $2)
ORDER BY COALESCE(next_refresh_at, last_requested_at) ASC;
`, since.UTC(), dueBefore.UTC())
	if err != nil {
		return nil, fmt.Errorf("query due weather refresh candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]weatherd.RefreshCandidate, 0, 16)
	for rows.Next() {
		var row weatherd.RefreshCandidate
		var raw []byte
		var lastRefreshed sql.NullTime
		var nextRefresh sql.NullTime
		if err := rows.Scan(&row.CanonicalLocationKey, &raw, &row.LastRequestedAt, &lastRefreshed, &nextRefresh); err != nil {
			return nil, fmt.Errorf("scan due weather refresh candidate: %w", err)
		}
		if err := json.Unmarshal(raw, &row.Request); err != nil {
			return nil, fmt.Errorf("decode due weather refresh candidate request: %w", err)
		}
		if lastRefreshed.Valid {
			v := lastRefreshed.Time.UTC()
			row.LastRefreshedAt = &v
		}
		if nextRefresh.Valid {
			v := nextRefresh.Time.UTC()
			row.NextRefreshAt = &v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due weather refresh candidates: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) MarkRefreshCandidateRefreshed(ctx context.Context, canonicalLocationKey string, refreshedAt, nextRefreshAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE weather_refresh_candidates
SET last_refreshed_at = $2,
	next_refresh_at = $3,
	updated_at = $2
WHERE canonical_location_key = $1;
`, canonicalLocationKey, refreshedAt.UTC(), nextRefreshAt.UTC())
	if err != nil {
		return fmt.Errorf("mark weather refresh candidate refreshed: %w", err)
	}
	return nil
}

func (s *PostgresStore) PruneHotData(ctx context.Context, verificationCutoff, candidateCutoff time.Time) (WeatherPruneStats, error) {
	var stats WeatherPruneStats
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return stats, fmt.Errorf("begin weather prune tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	compacted, err := tx.ExecContext(ctx, `
WITH ranked AS (
    SELECT ctid,
           row_number() OVER (
               PARTITION BY canonical_location_key
               ORDER BY issued_at DESC, updated_at DESC, id DESC
           ) AS row_num
    FROM weather_forecast_snapshots
)
DELETE FROM weather_forecast_snapshots s
USING ranked r
WHERE s.ctid = r.ctid
  AND r.row_num > 1;
`)
	if err != nil {
		return stats, fmt.Errorf("compact weather snapshots: %w", err)
	}
	stats.CompactedSnapshots, err = compacted.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("read compacted weather snapshot count: %w", err)
	}

	prunedVerifications, err := tx.ExecContext(ctx, `
DELETE FROM weather_yesterday_verifications
WHERE verification_date < $1;
`, verificationCutoff.UTC())
	if err != nil {
		return stats, fmt.Errorf("prune weather verifications: %w", err)
	}
	stats.PrunedVerifications, err = prunedVerifications.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("read pruned weather verification count: %w", err)
	}

	prunedCandidates, err := tx.ExecContext(ctx, `
DELETE FROM weather_refresh_candidates
WHERE GREATEST(
    COALESCE(last_requested_at, TIMESTAMPTZ 'epoch'),
    COALESCE(last_refreshed_at, TIMESTAMPTZ 'epoch')
) < $1;
`, candidateCutoff.UTC())
	if err != nil {
		return stats, fmt.Errorf("prune weather refresh candidates: %w", err)
	}
	stats.PrunedCandidates, err = prunedCandidates.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("read pruned weather refresh candidate count: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM weather_verification_forecast_anchors
WHERE verification_date < $1;
`, verificationCutoff.UTC()); err != nil {
		return stats, fmt.Errorf("prune weather verification forecast anchors: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM weather_forecast_snapshots s
WHERE s.issued_at < $1
  AND NOT EXISTS (
      SELECT 1
      FROM weather_refresh_candidates c
      WHERE c.canonical_location_key = s.canonical_location_key
        AND c.last_requested_at >= $1
  );
`, candidateCutoff.UTC()); err != nil {
		return stats, fmt.Errorf("prune inactive weather snapshots: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("commit weather prune tx: %w", err)
	}
	return stats, nil
}

func (s *PostgresStore) CountHotData(ctx context.Context, dueBefore time.Time) (WeatherRetainedCounts, error) {
	var counts WeatherRetainedCounts
	err := s.db.QueryRowContext(ctx, `
SELECT
    (SELECT count(*) FROM weather_forecast_snapshots),
    (SELECT count(*) FROM weather_verification_forecast_anchors),
    (SELECT count(*) FROM weather_yesterday_verifications),
    (SELECT count(*) FROM weather_refresh_candidates),
    (SELECT count(*) FROM weather_refresh_candidates WHERE next_refresh_at IS NULL OR next_refresh_at <= $1);
`, dueBefore.UTC()).Scan(
		&counts.Snapshots,
		&counts.VerificationAnchors,
		&counts.Verifications,
		&counts.RefreshCandidates,
		&counts.DueRefreshCandidates,
	)
	if err != nil {
		return counts, fmt.Errorf("count retained weather hot data: %w", err)
	}
	return counts, nil
}

func nextLocalDayStartUTCForPostgres(bundle weatherd.Bundle) time.Time {
	loc := time.UTC
	if bundle.Provenance.Timezone != "" {
		if loaded, err := time.LoadLocation(bundle.Provenance.Timezone); err == nil {
			loc = loaded
		}
	}
	issuedLocal := bundle.Provenance.IssuedAt.In(loc)
	nextDay := time.Date(issuedLocal.Year(), issuedLocal.Month(), issuedLocal.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	return nextDay.UTC()
}

func chooseTime(primary, fallback time.Time) time.Time {
	if !primary.IsZero() {
		return primary.UTC()
	}
	return fallback.UTC()
}
