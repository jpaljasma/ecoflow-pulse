package main

import (
	"math"
	"strings"
	"testing"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

func TestRenderDashboardIncludesSummaryAndPackRows(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 27.4
	snapshot.HasDeviceSOC = true
	snapshot.WattsIn = 36
	snapshot.HasWattsIn = true
	snapshot.WattsOut = 12
	snapshot.HasWattsOut = true
	snapshot.RemainTimeRaw = 960
	snapshot.HasRemainTime = true
	pack := snapshot.ensurePack(2)
	pack.SOC = 27.4
	pack.HasSOC = true
	pack.TempC = 24
	pack.HasTemp = true
	pack.PowerW = -36
	pack.HasPower = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "kitInfo"},
		snapshot,
		nil,
		minuteTableConfig{},
	)

	for _, expected := range []string{
		"EcoFlow Live Telemetry",
		"Kitchen Delta 2 Max",
		"solar #1 [500W]",
		"max: 11-60V 15A 500W",
		"watts: n/a",
		"idle",
		"solar #2 [500W]",
		"[ ] AC On",
		"[ ] USB On",
		"[ ] 12V DC On",
		"[ ] UPS Passthrough",
		"[ ] Grounded (Estimated)",
		"[ ] Solar Passthrough",
		"[ ] Solar Charging",
		"Device",
		"New (",
		"Δ ETA vs Unit",
		"| Pack",
		"bp2",
		"discharging",
		"~16h 0m",
		"Solar Generated (Wh)",
		"| n/a",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("dashboard missing %q; output=%q", expected, output)
		}
	}
}

func TestRenderDashboardShowsPassthroughAndGroundedWhenACInOutEquivalent(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasInAC = true
	snapshot.InACWatts = 900
	snapshot.HasOutAC = true
	snapshot.OutACWatts = 890

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra", SN: "Y711ZABA9H2P0294"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{ShowSolarCandidates: true},
	)
	if !strings.Contains(output, "[x] UPS Passthrough") {
		t.Fatalf("expected passthrough checkbox on, got=%q", output)
	}
	if !strings.Contains(output, "[x] Grounded (Estimated)") {
		t.Fatalf("expected grounded estimate checkbox on, got=%q", output)
	}
	if !strings.Contains(output, "[ ] Solar Passthrough") {
		t.Fatalf("expected solar passthrough checkbox off during AC passthrough, got=%q", output)
	}
	if strings.Contains(output, "xt150_in:") || strings.Contains(output, "xt150_out:") {
		t.Fatalf("dpu dashboard should hide xt150 channels, got=%q", output)
	}
}

func TestFormatPVInputRowLabelUsesCapacityHints(t *testing.T) {
	d2m := ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max"}
	if got := formatPVInputRowLabel("low", d2m, newEnergySnapshot()); got != "solar #1 [500W]" {
		t.Fatalf("d2m low label mismatch: got=%q want=%q", got, "solar #1 [500W]")
	}
	if got := formatPVInputRowLabel("high", d2m, newEnergySnapshot()); got != "solar #2 [500W]" {
		t.Fatalf("d2m high label mismatch: got=%q want=%q", got, "solar #2 [500W]")
	}
	if got := formatPVInputCapability("low", d2m, newEnergySnapshot()); got != "max: 11-60V 15A 500W" {
		t.Fatalf("d2m low capability mismatch: got=%q want=%q", got, "max: 11-60V 15A 500W")
	}
	if got := formatPVInputCapability("high", d2m, newEnergySnapshot()); got != "max: 11-60V 15A 500W" {
		t.Fatalf("d2m high capability mismatch: got=%q want=%q", got, "max: 11-60V 15A 500W")
	}

	dpu := ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra"}
	if got := formatPVInputRowLabel("low", dpu, newEnergySnapshot()); got != "solar [1.6kW]" {
		t.Fatalf("dpu low label mismatch: got=%q want=%q", got, "solar [1.6kW]")
	}
	if got := formatPVInputRowLabel("high", dpu, newEnergySnapshot()); got != "solar [4kW]" {
		t.Fatalf("dpu high label mismatch: got=%q want=%q", got, "solar [4kW]")
	}
	if got := formatPVInputCapability("low", dpu, newEnergySnapshot()); got != "max: 30-150V 15A 1.6kW" {
		t.Fatalf("dpu low capability mismatch: got=%q want=%q", got, "max: 30-150V 15A 1.6kW")
	}
	if got := formatPVInputCapability("high", dpu, newEnergySnapshot()); got != "max: 80-450V 15A 4kW" {
		t.Fatalf("dpu high capability mismatch: got=%q want=%q", got, "max: 80-450V 15A 4kW")
	}
}

func TestFormatPVUtilizationGaugeSupportsOverflowPercent(t *testing.T) {
	device := ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max"}
	got := formatPVUtilizationGauge("low", device, newEnergySnapshot(), true, 525, false, 0)
	if !strings.Contains(got, "105.0%") {
		t.Fatalf("expected overflow percentage in gauge, got=%q", got)
	}
	if !strings.Contains(got, "[██████████]") {
		t.Fatalf("expected capped full-width gauge on overflow, got=%q", got)
	}
}

func TestIsLikelyACPassthrough(t *testing.T) {
	if !isLikelyACPassthrough(true, 900, true, 890) {
		t.Fatalf("expected passthrough when ac in/out are equivalent")
	}
	if isLikelyACPassthrough(true, 900, true, 700) {
		t.Fatalf("did not expect passthrough when ac in/out diverge")
	}
	if isLikelyACPassthrough(true, 10, true, 10) {
		t.Fatalf("did not expect passthrough below minimum watts threshold")
	}
	if isLikelyACPassthrough(false, 900, true, 900) {
		t.Fatalf("did not expect passthrough when input signal is missing")
	}
}

