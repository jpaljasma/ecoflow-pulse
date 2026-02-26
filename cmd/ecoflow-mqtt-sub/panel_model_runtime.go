package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/pkg/panelselect"
)

const (
	defaultPanelModelPath       = "data/solar_panels/panel_select_model.json"
	defaultPanelModelMinSamples = 20
	panelModelThrottleLow       = 3
	panelModelThrottleMedium    = 5
	panelModelThrottleHigh      = 10
	panelSignalMinWatts         = 25.0
	panelSignalMinAmps          = 0.6
	panelMinMediumConfidence    = 0.60
)

type panelRuntimeModel struct {
	model      *panelselect.Model
	profile    string
	minSamples int
	low        *panelselect.Tracker
	high       *panelselect.Tracker
	lowState   panelPortRuntimeState
	highState  panelPortRuntimeState
}

type panelPortRuntimeState struct {
	sampleCount  int64
	predictEvery int
	lastStable   panelPortResult
}

type panelObservation struct {
	lowWatts     float64
	hasLowWatts  bool
	highWatts    float64
	hasHighWatts bool
	lowVolts     float64
	hasLowVolts  bool
	highVolts    float64
	hasHighVolts bool
	lowAmps      float64
	hasLowAmps   bool
	highAmps     float64
	hasHighAmps  bool
	lowState     string
	highState    string
}

type panelPortResult struct {
	applied       bool
	hasPrediction bool
	setup         string
	confidence    float64
	samples       int
	panelCount    int
	nominalWatts  float64
	status        string
}

type panelObservationResult struct {
	low  panelPortResult
	high panelPortResult
}

func (r panelObservationResult) HasUpdates() bool {
	return r.low.applied || r.high.applied
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
		lowState: panelPortRuntimeState{
			predictEvery: 1,
		},
		highState: panelPortRuntimeState{
			predictEvery: 1,
		},
	}
}

func (p *panelRuntimeModel) Observe(snapshot *energySnapshot) {
	if p == nil || snapshot == nil {
		return
	}
	sample := buildPanelObservation(snapshot)
	result := p.ObserveSample(sample)
	applyPanelObservationResult(snapshot, result)
}

func buildPanelObservation(snapshot *energySnapshot) panelObservation {
	observation := panelObservation{}
	if snapshot == nil {
		return observation
	}
	_, _, lowW, hasLowW, highW, hasHighW := snapshot.effectivePVInputChannels()
	observation.lowWatts = lowW
	observation.hasLowWatts = hasLowW
	observation.highWatts = highW
	observation.hasHighWatts = hasHighW
	if snapshot.HasSolarLVVolts {
		observation.lowVolts = snapshot.SolarLVVolts
		observation.hasLowVolts = true
	}
	if snapshot.HasSolarHVVolts {
		observation.highVolts = snapshot.SolarHVVolts
		observation.hasHighVolts = true
	}
	if snapshot.HasSolarLVAmp {
		observation.lowAmps = snapshot.SolarLVAmp
		observation.hasLowAmps = true
	}
	if snapshot.HasSolarHVAmp {
		observation.highAmps = snapshot.SolarHVAmp
		observation.hasHighAmps = true
	}
	observation.lowState = inferPanelPortState(hasLowW, lowW, snapshot.HasPVLowChgState, snapshot.PVLowChgStateRaw)
	observation.highState = inferPanelPortState(hasHighW, highW, snapshot.HasPVHighChgState, snapshot.PVHighChgStateRaw)
	return observation
}

func (p *panelRuntimeModel) ObserveSample(sample panelObservation) panelObservationResult {
	if p == nil || p.model == nil {
		return panelObservationResult{}
	}
	result := panelObservationResult{}
	if shouldRunPanelModelForPort(sample.hasLowVolts, sample.lowVolts) {
		p.low.Observe(sample.lowWatts, sample.lowVolts, sample.lowAmps, sample.lowState)
		if shouldRunPanelPrediction(&p.lowState) {
			result.low = predictPortResult(p.model, p.profile, "low", p.low, p.minSamples)
			result.low = stabilizePanelPrediction(p.lowState.lastStable, result.low, sample.lowWatts)
			result.low = gateWeakInitialPrediction(p.lowState.lastStable, result.low, sample.lowWatts)
			result.low = gatePredictionByConfidenceAndSignal(result.low, sample.lowWatts, sample.lowAmps)
			if result.low.hasPrediction {
				p.lowState.lastStable = result.low
			}
			result.low.applied = true
			p.lowState.predictEvery = panelPredictionThrottle(result.low)
		}
	}
	if shouldRunPanelModelForPort(sample.hasHighVolts, sample.highVolts) {
		p.high.Observe(sample.highWatts, sample.highVolts, sample.highAmps, sample.highState)
		if shouldRunPanelPrediction(&p.highState) {
			result.high = predictPortResult(p.model, p.profile, "high", p.high, p.minSamples)
			result.high = stabilizePanelPrediction(p.highState.lastStable, result.high, sample.highWatts)
			result.high = gateWeakInitialPrediction(p.highState.lastStable, result.high, sample.highWatts)
			result.high = gatePredictionByConfidenceAndSignal(result.high, sample.highWatts, sample.highAmps)
			if result.high.hasPrediction {
				p.highState.lastStable = result.high
			}
			result.high.applied = true
			p.highState.predictEvery = panelPredictionThrottle(result.high)
		}
	}
	return result
}

