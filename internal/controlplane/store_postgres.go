package controlplane

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStore struct {
	db  *sql.DB
	now func() time.Time
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &PostgresStore{
		db:  db,
		now: utcNow,
	}, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) CreateProviderCredential(ctx context.Context, in CreateProviderCredentialInput) (ProviderCredential, error) {
	provider := NormalizeProvider(in.Provider)
	now := s.now()
	query := `
WITH target_user AS (
	SELECT id
	FROM users
	WHERE keycloak_subject = $1
)
INSERT INTO provider_credentials (
	user_id,
	provider,
	access_key_ciphertext,
	secret_key_ciphertext,
	access_key_hash,
	access_key_mask,
	is_active,
	created_at,
	updated_at
)
SELECT
	target_user.id,
	$2,
	$3,
	$4,
	$5,
	$6,
	$7,
	$8,
	$8
FROM target_user
RETURNING id::text, user_id::text, provider, access_key_mask, is_active, created_at, updated_at;
`
	var out ProviderCredential
	row := s.db.QueryRowContext(
		ctx,
		query,
		in.UserSubject,
		provider,
		[]byte(in.AccessKey),
		[]byte(in.SecretKey),
		HashAccessKey(in.AccessKey),
		MaskAccessKey(in.AccessKey),
		in.IsActive,
		now,
	)
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.AccessKeyMask,
		&out.IsActive,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProviderCredential{}, ErrUserNotFound
		}
		return ProviderCredential{}, fmt.Errorf("insert provider credential: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) ListProviderCredentials(ctx context.Context, in ListProviderCredentialsInput) ([]ProviderCredential, error) {
	provider := NormalizeProvider(in.Provider)
	query := `
SELECT
	pc.id::text,
	pc.user_id::text,
	pc.provider,
	pc.access_key_mask,
	pc.access_key_ciphertext,
	pc.secret_key_ciphertext,
	pc.is_active,
	pc.created_at,
	pc.updated_at
FROM provider_credentials pc
JOIN users u ON u.id = pc.user_id
WHERE u.keycloak_subject = $1
  AND ($2 = '' OR pc.provider = $2)
ORDER BY pc.created_at DESC, pc.id DESC;
`
	rows, err := s.db.QueryContext(ctx, query, in.UserSubject, provider)
	if err != nil {
		return nil, fmt.Errorf("query provider credentials: %w", err)
	}
	defer rows.Close()

	out := make([]ProviderCredential, 0, 4)
	for rows.Next() {
		var row ProviderCredential
		if err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.Provider,
			&row.AccessKeyMask,
			&row.IsActive,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan provider credential row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider credentials: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) SetProviderCredentialActive(ctx context.Context, in SetProviderCredentialActiveInput) (ProviderCredential, error) {
	query := `
UPDATE provider_credentials pc
SET is_active = $3,
    updated_at = $4
FROM users u
WHERE pc.id = $1::uuid
  AND u.id = pc.user_id
  AND u.keycloak_subject = $2
RETURNING pc.id::text, pc.user_id::text, pc.provider, pc.access_key_mask, pc.is_active, pc.created_at, pc.updated_at;
`
	var out ProviderCredential
	row := s.db.QueryRowContext(ctx, query, in.CredentialID, in.UserSubject, in.IsActive, s.now())
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.AccessKeyMask,
		&out.IsActive,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProviderCredential{}, ErrCredentialNotFound
		}
		return ProviderCredential{}, fmt.Errorf("update provider credential active state: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) GetProviderCredential(ctx context.Context, userSubject string, credentialID string) (ProviderCredential, error) {
	query := `
SELECT
	pc.id::text,
	pc.user_id::text,
	pc.provider,
	pc.access_key_mask,
	pc.is_active,
	pc.created_at,
	pc.updated_at
FROM provider_credentials pc
JOIN users u ON u.id = pc.user_id
WHERE pc.id = $1::uuid
  AND u.keycloak_subject = $2;
`
	var out ProviderCredential
	var accessKeyBytes []byte
	var secretKeyBytes []byte
	row := s.db.QueryRowContext(ctx, query, credentialID, userSubject)
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.AccessKeyMask,
		&accessKeyBytes,
		&secretKeyBytes,
		&out.IsActive,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProviderCredential{}, ErrCredentialNotFound
		}
		return ProviderCredential{}, fmt.Errorf("get provider credential: %w", err)
	}
	out.AccessKey = string(accessKeyBytes)
	out.SecretKey = string(secretKeyBytes)
	return out, nil
}

func (s *PostgresStore) ListProviderDevices(ctx context.Context, in ListProviderDevicesInput) ([]ProviderDevice, error) {
	provider := NormalizeProvider(in.Provider)
	query := `
SELECT
	pd.id::text,
	pd.device_id::text,
	pd.provider,
	pd.provider_device_id,
	pd.credential_id::text,
	d.ecoflow_sn,
	COALESCE(pd.product_name, d.product_name, ''),
	COALESCE(pd.model, d.model, ''),
	pd.is_active,
	pd.ingest_desired_state
FROM provider_devices pd
JOIN devices d ON d.id = pd.device_id
JOIN user_devices ud ON ud.device_id = d.id
JOIN users u ON u.id = ud.user_id
WHERE u.keycloak_subject = $1
  AND ($2 = '' OR pd.provider = $2)
  AND (NOT $3 OR pd.is_active = TRUE)
ORDER BY pd.provider ASC, d.product_name ASC, d.ecoflow_sn ASC;
`
	rows, err := s.db.QueryContext(ctx, query, in.UserSubject, provider, in.ActiveOnly)
	if err != nil {
		return nil, fmt.Errorf("query provider devices: %w", err)
	}
	defer rows.Close()

	out := make([]ProviderDevice, 0, 8)
	for rows.Next() {
		var row ProviderDevice
		if err := rows.Scan(
			&row.ID,
			&row.DeviceID,
			&row.Provider,
			&row.ProviderDeviceID,
			&row.CredentialID,
			&row.CanonicalSN,
			&row.ProductName,
			&row.Model,
			&row.IsActive,
			&row.IngestDesiredState,
		); err != nil {
			return nil, fmt.Errorf("scan provider devices row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provider devices rows: %w", err)
	}
	return out, nil
}

func utcNow() time.Time {
	return time.Now().UTC()
}