func TestIsLikelySolarPassthrough(t *testing.T) {
	chargingFromSolar := newEnergySnapshot()
	chargingFromSolar.HasOutAC = true
	chargingFromSolar.OutACWatts = 130
	chargingFromSolar.HasInAC = true
	chargingFromSolar.InACWatts = 0
	chargingFromSolar.HasInPV = true
	chargingFromSolar.InPVWatts = 170
	if !isLikelySolarPassthrough(chargingFromSolar, 42, true, 0, true) {
		t.Fatalf("expected solar passthrough when AC output is served by PV and battery is charging")
	}
	if !isLikelySolarPassthrough(chargingFromSolar, 0, false, 0, false) {
		t.Fatalf("expected solar passthrough when PV covers AC output and batteries are full/not charging")
	}

	batteryDischarging := newEnergySnapshot()
	batteryDischarging.HasOutAC = true
	batteryDischarging.OutACWatts = 130
	batteryDischarging.HasInAC = true
	batteryDischarging.InACWatts = 0
	batteryDischarging.HasInPV = true
	batteryDischarging.InPVWatts = 70
	if isLikelySolarPassthrough(batteryDischarging, 0, true, 45, true) {
		t.Fatalf("did not expect solar passthrough when battery is discharging")
	}
}

func TestInferBatteryChargeSource(t *testing.T) {
	tests := []struct {
		name            string
		state           systemStateKind
		acInW           float64
		hasACIn         bool
		acOutW          float64
		hasACOut        bool
		pvInW           float64
		hasPVIn         bool
		batteryInW      float64
		hasBatteryIn    bool
		effectiveInW    float64
		hasEffectiveIn  bool
		effectiveOutW   float64
		hasEffectiveOut bool
		want            string
	}{
		{
			name:    "charging from ac",
			state:   systemStateCharging,
			acInW:   900,
			hasACIn: true,
			want:    "ac",
		},
		{
			name:    "charging from solar",
			state:   systemStateCharging,
			pvInW:   240,
			hasPVIn: true,
			want:    "solar",
		},
		{
			name:     "charging hybrid",
			state:    systemStateCharging,
			acInW:    350,
			hasACIn:  true,
			acOutW:   120,
			hasACOut: true,
			pvInW:    210,
			hasPVIn:  true,
			want:     "hybrid(ac+solar)",
		},
		{
			name:     "charging solar when ac passthrough dominates",
			state:    systemStateCharging,
			acInW:    61,
			hasACIn:  true,
			acOutW:   60,
			hasACOut: true,
			pvInW:    55,
			hasPVIn:  true,
			want:     "solar",
		},
		{
			name:         "charging hybrid when ac passthrough-like but battery charge exceeds pv",
			state:        systemStateCharging,
			acInW:        61,
			hasACIn:      true,
			acOutW:       60,
			hasACOut:     true,
			pvInW:        55,
			hasPVIn:      true,
			batteryInW:   90,
			hasBatteryIn: true,
			want:         "hybrid(ac+solar)",
		},
		{
			name:            "charging hybrid inferred from net charge when battery flow unavailable",
			state:           systemStateCharging,
			acInW:           61,
			hasACIn:         true,
			acOutW:          60,
			hasACOut:        true,
			pvInW:           55,
			hasPVIn:         true,
			effectiveInW:    220,
			hasEffectiveIn:  true,
			effectiveOutW:   120,
			hasEffectiveOut: true,
			want:            "hybrid(ac+solar)",
		},
		{
			name:  "idle no source",
			state: systemStateIdle,
			want:  "none",
		},
		{
			name:    "discharging with solar assist",
			state:   systemStateDischarging,
			pvInW:   120,
			hasPVIn: true,
			want:    "battery+solar",
		},
		{
			name:  "discharging battery only",
			state: systemStateDischarging,
			want:  "battery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := newEnergySnapshot()
			snapshot.InACWatts = tt.acInW
			snapshot.HasInAC = tt.hasACIn
			snapshot.OutACWatts = tt.acOutW
			snapshot.HasOutAC = tt.hasACOut
			snapshot.InPVWatts = tt.pvInW
			snapshot.HasInPV = tt.hasPVIn
			snapshot.BatteryInWatts = tt.batteryInW
			snapshot.HasBatteryIn = tt.hasBatteryIn

			derived := snapshotDerived{
				SystemStateValue: string(tt.state),
				EffectiveIn:      tt.effectiveInW,
				HasEffectiveIn:   tt.hasEffectiveIn,
				EffectiveOut:     tt.effectiveOutW,
				HasEffectiveOut:  tt.hasEffectiveOut,
			}
			if got := inferBatteryChargeSource(snapshot, derived); got != tt.want {
				t.Fatalf("source mismatch: got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestFormatSolarNetSummaryIncludesSmoothedAverageWhenActive(t *testing.T) {
	got := formatSolarNetSummary("active(92.5W)", "92.5W (~96.3W avg)")
	want := "active: 92.5W (~96.3W avg)"
	if got != want {
		t.Fatalf("solar net summary mismatch: got=%q want=%q", got, want)
	}

	got = formatSolarNetSummary("active(92.5W)", "n/a")
	want = "active: 92.5W"
	if got != want {
		t.Fatalf("solar net summary fallback mismatch: got=%q want=%q", got, want)
	}
}

func TestRenderDashboardShowsSmoothedPVButSummaryStaysRaw(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.configurePVSmoothing(3)

	applyPV := func(low, high float64) {
		snapshot.InPVLowWatts = low
		snapshot.HasInPVLow = true
		snapshot.InPVHighWatts = high
		snapshot.HasInPVHigh = true
		snapshot.refreshPVTotalFromChannels()
		snapshot.pushPVSmoothingSample()
	}
	applyPV(30, 60)
	applyPV(40, 70)
	applyPV(50, 80)

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{ShowSolarCandidates: true},
	)

	if !strings.Contains(output, "130.0W (~110.0W avg)") {
		t.Fatalf("expected smoothed pv total in dashboard, got=%q", output)
	}
	if !strings.Contains(output, "watts: 50.0W (~40.0W avg)") {
		t.Fatalf("expected smoothed pv low in dashboard, got=%q", output)
	}
	if !strings.Contains(output, "watts: 80.0W (~70.0W avg)") {
		t.Fatalf("expected smoothed pv high in dashboard, got=%q", output)
	}

	summary := snapshot.String()
	if !strings.Contains(summary, "in_pv_low=50.0W") || !strings.Contains(summary, "in_pv_high=80.0W") || !strings.Contains(summary, "in_pv=130.0W") {
		t.Fatalf("expected raw pv values in summary, got=%q", summary)
	}
	if strings.Contains(summary, "avg") {
		t.Fatalf("summary should not include smoothed marker, got=%q", summary)
	}
}

func TestRenderDashboardShowsSmoothedTotalsButSummaryStaysRaw(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.configurePowerSmoothing(3)

	applyTotals := func(inWatts, outWatts float64) {
		snapshot.WattsIn = inWatts
		snapshot.HasWattsIn = true
		snapshot.WattsOut = outWatts
		snapshot.HasWattsOut = true
		snapshot.InACWatts = inWatts
		snapshot.HasInAC = true
		snapshot.pushPowerSmoothingSample()
	}
	applyTotals(300, 100)
	applyTotals(200, 100)
	applyTotals(100, 100)

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)

	if !strings.Contains(output, "100.0W (~200.0W avg)") {
		t.Fatalf("expected smoothed total input in dashboard, got=%q", output)
	}
	if !strings.Contains(output, "0.0W (~100.0W avg)") {
		t.Fatalf("expected smoothed total net in dashboard, got=%q", output)
	}

	summary := snapshot.String()
	if !strings.Contains(summary, "in=100.0W") || !strings.Contains(summary, "out=100.0W") || !strings.Contains(summary, "net=0.0W") {
		t.Fatalf("expected raw totals in summary, got=%q", summary)
	}
	if strings.Contains(summary, "avg") {
		t.Fatalf("summary should not include smoothed marker, got=%q", summary)
	}
}

func TestRenderDashboardHidesPreconditioningForD2M(t *testing.T) {
	snapshot := newEnergySnapshot()
	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if strings.Contains(output, "Battery Preconditioning On") {
		t.Fatalf("d2m dashboard should hide preconditioning line, got=%q", output)
	}
}

func TestRenderDashboardShowsPreconditioningForDPU(t *testing.T) {
	snapshot := newEnergySnapshot()
	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra", SN: "Y711ZABA9H2P0294"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{ShowSolarCandidates: true},
	)
	if !strings.Contains(output, "Battery Preconditioning On") {
		t.Fatalf("dpu dashboard should show preconditioning line, got=%q", output)
	}
}

func TestTopStateDisplayIcon(t *testing.T) {
	tests := []struct {
		name   string
		state  systemStateKind
		value  string
		source string
		want   string
	}{
		{name: "charging ac source", state: systemStateCharging, value: "charging: 1", source: "ac", want: "🌩"},
		{name: "charging solar source", state: systemStateCharging, value: "charging: 1", source: "solar", want: "🌞"},
		{name: "charging hybrid source", state: systemStateCharging, value: "charging: 1", source: "hybrid(ac+solar)", want: "🔆"},
		{name: "discharging", state: systemStateDischarging, value: "discharging: 1", source: "battery", want: "🔻"},
		{name: "idle", state: systemStateIdle, value: "idle: 1", source: "none", want: "🟢"},
		{name: "infer charging", state: systemStateUnknown, value: "charging: 1", source: "ac", want: "🌩"},
		{name: "infer charging solar source", state: systemStateUnknown, value: "charging: 1", source: "solar", want: "🌞"},
		{name: "infer charging hybrid source", state: systemStateUnknown, value: "charging: 1", source: "hybrid(ac+solar)", want: "🔆"},
		{name: "infer discharging", state: systemStateUnknown, value: "discharging: 1", source: "battery", want: "🔻"},
		{name: "infer idle", state: systemStateUnknown, value: "idle: 1", source: "none", want: "🟢"},
		{name: "value charging overrides stale idle state", state: systemStateIdle, value: "charging: 1", source: "solar", want: "🌞"},
		{name: "value discharging overrides stale charging state", state: systemStateCharging, value: "discharging: 1", source: "battery", want: "🔻"},
		{name: "source solar overrides stale discharging value", state: systemStateDischarging, value: "discharging: 1", source: "solar", want: "🌞"},
		{name: "source hybrid overrides stale discharging value", state: systemStateDischarging, value: "discharging: 1", source: "hybrid(ac+solar)", want: "🔆"},
		{name: "unknown", state: systemStateUnknown, value: "n/a", source: "n/a", want: ""},
	}

	for _, tc := range tests {
		got := topStateDisplayIcon(tc.state, tc.value, tc.source)
		if got != tc.want {
			t.Fatalf("%s: icon mismatch got=%q want=%q", tc.name, got, tc.want)
		}
	}
}

func TestSanitizeStateColumnValue(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "⚡ charging: 320min (~5h 20m)", want: "charging: 320min (~5h 20m)"},
		{in: "↓ discharging: 455min (~7h 35m)", want: "discharging: 455min (~7h 35m)"},
		{in: "⏸ idle: 30min (~30m)", want: "idle: 30min (~30m)"},
		{in: "charging: 120min (~2h 0m)", want: "charging: 120min (~2h 0m)"},
	}

	for _, tc := range tests {
		got := sanitizeStateColumnValue(tc.in)
		if got != tc.want {
			t.Fatalf("sanitize state mismatch: in=%q got=%q want=%q", tc.in, got, tc.want)
		}
	}
}

