package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
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
}

func TestRunDiscoveryPromptsForDeviceSelectionAndPrintsProbe(t *testing.T) {
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
	err := runDiscovery(
		t.Context(),
		[]string{"-duration=25ms", "-redact=false"},
		strings.NewReader("2\n"),
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
		`1) address=AA:BB:CC:DD:EE:01 name="EF-YJ0000"`,
		`2) address=AA:BB:CC:DD:EE:02 name="EF-PR1W0000"`,
		"select device [1-2, empty to skip]:",
		`probing address=AA:BB:CC:DD:EE:02 name="EF-PR1W0000" model="EcoFlow DELTA 3 1000 Air"`,
		"capabilities services=1 characteristics=2 mtus=23",
		"service uuid=0000ec00-0000-1000-8000-00805f9b34fb characteristics=2",
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
