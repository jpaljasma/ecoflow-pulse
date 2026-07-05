package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestInferEcoFlowModelFromDelta3AirBLEName(t *testing.T) {
	t.Parallel()

	got := inferEcoFlowDevice("EF-PR12DEMO0000")
	if !got.Matched {
		t.Fatal("expected EcoFlow BLE name to match")
	}
	if got.Prefix != "PR12" {
		t.Fatalf("prefix = %q, want PR12", got.Prefix)
	}
	if got.Model != "EcoFlow DELTA 3 1000 Air (10ms UPS)" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.PacketFamily != "v3" {
		t.Fatalf("packet family = %q, want v3", got.PacketFamily)
	}
}

func TestInferEcoFlowModelFromObservedBLEPrefixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		localName  string
		wantPrefix string
		wantModel  string
	}{
		{
			name:       "delta pro ultra",
			localName:  "EF-YJ0000",
			wantPrefix: "YJ",
			wantModel:  "EcoFlow DELTA Pro Ultra",
		},
		{
			name:       "delta 3 air",
			localName:  "EF-PR1W0000",
			wantPrefix: "PR1W",
			wantModel:  "EcoFlow DELTA 3 1000 Air",
		},
		{
			name:       "river 3 plus",
			localName:  "EF-R3PG0000",
			wantPrefix: "R3PG",
			wantModel:  "EcoFlow RIVER 3 Plus (270Wh)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := inferEcoFlowDevice(tt.localName)
			if !got.Matched {
				t.Fatalf("expected %q to match EcoFlow prefix", tt.localName)
			}
			if got.Prefix != tt.wantPrefix {
				t.Fatalf("prefix = %q, want %q", got.Prefix, tt.wantPrefix)
			}
			if got.Model != tt.wantModel {
				t.Fatalf("model = %q, want %q", got.Model, tt.wantModel)
			}
			if got.PacketFamily != "v3" {
				t.Fatalf("packet family = %q, want v3", got.PacketFamily)
			}
		})
	}
}

func TestFilterDiscoveryHonorsEcoFlowAndRSSI(t *testing.T) {
	t.Parallel()

	cfg := discoveryConfig{MinRSSI: -85}
	if !shouldIncludeAdvertisement(discoveredBLEDevice{LocalName: "EF-PR12DEMO0000", RSSI: -70}, cfg) {
		t.Fatal("expected EcoFlow candidate above RSSI floor to be included")
	}
	if shouldIncludeAdvertisement(discoveredBLEDevice{LocalName: "EF-PR12DEMO0000", RSSI: -91}, cfg) {
		t.Fatal("expected weak EcoFlow candidate below RSSI floor to be filtered")
	}
	if shouldIncludeAdvertisement(discoveredBLEDevice{LocalName: "Keyboard", RSSI: -40}, cfg) {
		t.Fatal("expected non-EcoFlow advertisement to be filtered by default")
	}
	cfg.IncludeAll = true
	if !shouldIncludeAdvertisement(discoveredBLEDevice{LocalName: "Keyboard", RSSI: -40}, cfg) {
		t.Fatal("expected -all to include non-EcoFlow advertisements")
	}
}

func TestFormatDiscoveryTextRedactsIdentifiers(t *testing.T) {
	t.Parallel()

	line := formatDiscoveryText(discoveredBLEDevice{
		Address:   "A1B2C3D4-E5F6-4711-8899-001122334455",
		RSSI:      -54,
		LocalName: "EF-PR12DEMO0000",
		Info: ecoFlowDeviceInfo{
			Matched:      true,
			Prefix:       "PR12",
			Model:        "EcoFlow DELTA 3 1000 Air (10ms UPS)",
			PacketFamily: "v3",
		},
	}, true)

	if strings.Contains(line, "001122334455") || strings.Contains(line, "DEMO0000") {
		t.Fatalf("line leaked raw identifier: %s", line)
	}
	for _, want := range []string{"address=A1B2...4455", `name="EF-PR...0000"`, `model="EcoFlow DELTA 3 1000 Air (10ms UPS)"`, "packets=v3"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}
}

func TestParseDiscoveryConfigDefaultsToFiveSecondSelectionWindow(t *testing.T) {
	t.Parallel()

	cfg, err := parseDiscoveryConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseDiscoveryConfig() error = %v", err)
	}
	if cfg.Duration != 5*time.Second {
		t.Fatalf("duration = %s, want 5s", cfg.Duration)
	}
	if cfg.ScanOnly {
		t.Fatal("expected probing to be enabled by default")
	}
	if cfg.ProbeTimeout != 10*time.Second {
		t.Fatalf("probe timeout = %s, want 10s", cfg.ProbeTimeout)
	}
	if cfg.ListenDuration != 5*time.Second {
		t.Fatalf("listen duration = %s, want 5s", cfg.ListenDuration)
	}
	if cfg.ActiveProbe != activeProbeNone {
		t.Fatalf("active probe = %q, want %q", cfg.ActiveProbe, activeProbeNone)
	}
	if cfg.BLETransport != bleTransportAuto {
		t.Fatalf("BLE transport = %q, want %q", cfg.BLETransport, bleTransportAuto)
	}
	if cfg.RawOutputPath != defaultRawOutputPath {
		t.Fatalf("raw output path = %q, want %q", cfg.RawOutputPath, defaultRawOutputPath)
	}
}

