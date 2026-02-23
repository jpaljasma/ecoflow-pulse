package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
)

const (
	defaultUserSubject = "jpaljasma@gmail.com"
	defaultProvider    = controlplane.ProviderEcoFlow
)

var defaultSeedSerials = []string{
	"R351ZABAPH331057",
	"Y711ZABA9H2P0294",
}

type deviceSeed struct {
	SN          string
	ProductName string
	Model       string
}

var knownSeedDevices = map[string]deviceSeed{
	"R351ZABAPH331057": {
		SN:          "R351ZABAPH331057",
		ProductName: "Kitchen Delta 2 Max",
		Model:       "DELTA 2 Max",
	},
	"Y711ZABA9H2P0294": {
		SN:          "Y711ZABA9H2P0294",
		ProductName: "DPU A 12 kWh",
		Model:       "DELTA Pro Ultra",
	},
}

type seedConfig struct {
	DSN         string
	UserSubject string
	UserEmail   string
	Provider    string
	AccessKey   string
	SecretKey   string
	Devices     []deviceSeed
}

type seedResult struct {
	UserID         string
	CredentialID   string
	AccessKeyMask  string
	SeededSerialSN []string
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := configFromEnv()
	if err != nil {
		log.Error("invalid seed configuration", "error", err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		log.Error("open postgres connection failed", "error", err.Error())
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		log.Error("ping postgres failed", "error", err.Error())
		os.Exit(1)
	}

	out, err := seedDevData(ctx, db, cfg)
	if err != nil {
		log.Error("dev seed failed", "error", err.Error())
		os.Exit(1)
	}

	log.Info("dev seed completed",
		"user_subject", cfg.UserSubject,
		"user_email", cfg.UserEmail,
		"provider", cfg.Provider,
		"credential_id", out.CredentialID,
		"access_key_mask", out.AccessKeyMask,
		"device_count", len(out.SeededSerialSN),
		"devices", strings.Join(out.SeededSerialSN, ","),
	)
}

func configFromEnv() (seedConfig, error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	if dsn == "" {
		return seedConfig{}, errors.New("CONTROL_PLANE_DB_DSN is required")
	}

	accessKey := strings.TrimSpace(os.Getenv("ECOFLOW_DEV_ACCESS_KEY"))
	if accessKey == "" {
		return seedConfig{}, errors.New("ECOFLOW_DEV_ACCESS_KEY is required")
	}
	secretKey := strings.TrimSpace(os.Getenv("ECOFLOW_DEV_SECRET_KEY"))
	if secretKey == "" {
		return seedConfig{}, errors.New("ECOFLOW_DEV_SECRET_KEY is required")
	}

	userSubject := strings.TrimSpace(os.Getenv("ECOFLOW_DEV_USER_SUBJECT"))
	if userSubject == "" {
		userSubject = defaultUserSubject
	}
	userEmail := strings.TrimSpace(os.Getenv("ECOFLOW_DEV_USER_EMAIL"))
	if userEmail == "" {
		userEmail = userSubject
	}

	provider := controlplane.NormalizeProvider(os.Getenv("ECOFLOW_DEV_PROVIDER"))
	if provider == "" {
		provider = defaultProvider
	}
	if !controlplane.IsSupportedProvider(provider) {
		return seedConfig{}, fmt.Errorf("unsupported provider %q", provider)
	}

	serials, err := parseSeedSerials(strings.TrimSpace(os.Getenv("ECOFLOW_DEV_SEED_SNS")))
	if err != nil {
		return seedConfig{}, err
	}
	devices := buildSeedDevices(serials)
	if len(devices) == 0 {
		return seedConfig{}, errors.New("no seed serial numbers configured")
	}

	return seedConfig{
		DSN:         dsn,
		UserSubject: userSubject,
		UserEmail:   userEmail,
		Provider:    provider,
		AccessKey:   accessKey,
		SecretKey:   secretKey,
		Devices:     devices,
	}, nil
}

func parseSeedSerials(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		out := make([]string, 0, len(defaultSeedSerials))
		for _, sn := range defaultSeedSerials {
			out = append(out, strings.ToUpper(strings.TrimSpace(sn)))
		}
		return out, nil
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	})
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		sn := strings.ToUpper(strings.TrimSpace(part))
		if sn == "" {
			continue
		}
		if _, ok := seen[sn]; ok {
			continue
		}
		seen[sn] = struct{}{}
		out = append(out, sn)
	}
	if len(out) == 0 {
		return nil, errors.New("ECOFLOW_DEV_SEED_SNS contains no valid serial numbers")
	}
	return out, nil
}

