package controlplane

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *PostgresStore) CreateEdgeCollector(ctx context.Context, in CreateEdgeCollectorInput) (EdgeCollector, error) {
	now := normalizeWriteTime(s.now())
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		displayName = "Pulse Edge Collector"
	}
	query := `
INSERT INTO edge_collectors (
	user_id,
	display_name,
	setup_token_hash,
	is_active,
	created_at,
	updated_at
)
SELECT u.id, $2, $3, false, $4, $4
FROM users u
WHERE u.keycloak_subject = $1
RETURNING id::text, user_id::text, display_name, setup_token_hash, COALESCE(collector_secret_hash, ''),
	is_active, COALESCE(collector_version, ''), COALESCE(hostname, ''), last_heartbeat_at, created_at, updated_at;
`
	row, err := scanEdgeCollector(s.db.QueryRowContext(ctx, query, in.UserSubject, displayName, strings.TrimSpace(in.SetupTokenHash), now))
	if errors.Is(err, sql.ErrNoRows) {
		return EdgeCollector{}, ErrUserNotFound
	}
	if err != nil {
		return EdgeCollector{}, fmt.Errorf("insert edge collector: %w", err)
	}
	return row, nil
}

func (s *PostgresStore) ListEdgeCollectors(ctx context.Context, in ListEdgeCollectorsInput) ([]EdgeCollector, error) {
	query := `
SELECT ec.id::text, ec.user_id::text, ec.display_name, ec.setup_token_hash, COALESCE(ec.collector_secret_hash, ''),
	ec.is_active, COALESCE(ec.collector_version, ''), COALESCE(ec.hostname, ''), ec.last_heartbeat_at, ec.created_at, ec.updated_at
FROM edge_collectors ec
JOIN users u ON u.id = ec.user_id
WHERE u.keycloak_subject = $1
ORDER BY ec.created_at DESC, ec.id DESC;
`
	rows, err := s.db.QueryContext(ctx, query, in.UserSubject)
	if err != nil {
		return nil, fmt.Errorf("query edge collectors: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]EdgeCollector, 0, 4)
	for rows.Next() {
		row, err := scanEdgeCollector(rows)
		if err != nil {
			return nil, fmt.Errorf("scan edge collector row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate edge collectors: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) EnrollEdgeCollector(ctx context.Context, in EnrollEdgeCollectorInput) (EdgeCollector, error) {
	now := normalizeWriteTime(s.now())
	query := `
UPDATE edge_collectors
SET setup_token_hash = '',
    collector_secret_hash = $2,
    collector_version = $3,
    hostname = $4,
    is_active = true,
    last_heartbeat_at = $5,
    updated_at = $5
WHERE setup_token_hash = $1
  AND is_active = false
RETURNING id::text, user_id::text, display_name, setup_token_hash, COALESCE(collector_secret_hash, ''),
	is_active, COALESCE(collector_version, ''), COALESCE(hostname, ''), last_heartbeat_at, created_at, updated_at;
`
	row, err := scanEdgeCollector(s.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(in.SetupTokenHash),
		strings.TrimSpace(in.CollectorSecretHash),
		strings.TrimSpace(in.CollectorVersion),
		strings.TrimSpace(in.Hostname),
		now,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return EdgeCollector{}, ErrEdgeCollectorNotFound
	}
	if err != nil {
		return EdgeCollector{}, fmt.Errorf("enroll edge collector: %w", err)
	}
	return row, nil
}

func (s *PostgresStore) AuthenticateEdgeCollector(ctx context.Context, in AuthenticateEdgeCollectorInput) (EdgeCollector, error) {
	query := `
SELECT id::text, user_id::text, display_name, setup_token_hash, COALESCE(collector_secret_hash, ''),
	is_active, COALESCE(collector_version, ''), COALESCE(hostname, ''), last_heartbeat_at, created_at, updated_at
FROM edge_collectors
WHERE collector_secret_hash = $1
  AND is_active = true;
`
	row, err := scanEdgeCollector(s.db.QueryRowContext(ctx, query, strings.TrimSpace(in.CollectorSecretHash)))
	if errors.Is(err, sql.ErrNoRows) {
		return EdgeCollector{}, ErrEdgeCollectorNotFound
	}
	if err != nil {
		return EdgeCollector{}, fmt.Errorf("authenticate edge collector: %w", err)
	}
	return row, nil
}

func (s *PostgresStore) UpdateEdgeCollectorHeartbeat(ctx context.Context, in UpdateEdgeCollectorHeartbeatInput) (EdgeCollector, error) {
	now := normalizeWriteTime(s.now())
	query := `
UPDATE edge_collectors
SET collector_version = CASE WHEN $2 = '' THEN collector_version ELSE $2 END,
    hostname = CASE WHEN $3 = '' THEN hostname ELSE $3 END,
    last_heartbeat_at = $4,
    updated_at = $4
WHERE id = $1::uuid
  AND is_active = true
RETURNING id::text, user_id::text, display_name, setup_token_hash, COALESCE(collector_secret_hash, ''),
	is_active, COALESCE(collector_version, ''), COALESCE(hostname, ''), last_heartbeat_at, created_at, updated_at;
`
	row, err := scanEdgeCollector(s.db.QueryRowContext(ctx, query, in.CollectorID, strings.TrimSpace(in.CollectorVersion), strings.TrimSpace(in.Hostname), now))
	if errors.Is(err, sql.ErrNoRows) {
		return EdgeCollector{}, ErrEdgeCollectorNotFound
	}
	if err != nil {
		return EdgeCollector{}, fmt.Errorf("update edge collector heartbeat: %w", err)
	}
	return row, nil
}

func (s *PostgresStore) UpsertEdgeDeviceSource(ctx context.Context, in UpsertEdgeDeviceSourceInput) (EdgeDeviceSource, error) {
	collector, err := s.edgeCollectorByID(ctx, in.CollectorID)
	if err != nil {
		return EdgeDeviceSource{}, err
	}
	if !collector.IsActive {
		return EdgeDeviceSource{}, ErrEdgeCollectorNotFound
	}
	now := normalizeWriteTime(s.now())
	observedAt := normalizeWriteTime(in.ObservedAt)
	if observedAt.IsZero() {
		observedAt = now
	}
	metadataJSON, err := marshalEdgeMetadata(in.Metadata)
	if err != nil {
		return EdgeDeviceSource{}, fmt.Errorf("marshal edge source metadata: %w", err)
	}
	query := `
INSERT INTO edge_device_sources (
	collector_id,
	user_id,
	provider,
	transport,
	provider_device_id,
	display_name,
	model,
	address_hash,
	rssi_dbm,
	metadata,
	status,
	last_seen_at,
	created_at,
	updated_at
)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, 'pending', $11, $12, $12)
ON CONFLICT (collector_id, provider, transport, provider_device_id)
DO UPDATE SET
	display_name = EXCLUDED.display_name,
	model = EXCLUDED.model,
	address_hash = EXCLUDED.address_hash,
	rssi_dbm = EXCLUDED.rssi_dbm,
	metadata = EXCLUDED.metadata,
	last_seen_at = EXCLUDED.last_seen_at,
	updated_at = EXCLUDED.updated_at
RETURNING id::text, collector_id::text, user_id::text, provider, transport, provider_device_id,
	COALESCE(display_name, ''), COALESCE(model, ''), COALESCE(address_hash, ''), rssi_dbm,
	metadata, status, COALESCE(linked_device_id::text, ''), last_seen_at, created_at, updated_at;
`
	row, err := scanEdgeDeviceSource(s.db.QueryRowContext(
		ctx,
		query,
		collector.ID,
		collector.UserID,
		NormalizeProvider(in.Provider),
		normalizeEdgeTransport(in.Transport),
		strings.ToUpper(strings.TrimSpace(in.ProviderDeviceID)),
		strings.TrimSpace(in.DisplayName),
		strings.TrimSpace(in.Model),
		strings.TrimSpace(in.AddressHash),
		in.RSSIDBm,
		metadataJSON,
		observedAt,
		now,
	))
	if err != nil {
		return EdgeDeviceSource{}, fmt.Errorf("upsert edge device source: %w", err)
	}
	return row, nil
}

func (s *PostgresStore) ListEdgeDeviceSources(ctx context.Context, in ListEdgeDeviceSourcesInput) ([]EdgeDeviceSource, error) {
	query := `
SELECT eds.id::text, eds.collector_id::text, eds.user_id::text, eds.provider, eds.transport, eds.provider_device_id,
	COALESCE(eds.display_name, ''), COALESCE(eds.model, ''), COALESCE(eds.address_hash, ''), eds.rssi_dbm,
	eds.metadata, eds.status, COALESCE(eds.linked_device_id::text, ''), eds.last_seen_at, eds.created_at, eds.updated_at
FROM edge_device_sources eds
JOIN users u ON u.id = eds.user_id
WHERE u.keycloak_subject = $1
  AND ($2 = '' OR eds.collector_id = $2::uuid)
  AND ($3 = '' OR eds.status = $3)
ORDER BY eds.last_seen_at DESC, eds.provider_device_id ASC;
`
	rows, err := s.db.QueryContext(ctx, query, in.UserSubject, strings.TrimSpace(in.CollectorID), strings.ToLower(strings.TrimSpace(in.Status)))
	if err != nil {
		return nil, fmt.Errorf("query edge device sources: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]EdgeDeviceSource, 0, 8)
	for rows.Next() {
		row, err := scanEdgeDeviceSource(rows)
		if err != nil {
			return nil, fmt.Errorf("scan edge source row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate edge device sources: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) ApproveEdgeDeviceSource(ctx context.Context, in ApproveEdgeDeviceSourceInput) (ApprovedEdgeDeviceSource, error) {
	now := normalizeWriteTime(s.now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ApprovedEdgeDeviceSource{}, fmt.Errorf("begin approve edge source tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userID, err := resolveUserIDTx(ctx, tx, in.UserSubject)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ApprovedEdgeDeviceSource{}, ErrUserNotFound
		}
		return ApprovedEdgeDeviceSource{}, fmt.Errorf("resolve user for edge source approve: %w", err)
	}
	sourceQuery := `
SELECT id::text, collector_id::text, user_id::text, provider, transport, provider_device_id,
	COALESCE(display_name, ''), COALESCE(model, ''), COALESCE(address_hash, ''), rssi_dbm,
	metadata, status, COALESCE(linked_device_id::text, ''), last_seen_at, created_at, updated_at
FROM edge_device_sources
WHERE id = $1::uuid
  AND user_id = $2::uuid
FOR UPDATE;
`
	source, err := scanEdgeDeviceSource(tx.QueryRowContext(ctx, sourceQuery, strings.TrimSpace(in.SourceID), userID))
	if errors.Is(err, sql.ErrNoRows) {
		return ApprovedEdgeDeviceSource{}, ErrEdgeDeviceSourceNotFound
	}
	if err != nil {
		return ApprovedEdgeDeviceSource{}, fmt.Errorf("load edge source for approve: %w", err)
	}

	device, err := s.approveEdgeDeviceTx(ctx, tx, userID, source, in, now)
	if err != nil {
		return ApprovedEdgeDeviceSource{}, err
	}
	updateQuery := `
UPDATE edge_device_sources
SET status = 'linked',
    linked_device_id = $2::uuid,
    updated_at = $3
WHERE id = $1::uuid
RETURNING id::text, collector_id::text, user_id::text, provider, transport, provider_device_id,
	COALESCE(display_name, ''), COALESCE(model, ''), COALESCE(address_hash, ''), rssi_dbm,
	metadata, status, COALESCE(linked_device_id::text, ''), last_seen_at, created_at, updated_at;
`
	source, err = scanEdgeDeviceSource(tx.QueryRowContext(ctx, updateQuery, source.ID, device.DeviceID, now))
	if err != nil {
		return ApprovedEdgeDeviceSource{}, fmt.Errorf("update edge source approval: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return ApprovedEdgeDeviceSource{}, fmt.Errorf("commit approve edge source tx: %w", err)
	}
	return ApprovedEdgeDeviceSource{Source: source, Device: device}, nil
}

func (s *PostgresStore) GetLinkedEdgeDeviceSource(ctx context.Context, in GetLinkedEdgeDeviceSourceInput) (EdgeDeviceSource, error) {
	query := `
SELECT id::text, collector_id::text, user_id::text, provider, transport, provider_device_id,
	COALESCE(display_name, ''), COALESCE(model, ''), COALESCE(address_hash, ''), rssi_dbm,
	metadata, status, COALESCE(linked_device_id::text, ''), last_seen_at, created_at, updated_at
FROM edge_device_sources
WHERE collector_id = $1::uuid
  AND provider = $2
  AND transport = $3
  AND provider_device_id = $4
  AND status = 'linked'
  AND linked_device_id IS NOT NULL;
`
	row, err := scanEdgeDeviceSource(s.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(in.CollectorID),
		NormalizeProvider(in.Provider),
		normalizeEdgeTransport(in.Transport),
		strings.ToUpper(strings.TrimSpace(in.ProviderDeviceID)),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return EdgeDeviceSource{}, ErrEdgeDeviceSourceNotFound
	}
	if err != nil {
		return EdgeDeviceSource{}, fmt.Errorf("get linked edge device source: %w", err)
	}
	return row, nil
}

func (s *PostgresStore) edgeCollectorByID(ctx context.Context, collectorID string) (EdgeCollector, error) {
	query := `
SELECT id::text, user_id::text, display_name, setup_token_hash, COALESCE(collector_secret_hash, ''),
	is_active, COALESCE(collector_version, ''), COALESCE(hostname, ''), last_heartbeat_at, created_at, updated_at
FROM edge_collectors
WHERE id = $1::uuid;
`
	row, err := scanEdgeCollector(s.db.QueryRowContext(ctx, query, strings.TrimSpace(collectorID)))
	if errors.Is(err, sql.ErrNoRows) {
		return EdgeCollector{}, ErrEdgeCollectorNotFound
	}
	if err != nil {
		return EdgeCollector{}, fmt.Errorf("load edge collector: %w", err)
	}
	return row, nil
}

func (s *PostgresStore) approveEdgeDeviceTx(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	source EdgeDeviceSource,
	in ApproveEdgeDeviceSourceInput,
	now time.Time,
) (UserDevice, error) {
	deviceID := strings.TrimSpace(in.DeviceID)
	if deviceID != "" {
		query := `
SELECT d.id::text, d.ecoflow_sn, COALESCE(d.product_name, ''), COALESCE(d.model, ''), ud.role, d.created_at, d.updated_at
FROM devices d
JOIN user_devices ud ON ud.device_id = d.id
WHERE d.id = $1::uuid
  AND ud.user_id = $2::uuid
  AND ud.role = 'admin';
`
		row, err := scanUserDevice(tx.QueryRowContext(ctx, query, deviceID, userID))
		if errors.Is(err, sql.ErrNoRows) {
			return UserDevice{}, ErrPermissionDenied
		}
		if err != nil {
			return UserDevice{}, fmt.Errorf("load approved edge target device: %w", err)
		}
		return row, nil
	}

	productName := strings.TrimSpace(in.ProductName)
	if productName == "" {
		productName = source.DisplayName
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = source.Model
	}
	deviceQuery := `
INSERT INTO devices (ecoflow_sn, product_name, model, metadata, created_at, updated_at)
VALUES ($1, $2, $3, '{}'::jsonb, $4, $4)
ON CONFLICT (ecoflow_sn)
DO UPDATE SET
	product_name = COALESCE(NULLIF(EXCLUDED.product_name, ''), devices.product_name),
	model = COALESCE(NULLIF(EXCLUDED.model, ''), devices.model),
	updated_at = EXCLUDED.updated_at
RETURNING id::text, ecoflow_sn, COALESCE(product_name, ''), COALESCE(model, ''), created_at, updated_at;
`
	var dev memoryDevice
	if err := tx.QueryRowContext(ctx, deviceQuery, source.ProviderDeviceID, productName, model, now).Scan(
		&dev.ID,
		&dev.EcoflowSN,
		&dev.ProductName,
		&dev.Model,
		&dev.CreatedAt,
		&dev.UpdatedAt,
	); err != nil {
		return UserDevice{}, fmt.Errorf("upsert edge approved device: %w", err)
	}
	linkQuery := `
INSERT INTO user_devices (user_id, device_id, role, created_at, updated_at)
VALUES ($1::uuid, $2::uuid, 'admin', $3, $3)
ON CONFLICT (user_id, device_id)
DO UPDATE SET role = 'admin', updated_at = EXCLUDED.updated_at;
`
	if _, err := tx.ExecContext(ctx, linkQuery, userID, dev.ID, now); err != nil {
		return UserDevice{}, fmt.Errorf("link edge approved device to user: %w", err)
	}
	return UserDevice{
		DeviceID:    dev.ID,
		EcoflowSN:   dev.EcoflowSN,
		ProductName: dev.ProductName,
		Model:       dev.Model,
		Role:        "admin",
		CreatedAt:   dev.CreatedAt,
		UpdatedAt:   dev.UpdatedAt,
	}, nil
}

func scanEdgeCollector(scanner interface{ Scan(dest ...any) error }) (EdgeCollector, error) {
	var row EdgeCollector
	var heartbeat sql.NullTime
	if err := scanner.Scan(
		&row.ID,
		&row.UserID,
		&row.DisplayName,
		&row.SetupTokenHash,
		&row.CollectorSecretHash,
		&row.IsActive,
		&row.CollectorVersion,
		&row.Hostname,
		&heartbeat,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return EdgeCollector{}, err
	}
	if heartbeat.Valid {
		row.LastHeartbeatAt = heartbeat.Time
	}
	return row, nil
}

func scanEdgeDeviceSource(scanner interface{ Scan(dest ...any) error }) (EdgeDeviceSource, error) {
	var row EdgeDeviceSource
	if err := scanner.Scan(
		&row.ID,
		&row.CollectorID,
		&row.UserID,
		&row.Provider,
		&row.Transport,
		&row.ProviderDeviceID,
		&row.DisplayName,
		&row.Model,
		&row.AddressHash,
		&row.RSSIDBm,
		(*jsonbMap)(&row.Metadata),
		&row.Status,
		&row.LinkedDeviceID,
		&row.LastSeenAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return EdgeDeviceSource{}, err
	}
	return row, nil
}

func scanUserDevice(scanner interface{ Scan(dest ...any) error }) (UserDevice, error) {
	var row UserDevice
	if err := scanner.Scan(
		&row.DeviceID,
		&row.EcoflowSN,
		&row.ProductName,
		&row.Model,
		&row.Role,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return UserDevice{}, err
	}
	return row, nil
}

func marshalEdgeMetadata(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	return marshalJSONBMap(cloneAnyMap(value))
}
