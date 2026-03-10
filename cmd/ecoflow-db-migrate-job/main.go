package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jpaljasma/ecoflow-pulse/internal/dbmigrate"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
)

func main() {
	logCfg := pulselog.DefaultServiceConfig("db-migrate-job")
	logCfg.Level = pulselog.ParseLevel(os.Getenv("LOG_LEVEL"), slog.LevelInfo)
	logCfg.AsyncEnabled = !runtimecfg.Bool("LOG_ASYNC_DISABLED", false)
	logCfg.AsyncQueueSize = runtimecfg.IntMin("LOG_ASYNC_QUEUE_SIZE", logCfg.AsyncQueueSize, 128)
	logCfg.AsyncBypassLevel = pulselog.ParseLevel(runtimecfg.EnvOrDefault("LOG_ASYNC_BYPASS_LEVEL", "warn"), slog.LevelWarn)

	log, asyncLogHandler, err := pulselog.BuildServiceLogger(logCfg)
	if err != nil {
		_, _ = os.Stderr.WriteString("init logger failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() {
		if asyncLogHandler != nil {
			asyncLogHandler.Close()
		}
	}()

	cfg := loadMigrationConfigFromEnv()

	if err := cfg.Validate(); err != nil {
		log.Error("invalid migration rollout config", "error", err.Error())
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	db, err := dbmigrate.Open(ctx, cfg)
	if err != nil {
		log.Error("open migration database failed", "error", err.Error())
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	result, err := dbmigrate.Run(ctx, db, cfg)
	if err != nil {
		log.Error("migration rollout failed",
			"error", err.Error(),
			"rollout_env", cfg.RolloutEnv,
			"migrations_dir", cfg.MigrationsDir,
			"require_backup", cfg.RequireBackup,
			"backup_ref", cfg.BackupRef,
			"forward_only", cfg.ForwardOnly,
		)
		os.Exit(1)
	}

	log.Info("migration rollout completed",
		"rollout_env", cfg.RolloutEnv,
		"migrations_dir", cfg.MigrationsDir,
		"require_backup", cfg.RequireBackup,
		"backup_ref", cfg.BackupRef,
		"forward_only", cfg.ForwardOnly,
		"applied_versions", strings.Join(result.AppliedVersions, ","),
		"skipped_versions", strings.Join(result.SkippedVersions, ","),
	)
}

func loadMigrationConfigFromEnv() dbmigrate.Config {
	cfg := dbmigrate.DefaultConfig()
	cfg.DSN = strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	cfg.DBHost = strings.TrimSpace(os.Getenv("DB_MIGRATION_DB_HOST"))
	cfg.DBPort = runtimecfg.IntMin("DB_MIGRATION_DB_PORT", cfg.DBPort, 1)
	cfg.DBUser = strings.TrimSpace(os.Getenv("DB_MIGRATION_DB_USER"))
	cfg.DBPassword = os.Getenv("DB_MIGRATION_DB_PASSWORD")
	cfg.DBName = strings.TrimSpace(runtimecfg.EnvOrDefault("DB_MIGRATION_DB_NAME", "pulse"))
	cfg.DBSSLMode = strings.TrimSpace(runtimecfg.EnvOrDefault("DB_MIGRATION_DB_SSLMODE", cfg.DBSSLMode))
	cfg.MigrationsDir = strings.TrimSpace(runtimecfg.EnvOrDefault("DB_MIGRATIONS_DIR", "/app/deploy/db/migrations"))
	cfg.RolloutEnv = strings.TrimSpace(runtimecfg.EnvOrDefault("DB_MIGRATION_ENVIRONMENT", "dev"))
	cfg.RequireBackup = runtimecfg.Bool("DB_MIGRATION_REQUIRE_BACKUP", false)
	cfg.BackupRef = strings.TrimSpace(os.Getenv("DB_MIGRATION_BACKUP_REF"))
	cfg.ForwardOnly = runtimecfg.Bool("DB_MIGRATION_FORWARD_ONLY", true)
	return cfg
}
