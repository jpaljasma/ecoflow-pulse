package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

func TestEnergySnapshotETAEstimatesRow(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 50
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5
	snapshot.HasBatteryIn = true
	snapshot.BatteryInWatts = 200
	snapshot.HasBatteryOut = true
	snapshot.BatteryOutWatts = 0
	snapshot.HasWattsIn = true
	snapshot.WattsIn = 320
	snapshot.HasWattsOut = true
	snapshot.WattsOut = 120

	derived := snapshot.derived()
	if derived.SystemStateValue != "charging" {
		t.Fatalf("system state mismatch: got=%s want=charging", derived.SystemStateValue)
	}
	if derived.EstimateChargeValue != "540min (~9h 0m)" {
		t.Fatalf("charge eta mismatch: got=%s want=%s", derived.EstimateChargeValue, "540min (~9h 0m)")
	}
	if derived.EstimateActiveValue != "540min (~9h 0m)" {
		t.Fatalf("active eta mismatch: got=%s want=%s", derived.EstimateActiveValue, "540min (~9h 0m)")
	}
	if derived.EstimatePowerValue != "power: chg@200.0W" {
		t.Fatalf("estimate power mismatch: got=%s want=%s", derived.EstimatePowerValue, "power: chg@200.0W")
	}
	if derived.EstimateConfidenceValue != "0.96 (high)" {
		t.Fatalf("estimate confidence mismatch: got=%s want=%s", derived.EstimateConfidenceValue, "0.96 (high)")
	}

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	for _, expected := range []string{
		"MPPT",
		"Generic",
		"New (",
		"Δ ETA vs Unit",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("dashboard missing %q in estimates row; output=%q", expected, output)
		}
	}
}

func TestEstimateBatteryETAsML(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 50
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5

	history := newMinuteTelemetryHistory(16)
	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)
	for i := 0; i < 6; i++ {
		snapshot.HasInPV = true
		snapshot.InPVWatts = 200 + float64(i)
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 45 + float64(i%2)
		snapshot.HasOutDC = true
		snapshot.OutDCWatts = 8
		history.AddSample(base.Add(time.Duration(i)*time.Minute), snapshot)
	}

	estimates := estimateBatteryETAsML(snapshot, history, systemStateCharging)
	if estimates.ChargeValue == "n/a" {
		t.Fatalf("ml charge eta should be available, got=%s", estimates.ChargeValue)
	}
	if !strings.Contains(estimates.PowerValue, "ewma+trend") {
		t.Fatalf("ml power label mismatch: got=%s", estimates.PowerValue)
	}
	if estimates.ConfidenceValue == "n/a" {
		t.Fatalf("ml confidence should be available, got=%s", estimates.ConfidenceValue)
	}
}

func TestEstimateBatteryETAsMLProfiledDetectsD2MProfile(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasXT150 = true
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 50
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5
	snapshot.mlFastHistory = newPowerTelemetryHistory(10*time.Second, 180)

	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)
	for i := 0; i < 16; i++ {
		snapshot.HasInPV = true
		snapshot.InPVWatts = 190 + float64(i%3)
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 40 + float64(i%2)
		snapshot.HasOutDC = true
		snapshot.OutDCWatts = 8
		snapshot.mlFastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), snapshot)
	}

	estimates, profile := estimateBatteryETAsMLProfiled(snapshot, nil, systemStateCharging)
	if profile != mlEstimateProfileD2M {
		t.Fatalf("expected d2m profile, got=%s", profile)
	}
	if !strings.Contains(estimates.PowerValue, "profile:d2m") {
		t.Fatalf("expected d2m profile marker in power value, got=%s", estimates.PowerValue)
	}
}

