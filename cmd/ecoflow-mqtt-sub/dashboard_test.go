package main

import (
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
		"channels",
		"ac: n/a pv_total: n/a xt150_in: n/a",
		"ac: n/a (l14: n/a) dc: n/a xt150_out: n/a",
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
		"est ML",
		"charge: n/a",
		"discharge: n/a",
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
		minuteTableConfig{},
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
		name     string
		state    systemStateKind
		acInW    float64
		hasACIn  bool
		acOutW   float64
		hasACOut bool
		pvInW    float64
		hasPVIn  bool
		want     string
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

			derived := snapshotDerived{
				SystemStateValue: string(tt.state),
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
		minuteTableConfig{},
	)

	if !strings.Contains(output, "pv_total: 130.0W (~110.0W avg)") {
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
		minuteTableConfig{},
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
		minuteTableConfig{},
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
