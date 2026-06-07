package dbmigrate

import (
	"crypto/sha256"
	"fmt"
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

func TestConfigValidateAllowsApplianceRolloutEnvironment(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.DSN = "postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable"
	cfg.RolloutEnv = "appliance"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected appliance rollout env to validate: %v", err)
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
	if got, want := migrations[0].Checksum[:len(migrationChecksumVersion)+1], migrationChecksumVersion+":"; got != want {
		t.Fatalf("expected versioned xxh3 checksum prefix, got %q want %q", got, want)
	}
}

func TestCanAttemptLocalLegacyAdoption(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.RolloutEnv = "local"
	migration := Migration{Version: "000004_m3_rollups_hypertables_schema"}
	if !canAttemptLocalLegacyAdoption(cfg, migration) {
		t.Fatalf("expected local legacy rollup migration to be adoptable")
	}

	cfg.RolloutEnv = "dev"
	if canAttemptLocalLegacyAdoption(cfg, migration) {
		t.Fatalf("did not expect non-local rollout env to allow adoption")
	}

	cfg.RolloutEnv = "local"
	migration.Version = "000005_m3_rollup_retention_policies"
	if canAttemptLocalLegacyAdoption(cfg, migration) {
		t.Fatalf("did not expect retention policy migration to allow adoption")
	}

	migration.Version = "000006_m3_rollup_zero_sample_solar_buckets"
	if !canAttemptLocalLegacyAdoption(cfg, migration) {
		t.Fatalf("expected zero-sample migration to allow local adoption")
	}

	migration.Version = "000009_m1_user_profile_preferences"
	if !canAttemptLocalLegacyAdoption(cfg, migration) {
		t.Fatalf("expected user-profile migration to allow local adoption")
	}
}

func TestMigrationChecksumMatchesLegacySHA256(t *testing.T) {
	t.Parallel()

	sqlBody := "CREATE TABLE foo(id bigint primary key);"
	legacy := sha256.Sum256([]byte(sqlBody))
	legacyHex := fmt.Sprintf("%x", legacy[:])

	if !migrationChecksumMatches(legacyHex, sqlBody) {
		t.Fatalf("expected legacy sha256 checksum to remain accepted")
	}
	if migrationChecksumMatches(legacyHex, sqlBody+" -- drift") {
		t.Fatalf("expected legacy checksum mismatch after sql drift")
	}
}
