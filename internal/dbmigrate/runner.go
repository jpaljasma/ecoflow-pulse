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
)

const (
	defaultMigrationsDir       = "deploy/db/migrations"
	defaultLockID        int64 = 884215901
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
		sum := sha256.Sum256([]byte(sqlBody))
		out = append(out, Migration{
			Version:  version,
			FileName: name,
			Path:     path,
			Checksum: fmt.Sprintf("%x", sum[:]),
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
			if existingChecksum != migration.Checksum {
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
			return result, fmt.Errorf("apply migration %s: %w", migration.Version, err)
		}
		if _, err := tx.ExecContext(
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
		); err != nil {
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
