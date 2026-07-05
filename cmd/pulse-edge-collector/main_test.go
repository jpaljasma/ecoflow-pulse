package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	edgev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/edge/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/edgecollector"
	"google.golang.org/grpc"
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

func TestRuntimeDefaultsForPi5CapsCPUAndMemory(t *testing.T) {
	t.Parallel()

	defaults := runtimeDefaultsFor(func(string) string { return "" }, 8)
	if defaults.GOMAXPROCS != defaultPi5GOMAXPROCS {
		t.Fatalf("GOMAXPROCS=%d want %d", defaults.GOMAXPROCS, defaultPi5GOMAXPROCS)
	}
	if defaults.MemoryLimit != defaultPi5Memory {
		t.Fatalf("MemoryLimit=%d want %d", defaults.MemoryLimit, defaultPi5Memory)
	}
	if defaults.GCPercent != defaultPi5GCPercent {
		t.Fatalf("GCPercent=%d want %d", defaults.GCPercent, defaultPi5GCPercent)
	}
}

func TestRuntimeDefaultsRespectExplicitGoEnv(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"GOMAXPROCS": "2",
		"GOMEMLIMIT": "256MiB",
		"GOGC":       "80",
	}
	defaults := runtimeDefaultsFor(func(key string) string { return env[key] }, 4)
	if defaults.GOMAXPROCS != 0 {
		t.Fatalf("GOMAXPROCS default=%d want unset", defaults.GOMAXPROCS)
	}
	if defaults.MemoryLimit != -1 {
		t.Fatalf("MemoryLimit default=%d want unset", defaults.MemoryLimit)
	}
	if defaults.GCPercent != -1 {
		t.Fatalf("GCPercent default=%d want unset", defaults.GCPercent)
	}
}

func TestNewEdgeHTTPClientUsesPiFriendlyTransportBounds(t *testing.T) {
	t.Parallel()

	client := newEdgeHTTPClient()
	if client.Timeout != defaultHTTPTimeout {
		t.Fatalf("timeout=%v want %v", client.Timeout, defaultHTTPTimeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport=%T want *http.Transport", client.Transport)
	}
	if transport.MaxIdleConns != 8 || transport.MaxIdleConnsPerHost != 4 {
		t.Fatalf("idle connection bounds=%d/%d want 8/4", transport.MaxIdleConns, transport.MaxIdleConnsPerHost)
	}
	if !transport.ForceAttemptHTTP2 {
		t.Fatalf("ForceAttemptHTTP2=false want true")
	}
	if transport.TLSHandshakeTimeout != 5*time.Second {
		t.Fatalf("TLSHandshakeTimeout=%v want 5s", transport.TLSHandshakeTimeout)
	}
}

func TestEdgeTransportConfigFromEnvDefaultsToREST(t *testing.T) {
	t.Parallel()

	cfg, err := edgeTransportConfigFromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatalf("edgeTransportConfigFromEnv failed: %v", err)
	}
	if cfg.transport != edgeTransportREST {
		t.Fatalf("transport=%v want REST", cfg.transport)
	}
	if cfg.grpcAddr != defaultEdgeGRPCAddr {
		t.Fatalf("grpcAddr=%q want %q", cfg.grpcAddr, defaultEdgeGRPCAddr)
	}
}

func TestEdgeTransportConfigFromEnvSelectsGRPC(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"PULSE_EDGE_TRANSPORT": " grpc ",
		"PULSE_EDGE_GRPC_ADDR": "127.0.0.1:19090",
	}
	cfg, err := edgeTransportConfigFromEnv(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("edgeTransportConfigFromEnv failed: %v", err)
	}
	if cfg.transport != edgeTransportGRPC {
		t.Fatalf("transport=%v want gRPC", cfg.transport)
	}
	if cfg.grpcAddr != "127.0.0.1:19090" {
		t.Fatalf("grpcAddr=%q", cfg.grpcAddr)
	}
}

func TestEdgeTransportConfigFromEnvRejectsRemoteInsecureGRPC(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"PULSE_EDGE_TRANSPORT": "grpc",
		"PULSE_EDGE_GRPC_ADDR": "pulse.example.test:19090",
	}
	_, err := edgeTransportConfigFromEnv(func(key string) string { return env[key] })
	if err == nil {
		t.Fatalf("expected remote insecure gRPC address to be rejected")
	}
}

func TestEdgeTransportConfigFromEnvRejectsUnknownTransport(t *testing.T) {
	t.Parallel()

	_, err := edgeTransportConfigFromEnv(func(key string) string {
		if key == "PULSE_EDGE_TRANSPORT" {
			return "mqtt"
		}
		return ""
	})
	if err == nil {
		t.Fatalf("expected unknown transport error")
	}
}

