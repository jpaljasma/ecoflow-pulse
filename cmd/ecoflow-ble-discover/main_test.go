package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestInferEcoFlowModelFromDelta3AirBLEName(t *testing.T) {
	t.Parallel()

	got := inferEcoFlowDevice("EF-PR12ZA1CDHAW0498")
	if !got.Matched {
		t.Fatal("expected EcoFlow BLE name to match")
	}
	if got.SerialPrefix != "PR12" {
		t.Fatalf("serial prefix = %q, want PR12", got.SerialPrefix)
	}
	if got.Model != "EcoFlow DELTA 3 1000 Air (10ms UPS)" {
		t.Fatalf("model = %q", got.Model)
	}
	if got.PacketFamily != "v3" {
		t.Fatalf("packet family = %q, want v3", got.PacketFamily)
	}
}

func TestFilterDiscoveryHonorsEcoFlowAndRSSI(t *testing.T) {
	t.Parallel()

	cfg := discoveryConfig{MinRSSI: -85}
	if !shouldIncludeAdvertisement(discoveredBLEDevice{LocalName: "EF-PR12ZA1CDHAW0498", RSSI: -70}, cfg) {
		t.Fatal("expected EcoFlow candidate above RSSI floor to be included")
	}
	if shouldIncludeAdvertisement(discoveredBLEDevice{LocalName: "EF-PR12ZA1CDHAW0498", RSSI: -91}, cfg) {
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
		LocalName: "EF-PR12ZA1CDHAW0498",
		Info: ecoFlowDeviceInfo{
			Matched:      true,
			SerialPrefix: "PR12",
			Model:        "EcoFlow DELTA 3 1000 Air (10ms UPS)",
			PacketFamily: "v3",
		},
	}, true)

	if strings.Contains(line, "001122334455") || strings.Contains(line, "ZA1CDHAW0498") {
		t.Fatalf("line leaked raw identifier: %s", line)
	}
	for _, want := range []string{"address=A1B2...4455", `name="EF-PR...0498"`, `model="EcoFlow DELTA 3 1000 Air (10ms UPS)"`, "packets=v3"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line %q missing %q", line, want)
		}
	}
}

func TestRunDiscoveryWithFakeScannerPrintsStableSummary(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runDiscovery(
		t.Context(),
		[]string{"-duration=25ms", "-format=text"},
		&stdout,
		&stderr,
		fakeScanner{devices: []discoveredBLEDevice{
			{Address: "AA:BB:CC:DD:EE:01", RSSI: -60, LocalName: "Keyboard"},
			{Address: "AA:BB:CC:DD:EE:02", RSSI: -52, LocalName: "EF-PR12ZA1CDHAW0498"},
			{Address: "AA:BB:CC:DD:EE:02", RSSI: -48, LocalName: "EF-PR12ZA1CDHAW0498"},
		}},
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

func TestRunDiscoveryReturnsWriteErrors(t *testing.T) {
	t.Parallel()

	err := runDiscovery(
		t.Context(),
		[]string{"-duration=25ms"},
		failingWriter{},
		&bytes.Buffer{},
		fakeScanner{devices: []discoveredBLEDevice{
			{Address: "AA:BB:CC:DD:EE:02", RSSI: -52, LocalName: "EF-PR12ZA1CDHAW0498"},
		}},
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