func TestEstimateBatteryETAsMLProfiledDetectsDPUProfile(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 12288
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 40
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5
	snapshot.mlFastHistory = newPowerTelemetryHistory(10*time.Second, 180)
	pack := snapshot.ensurePack(1)
	pack.SOC = 40
	pack.HasSOC = true

	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)
	for i := 0; i < 18; i++ {
		snapshot.HasInPV = true
		snapshot.InPVWatts = 260 + float64(i%4)
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 120 + float64(i%3)
		snapshot.HasOutDC = true
		snapshot.OutDCWatts = 12
		snapshot.mlFastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), snapshot)
	}

	estimates, profile := estimateBatteryETAsMLProfiled(snapshot, nil, systemStateCharging)
	if profile != mlEstimateProfileDPU {
		t.Fatalf("expected dpu profile, got=%s", profile)
	}
	if !strings.Contains(estimates.PowerValue, "profile:dpu") {
		t.Fatalf("expected dpu profile marker in power value, got=%s", estimates.PowerValue)
	}
}

func TestEstimateBatteryETAsMLProfiledConvergesHighOnStableSignal(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasXT150 = true
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4096
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 46
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5
	snapshot.mlFastHistory = newPowerTelemetryHistory(10*time.Second, 180)

	base := time.Date(2026, time.February, 15, 11, 0, 0, 0, time.Local)
	for i := 0; i < 8; i++ {
		snapshot.HasInPV = true
		snapshot.InPVWatts = 190 + float64(i%2)
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 33 + float64(i%2)
		snapshot.HasOutDC = true
		snapshot.OutDCWatts = 8
		snapshot.mlFastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), snapshot)
	}

	estimates, profile := estimateBatteryETAsMLProfiled(snapshot, nil, systemStateCharging)
	if profile != mlEstimateProfileD2M {
		t.Fatalf("expected d2m profile, got=%s", profile)
	}
	score, ok := parseConfidenceScore(estimates.ConfidenceValue)
	if !ok {
		t.Fatalf("expected confidence score, got=%q", estimates.ConfidenceValue)
	}
	if score < 0.90 {
		t.Fatalf("expected profiled ML confidence >= 0.90 on early stable signal, got=%s", estimates.ConfidenceValue)
	}
}

func TestEstimateBatteryETAsMLConvergesFasterOnStableSignal(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 50
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5

	history := newMinuteTelemetryHistory(16)
	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)
	for i := 0; i < 4; i++ {
		snapshot.HasInPV = true
		snapshot.InPVWatts = 185 + float64(i%2)
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 42
		snapshot.HasOutDC = true
		snapshot.OutDCWatts = 8
		history.AddSample(base.Add(time.Duration(i)*time.Minute), snapshot)
	}

	estimates := estimateBatteryETAsML(snapshot, history, systemStateCharging)
	if estimates.ConfidenceValue == "n/a" {
		t.Fatalf("ml confidence should be available, got=%s", estimates.ConfidenceValue)
	}
	parts := strings.Fields(estimates.ConfidenceValue)
	if len(parts) == 0 {
		t.Fatalf("unexpected ml confidence format: %q", estimates.ConfidenceValue)
	}
	score, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		t.Fatalf("parse ml confidence score: %v (value=%q)", err, estimates.ConfidenceValue)
	}
	if score < 0.90 {
		t.Fatalf("expected faster ML convergence confidence >= 0.90 on stable signal, got=%s", estimates.ConfidenceValue)
	}
}

func TestEstimateBatteryETAsMLStabilizesHighConfidenceOnIdleAfterLoadDrop(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 50
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5
	snapshot.mlFastHistory = newPowerTelemetryHistory(10*time.Second, 180)

	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)
	for i := 0; i < 18; i++ {
		snapshot.HasInPV = false
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 138
		snapshot.HasOutDC = false
		snapshot.mlFastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), snapshot)
	}
	for i := 18; i < 42; i++ {
		snapshot.HasInPV = false
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 0
		snapshot.HasOutDC = false
		snapshot.mlFastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), snapshot)
	}

	estimates := estimateBatteryETAsML(snapshot, newMinuteTelemetryHistory(16), systemStateIdle)
	parts := strings.Fields(estimates.ConfidenceValue)
	if len(parts) == 0 {
		t.Fatalf("unexpected confidence value format: %q", estimates.ConfidenceValue)
	}
	score, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		t.Fatalf("parse confidence score: %v (value=%q)", err, estimates.ConfidenceValue)
	}
	if score < 0.80 {
		t.Fatalf("expected high confidence on stable idle after load drop, got=%s", estimates.ConfidenceValue)
	}
	if !strings.Contains(estimates.ConfidenceValue, "(high)") {
		t.Fatalf("expected high confidence tier on stable idle, got=%s", estimates.ConfidenceValue)
	}
}