func TestPrepareRawOutputPathDoesNotChmodExistingParent(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatalf("create shared parent: %v", err)
	}

	if err := prepareRawOutputPath(filepath.Join(parent, "raw.jsonl")); err != nil {
		t.Fatalf("prepareRawOutputPath failed: %v", err)
	}
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("stat shared parent: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("parent mode=%#o want 0755", got)
	}
}

func TestPrepareRawOutputPathCreatesMissingParentPrivate(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(t.TempDir(), "pulse-edge")
	if err := prepareRawOutputPath(filepath.Join(parent, "raw.jsonl")); err != nil {
		t.Fatalf("prepareRawOutputPath failed: %v", err)
	}
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("stat raw output parent: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("parent mode=%#o want 0700", got)
	}
}

func TestPrepareRawOutputPathRejectsWritableParent(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if err := os.Chmod(parent, 0o777); err != nil {
		t.Fatalf("loosen parent: %v", err)
	}
	if err := prepareRawOutputPath(filepath.Join(parent, "raw.jsonl")); err == nil {
		t.Fatal("expected writable raw output parent to be rejected")
	}
}

func TestWriteEnrollmentEnvFiltersCollectorEnv(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	writeEnrollmentEnv(&out, edgeEnrollment{
		CollectorSecret: "issued-secret",
		CollectorEnv: map[string]string{
			edgecollector.EcoFlowBLEUserIDEnvKey: " ble-user-123 ",
			"PATH":                               "/tmp/injected",
			"BAD":                                "line\nbreak",
		},
	})
	got := out.String()
	if !strings.Contains(got, "PULSE_EDGE_COLLECTOR_SECRET=issued-secret\n") {
		t.Fatalf("missing collector secret in output: %q", got)
	}
	if !strings.Contains(got, edgecollector.EcoFlowBLEUserIDEnvKey+"=ble-user-123\n") {
		t.Fatalf("missing BLE user id in output: %q", got)
	}
	if strings.Contains(got, "PATH=") || strings.Contains(got, "BAD=") {
		t.Fatalf("unexpected env key in output: %q", got)
	}
}

func TestEdgeClientGRPCMethodsMapRequests(t *testing.T) {
	t.Parallel()

	fake := &fakeEdgeIngestClient{
		enrollResp: &edgev1.EnrollCollectorResponse{
			CollectorSecret: "issued-secret",
			CollectorEnv: map[string]string{
				edgecollector.EcoFlowBLEUserIDEnvKey: "ble-user-123",
			},
		},
	}
	client := edgeClient{
		transport:  edgeTransportGRPC,
		secret:     "collector-secret",
		grpcClient: fake,
	}
	record := rawProbeRecord{}
	record.Time = time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	record.Device.Address = "AA:BB:CC:DD:EE:FF"
	record.Device.RSSI = -42
	record.Device.LocalName = "DEMOEDGE0001"
	record.Device.Info.Model = "delta-pro"
	record.Device.Info.Prefix = "dp"
	record.Device.Info.PacketFamily = "display"

	enrollment, err := client.enroll(context.Background(), "setup-token", "v-test", "pulse")
	if err != nil {
		t.Fatalf("enroll failed: %v", err)
	}
	if enrollment.CollectorSecret != "issued-secret" {
		t.Fatalf("secret=%q", enrollment.CollectorSecret)
	}
	if enrollment.CollectorEnv[edgecollector.EcoFlowBLEUserIDEnvKey] != "ble-user-123" {
		t.Fatalf("collector env=%v", enrollment.CollectorEnv)
	}
	if fake.enrollReq.GetSetupToken() != "setup-token" || fake.enrollReq.GetCollectorVersion() != "v-test" || fake.enrollReq.GetHostname() != "pulse" {
		t.Fatalf("unexpected enroll request: %#v", fake.enrollReq)
	}
	assertContextDeadline(t, fake.enrollDeadline, fake.enrollHasDeadline)

	if err := client.heartbeat(context.Background(), "v-test", "pulse"); err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	if fake.heartbeatReq.GetCollectorSecret() != "collector-secret" || fake.heartbeatReq.GetCollectorVersion() != "v-test" || fake.heartbeatReq.GetHostname() != "pulse" {
		t.Fatalf("unexpected heartbeat request: %#v", fake.heartbeatReq)
	}
	assertContextDeadline(t, fake.heartbeatDeadline, fake.heartbeatHasDeadline)

	if err := client.uploadDiscovery(context.Background(), record); err != nil {
		t.Fatalf("uploadDiscovery failed: %v", err)
	}
	if got := fake.discoveryReq.GetCollectorSecret(); got != "collector-secret" {
		t.Fatalf("discovery collector secret=%q", got)
	}
	discovery := fake.discoveryReq.GetDiscoveries()[0]
	if discovery.GetProvider() != "ecoflow" || discovery.GetTransport() != "ble" || discovery.GetProviderDeviceId() != "DEMOEDGE0001" {
		t.Fatalf("unexpected discovery: %#v", discovery)
	}
	if discovery.GetMetadata().GetFields()["packet_family"].GetStringValue() != "display" {
		t.Fatalf("metadata=%v", discovery.GetMetadata())
	}
	assertContextDeadline(t, fake.discoveryDeadline, fake.discoveryHasDeadline)

	metrics := map[string]any{"battery_soc_percent": float64(91), "ac_input_plugged": true}
	if err := client.uploadTelemetry(context.Background(), record, metrics); err != nil {
		t.Fatalf("uploadTelemetry failed: %v", err)
	}
	if got := fake.telemetryReq.GetCollectorSecret(); got != "collector-secret" {
		t.Fatalf("telemetry collector secret=%q", got)
	}
	sample := fake.telemetryReq.GetSamples()[0]
	if sample.GetProvider() != "ecoflow" || sample.GetTransport() != "ble" || sample.GetProviderDeviceId() != "DEMOEDGE0001" {
		t.Fatalf("unexpected telemetry sample: %#v", sample)
	}
	if sample.GetMetrics().GetFields()["battery_soc_percent"].GetNumberValue() != 91 {
		t.Fatalf("metrics=%v", sample.GetMetrics())
	}
	assertContextDeadline(t, fake.telemetryDeadline, fake.telemetryHasDeadline)
}