func TestRenderDashboardShowsMQTTQueueRow(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.MQTTQueueDepth = 3
	snapshot.MQTTQueueCapacity = 128
	snapshot.MQTTQueueDroppedOldest = 2

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{ShowSolarCandidates: true},
	)
	if !strings.Contains(output, "mqtt") || !strings.Contains(output, "queue: 3/128") || !strings.Contains(output, "drop-oldest: 2") || !strings.Contains(output, "status: MQTT reconnecting") {
		t.Fatalf("dashboard missing mqtt queue row, got=%q", output)
	}
}

func TestFormatMQTTStatusDegradedFallback(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.MQTTDegraded = true
	snapshot.MQTTDegradedReason = "MQTT auth degraded (broker reject code 5)"
	snapshot.MQTTFallbackActive = true

	got := formatMQTTStatus(snapshot)
	want := "MQTT auth degraded (broker reject code 5) + REST fallback"
	if got != want {
		t.Fatalf("mqtt degraded status mismatch: got=%q want=%q", got, want)
	}
}

func TestRenderDashboardShowsMQTTDegradedStatus(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.MQTTQueueDepth = 0
	snapshot.MQTTQueueCapacity = 128
	snapshot.MQTTQueueDroppedOldest = 0
	snapshot.MQTTDegraded = true
	snapshot.MQTTDegradedReason = "MQTT auth degraded (broker reject code 5)"
	snapshot.MQTTFallbackActive = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "n/a"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "status: MQTT auth degraded (broker reject code 5) + REST fallback") {
		t.Fatalf("dashboard missing mqtt degraded status row, got=%q", output)
	}
}