func TestEstimateBatteryETAsMLKeepsActiveDuringUnknownStateTransitions(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 60
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5

	history := newMinuteTelemetryHistory(16)
	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)
	for i := 0; i < 8; i++ {
		snapshot.HasInPV = false
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 148
		snapshot.HasOutDC = true
		snapshot.OutDCWatts = 12
		history.AddSample(base.Add(time.Duration(i)*time.Minute), snapshot)
	}

	estimates := estimateBatteryETAsML(snapshot, history, systemStateUnknown)
	if estimates.DischargeValue == "n/a" {
		t.Fatalf("expected discharge ETA to be available during transition, got=%s", estimates.DischargeValue)
	}
	if estimates.ActiveValue == "n/a" {
		t.Fatalf("expected active ETA to keep dominant direction during transition, got=%s", estimates.ActiveValue)
	}
	if estimates.ActiveValue != estimates.DischargeValue {
		t.Fatalf("active ETA should follow dominant discharge direction during unknown state: active=%s discharge=%s", estimates.ActiveValue, estimates.DischargeValue)
	}
}

func TestResolveMLScoringStateDampsReportedStateFlapping(t *testing.T) {
	if got := resolveMLScoringState(systemStateCharging, false, true, -36, -34); got != systemStateDischarging {
		t.Fatalf("expected discharging override on opposite modeled direction, got=%s", got)
	}
	if got := resolveMLScoringState(systemStateIdle, true, false, 42, 35); got != systemStateCharging {
		t.Fatalf("expected charging override from idle when modeled direction is clear, got=%s", got)
	}
	if got := resolveMLScoringState(systemStateDischarging, false, false, 1.5, 2.0); got != systemStateIdle {
		t.Fatalf("expected idle override near zero after discharge, got=%s", got)
	}
}

func TestEstimateBatteryETAsMLPrefersFastTenSecondBuckets(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 60
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5

	minuteHistory := newMinuteTelemetryHistory(16)
	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)
	for i := 0; i < 6; i++ {
		snapshot.HasInPV = false
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 70
		snapshot.HasOutDC = false
		minuteHistory.AddSample(base.Add(time.Duration(i)*time.Minute), snapshot)
	}

	fastHistory := newPowerTelemetryHistory(10*time.Second, 180)
	snapshot.mlFastHistory = fastHistory
	for i := 0; i < 12; i++ {
		snapshot.HasInPV = false
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 140
		snapshot.HasOutDC = false
		fastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), snapshot)
	}

	estimates := estimateBatteryETAsML(snapshot, minuteHistory, systemStateDischarging)
	if !strings.Contains(estimates.PowerValue, "dsg@140.0W") {
		t.Fatalf("expected ML estimator to prefer fast 10s buckets, got power=%s", estimates.PowerValue)
	}
}

func TestAdaptMLPredictionSamplesShrinksOnStepChange(t *testing.T) {
	samples := make([]float64, 0, 80)
	for i := 0; i < 74; i++ {
		samples = append(samples, 42)
	}
	for i := 0; i < 6; i++ {
		samples = append(samples, -88)
	}

	adapted := adaptMLPredictionSamples(samples)
	if len(adapted) >= len(samples) {
		t.Fatalf("expected adaptive window to shrink on step change, got len=%d original=%d", len(adapted), len(samples))
	}
	if len(adapted) > 24 {
		t.Fatalf("expected fast window (<=24), got len=%d", len(adapted))
	}
	if adapted[len(adapted)-1] != samples[len(samples)-1] {
		t.Fatalf("expected latest sample to be preserved, got=%f want=%f", adapted[len(adapted)-1], samples[len(samples)-1])
	}
}

