package main

import (
	"strings"
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
				ID:            "d2m_low",
				Profile:       "d2m",
				Port:          "low",
				PanelSetup:    "EcoFlow 220W",
				PanelCount:    1,
				NominalTotalW: 220,
				Centroid:      []float64{45, 50, 33, 35, 1.2, 0.9, 0.9},
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
	if !snapshot.HasPVLowPanelNominal || snapshot.PVLowPanelNominalWatts <= 0 {
		t.Fatalf("expected low panel nominal watts")
	}
	if !snapshot.HasPVLowPanelCount || snapshot.PVLowPanelCount <= 0 {
		t.Fatalf("expected low panel count")
	}
	if snapshot.PVLowPanelStatus == "" {
		t.Fatalf("expected low panel status")
	}
}

func TestPanelRuntimeModelObserveCollectingStatus(t *testing.T) {
	t.Parallel()

	model := &panelselect.Model{
		Version:      panelselect.ModelVersion,
		FeatureNames: append([]string(nil), panelselect.FeatureNames...),
		Classes: []panelselect.Class{
			{
				ID:            "d2m_low",
				Profile:       "d2m",
				Port:          "low",
				PanelSetup:    "EcoFlow 220W",
				PanelCount:    1,
				NominalTotalW: 220,
				Centroid:      []float64{45, 50, 33, 35, 1.2, 0.9, 0.9},
			},
		},
	}
	if err := model.Validate(); err != nil {
		t.Fatalf("validate model: %v", err)
	}

	runtime := newPanelRuntimeModel(model, "DELTA 2 Max", 60, 20)
	if runtime == nil {
		t.Fatalf("expected runtime model")
	}
	snapshot := newEnergySnapshot()
	snapshot.HasInPVLow = true
	snapshot.HasSolarLVVolts = true
	snapshot.HasSolarLVAmp = true
	snapshot.HasPVLowChgState = true

	for i := 0; i < 5; i++ {
		snapshot.InPVLowWatts = 30
		snapshot.SolarLVVolts = 30
		snapshot.SolarLVAmp = 1.0
		snapshot.PVLowChgStateRaw = 1
		runtime.Observe(snapshot)
	}

	if snapshot.HasPVLowPanelPrediction {
		t.Fatalf("did not expect prediction while collecting samples")
	}
	if !strings.HasPrefix(snapshot.PVLowPanelStatus, "collecting samples") {
		t.Fatalf("expected collecting status, got=%q", snapshot.PVLowPanelStatus)
	}
}

func TestPanelRuntimeModelPreservesLastPredictionWithoutVolts(t *testing.T) {
	t.Parallel()

	model := &panelselect.Model{
		Version:      panelselect.ModelVersion,
		FeatureNames: append([]string(nil), panelselect.FeatureNames...),
		Classes: []panelselect.Class{
			{
				ID:            "d2m_low",
				Profile:       "d2m",
				Port:          "low",
				PanelSetup:    "EcoFlow 220W",
				PanelCount:    1,
				NominalTotalW: 220,
				Centroid:      []float64{45, 50, 33, 35, 1.2, 0.9, 0.9},
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

	lastSetup := snapshot.PVLowPanelSetup
	lastSamples := snapshot.PVLowPanelSamples
	lastStatus := snapshot.PVLowPanelStatus

	snapshot.HasSolarLVVolts = false
	snapshot.SolarLVVolts = 0
	snapshot.InPVLowWatts = 0
	snapshot.HasInPVLow = false

	for i := 0; i < 10; i++ {
		runtime.Observe(snapshot)
	}

	if !snapshot.HasPVLowPanelPrediction {
		t.Fatalf("expected low panel prediction to be preserved when no volts are present")
	}
	if snapshot.PVLowPanelSetup != lastSetup {
		t.Fatalf("expected setup preserved, got=%q want=%q", snapshot.PVLowPanelSetup, lastSetup)
	}
	if snapshot.PVLowPanelSamples != lastSamples {
		t.Fatalf("expected samples preserved, got=%d want=%d", snapshot.PVLowPanelSamples, lastSamples)
	}
	if snapshot.PVLowPanelStatus != lastStatus {
		t.Fatalf("expected status preserved, got=%q want=%q", snapshot.PVLowPanelStatus, lastStatus)
	}
}