func buildSeedDevices(serials []string) []deviceSeed {
	out := make([]deviceSeed, 0, len(serials))
	for _, serial := range serials {
		sn := strings.ToUpper(strings.TrimSpace(serial))
		if sn == "" {
			continue
		}
		if known, ok := knownSeedDevices[sn]; ok {
			out = append(out, known)
			continue
		}
		out = append(out, deviceSeed{
			SN:          sn,
			ProductName: "EcoFlow Device",
			Model:       "Unknown",
		})
	}
	return out
}

func seedDevData(ctx context.Context, db *sql.DB, cfg seedConfig) (seedResult, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return seedResult{}, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC()

	userID, err := upsertUser(ctx, tx, cfg.UserSubject, cfg.UserEmail, now)
	if err != nil {
		return seedResult{}, err
	}
	credentialID, accessMask, err := upsertProviderCredential(ctx, tx, userID, cfg, now)
	if err != nil {
		return seedResult{}, err
	}

	seeded := make([]string, 0, len(cfg.Devices))
	for _, binding := range cfg.Devices {
		deviceID, err := upsertDevice(ctx, tx, binding, cfg.Provider, now)
		if err != nil {
			return seedResult{}, err
		}
		if err := upsertUserDevice(ctx, tx, userID, deviceID, now); err != nil {
			return seedResult{}, err
		}
		if err := upsertProviderDevice(ctx, tx, deviceID, credentialID, cfg.Provider, binding, now); err != nil {
			return seedResult{}, err
		}
		seeded = append(seeded, binding.SN)
	}

	if err := tx.Commit(); err != nil {
		return seedResult{}, fmt.Errorf("commit tx: %w", err)
	}
	committed = true

	return seedResult{
		UserID:         userID,
		CredentialID:   credentialID,
		AccessKeyMask:  accessMask,
		SeededSerialSN: seeded,
	}, nil
}

func upsertUser(ctx context.Context, tx *sql.Tx, userSubject string, userEmail string, now time.Time) (string, error) {
	const query = `
INSERT INTO users (keycloak_subject, email, display_name, created_at, updated_at)
VALUES ($1, $2, $3, $4, $4)
ON CONFLICT (keycloak_subject) DO UPDATE
SET email = COALESCE(users.email, EXCLUDED.email),
    display_name = COALESCE(users.display_name, EXCLUDED.display_name),
    updated_at = EXCLUDED.updated_at
RETURNING id::text;
`
	var userID string
	if err := tx.QueryRowContext(ctx, query, userSubject, userEmail, "Dev Seed User", now).Scan(&userID); err != nil {
		return "", fmt.Errorf("upsert user: %w", err)
	}
	return userID, nil
}

func upsertProviderCredential(ctx context.Context, tx *sql.Tx, userID string, cfg seedConfig, now time.Time) (string, string, error) {
	const query = `
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
VALUES ($1, $2, $3, $4, $5, $6, TRUE, $7, $7)
ON CONFLICT (user_id, provider, access_key_hash) DO UPDATE
SET access_key_ciphertext = EXCLUDED.access_key_ciphertext,
	secret_key_ciphertext = EXCLUDED.secret_key_ciphertext,
	access_key_mask = EXCLUDED.access_key_mask,
	is_active = TRUE,
	updated_at = EXCLUDED.updated_at
RETURNING id::text, access_key_mask;
`
	mask := controlplane.MaskAccessKey(cfg.AccessKey)
	hash := controlplane.HashAccessKey(cfg.AccessKey)
	var credentialID string
	var accessMask string
	if err := tx.QueryRowContext(
		ctx,
		query,
		userID,
		cfg.Provider,
		[]byte(cfg.AccessKey),
		[]byte(cfg.SecretKey),
		hash,
		mask,
		now,
	).Scan(&credentialID, &accessMask); err != nil {
		return "", "", fmt.Errorf("upsert provider credential: %w", err)
	}
	return credentialID, accessMask, nil
}

