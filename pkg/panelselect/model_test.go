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
