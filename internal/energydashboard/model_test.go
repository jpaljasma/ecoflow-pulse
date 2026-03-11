package energydashboard

import (
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
)

func TestBuildSummaryAndBatterySummary(t *testing.T) {
	t.Parallel()

	current := telemetryquery.Series{
		Points: []telemetryquery.Point{
			{
				BucketStart: time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC),
				BucketEnd:   time.Date(2026, time.March, 11, 13, 0, 0, 0, time.UTC),
				Metrics: telemetryquery.Metrics{
					SolarGeneratedWh: floatPtr(400),
					LoadAvgW:         floatPtr(300),
					ACInAvgW:         floatPtr(90),
					DCAvgW:           floatPtr(75),
					BatteryAvgW:      floatPtr(50),
					SOCAvgPct:        floatPtr(42),
					SOCMinPct:        floatPtr(40),
					SOCMaxPct:        floatPtr(45),
				},
			},
			{
				BucketStart: time.Date(2026, time.March, 11, 13, 0, 0, 0, time.UTC),
				BucketEnd:   time.Date(2026, time.March, 11, 14, 0, 0, 0, time.UTC),
				Metrics: telemetryquery.Metrics{
					PVAvgW:      floatPtr(250),
					LoadAvgW:    floatPtr(200),
					ACInAvgW:    floatPtr(40),
					DCAvgW:      floatPtr(60),
					BatteryAvgW: floatPtr(-70),
					SOCAvgPct:   floatPtr(44),
					SOCMinPct:   floatPtr(39),
					SOCMaxPct:   floatPtr(46),
				},
			},
		},
	}
	previous := telemetryquery.Series{
		Points: []telemetryquery.Point{
			{
				BucketStart: time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC),
				BucketEnd:   time.Date(2026, time.March, 10, 13, 0, 0, 0, time.UTC),
				Metrics: telemetryquery.Metrics{
					SolarGeneratedWh: floatPtr(200),
					LoadAvgW:         floatPtr(250),
					ACInAvgW:         floatPtr(100),
					DCAvgW:           floatPtr(50),
					BatteryAvgW:      floatPtr(-20),
				},
			},
		},
	}

	summary := BuildSummary(current, previous, 0.30)
	if got := summary.SolarGeneratedKWh.Current; got != 0.65 {
		t.Fatalf("SolarGeneratedKWh current mismatch: got=%v want=0.65", got)
	}
	if got := summary.LoadConsumedKWh.Current; got != 0.5 {
		t.Fatalf("LoadConsumedKWh current mismatch: got=%v want=0.5", got)
	}
	if got := summary.BatteryNetKWh.Current; got != -0.02 {
		t.Fatalf("BatteryNetKWh current mismatch: got=%v want=-0.02", got)
	}
	if got := summary.EstimatedValue.Current; got != 0.195 {
		t.Fatalf("EstimatedValue current mismatch: got=%v want=0.195", got)
	}
	if got := summary.EstimatedValue.Previous; got != 0.06 {
		t.Fatalf("EstimatedValue previous mismatch: got=%v want=0.06", got)
	}
	if summary.SelfSufficiencyPct.DeltaPct == nil {
		t.Fatalf("expected self-sufficiency delta pct")
	}

	battery := BuildBatterySummary(current)
	if battery.ChargeKWh != 0.05 {
		t.Fatalf("ChargeKWh mismatch: got=%v want=0.05", battery.ChargeKWh)
	}
	if battery.DischargeKWh != 0.07 {
		t.Fatalf("DischargeKWh mismatch: got=%v want=0.07", battery.DischargeKWh)
	}
	if battery.NetKWh != -0.02 {
		t.Fatalf("NetKWh mismatch: got=%v want=-0.02", battery.NetKWh)
	}
	if battery.StartSOCPct != 42 {
		t.Fatalf("StartSOCPct mismatch: got=%v want=42", battery.StartSOCPct)
	}
	if battery.EndSOCPct != 44 {
		t.Fatalf("EndSOCPct mismatch: got=%v want=44", battery.EndSOCPct)
	}
	if battery.MinSOCPct != 39 {
		t.Fatalf("MinSOCPct mismatch: got=%v want=39", battery.MinSOCPct)
	}
	if battery.MaxSOCPct != 46 {
		t.Fatalf("MaxSOCPct mismatch: got=%v want=46", battery.MaxSOCPct)
	}
}