func TestEdgeClientRESTTelemetryPreservesClientSampleID(t *testing.T) {
	t.Parallel()

	type telemetryRequest struct {
		CollectorSecret string `json:"collectorSecret"`
		Samples         []struct {
			Provider         string         `json:"provider"`
			Transport        string         `json:"transport"`
			ProviderDeviceID string         `json:"providerDeviceId"`
			ObservedAtUnixMS int64          `json:"observedAtUnixMs"`
			ClientSampleID   string         `json:"clientSampleId"`
			Metrics          map[string]any `json:"metrics"`
		} `json:"samples"`
	}
	requests := make(chan telemetryRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/api/v1/edge/telemetry" {
			http.NotFound(w, req)
			return
		}
		var payload telemetryRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := edgeClient{
		transport:  edgeTransportREST,
		baseURL:    server.URL,
		secret:     "collector-secret",
		httpClient: server.Client(),
	}
	telemetry := edgeOutboxTelemetry{
		Provider:         "ecoflow",
		Transport:        "ble",
		ProviderDeviceID: "DEMOEDGE0001",
		ObservedAtUnixMS: 1772197190000,
		ClientSampleID:   "edge-sample-1",
		Metrics:          map[string]any{"output_power_w": float64(118)},
	}

	if err := client.sendTelemetry(context.Background(), telemetry); err != nil {
		t.Fatalf("sendTelemetry failed: %v", err)
	}
	payload := <-requests
	if payload.CollectorSecret != "collector-secret" {
		t.Fatalf("collectorSecret=%q", payload.CollectorSecret)
	}
	if len(payload.Samples) != 1 {
		t.Fatalf("samples len=%d want 1", len(payload.Samples))
	}
	if payload.Samples[0].ClientSampleID != "edge-sample-1" {
		t.Fatalf("clientSampleId=%q want edge-sample-1", payload.Samples[0].ClientSampleID)
	}
}

func TestStableTelemetrySampleIDMatchesLegacyFingerprint(t *testing.T) {
	t.Parallel()

	metrics := map[string]any{"battery_soc_percent": float64(91), "ac_input_plugged": true}
	legacyPayload, err := json.Marshal(map[string]any{
		"provider":            "ecoflow",
		"transport":           "ble",
		"provider_device_id":  "DEMOEDGE0001",
		"observed_at_unix_ms": int64(1772197190000),
		"metrics":             metrics,
	})
	if err != nil {
		t.Fatalf("marshal legacy payload: %v", err)
	}
	sum := sha256.Sum256(legacyPayload)
	want := "edge-telemetry-" + hex.EncodeToString(sum[:])

	if got := stableTelemetrySampleID("DEMOEDGE0001", 1772197190000, metrics); got != want {
		t.Fatalf("stable telemetry sample id=%q want %q", got, want)
	}
}

