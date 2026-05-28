package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigSelectsProfileFromEnv(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
profile: local
targets:
  local:
    base_url: http://localhost:8081
  hosted:
    base_url: https://pulse.example.test
ble:
  raw_output_path: /tmp/raw.jsonl
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := loadConfig(path, func(key string) string {
		if key == "PULSE_EDGE_PROFILE" {
			return "hosted"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if cfg.targetBaseURL() != "https://pulse.example.test" {
		t.Fatalf("base URL=%q", cfg.targetBaseURL())
	}
}

func TestRawProbeRecordMetricMapConvertsNumbersAndBooleans(t *testing.T) {
	t.Parallel()

	record := rawProbeRecord{}
	record.Time = time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	record.Device.LocalName = "DEMOEDGE0001"
	record.Event.Metrics = []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Unit  string `json:"unit"`
	}{
		{Name: "output_power_w", Value: "118"},
		{Name: "ac_input_plugged", Value: "true"},
		{Name: "auth_result", Value: "ok"},
	}

	metrics := record.metricMap()
	if got := metrics["output_power_w"]; got != float64(118) {
		t.Fatalf("output_power_w=%v", got)
	}
	if got := metrics["ac_input_plugged"]; got != true {
		t.Fatalf("ac_input_plugged=%v", got)
	}
	if _, ok := metrics["auth_result"]; ok {
		t.Fatalf("auth_result should not be forwarded")
	}
	if got := record.providerDeviceID(); got != "DEMOEDGE0001" {
		t.Fatalf("providerDeviceID=%q", got)
	}
}

func TestRawProbeRecordAuthError(t *testing.T) {
	t.Parallel()

	record := rawProbeRecord{}
	record.Event.Metrics = []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Unit  string `json:"unit"`
	}{
		{Name: "auth_result", Value: "wrong_key"},
	}
	if err := record.authError(); err == nil {
		t.Fatalf("expected auth error")
	}
}

func TestResetRawProbeOutputTruncatesExistingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "raw.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"probe_event"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write raw file: %v", err)
	}
	if err := resetRawProbeOutput(path); err != nil {
		t.Fatalf("resetRawProbeOutput failed: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw file: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("raw file len=%d want 0", len(body))
	}
}