func TestEstimateBatteryETAsMLRespondsToAbruptRamp(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 6000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 55
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5

	history := newMinuteTelemetryHistory(16)
	fastHistory := newPowerTelemetryHistory(10*time.Second, 180)
	snapshot.mlFastHistory = fastHistory
	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)

	for i := 0; i < 30; i++ {
		snapshot.HasInPV = false
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 40
		snapshot.HasOutDC = false
		fastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), snapshot)
	}
	for i := 30; i < 36; i++ {
		snapshot.HasInPV = false
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 150
		snapshot.HasOutDC = false
		fastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), snapshot)
	}

	estimates := estimateBatteryETAsML(snapshot, history, systemStateDischarging)
	idx := strings.Index(estimates.PowerValue, "dsg@")
	if idx < 0 {
		t.Fatalf("expected discharge power in ML output, got=%s", estimates.PowerValue)
	}
	start := idx + len("dsg@")
	end := strings.Index(estimates.PowerValue[start:], "W")
	if end <= 0 {
		t.Fatalf("failed to parse ML power value: %s", estimates.PowerValue)
	}
	watts, err := strconv.ParseFloat(estimates.PowerValue[start:start+end], 64)
	if err != nil {
		t.Fatalf("parse ML watts: %v (value=%q)", err, estimates.PowerValue)
	}
	if watts < 95 {
		t.Fatalf("expected faster convergence toward ramped discharge (>95W), got=%fW (%s)", watts, estimates.PowerValue)
	}
}

func TestEstimateBatteryETAsMLKeepsConfidenceOnRampWithStaleDirectionalRemain(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 6000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 55
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5
	snapshot.applyDischargeRemain(420)

	history := newMinuteTelemetryHistory(16)
	fastHistory := newPowerTelemetryHistory(10*time.Second, 180)
	snapshot.mlFastHistory = fastHistory
	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)

	for i := 0; i < 30; i++ {
		snapshot.HasInPV = false
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 10
		snapshot.HasOutDC = false
		fastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), snapshot)
	}
	for i := 30; i < 40; i++ {
		snapshot.HasInPV = false
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 151
		snapshot.HasOutDC = false
		fastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), snapshot)
	}

	estimates := estimateBatteryETAsML(snapshot, history, systemStateDischarging)
	parts := strings.Fields(estimates.ConfidenceValue)
	if len(parts) == 0 {
		t.Fatalf("invalid confidence format: %q", estimates.ConfidenceValue)
	}
	score, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		t.Fatalf("parse confidence score: %v (value=%q)", err, estimates.ConfidenceValue)
	}
	if score < 0.68 {
		t.Fatalf("expected confidence to remain at least medium on strong discharge ramp, got=%s", estimates.ConfidenceValue)
	}
	if !strings.Contains(estimates.ConfidenceValue, "(medium)") && !strings.Contains(estimates.ConfidenceValue, "(high)") {
		t.Fatalf("expected medium/high confidence tier on strong discharge ramp, got=%s", estimates.ConfidenceValue)
	}
}

