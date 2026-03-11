package energydashboard

import (
	"math"

	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
)

// Until explicit energy buckets are persisted for every channel, the Energy
// dashboard derives window energy from stored bucket-average power and bucket
// duration on the server side so the UI does not carry that logic.
type Totals struct {
	SolarGeneratedWh      float64
	LoadEnergyWh          float64
	ACInputEnergyWh       float64
	ACOutputEnergyWh      float64
	DCOutputEnergyWh      float64
	BatteryChargeEnergyWh float64
	BatteryDischargeWh    float64
	BatteryNetEnergyWh    float64
}

type Comparison struct {
	Current  float64
	Previous float64
	Delta    float64
	DeltaPct *float64
}

func TotalsFromSeries(series telemetryquery.Series) Totals {
	var totals Totals
	for _, point := range series.Points {
		durationHours := point.BucketEnd.Sub(point.BucketStart).Hours()
		if durationHours <= 0 {
			continue
		}

		solarWh := metricValue(point.Metrics.SolarGeneratedWh)
		if solarWh <= 0 {
			solarWh = metricValue(point.Metrics.PVAvgW) * durationHours
		}
		loadWh := metricOrDerivedEnergyValue(point.Metrics.LoadEnergyWh, point.Metrics.LoadAvgW, durationHours)
		acInputWh := metricOrDerivedEnergyValue(point.Metrics.ACInputEnergyWh, point.Metrics.ACInAvgW, durationHours)
		dcOutputWh := metricOrDerivedEnergyValue(point.Metrics.DCOutputEnergyWh, point.Metrics.DCAvgW, durationHours)
		acOutputWh := metricOrDerivedEnergyValue(point.Metrics.ACOutputEnergyWh, point.Metrics.ACOutputAvgW, durationHours)
		if acOutputWh <= 0 {
			acOutputWh = math.Max(metricValue(point.Metrics.LoadAvgW)-metricValue(point.Metrics.DCAvgW), 0) * durationHours
		}
		batteryAvgW := signedMetricValue(point.Metrics.BatteryAvgW)
		chargeWh := metricValue(point.Metrics.BatteryChargeEnergyWh)
		if chargeWh <= 0 {
			chargeWh = math.Max(batteryAvgW, 0) * durationHours
		}
		dischargeWh := metricValue(point.Metrics.BatteryDischargeEnergyWh)
		if dischargeWh <= 0 {
			dischargeWh = math.Max(-batteryAvgW, 0) * durationHours
		}

		totals.SolarGeneratedWh += solarWh
		totals.LoadEnergyWh += loadWh
		totals.ACInputEnergyWh += acInputWh
		totals.ACOutputEnergyWh += acOutputWh
		totals.DCOutputEnergyWh += dcOutputWh
		totals.BatteryChargeEnergyWh += chargeWh
		totals.BatteryDischargeWh += dischargeWh
	}
	totals.BatteryNetEnergyWh = totals.BatteryChargeEnergyWh - totals.BatteryDischargeWh
	return totals
}

func metricOrDerivedEnergyValue(explicit, avgPower *float64, durationHours float64) float64 {
	value := metricValue(explicit)
	if value > 0 {
		return value
	}
	return metricValue(avgPower) * durationHours
}

func CompareValues(current, previous float64) Comparison {
	comparison := Comparison{
		Current:  current,
		Previous: previous,
		Delta:    current - previous,
	}
	if previous > 0 {
		deltaPct := ((current - previous) / previous) * 100
		comparison.DeltaPct = &deltaPct
	}
	return comparison
}

func SelfSufficiencyPct(loadEnergyWh, acInputEnergyWh float64) float64 {
	return clampPercent(((loadEnergyWh - acInputEnergyWh) / math.Max(loadEnergyWh, 1)) * 100)
}

func EstimatedGeneratedValue(solarGeneratedWh, gridPricePerKWh float64) float64 {
	return math.Max(solarGeneratedWh, 0) / 1000 * gridPricePerKWh
}

func EstimatedACInputCost(acInputEnergyWh, gridPricePerKWh float64) float64 {
	return acInputEnergyWh / 1000 * gridPricePerKWh
}

func metricValue(value *float64) float64 {
	if value == nil || !isFinite(*value) {
		return 0
	}
	return math.Max(*value, 0)
}

func signedMetricValue(value *float64) float64 {
	if value == nil || !isFinite(*value) {
		return 0
	}
	return *value
}

func clampPercent(value float64) float64 {
	switch {
	case !isFinite(value):
		return 0
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