func TestRunDiscoverySelectsDeviceAndPrintsProbe(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	prober := &fakeProber{
		result: deviceProbe{
			Capabilities: probeCapabilities{
				ServiceCount:        1,
				CharacteristicCount: 2,
				MTUs:                []uint16{23},
				Services: []probeService{
					{UUID: "0000ec00-0000-1000-8000-00805f9b34fb", Characteristics: []string{
						"0000ec01-0000-1000-8000-00805f9b34fb",
						"0000ec02-0000-1000-8000-00805f9b34fb",
					}},
				},
			},
			Metrics: []probeMetric{
				{Name: "battery_percent", Value: "82", Unit: "%", Source: "gatt"},
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := runDiscovery(
		ctx,
		[]string{"-duration=25ms", "-select=1", "-redact=false"},
		strings.NewReader("1\n"),
		&stdout,
		&stderr,
		fakeScanner{devices: []discoveredBLEDevice{
			{Address: "AA:BB:CC:DD:EE:01", RSSI: -65, LocalName: "EF-YJ0000"},
			{Address: "AA:BB:CC:DD:EE:02", RSSI: -52, LocalName: "EF-PR1W0000", ServiceUUIDs: []string{"0000ec00-0000-1000-8000-00805f9b34fb"}, ManufacturerData: map[string]string{"0x0001": "01"}},
		}},
		prober,
	)
	if err != nil {
		t.Fatalf("runDiscovery() error = %v", err)
	}
	if len(prober.devices) != 1 {
		t.Fatalf("probe calls = %d, want 1", len(prober.devices))
	}
	if got := prober.devices[0].Address; got != "AA:BB:CC:DD:EE:02" {
		t.Fatalf("probed address = %q, want second device", got)
	}
	output := stdout.String()
	for _, want := range []string{
		"summary seen=2 ecoflow=2",
		"discovered devices:",
		`1) address=AA:BB:CC:DD:EE:02 name="EF-PR1W0000"`,
		`2) address=AA:BB:CC:DD:EE:01 name="EF-YJ0000"`,
		`probing address=AA:BB:CC:DD:EE:02 name="EF-PR1W0000" model="EcoFlow DELTA 3 1000 Air"`,
		"capabilities services=1 characteristics=2 mtus=23",
		"service uuid=0000ec00-0000-1000-8000-00805f9b34fb characteristics=2",
		"characteristic uuid=0000ec01-0000-1000-8000-00805f9b34fb",
		`metric battery_percent=82 unit="%" source=gatt`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestOrderedDiscoveredDevicesUsesStableDisplayOrder(t *testing.T) {
	t.Parallel()

	seen := map[string]discoveredBLEDevice{
		"river": {Address: "AA:BB:CC:DD:EE:03", RSSI: -70, LocalName: "EF-R3PG0000", Info: inferEcoFlowDevice("EF-R3PG0000")},
		"delta": {Address: "AA:BB:CC:DD:EE:01", RSSI: -52, LocalName: "EF-PR1W0000", Info: inferEcoFlowDevice("EF-PR1W0000")},
		"ultra": {Address: "AA:BB:CC:DD:EE:02", RSSI: -81, LocalName: "EF-YJ0000", Info: inferEcoFlowDevice("EF-YJ0000")},
	}
	got := orderedDiscoveredDevices(seen, []string{"ultra", "river", "delta"})
	var names []string
	for _, device := range got {
		names = append(names, device.LocalName)
	}
	want := []string{"EF-PR1W0000", "EF-R3PG0000", "EF-YJ0000"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", names, want)
	}
}

func TestRunDiscoveryStreamsProbeEventsBeforeProbeReturns(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	prober := &fakeStreamingProber{stdout: &stdout}
	err := runDiscovery(
		context.Background(),
		[]string{"-duration=25ms", "-select=1", "-redact=false"},
		strings.NewReader(""),
		&stdout,
		&stderr,
		fakeScanner{devices: []discoveredBLEDevice{
			{Address: "AA:BB:CC:DD:EE:02", RSSI: -52, LocalName: "EF-PR1W0000"},
		}},
		prober,
	)
	if err != nil {
		t.Fatalf("runDiscovery() error = %v", err)
	}
	for _, want := range []string{
		`probing address=AA:BB:CC:DD:EE:02 name="EF-PR1W0000" model="EcoFlow DELTA 3 1000 Air"`,
		"capabilities services=1 characteristics=1",
		"notification direction=rx service=00000001-0000-1000-8000-00805f9b34fb characteristic=00000003-0000-1000-8000-00805f9b34fb",
		`metric streamed_power_w=123 unit="W" source=ble_notify`,
	} {
		if !strings.Contains(prober.outputDuringProbe, want) {
			t.Fatalf("streamed output before probe returned missing %q:\n%s", want, prober.outputDuringProbe)
		}
	}
	output := stdout.String()
	if count := strings.Count(output, "notification direction=rx"); count != 1 {
		t.Fatalf("notification printed %d times, want once:\n%s", count, output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDiscoveryAutoProbesSupportedDevicesConcurrentlyAndWritesRawEvents(t *testing.T) {
	rawPath := filepath.Join(t.TempDir(), "raw.jsonl")
	if err := os.WriteFile(rawPath, []byte("stale raw data\n"), 0o600); err != nil {
		t.Fatalf("seed raw output file: %v", err)
	}
	if err := os.Chmod(rawPath, 0o644); err != nil {
		t.Fatalf("loosen raw output file: %v", err)
	}
	prober := newConcurrentStreamingProber()
	var stdout lockedBuffer
	var stderr bytes.Buffer

	err := runDiscovery(
		t.Context(),
		[]string{"-duration=25ms", "-redact=false", "-raw-output=" + rawPath},
		strings.NewReader(""),
		&stdout,
		&stderr,
		fakeScanner{devices: []discoveredBLEDevice{
			{Address: "AA:BB:CC:DD:EE:01", RSSI: -61, LocalName: "EF-YJ0000"},
			{Address: "AA:BB:CC:DD:EE:02", RSSI: -52, LocalName: "EF-PR1W0000"},
			{Address: "AA:BB:CC:DD:EE:03", RSSI: -58, LocalName: "EF-R3PG0000"},
			{Address: "AA:BB:CC:DD:EE:04", RSSI: -35, LocalName: "Keyboard"},
		}},
		prober,
	)
	if err != nil {
		t.Fatalf("runDiscovery() error = %v", err)
	}
	if prober.maxActiveProbeCount() < 2 {
		t.Fatalf("max active probes = %d, want at least 2", prober.maxActiveProbeCount())
	}
	if got := strings.Join(prober.probedNames(), ","); got != "EF-PR1W0000,EF-R3PG0000" {
		t.Fatalf("probed devices = %s, want supported Delta/River devices", got)
	}
	if got := strings.Join(prober.activeProbeModes(), ","); got != "auto,auto" {
		t.Fatalf("active probe modes = %s, want auto for default supported-device probes", got)
	}
	output := stdout.String()
	for _, want := range []string{
		"summary seen=3 ecoflow=3",
		"auto probing supported devices=2",
		"raw_output path=" + rawPath,
		`probing address=AA:BB:CC:DD:EE:02 name="EF-PR1W0000" model="EcoFlow DELTA 3 1000 Air"`,
		`probing address=AA:BB:CC:DD:EE:03 name="EF-R3PG0000" model="EcoFlow RIVER 3 Plus (270Wh)"`,
		"| Metric              | EF-PR1W0000                   | EF-R3PG0000",
		"| Device              | EF-PR1W0000                   | EF-R3PG0000",
		"| Current load        | 321 W                         | 321 W",
		"| Solar in            | 55 W                          | 55 W",
		"| Battery charge      | 79%                           | 79%",
		"| ETA                 | discharge: 33h 03m            | discharge: 33h 03m",
		"| Services            | 1                             | 1",
		"| Characteristics     | 2                             | 2",
		"| MTUs                | 497                           | 497",
		"| Total input         | 123 W                         | 123 W",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "EF-YJ0000") && strings.Contains(output, "probing address=AA:BB:CC:DD:EE:01") {
		t.Fatalf("unsupported device was probed:\n%s", output)
	}
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read raw output: %v", err)
	}
	rawText := string(raw)
	for _, want := range []string{`"type":"probe_event"`, "EF-PR1W0000", "EF-R3PG0000", `"service_count":1`} {
		if !strings.Contains(rawText, want) {
			t.Fatalf("raw output missing %q:\n%s", want, rawText)
		}
	}
	if strings.Contains(rawText, "stale raw data") {
		t.Fatalf("raw output was not overwritten:\n%s", rawText)
	}
	info, err := os.Stat(rawPath)
	if err != nil {
		t.Fatalf("stat raw output: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("raw output mode=%#o want 0600", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestProbeRawEventLoggerCreatesPrivateParent(t *testing.T) {
	t.Parallel()

	rawPath := filepath.Join(t.TempDir(), "nested", "raw.jsonl")
	logger, err := newProbeRawEventLogger(rawPath)
	if err != nil {
		t.Fatalf("newProbeRawEventLogger failed: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("close raw logger: %v", err)
	}
	info, err := os.Stat(filepath.Dir(rawPath))
	if err != nil {
		t.Fatalf("stat raw output parent: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("raw output parent mode=%#o want 0700", got)
	}
}

func TestProbeRawEventLoggerRejectsSymlinkOutput(t *testing.T) {
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
	logger, err := newProbeRawEventLogger(link)
	if err == nil {
		_ = logger.Close()
		t.Fatal("expected symlink raw output to be rejected")
	}
	body, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read target: %v", readErr)
	}
	if string(body) != "keep\n" {
		t.Fatalf("target was modified: %q", body)
	}
}

func TestRunDiscoveryAutoProbeRunsUntilParentContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	prober := &cancelAwareProber{started: make(chan struct{}, 1)}
	var stdout lockedBuffer
	var stderr bytes.Buffer
	errCh := make(chan error, 1)

	go func() {
		errCh <- runDiscovery(
			ctx,
			[]string{"-duration=25ms", "-redact=false", "-raw-output="},
			strings.NewReader(""),
			&stdout,
			&stderr,
			fakeScanner{devices: []discoveredBLEDevice{
				{Address: "AA:BB:CC:DD:EE:02", RSSI: -52, LocalName: "EF-PR1W0000"},
			}},
			prober,
		)
	}()

	select {
	case <-prober.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for auto probe to start")
	}
	select {
	case err := <-errCh:
		t.Fatalf("runDiscovery returned before cancellation: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runDiscovery() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for auto probe to stop after cancellation")
	}
	if got := prober.activeProbe; got != activeProbeAuto {
		t.Fatalf("active probe = %q, want %q", got, activeProbeAuto)
	}
	if !prober.listenUntilCanceled {
		t.Fatal("expected auto probe to listen until cancellation")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDiscoveryAutoProbeStopsAllDevicesWhenAuthenticationFails(t *testing.T) {
	prober := &authFailingStreamingProber{failName: "EF-PR1W0000"}
	var stdout lockedBuffer
	var stderr bytes.Buffer

	err := runDiscovery(
		context.Background(),
		[]string{"-duration=25ms", "-redact=false", "-raw-output="},
		strings.NewReader(""),
		&stdout,
		&stderr,
		fakeScanner{devices: []discoveredBLEDevice{
			{Address: "AA:BB:CC:DD:EE:02", RSSI: -52, LocalName: "EF-PR1W0000"},
			{Address: "AA:BB:CC:DD:EE:03", RSSI: -58, LocalName: "EF-R3PG0000"},
		}},
		prober,
	)
	if err == nil {
		t.Fatal("runDiscovery() error = nil, want auth failure")
	}
	if !strings.Contains(err.Error(), "BLE authentication failed: wrong_key") {
		t.Fatalf("runDiscovery() error = %v, want wrong_key auth failure", err)
	}
	if !prober.cancelObserved.Load() {
		t.Fatal("sibling probe did not observe cancellation after auth failure")
	}
	output := stdout.String()
	if !strings.Contains(output, "| Auth") || !strings.Contains(output, "wrong_key") {
		t.Fatalf("output missing final auth result table:\n%s", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestPrintProbeTextShowsEcoFlowCharacteristicRoles(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	probe := deviceProbe{
		Capabilities: probeCapabilities{
			ServiceCount:        2,
			CharacteristicCount: 4,
			Services: []probeService{
				{UUID: ecoFlowRFCOMMServiceUUID, Characteristics: []string{
					ecoFlowRFCOMMWriteCharacteristicUUID,
					ecoFlowRFCOMMNotifyCharacteristicUUID,
				}},
				{UUID: ecoFlowAlternateServiceUUID, Characteristics: []string{
					ecoFlowAlternateWriteCharacteristicUUID,
					ecoFlowAlternateNotifyCharacteristicUUID,
				}},
			},
		},
	}
	err := printProbeText(&stdout, discoveredBLEDevice{
		Address:   "AA:BB:CC:DD:EE:02",
		LocalName: "EF-PR1W0000",
		Info: ecoFlowDeviceInfo{
			Matched: true,
			Model:   "EcoFlow DELTA 3 1000 Air",
		},
	}, probe, false)
	if err != nil {
		t.Fatalf("printProbeText() error = %v", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"characteristic uuid=00000002-0000-1000-8000-00805f9b34fb role=ecoflow_rfcomm_write",
		"characteristic uuid=00000003-0000-1000-8000-00805f9b34fb role=ecoflow_rfcomm_notify",
		"characteristic uuid=00000006-0000-1000-8000-00805f9b34fb role=ecoflow_alt_write",
		"characteristic uuid=00000007-0000-1000-8000-00805f9b34fb role=ecoflow_alt_notify",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestDecodeDelta3DisplayUploadMetrics(t *testing.T) {
	t.Parallel()

	payload := append([]byte{},
		encodeFixed32ProtoFieldForTest(3, math.Float32bits(145.5))...)
	payload = append(payload, encodeFixed32ProtoFieldForTest(4, math.Float32bits(80))...)
	payload = append(payload, encodeFixed32ProtoFieldForTest(54, math.Float32bits(123))...)
	payload = append(payload, encodeFixed32ProtoFieldForTest(368, math.Float32bits(-80))...)
	payload = append(payload, encodeFixed32ProtoFieldForTest(361, math.Float32bits(55))...)
	payload = append(payload, encodeVarintProtoFieldForTest(363, 2)...)
	payload = append(payload, encodeFixed32ProtoFieldForTest(11, math.Float32bits(-12.5))...)
	payload = append(payload, encodeFixed32ProtoFieldForTest(9, math.Float32bits(-5))...)
	payload = append(payload, encodeFixed32ProtoFieldForTest(37, math.Float32bits(-8))...)
	payload = append(payload, encodeFixed32ProtoFieldForTest(158, math.Float32bits(-2))...)
	payload = append(payload, encodeVarintProtoFieldForTest(148, 2)...)
	payload = append(payload, encodeFixed32ProtoFieldForTest(262, math.Float32bits(76))...)

	decoded := decodeEcoFlowNotification(buildDelta3DisplayPacketForTest(payload))
	if decoded.Frame != "ecoflow_packet" {
		t.Fatalf("frame = %q, want ecoflow_packet", decoded.Frame)
	}
	if decoded.Packet == nil {
		t.Fatal("expected packet summary")
	}
	if decoded.Packet.Description != "v3 display property upload" {
		t.Fatalf("description = %q", decoded.Packet.Description)
	}

	got := metricMap(decoded.Metrics)
	want := map[string]string{
		"input_power_w":          "145.5",
		"output_power_w":         "80",
		"ac_input_power_w":       "123",
		"ac_output_power_w":      "80",
		"pv_input_power_w":       "55",
		"pv_input_state":         "solar",
		"usb_c_1_output_power_w": "12.5",
		"usb_a_1_output_power_w": "5",
		"dc_12v_output_power_w":  "8",
		"battery_power_w":        "-2",
		"battery_input_power_w":  "0",
		"battery_output_power_w": "2",
		"dc_charging_type":       "solar",
		"battery_soc_percent":    "76",
	}
	for name, wantValue := range want {
		if got[name] != wantValue {
			t.Fatalf("metric %s = %q, want %q; all metrics: %#v", name, got[name], wantValue, got)
		}
	}
	for _, metric := range decoded.Metrics {
		if metric.Source != "ble_notify" {
			t.Fatalf("metric %s source = %q, want ble_notify", metric.Name, metric.Source)
		}
		if metric.Decoder != "v3_display" {
			t.Fatalf("metric %s decoder = %q, want v3_display", metric.Name, metric.Decoder)
		}
	}
}

func TestPrintProbeMetricsIncludesDecoderSeparatelyFromSource(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := printProbeMetricsText(&stdout, []probeMetric{
		{Name: "input_power_w", Value: "125", Unit: "W", Source: "ble_notify", Decoder: "v3_display"},
	})
	if err != nil {
		t.Fatalf("printProbeMetricsText() error = %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	want := `metric input_power_w=125 unit="W" source=ble_notify decoder=v3_display`
	if got != want {
		t.Fatalf("metric line = %q, want %q", got, want)
	}
}

func TestDecodeEncPacketClassifiesInnerPacket(t *testing.T) {
	t.Parallel()

	payload := encodeFixed32ProtoFieldForTest(3, math.Float32bits(12))
	inner := buildDelta3DisplayPacketForTest(payload)
	decoded := decodeEcoFlowNotification(buildEncPacketForTest(0x01, inner))
	if decoded.Frame != "ecoflow_enc_packet" {
		t.Fatalf("frame = %q, want ecoflow_enc_packet", decoded.Frame)
	}
	if decoded.Packet == nil {
		t.Fatal("expected inner packet summary")
	}
	if decoded.Packet.Command != "src=0x02,dst=0x21,cmd_set=0xfe,cmd_id=0x15" {
		t.Fatalf("command = %q", decoded.Packet.Command)
	}
	if got := metricMap(decoded.Metrics)["input_power_w"]; got != "12" {
		t.Fatalf("input_power_w = %q, want 12", got)
	}
}

func TestDecodeAuthResponseReportsWrongKey(t *testing.T) {
	t.Parallel()

	decoded := decodeEcoFlowNotification(buildEcoFlowV3Packet(0x35, 0x21, 0x35, 0x86, []byte{0x06}))
	if decoded.Packet == nil {
		t.Fatal("expected packet summary")
	}
	if decoded.Packet.Description != "auth response" {
		t.Fatalf("description = %q, want auth response", decoded.Packet.Description)
	}
	got := metricMap(decoded.Metrics)
	if got["auth_result"] != "wrong_key" {
		t.Fatalf("auth_result = %q, want wrong_key; metrics=%#v", got["auth_result"], decoded.Metrics)
	}
}

func TestSummarizeAuthRequestDistinguishesDirection(t *testing.T) {
	t.Parallel()

	data := buildEcoFlowV3Packet(0x21, 0x35, 0x35, 0x86, []byte(strings.Repeat("A", 32)))
	packet, err := parseEcoFlowPacket(data)
	if err != nil {
		t.Fatalf("parse auth request: %v", err)
	}
	got := summarizeEcoFlowPacket(packet)
	if got.Description != "auth request" {
		t.Fatalf("description = %q, want auth request", got.Description)
	}
	if metrics := decodeEcoFlowNotification(data).Metrics; len(metrics) != 0 {
		t.Fatalf("auth request metrics = %#v, want none", metrics)
	}
}

func TestSummarizeEcoFlowEdgeCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		packet          []byte
		wantDescription string
	}{
		{
			name:            "ble ping",
			packet:          buildEcoFlowV3Packet(0x35, 0x20, 0x35, 0x20, []byte{0x01, 0x02, 0x03}),
			wantDescription: "ble ping",
		},
		{
			name:            "device time request",
			packet:          buildEcoFlowV3Packet(0x35, 0x21, 0x01, 0x52, nil),
			wantDescription: "device time request",
		},
		{
			name:            "rtc time check",
			packet:          buildEcoFlowV3Packet(0x21, 0x35, 0x01, 0x53, []byte{0x01}),
			wantDescription: "rtc time check",
		},
		{
			name:            "utc time sync",
			packet:          buildEcoFlowV3Packet(0x21, 0x0b, 0x01, 0x55, []byte{0x01}),
			wantDescription: "utc time sync",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := decodeEcoFlowNotification(tt.packet)
			if got.Packet == nil {
				t.Fatal("expected packet summary")
			}
			if got.Packet.Description != tt.wantDescription {
				t.Fatalf("description = %q, want %q", got.Packet.Description, tt.wantDescription)
			}
		})
	}
}

func TestDecodeSimpleEncPacketShowsDecodedPayload(t *testing.T) {
	t.Parallel()

	frame := buildEncPacketForTest(0x00, append([]byte{0x01, 0x00}, bytes.Repeat([]byte{0x42}, 40)...))
	decoded := decodeEcoFlowNotification(frame)
	if decoded.Frame != "ecoflow_enc_packet" {
		t.Fatalf("frame = %q, want ecoflow_enc_packet", decoded.Frame)
	}
	if !strings.Contains(decoded.Detail, "simple=ecdh_public_key") {
		t.Fatalf("detail = %q, want ECDH simple frame detail", decoded.Detail)
	}
	if decoded.DecodedHex != "0100"+strings.Repeat("42", 40) {
		t.Fatalf("decoded hex = %q", decoded.DecodedHex)
	}
}

func TestDecodeProtocolEncPacketDoesNotClassifyEncryptedPayloadAsSimpleCommand(t *testing.T) {
	t.Parallel()

	frame := buildEncPacketForTest(0x01, append([]byte{0x02}, bytes.Repeat([]byte{0x42}, 15)...))
	decoded := decodeEcoFlowNotification(frame)
	if decoded.Frame != "ecoflow_enc_packet" {
		t.Fatalf("frame = %q, want ecoflow_enc_packet", decoded.Frame)
	}
	if strings.Contains(decoded.Detail, "simple=session_key_info") {
		t.Fatalf("detail = %q, must not classify encrypted protocol payload as a simple command", decoded.Detail)
	}
	if !strings.Contains(decoded.Detail, "encrypted_or_unknown_payload") {
		t.Fatalf("detail = %q, want encrypted_or_unknown_payload", decoded.Detail)
	}
}

func TestDecodeProtocolEncPacketTreatsInvalidInnerPacketLikeEncryptedPayload(t *testing.T) {
	t.Parallel()

	frame := buildEncPacketForTest(0x01, []byte{0xaa, 0x4c, 0x00, 0x00})
	decoded := decodeEcoFlowNotification(frame)
	if decoded.Frame != "ecoflow_enc_packet" {
		t.Fatalf("frame = %q, want ecoflow_enc_packet", decoded.Frame)
	}
	if strings.Contains(decoded.Detail, "inner_packet_error") {
		t.Fatalf("detail = %q, must not expose inner packet parse errors for encrypted protocol payloads", decoded.Detail)
	}
	if !strings.Contains(decoded.Detail, "encrypted_or_unknown_payload") {
		t.Fatalf("detail = %q, want encrypted_or_unknown_payload", decoded.Detail)
	}
	if decoded.Packet != nil {
		t.Fatalf("packet = %#v, want nil for encrypted payload", decoded.Packet)
	}
}

func TestAdvertisementProbeMetricsIncludesEcoFlowScanRecord(t *testing.T) {
	t.Parallel()

	manufacturerData := append([]byte{0x02}, []byte("PR1WDEMO00000000")...)
	manufacturerData = append(manufacturerData, 0x80, 0x12, 0x00, 0x00, 0x00, 0b00111000)
	got := metricMap(advertisementProbeMetrics(discoveredBLEDevice{
		RSSI: -48,
		ManufacturerData: map[string]string{
			"0xb5b5": hex.EncodeToString(manufacturerData),
		},
	}))
	if got["manufacturer_proto_version"] != "2" {
		t.Fatalf("manufacturer proto version = %q", got["manufacturer_proto_version"])
	}
	if got["manufacturer_serial"] != "PR1WDEMO00000000" {
		t.Fatalf("manufacturer serial = %q", got["manufacturer_serial"])
	}
	if got["manufacturer_encrypt_type"] != "7" {
		t.Fatalf("manufacturer encrypt type = %q", got["manufacturer_encrypt_type"])
	}
	if got["manufacturer_active"] != "true" {
		t.Fatalf("manufacturer active = %q", got["manufacturer_active"])
	}
}

func TestParseEcoFlowScanRecordExtractsDeviceSerial(t *testing.T) {
	t.Parallel()

	manufacturerData := append([]byte{0x13}, []byte("PR1WDEMO00000000")...)
	manufacturerData = append(manufacturerData, 0x80, 0x12, 0x00, 0x00, 0x00, 0b00111000)
	record, ok := parseEcoFlowScanRecord(manufacturerData)
	if !ok {
		t.Fatal("expected scan record to parse")
	}
	if record.Serial != "PR1WDEMO00000000" {
		t.Fatalf("serial = %q, want PR1WDEMO00000000", record.Serial)
	}
}

func TestBuildActiveProbeRequestUsesECDHForEncryptedScanRecord(t *testing.T) {
	t.Parallel()

	request, err := buildActiveProbeRequest(activeProbeAuto, ecoFlowScanRecord{EncryptType: 7})
	if err != nil {
		t.Fatalf("buildActiveProbeRequest() error = %v", err)
	}
	if request.Step != "ecdh_public_key_request" {
		t.Fatalf("step = %q", request.Step)
	}
	decoded := decodeEcoFlowNotification(request.Data)
	if decoded.Frame != "ecoflow_enc_packet" || !strings.Contains(decoded.Detail, "ecdh_public_key") {
		t.Fatalf("decoded request = %#v", decoded)
	}
}

func TestGenerateSECP160R1KeyReturnsPublicKeyOnCurve(t *testing.T) {
	t.Parallel()

	_, publicKey, err := generateSECP160R1Key(rand.Reader)
	if err != nil {
		t.Fatalf("generateSECP160R1Key() error = %v", err)
	}
	if len(publicKey) != 40 {
		t.Fatalf("public key len = %d, want 40", len(publicKey))
	}
	curve := newSECP160R1Curve()
	x := new(big.Int).SetBytes(publicKey[:20])
	y := new(big.Int).SetBytes(publicKey[20:])
	if !curve.isOnCurve(x, y) {
		t.Fatalf("generated public key is not on secp160r1")
	}
}

func TestDeriveType7InitialEncryptionIsSymmetric(t *testing.T) {
	t.Parallel()

	clientPrivate := leftPadBytes(big.NewInt(2), 20)
	devicePrivate := leftPadBytes(big.NewInt(3), 20)
	clientPublic := secp160r1PublicKeyForTest(clientPrivate)
	devicePublic := secp160r1PublicKeyForTest(devicePrivate)

	clientKey, clientIV, err := deriveType7InitialEncryption(clientPrivate, append([]byte{0x01, 0x00, 0x00}, devicePublic...))
	if err != nil {
		t.Fatalf("deriveType7InitialEncryption(client) error = %v", err)
	}
	deviceKey, deviceIV, err := deriveType7InitialEncryption(devicePrivate, append([]byte{0x01, 0x00, 0x00}, clientPublic...))
	if err != nil {
		t.Fatalf("deriveType7InitialEncryption(device) error = %v", err)
	}
	if !bytes.Equal(clientKey, deviceKey) {
		t.Fatalf("session keys differ: client=%x device=%x", clientKey, deviceKey)
	}
	if !bytes.Equal(clientIV, deviceIV) {
		t.Fatalf("ivs differ: client=%x device=%x", clientIV, deviceIV)
	}
	if len(clientKey) != 16 || len(clientIV) != 16 {
		t.Fatalf("key/iv lengths = %d/%d, want 16/16", len(clientKey), len(clientIV))
	}
}

func TestActiveProbeSessionSendsSessionKeyInfoAfterECDHResponse(t *testing.T) {
	t.Parallel()

	writer := &fakeBLEWriter{}
	transport := activeProbeTransport{
		Name:                    bleTransportRFCOMM,
		WriteServiceUUID:        ecoFlowRFCOMMServiceUUID,
		WriteCharacteristicUUID: ecoFlowRFCOMMWriteCharacteristicUUID,
		WriteRole:               "ecoflow_rfcomm_write",
		Write:                   writer,
	}
	privateScalar := leftPadBytes(big.NewInt(2), 20)
	devicePublic := secp160r1PublicKeyForTest(leftPadBytes(big.NewInt(3), 20))
	session := newActiveProbeSession()
	session.trackECDHExchange(transport, privateScalar)

	notifications := session.HandleNotification(rawBLENotification{
		ServiceUUID:        ecoFlowRFCOMMServiceUUID,
		CharacteristicUUID: ecoFlowRFCOMMNotifyCharacteristicUUID,
		Role:               "ecoflow_rfcomm_notify",
		Data:               wrapEcoFlowSimpleEncPacket(append([]byte{0x01, 0x00, 0x00}, devicePublic...)),
	}, probeOptions{RawNotifications: true})

	if len(writer.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(writer.writes))
	}
	decoded := decodeEcoFlowNotification(writer.writes[0])
	if decoded.DecodedHex != "02" {
		t.Fatalf("written decoded hex = %q, want 02", decoded.DecodedHex)
	}
	if len(notifications) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifications))
	}
	got := notifications[0]
	if got.Direction != "tx" || got.Step != "session_key_info_request" {
		t.Fatalf("follow-up notification = %#v, want tx session key request", got)
	}
	if got.DecodedHex != "02" {
		t.Fatalf("follow-up decoded hex = %q, want 02", got.DecodedHex)
	}
}

func TestActiveProbeSessionDecryptsSessionKeyInfoPayload(t *testing.T) {
	t.Parallel()

	initialKey := bytes.Repeat([]byte{0x11}, 16)
	iv := bytes.Repeat([]byte{0x22}, 16)
	plaintext := append(bytes.Repeat([]byte{0x33}, 16), 0x12, 0x34)
	ciphertext := encryptType7ForTest(t, initialKey, iv, plaintext)

	session := newActiveProbeSession()
	session.exchanges[bleTransportRFCOMM] = &activeProbeExchange{
		transport: activeProbeTransport{
			Name: bleTransportRFCOMM,
		},
		stage:      "session_key_info_request",
		initialKey: initialKey,
		iv:         iv,
	}

	notifications := session.HandleNotification(rawBLENotification{
		ServiceUUID:        ecoFlowRFCOMMServiceUUID,
		CharacteristicUUID: ecoFlowRFCOMMNotifyCharacteristicUUID,
		Role:               "ecoflow_rfcomm_notify",
		Data:               wrapEcoFlowSimpleEncPacket(append([]byte{0x02}, ciphertext...)),
	}, probeOptions{RawNotifications: true})

	if len(notifications) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifications))
	}
	got := notifications[0]
	if got.Direction != "rx" || got.Step != "session_key_info_decrypted" {
		t.Fatalf("notification = %#v, want rx decrypted session key info", got)
	}
	if got.DecodedHex != hex.EncodeToString(plaintext) {
		t.Fatalf("decoded hex = %q, want %x", got.DecodedHex, plaintext)
	}
}

func TestDeriveEcoFlowSessionKeyUsesSeedAndSRand(t *testing.T) {
	t.Parallel()

	loginKey := make([]byte, 0x200)
	copy(loginKey[0x1d0:], []byte("0123456789abcdef"))
	srand := []byte("abcdefghijklmnop")
	seed := []byte{0x0d, 0x02}

	got, err := deriveEcoFlowSessionKeyWithLoginKey(seed, srand, loginKey)
	if err != nil {
		t.Fatalf("deriveEcoFlowSessionKeyWithLoginKey() error = %v", err)
	}
	wantData := append([]byte("0123456789abcdef"), srand...)
	want := md5.Sum(wantData)
	if !bytes.Equal(got, want[:]) {
		t.Fatalf("session key = %x, want %x", got, want)
	}
}

func TestActiveProbeSessionSendsEncryptedAuthStatusAfterSessionKeyInfo(t *testing.T) {
	t.Parallel()

	writer := &fakeBLEWriter{}
	initialKey := bytes.Repeat([]byte{0x11}, 16)
	iv := bytes.Repeat([]byte{0x22}, 16)
	loginKey := make([]byte, 0x200)
	copy(loginKey[0x1d0:], []byte("0123456789abcdef"))
	keyInfoPlaintext := append([]byte("abcdefghijklmnop"), 0x0d, 0x02)
	keyInfoCiphertext := encryptType7ForTest(t, initialKey, iv, keyInfoPlaintext)
	session := newActiveProbeSession()
	session.loginKey = loginKey
	session.exchanges[bleTransportRFCOMM] = &activeProbeExchange{
		transport: activeProbeTransport{
			Name:                    bleTransportRFCOMM,
			WriteServiceUUID:        ecoFlowRFCOMMServiceUUID,
			WriteCharacteristicUUID: ecoFlowRFCOMMWriteCharacteristicUUID,
			WriteRole:               "ecoflow_rfcomm_write",
			Write:                   writer,
		},
		stage:      "session_key_info_request",
		initialKey: initialKey,
		iv:         iv,
	}

	notifications := session.HandleNotification(rawBLENotification{
		ServiceUUID:        ecoFlowRFCOMMServiceUUID,
		CharacteristicUUID: ecoFlowRFCOMMNotifyCharacteristicUUID,
		Role:               "ecoflow_rfcomm_notify",
		Data:               wrapEcoFlowSimpleEncPacket(append([]byte{0x02}, keyInfoCiphertext...)),
	}, probeOptions{RawNotifications: true})

	if len(writer.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(writer.writes))
	}
	sessionKey, err := deriveEcoFlowSessionKeyWithLoginKey([]byte{0x0d, 0x02}, []byte("abcdefghijklmnop"), loginKey)
	if err != nil {
		t.Fatalf("derive expected session key: %v", err)
	}
	plaintextPacket := decryptEncPacketPayloadForTest(t, writer.writes[0], sessionKey, iv)
	packet, err := parseEcoFlowPacket(plaintextPacket)
	if err != nil {
		t.Fatalf("parse auth status packet: %v", err)
	}
	if packet.CmdSet != 0x35 || packet.CmdID != 0x89 {
		t.Fatalf("packet command = 0x%02x/0x%02x, want 0x35/0x89", packet.CmdSet, packet.CmdID)
	}
	if len(notifications) != 2 {
		t.Fatalf("notifications = %d, want decrypted key-info and auth-status tx", len(notifications))
	}
	got := notifications[1]
	if got.Direction != "tx" || got.Step != "auth_status_request" {
		t.Fatalf("follow-up notification = %#v, want tx auth status", got)
	}
	if !strings.HasPrefix(got.DecodedHex, "aa03") {
		t.Fatalf("decoded auth-status plaintext = %q, want V3 packet", got.DecodedHex)
	}
}

func TestActiveProbeSessionDecryptsEncryptedType7TelemetryPacket(t *testing.T) {
	t.Parallel()

	sessionKey := bytes.Repeat([]byte{0x44}, 16)
	iv := bytes.Repeat([]byte{0x55}, 16)
	telemetryPayload := encodeFixed32ProtoFieldForTest(3, math.Float32bits(22.5))
	encryptedPacket, err := wrapEcoFlowEncryptedProtocolPacket(buildDelta3DisplayPacketForTest(telemetryPayload), sessionKey, iv)
	if err != nil {
		t.Fatalf("wrap encrypted packet: %v", err)
	}
	session := newActiveProbeSession()
	session.exchanges[bleTransportRFCOMM] = &activeProbeExchange{
		stage:      "auth_status_request",
		sessionKey: sessionKey,
		iv:         iv,
	}

	event := session.HandleNotificationEvent(rawBLENotification{
		ServiceUUID:        ecoFlowRFCOMMServiceUUID,
		CharacteristicUUID: ecoFlowRFCOMMNotifyCharacteristicUUID,
		Role:               "ecoflow_rfcomm_notify",
		Data:               encryptedPacket,
	}, probeOptions{RawNotifications: true})

	if len(event.Notifications) != 1 {
		t.Fatalf("notifications = %d, want 1", len(event.Notifications))
	}
	got := event.Notifications[0]
	if got.Step != "type7_packet_decrypted" {
		t.Fatalf("step = %q, want type7_packet_decrypted", got.Step)
	}
	if !strings.Contains(got.Packet, "description=v3_display_property_upload") {
		t.Fatalf("packet summary = %q, want v3 display upload", got.Packet)
	}
	if metricMap(event.Metrics)["input_power_w"] != "22.5" {
		t.Fatalf("metrics = %#v, want input_power_w=22.5", event.Metrics)
	}
}

func TestActiveProbeSessionSendsAuthRequestWhenCredentialsAvailable(t *testing.T) {
	t.Parallel()

	writer := &fakeBLEWriter{}
	sessionKey := bytes.Repeat([]byte{0x44}, 16)
	iv := bytes.Repeat([]byte{0x55}, 16)
	userID := "1234567890"
	deviceSerial := "PR1WDEMO00000000"
	statusPacket, err := wrapEcoFlowEncryptedProtocolPacket(buildEcoFlowV3Packet(0x35, 0x21, 0x35, 0x89, []byte{0x01, 0x00}), sessionKey, iv)
	if err != nil {
		t.Fatalf("wrap status packet: %v", err)
	}
	session := newActiveProbeSession()
	session.authUserID = userID
	session.authDeviceSerial = deviceSerial
	session.exchanges[bleTransportRFCOMM] = &activeProbeExchange{
		transport: activeProbeTransport{
			Name:                    bleTransportRFCOMM,
			WriteServiceUUID:        ecoFlowRFCOMMServiceUUID,
			WriteCharacteristicUUID: ecoFlowRFCOMMWriteCharacteristicUUID,
			WriteRole:               "ecoflow_rfcomm_write",
			Write:                   writer,
		},
		stage:      "auth_status_request",
		sessionKey: sessionKey,
		iv:         iv,
	}

	event := session.HandleNotificationEvent(rawBLENotification{
		ServiceUUID:        ecoFlowRFCOMMServiceUUID,
		CharacteristicUUID: ecoFlowRFCOMMNotifyCharacteristicUUID,
		Role:               "ecoflow_rfcomm_notify",
		Data:               statusPacket,
	}, probeOptions{RawNotifications: true})

	if len(writer.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(writer.writes))
	}
	plaintext := decryptEncPacketPayloadForTest(t, writer.writes[0], sessionKey, iv)
	packet, err := parseEcoFlowPacket(plaintext)
	if err != nil {
		t.Fatalf("parse auth packet: %v", err)
	}
	if packet.CmdSet != 0x35 || packet.CmdID != 0x86 {
		t.Fatalf("packet command = 0x%02x/0x%02x, want 0x35/0x86", packet.CmdSet, packet.CmdID)
	}
	wantMD5 := md5.Sum([]byte(userID + deviceSerial))
	wantPayload := strings.ToUpper(hex.EncodeToString(wantMD5[:]))
	if string(packet.Payload) != wantPayload {
		t.Fatalf("auth payload = %q, want %q", string(packet.Payload), wantPayload)
	}
	if len(event.Notifications) != 2 {
		t.Fatalf("notifications = %d, want decrypted status and auth tx", len(event.Notifications))
	}
	authNotification := event.Notifications[1]
	if authNotification.Step != "auth_request" {
		t.Fatalf("step = %q, want auth_request", authNotification.Step)
	}
	if authNotification.DecodedHex != "" {
		t.Fatalf("auth decoded hex leaked sensitive payload: %q", authNotification.DecodedHex)
	}
}

func TestRunDiscoveryWithFakeScannerPrintsStableSummary(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runDiscovery(
		t.Context(),
		[]string{"-duration=25ms", "-format=text", "-scan-only"},
		strings.NewReader(""),
		&stdout,
		&stderr,
		fakeScanner{devices: []discoveredBLEDevice{
			{Address: "AA:BB:CC:DD:EE:01", RSSI: -60, LocalName: "Keyboard"},
			{Address: "AA:BB:CC:DD:EE:02", RSSI: -52, LocalName: "EF-PR12DEMO0000"},
			{Address: "AA:BB:CC:DD:EE:02", RSSI: -48, LocalName: "EF-PR12DEMO0000"},
		}},
		nil,
	)
	if err != nil {
		t.Fatalf("runDiscovery() error = %v", err)
	}
	output := stdout.String()
	if strings.Count(output, "device address=") != 1 {
		t.Fatalf("expected one deduped EcoFlow device line, got:\n%s", output)
	}
	if !strings.Contains(output, "summary seen=1 ecoflow=1") {
		t.Fatalf("missing summary, got:\n%s", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestValidateDiscoveryConfigRejectsBadValues(t *testing.T) {
	t.Parallel()

	if err := validateDiscoveryConfig(discoveryConfig{Duration: time.Second, Format: "xml"}); err == nil {
		t.Fatal("expected invalid format to fail")
	}
	if err := validateDiscoveryConfig(discoveryConfig{Duration: -time.Second, Format: outputFormatText}); err == nil {
		t.Fatal("expected negative duration to fail")
	}
	if err := validateDiscoveryConfig(discoveryConfig{Duration: time.Second, Format: outputFormatText, ProbeTimeout: time.Second, ListenDuration: -time.Second}); err == nil {
		t.Fatal("expected negative listen duration to fail")
	}
}

func TestEnableBluetoothAdapterTreatsDarwinSecondEnableAsReady(t *testing.T) {
	t.Parallel()

	adapter := fakeEnableAdapter{err: errors.New("already calling Enable function")}
	if err := enableBluetoothAdapter(adapter); err != nil {
		t.Fatalf("enableBluetoothAdapter() error = %v, want nil", err)
	}

	adapter.err = errors.New("adapter unavailable")
	if err := enableBluetoothAdapter(adapter); err == nil {
		t.Fatal("expected non-idempotent adapter error to fail")
	}
}

func TestRunDiscoveryReturnsWriteErrors(t *testing.T) {
	t.Parallel()

	err := runDiscovery(
		t.Context(),
		[]string{"-duration=25ms", "-scan-only"},
		strings.NewReader(""),
		failingWriter{},
		&bytes.Buffer{},
		fakeScanner{devices: []discoveredBLEDevice{
			{Address: "AA:BB:CC:DD:EE:02", RSSI: -52, LocalName: "EF-PR12DEMO0000"},
		}},
		nil,
	)
	if err == nil {
		t.Fatal("expected write error")
	}
	if !strings.Contains(err.Error(), "write discovery device") {
		t.Fatalf("error = %q", err)
	}
}

func encodeFixed32ProtoFieldForTest(fieldNumber int, value uint32) []byte {
	out := encodeVarintForTest(uint64(fieldNumber<<3 | 5))
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], value)
	return append(out, buf[:]...)
}

func encodeVarintProtoFieldForTest(fieldNumber int, value uint64) []byte {
	out := encodeVarintForTest(uint64(fieldNumber << 3))
	return append(out, encodeVarintForTest(value)...)
}

func encodeVarintForTest(value uint64) []byte {
	var out []byte
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

func buildDelta3DisplayPacketForTest(payload []byte) []byte {
	packet := []byte{0xaa, 0x03}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(len(payload)))
	packet = append(packet, ecoFlowCRC8(packet))
	packet = append(packet,
		0x0d,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00,
		0x02, 0x21,
		0x01, 0x01,
		0xfe, 0x15,
	)
	packet = append(packet, payload...)
	packet = binary.LittleEndian.AppendUint16(packet, ecoFlowCRC16(packet))
	return packet
}

func buildEncPacketForTest(frameType byte, payload []byte) []byte {
	packet := []byte{0x5a, 0x5a, frameType << 4, 0x01}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(len(payload)+2))
	packet = append(packet, payload...)
	packet = binary.LittleEndian.AppendUint16(packet, ecoFlowCRC16(packet))
	return packet
}

func secp160r1PublicKeyForTest(privateScalar []byte) []byte {
	curve := newSECP160R1Curve()
	x, y := curve.scalarMult(curve.gx, curve.gy, privateScalar)
	return append(leftPadBytes(x, 20), leftPadBytes(y, 20)...)
}

func encryptType7ForTest(t *testing.T, key []byte, iv []byte, plaintext []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := append([]byte(nil), plaintext...)
	padded = append(padded, bytes.Repeat([]byte{byte(padding)}, padding)...)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return out
}

func decryptEncPacketPayloadForTest(t *testing.T, data []byte, key []byte, iv []byte) []byte {
	t.Helper()
	if len(data) < 8 || data[0] != 0x5a || data[1] != 0x5a {
		t.Fatalf("invalid EncPacket: %x", data)
	}
	payloadLength := int(binary.LittleEndian.Uint16(data[4:6]))
	payload := data[6 : 6+payloadLength-2]
	plaintext, err := decryptEcoFlowType7(payload, key, iv)
	if err != nil {
		t.Fatalf("decrypt EncPacket payload: %v", err)
	}
	return plaintext
}

func metricMap(metrics []probeMetric) map[string]string {
	got := make(map[string]string, len(metrics))
	for _, metric := range metrics {
		got[metric.Name] = metric.Value
	}
	return got
}

type fakeScanner struct {
	devices []discoveredBLEDevice
}

func (s fakeScanner) Scan(_ <-chan struct{}, emit func(discoveredBLEDevice)) error {
	for _, device := range s.devices {
		emit(device)
	}
	return nil
}

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type fakeBLEWriter struct {
	writes [][]byte
	err    error
}

func (w *fakeBLEWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	w.writes = append(w.writes, append([]byte(nil), data...))
	return len(data), nil
}

type fakeEnableAdapter struct {
	err error
}

func (a fakeEnableAdapter) Enable() error {
	return a.err
}

type fakeProber struct {
	result  deviceProbe
	devices []discoveredBLEDevice
	err     error
}

func (p *fakeProber) Probe(_ context.Context, device discoveredBLEDevice) (deviceProbe, error) {
	p.devices = append(p.devices, device)
	if p.err != nil {
		return deviceProbe{}, p.err
	}
	return p.result, nil
}

type fakeStreamingProber struct {
	stdout            *bytes.Buffer
	outputDuringProbe string
	devices           []discoveredBLEDevice
}

func (p *fakeStreamingProber) Probe(_ context.Context, device discoveredBLEDevice) (deviceProbe, error) {
	p.devices = append(p.devices, device)
	return deviceProbe{}, nil
}

func (p *fakeStreamingProber) ProbeWithOptions(_ context.Context, device discoveredBLEDevice, options probeOptions) (deviceProbe, error) {
	p.devices = append(p.devices, device)
	if options.EventSink == nil {
		return deviceProbe{}, errors.New("missing probe event sink")
	}
	probe := deviceProbe{
		Capabilities: probeCapabilities{
			ServiceCount:        1,
			CharacteristicCount: 1,
			Services: []probeService{
				{UUID: ecoFlowRFCOMMServiceUUID, Characteristics: []string{ecoFlowRFCOMMNotifyCharacteristicUUID}},
			},
		},
		Notifications: []probeNotification{
			{
				Direction:          "rx",
				ServiceUUID:        ecoFlowRFCOMMServiceUUID,
				CharacteristicUUID: ecoFlowRFCOMMNotifyCharacteristicUUID,
				Role:               "ecoflow_rfcomm_notify",
				Bytes:              3,
				Frame:              "ecoflow_enc_packet",
				Detail:             "simple=session_key_info",
			},
		},
		Metrics: []probeMetric{
			{Name: "streamed_power_w", Value: "123", Unit: "W", Source: "ble_notify"},
		},
	}
	if err := options.EventSink(probeEvent{Capabilities: &probe.Capabilities}); err != nil {
		return deviceProbe{}, err
	}
	if err := options.EventSink(probeEvent{
		Notifications: probe.Notifications,
		Metrics:       probe.Metrics,
	}); err != nil {
		return deviceProbe{}, err
	}
	p.outputDuringProbe = p.stdout.String()
	return probe, nil
}

type concurrentStreamingProber struct {
	mu          sync.Mutex
	active      int
	maxActive   int
	devices     []discoveredBLEDevice
	activeModes []activeProbeMode
}

type cancelAwareProber struct {
	started             chan struct{}
	activeProbe         activeProbeMode
	listenUntilCanceled bool
}

type authFailingStreamingProber struct {
	failName       string
	cancelObserved atomic.Bool
}

func (p *cancelAwareProber) Probe(_ context.Context, device discoveredBLEDevice) (deviceProbe, error) {
	return p.ProbeWithOptions(context.Background(), device, probeOptions{})
}

func (p *cancelAwareProber) ProbeWithOptions(ctx context.Context, _ discoveredBLEDevice, options probeOptions) (deviceProbe, error) {
	p.activeProbe = options.ActiveProbe
	p.listenUntilCanceled = options.ListenUntilCanceled
	close(p.started)
	<-ctx.Done()
	return deviceProbe{}, nil
}

func newConcurrentStreamingProber() *concurrentStreamingProber {
	return &concurrentStreamingProber{}
}

func (p *authFailingStreamingProber) Probe(_ context.Context, device discoveredBLEDevice) (deviceProbe, error) {
	return p.ProbeWithOptions(context.Background(), device, probeOptions{})
}

func (p *authFailingStreamingProber) ProbeWithOptions(ctx context.Context, device discoveredBLEDevice, options probeOptions) (deviceProbe, error) {
	if device.LocalName == p.failName {
		time.Sleep(time.Millisecond)
		if options.EventSink != nil {
			return deviceProbe{}, options.EventSink(probeEvent{Metrics: []probeMetric{
				{Name: "auth_result", Value: "wrong_key", Source: "ble_auth"},
			}})
		}
		return deviceProbe{Metrics: []probeMetric{{Name: "auth_result", Value: "wrong_key", Source: "ble_auth"}}}, nil
	}
	select {
	case <-ctx.Done():
		p.cancelObserved.Store(true)
	case <-time.After(5 * time.Millisecond):
	}
	return deviceProbe{}, nil
}

func (p *concurrentStreamingProber) Probe(_ context.Context, device discoveredBLEDevice) (deviceProbe, error) {
	return p.ProbeWithOptions(context.Background(), device, probeOptions{})
}

func (p *concurrentStreamingProber) ProbeWithOptions(ctx context.Context, device discoveredBLEDevice, options probeOptions) (deviceProbe, error) {
	p.mu.Lock()
	p.active++
	if p.active > p.maxActive {
		p.maxActive = p.active
	}
	p.devices = append(p.devices, device)
	p.activeModes = append(p.activeModes, options.ActiveProbe)
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
	}()

	if options.EventSink != nil {
		capabilities := probeCapabilities{
			ServiceCount:        1,
			CharacteristicCount: 2,
			MTUs:                []uint16{497},
		}
		if err := options.EventSink(probeEvent{Capabilities: &capabilities}); err != nil {
			return deviceProbe{}, err
		}
		metrics := []probeMetric{
			{Name: "battery_soc_percent", Value: "79", Unit: "%", Source: "ble_notify", Decoder: "v3_display"},
			{Name: "input_power_w", Value: "123", Unit: "W", Source: "ble_notify", Decoder: "v3_display"},
			{Name: "output_power_w", Value: "321", Unit: "W", Source: "ble_notify", Decoder: "v3_display"},
			{Name: "pv_input_power_w", Value: "55", Unit: "W", Source: "ble_notify", Decoder: "v3_display"},
			{Name: "battery_discharge_remaining_min", Value: "1983", Unit: "min", Source: "ble_notify", Decoder: "v3_display"},
		}
		if err := options.EventSink(probeEvent{Metrics: metrics}); err != nil {
			return deviceProbe{}, err
		}
	}

	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		if p.maxActiveProbeCount() >= 2 {
			break
		}
		runtime.Gosched()
	}
	return deviceProbe{}, nil
}

func (p *concurrentStreamingProber) maxActiveProbeCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.maxActive
}

func (p *concurrentStreamingProber) probedNames() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	names := make([]string, 0, len(p.devices))
	for _, device := range p.devices {
		names = append(names, device.LocalName)
	}
	sort.Strings(names)
	return names
}

func (p *concurrentStreamingProber) activeProbeModes() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	modes := make([]string, 0, len(p.activeModes))
	for _, mode := range p.activeModes {
		modes = append(modes, string(mode))
	}
	sort.Strings(modes)
	return modes
}
