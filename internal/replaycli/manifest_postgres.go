package replaycli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresManifestStore struct {
	pool *pgxpool.Pool
}

func NewPostgresManifestStore(dsn string) (*PostgresManifestStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("manifest postgres dsn is required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse manifest postgres dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("open manifest postgres pool: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping manifest postgres: %w", err)
	}
	return &PostgresManifestStore{pool: pool}, nil
}

func (s *PostgresManifestStore) Close() error {
	if s == nil || s.pool == nil {
		return nil
	}
	s.pool.Close()
	return nil
}

func (s *PostgresManifestStore) ListByDevices(ctx context.Context, query DeviceQuery) ([]ManifestObject, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("manifest postgres store is not initialized")
	}
	if query.FromUnixMS <= 0 || query.ToUnixMS <= 0 || query.FromUnixMS > query.ToUnixMS {
		return nil, errors.New("invalid manifest device query window")
	}
	deviceIDs := normalizeStrings(query.DeviceIDs, false)
	providerDeviceIDs := normalizeStrings(query.ProviderDeviceIDs, true)
	if len(deviceIDs) == 0 && len(providerDeviceIDs) == 0 {
		return nil, errors.New("manifest device query requires at least one id filter")
	}
	sql := `
SELECT provider, shard, shard_count, partition_hour, ts_min_unix_ms, ts_max_unix_ms, object_bucket, object_key, device_ids, provider_device_ids
FROM archive_object_manifest
WHERE ts_max_unix_ms >= $1
  AND ts_min_unix_ms <= $2
  AND (
    (COALESCE(cardinality($3::text[]), 0) > 0 AND device_ids && $3::text[])
    OR
    (COALESCE(cardinality($4::text[]), 0) > 0 AND provider_device_ids && $4::text[])
  )
ORDER BY partition_hour ASC, shard ASC, object_key ASC
`
	args := []any{query.FromUnixMS, query.ToUnixMS, deviceIDs, providerDeviceIDs}
	if query.MaxObjectsReturned > 0 {
		sql += "LIMIT $5"
		args = append(args, query.MaxObjectsReturned)
	}
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query manifest by device filters: %w", err)
	}
	defer rows.Close()
	return scanManifestRows(rows)
}

func (s *PostgresManifestStore) ListByFleetRange(ctx context.Context, query FleetQuery) ([]ManifestObject, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("manifest postgres store is not initialized")
	}
	if query.FromUnixMS <= 0 || query.ToUnixMS <= 0 || query.FromUnixMS > query.ToUnixMS {
		return nil, errors.New("invalid manifest fleet query window")
	}
	var (
		sql = `
SELECT provider, shard, shard_count, partition_hour, ts_min_unix_ms, ts_max_unix_ms, object_bucket, object_key, device_ids, provider_device_ids
FROM archive_object_manifest
WHERE ts_max_unix_ms >= $1
  AND ts_min_unix_ms <= $2
`
		args []any
	)
	args = append(args, query.FromUnixMS, query.ToUnixMS)
	if len(query.Shards) > 0 {
		shards, err := uint32SliceToInt32(query.Shards, "manifest shard filter")
		if err != nil {
			return nil, err
		}
		sql += "  AND shard = ANY($3::integer[])\n"
		args = append(args, shards)
	}
	sql += "ORDER BY partition_hour ASC, shard ASC, object_key ASC\n"
	if query.MaxObjectsReturned > 0 {
		sql += fmt.Sprintf("LIMIT $%d", len(args)+1)
		args = append(args, query.MaxObjectsReturned)
	}
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query manifest by fleet range: %w", err)
	}
	defer rows.Close()
	return scanManifestRows(rows)
}

func scanManifestRows(rows pgx.Rows) ([]ManifestObject, error) {
	out := make([]ManifestObject, 0, 128)
	for rows.Next() {
		var (
			record          ManifestObject
			shard           int32
			shardCount      int32
			deviceIDs       []string
			providerDevices []string
		)
		if err := rows.Scan(
			&record.Provider,
			&shard,
			&shardCount,
			&record.PartitionHour,
			&record.TSMinUnixMS,
			&record.TSMaxUnixMS,
			&record.ObjectBucket,
			&record.ObjectKey,
			&deviceIDs,
			&providerDevices,
		); err != nil {
			return nil, fmt.Errorf("scan manifest row: %w", err)
		}
		if shard < 0 || shardCount < 0 {
			return nil, fmt.Errorf("invalid shard values shard=%d shard_count=%d", shard, shardCount)
		}
		record.Shard = uint32(shard)
		record.ShardCount = uint32(shardCount)
		record.DeviceIDs = normalizeStrings(deviceIDs, false)
		record.ProviderDeviceIDs = normalizeStrings(providerDevices, true)
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate manifest rows: %w", err)
	}
	return out, nil
}

func uint32SliceToInt32(values []uint32, field string) ([]int32, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]int32, 0, len(values))
	for _, value := range values {
		if value > 2147483647 {
			return nil, fmt.Errorf("%s exceeds int32 bounds: %d", field, value)
		}
		out = append(out, int32(value))
	}
	return out, nil
}