func TestEdgeClientWaitForInitialHeartbeatRetriesDuringStartup(t *testing.T) {
	t.Parallel()

	fake := &fakeEdgeIngestClient{
		heartbeatErrs: []error{
			errors.New("connection refused"),
			errors.New("edge api not ready"),
		},
	}
	client := edgeClient{
		transport:         edgeTransportGRPC,
		secret:            "collector-secret",
		grpcClient:        fake,
		startupWait:       time.Second,
		startupRetryDelay: time.Millisecond,
	}

	err := client.waitForInitialHeartbeat(
		context.Background(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		"v-test",
		"pulse",
	)
	if err != nil {
		t.Fatalf("waitForInitialHeartbeat failed: %v", err)
	}
	if fake.heartbeatCalls != 3 {
		t.Fatalf("heartbeatCalls=%d want 3", fake.heartbeatCalls)
	}
}

func TestEdgeClientOutboxReplaysTelemetryAfterRestart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	record := rawProbeRecord{}
	record.Time = time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	record.Device.LocalName = "DEMOEDGE0001"
	metrics := map[string]any{"battery_soc_percent": float64(91)}

	firstOutbox, err := newEdgeOutbox(edgeOutboxConfig{
		Dir:      dir,
		MaxAge:   time.Hour,
		MaxBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("newEdgeOutbox failed: %v", err)
	}
	firstClient := edgeClient{
		transport:  edgeTransportGRPC,
		secret:     "collector-secret",
		grpcClient: &fakeEdgeIngestClient{telemetryErrs: []error{errors.New("offline")}},
		outbox:     firstOutbox,
	}
	if err := firstClient.uploadTelemetry(context.Background(), record, metrics); err == nil {
		t.Fatal("uploadTelemetry error=nil want visible send failure after durable enqueue")
	}
	if got := countOutboxFiles(t, dir); got != 1 {
		t.Fatalf("outbox files after failed send=%d want 1", got)
	}
	paths, err := firstOutbox.pendingPaths()
	if err != nil {
		t.Fatalf("pendingPaths failed: %v", err)
	}
	body, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatalf("read outbox file: %v", err)
	}
	if strings.Contains(string(body), "collector-secret") {
		t.Fatalf("outbox file persisted collector secret")
	}

	secondFake := &fakeEdgeIngestClient{}
	secondOutbox, err := newEdgeOutbox(edgeOutboxConfig{
		Dir:      dir,
		MaxAge:   time.Hour,
		MaxBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("newEdgeOutbox restart failed: %v", err)
	}
	secondClient := edgeClient{
		transport:  edgeTransportGRPC,
		secret:     "collector-secret",
		grpcClient: secondFake,
		outbox:     secondOutbox,
	}
	if err := secondClient.flushOutbox(context.Background()); err != nil {
		t.Fatalf("flushOutbox failed: %v", err)
	}
	if got := countOutboxFiles(t, dir); got != 0 {
		t.Fatalf("outbox files after replay=%d want 0", got)
	}
	sample := secondFake.telemetryReq.GetSamples()[0]
	if sample.GetClientSampleId() == "" {
		t.Fatalf("client_sample_id was not replayed")
	}
	if sample.GetClientSampleId() != stableTelemetrySampleID(record.providerDeviceID(), record.observedAtUnixMS(), metrics) {
		t.Fatalf("client_sample_id=%q want stable id", sample.GetClientSampleId())
	}
}

func TestEdgeOutboxEnqueueAndFlushOnlyAttemptsNewEntry(t *testing.T) {
	t.Parallel()

	outbox, err := newEdgeOutbox(edgeOutboxConfig{
		Dir:      t.TempDir(),
		MaxAge:   0,
		MaxBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("newEdgeOutbox failed: %v", err)
	}
	for _, id := range []string{"backlog-1", "backlog-2"} {
		if _, err := outbox.enqueue(edgeOutboxEntry{ID: id, Kind: edgeOutboxKindDiscovery, Discovery: &edgeOutboxDiscovery{ProviderDeviceID: id}}); err != nil {
			t.Fatalf("enqueue backlog %s: %v", id, err)
		}
	}

	var attempted []string
	err = outbox.enqueueAndFlush(
		context.Background(),
		edgeOutboxEntry{ID: "current", Kind: edgeOutboxKindDiscovery, Discovery: &edgeOutboxDiscovery{ProviderDeviceID: "current"}},
		func(_ context.Context, entry edgeOutboxEntry) error {
			attempted = append(attempted, entry.ID)
			return errors.New("offline")
		},
	)
	if err == nil {
		t.Fatal("enqueueAndFlush error=nil want visible send failure")
	}
	if len(attempted) != 1 || attempted[0] != "current" {
		t.Fatalf("attempted sends=%v want only current event", attempted)
	}
	if got := countOutboxFiles(t, outbox.dir); got != 3 {
		t.Fatalf("outbox files after failed current send=%d want 3", got)
	}
}

func TestEdgeOutboxFlushFailsWhenDirectoryUnavailable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outbox, err := newEdgeOutbox(edgeOutboxConfig{
		Dir:      dir,
		MaxAge:   time.Hour,
		MaxBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("newEdgeOutbox failed: %v", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove outbox dir: %v", err)
	}

	err = outbox.flush(context.Background(), func(context.Context, edgeOutboxEntry) error {
		t.Fatal("send should not be called when outbox dir is unavailable")
		return nil
	})
	if err == nil {
		t.Fatal("flush error=nil want unavailable outbox dir error")
	}
}

func TestEdgeOutboxFlushQuarantinesCorruptEntriesAndContinues(t *testing.T) {
	t.Parallel()

	outbox, err := newEdgeOutbox(edgeOutboxConfig{
		Dir:      t.TempDir(),
		MaxAge:   0,
		MaxBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("newEdgeOutbox failed: %v", err)
	}
	outbox.maxAge = 0
	if _, err := outbox.enqueue(edgeOutboxEntry{
		ID:        "z-valid",
		Kind:      edgeOutboxKindDiscovery,
		Discovery: &edgeOutboxDiscovery{ProviderDeviceID: "DEMOEDGE0001"},
	}); err != nil {
		t.Fatalf("enqueue valid outbox entry: %v", err)
	}
	corruptPath := outbox.pathForID("a-corrupt")
	if err := os.WriteFile(corruptPath, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt outbox file: %v", err)
	}

	var sent []string
	err = outbox.flush(context.Background(), func(_ context.Context, entry edgeOutboxEntry) error {
		sent = append(sent, entry.ID)
		return nil
	})
	if !errors.Is(err, errCorruptOutboxEntry) {
		t.Fatalf("flush error=%v want errCorruptOutboxEntry", err)
	}
	if len(sent) != 1 || sent[0] != "z-valid" {
		t.Fatalf("sent entries=%v want valid entry", sent)
	}
	if got := countOutboxFiles(t, outbox.dir); got != 0 {
		t.Fatalf("pending outbox files=%d want 0", got)
	}
	if _, err := os.Stat(corruptPath + ".corrupt"); err != nil {
		t.Fatalf("corrupt outbox entry was not quarantined: %v", err)
	}
}

func TestEdgeOutboxFlushQuarantinesCorruptEntryAfterUsageLoaded(t *testing.T) {
	t.Parallel()

	outbox, err := newEdgeOutbox(edgeOutboxConfig{
		Dir:      t.TempDir(),
		MaxAge:   time.Hour,
		MaxBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("newEdgeOutbox failed: %v", err)
	}
	outbox.maxAge = 0
	if _, err := outbox.enqueue(edgeOutboxEntry{
		ID:        "a-valid",
		Kind:      edgeOutboxKindDiscovery,
		Discovery: &edgeOutboxDiscovery{ProviderDeviceID: "DEMOEDGE0001"},
	}); err != nil {
		t.Fatalf("enqueue valid outbox entry: %v", err)
	}
	if !outbox.usageLoaded || outbox.usedBytes <= 0 {
		t.Fatalf("expected cached usage after enqueue, loaded=%v bytes=%d", outbox.usageLoaded, outbox.usedBytes)
	}
	corruptPath := outbox.pathForID("z-corrupt")
	if err := os.WriteFile(corruptPath, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt outbox file: %v", err)
	}
	corruptInfo, err := os.Stat(corruptPath)
	if err != nil {
		t.Fatalf("stat corrupt outbox file: %v", err)
	}
	outbox.usedBytes += corruptInfo.Size()

	var sent []string
	err = outbox.flush(context.Background(), func(_ context.Context, entry edgeOutboxEntry) error {
		sent = append(sent, entry.ID)
		return nil
	})
	if !errors.Is(err, errCorruptOutboxEntry) {
		t.Fatalf("flush error=%v want errCorruptOutboxEntry", err)
	}
	if len(sent) != 1 || sent[0] != "a-valid" {
		t.Fatalf("sent entries=%v want valid entry", sent)
	}
	if got := outbox.usedBytes; got != 0 {
		t.Fatalf("usedBytes=%d want 0 after sent and quarantined entries", got)
	}
	if _, err := os.Stat(corruptPath + ".corrupt"); err != nil {
		t.Fatalf("corrupt outbox entry was not quarantined: %v", err)
	}
}

func TestEdgeOutboxCapacityAccountsForReplacement(t *testing.T) {
	t.Parallel()

	outbox, err := newEdgeOutbox(edgeOutboxConfig{
		Dir:      t.TempDir(),
		MaxAge:   time.Hour,
		MaxBytes: defaultOutboxMaxBytes,
	})
	if err != nil {
		t.Fatalf("newEdgeOutbox failed: %v", err)
	}
	entry := edgeOutboxEntry{
		ID:   "same-entry",
		Kind: edgeOutboxKindDiscovery,
		Discovery: &edgeOutboxDiscovery{
			ProviderDeviceID: "DEMOEDGE0001",
		},
	}
	queued, err := outbox.enqueue(entry)
	if err != nil {
		t.Fatalf("enqueue first entry: %v", err)
	}
	info, err := os.Stat(outbox.pathForID(queued.ID))
	if err != nil {
		t.Fatalf("stat first entry: %v", err)
	}
	outbox.maxBytes = info.Size()
	if _, err := outbox.enqueue(entry); err != nil {
		t.Fatalf("replacement enqueue should fit existing capacity: %v", err)
	}
	if got := outbox.usedBytes; got != info.Size() {
		t.Fatalf("usedBytes=%d want %d", got, info.Size())
	}
}

func TestEdgeOutboxConcurrentEnqueueAndFlush(t *testing.T) {
	t.Parallel()

	outbox, err := newEdgeOutbox(edgeOutboxConfig{
		Dir:      t.TempDir(),
		MaxAge:   time.Hour,
		MaxBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("newEdgeOutbox failed: %v", err)
	}
	ctx := context.Background()
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	send := func(context.Context, edgeOutboxEntry) error {
		return nil
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			id := fmt.Sprintf("discovery-%d", i)
			if err := outbox.enqueueAndFlush(ctx, edgeOutboxEntry{
				ID:        id,
				Kind:      edgeOutboxKindDiscovery,
				Discovery: &edgeOutboxDiscovery{ProviderDeviceID: id},
			}, send); err != nil {
				errCh <- err
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := outbox.flush(ctx, send); err != nil {
				errCh <- err
				return
			}
		}
	}()

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent outbox operation failed: %v", err)
		}
	}
	if got := countOutboxFiles(t, outbox.dir); got != 0 {
		t.Fatalf("outbox files after concurrent flush=%d want 0", got)
	}
}

func TestEdgeOutboxSendDoesNotBlockRewriteOrRemoveNewerEntry(t *testing.T) {
	t.Parallel()

	outbox, err := newEdgeOutbox(edgeOutboxConfig{
		Dir:      t.TempDir(),
		MaxAge:   time.Hour,
		MaxBytes: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("newEdgeOutbox failed: %v", err)
	}
	sendStarted := make(chan struct{})
	releaseSend := make(chan struct{})
	sendDone := make(chan error, 1)
	go func() {
		sendDone <- outbox.enqueueAndFlush(
			context.Background(),
			edgeOutboxEntry{
				ID:   "same-entry",
				Kind: edgeOutboxKindDiscovery,
				Discovery: &edgeOutboxDiscovery{
					ProviderDeviceID: "old-device",
				},
			},
			func(context.Context, edgeOutboxEntry) error {
				close(sendStarted)
				<-releaseSend
				return nil
			},
		)
	}()
	<-sendStarted

	rewriteDone := make(chan error, 1)
	go func() {
		_, err := outbox.enqueue(edgeOutboxEntry{
			ID:   "same-entry",
			Kind: edgeOutboxKindDiscovery,
			Discovery: &edgeOutboxDiscovery{
				ProviderDeviceID: "new-device",
			},
		})
		rewriteDone <- err
	}()
	select {
	case err := <-rewriteDone:
		if err != nil {
			t.Fatalf("rewrite while send is in flight: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("rewrite blocked behind slow send")
	}

	close(releaseSend)
	if err := <-sendDone; err != nil {
		t.Fatalf("send old entry: %v", err)
	}
	if got := countOutboxFiles(t, outbox.dir); got != 1 {
		t.Fatalf("outbox files after stale send=%d want 1", got)
	}
	queued, err := readOutboxEntry(outbox.pathForID("same-entry"))
	if err != nil {
		t.Fatalf("read rewritten outbox entry: %v", err)
	}
	if got := queued.Discovery.ProviderDeviceID; got != "new-device" {
		t.Fatalf("providerDeviceID=%q want new-device", got)
	}
}

type fakeEdgeIngestClient struct {
	edgev1.EdgeIngestServiceClient

	enrollReq      *edgev1.EnrollCollectorRequest
	enrollResp     *edgev1.EnrollCollectorResponse
	heartbeatReq   *edgev1.HeartbeatRequest
	heartbeatErrs  []error
	heartbeatCalls int
	discoveryReq   *edgev1.UploadDiscoveryRequest
	telemetryReq   *edgev1.UploadTelemetryBatchRequest
	telemetryErrs  []error

	enrollDeadline       time.Time
	enrollHasDeadline    bool
	heartbeatDeadline    time.Time
	heartbeatHasDeadline bool
	discoveryDeadline    time.Time
	discoveryHasDeadline bool
	telemetryDeadline    time.Time
	telemetryHasDeadline bool
}

func (f *fakeEdgeIngestClient) EnrollCollector(ctx context.Context, req *edgev1.EnrollCollectorRequest, _ ...grpc.CallOption) (*edgev1.EnrollCollectorResponse, error) {
	f.enrollReq = req
	f.enrollDeadline, f.enrollHasDeadline = ctx.Deadline()
	return f.enrollResp, nil
}

func (f *fakeEdgeIngestClient) Heartbeat(ctx context.Context, req *edgev1.HeartbeatRequest, _ ...grpc.CallOption) (*edgev1.HeartbeatResponse, error) {
	f.heartbeatReq = req
	f.heartbeatDeadline, f.heartbeatHasDeadline = ctx.Deadline()
	f.heartbeatCalls++
	if len(f.heartbeatErrs) > 0 {
		err := f.heartbeatErrs[0]
		f.heartbeatErrs = f.heartbeatErrs[1:]
		return nil, err
	}
	return &edgev1.HeartbeatResponse{}, nil
}

func (f *fakeEdgeIngestClient) UploadDiscovery(ctx context.Context, req *edgev1.UploadDiscoveryRequest, _ ...grpc.CallOption) (*edgev1.UploadDiscoveryResponse, error) {
	f.discoveryReq = req
	f.discoveryDeadline, f.discoveryHasDeadline = ctx.Deadline()
	return &edgev1.UploadDiscoveryResponse{AcceptedCount: uint32(len(req.GetDiscoveries()))}, nil
}

func (f *fakeEdgeIngestClient) UploadTelemetryBatch(ctx context.Context, req *edgev1.UploadTelemetryBatchRequest, _ ...grpc.CallOption) (*edgev1.UploadTelemetryBatchResponse, error) {
	f.telemetryReq = req
	f.telemetryDeadline, f.telemetryHasDeadline = ctx.Deadline()
	if len(f.telemetryErrs) > 0 {
		err := f.telemetryErrs[0]
		f.telemetryErrs = f.telemetryErrs[1:]
		return nil, err
	}
	return &edgev1.UploadTelemetryBatchResponse{AcceptedCount: uint32(len(req.GetSamples()))}, nil
}

func assertContextDeadline(t *testing.T, deadline time.Time, ok bool) {
	t.Helper()
	if !ok {
		t.Fatal("context has no deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > defaultHTTPTimeout {
		t.Fatalf("deadline remaining=%v want within %v", remaining, defaultHTTPTimeout)
	}
}

func countOutboxFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read outbox dir: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}
	return count
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
		{Name: "manufacturer_serial", Value: "PR1WDEMO00000000"},
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
	if _, ok := metrics["manufacturer_serial"]; ok {
		t.Fatalf("manufacturer_serial should not be forwarded as telemetry")
	}
	if got := record.providerDeviceID(); got != "PR1WDEMO00000000" {
		t.Fatalf("providerDeviceID=%q", got)
	}
}

func TestRawProbeRecordProviderDeviceIDFallsBackToLocalName(t *testing.T) {
	t.Parallel()

	record := rawProbeRecord{}
	record.Device.LocalName = "DEMOEDGE0001"

	if got := record.providerDeviceID(); got != "DEMOEDGE0001" {
		t.Fatalf("providerDeviceID=%q", got)
	}
}

func TestRawProbeRecordManufacturerSerialSeparatesDiscoveryAndTelemetryIdentity(t *testing.T) {
	t.Parallel()

	first := rawProbeRecord{}
	first.Time = time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	first.Device.LocalName = "EF-BLE"
	first.Device.Info.Prefix = "EF"
	first.Event.Metrics = []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Unit  string `json:"unit"`
	}{
		{Name: "manufacturer_serial", Value: "PR1WDEMO00000001"},
	}

	second := first
	second.Event.Metrics = []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Unit  string `json:"unit"`
	}{
		{Name: "manufacturer_serial", Value: "PR1WDEMO00000002"},
	}

	firstDiscovery := discoveryOutboxEntry(first)
	secondDiscovery := discoveryOutboxEntry(second)
	if firstDiscovery.Discovery.ProviderDeviceID != "PR1WDEMO00000001" {
		t.Fatalf("first provider device id=%q", firstDiscovery.Discovery.ProviderDeviceID)
	}
	if secondDiscovery.Discovery.ProviderDeviceID != "PR1WDEMO00000002" {
		t.Fatalf("second provider device id=%q", secondDiscovery.Discovery.ProviderDeviceID)
	}
	if firstDiscovery.ID == secondDiscovery.ID {
		t.Fatalf("discovery IDs should differ for distinct manufacturer serials")
	}

	metrics := map[string]any{"battery_soc_percent": float64(91)}
	firstTelemetry := telemetryOutboxEntry(first, metrics)
	secondTelemetry := telemetryOutboxEntry(second, metrics)
	if firstTelemetry.Telemetry.ProviderDeviceID != "PR1WDEMO00000001" {
		t.Fatalf("first telemetry provider device id=%q", firstTelemetry.Telemetry.ProviderDeviceID)
	}
	if secondTelemetry.Telemetry.ProviderDeviceID != "PR1WDEMO00000002" {
		t.Fatalf("second telemetry provider device id=%q", secondTelemetry.Telemetry.ProviderDeviceID)
	}
	if firstTelemetry.ID == secondTelemetry.ID {
		t.Fatalf("telemetry sample IDs should differ for distinct manufacturer serials")
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

func TestRawProbeRecordMetricMapDropsIdentityOnlyMetrics(t *testing.T) {
	t.Parallel()

	record := rawProbeRecord{}
	record.Event.Metrics = []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Unit  string `json:"unit"`
	}{
		{Name: "manufacturer_serial", Value: "PR1WDEMO00000000"},
		{Name: "auth_result", Value: "ok"},
	}

	if metrics := record.metricMap(); len(metrics) != 0 {
		t.Fatalf("metrics=%v want empty", metrics)
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
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat raw file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("raw file mode=%#o want 0600", got)
	}
}

func TestResetRawProbeOutputRejectsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target.jsonl")
	if err := os.WriteFile(target, []byte("keep\n"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "raw.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := resetRawProbeOutput(link); err == nil {
		t.Fatal("expected symlink raw output to be rejected")
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(body) != "keep\n" {
		t.Fatalf("target was modified: %q", body)
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

func TestReadNewRawProbeEventsRetriesIncompleteFinalLine(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "raw.jsonl")
	complete := `{"type":"probe_event","time":"2026-05-28T12:00:00Z","device":{"local_name":"DEMOEDGE0001"},"event":{"metrics":[{"name":"battery_soc_percent","value":"99"}]}}` + "\n"
	partial := `{"type":"probe_event","time":"2026-05-28T12:01:00Z","device":{"local_name":"DEMOEDGE0001"}`
	if err := os.WriteFile(path, []byte(complete+partial), 0o644); err != nil {
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
		t.Fatalf("seen=%d want only complete line", seen)
	}
	if offset != int64(len(complete)) {
		t.Fatalf("offset=%d want %d", offset, len(complete))
	}

	finished := partial + `,"event":{"metrics":[{"name":"output_power_w","value":"118"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(complete+finished), 0o644); err != nil {
		t.Fatalf("finish raw file: %v", err)
	}
	if err := readNewRawProbeEvents(path, &offset, func(record rawProbeRecord) error {
		seen++
		return nil
	}); err != nil {
		t.Fatalf("readNewRawProbeEvents after complete failed: %v", err)
	}
	if seen != 2 {
		t.Fatalf("seen=%d want completed retry line", seen)
	}
}

func TestReadNewRawProbeEventsRejectsOversizedIncompleteFinalLine(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "raw.jsonl")
	if err := os.WriteFile(path, []byte(strings.Repeat("A", rawScannerMaxBytes+1)), 0o644); err != nil {
		t.Fatalf("write raw file: %v", err)
	}

	var offset int64
	err := readNewRawProbeEvents(path, &offset, func(rawProbeRecord) error {
		t.Fatal("handle should not be called for oversized incomplete line")
		return nil
	})
	if err == nil {
		t.Fatal("readNewRawProbeEvents error=nil want oversized line error")
	}
	if !strings.Contains(err.Error(), "line exceeds") {
		t.Fatalf("error=%v want line exceeds", err)
	}
	if offset != 0 {
		t.Fatalf("offset=%d want unchanged offset 0", offset)
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

func BenchmarkRawProbeRecordMetricMapIdentityOnly(b *testing.B) {
	record := rawProbeRecord{}
	for _, metric := range []struct {
		name  string
		value string
	}{
		{"manufacturer_serial", "PR1WDEMO00000000"},
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
		if metrics := record.metricMap(); len(metrics) != 0 {
			b.Fatal("unexpected telemetry metrics")
		}
	}
}
