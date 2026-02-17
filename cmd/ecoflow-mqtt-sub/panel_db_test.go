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

func TestApplyPanelDBPortMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "panel.index.json")
	content := `{
  "source_csv": "/tmp/a.csv",
  "generated_at_utc": "2026-02-17T00:00:00Z",
  "row_count": 5,
  "panel_count": 5,
  "by_panel_key": {
    "eco_220": {
      "id": "eco_220",
      "brand": "EcoFlow",
      "model": "220W Bifacial Portable",
      "pmax_stc_w": 220,
      "purchase_link": "https://www.ecoflow.com/us/220w-bifacial-portable-solar-panel",
      "compatibility": {"d2_d2_max":{"status":"compatible","max_series":2}}
    },
    "ja_400": {
      "id": "ja_400",
      "brand": "JA Solar",
      "model": "400W Bifacial",
      "pmax_stc_w": 400,
      "compatibility": {"dpu_low":{"status":"compatible","max_series":4}}
    },
    "ja_320": {
      "id": "ja_320",
      "brand": "JA Solar",
      "model": "320W Mono",
      "pmax_stc_w": 320,
      "compatibility": {"dpu_low":{"status":"compatible","max_series":4}}
    },
    "big_hv": {
      "id": "big_hv",
      "brand": "HV Solar",
      "model": "900W Array Module",
      "pmax_stc_w": 900,
      "compatibility": {"dpu_high":{"status":"compatible","max_series":6}}
    },
    "mid_hv": {
      "id": "mid_hv",
      "brand": "HV Solar",
      "model": "750W Array Module",
      "pmax_stc_w": 750,
      "compatibility": {"dpu_high":{"status":"compatible","max_series":6}}
    }
  },
  "by_device_tag": {
    "d2_d2_max": ["eco_220"],
    "dpu_low": ["ja_320", "ja_400"],
    "dpu_high": ["mid_hv", "big_hv"]
  },
  "device_labels": {}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	db, err := loadSolarPanelIndex(path)
	if err != nil {
		t.Fatalf("loadSolarPanelIndex returned error: %v", err)
	}

	d2m := ecoflow.GeneralInfoDevice{ProductName: "DELTA 2 Max"}
	d2mSnapshot := newEnergySnapshot()
	applyPanelDBPortMetadata(d2mSnapshot, db, d2m)
	if !d2mSnapshot.HasPVLowBestPanelLabel || d2mSnapshot.PVLowBestPanelLabel == "" {
		t.Fatalf("expected d2m low best panel label")
	}
	if !d2mSnapshot.HasPVLowBestPanelWatts || d2mSnapshot.PVLowBestPanelWatts != 220 {
		t.Fatalf("expected d2m low best panel watts=220, got has=%v value=%.1f", d2mSnapshot.HasPVLowBestPanelWatts, d2mSnapshot.PVLowBestPanelWatts)
	}
	if !d2mSnapshot.HasPVLowBestPanelType || !d2mSnapshot.PVLowBestPanelBifacial {
		t.Fatalf("expected d2m low best panel type=bifacial, got hasType=%v bifacial=%v", d2mSnapshot.HasPVLowBestPanelType, d2mSnapshot.PVLowBestPanelBifacial)
	}
	if !d2mSnapshot.HasPVLowDBCandidates || len(d2mSnapshot.PVLowDBCandidates) == 0 {
		t.Fatalf("expected d2m low db candidates to be populated")
	}
	if got := d2mSnapshot.PVLowDBCandidates[0].Label; got == "" {
		t.Fatalf("expected non-empty d2m low candidate label")
	}
	if got := d2mSnapshot.PVLowDBCandidates[0].PurchaseLink; got == "" {
		t.Fatalf("expected non-empty d2m low candidate purchase link")
	}
	if !d2mSnapshot.HasPVLowBestPanelLink || d2mSnapshot.PVLowBestPanelLink == "" {
		t.Fatalf("expected d2m low best panel purchase link to be populated")
	}

	dpu := ecoflow.GeneralInfoDevice{ProductName: "DELTA Pro Ultra"}
	dpuSnapshot := newEnergySnapshot()
	applyPanelDBPortMetadata(dpuSnapshot, db, dpu)
	if !dpuSnapshot.HasPVLowBestPanelWatts || dpuSnapshot.PVLowBestPanelWatts != 400 {
		t.Fatalf("expected dpu low best panel watts=400, got has=%v value=%.1f", dpuSnapshot.HasPVLowBestPanelWatts, dpuSnapshot.PVLowBestPanelWatts)
	}
	if !dpuSnapshot.HasPVLowAltPanelWatts || dpuSnapshot.PVLowAltPanelWatts != 320 {
		t.Fatalf("expected dpu low alt panel watts=320, got has=%v value=%.1f", dpuSnapshot.HasPVLowAltPanelWatts, dpuSnapshot.PVLowAltPanelWatts)
	}
	if !dpuSnapshot.HasPVLowBestPanelType || !dpuSnapshot.PVLowBestPanelBifacial {
		t.Fatalf("expected dpu low best panel type=bifacial, got hasType=%v bifacial=%v", dpuSnapshot.HasPVLowBestPanelType, dpuSnapshot.PVLowBestPanelBifacial)
	}
	if !dpuSnapshot.HasPVLowAltPanelType || dpuSnapshot.PVLowAltPanelBifacial {
		t.Fatalf("expected dpu low alt panel type=non-bifacial, got hasType=%v bifacial=%v", dpuSnapshot.HasPVLowAltPanelType, dpuSnapshot.PVLowAltPanelBifacial)
	}
	if !dpuSnapshot.HasPVHighBestPanelWatts || dpuSnapshot.PVHighBestPanelWatts != 900 {
		t.Fatalf("expected dpu high best panel watts=900, got has=%v value=%.1f", dpuSnapshot.HasPVHighBestPanelWatts, dpuSnapshot.PVHighBestPanelWatts)
	}
	if !dpuSnapshot.HasPVHighAltPanelWatts || dpuSnapshot.PVHighAltPanelWatts != 750 {
		t.Fatalf("expected dpu high alt panel watts=750, got has=%v value=%.1f", dpuSnapshot.HasPVHighAltPanelWatts, dpuSnapshot.PVHighAltPanelWatts)
	}
	if !dpuSnapshot.HasPVHighBestPanelType || dpuSnapshot.PVHighBestPanelBifacial {
		t.Fatalf("expected dpu high best panel type=non-bifacial, got hasType=%v bifacial=%v", dpuSnapshot.HasPVHighBestPanelType, dpuSnapshot.PVHighBestPanelBifacial)
	}
	if !dpuSnapshot.HasPVHighAltPanelType || dpuSnapshot.PVHighAltPanelBifacial {
		t.Fatalf("expected dpu high alt panel type=non-bifacial, got hasType=%v bifacial=%v", dpuSnapshot.HasPVHighAltPanelType, dpuSnapshot.PVHighAltPanelBifacial)
	}
	if !dpuSnapshot.HasPVHighDBCandidates || len(dpuSnapshot.PVHighDBCandidates) != 2 {
		t.Fatalf("expected dpu high db candidates length=2, got has=%v len=%d", dpuSnapshot.HasPVHighDBCandidates, len(dpuSnapshot.PVHighDBCandidates))
	}
}

func TestTopPanelCandidatesForChannelAppliesColdVocSafety(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "panel.index.json")
	content := `{
  "source_csv": "/tmp/a.csv",
  "generated_at_utc": "2026-02-17T00:00:00Z",
  "row_count": 2,
  "panel_count": 2,
  "by_panel_key": {
    "risky_600": {
      "id": "risky_600",
      "brand": "Risky",
      "model": "600W HV",
      "pmax_stc_w": 600,
      "voc_v": 55.0,
      "compatibility": {
        "d2_d2_max": {"label":"D2/D2 Max 11–60V/15A","status":"yes","max_series":2}
      }
    },
    "safe_400": {
      "id": "safe_400",
      "brand": "Safe",
      "model": "400W LV",
      "pmax_stc_w": 400,
      "voc_v": 30.0,
      "compatibility": {
        "d2_d2_max": {"label":"D2/D2 Max 11–60V/15A","status":"yes","max_series":2}
      }
    }
  },
  "by_device_tag": {
    "d2_d2_max": ["risky_600", "safe_400"]
  },
  "device_labels": {}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	db, err := loadSolarPanelIndex(path)
	if err != nil {
		t.Fatalf("loadSolarPanelIndex returned error: %v", err)
	}
	device := ecoflow.GeneralInfoDevice{ProductName: "DELTA 2 Max"}
	candidates := topPanelCandidatesForChannel(db, device, "low", 2)
	if len(candidates) != 1 {
		t.Fatalf("expected only one cold-safe candidate, got=%d", len(candidates))
	}
	if candidates[0].record.ID != "safe_400" {
		t.Fatalf("unexpected candidate selected: got=%s want=safe_400", candidates[0].record.ID)
	}
	if candidates[0].maxSeries != 1 {
		t.Fatalf("expected cold Voc safety to cap series to 1, got=%d", candidates[0].maxSeries)
	}
}