func TestEstimateBatteryETAsMLCorrectsAgainstDeviceRemain(t *testing.T) {
	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)
	history := newMinuteTelemetryHistory(16)
	fastHistory := newPowerTelemetryHistory(10*time.Second, 180)

	buildSnapshot := func(withDeviceRemain bool) *energySnapshot {
		snapshot := newEnergySnapshot()
		snapshot.HasFullEnergy = true
		snapshot.FullEnergyWh = 8000
		snapshot.HasDeviceSOC = true
		snapshot.DeviceSOC = 55
		snapshot.HasMaxChargeSOC = true
		snapshot.MaxChargeSOC = 95
		snapshot.HasMinDischarge = true
		snapshot.MinDischargeSOC = 5
		if withDeviceRemain {
			snapshot.applyDischargeRemain(420)
		}
		snapshot.mlFastHistory = fastHistory
		return snapshot
	}

	seed := buildSnapshot(false)
	for i := 0; i < 36; i++ {
		seed.HasInPV = false
		seed.HasInAC = false
		seed.HasOutAC = true
		seed.OutACWatts = 85
		seed.HasOutDC = false
		fastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), seed)
	}

	parseMinutes := func(value string) float64 {
		idx := strings.Index(value, "min")
		if idx <= 0 {
			t.Fatalf("invalid ETA format: %q", value)
		}
		minutes, err := strconv.ParseFloat(strings.TrimSpace(value[:idx]), 64)
		if err != nil {
			t.Fatalf("parse ETA minutes: %v (value=%q)", err, value)
		}
		return minutes
	}
	parseConfidence := func(value string) float64 {
		parts := strings.Fields(value)
		if len(parts) == 0 {
			t.Fatalf("invalid confidence format: %q", value)
		}
		score, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			t.Fatalf("parse confidence score: %v (value=%q)", err, value)
		}
		return score
	}

	noDeviceRemain := estimateBatteryETAsML(buildSnapshot(false), history, systemStateDischarging)
	withDeviceRemain := estimateBatteryETAsML(buildSnapshot(true), history, systemStateDischarging)

	minNoDevice := parseMinutes(noDeviceRemain.ActiveValue)
	minWithDevice := parseMinutes(withDeviceRemain.ActiveValue)
	if minWithDevice >= minNoDevice*0.8 {
		t.Fatalf("expected ML ETA to move materially toward device remain; without=%s with=%s", noDeviceRemain.ActiveValue, withDeviceRemain.ActiveValue)
	}

	confNoDevice := parseConfidence(noDeviceRemain.ConfidenceValue)
	confWithDevice := parseConfidence(withDeviceRemain.ConfidenceValue)
	if confWithDevice >= confNoDevice {
		t.Fatalf("expected confidence penalty on large ML/device divergence; without=%s with=%s", noDeviceRemain.ConfidenceValue, withDeviceRemain.ConfidenceValue)
	}
}

func TestEstimateBatteryETAsMLTreatsGlobalRemainAsLowTrust(t *testing.T) {
	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)
	history := newMinuteTelemetryHistory(16)
	fastHistory := newPowerTelemetryHistory(10*time.Second, 180)

	buildSnapshot := func(withGlobalRemain bool) *energySnapshot {
		snapshot := newEnergySnapshot()
		snapshot.HasFullEnergy = true
		snapshot.FullEnergyWh = 8000
		snapshot.HasDeviceSOC = true
		snapshot.DeviceSOC = 55
		snapshot.HasMaxChargeSOC = true
		snapshot.MaxChargeSOC = 95
		snapshot.HasMinDischarge = true
		snapshot.MinDischargeSOC = 5
		if withGlobalRemain {
			snapshot.applyGlobalRemain(420, "pdStatus")
		}
		snapshot.mlFastHistory = fastHistory
		return snapshot
	}

	seed := buildSnapshot(false)
	for i := 0; i < 36; i++ {
		seed.HasInPV = false
		seed.HasInAC = false
		seed.HasOutAC = true
		seed.OutACWatts = 85
		seed.HasOutDC = false
		fastHistory.AddSample(base.Add(time.Duration(i)*10*time.Second), seed)
	}

	parseMinutes := func(value string) float64 {
		idx := strings.Index(value, "min")
		if idx <= 0 {
			t.Fatalf("invalid ETA format: %q", value)
		}
		minutes, err := strconv.ParseFloat(strings.TrimSpace(value[:idx]), 64)
		if err != nil {
			t.Fatalf("parse ETA minutes: %v (value=%q)", err, value)
		}
		return minutes
	}
	parseConfidence := func(value string) float64 {
		parts := strings.Fields(value)
		if len(parts) == 0 {
			t.Fatalf("invalid confidence format: %q", value)
		}
		score, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			t.Fatalf("parse confidence score: %v (value=%q)", err, value)
		}
		return score
	}

	noGlobalRemain := estimateBatteryETAsML(buildSnapshot(false), history, systemStateDischarging)
	withGlobalRemain := estimateBatteryETAsML(buildSnapshot(true), history, systemStateDischarging)

	minNoGlobal := parseMinutes(noGlobalRemain.ActiveValue)
	minWithGlobal := parseMinutes(withGlobalRemain.ActiveValue)
	if minWithGlobal < minNoGlobal*0.85 || minWithGlobal > minNoGlobal*1.15 {
		t.Fatalf("global remain should not strongly pull ML ETA; without=%s with=%s", noGlobalRemain.ActiveValue, withGlobalRemain.ActiveValue)
	}

	confNoGlobal := parseConfidence(noGlobalRemain.ConfidenceValue)
	confWithGlobal := parseConfidence(withGlobalRemain.ConfidenceValue)
	if confWithGlobal < 0.85 {
		t.Fatalf("global remain should keep confidence high on stable signal; got=%s", withGlobalRemain.ConfidenceValue)
	}
	if confWithGlobal < confNoGlobal-0.12 {
		t.Fatalf("global remain should only apply a soft confidence nudge; without=%s with=%s", noGlobalRemain.ConfidenceValue, withGlobalRemain.ConfidenceValue)
	}
}

