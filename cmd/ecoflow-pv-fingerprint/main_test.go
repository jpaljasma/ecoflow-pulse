package main

import (
	"encoding/csv"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzePVFingerprintsComputesMedianAndStates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "training.csv")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create csv: %v", err)
	}
	writer := csv.NewWriter(file)
	header := []string{
		"ts_unix_ms",
		"device_sn",
		"product_name",
		"solar_low_in_w",
		"solar_high_in_w",
		"solar_low_v",
		"solar_high_v",
		"solar_low_a",
		"solar_high_a",
		"mppt_low_state",
		"mppt_high_state",
	}
	if err := writer.Write(header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	rows := [][]string{
		{"1000", "RTESTSN", "DELTA 2 Max", "100", "0", "20", "0", "5", "0", "charging", ""},
		{"2000", "RTESTSN", "DELTA 2 Max", "200", "300", "25", "30", "8", "10", "charging", "charging"},
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	hints, err := loadPanelHints("")
	if err != nil {
		t.Fatalf("load panel hints: %v", err)
	}
	result, err := analyzePVFingerprints(path, hints)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("row count mismatch: got=%d want=2", len(result))
	}

	low := mustFindPort(t, result, "RTESTSN", "low")
	high := mustFindPort(t, result, "RTESTSN", "high")

	if math.Abs(low.MedianW-150) > 1e-9 {
		t.Fatalf("low median_w mismatch: got=%f want=150", low.MedianW)
	}
	if math.Abs(low.MedianActiveW-150) > 1e-9 {
		t.Fatalf("low median_active_w mismatch: got=%f want=150", low.MedianActiveW)
	}
	if math.Abs(high.MedianW-150) > 1e-9 {
		t.Fatalf("high median_w mismatch: got=%f want=150", high.MedianW)
	}
	if math.Abs(high.MedianActiveW-300) > 1e-9 {
		t.Fatalf("high median_active_w mismatch: got=%f want=300", high.MedianActiveW)
	}
	if math.Abs(high.StateEmptyPct-50) > 1e-9 {
		t.Fatalf("high empty state pct mismatch: got=%f want=50", high.StateEmptyPct)
	}
	if math.Abs(high.StateChargingPct-50) > 1e-9 {
		t.Fatalf("high charging state pct mismatch: got=%f want=50", high.StateChargingPct)
	}
	if low.CapW != 500 || high.CapW != 500 {
		t.Fatalf("capability mapping mismatch: low=%f high=%f", low.CapW, high.CapW)
	}
}

func TestLoadPanelHintsOverridesDefaultBySN(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panel_map.json")
	content := `[
  {
    "device_sn": "DEMODPU0000294",
    "port": "low",
    "panel_setup": "OVERRIDE",
    "panel_count": 9,
    "nominal_total_w": 999
  }
]`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write panel map: %v", err)
	}

	hints, err := loadPanelHints(path)
	if err != nil {
		t.Fatalf("load panel hints: %v", err)
	}
	hint, ok := hints.Resolve("DEMODPU0000294", "DELTA Pro Ultra", "low")
	if !ok {
		t.Fatalf("expected hint to resolve")
	}
	if hint.PanelSetup != "OVERRIDE" || hint.PanelCount != 9 || math.Abs(hint.NominalTotalW-999) > 1e-9 {
		t.Fatalf("override mismatch: %+v", hint)
	}
}

func mustFindPort(t *testing.T, rows []fingerprintRow, sn string, port string) fingerprintRow {
	t.Helper()
	for _, row := range rows {
		if row.SN == sn && row.Port == port {
			return row
		}
	}
	t.Fatalf("missing row sn=%s port=%s", sn, port)
	return fingerprintRow{}
}
