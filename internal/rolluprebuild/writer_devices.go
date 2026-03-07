package rolluprebuild

import (
	"context"
	"fmt"
	"strings"
)

func (w *PostgresWriter) ResolveDeviceMappings(ctx context.Context, provider string, deviceIDs []string, providerDeviceIDs []string) (map[string]string, error) {
	if w == nil || w.pool == nil {
		return nil, fmt.Errorf("rollup rebuild postgres writer is not initialized")
	}
	deviceIDs = normalizeTextList(deviceIDs, false)
	providerDeviceIDs = normalizeTextList(providerDeviceIDs, true)
	if len(deviceIDs) == 0 && len(providerDeviceIDs) == 0 {
		return nil, fmt.Errorf("at least one device filter is required")
	}

	rows, err := w.pool.Query(ctx, `
SELECT d.id::text, UPPER(pd.provider_device_id)
FROM provider_devices pd
JOIN devices d ON d.id = pd.device_id
WHERE ($1::text = '' OR pd.provider = $1::text)
  AND (
    (COALESCE(cardinality($2::text[]), 0) > 0 AND d.id::text = ANY($2::text[]))
    OR
    (COALESCE(cardinality($3::text[]), 0) > 0 AND UPPER(pd.provider_device_id) = ANY($3::text[]))
  )
`, strings.TrimSpace(provider), deviceIDs, providerDeviceIDs)
	if err != nil {
		return nil, fmt.Errorf("query provider device mappings: %w", err)
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var deviceID string
		var providerDeviceID string
		if err := rows.Scan(&deviceID, &providerDeviceID); err != nil {
			return nil, fmt.Errorf("scan provider device mapping: %w", err)
		}
		if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(providerDeviceID) == "" {
			continue
		}
		out[strings.ToUpper(strings.TrimSpace(providerDeviceID))] = strings.TrimSpace(deviceID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider device mappings: %w", err)
	}
	return out, nil
}

func normalizeTextList(values []string, upper bool) []string {
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
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
