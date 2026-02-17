package main

import (
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/pkg/panelselect"
)

func TestPanelRuntimeModelObserve(t *testing.T) {
	t.Parallel()

	model := &panelselect.Model{
		Version:      panelselect.ModelVersion,
		FeatureNames: append([]string(nil), panelselect.FeatureNames...),
		Classes: []panelselect.Class{
			{
				ID:         "d2m_low",
				Profile:    "d2m",
				Port:       "low",
				PanelSetup: "EcoFlow 220W",
				Centroid:   []float64{45, 50, 33, 35, 1.2, 0.9, 0.9},
			},
		},
	}
	if err := model.Validate(); err != nil {
		t.Fatalf("validate model: %v", err)
	}

	runtime := newPanelRuntimeModel(model, "DELTA 2 Max", 60, 5)
	if runtime == nil {
		t.Fatalf("expected runtime model")
	}
	snapshot := newEnergySnapshot()
	snapshot.HasInPVLow = true
	snapshot.HasSolarLVVolts = true
	snapshot.HasSolarLVAmp = true
	snapshot.HasPVLowChgState = true

	for i := 0; i < 12; i++ {
		snapshot.InPVLowWatts = 44
		snapshot.SolarLVVolts = 33
		snapshot.SolarLVAmp = 1.3
		snapshot.PVLowChgStateRaw = 1
		runtime.Observe(snapshot)
	}

	if !snapshot.HasPVLowPanelPrediction {
		t.Fatalf("expected low panel prediction")
	}
	if snapshot.PVLowPanelSetup == "" {
		t.Fatalf("expected non-empty panel setup")
	}
	if snapshot.PVLowPanelConfidence <= 0 {
		t.Fatalf("expected positive confidence, got=%.4f", snapshot.PVLowPanelConfidence)
	}
}