func gateWeakInitialPrediction(prev panelPortResult, curr panelPortResult, observedWatts float64) panelPortResult {
	// No-op unless we're setting the first stable setup for this runtime.
	if prev.hasPrediction || !curr.hasPrediction {
		return curr
	}
	if curr.nominalWatts <= 0 {
		return curr
	}
	// Low irradiance is especially ambiguous for tiny panel classes; avoid locking these too early.
	if curr.nominalWatts > 150 {
		return curr
	}
	weakIrradiance := observedWatts > 0 && observedWatts < math.Max(80, curr.nominalWatts*0.55)
	weakConfidence := curr.confidence < 0.80
	if weakIrradiance && weakConfidence {
		return panelPortResult{
			applied:       curr.applied,
			hasPrediction: false,
			samples:       curr.samples,
			status:        "low irradiance / collecting stronger signal",
		}
	}
	return curr
}

func gatePredictionByConfidenceAndSignal(curr panelPortResult, observedWatts, observedAmps float64) panelPortResult {
	if !curr.hasPrediction {
		return curr
	}
	hasSignal := observedWatts >= panelSignalMinWatts || observedAmps >= panelSignalMinAmps
	if curr.confidence >= panelMinMediumConfidence && hasSignal {
		return curr
	}
	return panelPortResult{
		applied:       curr.applied,
		hasPrediction: false,
		samples:       curr.samples,
		status:        fmt.Sprintf("collecting stronger signal (c=%.2f, n=%d)", curr.confidence, curr.samples),
	}
}

func stabilizePanelPrediction(prev panelPortResult, curr panelPortResult, observedWatts float64) panelPortResult {
	if !prev.hasPrediction || !curr.hasPrediction {
		return curr
	}
	// Prevent low-irradiance downgrades (e.g. 220W/500W setups collapsing to 100W/60W classes).
	downgradeRatio := 1.0
	if curr.nominalWatts > 0 && prev.nominalWatts > 0 {
		downgradeRatio = curr.nominalWatts / prev.nominalWatts
	}
	perPanelDowngrade := false
	_, prevPerPanelW, prevOK := parsePanelSetupCountAndWatts(prev.setup)
	_, currPerPanelW, currOK := parsePanelSetupCountAndWatts(curr.setup)
	if prevOK && currOK && prevPerPanelW > 0 && currPerPanelW > 0 && currPerPanelW < prevPerPanelW*0.75 {
		perPanelDowngrade = true
	}
	lowIrradiance := observedWatts > 0 && observedWatts < (prev.nominalWatts*0.45)
	weakEvidence := curr.confidence < 0.90
	if (downgradeRatio <= 0.75 || perPanelDowngrade) && (lowIrradiance || weakEvidence) {
		held := prev
		held.samples = curr.samples
		if strings.TrimSpace(curr.status) != "" {
			held.status = "holding prior setup (" + curr.status + ")"
		}
		return held
	}
	return curr
}

func shouldRunPanelPrediction(state *panelPortRuntimeState) bool {
	if state == nil {
		return true
	}
	state.sampleCount++
	every := state.predictEvery
	if every <= 1 {
		return true
	}
	return state.sampleCount%int64(every) == 0
}

func panelPredictionThrottle(result panelPortResult) int {
	if !result.hasPrediction {
		return panelModelThrottleLow
	}
	switch {
	case result.confidence >= 0.80:
		return panelModelThrottleHigh
	case result.confidence >= 0.60:
		return panelModelThrottleMedium
	default:
		return panelModelThrottleLow
	}
}

