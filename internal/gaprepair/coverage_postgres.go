package gaprepair

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jpaljasma/ecoflow-pulse/internal/dbpool"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
)

type PostgresCoverageStore struct {
	pool *pgxpool.Pool
}

func NewPostgresCoverageStore(dsn string) (*PostgresCoverageStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("gap coverage postgres dsn is required")
	}
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, fmt.Errorf("apply gap coverage postgres search_path: %w", err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse gap coverage postgres dsn: %w", err)
	}
	dbpool.ConfigurePGX(cfg)
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("open gap coverage postgres pool: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping gap coverage postgres: %w", err)
	}
	return &PostgresCoverageStore{pool: pool}, nil
}

func (s *PostgresCoverageStore) Close() error {
	if s == nil || s.pool == nil {
		return nil
	}
	s.pool.Close()
	return nil
}

func (s *PostgresCoverageStore) CoverageByProviderDevices(
	ctx context.Context,
	provider string,
	providerDeviceIDs []string,
	fromUnixMS int64,
	toUnixMS int64,
) (map[string]CoverageWindow, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("gap coverage store is not initialized")
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return nil, errors.New("provider is required")
	}
	if fromUnixMS <= 0 || toUnixMS <= 0 || fromUnixMS > toUnixMS {
		return nil, errors.New("invalid coverage query window")
	}
	ids := normalizeProviderDeviceIDs(providerDeviceIDs)
	if len(ids) == 0 {
		return map[string]CoverageWindow{}, nil
	}

	rows, err := s.pool.Query(ctx, `
SELECT
  pd.provider_device_id,
  MIN(m.ts_min_unix_ms) AS min_ts,
  MAX(m.ts_max_unix_ms) AS max_ts,
  COUNT(*) AS object_count
FROM archive_object_manifest m
JOIN LATERAL unnest(m.provider_device_ids) pd(provider_device_id) ON true
WHERE m.provider = $1
  AND m.ts_max_unix_ms >= $2
  AND m.ts_min_unix_ms <= $3
  AND pd.provider_device_id = ANY($4::text[])
GROUP BY pd.provider_device_id
`, provider, fromUnixMS, toUnixMS, ids)
	if err != nil {
		return nil, fmt.Errorf("query gap coverage windows: %w", err)
	}
	defer rows.Close()

	out := make(map[string]CoverageWindow, len(ids))
	for rows.Next() {
		var (
			providerID  string
			minTS       int64
			maxTS       int64
			objectCount int64
		)
		if err := rows.Scan(&providerID, &minTS, &maxTS, &objectCount); err != nil {
			return nil, fmt.Errorf("scan gap coverage row: %w", err)
		}
		if objectCount < 0 || objectCount > math.MaxInt {
			return nil, fmt.Errorf("gap coverage object_count out of range: %d", objectCount)
		}
		providerID = strings.ToUpper(strings.TrimSpace(providerID))
		if providerID == "" {
			continue
		}
		out[providerID] = CoverageWindow{
			ProviderDeviceID: providerID,
			MinUnixMS:        minTS,
			MaxUnixMS:        maxTS,
			ObjectCount:      int(objectCount),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate gap coverage rows: %w", err)
	}
	return out, nil
}

func normalizeProviderDeviceIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		clean := strings.ToUpper(strings.TrimSpace(raw))
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	slices.Sort(out)
	return out
}
