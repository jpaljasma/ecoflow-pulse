package energydashboard

import "github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"

type Summary struct {
	SolarGeneratedKWh    Comparison
	LoadConsumedKWh      Comparison
	SelfSufficiencyPct   Comparison
	BatteryNetKWh        Comparison
	EstimatedValue       Comparison
	EstimatedACInputCost Comparison
}

type BatterySummary struct {
	ChargeKWh    float64
	DischargeKWh float64
	NetKWh       float64
	StartSOCPct  float64
	EndSOCPct    float64
	MinSOCPct    float64
	MaxSOCPct    float64
}

func BuildSummary(currentSeries, previousSeries telemetryquery.Series, gridPricePerKWh float64) Summary {
	currentTotals := TotalsFromSeries(currentSeries)
	previousTotals := TotalsFromSeries(previousSeries)

	return Summary{
		SolarGeneratedKWh: CompareValues(currentTotals.SolarGeneratedWh/1000, previousTotals.SolarGeneratedWh/1000),
		LoadConsumedKWh:   CompareValues(currentTotals.LoadEnergyWh/1000, previousTotals.LoadEnergyWh/1000),
		SelfSufficiencyPct: CompareValues(
			SelfSufficiencyPct(currentTotals.LoadEnergyWh, currentTotals.ACInputEnergyWh),
			SelfSufficiencyPct(previousTotals.LoadEnergyWh, previousTotals.ACInputEnergyWh),
		),
		BatteryNetKWh: CompareValues(currentTotals.BatteryNetEnergyWh/1000, previousTotals.BatteryNetEnergyWh/1000),
		EstimatedValue: CompareValues(
			EstimatedGeneratedValue(currentTotals.SolarGeneratedWh, gridPricePerKWh),
			EstimatedGeneratedValue(previousTotals.SolarGeneratedWh, gridPricePerKWh),
		),
		EstimatedACInputCost: CompareValues(
			EstimatedACInputCost(currentTotals.ACInputEnergyWh, gridPricePerKWh),
			EstimatedACInputCost(previousTotals.ACInputEnergyWh, gridPricePerKWh),
		),
	}
}

func BuildBatterySummary(series telemetryquery.Series) BatterySummary {
	totals := TotalsFromSeries(series)
	return BatterySummary{
		ChargeKWh:    totals.BatteryChargeEnergyWh / 1000,
		DischargeKWh: totals.BatteryDischargeWh / 1000,
		NetKWh:       totals.BatteryNetEnergyWh / 1000,
		StartSOCPct:  seriesStartSOC(series),
		EndSOCPct:    seriesEndSOC(series),
		MinSOCPct:    seriesMinSOC(series),
		MaxSOCPct:    seriesMaxSOC(series),
	}
}

func seriesStartSOC(series telemetryquery.Series) float64 {
	for _, point := range series.Points {
		if point.Metrics.SOCAvgPct != nil {
			return *point.Metrics.SOCAvgPct
		}
	}
	return 0
}

func seriesEndSOC(series telemetryquery.Series) float64 {
	for idx := len(series.Points) - 1; idx >= 0; idx-- {
		if series.Points[idx].Metrics.SOCAvgPct != nil {
			return *series.Points[idx].Metrics.SOCAvgPct
		}
	}
	return 0
}

func seriesMinSOC(series telemetryquery.Series) float64 {
	var (
		best  float64
		found bool
	)
	for _, point := range series.Points {
		if point.Metrics.SOCMinPct != nil {
			if !found || *point.Metrics.SOCMinPct < best {
				best = *point.Metrics.SOCMinPct
				found = true
			}
			continue
		}
		if point.Metrics.SOCAvgPct != nil && (!found || *point.Metrics.SOCAvgPct < best) {
			best = *point.Metrics.SOCAvgPct
			found = true
		}
	}
	if !found {
		return 0
	}
	return best
}

func seriesMaxSOC(series telemetryquery.Series) float64 {
	var (
		best  float64
		found bool
	)
	for _, point := range series.Points {
		if point.Metrics.SOCMaxPct != nil {
			if !found || *point.Metrics.SOCMaxPct > best {
				best = *point.Metrics.SOCMaxPct
				found = true
			}
			continue
		}
		if point.Metrics.SOCAvgPct != nil && (!found || *point.Metrics.SOCAvgPct > best) {
			best = *point.Metrics.SOCAvgPct
			found = true
		}
	}
	if !found {
		return 0
	}
	return best
}