func TestRenderDashboardShowsSolarRecommendations(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 40
	snapshot.HasDeviceSOC = true
	snapshot.FullEnergyWh = 2048
	snapshot.HasFullEnergy = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMaxChargeSOC = true
	snapshot.InPVLowWatts = 120
	snapshot.HasInPVLow = true
	snapshot.InPVHighWatts = 260
	snapshot.HasInPVHigh = true
	snapshot.refreshPVTotalFromChannels()

	snapshot.HasPVLowPanelPrediction = true
	snapshot.PVLowPanelSetup = "EcoFlow 220W Bifacial Portable"
	snapshot.PVLowPanelConfidence = 0.93
	snapshot.PVLowPanelSamples = 88
	snapshot.PVLowPanelCount = 1
	snapshot.HasPVLowPanelCount = true
	snapshot.PVLowPanelNominalWatts = 220
	snapshot.HasPVLowPanelNominal = true
	snapshot.PVLowBestPanelLabel = "Premium 500W Panel"
	snapshot.HasPVLowBestPanelLabel = true
	snapshot.PVLowBestPanelWatts = 500
	snapshot.HasPVLowBestPanelWatts = true
	snapshot.PVLowAltPanelLabel = "Value 400W Panel"
	snapshot.HasPVLowAltPanelLabel = true
	snapshot.PVLowAltPanelWatts = 400
	snapshot.HasPVLowAltPanelWatts = true

	snapshot.HasPVHighPanelPrediction = true
	snapshot.PVHighPanelSetup = "4x125W EcoFlow Bifacial Modular"
	snapshot.PVHighPanelConfidence = 0.90
	snapshot.PVHighPanelSamples = 92
	snapshot.PVHighPanelCount = 4
	snapshot.HasPVHighPanelCount = true
	snapshot.PVHighPanelNominalWatts = 500
	snapshot.HasPVHighPanelNominal = true
	snapshot.PVHighBestPanelLabel = "Premium 500W Panel"
	snapshot.HasPVHighBestPanelLabel = true
	snapshot.PVHighBestPanelWatts = 500
	snapshot.HasPVHighBestPanelWatts = true
	snapshot.PVHighAltPanelLabel = "Value 420W Panel"
	snapshot.HasPVHighAltPanelLabel = true
	snapshot.PVHighAltPanelWatts = 420
	snapshot.HasPVHighAltPanelWatts = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)

	for _, expected := range []string{
		"| Metric",
		"| Detected",
		"| Add Panels",
		"| Upgrade Panels",
		"| Upgrade Panels #2",
		"| Charge ETA Impact",
		"| Charge ETA Impact #2",
		"| All Ports ETA Impact",
		"| All Ports ETA Impact #2",
		"| Best Upgrade Path",
		"add 2x ~220W panel",
		"already near port max",
		"sunny base:",
		"* Results: Charge time",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("dashboard missing %q; output=%q", expected, output)
		}
	}
}

func TestRenderDashboardPrefersNonClippingSecondUpgradeRecommendation(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 50
	snapshot.HasDeviceSOC = true
	snapshot.FullEnergyWh = 2048
	snapshot.HasFullEnergy = true
	snapshot.HasPVLowPanelPrediction = true
	snapshot.PVLowPanelSetup = "1x 220W Panel"
	snapshot.PVLowPanelConfidence = 0.91
	snapshot.PVLowPanelSamples = 40
	snapshot.PVLowPanelCount = 1
	snapshot.HasPVLowPanelCount = true
	snapshot.PVLowPanelNominalWatts = 220
	snapshot.HasPVLowPanelNominal = true
	snapshot.PVLowBestPanelLabel = "Aggressive 400W Panel"
	snapshot.HasPVLowBestPanelLabel = true
	snapshot.PVLowBestPanelWatts = 400
	snapshot.HasPVLowBestPanelWatts = true
	snapshot.PVLowAltPanelLabel = "Balanced 300W Panel"
	snapshot.HasPVLowAltPanelLabel = true
	snapshot.PVLowAltPanelWatts = 300
	snapshot.HasPVLowAltPanelWatts = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "replace with 2x Aggressive 400W Panel (2S1P, ~800W STC, clipped to 500W)") {
		t.Fatalf("expected clipped first upgrade recommendation, got=%q", output)
	}
	if !strings.Contains(output, "replace with 2x Panel (2S1P, ~440W STC)") {
		t.Fatalf("expected non-clipping second upgrade recommendation, got=%q", output)
	}
}

