package main

import (
	"testing"
)

func TestExtractModuleEfficiencyFromNotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		notes string
		want  float64
		ok    bool
	}{
		{
			name:  "leading phrase",
			notes: "Efficiency 25%. IP68.",
			want:  25,
			ok:    true,
		},
		{
			name:  "trailing phrase",
			notes: "EcoFlow rigid module; ~23% efficiency; IP68.",
			want:  23,
			ok:    true,
		},
		{
			name:  "up to phrase",
			notes: "Includes DC5521 + USB-C output; Efficiency up to 25%.",
			want:  25,
			ok:    true,
		},
		{
			name:  "no efficiency",
			notes: "No efficiency value in this note.",
			want:  0,
			ok:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := extractModuleEfficiencyFromNotes(tt.notes)
			if ok != tt.ok {
				t.Fatalf("ok mismatch: got=%t want=%t", ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if got != tt.want {
				t.Fatalf("value mismatch: got=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestDeriveModuleEfficiencyPct(t *testing.T) {
	t.Parallel()

	notesKey := normalizeColumn("Notes")
	effKey := normalizeColumn("Efficiency_pct")
	record := map[string]any{
		notesKey: "Efficiency 19.5%.",
		effKey:   float64(21.3),
	}

	got, ok := deriveModuleEfficiencyPct(record)
	if !ok {
		t.Fatalf("expected efficiency to be detected")
	}
	if got != 21.3 {
		t.Fatalf("expected structured value to win: got=%v want=21.3", got)
	}
}

func TestParseCompatibilityEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		raw         string
		wantTag     string
		wantStatus  string
		wantMin     int
		wantMax     int
		wantClipLik bool
	}{
		{
			name:       "yes",
			raw:        "D2/D2 Max 11–60V/15A: YES",
			wantTag:    "d2_d2_max",
			wantStatus: "yes",
		},
		{
			name:       "no",
			raw:        "DPU Low 30–150V/15A: NO",
			wantTag:    "dpu_low",
			wantStatus: "no",
		},
		{
			name:       "needs series",
			raw:        "DPU‑X High 80–500V/15A: needs ≥2S (max 8S)",
			wantTag:    "dpu_x_high",
			wantStatus: "needs_series",
			wantMin:    2,
			wantMax:    8,
		},
		{
			name:        "current clip likely",
			raw:         "D3 Max 11–60V/13A: YES (current clip likely)",
			wantTag:     "d3_max",
			wantStatus:  "yes",
			wantClipLik: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotTag, gotSummary := parseCompatibilityEntry(tt.raw)
			if gotTag != tt.wantTag {
				t.Fatalf("tag mismatch: got=%q want=%q", gotTag, tt.wantTag)
			}
			if gotSummary.Status != tt.wantStatus {
				t.Fatalf("status mismatch: got=%q want=%q", gotSummary.Status, tt.wantStatus)
			}
			if gotSummary.MinSeries != tt.wantMin {
				t.Fatalf("min series mismatch: got=%d want=%d", gotSummary.MinSeries, tt.wantMin)
			}
			if gotSummary.MaxSeries != tt.wantMax {
				t.Fatalf("max series mismatch: got=%d want=%d", gotSummary.MaxSeries, tt.wantMax)
			}
			if gotSummary.CurrentClipLikely != tt.wantClipLik {
				t.Fatalf("clip flag mismatch: got=%t want=%t", gotSummary.CurrentClipLikely, tt.wantClipLik)
			}
		})
	}
}

func TestBuildPanelIndex(t *testing.T) {
	t.Parallel()

	brandKey := normalizeColumn("Brand")
	modelKey := normalizeColumn("Model")
	typeKey := normalizeColumn("Type")
	pmaxKey := normalizeColumn("Pmax_STC_W")
	vocKey := normalizeColumn("Voc_V")
	vmpKey := normalizeColumn("Vmp_V")
	impKey := normalizeColumn("Imp_A")
	iscKey := normalizeColumn("Isc_A")

	data := &dataset{
		SourceCSV:      "/tmp/test.csv",
		GeneratedAtUTC: "2026-02-17T00:00:00Z",
		RowCount:       1,
		Records: []map[string]any{
			{
				"id":                    "ecoflow_220w_bifacial_portable",
				"source_row":            int64(2),
				brandKey:                "EcoFlow",
				modelKey:                "220W Bifacial Portable",
				typeKey:                 "Portable bifacial",
				pmaxKey:                 float64(220),
				vocKey:                  float64(24.3),
				vmpKey:                  float64(20.3),
				impKey:                  float64(10.8),
				iscKey:                  float64(11.2),
				"module_efficiency_pct": float64(25.0),
				"ecoflow_compatibility_entries": []string{
					"D2/D2 Max 11–60V/15A: YES",
					"DPU Low 30–150V/15A: NO",
				},
			},
		},
	}

	index := buildPanelIndex(data)
	if index.PanelCount != 1 {
		t.Fatalf("panel count mismatch: got=%d want=1", index.PanelCount)
	}

	panel, ok := index.ByPanelKey["ecoflow_220w_bifacial_portable"]
	if !ok {
		t.Fatalf("missing expected panel key")
	}
	if panel.Brand != "EcoFlow" || panel.Model != "220W Bifacial Portable" {
		t.Fatalf("unexpected panel identity: brand=%q model=%q", panel.Brand, panel.Model)
	}
	if panel.ModuleEfficiencyPct != 25.0 {
		t.Fatalf("module efficiency mismatch: got=%v want=25.0", panel.ModuleEfficiencyPct)
	}
	if len(panel.CompatibilityTags) != 2 {
		t.Fatalf("compatibility tags mismatch: got=%v", panel.CompatibilityTags)
	}
	if panel.Compatibility["d2_d2_max"].Status != "yes" {
		t.Fatalf("d2 status mismatch: got=%q", panel.Compatibility["d2_d2_max"].Status)
	}
	if panel.Compatibility["dpu_low"].Status != "no" {
		t.Fatalf("dpu low status mismatch: got=%q", panel.Compatibility["dpu_low"].Status)
	}
	if got := len(index.ByDeviceTag["d2_d2_max"]); got != 1 {
		t.Fatalf("device index mismatch for d2_d2_max: got=%d", got)
	}
	if got := len(index.ByDeviceTag["dpu_low"]); got != 1 {
		t.Fatalf("device index mismatch for dpu_low: got=%d", got)
	}
}
