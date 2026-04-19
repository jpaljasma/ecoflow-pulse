package dbmigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jpaljasma/ecoflow-pulse/internal/hashutil"
)

const (
	defaultMigrationsDir           = "deploy/db/migrations"
	defaultLockID            int64 = 884215901
	migrationChecksumVersion       = "xxh3-128"
)

type Config struct {
	DSN           string
	DBHost        string
	DBPort        int
	DBUser        string
	DBPassword    string
	DBName        string
	DBSSLMode     string
	MigrationsDir string
	RolloutEnv    string
	RequireBackup bool
	BackupRef     string
	ForwardOnly   bool
	LockID        int64
	NowFn         func() time.Time
}

type Migration struct {
	Version  string
	FileName string
	Path     string
	Checksum string
	SQL      string
}

type Result struct {
	AppliedVersions []string
	AdoptedVersions []string
	SkippedVersions []string
}

func DefaultConfig() Config {
	return Config{
		DBPort:        5432,
		DBSSLMode:     "disable",
		MigrationsDir: defaultMigrationsDir,
		ForwardOnly:   true,
		LockID:        defaultLockID,
		NowFn:         time.Now,
	}
}

func (c Config) normalized() Config {
	out := c
	if out.DBPort <= 0 {
		out.DBPort = 5432
	}
	if strings.TrimSpace(out.DBSSLMode) == "" {
		out.DBSSLMode = "disable"
	}
	if strings.TrimSpace(out.MigrationsDir) == "" {
		out.MigrationsDir = defaultMigrationsDir
	}
	if out.LockID == 0 {
		out.LockID = defaultLockID
	}
	if out.NowFn == nil {
		out.NowFn = time.Now
	}
	return out
}

func (c Config) Validate() error {
	cfg := c.normalized()
	if !cfg.ForwardOnly {
		return errors.New("migration rollout path is forward-only")
	}
	switch strings.TrimSpace(strings.ToLower(cfg.RolloutEnv)) {
	case "dev", "staging", "prod", "local":
	default:
		return fmt.Errorf("unsupported DB_MIGRATION_ENVIRONMENT %q", cfg.RolloutEnv)
	}
	if cfg.RequireBackup && strings.TrimSpace(cfg.BackupRef) == "" {
		return errors.New("DB_MIGRATION_BACKUP_REF is required when DB_MIGRATION_REQUIRE_BACKUP=true")
	}
	if strings.TrimSpace(cfg.MigrationsDir) == "" {
		return errors.New("DB_MIGRATIONS_DIR is required")
	}
	if strings.TrimSpace(cfg.DSN) == "" {
		if strings.TrimSpace(cfg.DBHost) == "" ||
			strings.TrimSpace(cfg.DBUser) == "" ||
			strings.TrimSpace(cfg.DBPassword) == "" ||
			strings.TrimSpace(cfg.DBName) == "" {
			return errors.New("database connection settings are incomplete")
		}
	}
	return nil
}

func (c Config) ConnectionString() string {
	cfg := c.normalized()
	if strings.TrimSpace(cfg.DSN) != "" {
		return strings.TrimSpace(cfg.DSN)
	}
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		strings.TrimSpace(cfg.DBHost),
		cfg.DBPort,
		strings.TrimSpace(cfg.DBUser),
		cfg.DBPassword,
		strings.TrimSpace(cfg.DBName),
		strings.TrimSpace(cfg.DBSSLMode),
	)
}

func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

func LoadMigrations(dir string) ([]Migration, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("migrations directory is required")
	}
	if !filepath.IsAbs(dir) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		dir = filepath.Join(cwd, dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", dir, err)
	}
	out := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		path := filepath.Join(dir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", path, err)
		}
		sqlBody := strings.TrimSpace(string(body))
		if sqlBody == "" {
			continue
		}
		version := strings.TrimSuffix(name, ".up.sql")
		out = append(out, Migration{
			Version:  version,
			FileName: name,
			Path:     path,
			Checksum: migrationChecksum(sqlBody),
			SQL:      sqlBody,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Version < out[j].Version
	})
	if len(out) == 0 {
		return nil, fmt.Errorf("no .up.sql migrations found in %q", dir)
	}
	return out, nil
}