func TestNetPowerSamplesPreferBatteryNetSignalMinute(t *testing.T) {
	history := newMinuteTelemetryHistory(4)
	snapshot := newEnergySnapshot()
	snapshot.HasInPV = true
	snapshot.InPVWatts = 120
	snapshot.HasOutAC = true
	snapshot.OutACWatts = 20
	snapshot.Packs[1] = &packSnapshot{HasPower: true, PowerW: -35}
	history.AddSample(time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local), snapshot)

	samples := netPowerSamplesFromMinuteHistory(history, 10)
	if len(samples) != 1 {
		t.Fatalf("sample count mismatch: got=%d want=1", len(samples))
	}
	if samples[0] > -34 || samples[0] < -36 {
		t.Fatalf("expected battery-net sample around -35W, got=%f", samples[0])
	}
}

func TestNetPowerSamplesPreferBatteryNetSignalFast(t *testing.T) {
	history := newPowerTelemetryHistory(10*time.Second, 4)
	snapshot := newEnergySnapshot()
	snapshot.HasInPV = true
	snapshot.InPVWatts = 120
	snapshot.HasOutAC = true
	snapshot.OutACWatts = 20
	snapshot.Packs[1] = &packSnapshot{HasPower: true, PowerW: -42}
	history.AddSample(time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local), snapshot)

	samples := netPowerSamplesFromPowerHistory(history, 10)
	if len(samples) != 1 {
		t.Fatalf("sample count mismatch: got=%d want=1", len(samples))
	}
	if samples[0] > -41 || samples[0] < -43 {
		t.Fatalf("expected battery-net sample around -42W, got=%f", samples[0])
	}
}

func TestSelectTopStateValueUsesDeviceUntilMLReady(t *testing.T) {
	snapshot := newEnergySnapshot()
	deviceState := "charging: 320min (~5h 20m)"
	ml := batteryETAEstimates{
		ActiveValue:     "120min (~2h 0m)",
		PowerValue:      "power: chg@180.0W",
		ConfidenceValue: "0.95 (high)",
	}

	got := selectTopStateValue(snapshot, deviceState, systemStateCharging, ml)
	if got != deviceState {
		t.Fatalf("top state should use device until ML is ready: got=%q want=%q", got, deviceState)
	}
}

func TestSelectTopStateValueUsesMLWhenReadyAndHighConfidence(t *testing.T) {
	snapshot := newEnergySnapshot()
	deviceState := "charging: 320min (~5h 20m)"
	ml := batteryETAEstimates{
		ActiveValue:     "120min (~2h 0m)",
		PowerValue:      "power: chg@180.0W (ewma+trend)",
		ConfidenceValue: "0.92 (high)",
	}

	got := selectTopStateValue(snapshot, deviceState, systemStateCharging, ml)
	want := "charging: 120min (~2h 0m)"
	if got != want {
		t.Fatalf("top state should use ML when ready and high confidence: got=%q want=%q", got, want)
	}
}

func TestSelectTopStateValueUsesDeviceWhenMLNotHigh(t *testing.T) {
	snapshot := newEnergySnapshot()
	deviceState := "discharging: 455min (~7h 35m)"
	ml := batteryETAEstimates{
		ActiveValue:     "300min (~5h 0m)",
		PowerValue:      "power: dsg@250.0W (ewma+trend)",
		ConfidenceValue: "0.68 (medium)",
	}

	got := selectTopStateValue(snapshot, deviceState, systemStateDischarging, ml)
	if got != deviceState {
		t.Fatalf("top state should use device remain when ML confidence is not high: got=%q want=%q", got, deviceState)
	}
}

