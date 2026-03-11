package energydashboard

import (
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
)

func TestTotalsFromSeriesDerivesWindowEnergyFromAveragePower(t *testing.T) {
	t.Parallel()

	series := telemetryquery.Series{
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
				},
			},
		},
	}

	totals := TotalsFromSeries(series)

	if totals.SolarGeneratedWh != 650 {
		t.Fatalf("SolarGeneratedWh mismatch: got=%v want=650", totals.SolarGeneratedWh)
	}
	if totals.LoadEnergyWh != 500 {
		t.Fatalf("LoadEnergyWh mismatch: got=%v want=500", totals.LoadEnergyWh)
	}
	if totals.ACInputEnergyWh != 130 {
		t.Fatalf("ACInputEnergyWh mismatch: got=%v want=130", totals.ACInputEnergyWh)
	}
	if totals.DCOutputEnergyWh != 135 {
		t.Fatalf("DCOutputEnergyWh mismatch: got=%v want=135", totals.DCOutputEnergyWh)
	}
	if totals.ACOutputEnergyWh != 365 {
		t.Fatalf("ACOutputEnergyWh mismatch: got=%v want=365", totals.ACOutputEnergyWh)
	}
	if totals.BatteryChargeEnergyWh != 50 {
		t.Fatalf("BatteryChargeEnergyWh mismatch: got=%v want=50", totals.BatteryChargeEnergyWh)
	}
	if totals.BatteryDischargeWh != 70 {
		t.Fatalf("BatteryDischargeWh mismatch: got=%v want=70", totals.BatteryDischargeWh)
	}
	if totals.BatteryNetEnergyWh != -20 {
		t.Fatalf("BatteryNetEnergyWh mismatch: got=%v want=-20", totals.BatteryNetEnergyWh)
	}
}

func TestCompareValues(t *testing.T) {
	t.Parallel()

	comparison := CompareValues(125, 100)
	if comparison.Delta != 25 {
		t.Fatalf("Delta mismatch: got=%v want=25", comparison.Delta)
	}
	if comparison.DeltaPct == nil || *comparison.DeltaPct != 25 {
		t.Fatalf("DeltaPct mismatch: got=%v want=25", comparison.DeltaPct)
	}

	noBaseline := CompareValues(50, 0)
	if noBaseline.DeltaPct != nil {
		t.Fatalf("expected nil DeltaPct without positive baseline, got=%v", *noBaseline.DeltaPct)
	}
}

func TestSummaryFormulas(t *testing.T) {
	t.Parallel()

	if got, want := SelfSufficiencyPct(600, 100), (500.0/600.0)*100; mathAbs(got-want) > 1e-9 {
		t.Fatalf("SelfSufficiencyPct mismatch: got=%v want=%v", got, want)
	}
	if got, want := EstimatedGeneratedValue(600, 0.35), 0.21; mathAbs(got-want) > 1e-9 {
		t.Fatalf("EstimatedGeneratedValue mismatch: got=%v want=%v", got, want)
	}
	if got, want := EstimatedACInputCost(100, 0.35), 0.035; mathAbs(got-want) > 1e-9 {
		t.Fatalf("EstimatedACInputCost mismatch: got=%v want=%v", got, want)
	}
}

func floatPtr(value float64) *float64 {
	v := value
	return &v
}

func mathAbs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