func Run(ctx context.Context, db *sql.DB, cfg Config) (Result, error) {
	if db == nil {
		return Result{}, errors.New("database is required")
	}
	cfg = cfg.normalized()
	if err := cfg.Validate(); err != nil {
		return Result{}, err
	}
	migrations, err := LoadMigrations(cfg.MigrationsDir)
	if err != nil {
		return Result{}, err
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("open dedicated postgres connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", cfg.LockID); err != nil {
		return Result{}, fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", cfg.LockID)
	}()

	if _, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migration_rollouts (
	version TEXT PRIMARY KEY,
	checksum TEXT NOT NULL,
	file_name TEXT NOT NULL,
	rollout_env TEXT NOT NULL,
	backup_ref TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL
)`); err != nil {
		return Result{}, fmt.Errorf("ensure schema_migration_rollouts table: %w", err)
	}

	var result Result
	for _, migration := range migrations {
		var existingChecksum string
		err := conn.QueryRowContext(
			ctx,
			"SELECT checksum FROM schema_migration_rollouts WHERE version = $1",
			migration.Version,
		).Scan(&existingChecksum)
		switch {
		case err == nil:
			if !migrationChecksumMatches(existingChecksum, migration.SQL) {
				return result, fmt.Errorf(
					"migration %s already applied with checksum %s (expected %s)",
					migration.Version,
					existingChecksum,
					migration.Checksum,
				)
			}
			result.SkippedVersions = append(result.SkippedVersions, migration.Version)
			continue
		case errors.Is(err, sql.ErrNoRows):
		default:
			return result, fmt.Errorf("query existing migration %s: %w", migration.Version, err)
		}

		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return result, fmt.Errorf("begin migration %s transaction: %w", migration.Version, err)
		}
		if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
			_ = tx.Rollback()
			adopted, adoptErr := tryRepairOrAdoptPreexistingLocalMigration(ctx, conn, cfg, migration)
			if adoptErr != nil {
				return result, fmt.Errorf("check local migration adoption for %s: %w", migration.Version, adoptErr)
			}
			if adopted {
				result.AdoptedVersions = append(result.AdoptedVersions, migration.Version)
				continue
			}
			return result, fmt.Errorf("apply migration %s: %w", migration.Version, err)
		}
		if err := recordAppliedMigration(ctx, tx, cfg, migration); err != nil {
			_ = tx.Rollback()
			return result, fmt.Errorf("record migration %s rollout: %w", migration.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return result, fmt.Errorf("commit migration %s transaction: %w", migration.Version, err)
		}
		result.AppliedVersions = append(result.AppliedVersions, migration.Version)
	}
	return result, nil
}

func tryRepairOrAdoptPreexistingLocalMigration(ctx context.Context, conn *sql.Conn, cfg Config, migration Migration) (bool, error) {
	if !canAttemptLocalLegacyAdoption(cfg, migration) {
		return false, nil
	}
	ready, err := legacyLocalMigrationPresent(ctx, conn, migration.Version)
	if err != nil {
		return false, err
	}
	if ready {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return false, fmt.Errorf("begin adoption transaction: %w", err)
		}
		if err := recordAppliedMigration(ctx, tx, cfg, migration); err != nil {
			_ = tx.Rollback()
			return false, err
		}
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit adoption transaction: %w", err)
		}
		return true, nil
	}

	convertible, err := legacyLocalPlainRollupTablesPresent(ctx, conn)
	if err != nil {
		return false, err
	}
	if !convertible {
		return false, nil
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin legacy repair transaction: %w", err)
	}
	if err := convertLegacyLocalRollupTables(ctx, tx); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if err := recordAppliedMigration(ctx, tx, cfg, migration); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit legacy repair transaction: %w", err)
	}
	return true, nil
}

func canAttemptLocalLegacyAdoption(cfg Config, migration Migration) bool {
	if !strings.EqualFold(strings.TrimSpace(cfg.RolloutEnv), "local") {
		return false
	}
	switch migration.Version {
	case "000004_m3_rollups_hypertables_schema",
		"000006_m3_rollup_zero_sample_solar_buckets",
		"000007_m3_rollup_explicit_energy_buckets",
		"000008_m3_rollup_ac_output_power_columns",
		"000009_m1_user_profile_preferences":
		return true
	default:
		return false
	}
}

func legacyLocalMigrationPresent(ctx context.Context, conn *sql.Conn, version string) (bool, error) {
	switch version {
	case "000004_m3_rollups_hypertables_schema":
		return legacyLocalRollupSchemaPresent(ctx, conn)
	case "000006_m3_rollup_zero_sample_solar_buckets":
		return legacyLocalZeroSampleConstraintPresent(ctx, conn)
	case "000007_m3_rollup_explicit_energy_buckets":
		return legacyLocalExplicitEnergyColumnsPresent(ctx, conn)
	case "000008_m3_rollup_ac_output_power_columns":
		return legacyLocalAcOutputPowerColumnsPresent(ctx, conn)
	case "000009_m1_user_profile_preferences":
		return legacyLocalUserProfileColumnsPresent(ctx, conn)
	default:
		return false, nil
	}
}

func legacyLocalRollupSchemaPresent(ctx context.Context, conn *sql.Conn) (bool, error) {
	var hypertableCount int
	if err := conn.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM timescaledb_information.hypertables
WHERE hypertable_name IN ('telemetry_rollup_minute', 'telemetry_rollup_hour', 'telemetry_rollup_day')
`).Scan(&hypertableCount); err != nil {
		return false, fmt.Errorf("query existing hypertables: %w", err)
	}
	return hypertableCount == 3, nil
}

