package main

import (
	"strings"
	"testing"
)

func TestParseConfigRequiresDSN(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "")

	_, err := parseConfig([]string{"-email", "jpaljasma@gmail.com", "-user-subject", "cloud-subject"})
	if err == nil || !strings.Contains(err.Error(), "CONTROL_PLANE_DB_DSN") {
		t.Fatalf("expected missing dsn error, got %v", err)
	}
}

func TestParseConfigRequiresEmailAndSubject(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "postgres://pulse")

	_, err := parseConfig([]string{"-user-subject", "cloud-subject"})
	if err == nil || !strings.Contains(err.Error(), "-email") {
		t.Fatalf("expected missing email error, got %v", err)
	}

	_, err = parseConfig([]string{"-email", "jpaljasma@gmail.com"})
	if err == nil || !strings.Contains(err.Error(), "-user-subject") {
		t.Fatalf("expected missing user subject error, got %v", err)
	}
}

func TestParseConfigUsesEnvBackedDSN(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "postgres://pulse")

	cfg, err := parseConfig([]string{"-email", "jpaljasma@gmail.com", "-user-subject", "cloud-subject"})
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.dsn != "postgres://pulse" {
		t.Fatalf("dsn mismatch: got=%q", cfg.dsn)
	}
	if cfg.email != "jpaljasma@gmail.com" {
		t.Fatalf("email mismatch: got=%q", cfg.email)
	}
	if cfg.userSubject != "cloud-subject" {
		t.Fatalf("user subject mismatch: got=%q", cfg.userSubject)
	}
}