func TestRenderDashboardPromotesSecondUpgradeWhenFirstIsUnsafe(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 45
	snapshot.HasDeviceSOC = true
	snapshot.HasPVLowPanelPrediction = true
	snapshot.PVLowPanelSetup = "1x 220W Panel"
	snapshot.PVLowPanelConfidence = 0.88
	snapshot.PVLowPanelSamples = 28
	snapshot.PVLowPanelCount = 1
	snapshot.HasPVLowPanelCount = true
	snapshot.PVLowPanelNominalWatts = 220
	snapshot.HasPVLowPanelNominal = true

	snapshot.PVLowBestPanelLabel = "High Current 500W Panel"
	snapshot.HasPVLowBestPanelLabel = true
	snapshot.PVLowBestPanelWatts = 500
	snapshot.HasPVLowBestPanelWatts = true
	snapshot.PVLowBestPanelImpA = 20.0
	snapshot.HasPVLowBestPanelImpA = true

	snapshot.PVLowAltPanelLabel = "Safe 450W Panel"
	snapshot.HasPVLowAltPanelLabel = true
	snapshot.PVLowAltPanelWatts = 450
	snapshot.HasPVLowAltPanelWatts = true
	snapshot.PVLowAltPanelImpA = 12.0
	snapshot.HasPVLowAltPanelImpA = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "replace with 2x Safe 450W Panel") {
		t.Fatalf("expected safe alt panel to be promoted into upgrade #1, got=%q", output)
	}
	if strings.Contains(output, "replace with 1x High Current 500W Panel") {
		t.Fatalf("unexpected unsafe best panel recommendation in output=%q", output)
	}
}

func TestRenderDashboardPrefersPeerPortDetectedSetupForTwinPVPorts(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 52
	snapshot.HasDeviceSOC = true

	// Current port (low): underutilized setup.
	snapshot.HasPVLowPanelPrediction = true
	snapshot.PVLowPanelSetup = "1x 220W EcoFlow Bifacial Portable"
	snapshot.PVLowPanelConfidence = 0.92
	snapshot.PVLowPanelSamples = 55
	snapshot.PVLowPanelCount = 1
	snapshot.HasPVLowPanelCount = true
	snapshot.PVLowPanelNominalWatts = 220
	snapshot.HasPVLowPanelNominal = true

	// Peer port (high): known-good setup the user already runs.
	snapshot.HasPVHighPanelPrediction = true
	snapshot.PVHighPanelSetup = "4x125W EcoFlow Bifacial Modular"
	snapshot.PVHighPanelConfidence = 0.95
	snapshot.PVHighPanelSamples = 80
	snapshot.PVHighPanelCount = 4
	snapshot.HasPVHighPanelCount = true
	snapshot.PVHighPanelNominalWatts = 500
	snapshot.HasPVHighPanelNominal = true

	// DB options exist, but peer setup should still be eligible/preferred when better.
	snapshot.PVLowBestPanelLabel = "Generic 450W Panel"
	snapshot.HasPVLowBestPanelLabel = true
	snapshot.PVLowBestPanelWatts = 450
	snapshot.HasPVLowBestPanelWatts = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "replace with 4x EcoFlow Bifacial Modular") {
		t.Fatalf("expected peer-port setup recommendation for twin 500W ports, got=%q", output)
	}
}

func TestRenderDashboardSupportsJASolar400RecommendationForD2M(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 51
	snapshot.HasDeviceSOC = true

	snapshot.HasPVLowPanelPrediction = true
	snapshot.PVLowPanelSetup = "1x220W EcoFlow Bifacial Portable"
	snapshot.PVLowPanelConfidence = 0.90
	snapshot.PVLowPanelSamples = 50
	snapshot.PVLowPanelCount = 1
	snapshot.HasPVLowPanelCount = true
	snapshot.PVLowPanelNominalWatts = 220
	snapshot.HasPVLowPanelNominal = true

	snapshot.HasPVHighPanelPrediction = true
	snapshot.PVHighPanelSetup = "2x400W JA Solar bifacial"
	snapshot.PVHighPanelConfidence = 0.92
	snapshot.PVHighPanelSamples = 44
	snapshot.PVHighPanelCount = 2
	snapshot.HasPVHighPanelCount = true
	snapshot.PVHighPanelNominalWatts = 800
	snapshot.HasPVHighPanelNominal = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "JA Solar bifacial") {
		t.Fatalf("expected JA Solar bifacial recommendation for D2M, got=%q", output)
	}
}

func TestRenderDashboardSupportsJASolar400RecommendationForDPULow(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 35
	snapshot.HasDeviceSOC = true

	snapshot.HasPVLowPanelPrediction = true
	snapshot.PVLowPanelSetup = "2x400W JA Solar bifacial"
	snapshot.PVLowPanelConfidence = 0.94
	snapshot.PVLowPanelSamples = 66
	snapshot.PVLowPanelCount = 2
	snapshot.HasPVLowPanelCount = true
	snapshot.PVLowPanelNominalWatts = 800
	snapshot.HasPVLowPanelNominal = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra", SN: "Y711ZABA9H2P0294"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{ShowSolarCandidates: true},
	)
	if !strings.Contains(output, "JA Solar bifacial") {
		t.Fatalf("expected JA Solar bifacial recommendation for DPU, got=%q", output)
	}
}

