package controlplane

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jpaljasma/ecoflow-pulse/internal/dbpool"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
	"golang.org/x/sync/errgroup"
)

const (
	providerCredentialUserProviderAccessHashConstraint = "uq_provider_credentials_user_provider_access_key_hash"
	providerCredentialProviderAccessHashIndex          = "uq_provider_credentials_provider_access_key_hash"
)

type PostgresStore struct {
	db  *sql.DB
	now func() time.Time
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	var err error
	dsn, err = pgsearchpath.ApplyFromEnv(dsn, "")
	if err != nil {
		return nil, fmt.Errorf("apply postgres search_path: %w", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}
	dbpool.ConfigureSQL(db)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return newPostgresStore(db), nil
}

func newPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{
		db:  db,
		now: utcNow,
	}
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) CreateProviderCredential(ctx context.Context, in CreateProviderCredentialInput) (ProviderCredential, error) {
	provider := NormalizeProvider(in.Provider)
	now := normalizeWriteTime(s.now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProviderCredential{}, fmt.Errorf("begin create provider credential tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userID, err := resolveUserIDTx(ctx, tx, in.UserSubject)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProviderCredential{}, ErrUserNotFound
		}
		return ProviderCredential{}, fmt.Errorf("resolve user for provider credential create: %w", err)
	}
	if in.IsActive {
		if err := deactivateOtherProviderCredentialsTx(ctx, tx, userID, provider, "", now); err != nil {
			return ProviderCredential{}, err
		}
	}

	query := `
INSERT INTO provider_credentials (
	user_id,
	provider,
	access_key_ciphertext,
	secret_key_ciphertext,
	access_key_hash,
	access_key_mask,
	provider_config,
	is_active,
	created_at,
	updated_at
)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING id::text, user_id::text, provider, access_key_mask, provider_config, is_active, created_at, updated_at;
`
	var out ProviderCredential
	configJSON, err := marshalProviderCredentialConfig(in.Config)
	if err != nil {
		return ProviderCredential{}, fmt.Errorf("marshal provider credential config: %w", err)
	}
	row := tx.QueryRowContext(
		ctx,
		query,
		userID,
		provider,
		[]byte(in.AccessKey),
		[]byte(in.SecretKey),
		HashAccessKey(in.AccessKey),
		MaskAccessKey(in.AccessKey),
		configJSON,
		in.IsActive,
		now,
	)
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.AccessKeyMask,
		(*jsonbMap)(&out.Config),
		&out.IsActive,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if isProviderCredentialAccessKeyConflict(err) {
			return ProviderCredential{}, ErrCredentialAlreadyExists
		}
		return ProviderCredential{}, fmt.Errorf("insert provider credential: %w", err)
	}
	if out.IsActive {
		if err := rebindProviderDevicesTx(ctx, tx, userID, out.Provider, out.ID, now); err != nil {
			return ProviderCredential{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ProviderCredential{}, fmt.Errorf("commit create provider credential tx: %w", err)
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
	pc.provider_config,
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
	defer func() { _ = rows.Close() }()

	out := make([]ProviderCredential, 0, 4)
	for rows.Next() {
		var row ProviderCredential
		if err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.Provider,
			&row.AccessKeyMask,
			(*jsonbMap)(&row.Config),
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
	now := normalizeWriteTime(s.now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProviderCredential{}, fmt.Errorf("begin set provider credential active tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	target, err := getProviderCredentialTx(ctx, tx, in.UserSubject, in.CredentialID)
	if err != nil {
		if errors.Is(err, ErrCredentialNotFound) {
			return ProviderCredential{}, ErrCredentialNotFound
		}
		return ProviderCredential{}, fmt.Errorf("load provider credential for active update: %w", err)
	}
	if in.IsActive {
		if err := deactivateOtherProviderCredentialsTx(ctx, tx, target.UserID, target.Provider, target.ID, now); err != nil {
			return ProviderCredential{}, err
		}
	}
	query := `
UPDATE provider_credentials
SET is_active = $2,
    updated_at = $3
WHERE id = $1::uuid
RETURNING id::text, user_id::text, provider, access_key_mask, provider_config, is_active, created_at, updated_at;
`
	var out ProviderCredential
	row := tx.QueryRowContext(ctx, query, target.ID, in.IsActive, now)
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.AccessKeyMask,
		(*jsonbMap)(&out.Config),
		&out.IsActive,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProviderCredential{}, ErrCredentialNotFound
		}
		return ProviderCredential{}, fmt.Errorf("update provider credential active state: %w", err)
	}
	if out.IsActive {
		if err := rebindProviderDevicesTx(ctx, tx, out.UserID, out.Provider, out.ID, now); err != nil {
			return ProviderCredential{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ProviderCredential{}, fmt.Errorf("commit set provider credential active tx: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) UpdateProviderCredential(ctx context.Context, in UpdateProviderCredentialInput) (ProviderCredential, error) {
	now := normalizeWriteTime(s.now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProviderCredential{}, fmt.Errorf("begin update provider credential tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	target, err := getProviderCredentialTx(ctx, tx, in.UserSubject, in.CredentialID)
	if err != nil {
		if errors.Is(err, ErrCredentialNotFound) {
			return ProviderCredential{}, ErrCredentialNotFound
		}
		return ProviderCredential{}, fmt.Errorf("load provider credential for update: %w", err)
	}
	if in.IsActive {
		if err := deactivateOtherProviderCredentialsTx(ctx, tx, target.UserID, target.Provider, target.ID, now); err != nil {
			return ProviderCredential{}, err
		}
	}

	query := `
UPDATE provider_credentials
SET access_key_ciphertext = $2,
    secret_key_ciphertext = $3,
    access_key_hash = $4,
    access_key_mask = $5,
    provider_config = $6,
    is_active = $7,
    updated_at = $8
WHERE id = $1::uuid
RETURNING id::text, user_id::text, provider, access_key_mask, provider_config, is_active, created_at, updated_at;
`
	var out ProviderCredential
	configJSON, err := marshalProviderCredentialConfig(in.Config)
	if err != nil {
		return ProviderCredential{}, fmt.Errorf("marshal provider credential config: %w", err)
	}
	row := tx.QueryRowContext(
		ctx,
		query,
		target.ID,
		[]byte(in.AccessKey),
		[]byte(in.SecretKey),
		HashAccessKey(in.AccessKey),
		MaskAccessKey(in.AccessKey),
		configJSON,
		in.IsActive,
		now,
	)
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.AccessKeyMask,
		(*jsonbMap)(&out.Config),
		&out.IsActive,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProviderCredential{}, ErrCredentialNotFound
		}
		if isProviderCredentialAccessKeyConflict(err) {
			return ProviderCredential{}, ErrCredentialAlreadyExists
		}
		return ProviderCredential{}, fmt.Errorf("update provider credential: %w", err)
	}
	if out.IsActive {
		if err := rebindProviderDevicesTx(ctx, tx, out.UserID, out.Provider, out.ID, now); err != nil {
			return ProviderCredential{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ProviderCredential{}, fmt.Errorf("commit update provider credential tx: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) GetProviderCredential(ctx context.Context, userSubject string, credentialID string) (ProviderCredential, error) {
	out, err := getProviderCredentialQuery(ctx, s.db, userSubject, credentialID)
	if err != nil {
		return ProviderCredential{}, err
	}
	return out, nil
}

func resolveUserIDTx(ctx context.Context, tx *sql.Tx, userSubject string) (string, error) {
	var userID string
	if err := tx.QueryRowContext(
		ctx,
		`SELECT id::text FROM users WHERE keycloak_subject = $1`,
		userSubject,
	).Scan(&userID); err != nil {
		return "", err
	}
	return userID, nil
}

func deactivateOtherProviderCredentialsTx(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	provider string,
	excludeCredentialID string,
	now time.Time,
) error {
	if strings.TrimSpace(excludeCredentialID) == "" {
		query := `
UPDATE provider_credentials
SET is_active = FALSE,
    updated_at = $3
WHERE user_id = $1::uuid
  AND provider = $2
  AND is_active = TRUE;
`
		if _, err := tx.ExecContext(ctx, query, userID, provider, now); err != nil {
			return fmt.Errorf("deactivate provider credentials: %w", err)
		}
		return nil
	}

	query := `
UPDATE provider_credentials
SET is_active = FALSE,
    updated_at = $4
WHERE user_id = $1::uuid
  AND provider = $2
  AND id <> $3::uuid
  AND is_active = TRUE;
`
	if _, err := tx.ExecContext(ctx, query, userID, provider, excludeCredentialID, now); err != nil {
		return fmt.Errorf("deactivate provider credentials: %w", err)
	}
	return nil
}

func rebindProviderDevicesTx(
	ctx context.Context,
	tx *sql.Tx,
	userID string,
	provider string,
	credentialID string,
	now time.Time,
) error {
	query := `
UPDATE provider_devices pd
SET credential_id = $4::uuid,
    updated_at = $3
FROM provider_credentials pc
WHERE pd.credential_id = pc.id
  AND pc.user_id = $1::uuid
  AND pc.provider = $2
  AND pd.provider = $2
  AND pd.credential_id <> $4::uuid;
`
	if _, err := tx.ExecContext(ctx, query, userID, provider, now, credentialID); err != nil {
		return fmt.Errorf("rebind provider devices: %w", err)
	}
	return nil
}

type queryRowScanner interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getProviderCredentialQuery(ctx context.Context, db queryRowScanner, userSubject string, credentialID string) (ProviderCredential, error) {
	query := `
SELECT
	pc.id::text,
	pc.user_id::text,
	pc.provider,
	pc.access_key_mask,
	pc.access_key_ciphertext,
	pc.secret_key_ciphertext,
	pc.provider_config,
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
	row := db.QueryRowContext(ctx, query, credentialID, userSubject)
	if err := row.Scan(
		&out.ID,
		&out.UserID,
		&out.Provider,
		&out.AccessKeyMask,
		&accessKeyBytes,
		&secretKeyBytes,
		(*jsonbMap)(&out.Config),
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

func getProviderCredentialTx(ctx context.Context, tx *sql.Tx, userSubject string, credentialID string) (ProviderCredential, error) {
	return getProviderCredentialQuery(ctx, tx, userSubject, credentialID)
}

func isProviderCredentialAccessKeyConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	return pgErr.ConstraintName == providerCredentialUserProviderAccessHashConstraint ||
		pgErr.ConstraintName == providerCredentialProviderAccessHashIndex
}

func (s *PostgresStore) CreateDevice(ctx context.Context, in CreateDeviceInput) (UserDevice, error) {
	now := normalizeWriteTime(s.now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UserDevice{}, fmt.Errorf("begin create device tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var userID string
	if err := tx.QueryRowContext(
		ctx,
		`SELECT id::text FROM users WHERE keycloak_subject = $1`,
		in.UserSubject,
	).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserDevice{}, ErrUserNotFound
		}
		return UserDevice{}, fmt.Errorf("resolve user for create device: %w", err)
	}

	var row UserDevice
	if err := tx.QueryRowContext(
		ctx,
		`
INSERT INTO devices (ecoflow_sn, product_name, model, metadata, created_at, updated_at)
VALUES ($1, NULLIF(BTRIM($2), ''), NULLIF(BTRIM($3), ''), '{}'::jsonb, $4, $4)
ON CONFLICT (ecoflow_sn)
DO UPDATE
SET product_name = COALESCE(EXCLUDED.product_name, devices.product_name),
	model = COALESCE(EXCLUDED.model, devices.model),
	updated_at = EXCLUDED.updated_at
RETURNING id::text, ecoflow_sn, COALESCE(product_name, ''), COALESCE(model, ''), created_at, updated_at;
`,
		in.EcoflowSN,
		in.ProductName,
		in.Model,
		now,
	).Scan(
		&row.DeviceID,
		&row.EcoflowSN,
		&row.ProductName,
		&row.Model,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return UserDevice{}, fmt.Errorf("upsert device: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`
INSERT INTO user_devices (user_id, device_id, role, created_at, updated_at)
VALUES ($1::uuid, $2::uuid, 'admin', $3, $3)
ON CONFLICT (user_id, device_id)
DO UPDATE SET role = 'admin', updated_at = EXCLUDED.updated_at;
`,
		userID,
		row.DeviceID,
		now,
	); err != nil {
		return UserDevice{}, fmt.Errorf("upsert user device admin link: %w", err)
	}

	row.Role = "admin"
	if err := tx.Commit(); err != nil {
		return UserDevice{}, fmt.Errorf("commit create device tx: %w", err)
	}
	return row, nil
}

func (s *PostgresStore) LinkDevice(ctx context.Context, in LinkDeviceInput) (UserDevice, error) {
	now := normalizeWriteTime(s.now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UserDevice{}, fmt.Errorf("begin link device tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var requesterID string
	if err := tx.QueryRowContext(
		ctx,
		`SELECT id::text FROM users WHERE keycloak_subject = $1`,
		in.UserSubject,
	).Scan(&requesterID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserDevice{}, ErrUserNotFound
		}
		return UserDevice{}, fmt.Errorf("resolve requester user: %w", err)
	}

	targetSubject := in.TargetUserSubject
	if targetSubject == "" {
		targetSubject = in.UserSubject
	}

	var targetUserID string
	if err := tx.QueryRowContext(
		ctx,
		`SELECT id::text FROM users WHERE keycloak_subject = $1`,
		targetSubject,
	).Scan(&targetUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserDevice{}, ErrUserNotFound
		}
		return UserDevice{}, fmt.Errorf("resolve target user: %w", err)
	}

	var row UserDevice
	if err := tx.QueryRowContext(
		ctx,
		`
SELECT id::text, ecoflow_sn, COALESCE(product_name, ''), COALESCE(model, ''), created_at, updated_at
FROM devices
WHERE id = $1::uuid;
`,
		in.DeviceID,
	).Scan(
		&row.DeviceID,
		&row.EcoflowSN,
		&row.ProductName,
		&row.Model,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserDevice{}, ErrDeviceNotFound
		}
		return UserDevice{}, fmt.Errorf("resolve device for link: %w", err)
	}

	var requesterRole string
	if err := tx.QueryRowContext(
		ctx,
		`
SELECT role
FROM user_devices
WHERE user_id = $1::uuid
  AND device_id = $2::uuid;
`,
		requesterID,
		in.DeviceID,
	).Scan(&requesterRole); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserDevice{}, ErrPermissionDenied
		}
		return UserDevice{}, fmt.Errorf("resolve requester role for link: %w", err)
	}
	if requesterRole != "admin" {
		return UserDevice{}, ErrPermissionDenied
	}

	if _, err := tx.ExecContext(
		ctx,
		`
INSERT INTO user_devices (user_id, device_id, role, created_at, updated_at)
VALUES ($1::uuid, $2::uuid, $3, $4, $4)
ON CONFLICT (user_id, device_id)
DO UPDATE SET role = EXCLUDED.role, updated_at = EXCLUDED.updated_at;
`,
		targetUserID,
		in.DeviceID,
		in.Role,
		now,
	); err != nil {
		return UserDevice{}, fmt.Errorf("upsert user device link: %w", err)
	}
	row.Role = in.Role
	if err := tx.Commit(); err != nil {
		return UserDevice{}, fmt.Errorf("commit link device tx: %w", err)
	}
	return row, nil
}

func (s *PostgresStore) ListUserDevices(ctx context.Context, in ListUserDevicesInput) ([]UserDevice, error) {
	query := `
SELECT
	d.id::text,
	d.ecoflow_sn,
	COALESCE(d.product_name, ''),
	COALESCE(d.model, ''),
	ud.role,
	d.created_at,
	d.updated_at
FROM user_devices ud
JOIN users u ON u.id = ud.user_id
JOIN devices d ON d.id = ud.device_id
WHERE u.keycloak_subject = $1
ORDER BY d.product_name ASC, d.ecoflow_sn ASC;
`
	return dbpool.RetryRead(ctx, func(ctx context.Context) ([]UserDevice, error) {
		rows, err := s.db.QueryContext(ctx, query, in.UserSubject)
		if err != nil {
			return nil, fmt.Errorf("query user devices: %w", err)
		}
		defer func() { _ = rows.Close() }()

		out := make([]UserDevice, 0, 8)
		for rows.Next() {
			var row UserDevice
			if err := rows.Scan(
				&row.DeviceID,
				&row.EcoflowSN,
				&row.ProductName,
				&row.Model,
				&row.Role,
				&row.CreatedAt,
				&row.UpdatedAt,
			); err != nil {
				return nil, fmt.Errorf("scan user devices row: %w", err)
			}
			out = append(out, row)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate user devices rows: %w", err)
		}
		return out, nil
	})
}

func (s *PostgresStore) GetOrProvisionCurrentUser(ctx context.Context, in GetOrProvisionCurrentUserInput) (CurrentUser, error) {
	now := normalizeWriteTime(s.now())
	providerDisplayName := PreferredProviderDisplayName(in.DisplayName, in.GivenName, in.FamilyName, in.Email)
	query := `
INSERT INTO users (
	keycloak_subject,
	email,
	email_verified,
	display_name,
	display_name_source,
	avatar_url,
	given_name,
	family_name,
	locale,
	created_at,
	updated_at,
	last_login_at
)
VALUES (
	$1,
	NULLIF($2, ''),
	$3,
	NULLIF($4, ''),
	'provider',
	NULLIF($5, ''),
	NULLIF($6, ''),
	NULLIF($7, ''),
	NULLIF($8, ''),
	$9,
	$9,
	$9
)
ON CONFLICT (keycloak_subject)
DO UPDATE SET
	email = CASE
		WHEN length(trim(COALESCE(EXCLUDED.email, ''))) > 0 THEN EXCLUDED.email
		ELSE users.email
	END,
	email_verified = EXCLUDED.email_verified,
	display_name = CASE
		WHEN users.display_name_source = 'pulse' AND length(trim(COALESCE(users.display_name, ''))) > 0 THEN users.display_name
		WHEN length(trim(COALESCE(EXCLUDED.display_name, ''))) > 0 THEN EXCLUDED.display_name
		ELSE users.display_name
	END,
	display_name_source = CASE
		WHEN users.display_name_source = 'pulse' AND length(trim(COALESCE(users.display_name, ''))) > 0 THEN 'pulse'
		ELSE 'provider'
	END,
	avatar_url = CASE
		WHEN length(trim(COALESCE(EXCLUDED.avatar_url, ''))) > 0 THEN EXCLUDED.avatar_url
		ELSE users.avatar_url
	END,
	given_name = CASE
		WHEN length(trim(COALESCE(EXCLUDED.given_name, ''))) > 0 THEN EXCLUDED.given_name
		ELSE users.given_name
	END,
	family_name = CASE
		WHEN length(trim(COALESCE(EXCLUDED.family_name, ''))) > 0 THEN EXCLUDED.family_name
		ELSE users.family_name
	END,
	locale = CASE
		WHEN length(trim(COALESCE(EXCLUDED.locale, ''))) > 0 THEN EXCLUDED.locale
		ELSE users.locale
	END,
	last_login_at = EXCLUDED.last_login_at,
	updated_at = EXCLUDED.updated_at
RETURNING
	id::text,
	keycloak_subject,
	COALESCE(email, ''),
	email_verified,
	COALESCE(display_name, ''),
	COALESCE(display_name_source, 'provider'),
	COALESCE(avatar_url, ''),
	COALESCE(given_name, ''),
	COALESCE(family_name, ''),
	COALESCE(locale, ''),
	COALESCE(timezone, ''),
	weather_location_enabled,
	COALESCE(weather_location_source, 'none'),
	COALESCE(weather_location_label, ''),
	weather_latitude,
	weather_longitude,
	last_login_at,
	created_at,
	updated_at;
`
	out, err := dbpool.RetryRead(ctx, func(ctx context.Context) (CurrentUser, error) {
		row := s.db.QueryRowContext(
			ctx,
			query,
			strings.TrimSpace(in.UserSubject),
			strings.TrimSpace(in.Email),
			in.EmailVerified,
			providerDisplayName,
			strings.TrimSpace(in.AvatarURL),
			strings.TrimSpace(in.GivenName),
			strings.TrimSpace(in.FamilyName),
			strings.TrimSpace(in.Locale),
			now,
		)
		return scanCurrentUser(row)
	})
	if err != nil {
		return CurrentUser{}, fmt.Errorf("get or provision current user: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) UpdateCurrentUserProfile(ctx context.Context, in UpdateCurrentUserProfileInput) (CurrentUser, error) {
	query := `
UPDATE users
SET
	display_name = CASE
		WHEN length(trim($2)) > 0 THEN trim($2)
		ELSE display_name
	END,
	display_name_source = CASE
		WHEN length(trim($2)) > 0 THEN 'pulse'
		ELSE display_name_source
	END,
	timezone = NULLIF(trim($3), ''),
	weather_location_enabled = $4,
	weather_location_source = CASE
		WHEN $4 AND $5 THEN COALESCE(NULLIF(trim($6), ''), 'auto')
		ELSE 'none'
	END,
	weather_location_label = CASE
		WHEN $4 AND $5 THEN NULLIF(trim($7), '')
		ELSE NULL
	END,
	weather_latitude = CASE
		WHEN $4 AND $5 THEN $8::double precision
		ELSE NULL
	END,
	weather_longitude = CASE
		WHEN $4 AND $5 THEN $9::double precision
		ELSE NULL
	END,
	updated_at = $10
WHERE keycloak_subject = $1
RETURNING
	id::text,
	keycloak_subject,
	COALESCE(email, ''),
	email_verified,
	COALESCE(display_name, ''),
	COALESCE(display_name_source, 'provider'),
	COALESCE(avatar_url, ''),
	COALESCE(given_name, ''),
	COALESCE(family_name, ''),
	COALESCE(locale, ''),
	COALESCE(timezone, ''),
	weather_location_enabled,
	COALESCE(weather_location_source, 'none'),
	COALESCE(weather_location_label, ''),
	weather_latitude,
	weather_longitude,
	last_login_at,
	created_at,
	updated_at;
`
	row := s.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(in.UserSubject),
		strings.TrimSpace(in.DisplayName),
		strings.TrimSpace(in.Timezone),
		in.WeatherLocationEnabled,
		in.HasWeatherLocationValue,
		strings.TrimSpace(in.WeatherLocationSource),
		strings.TrimSpace(in.WeatherLocationLabel),
		in.WeatherLatitude,
		in.WeatherLongitude,
		normalizeWriteTime(s.now()),
	)
	out, err := scanCurrentUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CurrentUser{}, ErrUserNotFound
		}
		return CurrentUser{}, fmt.Errorf("update current user profile: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) ReconcileUserSubjectByEmail(ctx context.Context, in ReconcileUserSubjectByEmailInput) (CurrentUser, error) {
	email := strings.TrimSpace(in.Email)
	userSubject := strings.TrimSpace(in.UserSubject)
	if email == "" {
		return CurrentUser{}, ErrVerifiedEmailNotFound
	}
	if userSubject == "" {
		return CurrentUser{}, ErrUserNotFound
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CurrentUser{}, fmt.Errorf("begin reconcile user subject tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existingBySubject := tx.QueryRowContext(
		ctx,
		`
SELECT
	id::text,
	keycloak_subject,
	COALESCE(email, ''),
	email_verified,
	COALESCE(display_name, ''),
	COALESCE(display_name_source, 'provider'),
	COALESCE(avatar_url, ''),
	COALESCE(given_name, ''),
	COALESCE(family_name, ''),
	COALESCE(locale, ''),
	COALESCE(timezone, ''),
	weather_location_enabled,
	COALESCE(weather_location_source, 'none'),
	COALESCE(weather_location_label, ''),
	weather_latitude,
	weather_longitude,
	last_login_at,
	created_at,
	updated_at
FROM users
WHERE keycloak_subject = $1;
`,
		userSubject,
	)
	current, err := scanCurrentUser(existingBySubject)
	switch {
	case err == nil:
		if strings.EqualFold(strings.TrimSpace(current.Email), email) {
			if err := tx.Commit(); err != nil {
				return CurrentUser{}, fmt.Errorf("commit reconcile user subject tx: %w", err)
			}
			return current, nil
		}
		return CurrentUser{}, ErrUserSubjectConflict
	case errors.Is(err, sql.ErrNoRows):
		// Continue to verified-email remap below.
	default:
		return CurrentUser{}, fmt.Errorf("lookup current user subject: %w", err)
	}

	row := tx.QueryRowContext(
		ctx,
		`
UPDATE users
SET keycloak_subject = $2,
	updated_at = $3
WHERE lower(COALESCE(email, '')) = lower($1)
  AND email_verified = TRUE
RETURNING
	id::text,
	keycloak_subject,
	COALESCE(email, ''),
	email_verified,
	COALESCE(display_name, ''),
	COALESCE(display_name_source, 'provider'),
	COALESCE(avatar_url, ''),
	COALESCE(given_name, ''),
	COALESCE(family_name, ''),
	COALESCE(locale, ''),
	COALESCE(timezone, ''),
	weather_location_enabled,
	COALESCE(weather_location_source, 'none'),
	COALESCE(weather_location_label, ''),
	weather_latitude,
	weather_longitude,
	last_login_at,
	created_at,
	updated_at;
`,
		email,
		userSubject,
		normalizeWriteTime(s.now()),
	)
	out, err := scanCurrentUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CurrentUser{}, ErrVerifiedEmailNotFound
		}
		return CurrentUser{}, fmt.Errorf("reconcile user subject by email: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return CurrentUser{}, fmt.Errorf("commit reconcile user subject tx: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) UpsertProviderDevice(ctx context.Context, in UpsertProviderDeviceInput) (ProviderDevice, error) {
	now := normalizeWriteTime(s.now())
	query := `
WITH synced_device AS (
	UPDATE devices
	SET product_name = CASE
			WHEN NULLIF(BTRIM($5), '') IS NULL THEN product_name
			ELSE NULLIF(BTRIM($5), '')
		END,
		model = CASE
			WHEN NULLIF(BTRIM($6), '') IS NULL THEN model
			ELSE NULLIF(BTRIM($6), '')
		END,
		updated_at = CASE
			WHEN (NULLIF(BTRIM($5), '') IS NOT NULL AND NULLIF(BTRIM($5), '') IS DISTINCT FROM product_name)
				OR (NULLIF(BTRIM($6), '') IS NOT NULL AND NULLIF(BTRIM($6), '') IS DISTINCT FROM model)
			THEN $11
			ELSE updated_at
		END
	WHERE id = $1::uuid
)
INSERT INTO provider_devices (
	device_id,
	provider,
	provider_device_id,
	credential_id,
	product_name,
	model,
	capabilities,
	metadata,
	is_active,
	ingest_desired_state,
	created_at,
	updated_at
)
VALUES (
	$1::uuid,
	$2,
	$3,
	$4::uuid,
	NULLIF(BTRIM($5), ''),
	NULLIF(BTRIM($6), ''),
	COALESCE($7::jsonb, '{}'::jsonb),
	COALESCE($8::jsonb, '{}'::jsonb),
	$9,
	$10,
	$11,
	$11
)
ON CONFLICT (provider, provider_device_id)
DO UPDATE
SET device_id = EXCLUDED.device_id,
	credential_id = EXCLUDED.credential_id,
	product_name = COALESCE(EXCLUDED.product_name, provider_devices.product_name),
	model = COALESCE(EXCLUDED.model, provider_devices.model),
	capabilities = CASE
		WHEN $7::jsonb IS NULL THEN provider_devices.capabilities
		ELSE $7::jsonb
	END,
	metadata = CASE
		WHEN $8::jsonb IS NULL THEN provider_devices.metadata
		ELSE $8::jsonb
	END,
	is_active = EXCLUDED.is_active,
	ingest_desired_state = EXCLUDED.ingest_desired_state,
	updated_at = EXCLUDED.updated_at
RETURNING id::text, device_id::text, provider, provider_device_id, credential_id::text, COALESCE(product_name, ''), COALESCE(model, ''), capabilities, metadata, is_active, ingest_desired_state;
`
	capabilitiesJSON, err := marshalJSONBMap(in.Capabilities)
	if err != nil {
		return ProviderDevice{}, fmt.Errorf("marshal provider device capabilities: %w", err)
	}
	metadataJSON, err := marshalJSONBMap(in.Metadata)
	if err != nil {
		return ProviderDevice{}, fmt.Errorf("marshal provider device metadata: %w", err)
	}
	var out ProviderDevice
	if err := s.db.QueryRowContext(
		ctx,
		query,
		in.DeviceID,
		NormalizeProvider(in.Provider),
		in.ProviderDeviceID,
		in.CredentialID,
		in.ProductName,
		in.Model,
		capabilitiesJSON,
		metadataJSON,
		in.IsActive,
		strings.ToLower(strings.TrimSpace(in.IngestDesiredState)),
		now,
	).Scan(
		&out.ID,
		&out.DeviceID,
		&out.Provider,
		&out.ProviderDeviceID,
		&out.CredentialID,
		&out.ProductName,
		&out.Model,
		(*jsonbMap)(&out.Capabilities),
		(*jsonbMap)(&out.Metadata),
		&out.IsActive,
		&out.IngestDesiredState,
	); err != nil {
		return ProviderDevice{}, fmt.Errorf("upsert provider device: %w", err)
	}
	out.CanonicalSN = strings.TrimSpace(in.ProviderDeviceID)
	return out, nil
}

func (s *PostgresStore) ImportProviderDevice(ctx context.Context, in ImportProviderDeviceInput) (ImportedProviderDevice, error) {
	now := normalizeWriteTime(s.now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportedProviderDevice{}, fmt.Errorf("begin import provider device tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userID, err := resolveUserIDTx(ctx, tx, in.UserSubject)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ImportedProviderDevice{}, ErrUserNotFound
		}
		return ImportedProviderDevice{}, fmt.Errorf("resolve user for provider device import: %w", err)
	}

	canonicalSN := strings.TrimSpace(in.CanonicalSN)
	if canonicalSN == "" {
		canonicalSN = strings.TrimSpace(in.ProviderDeviceID)
	}
	if canonicalSN == "" {
		return ImportedProviderDevice{}, ErrDeviceNotFound
	}

	var userDevice UserDevice
	if err := tx.QueryRowContext(
		ctx,
		`
INSERT INTO devices (ecoflow_sn, product_name, model, metadata, created_at, updated_at)
VALUES ($1, NULLIF(BTRIM($2), ''), NULLIF(BTRIM($3), ''), '{}'::jsonb, $4, $4)
ON CONFLICT (ecoflow_sn)
DO UPDATE
SET product_name = COALESCE(EXCLUDED.product_name, devices.product_name),
	model = COALESCE(EXCLUDED.model, devices.model),
	updated_at = EXCLUDED.updated_at
RETURNING id::text, ecoflow_sn, COALESCE(product_name, ''), COALESCE(model, ''), created_at, updated_at;
`,
		canonicalSN,
		in.ProductName,
		in.Model,
		now,
	).Scan(
		&userDevice.DeviceID,
		&userDevice.EcoflowSN,
		&userDevice.ProductName,
		&userDevice.Model,
		&userDevice.CreatedAt,
		&userDevice.UpdatedAt,
	); err != nil {
		return ImportedProviderDevice{}, fmt.Errorf("upsert imported device: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`
INSERT INTO user_devices (user_id, device_id, role, created_at, updated_at)
VALUES ($1::uuid, $2::uuid, 'admin', $3, $3)
ON CONFLICT (user_id, device_id)
DO UPDATE SET role = 'admin', updated_at = EXCLUDED.updated_at;
`,
		userID,
		userDevice.DeviceID,
		now,
	); err != nil {
		return ImportedProviderDevice{}, fmt.Errorf("upsert imported user device admin link: %w", err)
	}
	userDevice.Role = "admin"

	capabilitiesJSON, err := marshalJSONBMap(in.Capabilities)
	if err != nil {
		return ImportedProviderDevice{}, fmt.Errorf("marshal provider device capabilities: %w", err)
	}
	metadataJSON, err := marshalJSONBMap(in.Metadata)
	if err != nil {
		return ImportedProviderDevice{}, fmt.Errorf("marshal provider device metadata: %w", err)
	}
	query := `
INSERT INTO provider_devices (
	device_id,
	provider,
	provider_device_id,
	credential_id,
	product_name,
	model,
	capabilities,
	metadata,
	is_active,
	ingest_desired_state,
	created_at,
	updated_at
)
VALUES (
	$1::uuid,
	$2,
	$3,
	$4::uuid,
	NULLIF(BTRIM($5), ''),
	NULLIF(BTRIM($6), ''),
	COALESCE($7::jsonb, '{}'::jsonb),
	COALESCE($8::jsonb, '{}'::jsonb),
	$9,
	$10,
	$11,
	$11
)
ON CONFLICT (provider, provider_device_id)
DO UPDATE
SET device_id = EXCLUDED.device_id,
	credential_id = EXCLUDED.credential_id,
	product_name = COALESCE(EXCLUDED.product_name, provider_devices.product_name),
	model = COALESCE(EXCLUDED.model, provider_devices.model),
	capabilities = CASE
		WHEN $7::jsonb IS NULL THEN provider_devices.capabilities
		ELSE $7::jsonb
	END,
	metadata = CASE
		WHEN $8::jsonb IS NULL THEN provider_devices.metadata
		ELSE $8::jsonb
	END,
	is_active = EXCLUDED.is_active,
	ingest_desired_state = EXCLUDED.ingest_desired_state,
	updated_at = EXCLUDED.updated_at
RETURNING id::text, device_id::text, provider, provider_device_id, credential_id::text, COALESCE(product_name, ''), COALESCE(model, ''), capabilities, metadata, is_active, ingest_desired_state;
`
	var providerDevice ProviderDevice
	if err := tx.QueryRowContext(
		ctx,
		query,
		userDevice.DeviceID,
		NormalizeProvider(in.Provider),
		strings.TrimSpace(in.ProviderDeviceID),
		in.CredentialID,
		in.ProductName,
		in.Model,
		capabilitiesJSON,
		metadataJSON,
		in.IsActive,
		strings.ToLower(strings.TrimSpace(in.IngestDesiredState)),
		now,
	).Scan(
		&providerDevice.ID,
		&providerDevice.DeviceID,
		&providerDevice.Provider,
		&providerDevice.ProviderDeviceID,
		&providerDevice.CredentialID,
		&providerDevice.ProductName,
		&providerDevice.Model,
		(*jsonbMap)(&providerDevice.Capabilities),
		(*jsonbMap)(&providerDevice.Metadata),
		&providerDevice.IsActive,
		&providerDevice.IngestDesiredState,
	); err != nil {
		return ImportedProviderDevice{}, fmt.Errorf("upsert imported provider device: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return ImportedProviderDevice{}, fmt.Errorf("commit import provider device tx: %w", err)
	}
	providerDevice.CanonicalSN = userDevice.EcoflowSN
	if providerDevice.ProductName == "" {
		providerDevice.ProductName = userDevice.ProductName
	}
	if providerDevice.Model == "" {
		providerDevice.Model = userDevice.Model
	}
	return ImportedProviderDevice{
		ProviderDevice: providerDevice,
		UserDevice:     userDevice,
	}, nil
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
	pd.capabilities,
	pd.metadata,
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
	return dbpool.RetryRead(ctx, func(ctx context.Context) ([]ProviderDevice, error) {
		rows, err := s.db.QueryContext(ctx, query, in.UserSubject, provider, in.ActiveOnly)
		if err != nil {
			return nil, fmt.Errorf("query provider devices: %w", err)
		}
		defer func() { _ = rows.Close() }()

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
				(*jsonbMap)(&row.Capabilities),
				(*jsonbMap)(&row.Metadata),
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
	})
}

func (s *PostgresStore) GetProviderDeviceByDeviceID(ctx context.Context, deviceID string) (ProviderDevice, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ProviderDevice{}, ErrDeviceNotFound
	}
	const query = `
SELECT
	pd.id::text,
	pd.device_id::text,
	pd.provider,
	pd.provider_device_id,
	pd.credential_id::text,
	d.ecoflow_sn,
	COALESCE(pd.product_name, d.product_name, ''),
	COALESCE(pd.model, d.model, ''),
	pd.capabilities,
	pd.metadata,
	pd.is_active,
	pd.ingest_desired_state
FROM provider_devices pd
JOIN devices d ON d.id = pd.device_id
WHERE pd.device_id = $1
ORDER BY
	CASE WHEN pd.is_active THEN 0 ELSE 1 END,
	CASE WHEN pd.ingest_desired_state = 'active' THEN 0 ELSE 1 END,
	pd.provider ASC,
	pd.provider_device_id ASC
LIMIT 1;
`
	return dbpool.RetryRead(ctx, func(ctx context.Context) (ProviderDevice, error) {
		var row ProviderDevice
		if err := s.db.QueryRowContext(ctx, query, deviceID).Scan(
			&row.ID,
			&row.DeviceID,
			&row.Provider,
			&row.ProviderDeviceID,
			&row.CredentialID,
			&row.CanonicalSN,
			&row.ProductName,
			&row.Model,
			(*jsonbMap)(&row.Capabilities),
			(*jsonbMap)(&row.Metadata),
			&row.IsActive,
			&row.IngestDesiredState,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ProviderDevice{}, ErrDeviceNotFound
			}
			return ProviderDevice{}, fmt.Errorf("query provider device by device id: %w", err)
		}
		return row, nil
	})
}

func (s *PostgresStore) ListIngestAssignments(ctx context.Context, in ListIngestAssignmentsInput) ([]IngestAssignment, error) {
	provider := NormalizeProvider(in.Provider)
	query := `
SELECT
	pd.provider,
	pd.provider_device_id,
	pd.device_id::text,
	pd.credential_id::text,
	COALESCE(pd.product_name, d.product_name, ''),
	COALESCE(pd.model, d.model, ''),
	pc.access_key_ciphertext,
	pc.secret_key_ciphertext,
	pc.provider_config,
	pd.is_active,
	pc.is_active,
	pd.ingest_desired_state
FROM provider_devices pd
JOIN provider_credentials pc ON pc.id = pd.credential_id
JOIN devices d ON d.id = pd.device_id
WHERE ($1 = '' OR pd.provider = $1)
  AND (NOT $2 OR (pd.is_active = TRUE AND pd.ingest_desired_state = 'active'))
ORDER BY pd.provider ASC, pd.provider_device_id ASC;
`
	rows, err := s.db.QueryContext(ctx, query, provider, in.ActiveOnly)
	if err != nil {
		return nil, fmt.Errorf("query ingest assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]IngestAssignment, 0, 8)
	for rows.Next() {
		var row IngestAssignment
		var accessKeyBytes []byte
		var secretKeyBytes []byte
		var credentialConfig jsonbMap
		if err := rows.Scan(
			&row.Provider,
			&row.ProviderDeviceID,
			&row.DeviceID,
			&row.CredentialID,
			&row.ProductName,
			&row.Model,
			&accessKeyBytes,
			&secretKeyBytes,
			&credentialConfig,
			&row.DeviceIsActive,
			&row.CredentialIsActive,
			&row.IngestDesiredState,
		); err != nil {
			return nil, fmt.Errorf("scan ingest assignments row: %w", err)
		}
		row.AccessKey = string(accessKeyBytes)
		row.SecretKey = string(secretKeyBytes)
		row.CredentialConfig = cloneAnyMap(map[string]any(credentialConfig))
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ingest assignments rows: %w", err)
	}
	return out, nil
}

func (s *PostgresStore) SearchAdminLogFilters(ctx context.Context, in SearchAdminLogFiltersInput) ([]AdminLogFilterOption, error) {
	kind := normalizeAdminLogFilterKind(in.Kind)
	query := strings.ToLower(strings.TrimSpace(in.Query))
	limit := normalizeAdminLogFilterLimit(in.Limit)
	provider := NormalizeProvider(in.Provider)
	deviceIDs := normalizeAdminLogDeviceIDs(in.DeviceIDs)
	userSubject := strings.TrimSpace(in.UserSubject)

	if kind == "device" || kind == "serial" {
		return s.searchAdminLogDeviceFilters(ctx, kind, query, limit, provider, userSubject, in.GlobalAdmin, deviceIDs)
	}
	if kind == "user" {
		if !in.GlobalAdmin {
			return nil, nil
		}
		return s.searchAdminLogUserFilters(ctx, query, limit, deviceIDs)
	}
	if !in.GlobalAdmin {
		return s.searchAdminLogDeviceFilters(ctx, kind, query, limit, provider, userSubject, false, deviceIDs)
	}

	group, groupCtx := errgroup.WithContext(ctx)
	var deviceOptions []AdminLogFilterOption
	var userOptions []AdminLogFilterOption
	group.Go(func() error {
		var err error
		deviceOptions, err = s.searchAdminLogDeviceFilters(groupCtx, kind, query, limit, provider, userSubject, true, deviceIDs)
		return err
	})
	group.Go(func() error {
		var err error
		userOptions, err = s.searchAdminLogUserFilters(groupCtx, query, limit, deviceIDs)
		return err
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}

	out := make([]AdminLogFilterOption, 0, limit)
	out = appendAdminLogOptions(out, deviceOptions, limit)
	out = appendAdminLogOptions(out, userOptions, limit)
	return out, nil
}

func (s *PostgresStore) searchAdminLogDeviceFilters(ctx context.Context, kind string, query string, limit int, provider string, userSubject string, globalAdmin bool, deviceIDs []string) ([]AdminLogFilterOption, error) {
	if limit <= 0 {
		return nil, nil
	}
	deviceIDsJSON, err := json.Marshal(normalizeAdminLogDeviceIDs(deviceIDs))
	if err != nil {
		return nil, fmt.Errorf("encode admin log device ids filter: %w", err)
	}
	const adminSQLQuery = `
SELECT
	d.id::text,
	d.ecoflow_sn,
	COALESCE(d.product_name, ''),
	COALESCE(d.model, '')
FROM devices d
WHERE (
	$1 = ''
	OR lower(d.id::text) LIKE $2
	OR lower(d.ecoflow_sn) LIKE $2
	OR lower(COALESCE(d.product_name, '')) LIKE $2
	OR lower(COALESCE(d.model, '')) LIKE $2
)
AND (
	$4 = ''
	OR EXISTS (
		SELECT 1
		FROM provider_devices pd
		WHERE pd.device_id = d.id
		  AND pd.provider = $4
	)
)
AND (
	$5::jsonb = '[]'::jsonb
	OR EXISTS (
		SELECT 1
		FROM jsonb_array_elements_text($5::jsonb) AS filter(device_id)
		WHERE filter.device_id = d.id::text
	)
)
ORDER BY lower(COALESCE(NULLIF(d.product_name, ''), NULLIF(d.model, ''), d.ecoflow_sn, d.id::text)), d.id::text
LIMIT $3;
`
	const userSQLQuery = `
SELECT
	d.id::text,
	d.ecoflow_sn,
	COALESCE(d.product_name, ''),
	COALESCE(d.model, '')
FROM users u
JOIN user_devices ud ON ud.user_id = u.id
JOIN devices d ON d.id = ud.device_id
WHERE u.keycloak_subject = $6
  AND (
	$1 = ''
	OR lower(d.id::text) LIKE $2
	OR lower(d.ecoflow_sn) LIKE $2
	OR lower(COALESCE(d.product_name, '')) LIKE $2
	OR lower(COALESCE(d.model, '')) LIKE $2
)
AND (
	$4 = ''
	OR EXISTS (
		SELECT 1
		FROM provider_devices pd
		WHERE pd.device_id = d.id
		  AND pd.provider = $4
	)
)
AND (
	$5::jsonb = '[]'::jsonb
	OR EXISTS (
		SELECT 1
		FROM jsonb_array_elements_text($5::jsonb) AS filter(device_id)
		WHERE filter.device_id = d.id::text
	)
)
ORDER BY lower(COALESCE(NULLIF(d.product_name, ''), NULLIF(d.model, ''), d.ecoflow_sn, d.id::text)), d.id::text
LIMIT $3;
`
	return dbpool.RetryRead(ctx, func(ctx context.Context) ([]AdminLogFilterOption, error) {
		sqlQuery := adminSQLQuery
		args := []any{query, "%" + query + "%", limit, provider, string(deviceIDsJSON)}
		if !globalAdmin {
			sqlQuery = userSQLQuery
			args = append(args, userSubject)
		}
		rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
		if err != nil {
			return nil, fmt.Errorf("query admin log device filters: %w", err)
		}
		defer func() { _ = rows.Close() }()

		out := make([]AdminLogFilterOption, 0, limit)
		for rows.Next() {
			var deviceID, serial, productName, model string
			if err := rows.Scan(&deviceID, &serial, &productName, &model); err != nil {
				return nil, fmt.Errorf("scan admin log device filter row: %w", err)
			}
			if kind == "" || kind == "device" {
				out = append(out, AdminLogFilterOption{
					Kind:           "device",
					ID:             deviceID,
					Label:          adminLogFirstNonEmpty(productName, model, "Device "+shortID(deviceID)),
					SecondaryLabel: adminLogFirstNonEmpty(model, "UUID "+shortID(deviceID)),
					DeviceIDs:      []string{deviceID},
					Provider:       provider,
				})
			}
			if len(out) >= limit {
				break
			}
			if kind == "" || kind == "serial" {
				out = append(out, AdminLogFilterOption{
					Kind:           "serial",
					ID:             deviceID,
					Label:          serial,
					SecondaryLabel: adminLogFirstNonEmpty(productName, model, "Device "+shortID(deviceID)),
					DeviceIDs:      []string{deviceID},
					Provider:       provider,
				})
			}
			if len(out) >= limit {
				break
			}
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate admin log device filters: %w", err)
		}
		return out, nil
	})
}

func (s *PostgresStore) searchAdminLogUserFilters(ctx context.Context, query string, limit int, deviceIDs []string) ([]AdminLogFilterOption, error) {
	if limit <= 0 {
		return nil, nil
	}
	deviceIDsJSON, err := json.Marshal(normalizeAdminLogDeviceIDs(deviceIDs))
	if err != nil {
		return nil, fmt.Errorf("encode admin log user device ids filter: %w", err)
	}
	const sqlQuery = `
WITH matching_users AS (
	SELECT
		u.id,
		COALESCE(u.email, '') AS email,
		COALESCE(u.display_name, '') AS display_name
	FROM users u
	LEFT JOIN user_devices ud_filter ON ud_filter.user_id = u.id
	WHERE COALESCE(trim(u.email), '') <> ''
	  AND (
		$1 = ''
		OR lower(u.email) LIKE $2
		OR lower(COALESCE(u.display_name, '')) LIKE $2
		OR lower(u.keycloak_subject) LIKE $2
	  )
	  AND (
		$4::jsonb = '[]'::jsonb
		OR EXISTS (
			SELECT 1
			FROM jsonb_array_elements_text($4::jsonb) AS filter(device_id)
			WHERE filter.device_id = ud_filter.device_id::text
		)
	  )
	GROUP BY u.id, u.email, u.display_name
	ORDER BY lower(u.email), u.id::text
	LIMIT $3
)
SELECT
	mu.id::text,
	mu.email,
	mu.display_name,
	COALESCE(
		json_agg(ud.device_id::text ORDER BY ud.device_id::text) FILTER (WHERE ud.device_id IS NOT NULL),
		'[]'::json
	)::text AS device_ids_json
FROM matching_users mu
LEFT JOIN user_devices ud ON ud.user_id = mu.id
  AND (
	$4::jsonb = '[]'::jsonb
	OR EXISTS (
		SELECT 1
		FROM jsonb_array_elements_text($4::jsonb) AS filter(device_id)
		WHERE filter.device_id = ud.device_id::text
	)
  )
GROUP BY mu.id, mu.email, mu.display_name
ORDER BY lower(mu.email), mu.id::text;
`
	return dbpool.RetryRead(ctx, func(ctx context.Context) ([]AdminLogFilterOption, error) {
		rows, err := s.db.QueryContext(ctx, sqlQuery, query, "%"+query+"%", limit, string(deviceIDsJSON))
		if err != nil {
			return nil, fmt.Errorf("query admin log user filters: %w", err)
		}
		defer func() { _ = rows.Close() }()

		out := make([]AdminLogFilterOption, 0, limit)
		for rows.Next() {
			var userID, email, displayName, deviceIDsJSON string
			if err := rows.Scan(&userID, &email, &displayName, &deviceIDsJSON); err != nil {
				return nil, fmt.Errorf("scan admin log user filter row: %w", err)
			}
			deviceIDs, err := parseAdminLogDeviceIDsJSON(deviceIDsJSON)
			if err != nil {
				return nil, fmt.Errorf("decode admin log user device ids: %w", err)
			}
			secondaryLabel := adminLogFirstNonEmpty(displayName, fmt.Sprintf("%d devices", len(deviceIDs)))
			out = append(out, AdminLogFilterOption{
				Kind:           "user",
				ID:             userID,
				Label:          email,
				SecondaryLabel: secondaryLabel,
				DeviceIDs:      deviceIDs,
			})
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate admin log user filters: %w", err)
		}
		return out, nil
	})
}

func parseAdminLogDeviceIDsJSON(raw string) ([]string, error) {
	var deviceIDs []string
	if err := json.Unmarshal([]byte(raw), &deviceIDs); err != nil {
		return nil, err
	}
	sort.Strings(deviceIDs)
	return deviceIDs, nil
}

type jsonbMap map[string]any

func (m *jsonbMap) Scan(src any) error {
	if m == nil {
		return errors.New("jsonbMap scan target is nil")
	}
	switch value := src.(type) {
	case nil:
		*m = jsonbMap{}
		return nil
	case []byte:
		return m.unmarshal(value)
	case string:
		return m.unmarshal([]byte(value))
	default:
		return fmt.Errorf("scan jsonb map: unsupported source type %T", src)
	}
}

func (m *jsonbMap) unmarshal(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*m = jsonbMap{}
		return nil
	}
	decoded := make(map[string]any)
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("unmarshal jsonb map: %w", err)
	}
	*m = decoded
	return nil
}

func marshalJSONBMap(value map[string]any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func marshalProviderCredentialConfig(value map[string]any) ([]byte, error) {
	if len(value) == 0 {
		return []byte("{}"), nil
	}
	return marshalJSONBMap(cloneAnyMap(value))
}

type currentUserScanner interface {
	Scan(dest ...any) error
}

func scanCurrentUser(scanner currentUserScanner) (CurrentUser, error) {
	var (
		row              CurrentUser
		weatherLatitude  sql.NullFloat64
		weatherLongitude sql.NullFloat64
		lastLoginAt      sql.NullTime
	)
	if err := scanner.Scan(
		&row.ID,
		&row.KeycloakSubject,
		&row.Email,
		&row.EmailVerified,
		&row.DisplayName,
		&row.DisplayNameSource,
		&row.AvatarURL,
		&row.GivenName,
		&row.FamilyName,
		&row.Locale,
		&row.Timezone,
		&row.WeatherLocationEnabled,
		&row.WeatherLocationSource,
		&row.WeatherLocationLabel,
		&weatherLatitude,
		&weatherLongitude,
		&lastLoginAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return CurrentUser{}, err
	}
	row.HasWeatherLocation = weatherLatitude.Valid && weatherLongitude.Valid
	if weatherLatitude.Valid {
		row.WeatherLatitude = weatherLatitude.Float64
	}
	if weatherLongitude.Valid {
		row.WeatherLongitude = weatherLongitude.Float64
	}
	if lastLoginAt.Valid {
		row.LastLoginAt = lastLoginAt.Time
	}
	return row, nil
}

func utcNow() time.Time {
	return time.Now().UTC()
}

func normalizeWriteTime(value time.Time) time.Time {
	return value.UTC()
}
