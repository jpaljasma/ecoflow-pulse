package ecoflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvFile_LoadsValuesWithoutOverwriting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `
# Comment
export ECOFLOW_ENV=staging
ECOFLOW_STAGING_ACCESS_KEY="staging-ak"
ECOFLOW_STAGING_SECRET_KEY='staging-sk'
ECOFLOW_DEBUG=false
EXISTING_KEY=from-file # trailing comment
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("EXISTING_KEY", "already-set")
	if err := loadDotEnvFile(path, false); err != nil {
		t.Fatalf("loadDotEnvFile() error = %v", err)
	}

	if got := os.Getenv("ECOFLOW_ENV"); got != "staging" {
		t.Fatalf("ECOFLOW_ENV mismatch: got %q", got)
	}
	if got := os.Getenv("ECOFLOW_STAGING_ACCESS_KEY"); got != "staging-ak" {
		t.Fatalf("ECOFLOW_STAGING_ACCESS_KEY mismatch: got %q", got)
	}
	if got := os.Getenv("EXISTING_KEY"); got != "already-set" {
		t.Fatalf("EXISTING_KEY should not be overridden: got %q", got)
	}
}

func TestConfigFromEnvironment_UsesDotEnvPath(t *testing.T) {
	t.Setenv("ECOFLOW_ENV", "")
	t.Setenv("ECOFLOW_DEV_ACCESS_KEY", "")
	t.Setenv("ECOFLOW_DEV_SECRET_KEY", "")
	t.Setenv("ECOFLOW_STAGING_ACCESS_KEY", "")
	t.Setenv("ECOFLOW_STAGING_SECRET_KEY", "")
	t.Setenv("ECOFLOW_ACCESS_KEY", "")
	t.Setenv("ECOFLOW_SECRET_KEY", "")
	t.Setenv("ECOFLOW_DEBUG", "")

	dir := t.TempDir()
	path := filepath.Join(dir, ".env.custom")
	content := `
ECOFLOW_ENV=staging
ECOFLOW_STAGING_ACCESS_KEY=dotenv-ak
ECOFLOW_STAGING_SECRET_KEY=dotenv-sk
ECOFLOW_DEBUG=false
ECOFLOW_ADVANCED_DEBUG_TELEMETRY=true
ECOFLOW_DEBUG_LOG_HEADERS=true
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("ECOFLOW_DOTENV_PATH", path)

	cfg, err := ConfigFromEnvironment()
	if err != nil {
		t.Fatalf("ConfigFromEnvironment() error = %v", err)
	}
	if cfg.Environment != EnvironmentStaging {
		t.Fatalf("environment mismatch: got %q", cfg.Environment)
	}
	if cfg.Logging.Debug {
		t.Fatal("expected debug false from dotenv file")
	}
	if !cfg.Logging.AdvancedDebugTelemetry {
		t.Fatal("expected advanced debug telemetry true from dotenv file")
	}
	if !cfg.Logging.DebugLogHeaders {
		t.Fatal("expected debug header logging true from dotenv file")
	}
}