func legacyLocalPlainRollupTablesPresent(ctx context.Context, conn *sql.Conn) (bool, error) {
	var tableCount int
	if err := conn.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
  AND c.relkind = 'r'
  AND c.relname IN ('telemetry_rollup_minute', 'telemetry_rollup_hour', 'telemetry_rollup_day')
`).Scan(&tableCount); err != nil {
		return false, fmt.Errorf("query existing plain rollup tables: %w", err)
	}
	return tableCount == 3, nil
}

func legacyLocalZeroSampleConstraintPresent(ctx context.Context, conn *sql.Conn) (bool, error) {
	var count int
	if err := conn.QueryRowContext(ctx, `
SELECT COUNT(DISTINCT conname)
FROM pg_constraint
WHERE conname IN (
	'chk_rollup_minute_sample_count_nonnegative',
	'chk_rollup_hour_sample_count_nonnegative',
	'chk_rollup_day_sample_count_nonnegative'
)
`).Scan(&count); err != nil {
		return false, fmt.Errorf("query zero-sample rollup constraints: %w", err)
	}
	return count == 3, nil
}

func legacyLocalExplicitEnergyColumnsPresent(ctx context.Context, conn *sql.Conn) (bool, error) {
	var count int
	if err := conn.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name IN ('telemetry_rollup_minute', 'telemetry_rollup_hour', 'telemetry_rollup_day')
  AND column_name IN (
	'ac_input_energy_wh',
	'ac_output_energy_wh',
	'dc_output_energy_wh',
	'load_energy_wh',
	'battery_charge_energy_wh',
	'battery_discharge_energy_wh'
)
`).Scan(&count); err != nil {
		return false, fmt.Errorf("query explicit energy rollup columns: %w", err)
	}
	return count == 18, nil
}

