package controlplane

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrSchemaNotReady = errors.New("control-plane schema not ready")

const providerConfigColumnSchemaQuery = `
WITH resolved_provider_credentials AS (
	SELECT to_regclass('provider_credentials') AS relation_oid
)
SELECT a.atttypid::regtype::text AS data_type,
	CASE WHEN a.attnotnull THEN 'NO' ELSE 'YES' END AS is_nullable
FROM resolved_provider_credentials rpc
JOIN pg_attribute a ON a.attrelid = rpc.relation_oid
WHERE a.attnum > 0
  AND NOT a.attisdropped
  AND a.attname = 'provider_config';
`

const providerDeviceUniqueConstraintsSchemaQuery = `
WITH resolved_provider_devices AS (
	SELECT to_regclass('provider_devices') AS relation_oid
)
SELECT c.conname
FROM resolved_provider_devices rpd
JOIN pg_constraint c ON c.conrelid = rpd.relation_oid
WHERE c.contype = 'u'
  AND c.conname IN (
	'uq_provider_devices_provider_device_id',
	'uq_provider_devices_device_provider'
  )
ORDER BY c.conname;
`

const deviceEcoflowSNUniqueIndexSchemaQuery = `
WITH resolved_devices AS (
	SELECT to_regclass('devices') AS relation_oid
)
SELECT COUNT(*)::int
FROM resolved_devices rd
JOIN pg_index i ON i.indrelid = rd.relation_oid
WHERE i.indisunique
  AND i.indisvalid
  AND i.indpred IS NULL
  AND (
	SELECT array_agg(a.attname::text ORDER BY ord.n)
	FROM unnest(i.indkey) WITH ORDINALITY AS ord(attnum, n)
	JOIN pg_attribute a ON a.attrelid = rd.relation_oid AND a.attnum = ord.attnum
	WHERE ord.n <= i.indnkeyatts
  ) = ARRAY['ecoflow_sn'];
`

const userDeviceUniqueIndexSchemaQuery = `
WITH resolved_user_devices AS (
	SELECT to_regclass('user_devices') AS relation_oid
)
SELECT COUNT(*)::int
FROM resolved_user_devices rud
JOIN pg_index i ON i.indrelid = rud.relation_oid
WHERE i.indisunique
  AND i.indisvalid
  AND i.indpred IS NULL
  AND (
	SELECT array_agg(a.attname::text ORDER BY ord.n)
	FROM unnest(i.indkey) WITH ORDINALITY AS ord(attnum, n)
	JOIN pg_attribute a ON a.attrelid = rud.relation_oid AND a.attnum = ord.attnum
	WHERE ord.n <= i.indnkeyatts
  ) = ARRAY['user_id', 'device_id'];
`

const edgeCollectorsSchemaQuery = `
SELECT COUNT(*)::int
FROM (
	VALUES ('edge_collectors'::text), ('edge_device_sources'::text)
) AS required(table_name)
WHERE to_regclass(required.table_name) IS NOT NULL;
`

// RequireCurrentSchema fails before workers start if the database is older than
// the control-plane queries compiled into this binary.
func (s *PostgresStore) RequireCurrentSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("postgres store is not initialized")
	}
	var dataType, nullable string
	err := s.db.QueryRowContext(ctx, providerConfigColumnSchemaQuery).Scan(&dataType, &nullable)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: provider_credentials.provider_config column is missing", ErrSchemaNotReady)
	}
	if err != nil {
		return fmt.Errorf("check provider_credentials.provider_config schema: %w", err)
	}
	if dataType != "jsonb" || nullable != "NO" {
		return fmt.Errorf("%w: provider_credentials.provider_config has data_type=%s is_nullable=%s, want jsonb/NO", ErrSchemaNotReady, dataType, nullable)
	}
	rows, err := s.db.QueryContext(ctx, providerDeviceUniqueConstraintsSchemaQuery)
	if err != nil {
		return fmt.Errorf("check provider_devices unique constraints: %w", err)
	}
	defer func() { _ = rows.Close() }()

	gotConstraints := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan provider_devices unique constraint: %w", err)
		}
		gotConstraints[name] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate provider_devices unique constraints: %w", err)
	}
	for _, name := range []string{
		"uq_provider_devices_provider_device_id",
		"uq_provider_devices_device_provider",
	} {
		if !gotConstraints[name] {
			return fmt.Errorf("%w: provider_devices.%s unique constraint is missing", ErrSchemaNotReady, name)
		}
	}
	var deviceSerialUniqueIndexCount int
	err = s.db.QueryRowContext(ctx, deviceEcoflowSNUniqueIndexSchemaQuery).Scan(&deviceSerialUniqueIndexCount)
	if err != nil {
		return fmt.Errorf("check devices.ecoflow_sn unique index: %w", err)
	}
	if deviceSerialUniqueIndexCount < 1 {
		return fmt.Errorf("%w: devices.ecoflow_sn unique constraint is missing", ErrSchemaNotReady)
	}
	var userDeviceUniqueIndexCount int
	err = s.db.QueryRowContext(ctx, userDeviceUniqueIndexSchemaQuery).Scan(&userDeviceUniqueIndexCount)
	if err != nil {
		return fmt.Errorf("check user_devices unique index: %w", err)
	}
	if userDeviceUniqueIndexCount < 1 {
		return fmt.Errorf("%w: user_devices(user_id, device_id) unique constraint is missing", ErrSchemaNotReady)
	}
	var edgeTableCount int
	err = s.db.QueryRowContext(ctx, edgeCollectorsSchemaQuery).Scan(&edgeTableCount)
	if err != nil {
		return fmt.Errorf("check edge collector schema: %w", err)
	}
	if edgeTableCount != 2 {
		return fmt.Errorf("%w: edge collector tables are missing", ErrSchemaNotReady)
	}
	return nil
}
