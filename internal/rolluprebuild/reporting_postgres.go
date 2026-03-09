package rolluprebuild

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ReportFilter struct {
	Provider          string
	DeviceIDs         []string
	ProviderDeviceIDs []string
	From              time.Time
	To                time.Time
}

type ArchiveFootprint struct {
	Objects         int
	TotalBytes      int64
	TotalRecords    int
	ProviderDevices int
	WindowTSMinMS   int64
	WindowTSMaxMS   int64
}

type BucketWindowSummary struct {
	ProviderDeviceID string
	TotalBuckets     int
	LatestBucketUTC  string
	CurrentWh        float64
}

func (w *PostgresWriter) ArchiveFootprint(ctx context.Context, filter ReportFilter) (ArchiveFootprint, error) {
	if w == nil || w.pool == nil {
		return ArchiveFootprint{}, fmt.Errorf("rollup rebuild postgres writer is not initialized")
	}
	filter = normalizeReportFilter(filter)
	sql := `
SELECT
  COUNT(*)::int,
  COALESCE(SUM(object_size_bytes), 0)::bigint,
  COALESCE(SUM(record_count), 0)::int,
  COUNT(DISTINCT provider_device_id)::int,
  COALESCE(MIN(ts_min_unix_ms), 0)::bigint,
  COALESCE(MAX(ts_max_unix_ms), 0)::bigint
FROM archive_object_manifest
WHERE ts_max_unix_ms >= $1
  AND ts_min_unix_ms <= $2
  AND ($3::text = '' OR provider = $3::text)
  AND (COALESCE(cardinality($4::text[]), 0) = 0 OR device_ids && $4::text[])
  AND (COALESCE(cardinality($5::text[]), 0) = 0 OR provider_device_ids && $5::text[])
`
	var out ArchiveFootprint
	err := w.pool.QueryRow(
		ctx,
		sql,
		filter.From.UnixMilli(),
		filter.To.UnixMilli(),
		filter.Provider,
		filter.DeviceIDs,
		filter.ProviderDeviceIDs,
	).Scan(
		&out.Objects,
		&out.TotalBytes,
		&out.TotalRecords,
		&out.ProviderDevices,
		&out.WindowTSMinMS,
		&out.WindowTSMaxMS,
	)
	if err != nil {
		return ArchiveFootprint{}, fmt.Errorf("query archive footprint: %w", err)
	}
	return out, nil
}

func (w *PostgresWriter) MinuteWindowSummary(ctx context.Context, filter ReportFilter) (map[string]BucketWindowSummary, error) {
	if w == nil || w.pool == nil {
		return nil, fmt.Errorf("rollup rebuild postgres writer is not initialized")
	}
	filter = normalizeReportFilter(filter)
	sql := `
SELECT
  provider_device_id,
  COUNT(*)::int,
  COALESCE(to_char(MAX(bucket_start) AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), 'n/a'),
  COALESCE(SUM(COALESCE(solar_generated_wh, CASE WHEN COALESCE(pv_avg_w, 0) > 0 THEN pv_avg_w / 60.0 ELSE 0 END)), 0)::double precision
FROM telemetry_rollup_minute
WHERE bucket_start >= $1
  AND bucket_start < $2
  AND ($3::text = '' OR provider = $3::text)
  AND (COALESCE(cardinality($4::text[]), 0) = 0 OR device_id::text = ANY($4::text[]))
  AND (COALESCE(cardinality($5::text[]), 0) = 0 OR provider_device_id = ANY($5::text[]))
GROUP BY provider_device_id
ORDER BY provider_device_id
`
	rows, err := w.pool.Query(
		ctx,
		sql,
		filter.From,
		filter.To,
		filter.Provider,
		filter.DeviceIDs,
		filter.ProviderDeviceIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("query minute window summary: %w", err)
	}
	defer rows.Close()

	out := map[string]BucketWindowSummary{}
	for rows.Next() {
		var row BucketWindowSummary
		if err := rows.Scan(&row.ProviderDeviceID, &row.TotalBuckets, &row.LatestBucketUTC, &row.CurrentWh); err != nil {
			return nil, fmt.Errorf("scan minute window summary: %w", err)
		}
		out[row.ProviderDeviceID] = row
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate minute window summary: %w", err)
	}
	return out, nil
}

func normalizeReportFilter(in ReportFilter) ReportFilter {
	return ReportFilter{
		Provider:          strings.TrimSpace(in.Provider),
		DeviceIDs:         normalizeStrings(in.DeviceIDs, false),
		ProviderDeviceIDs: normalizeStrings(in.ProviderDeviceIDs, true),
		From:              in.From.UTC(),
		To:                in.To.UTC(),
	}
}

func normalizeStrings(values []string, upper bool) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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
	return out
}