func upsertDevice(ctx context.Context, tx *sql.Tx, binding deviceSeed, provider string, now time.Time) (string, error) {
	const query = `
INSERT INTO devices (ecoflow_sn, product_name, model, metadata, created_at, updated_at)
VALUES ($1, $2, $3, $4::jsonb, $5, $5)
ON CONFLICT (ecoflow_sn) DO UPDATE
SET product_name = CASE
		WHEN length(trim(COALESCE(devices.product_name, ''))) = 0 THEN EXCLUDED.product_name
		ELSE devices.product_name
	END,
	model = CASE
		WHEN length(trim(COALESCE(devices.model, ''))) = 0 THEN EXCLUDED.model
		ELSE devices.model
	END,
	metadata = CASE
		WHEN devices.metadata = '{}'::jsonb THEN EXCLUDED.metadata
		ELSE devices.metadata
	END,
	updated_at = EXCLUDED.updated_at
RETURNING id::text;
`
	metadataBytes, err := json.Marshal(map[string]any{
		"seed_source": "explicit-dev-seed",
		"provider":    provider,
	})
	if err != nil {
		return "", fmt.Errorf("marshal device metadata: %w", err)
	}

	var deviceID string
	if err := tx.QueryRowContext(
		ctx,
		query,
		binding.SN,
		binding.ProductName,
		binding.Model,
		string(metadataBytes),
		now,
	).Scan(&deviceID); err != nil {
		return "", fmt.Errorf("upsert device %s: %w", binding.SN, err)
	}
	return deviceID, nil
}

func upsertUserDevice(ctx context.Context, tx *sql.Tx, userID string, deviceID string, now time.Time) error {
	const query = `
INSERT INTO user_devices (user_id, device_id, role, created_at, updated_at)
VALUES ($1, $2, 'admin', $3, $3)
ON CONFLICT (user_id, device_id) DO UPDATE
SET role = 'admin',
	updated_at = EXCLUDED.updated_at;
`
	if _, err := tx.ExecContext(ctx, query, userID, deviceID, now); err != nil {
		return fmt.Errorf("upsert user_devices: %w", err)
	}
	return nil
}

func upsertProviderDevice(ctx context.Context, tx *sql.Tx, deviceID string, credentialID string, provider string, binding deviceSeed, now time.Time) error {
	const query = `
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
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, TRUE, 'active', $9, $9)
ON CONFLICT (provider, provider_device_id) DO UPDATE
SET device_id = EXCLUDED.device_id,
	credential_id = EXCLUDED.credential_id,
	product_name = CASE
		WHEN length(trim(COALESCE(provider_devices.product_name, ''))) = 0 THEN EXCLUDED.product_name
		ELSE provider_devices.product_name
	END,
	model = CASE
		WHEN length(trim(COALESCE(provider_devices.model, ''))) = 0 THEN EXCLUDED.model
		ELSE provider_devices.model
	END,
	capabilities = CASE
		WHEN provider_devices.capabilities = '{}'::jsonb THEN EXCLUDED.capabilities
		ELSE provider_devices.capabilities
	END,
	metadata = CASE
		WHEN provider_devices.metadata = '{}'::jsonb THEN EXCLUDED.metadata
		ELSE provider_devices.metadata
	END,
	is_active = TRUE,
	ingest_desired_state = 'active',
	updated_at = EXCLUDED.updated_at;
`
	capabilitiesBytes, err := json.Marshal(map[string]any{
		"source": "seed",
	})
	if err != nil {
		return fmt.Errorf("marshal provider capabilities: %w", err)
	}
	metadataBytes, err := json.Marshal(map[string]any{
		"seed_source": "explicit-dev-seed",
	})
	if err != nil {
		return fmt.Errorf("marshal provider metadata: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		query,
		deviceID,
		provider,
		binding.SN,
		credentialID,
		binding.ProductName,
		binding.Model,
		string(capabilitiesBytes),
		string(metadataBytes),
		now,
	); err != nil {
		return fmt.Errorf("upsert provider device %s: %w", binding.SN, err)
	}
	return nil
}
