package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPanelHintsSupportsWrappedPanelsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "panel-map.json")
	content := `{"panels":[{"device_sn":"sn-1","product_name":"Delta 2 Max","profile":"d2m","port":"low","panel_setup":"2x 220W","panel_count":2,"nominal_total_w":440}]}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write panel map: %v", err)
	}

	hints, err := loadPanelHints(path)
	if err != nil {
		t.Fatalf("loadPanelHints() error = %v", err)
	}
	hint, ok := hints.Resolve("sn-1", "Delta 2 Max", "d2m", "low")
	if !ok {
		t.Fatal("expected hint to resolve")
	}
	if hint.PanelSetup != "2x 220W" || hint.PanelCount != 2 {
		t.Fatalf("hint mismatch: %+v", hint)
	}
}