func TestRenderDashboardUsesPeerDetectedFallbackWhenPrimaryRecommendationMissing(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 28
	snapshot.HasDeviceSOC = true

	snapshot.HasPVLowPanelPrediction = true
	snapshot.PVLowPanelSetup = "2x400W JA Solar bifacial"
	snapshot.PVLowPanelConfidence = 0.95
	snapshot.PVLowPanelSamples = 70
	snapshot.PVLowPanelCount = 2
	snapshot.HasPVLowPanelCount = true
	snapshot.PVLowPanelNominalWatts = 800
	snapshot.HasPVLowPanelNominal = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra", SN: "Y711ZABA9H2P0294"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{ShowSolarCandidates: true},
	)
	if !strings.Contains(output, "replace with 10x JA Solar bifacial") {
		t.Fatalf("expected peer-detected fallback recommendation on missing primary target, got=%q", output)
	}
}

func TestRenderDashboardAddPanelsUsesPeerDetectionFallbackLowToHigh(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 27
	snapshot.HasDeviceSOC = true

	snapshot.HasPVLowPanelPrediction = true
	snapshot.PVLowPanelSetup = "2x400W JA Solar bifacial"
	snapshot.PVLowPanelConfidence = 0.95
	snapshot.PVLowPanelSamples = 60
	snapshot.PVLowPanelCount = 2
	snapshot.HasPVLowPanelCount = true
	snapshot.PVLowPanelNominalWatts = 800
	snapshot.HasPVLowPanelNominal = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra", SN: "Y711ZABA9H2P0294"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "mirror peer setup: add 10x ~400W panel") {
		t.Fatalf("expected high-port add fallback from low detected setup, got=%q", output)
	}
	if strings.Contains(output, "waiting for panel detection") {
		t.Fatalf("did not expect waiting state when peer detection is available, got=%q", output)
	}
}

func TestRenderDashboardAddPanelsUsesPeerDetectionFallbackHighToLow(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 27
	snapshot.HasDeviceSOC = true

	snapshot.HasPVHighPanelPrediction = true
	snapshot.PVHighPanelSetup = "2x400W JA Solar bifacial"
	snapshot.PVHighPanelConfidence = 0.95
	snapshot.PVHighPanelSamples = 60
	snapshot.PVHighPanelCount = 2
	snapshot.HasPVHighPanelCount = true
	snapshot.PVHighPanelNominalWatts = 800
	snapshot.HasPVHighPanelNominal = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra", SN: "Y711ZABA9H2P0294"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "mirror peer setup: add 4x ~400W panel") {
		t.Fatalf("expected low-port add fallback from high detected setup, got=%q", output)
	}
	if strings.Contains(output, "waiting for panel detection") {
		t.Fatalf("did not expect waiting state when peer detection is available, got=%q", output)
	}
}

func TestRenderDashboardShowsDPUHighCandidateElectricalData(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasPVHighDBCandidates = true
	snapshot.PVHighDBCandidates = []panelDBCandidate{
		{
			Label:      "EcoFlow 125W Bifacial Modular Solar Panel",
			Status:     "needs_series",
			PanelWatts: 125,
			VocV:       50,
			VmpV:       43,
			ImpA:       3.0,
			IscA:       3.2,
			MinSeries:  2,
			MaxSeries:  9,
			Bifacial:   true,
		},
		{
			Label:      "EcoFlow 220W Bifacial Portable Solar Panel",
			Status:     "needs_series",
			PanelWatts: 220,
			VocV:       21.5,
			VmpV:       18.4,
			ImpA:       11.9,
			IscA:       12.4,
			MinSeries:  5,
			MaxSeries:  20,
			Bifacial:   true,
		},
	}

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra", SN: "Y711ZABA9H2P0294"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{ShowSolarCandidates: true},
	)
	for _, expected := range []string{
		"| Series(cold)",
		"solar [4kW]",
		"EcoFlow 125W Bifacial Modular Solar Panel",
		"EcoFlow 220W Bifacial Portable Solar Panel",
		"50.0/43.0V",
		"21.5/18.4V",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected candidate electrical data %q in output=%q", expected, output)
		}
	}
}

func TestRenderDashboardShowsD2MSafeEcoFlowCandidatePanels(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasPVLowDBCandidates = true
	snapshot.PVLowDBCandidates = []panelDBCandidate{
		{
			Label:      "EcoFlow 125W Bifacial Modular Solar Panel",
			Status:     "yes",
			PanelWatts: 125,
			VocV:       50,
			VmpV:       43,
			ImpA:       3.0,
			IscA:       3.2,
			Bifacial:   true,
		},
		{
			Label:      "EcoFlow 220W Bifacial Portable Solar Panel",
			Status:     "yes",
			PanelWatts: 220,
			VocV:       21.5,
			VmpV:       18.4,
			ImpA:       11.9,
			IscA:       12.4,
			Bifacial:   true,
		},
	}

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{ShowSolarCandidates: true},
	)
	for _, expected := range []string{
		"EcoFlow 125W Bifacial Modular Solar Panel",
		"EcoFlow 220W Bifacial Portable Solar Panel",
		"1S4P (4x)",
		"2S1P (2x)",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected safe D2M candidate %q in output=%q", expected, output)
		}
	}
}

func TestRenderDashboardSkipsTrivialSamePanelAltLayout(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 30
	snapshot.HasDeviceSOC = true
	snapshot.HasPVHighPanelPrediction = true
	snapshot.PVHighPanelSetup = "6x400W JA Solar bifacial"
	snapshot.PVHighPanelConfidence = 0.93
	snapshot.PVHighPanelSamples = 80
	snapshot.PVHighPanelCount = 6
	snapshot.HasPVHighPanelCount = true
	snapshot.PVHighPanelNominalWatts = 2400
	snapshot.HasPVHighPanelNominal = true

	// Keep only one DB upgrade candidate so fallback would attempt same-model alt layout.
	snapshot.PVHighBestPanelLabel = "JA Solar bifacial"
	snapshot.HasPVHighBestPanelLabel = true
	snapshot.PVHighBestPanelWatts = 400
	snapshot.HasPVHighBestPanelWatts = true
	snapshot.PVHighBestPanelVocV = 50
	snapshot.HasPVHighBestPanelVocV = true
	snapshot.PVHighBestPanelImpA = 10
	snapshot.HasPVHighBestPanelImpA = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra", SN: "Y711ZABA9H2P0294"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if strings.Contains(output, "(alt layout)") {
		t.Fatalf("expected trivial same-panel alt layout to be suppressed, got=%q", output)
	}
}