func shouldRunPanelModelForPort(hasVolts bool, volts float64) bool {
	if !hasVolts {
		return false
	}
	return volts > 0.05
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

//nolint:unused // reserved for future per-port direct prediction API.
func predictPort(
	snapshot *energySnapshot,
	model *panelselect.Model,
	profile string,
	port string,
	tracker *panelselect.Tracker,
	minSamples int,
) {
	if snapshot == nil {
		return
	}
	result := predictPortResult(model, profile, port, tracker, minSamples)
	applyPanelPortResult(snapshot, port, result)
}

func predictPortResult(
	model *panelselect.Model,
	profile string,
	port string,
	tracker *panelselect.Tracker,
	minSamples int,
) panelPortResult {
	if model == nil || tracker == nil {
		return panelPortResult{}
	}
	samples := tracker.SampleCount()
	if samples < minSamples {
		return panelPortResult{
			status:  fmt.Sprintf("collecting samples %d/%d", samples, minSamples),
			samples: samples,
		}
	}
	features, ok := tracker.FeatureVector()
	if !ok {
		return panelPortResult{
			status:  fmt.Sprintf("analyzing signal (n=%d)", samples),
			samples: samples,
		}
	}
	prediction, ok := panelselect.Predict(model, profile, port, features, samples)
	if !ok || strings.TrimSpace(prediction.PanelSetup) == "" {
		return panelPortResult{
			status:  fmt.Sprintf("low confidence / unknown setup (n=%d)", samples),
			samples: samples,
		}
	}
	return panelPortResult{
		hasPrediction: true,
		setup:         prediction.PanelSetup,
		confidence:    prediction.Confidence,
		samples:       samples,
		panelCount:    prediction.PanelCount,
		nominalWatts:  prediction.NominalTotal,
		status:        panelPredictionConfidenceStatus(prediction.Confidence, samples),
	}
}

func applyPanelObservationResult(snapshot *energySnapshot, result panelObservationResult) {
	if snapshot == nil {
		return
	}
	if result.low.applied {
		applyPanelPortResult(snapshot, "low", result.low)
	}
	if result.high.applied {
		applyPanelPortResult(snapshot, "high", result.high)
	}
}

func applyPanelPortResult(snapshot *energySnapshot, port string, result panelPortResult) {
	if snapshot == nil {
		return
	}
	if !result.hasPrediction {
		clearPanelPrediction(snapshot, port)
		setPanelPredictionStatus(snapshot, port, result.status)
		return
	}
	switch panelselect.NormalizePort(port) {
	case "high":
		snapshot.PVHighPanelSetup = result.setup
		snapshot.PVHighPanelConfidence = result.confidence
		snapshot.PVHighPanelSamples = result.samples
		snapshot.PVHighPanelCount = result.panelCount
		snapshot.HasPVHighPanelCount = result.panelCount > 0
		snapshot.PVHighPanelNominalWatts = result.nominalWatts
		snapshot.HasPVHighPanelNominal = result.nominalWatts > 0
		snapshot.HasPVHighPanelPrediction = true
		setPanelPredictionStatus(snapshot, port, result.status)
	default:
		snapshot.PVLowPanelSetup = result.setup
		snapshot.PVLowPanelConfidence = result.confidence
		snapshot.PVLowPanelSamples = result.samples
		snapshot.PVLowPanelCount = result.panelCount
		snapshot.HasPVLowPanelCount = result.panelCount > 0
		snapshot.PVLowPanelNominalWatts = result.nominalWatts
		snapshot.HasPVLowPanelNominal = result.nominalWatts > 0
		snapshot.HasPVLowPanelPrediction = true
		setPanelPredictionStatus(snapshot, port, result.status)
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
		snapshot.PVHighPanelCount = 0
		snapshot.HasPVHighPanelCount = false
		snapshot.PVHighPanelNominalWatts = 0
		snapshot.HasPVHighPanelNominal = false
		snapshot.HasPVHighPanelPrediction = false
	default:
		snapshot.PVLowPanelSetup = ""
		snapshot.PVLowPanelConfidence = 0
		snapshot.PVLowPanelSamples = 0
		snapshot.PVLowPanelCount = 0
		snapshot.HasPVLowPanelCount = false
		snapshot.PVLowPanelNominalWatts = 0
		snapshot.HasPVLowPanelNominal = false
		snapshot.HasPVLowPanelPrediction = false
	}
}

func setPanelPredictionStatus(snapshot *energySnapshot, port string, status string) {
	if snapshot == nil {
		return
	}
	status = strings.TrimSpace(status)
	switch panelselect.NormalizePort(port) {
	case "high":
		snapshot.PVHighPanelStatus = status
	default:
		snapshot.PVLowPanelStatus = status
	}
}

func panelPredictionConfidenceStatus(confidence float64, samples int) string {
	tier := "low confidence"
	switch {
	case confidence >= 0.80:
		tier = "high confidence"
	case confidence >= 0.60:
		tier = "medium confidence"
	}
	return fmt.Sprintf("%s (%.2f, n=%d)", tier, confidence, samples)
}
