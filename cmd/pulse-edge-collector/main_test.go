package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

func TestRawProbeRecordMetricMapDropsNonFiniteNumbers(t *testing.T) {
	t.Parallel()

	record := rawProbeRecord{}
	record.Event.Metrics = []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Unit  string `json:"unit"`
	}{
		{Name: "output_power_w", Value: "+Inf"},
		{Name: "input_power_w", Value: "NaN"},
		{Name: "battery_soc_percent", Value: "99"},
	}

	metrics := record.metricMap()
	if _, ok := metrics["output_power_w"]; ok {
		t.Fatalf("non-finite output_power_w should be dropped")
	}
	if _, ok := metrics["input_power_w"]; ok {
		t.Fatalf("non-finite input_power_w should be dropped")
	}
	if got := metrics["battery_soc_percent"]; got != float64(99) {
		t.Fatalf("battery_soc_percent=%v want 99", got)
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

func TestReadNewRawProbeEventsHandlesLargeJSONLines(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "raw.jsonl")
	line := `{"type":"probe_event","time":"2026-05-28T12:00:00Z","device":{"local_name":"` +
		strings.Repeat("A", 96*1024) +
		`"},"event":{"metrics":[{"name":"battery_soc_percent","value":"100"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatalf("write raw file: %v", err)
	}
	var offset int64
	var seen int
	if err := readNewRawProbeEvents(path, &offset, func(record rawProbeRecord) error {
		seen++
		return nil
	}); err != nil {
		t.Fatalf("readNewRawProbeEvents failed: %v", err)
	}
	if seen != 1 {
		t.Fatalf("seen=%d want 1", seen)
	}
	if offset <= 0 {
		t.Fatalf("offset=%d want positive", offset)
	}
}

func TestRunBLEProbeLoopRestartsAfterProbeExit(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	starts := 0
	err := runBLEProbeLoop(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error {
		starts++
		if starts == 2 {
			cancel()
			return context.Canceled
		}
		return errors.New("probe exited")
	}, fixedBackoff(time.Millisecond))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runBLEProbeLoop error=%v want context canceled", err)
	}
	if starts != 2 {
		t.Fatalf("starts=%d want 2", starts)
	}
}

func TestRunBLEProbeLoopStopsOnAuthFailure(t *testing.T) {
	t.Parallel()

	err := runBLEProbeLoop(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error {
		return fmtBLEAuthError("bad_key")
	}, fixedBackoff(time.Millisecond))
	if !isBLEAuthError(err) {
		t.Fatalf("runBLEProbeLoop error=%v want BLE auth error", err)
	}
}

func TestExitCodeForCollectorErrorStopsRestartOnBLEAuthFailure(t *testing.T) {
	t.Parallel()

	if got := exitCodeForCollectorError(fmtBLEAuthError("bad_key")); got != bleAuthExitCode {
		t.Fatalf("exit code=%d want %d", got, bleAuthExitCode)
	}
	if got := exitCodeForCollectorError(errors.New("temporary failure")); got != 1 {
		t.Fatalf("exit code=%d want 1", got)
	}
}

func TestNextBackoffCapsAndIncreases(t *testing.T) {
	t.Parallel()

	first := nextBackoff(0, time.Second, 10*time.Second)
	second := nextBackoff(first, time.Second, 10*time.Second)
	capped := nextBackoff(8*time.Second, time.Second, 10*time.Second)
	if first != time.Second {
		t.Fatalf("first=%v want 1s", first)
	}
	if second != 2*time.Second {
		t.Fatalf("second=%v want 2s", second)
	}
	if capped != 10*time.Second {
		t.Fatalf("capped=%v want 10s", capped)
	}
}

func TestEdgeClientPostJSONRetriesTransientHTTPStatus(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) < int32(edgePostAttempts) {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := edgeClient{baseURL: server.URL, httpClient: server.Client()}
	if err := client.postJSON(context.Background(), "/edge", map[string]any{"ok": true}, nil); err != nil {
		t.Fatalf("postJSON failed: %v", err)
	}
	if got := hits.Load(); got != int32(edgePostAttempts) {
		t.Fatalf("hits=%d want %d", got, edgePostAttempts)
	}
}

func TestEdgeClientPostJSONDoesNotRetryPermanentHTTPStatus(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client := edgeClient{baseURL: server.URL, httpClient: server.Client()}
	err := client.postJSON(context.Background(), "/edge", map[string]any{"ok": true}, nil)
	if err == nil {
		t.Fatalf("postJSON succeeded, want permanent status error")
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("hits=%d want 1", got)
	}
}

func BenchmarkRawProbeRecordMetricMap(b *testing.B) {
	record := rawProbeRecord{}
	for _, metric := range []struct {
		name  string
		value string
	}{
		{"battery_soc_percent", "100"},
		{"input_power_w", "149"},
		{"output_power_w", "147"},
		{"pv_input_power_w", "42"},
		{"pv2_input_power_w", "17"},
		{"ac_input_plugged", "true"},
		{"auth_result", "ok"},
	} {
		record.Event.Metrics = append(record.Event.Metrics, struct {
			Name  string `json:"name"`
			Value string `json:"value"`
			Unit  string `json:"unit"`
		}{Name: metric.name, Value: metric.value})
	}

	b.ReportAllocs()
	for b.Loop() {
		metrics := record.metricMap()
		if math.IsNaN(metrics["battery_soc_percent"].(float64)) {
			b.Fatal("unexpected NaN")
		}
	}
}