func TestSelectTopStateValueUsesDeviceWhenBothLowConfidence(t *testing.T) {
	snapshot := newEnergySnapshot()
	deviceState := "discharging: 455min (~7h 35m)"
	ml := batteryETAEstimates{
		ActiveValue:     "300min (~5h 0m)",
		PowerValue:      "power: dsg@250.0W (ewma+trend)",
		ConfidenceValue: "0.40 (low)",
	}
	got := selectTopStateValue(snapshot, deviceState, systemStateDischarging, ml)
	if got != deviceState {
		t.Fatalf("top state should use device when ML confidence is low: got=%q want=%q", got, deviceState)
	}
}

func TestSelectTopStateValueAppliesConfidenceHysteresis(t *testing.T) {
	snapshot := newEnergySnapshot()
	deviceState := "discharging: 455min (~7h 35m)"
	ml := batteryETAEstimates{
		ActiveValue:     "300min (~5h 0m)",
		PowerValue:      "power: dsg@250.0W (ewma+trend)",
		ConfidenceValue: "0.92 (high)",
	}

	got := selectTopStateValue(snapshot, deviceState, systemStateDischarging, ml)
	if got != "discharging: 300min (~5h 0m)" {
		t.Fatalf("expected ML on initial high confidence, got=%q", got)
	}

	// Drop to medium: hysteresis should keep ML active and avoid source flip.
	ml.ConfidenceValue = "0.78 (medium)"
	got = selectTopStateValue(snapshot, deviceState, systemStateDischarging, ml)
	if got != "discharging: 300min (~5h 0m)" {
		t.Fatalf("expected ML to remain selected through medium confidence due to hysteresis, got=%q", got)
	}

	// Low-confidence streak: should eventually fall back to device.
	ml.ConfidenceValue = "0.40 (low)"
	for i := 0; i < 4; i++ {
		got = selectTopStateValue(snapshot, deviceState, systemStateDischarging, ml)
	}
	if got != deviceState {
		t.Fatalf("expected fallback to device once confidence drops below hysteresis floor, got=%q", got)
	}
}

func TestPickPreferredMLForTopStatePrefersNewWhenBothHighAndNewCloserToUnit(t *testing.T) {
	unit := batteryETAEstimates{
		ActiveValue:     "charging: 420min (~7h 0m)",
		PowerValue:      "power: chg@110.0W",
		ConfidenceValue: "0.95 (high)",
		ChargeValue:     "420min (~7h 0m)",
	}
	generic := batteryETAEstimates{
		ActiveValue:     "415min (~6h 55m)",
		PowerValue:      "power: chg@150.0W (profile:generic)",
		ConfidenceValue: "0.99 (high)",
		ChargeValue:     "415min (~6h 55m)",
	}
	new := batteryETAEstimates{
		ActiveValue:     "421min (~7h 1m)",
		PowerValue:      "power: chg@112.0W (profile:dpu)",
		ConfidenceValue: "0.99 (high)",
		ChargeValue:     "421min (~7h 1m)",
	}

	picked, label := pickPreferredMLForTopState(unit, generic, new, systemStateCharging)
	if label != "New" {
		t.Fatalf("expected New to be selected on equal confidence but better unit closeness, got=%s", label)
	}
	if picked.PowerValue != new.PowerValue {
		t.Fatalf("expected New estimate payload, got=%q", picked.PowerValue)
	}
}