func TestTopPanelCandidatesForChannelAppliesCurrentSafety(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "panel.index.json")
	content := `{
  "source_csv": "/tmp/a.csv",
  "generated_at_utc": "2026-02-17T00:00:00Z",
  "row_count": 2,
  "panel_count": 2,
  "by_panel_key": {
    "high_current_500": {
      "id": "high_current_500",
      "brand": "Unsafe",
      "model": "500W HC",
      "pmax_stc_w": 500,
      "voc_v": 40.0,
      "imp_a": 16.2,
      "compatibility": {
        "d2_d2_max": {"label":"D2/D2 Max 11–60V/15A","status":"yes","max_series":2}
      }
    },
    "safe_current_450": {
      "id": "safe_current_450",
      "brand": "Safe",
      "model": "450W",
      "pmax_stc_w": 450,
      "voc_v": 40.0,
      "imp_a": 12.5,
      "compatibility": {
        "d2_d2_max": {"label":"D2/D2 Max 11–60V/15A","status":"yes","max_series":2}
      }
    }
  },
  "by_device_tag": {
    "d2_d2_max": ["high_current_500", "safe_current_450"]
  },
  "device_labels": {}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	db, err := loadSolarPanelIndex(path)
	if err != nil {
		t.Fatalf("loadSolarPanelIndex returned error: %v", err)
	}
	device := ecoflow.GeneralInfoDevice{ProductName: "DELTA 2 Max"}
	candidates := topPanelCandidatesForChannel(db, device, "low", 2)
	if len(candidates) != 1 {
		t.Fatalf("expected one current-safe candidate, got=%d", len(candidates))
	}
	if candidates[0].record.ID != "safe_current_450" {
		t.Fatalf("unexpected candidate selected: got=%s want=safe_current_450", candidates[0].record.ID)
	}
}

func TestSelectPanelCandidatesForMetadataPrefersDistinctAlt(t *testing.T) {
	t.Parallel()

	candidates := []panelChannelCandidate{
		{
			record: solarPanelRecord{
				ID:       "a",
				Brand:    "BrandA",
				Model:    "ModelA 500W",
				PmaxSTCW: 500,
			},
			maxSeries: 2,
			score:     3,
		},
		{
			record: solarPanelRecord{
				ID:       "b",
				Brand:    "BrandA",
				Model:    "ModelA 510W",
				PmaxSTCW: 510,
			},
			maxSeries: 2,
			score:     3,
		},
		{
			record: solarPanelRecord{
				ID:       "c",
				Brand:    "BrandB",
				Model:    "ModelB 420W",
				PmaxSTCW: 420,
			},
			maxSeries: 3,
			score:     3,
		},
	}

	best, hasBest, alt, hasAlt := selectPanelCandidatesForMetadata(candidates)
	if !hasBest || best.record.ID != "a" {
		t.Fatalf("unexpected best candidate: has=%v id=%s", hasBest, best.record.ID)
	}
	if !hasAlt || alt.record.ID != "c" {
		t.Fatalf("expected distinct alt candidate id=c, got has=%v id=%s", hasAlt, alt.record.ID)
	}
}

func TestTopPanelCandidatesForChannelPrefersHigherEfficiencyOnEqualPower(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "panel.index.json")
	content := `{
  "source_csv": "/tmp/a.csv",
  "generated_at_utc": "2026-02-17T00:00:00Z",
  "row_count": 2,
  "panel_count": 2,
  "by_panel_key": {
    "eff_21": {
      "id": "eff_21",
      "brand": "Brand",
      "model": "Model 500A",
      "pmax_stc_w": 500,
      "voc_v": 40.0,
      "imp_a": 12.0,
      "module_efficiency_pct": 21.0,
      "compatibility": {
        "d2_d2_max": {"label":"D2/D2 Max 11–60V/15A","status":"yes","max_series":2}
      }
    },
    "eff_24": {
      "id": "eff_24",
      "brand": "Brand",
      "model": "Model 500B",
      "pmax_stc_w": 500,
      "voc_v": 40.0,
      "imp_a": 12.0,
      "module_efficiency_pct": 24.0,
      "compatibility": {
        "d2_d2_max": {"label":"D2/D2 Max 11–60V/15A","status":"yes","max_series":2}
      }
    }
  },
  "by_device_tag": {
    "d2_d2_max": ["eff_21", "eff_24"]
  },
  "device_labels": {}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	db, err := loadSolarPanelIndex(path)
	if err != nil {
		t.Fatalf("loadSolarPanelIndex returned error: %v", err)
	}
	device := ecoflow.GeneralInfoDevice{ProductName: "DELTA 2 Max"}
	candidates := topPanelCandidatesForChannel(db, device, "low", 2)
	if len(candidates) != 2 {
		t.Fatalf("expected two candidates, got=%d", len(candidates))
	}
	if candidates[0].record.ID != "eff_24" {
		t.Fatalf("expected higher-efficiency candidate first, got=%s", candidates[0].record.ID)
	}
}
