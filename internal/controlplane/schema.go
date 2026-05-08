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
	return nil
}