func legacyLocalAcOutputPowerColumnsPresent(ctx context.Context, conn *sql.Conn) (bool, error) {
	var count int
	if err := conn.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name IN ('telemetry_rollup_minute', 'telemetry_rollup_hour', 'telemetry_rollup_day')
  AND column_name IN ('ac_output_avg_w', 'ac_output_max_w')
`).Scan(&count); err != nil {
		return false, fmt.Errorf("query ac-output power columns: %w", err)
	}
	return count == 6, nil
}

func legacyLocalUserProfileColumnsPresent(ctx context.Context, conn *sql.Conn) (bool, error) {
	var count int
	if err := conn.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'users'
  AND column_name IN (
	'email_verified',
	'given_name',
	'family_name',
	'locale',
	'timezone',
	'weather_location_enabled',
	'weather_location_source',
	'weather_location_label',
	'weather_latitude',
	'weather_longitude',
	'display_name_source',
	'last_login_at'
)
`).Scan(&count); err != nil {
		return false, fmt.Errorf("query user profile preference columns: %w", err)
	}
	if count != 12 {
		return false, nil
	}
	var constraintCount int
	if err := conn.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM pg_constraint
WHERE conname IN ('chk_users_display_name_source', 'chk_users_weather_location_source')
`).Scan(&constraintCount); err != nil {
		return false, fmt.Errorf("query user profile preference constraints: %w", err)
	}
	return constraintCount == 2, nil
}

func convertLegacyLocalRollupTables(ctx context.Context, tx *sql.Tx) error {
	statements := []string{
		`SELECT create_hypertable(
			'telemetry_rollup_minute',
			'bucket_start',
			if_not_exists => TRUE,
			migrate_data => TRUE,
			chunk_time_interval => INTERVAL '1 day'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rollup_minute_device_bucket
			ON telemetry_rollup_minute (device_id, bucket_start DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_rollup_minute_provider_bucket
			ON telemetry_rollup_minute (provider, provider_device_id, bucket_start DESC)`,
		`SELECT create_hypertable(
			'telemetry_rollup_hour',
			'bucket_start',
			if_not_exists => TRUE,
			migrate_data => TRUE,
			chunk_time_interval => INTERVAL '7 days'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rollup_hour_device_bucket
			ON telemetry_rollup_hour (device_id, bucket_start DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_rollup_hour_provider_bucket
			ON telemetry_rollup_hour (provider, provider_device_id, bucket_start DESC)`,
		`SELECT create_hypertable(
			'telemetry_rollup_day',
			'bucket_start',
			if_not_exists => TRUE,
			migrate_data => TRUE,
			chunk_time_interval => INTERVAL '30 days'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rollup_day_device_bucket
			ON telemetry_rollup_day (device_id, bucket_start DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_rollup_day_provider_bucket
			ON telemetry_rollup_day (provider, provider_device_id, bucket_start DESC)`,
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("convert legacy local rollup tables: %w", err)
		}
	}
	return nil
}

type execContexter interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func recordAppliedMigration(ctx context.Context, execer execContexter, cfg Config, migration Migration) error {
	_, err := execer.ExecContext(
		ctx,
		`INSERT INTO schema_migration_rollouts (
			version, checksum, file_name, rollout_env, backup_ref, applied_at
		) VALUES ($1, $2, $3, $4, $5, $6)`,
		migration.Version,
		migration.Checksum,
		migration.FileName,
		strings.ToLower(strings.TrimSpace(cfg.RolloutEnv)),
		strings.TrimSpace(cfg.BackupRef),
		cfg.NowFn().UTC(),
	)
	return err
}

func migrationChecksum(sqlBody string) string {
	return migrationChecksumVersion + ":" + hashutil.XXH3Hex128(sqlBody)
}

func migrationChecksumMatches(storedChecksum string, sqlBody string) bool {
	storedChecksum = strings.TrimSpace(storedChecksum)
	if storedChecksum == "" {
		return false
	}
	if storedChecksum == migrationChecksum(sqlBody) {
		return true
	}
	return storedChecksum == legacyMigrationChecksum(sqlBody)
}

func legacyMigrationChecksum(sqlBody string) string {
	sum := sha256.Sum256([]byte(sqlBody))
	return fmt.Sprintf("%x", sum[:])
}
