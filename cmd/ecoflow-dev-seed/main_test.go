package main

import (
	"strings"
	"testing"
)

func TestParseSeedSerialsDefault(t *testing.T) {
	t.Parallel()

	serials, err := parseSeedSerials("")
	if err != nil {
		t.Fatalf("parseSeedSerials returned error: %v", err)
	}
	if got, want := len(serials), 2; got != want {
		t.Fatalf("serial count mismatch: got=%d want=%d", got, want)
	}
	if serials[0] != "R351ZABAPH331057" || serials[1] != "Y711ZABA9H2P0294" {
		t.Fatalf("unexpected defaults: %#v", serials)
	}
}

func TestParseSeedSerialsNormalizeAndDedupe(t *testing.T) {
	t.Parallel()

	serials, err := parseSeedSerials(" r351zabaph331057, Y711zaba9h2p0294 ; r351zabaph331057 ")
	if err != nil {
		t.Fatalf("parseSeedSerials returned error: %v", err)
	}
	if got, want := len(serials), 2; got != want {
		t.Fatalf("serial count mismatch: got=%d want=%d", got, want)
	}
	if serials[0] != "R351ZABAPH331057" || serials[1] != "Y711ZABA9H2P0294" {
		t.Fatalf("unexpected normalized serials: %#v", serials)
	}
}

func TestParseSeedSerialsRejectsEmptyInputAfterParsing(t *testing.T) {
	t.Parallel()

	_, err := parseSeedSerials(" \n\t ; , ")
	if err == nil {
		t.Fatalf("expected parse error for empty serial set")
	}
}

func TestConfigFromEnvRequiresDSN(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "")
	t.Setenv("ECOFLOW_DEV_ACCESS_KEY", "ak")
	t.Setenv("ECOFLOW_DEV_SECRET_KEY", "sk")

	_, err := configFromEnv()
	if err == nil || !strings.Contains(err.Error(), "CONTROL_PLANE_DB_DSN") {
		t.Fatalf("expected missing DSN error, got: %v", err)
	}
}

func TestConfigFromEnvRequiresKeys(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "host=localhost port=5432 dbname=pulse user=pulse password=pulse sslmode=disable")
	t.Setenv("ECOFLOW_DEV_ACCESS_KEY", "")
	t.Setenv("ECOFLOW_DEV_SECRET_KEY", "")

	_, err := configFromEnv()
	if err == nil || !strings.Contains(err.Error(), "ECOFLOW_DEV_ACCESS_KEY") {
		t.Fatalf("expected missing access key error, got: %v", err)
	}
}

func TestConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("CONTROL_PLANE_DB_DSN", "host=localhost port=5432 dbname=pulse user=pulse password=pulse sslmode=disable")
	t.Setenv("ECOFLOW_DEV_ACCESS_KEY", "ak-1")
	t.Setenv("ECOFLOW_DEV_SECRET_KEY", "sk-1")
	t.Setenv("ECOFLOW_DEV_PROVIDER", "")
	t.Setenv("ECOFLOW_DEV_USER_SUBJECT", "")
	t.Setenv("ECOFLOW_DEV_USER_EMAIL", "")
	t.Setenv("ECOFLOW_DEV_SEED_SNS", "")

	cfg, err := configFromEnv()
	if err != nil {
		t.Fatalf("configFromEnv returned error: %v", err)
	}

	if cfg.Provider != "ecoflow" {
		t.Fatalf("provider mismatch: got=%q want=%q", cfg.Provider, "ecoflow")
	}
	if cfg.UserSubject != defaultUserSubject {
		t.Fatalf("default user subject mismatch: got=%q want=%q", cfg.UserSubject, defaultUserSubject)
	}
	if cfg.UserEmail != defaultUserSubject {
		t.Fatalf("default user email mismatch: got=%q want=%q", cfg.UserEmail, defaultUserSubject)
	}
	if got, want := len(cfg.Devices), 2; got != want {
		t.Fatalf("default device count mismatch: got=%d want=%d", got, want)
	}
}

func TestBuildSeedDevicesKnownModel(t *testing.T) {
	t.Parallel()

	devices := buildSeedDevices([]string{"R351ZABAPH331057", "Y711ZABA9H2P0294"})
	if got, want := len(devices), 2; got != want {
		t.Fatalf("device count mismatch: got=%d want=%d", got, want)
	}
	if devices[0].Model != "DELTA 2 Max" {
		t.Fatalf("unexpected first model: %q", devices[0].Model)
	}
	if devices[1].Model != "DELTA Pro Ultra" {
		t.Fatalf("unexpected second model: %q", devices[1].Model)
	}
}
