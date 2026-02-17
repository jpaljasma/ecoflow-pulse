package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

func TestInferPanelDeviceTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		device ecoflow.GeneralInfoDevice
		want   []string
	}{
		{
			name: "dpu",
			device: ecoflow.GeneralInfoDevice{
				ProductName: "DELTA Pro Ultra",
				DeviceName:  "DPU A 12 kWh",
			},
			want: []string{"dpu_low", "dpu_high", "dpu_x_high"},
		},
		{
			name: "d2m",
			device: ecoflow.GeneralInfoDevice{
				ProductName: "DELTA 2 Max",
			},
			want: []string{"d2_d2_max"},
		},
		{
			name: "unknown",
			device: ecoflow.GeneralInfoDevice{
				ProductName: "RIVER 3 Plus",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := inferPanelDeviceTags(tt.device)
			if len(got) != len(tt.want) {
				t.Fatalf("tags length mismatch: got=%v want=%v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("tags mismatch: got=%v want=%v", got, tt.want)
				}
			}
		})
	}
}

func TestLoadSolarPanelIndexAndCandidates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "panel.index.json")
	content := `{
  "source_csv": "/tmp/a.csv",
  "generated_at_utc": "2026-02-17T00:00:00Z",
  "row_count": 2,
  "by_panel_key": {
    "panel_a": {"id":"a","brand":"EcoFlow","model":"220W"},
    "panel_b": {"id":"b","brand":"JA Solar","model":"JAM54"}
  },
  "by_device_tag": {
    "d2_d2_max": ["panel_a"],
    "dpu_low": ["panel_a", "panel_b"]
  },
  "device_labels": {
    "d2_d2_max": "D2/D2 Max 11–60V/15A"
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	db, err := loadSolarPanelIndex(path)
	if err != nil {
		t.Fatalf("loadSolarPanelIndex returned error: %v", err)
	}
	if db.PanelCount != 2 {
		t.Fatalf("panel count mismatch: got=%d want=2", db.PanelCount)
	}
	got := db.candidatePanelsForDeviceTags([]string{"d2_d2_max", "dpu_low"})
	if len(got) != 2 || got[0] != "panel_a" || got[1] != "panel_b" {
		t.Fatalf("candidate panels mismatch: got=%v", got)
	}
}
