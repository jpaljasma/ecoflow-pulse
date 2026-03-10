package main

import "testing"

func TestLoadMigrationConfigFromEnv(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "postgres://example")
	t.Setenv("DB_MIGRATION_DB_HOST", "db.local")
	t.Setenv("DB_MIGRATION_DB_PORT", "15432")
	t.Setenv("DB_MIGRATION_DB_USER", "pulse")
	t.Setenv("DB_MIGRATION_DB_PASSWORD", "secret")
	t.Setenv("DB_MIGRATION_DB_NAME", "pulse_test")
	t.Setenv("DB_MIGRATION_DB_SSLMODE", "require")
	t.Setenv("DB_MIGRATIONS_DIR", "/tmp/migrations")
	t.Setenv("DB_MIGRATION_ENVIRONMENT", "staging")
	t.Setenv("DB_MIGRATION_REQUIRE_BACKUP", "true")
	t.Setenv("DB_MIGRATION_BACKUP_REF", "backup-1")
	t.Setenv("DB_MIGRATION_FORWARD_ONLY", "false")

	cfg := loadMigrationConfigFromEnv()
	if cfg.DSN != "postgres://example" || cfg.DBHost != "db.local" || cfg.DBPort != 15432 || cfg.DBUser != "pulse" || cfg.DBPassword != "secret" {
		t.Fatalf("db config mismatch: %+v", cfg)
	}
	if cfg.DBName != "pulse_test" || cfg.DBSSLMode != "require" || cfg.MigrationsDir != "/tmp/migrations" || cfg.RolloutEnv != "staging" {
		t.Fatalf("rollout config mismatch: %+v", cfg)
	}
	if !cfg.RequireBackup || cfg.BackupRef != "backup-1" || cfg.ForwardOnly {
		t.Fatalf("backup/forward policy mismatch: %+v", cfg)
	}
}
