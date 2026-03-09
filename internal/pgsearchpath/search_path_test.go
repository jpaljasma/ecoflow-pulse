package pgsearchpath

import (
	"strings"
	"testing"
)

func TestApplyKeywordDSNAddsSearchPath(t *testing.T) {
	t.Parallel()

	dsn, err := Apply("host=localhost port=5432 dbname=pulse user=pulse password=secret sslmode=disable", "pulse_v2")
	if err != nil {
		t.Fatalf("apply search path: %v", err)
	}
	if !strings.Contains(dsn, "search_path=pulse_v2") {
		t.Fatalf("expected search_path in dsn, got %q", dsn)
	}
}

func TestApplyURLDSNAddsSearchPath(t *testing.T) {
	t.Parallel()

	dsn, err := Apply("postgres://pulse:secret@localhost:5432/pulse?sslmode=disable", "pulse_v3")
	if err != nil {
		t.Fatalf("apply search path: %v", err)
	}
	if !strings.Contains(dsn, "search_path=pulse_v3") {
		t.Fatalf("expected search_path in dsn, got %q", dsn)
	}
}

func TestResolveEnvPrefersOverride(t *testing.T) {
	t.Setenv(GlobalEnvKey, "global_schema")
	t.Setenv("CONTROL_PLANE_DB_SCHEMA_SEARCH_PATH", "controlplane_schema")

	got := ResolveEnv("CONTROL_PLANE_DB_SCHEMA_SEARCH_PATH")
	if got != "controlplane_schema" {
		t.Fatalf("expected override schema, got %q", got)
	}
}