func TestRenderASCIITableSupportsColspanRow(t *testing.T) {
	headers := []string{"Metric", "Solar #1", "Solar #2"}
	rows := [][]string{
		{"Detected", "A", "B"},
		{"All Ports ETA Impact", makeColspanCell("sunny base: 90min; add: 60min (-30m)")},
	}

	table := renderASCIITable(headers, rows)
	lines := strings.Split(table, "\n")
	var spanLine string
	for _, line := range lines {
		if strings.Contains(line, "All Ports ETA Impact") {
			spanLine = line
			break
		}
	}
	if spanLine == "" {
		t.Fatalf("expected span row in table, got=%q", table)
	}
	// Spanned row should have only outer + one internal separator:
	// | metric | spanned-content |
	if got, want := strings.Count(spanLine, "|"), 3; got != want {
		t.Fatalf("unexpected separator count for spanned row: got=%d want=%d row=%q", got, want, spanLine)
	}
}

func TestRenderASCIITableSupportsMultilineColspanRow(t *testing.T) {
	headers := []string{"Metric", "Solar #1", "Solar #2"}
	rows := [][]string{
		{"Best Upgrade Path", makeColspanCell("* Get 3x Panel A\n* Install Panel A into solar [500W]\n* Results: Charge time 120min (~2h 0m) (-1h 0m)")},
	}

	table := renderASCIITable(headers, rows)
	for _, expected := range []string{
		"Best Upgrade Path",
		"* Get 3x Panel A",
		"* Install Panel A into solar [500W]",
		"* Results: Charge time 120min (~2h 0m) (-1h 0m)",
	} {
		if !strings.Contains(table, expected) {
			t.Fatalf("expected multiline colspan content %q, table=%q", expected, table)
		}
	}
}

func TestSolarRecommendationPlanCacheRefreshesOnlyOnDetectedPanelChange(t *testing.T) {
	snapshot := newEnergySnapshot()

	lowPort := solarRecommendationPort{
		channel:     "low",
		label:       "solar [500W]",
		maxWatts:    500,
		hasMaxWatts: true,
		detected: detectedPanelSetup{
			has:         true,
			setup:       "2x100W panel A",
			panelCount:  2,
			hasCount:    true,
			nominalW:    200,
			hasNominalW: true,
		},
		upgrade: upgradePanelTarget{
			hasLabel:    true,
			label:       "Panel A",
			hasPanelW:   true,
			panelWatts:  200,
			hasPanelVoc: true,
			panelVocV:   40,
			hasPanelImp: true,
			panelImpA:   5,
		},
	}
	highPort := solarRecommendationPort{
		channel: "high",
		label:   "solar [500W]",
	}

	first := loadOrBuildSolarRecommendationPlans(snapshot, []solarRecommendationPort{lowPort, highPort}, false, false)
	if len(first) == 0 {
		t.Fatalf("expected cached recommendation plans on first build")
	}
	if !strings.Contains(first[0].upgradeText, "Panel A") {
		t.Fatalf("expected first recommendation to use Panel A, got=%q", first[0].upgradeText)
	}

	lowPort.upgrade.label = "Panel B"
	second := loadOrBuildSolarRecommendationPlans(snapshot, []solarRecommendationPort{lowPort, highPort}, false, false)
	if len(second) == 0 {
		t.Fatalf("expected cached recommendation plans on second build")
	}
	if !strings.Contains(second[0].upgradeText, "Panel A") {
		t.Fatalf("expected cached recommendation to remain Panel A until detected panel changes, got=%q", second[0].upgradeText)
	}

	lowPort.detected.setup = "2x100W panel B"
	third := loadOrBuildSolarRecommendationPlans(snapshot, []solarRecommendationPort{lowPort, highPort}, false, false)
	if len(third) == 0 {
		t.Fatalf("expected cached recommendation plans on third build")
	}
	if !strings.Contains(third[0].upgradeText, "Panel B") {
		t.Fatalf("expected recommendation refresh after detected panel change, got=%q", third[0].upgradeText)
	}
}

