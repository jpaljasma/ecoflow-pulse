package archiveworker

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultManifestCompression = "zstd"
	defaultManifestProvider    = "unknown"
)

type ManifestRecord struct {
	Provider          string
	Shard             uint32
	ShardCount        uint32
	PartitionHour     time.Time
	TSMinUnixMS       int64
	TSMaxUnixMS       int64
	RecordCount       int
	ObjectBucket      string
	ObjectKey         string
	ObjectSizeBytes   int64
	ContentType       string
	Compression       string
	ChecksumCRC32     string
	WriterID          string
	DeviceIDs         []string
	ProviderDeviceIDs []string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ManifestStore interface {
	UpsertObjectManifest(ctx context.Context, record ManifestRecord) error
	Close() error
}

type PostgresManifestStore struct {
	pool *pgxpool.Pool
	now  func() time.Time
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
	return &PostgresManifestStore{
		pool: pool,
		now:  utcNow,
	}, nil
}

func (s *PostgresManifestStore) Close() error {
	if s == nil || s.pool == nil {
		return nil
	}
	s.pool.Close()
	return nil
}

func (s *PostgresManifestStore) UpsertObjectManifest(ctx context.Context, record ManifestRecord) error {
	if s == nil || s.pool == nil {
		return errors.New("manifest postgres store is not initialized")
	}
	record = normalizeManifestRecord(record, s.now())
	if err := validateManifestRecord(record); err != nil {
		return err
	}
	const query = `
INSERT INTO archive_object_manifest (
	provider,
	shard,
	shard_count,
	partition_hour,
	ts_min_unix_ms,
	ts_max_unix_ms,
	record_count,
	object_bucket,
	object_key,
	object_size_bytes,
	content_type,
	compression,
	checksum_crc32,
	writer_id,
	device_ids,
	provider_device_ids,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
	$11, $12, $13, $14, $15, $16, $17, $18
)
ON CONFLICT (object_bucket, object_key) DO UPDATE
SET provider = EXCLUDED.provider,
	shard = EXCLUDED.shard,
	shard_count = EXCLUDED.shard_count,
	partition_hour = EXCLUDED.partition_hour,
	ts_min_unix_ms = EXCLUDED.ts_min_unix_ms,
	ts_max_unix_ms = EXCLUDED.ts_max_unix_ms,
	record_count = EXCLUDED.record_count,
	object_size_bytes = EXCLUDED.object_size_bytes,
	content_type = EXCLUDED.content_type,
	compression = EXCLUDED.compression,
	checksum_crc32 = EXCLUDED.checksum_crc32,
	writer_id = EXCLUDED.writer_id,
	device_ids = EXCLUDED.device_ids,
	provider_device_ids = EXCLUDED.provider_device_ids,
	updated_at = EXCLUDED.updated_at;
`
	_, err := s.pool.Exec(
		ctx,
		query,
		record.Provider,
		int32(record.Shard),
		int32(record.ShardCount),
		record.PartitionHour,
		record.TSMinUnixMS,
		record.TSMaxUnixMS,
		int32(record.RecordCount),
		record.ObjectBucket,
		record.ObjectKey,
		record.ObjectSizeBytes,
		record.ContentType,
		record.Compression,
		record.ChecksumCRC32,
		record.WriterID,
		record.DeviceIDs,
		record.ProviderDeviceIDs,
		record.CreatedAt,
		record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert archive object manifest %s/%s: %w", record.ObjectBucket, record.ObjectKey, err)
	}
	return nil
}

func normalizeManifestRecord(in ManifestRecord, now time.Time) ManifestRecord {
	out := in
	out.Provider = strings.ToLower(strings.TrimSpace(out.Provider))
	if out.Provider == "" {
		out.Provider = defaultManifestProvider
	}
	out.PartitionHour = out.PartitionHour.UTC().Truncate(time.Hour)
	out.ObjectBucket = strings.TrimSpace(out.ObjectBucket)
	out.ObjectKey = strings.Trim(strings.TrimSpace(out.ObjectKey), "/")
	out.ContentType = strings.TrimSpace(out.ContentType)
	if out.ContentType == "" {
		out.ContentType = defaultObjectContentType
	}
	out.Compression = strings.ToLower(strings.TrimSpace(out.Compression))
	if out.Compression == "" {
		out.Compression = defaultManifestCompression
	}
	out.ChecksumCRC32 = strings.ToLower(strings.TrimSpace(out.ChecksumCRC32))
	out.WriterID = strings.TrimSpace(out.WriterID)
	out.DeviceIDs = normalizeStringSet(out.DeviceIDs, false)
	out.ProviderDeviceIDs = normalizeStringSet(out.ProviderDeviceIDs, true)

	now = now.UTC()
	if out.CreatedAt.IsZero() {
		out.CreatedAt = now
	} else {
		out.CreatedAt = out.CreatedAt.UTC()
	}
	if out.UpdatedAt.IsZero() {
		out.UpdatedAt = now
	} else {
		out.UpdatedAt = out.UpdatedAt.UTC()
	}
	return out
}

func validateManifestRecord(record ManifestRecord) error {
	switch {
	case strings.TrimSpace(record.Provider) == "":
		return errors.New("manifest provider is required")
	case strings.TrimSpace(record.ObjectBucket) == "":
		return errors.New("manifest object bucket is required")
	case strings.TrimSpace(record.ObjectKey) == "":
		return errors.New("manifest object key is required")
	case strings.TrimSpace(record.WriterID) == "":
		return errors.New("manifest writer id is required")
	case record.ShardCount == 0:
		return errors.New("manifest shard count must be positive")
	case record.Shard >= record.ShardCount:
		return errors.New("manifest shard must be less than shard_count")
	case record.RecordCount <= 0:
		return errors.New("manifest record count must be positive")
	case record.ObjectSizeBytes <= 0:
		return errors.New("manifest object size must be positive")
	case record.TSMinUnixMS <= 0 || record.TSMaxUnixMS <= 0:
		return errors.New("manifest timestamps must be positive")
	case record.TSMinUnixMS > record.TSMaxUnixMS:
		return errors.New("manifest timestamps must be ordered")
	case record.PartitionHour.IsZero():
		return errors.New("manifest partition hour is required")
	}
	return nil
}

func normalizeStringSet(values []string, upper bool) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if upper {
			normalized = strings.ToUpper(normalized)
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

func utcNow() time.Time {
	return time.Now().UTC()
}
