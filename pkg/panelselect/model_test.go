package panelselect

import "testing"

func TestPredictPrefersClosestCentroid(t *testing.T) {
	t.Parallel()

	model := &Model{
		Version:      ModelVersion,
		FeatureNames: append([]string(nil), FeatureNames...),
		Classes: []Class{
			{
				ID:         "d2m_low_panel_a",
				Profile:    "d2m",
				Port:       "low",
				PanelSetup: "Panel A",
				Centroid:   []float64{30, 45, 20, 25, 1.2, 0.8, 0.7},
			},
			{
				ID:         "d2m_low_panel_b",
				Profile:    "d2m",
				Port:       "low",
				PanelSetup: "Panel B",
				Centroid:   []float64{60, 80, 35, 42, 2.3, 0.8, 0.7},
			},
		},
	}
	if err := model.Validate(); err != nil {
		t.Fatalf("validate model: %v", err)
	}

	features := []float64{58, 82, 34, 41, 2.4, 0.7, 0.7}
	pred, ok := Predict(model, "d2m", "low", features, 120)
	if !ok {
		t.Fatalf("predict returned !ok")
	}
	if pred.ClassID != "d2m_low_panel_b" {
		t.Fatalf("class mismatch: got=%s want=d2m_low_panel_b", pred.ClassID)
	}
	if pred.Confidence <= 0 {
		t.Fatalf("confidence should be > 0: got=%.4f", pred.Confidence)
	}
}

func TestPredictPrefersRealClassOverCloseSynthetic(t *testing.T) {
	t.Parallel()

	model := &Model{
		Version:      ModelVersion,
		FeatureNames: append([]string(nil), FeatureNames...),
		Classes: []Class{
			{
				ID:          "real",
				Profile:     "d2m",
				Port:        "low",
				PanelSetup:  "Real Setup",
				Centroid:    []float64{60, 80, 34, 40, 2.0, 0.8, 0.7},
				SampleCount: 120,
				DeviceSNs:   []string{"SN1"},
			},
			{
				ID:          "synthetic",
				Profile:     "d2m",
				Port:        "low",
				PanelSetup:  "Synthetic Setup",
				Centroid:    []float64{59, 79, 34, 40, 2.0, 0.8, 0.7},
				SampleCount: 1,
				Synthetic:   true,
			},
		},
	}
	if err := model.Validate(); err != nil {
		t.Fatalf("validate model: %v", err)
	}

	features := []float64{59, 79, 34, 40, 2.0, 0.8, 0.7}
	pred, ok := Predict(model, "d2m", "low", features, 120)
	if !ok {
		t.Fatalf("predict returned !ok")
	}
	if pred.ClassID != "real" {
		t.Fatalf("class mismatch: got=%s want=real", pred.ClassID)
	}
}

func TestPredictUsesSyntheticWhenNoRealClasses(t *testing.T) {
	t.Parallel()

	model := &Model{
		Version:      ModelVersion,
		FeatureNames: append([]string(nil), FeatureNames...),
		Classes: []Class{
			{
				ID:          "synthetic_only",
				Profile:     "dpu",
				Port:        "high",
				PanelSetup:  "Synthetic Only",
				Centroid:    []float64{40, 60, 100, 120, 0.5, 0.5, 0.5},
				SampleCount: 1,
				Synthetic:   true,
			},
		},
	}
	if err := model.Validate(); err != nil {
		t.Fatalf("validate model: %v", err)
	}
	features := []float64{41, 61, 101, 121, 0.5, 0.5, 0.5}
	pred, ok := Predict(model, "dpu", "high", features, 100)
	if !ok {
		t.Fatalf("predict returned !ok")
	}
	if pred.ClassID != "synthetic_only" {
		t.Fatalf("class mismatch: got=%s want=synthetic_only", pred.ClassID)
	}
}
