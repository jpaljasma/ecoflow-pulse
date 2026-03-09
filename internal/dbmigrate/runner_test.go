package dbmigrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidateRequiresBackupRefWhenConfigured(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.DSN = "postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable"
	cfg.RolloutEnv = "staging"
	cfg.RequireBackup = true

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected backup-ref validation error")
	}
}

func TestConfigConnectionStringBuildsFromParts(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.RolloutEnv = "dev"
	cfg.DBHost = "pulse-platform-core-rw"
	cfg.DBPort = 5432
	cfg.DBUser = "pulse"
	cfg.DBPassword = "secret"
	cfg.DBName = "pulse"

	got := cfg.ConnectionString()
	want := "host=pulse-platform-core-rw port=5432 user=pulse password=secret dbname=pulse sslmode=disable"
	if got != want {
		t.Fatalf("connection string mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestLoadMigrationsLoadsAndSortsUpFilesOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := map[string]string{
		"000002_second.up.sql":  "CREATE TABLE second(id int);",
		"000001_first.up.sql":   "CREATE TABLE first(id int);",
		"000003_third.down.sql": "DROP TABLE third;",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write migration %s: %v", name, err)
		}
	}

	migrations, err := LoadMigrations(dir)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("expected 2 up migrations, got %d", len(migrations))
	}
	if migrations[0].Version != "000001_first" || migrations[1].Version != "000002_second" {
		t.Fatalf("unexpected migration order: %#v", migrations)
	}
	if migrations[0].Checksum == "" || migrations[1].Checksum == "" {
		t.Fatalf("expected migration checksums to be populated")
	}
}
