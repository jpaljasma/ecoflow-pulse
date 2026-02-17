package main

import (
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/pkg/panelselect"
)

const (
	defaultPanelModelPath       = "data/solar_panels/panel_select_model.json"
	defaultPanelModelMinSamples = 20
)

type panelRuntimeModel struct {
	model      *panelselect.Model
	profile    string
	minSamples int
	low        *panelselect.Tracker
	high       *panelselect.Tracker
}

func newPanelRuntimeModel(model *panelselect.Model, productName string, window, minSamples int) *panelRuntimeModel {
	if model == nil {
		return nil
	}
	if window <= 0 {
		window = panelselect.DefaultTrackerLimit
	}
	if minSamples <= 0 {
		minSamples = defaultPanelModelMinSamples
	}
	return &panelRuntimeModel{
		model:      model,
		profile:    panelselect.NormalizeProfile(productName),
		minSamples: minSamples,
		low:        panelselect.NewTracker(window),
		high:       panelselect.NewTracker(window),
	}
}

func (p *panelRuntimeModel) Observe(snapshot *energySnapshot) {
	if p == nil || snapshot == nil || p.model == nil {
		return
	}
	_, _, lowW, hasLowW, highW, hasHighW := snapshot.effectivePVInputChannels()
	lowV := 0.0
	highV := 0.0
	lowA := 0.0
	highA := 0.0
	if snapshot.HasSolarLVVolts {
		lowV = snapshot.SolarLVVolts
	}
	if snapshot.HasSolarHVVolts {
		highV = snapshot.SolarHVVolts
	}
	if snapshot.HasSolarLVAmp {
		lowA = snapshot.SolarLVAmp
	}
	if snapshot.HasSolarHVAmp {
		highA = snapshot.SolarHVAmp
	}

	lowState := inferPanelPortState(hasLowW, lowW, snapshot.HasPVLowChgState, snapshot.PVLowChgStateRaw)
	highState := inferPanelPortState(hasHighW, highW, snapshot.HasPVHighChgState, snapshot.PVHighChgStateRaw)

	p.low.Observe(lowW, lowV, lowA, lowState)
	p.high.Observe(highW, highV, highA, highState)

	predictPort(snapshot, p.model, p.profile, "low", p.low, p.minSamples)
	predictPort(snapshot, p.model, p.profile, "high", p.high, p.minSamples)
}

func inferPanelPortState(hasWatts bool, watts float64, hasRaw bool, raw int64) string {
	if hasRaw {
		if isMPPTChargeStateActive(raw) {
			return "charging"
		}
		return "idle"
	}
	if hasWatts && watts > solarLockInputMaxWatts {
		return "charging"
	}
	return "idle"
}

func predictPort(
	snapshot *energySnapshot,
	model *panelselect.Model,
	profile string,
	port string,
	tracker *panelselect.Tracker,
	minSamples int,
) {
	if snapshot == nil || model == nil || tracker == nil {
		return
	}
	samples := tracker.SampleCount()
	if samples < minSamples {
		clearPanelPrediction(snapshot, port)
		return
	}
	features, ok := tracker.FeatureVector()
	if !ok {
		clearPanelPrediction(snapshot, port)
		return
	}
	prediction, ok := panelselect.Predict(model, profile, port, features, samples)
	if !ok || strings.TrimSpace(prediction.PanelSetup) == "" {
		clearPanelPrediction(snapshot, port)
		return
	}
	switch panelselect.NormalizePort(port) {
	case "high":
		snapshot.PVHighPanelSetup = prediction.PanelSetup
		snapshot.PVHighPanelConfidence = prediction.Confidence
		snapshot.PVHighPanelSamples = samples
		snapshot.HasPVHighPanelPrediction = true
	default:
		snapshot.PVLowPanelSetup = prediction.PanelSetup
		snapshot.PVLowPanelConfidence = prediction.Confidence
		snapshot.PVLowPanelSamples = samples
		snapshot.HasPVLowPanelPrediction = true
	}
}

func clearPanelPrediction(snapshot *energySnapshot, port string) {
	if snapshot == nil {
		return
	}
	switch panelselect.NormalizePort(port) {
	case "high":
		snapshot.PVHighPanelSetup = ""
		snapshot.PVHighPanelConfidence = 0
		snapshot.PVHighPanelSamples = 0
		snapshot.HasPVHighPanelPrediction = false
	default:
		snapshot.PVLowPanelSetup = ""
		snapshot.PVLowPanelConfidence = 0
		snapshot.PVLowPanelSamples = 0
		snapshot.HasPVLowPanelPrediction = false
	}
}