func TestPickPreferredMLForTopStateFallsBackToMPPTWhenNewNotReady(t *testing.T) {
	unit := batteryETAEstimates{
		ActiveValue:     "charging: 420min (~7h 0m)",
		PowerValue:      "power: chg@110.0W",
		ConfidenceValue: "0.95 (high)",
		ChargeValue:     "420min (~7h 0m)",
	}
	generic := batteryETAEstimates{
		ActiveValue:     "415min (~6h 55m)",
		PowerValue:      "power: chg@112.0W (profile:generic)",
		ConfidenceValue: "0.97 (high)",
		ChargeValue:     "415min (~6h 55m)",
	}
	new := batteryETAEstimates{
		ActiveValue:     "n/a",
		PowerValue:      "power: n/a",
		ConfidenceValue: "n/a",
		ChargeValue:     "n/a",
	}

	picked, label := pickPreferredMLForTopState(unit, generic, new, systemStateCharging)
	if label != "MPPT" {
		t.Fatalf("expected MPPT to be selected when New is not ready, got=%s", label)
	}
	if picked.PowerValue != unit.PowerValue {
		t.Fatalf("expected MPPT estimate payload, got=%q", picked.PowerValue)
	}
}

func TestPickPreferredMLForTopStateSkipsGenericWhenDivergenceIsLarge(t *testing.T) {
	unit := batteryETAEstimates{
		ActiveValue:     "discharging: 420min (~7h 0m)",
		PowerValue:      "power: dsg@95.0W",
		ConfidenceValue: "0.95 (high)",
		DischargeValue:  "420min (~7h 0m)",
	}
	generic := batteryETAEstimates{
		ActiveValue:     "discharging: 980min (~16h 20m)",
		PowerValue:      "power: dsg@18.0W (profile:generic)",
		ConfidenceValue: "0.99 (high)",
		DischargeValue:  "980min (~16h 20m)",
	}
	new := batteryETAEstimates{
		ActiveValue:     "n/a",
		PowerValue:      "power: n/a",
		ConfidenceValue: "n/a",
		DischargeValue:  "n/a",
	}

	picked, label := pickPreferredMLForTopState(unit, generic, new, systemStateDischarging)
	if label != "MPPT" {
		t.Fatalf("expected MPPT when generic diverges too far, got=%s", label)
	}
	if picked.PowerValue != unit.PowerValue {
		t.Fatalf("expected MPPT estimate payload, got=%q", picked.PowerValue)
	}
}

func TestPickPreferredMLForTopStateFallsBackToMPPTWhenNoMLReady(t *testing.T) {
	unit := batteryETAEstimates{
		ActiveValue:     "charging: 420min (~7h 0m)",
		PowerValue:      "power: chg@110.0W",
		ConfidenceValue: "0.95 (high)",
		ChargeValue:     "420min (~7h 0m)",
	}
	generic := batteryETAEstimates{
		ActiveValue:     "n/a",
		PowerValue:      "power: n/a",
		ConfidenceValue: "n/a",
	}
	new := batteryETAEstimates{
		ActiveValue:     "n/a",
		PowerValue:      "power: n/a",
		ConfidenceValue: "n/a",
	}

	picked, label := pickPreferredMLForTopState(unit, generic, new, systemStateCharging)
	if label != "MPPT" {
		t.Fatalf("expected MPPT fallback when ML models are not ready, got=%s", label)
	}
	if picked.PowerValue != unit.PowerValue {
		t.Fatalf("expected MPPT estimate payload, got=%q", picked.PowerValue)
	}
}

func TestAdaptiveProfileRemainBlendIncreasesWithDivergence(t *testing.T) {
	base := 0.72
	lowDiv := adaptiveProfileRemainBlend(base, mlEstimateProfileD2M, 520, 500, "charge")
	highDiv := adaptiveProfileRemainBlend(base, mlEstimateProfileD2M, 900, 500, "charge")
	if highDiv <= lowDiv {
		t.Fatalf("expected higher blend for larger divergence, low=%f high=%f", lowDiv, highDiv)
	}
}

func TestApplyProfileEtaBiasD2MReducesPersistentOffset(t *testing.T) {
	snapshot := newEnergySnapshot()
	eta := 900.0
	device := 600.0
	for i := 0; i < 5; i++ {
		eta = applyProfileEtaBias(snapshot, mlEstimateProfileD2M, systemStateCharging, eta, device, "charge")
	}
	if eta >= 900 {
		t.Fatalf("expected D2M eta bias correction to reduce eta, got=%f", eta)
	}
}
