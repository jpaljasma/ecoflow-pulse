package main

import (
	"strings"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/pkg/panelselect"
)

func TestPanelPredictionThrottleByConfidence(t *testing.T) {
	t.Parallel()

	if got := panelPredictionThrottle(panelPortResult{hasPrediction: false}); got != panelModelThrottleLow {
		t.Fatalf("no-prediction throttle mismatch: got=%d want=%d", got, panelModelThrottleLow)
	}
	if got := panelPredictionThrottle(panelPortResult{hasPrediction: true, confidence: 0.55}); got != panelModelThrottleLow {
		t.Fatalf("low-confidence throttle mismatch: got=%d want=%d", got, panelModelThrottleLow)
	}
	if got := panelPredictionThrottle(panelPortResult{hasPrediction: true, confidence: 0.62}); got != panelModelThrottleMedium {
		t.Fatalf("medium-confidence throttle mismatch: got=%d want=%d", got, panelModelThrottleMedium)
	}
	if got := panelPredictionThrottle(panelPortResult{hasPrediction: true, confidence: 0.93}); got != panelModelThrottleHigh {
		t.Fatalf("high-confidence throttle mismatch: got=%d want=%d", got, panelModelThrottleHigh)
	}
}

func TestShouldRunPanelPredictionHonorsInterval(t *testing.T) {
	t.Parallel()

	state := &panelPortRuntimeState{predictEvery: 5}
	for i := 1; i <= 12; i++ {
		got := shouldRunPanelPrediction(state)
		shouldRun := i%5 == 0
		if got != shouldRun {
			t.Fatalf("interval gate mismatch at sample=%d: got=%v want=%v", i, got, shouldRun)
		}
	}
}

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
				Centroid:      []float64{95, 100, 35, 36, 2.7, 0.9, 0.9},
			},
			{
				ID:            "d2m_low_alt",
				Profile:       "d2m",
				Port:          "low",
				PanelSetup:    "Alt Panel",
				PanelCount:    1,
				NominalTotalW: 400,
				Centroid:      []float64{30, 35, 18, 20, 1.0, 0.7, 0.7},
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

	for i := 0; i < 50; i++ {
		snapshot.InPVLowWatts = 95
		snapshot.SolarLVVolts = 35
		snapshot.SolarLVAmp = 2.7
		snapshot.PVLowChgStateRaw = 1
		runtime.Observe(snapshot)
	}

	if !snapshot.HasPVLowPanelPrediction {
		if !strings.Contains(snapshot.PVLowPanelStatus, "collecting stronger signal") {
			t.Fatalf("expected prediction or stronger-signal status, got status=%q", snapshot.PVLowPanelStatus)
		}
		return
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
				Centroid:      []float64{95, 100, 35, 36, 2.7, 0.9, 0.9},
			},
			{
				ID:            "d2m_low_alt",
				Profile:       "d2m",
				Port:          "low",
				PanelSetup:    "Alt Panel",
				PanelCount:    1,
				NominalTotalW: 400,
				Centroid:      []float64{30, 35, 18, 20, 1.0, 0.7, 0.7},
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
				Centroid:      []float64{95, 100, 35, 36, 2.7, 0.9, 0.9},
			},
			{
				ID:            "d2m_low_alt",
				Profile:       "d2m",
				Port:          "low",
				PanelSetup:    "Alt Panel",
				PanelCount:    1,
				NominalTotalW: 400,
				Centroid:      []float64{30, 35, 18, 20, 1.0, 0.7, 0.7},
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

	for i := 0; i < 50; i++ {
		snapshot.InPVLowWatts = 95
		snapshot.SolarLVVolts = 35
		snapshot.SolarLVAmp = 2.7
		snapshot.PVLowChgStateRaw = 1
		runtime.Observe(snapshot)
	}
	if !snapshot.HasPVLowPanelPrediction {
		if !strings.Contains(snapshot.PVLowPanelStatus, "collecting stronger signal") {
			t.Fatalf("expected prediction or stronger-signal status, got status=%q", snapshot.PVLowPanelStatus)
		}
		return
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

func TestStabilizePanelPredictionPreventsLowIrradianceDowngrade(t *testing.T) {
	t.Parallel()

	prev := panelPortResult{
		hasPrediction: true,
		setup:         "EcoFlow 220W Bifacial Portable",
		confidence:    0.91,
		samples:       180,
		panelCount:    1,
		nominalWatts:  220,
		status:        "high confidence (0.91, n=180)",
	}
	curr := panelPortResult{
		hasPrediction: true,
		setup:         "2x100W EcoFlow 100W Flexible Solar Panel",
		confidence:    0.77,
		samples:       200,
		panelCount:    2,
		nominalWatts:  200,
		status:        "medium confidence (0.77, n=200)",
	}
	got := stabilizePanelPrediction(prev, curr, 35)
	if got.setup != prev.setup {
		t.Fatalf("expected prior setup held, got=%q want=%q", got.setup, prev.setup)
	}
	if !strings.HasPrefix(got.status, "holding prior setup") {
		t.Fatalf("expected hold status, got=%q", got.status)
	}
}

func TestGateWeakInitialPredictionSuppressesAmbiguousLockIn(t *testing.T) {
	t.Parallel()

	curr := panelPortResult{
		hasPrediction: true,
		setup:         "EcoFlow 60W Solar Panel (EFSOLAR60)",
		confidence:    0.68,
		samples:       85,
		panelCount:    1,
		nominalWatts:  60,
		status:        "medium confidence (0.68, n=85)",
	}
	got := gateWeakInitialPrediction(panelPortResult{}, curr, 18)
	if got.hasPrediction {
		t.Fatalf("expected weak initial prediction to be suppressed")
	}
	if !strings.Contains(got.status, "low irradiance") {
		t.Fatalf("expected low irradiance status, got=%q", got.status)
	}
}

func TestGatePredictionByConfidenceAndSignalSuppressesWeak(t *testing.T) {
	t.Parallel()
	curr := panelPortResult{
		hasPrediction: true,
		setup:         "JJN Solar 400W N-Type Bifacial Solar Panel",
		confidence:    0.53,
		samples:       30,
		nominalWatts:  400,
	}
	got := gatePredictionByConfidenceAndSignal(curr, 8, 0.2)
	if got.hasPrediction {
		t.Fatalf("expected prediction suppressed on weak signal/confidence")
	}
	if !strings.Contains(got.status, "collecting stronger signal") {
		t.Fatalf("unexpected status: %q", got.status)
	}
}

func TestGatePredictionByConfidenceAndSignalAllowsMediumWithSignal(t *testing.T) {
	t.Parallel()
	curr := panelPortResult{
		hasPrediction: true,
		setup:         "JJN Solar 400W N-Type Bifacial Solar Panel",
		confidence:    0.62,
		samples:       80,
		nominalWatts:  400,
	}
	got := gatePredictionByConfidenceAndSignal(curr, 35, 0.9)
	if !got.hasPrediction {
		t.Fatalf("expected prediction allowed with medium confidence and signal")
	}
}