func TestBuildBestUpgradePathSummaryPrefersMixedFastestScenario(t *testing.T) {
	portRows := []solarPortRecommendationData{
		{
			channel:           "low",
			label:             "solar [1.6kW]",
			basePotentialETAW: 800,
			portMaxWatts:      1600,
			hasPortMaxWatts:   true,
			addOption: solarRecommendationOption{
				hasPotential: true,
				sourceLabel:  "JA Solar bifacial",
				series:       2,
				parallel:     1,
				units:        2,
				nominalW:     800,
				potentialW:   1200,
			},
			upgradeOption: solarRecommendationOption{
				hasPotential: true,
				sourceLabel:  "JA Solar bifacial",
				series:       4,
				parallel:     1,
				units:        4,
				nominalW:     1600,
				potentialW:   1600,
			},
			upgradeOption2: solarRecommendationOption{
				hasPotential: true,
				sourceLabel:  "TW Solar 500W",
				series:       3,
				parallel:     1,
				units:        3,
				nominalW:     1500,
				potentialW:   1500,
			},
		},
		{
			channel:           "high",
			label:             "solar [4kW]",
			basePotentialETAW: 0,
			portMaxWatts:      4000,
			hasPortMaxWatts:   true,
			addOption: solarRecommendationOption{
				hasPotential: true,
				sourceLabel:  "EcoFlow 400W Rigid",
				series:       10,
				parallel:     1,
				units:        10,
				nominalW:     4000,
				potentialW:   4000,
			},
			upgradeOption: solarRecommendationOption{
				hasPotential: true,
				sourceLabel:  "EcoFlow 400W Rigid",
				series:       10,
				parallel:     1,
				units:        10,
				nominalW:     4000,
				potentialW:   4000,
			},
			upgradeOption2: solarRecommendationOption{
				hasPotential: true,
				sourceLabel:  "Trina Solar TSM-NEG19RC.20 (Vertex N 615W, bifacial)",
				series:       7,
				parallel:     1,
				units:        7,
				nominalW:     4305,
				potentialW:   4000,
				clipped:      true,
				bifacial:     true,
			},
		},
	}

	got := buildBestUpgradePathSummary(portRows, 10000, true, 800)
	for _, expected := range []string{
		"* Get 4x JA Solar bifacial",
		"* Get 7x Trina Solar TSM-NEG19RC.20 (Vertex N 615W, bifacial)",
		"* Install JA Solar bifacial (4S1P, ~1.6kW STC) into solar [1.6kW]",
		"* Install Trina Solar TSM-NEG19RC.20 (Vertex N 615W, bifacial) (7S1P, ~4.3kW STC, clipped to 4kW) into solar [4kW]",
		"* Results: Charge time",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected best-upgrade summary to contain %q, got=%q", expected, got)
		}
	}
	if strings.Contains(got, "EcoFlow 400W Rigid") {
		t.Fatalf("expected mixed fastest scenario to avoid slower EcoFlow 400W path, got=%q", got)
	}
}

func TestSolarRecommendationETAWUsesShoulderGainWhenClipped(t *testing.T) {
	potentialW := 500.0
	nominalW := 800.0
	maxW := 500.0
	w := solarRecommendationETAW(potentialW, nominalW, maxW, true, false)
	if w <= potentialW {
		t.Fatalf("expected shoulder-hours ETA watts > potential; got=%.2f potential=%.2f", w, potentialW)
	}
	flatMinutes, okFlat := estimateSolarChargeMinutes(1000, potentialW)
	shoulderMinutes, okShoulder := estimateSolarChargeMinutes(1000, w)
	if !okFlat || !okShoulder {
		t.Fatalf("expected ETA minutes to be available for both paths; flat_ok=%v shoulder_ok=%v", okFlat, okShoulder)
	}
	if shoulderMinutes >= flatMinutes {
		t.Fatalf("expected shoulder-hours ETA to be faster; flat=%.2f shoulder=%.2f", flatMinutes, shoulderMinutes)
	}
}

func TestAdjustedPanelLayoutComplexityDiscountsEcoFlow125Bifacial(t *testing.T) {
	base := panelLayoutComplexityScore(panelLayout{series: 1, parallel: 4, units: 4})
	got := adjustedPanelLayoutComplexity(base, "EcoFlow 125W Bifacial Modular Solar Panel", 4)
	want := base * ecoflow125ComplexityFactor
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("expected discounted complexity %.2f, got=%.2f (base=%.2f)", want, got, base)
	}
}

func TestAdjustedPanelLayoutComplexityKeepsOtherPanelsUnchanged(t *testing.T) {
	base := panelLayoutComplexityScore(panelLayout{series: 1, parallel: 4, units: 4})
	got := adjustedPanelLayoutComplexity(base, "JA Solar 400W Bifacial", 4)
	if math.Abs(got-base) > 0.01 {
		t.Fatalf("expected unchanged complexity %.2f, got=%.2f", base, got)
	}
}

func TestShouldPreferUpgradeOptionUsesEfficiencyAsTieBreaker(t *testing.T) {
	maxW := 500.0
	current := solarRecommendationOption{
		hasPotential: true,
		potentialW:   500,
		nominalW:     500,
		effPct:       20.0,
		hasEffPct:    true,
		effSrc:       "reported",
	}
	candidate := solarRecommendationOption{
		hasPotential: true,
		potentialW:   500,
		nominalW:     500,
		effPct:       24.0,
		hasEffPct:    true,
		effSrc:       "reported",
	}
	if !shouldPreferUpgradeOption(candidate, current, maxW) {
		t.Fatalf("expected higher-efficiency candidate to win tie-breaker")
	}
}

func TestShouldPreferUpgradeOptionPrefersFewerPanelsWhenNearEqualAndNearMax(t *testing.T) {
	maxW := 4000.0
	current := solarRecommendationOption{
		hasPotential: true,
		potentialW:   4000,
		nominalW:     4000,
		clipped:      false,
		units:        10,
		series:       10,
		parallel:     1,
	}
	candidate := solarRecommendationOption{
		hasPotential: true,
		potentialW:   4000,
		nominalW:     4305,
		clipped:      true,
		units:        7,
		series:       7,
		parallel:     1,
	}
	if !shouldPreferUpgradeOption(candidate, current, maxW) {
		t.Fatalf("expected fewer-panel clipped candidate to win near-equal near-max tie")
	}
}

func TestShouldPreferUpgradeOptionPrefersFewerPanelsWithSmallNearMaxGap(t *testing.T) {
	maxW := 4000.0
	current := solarRecommendationOption{
		hasPotential: true,
		potentialW:   4000,
		nominalW:     4000,
		clipped:      false,
		units:        10,
		series:       10,
		parallel:     1,
	}
	candidate := solarRecommendationOption{
		hasPotential: true,
		potentialW:   3800,
		nominalW:     4305,
		clipped:      true,
		units:        7,
		series:       7,
		parallel:     1,
	}
	if !shouldPreferUpgradeOption(candidate, current, maxW) {
		t.Fatalf("expected fewer-panel near-max candidate to win with a small effective gap")
	}
}
