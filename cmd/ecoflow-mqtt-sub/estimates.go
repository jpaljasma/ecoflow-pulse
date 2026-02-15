package main

import (
	"fmt"
	"math"
)

func estimatedWhLeft(packs map[int]*packSnapshot) float64 {
	total := 0.0
	for _, pack := range packs {
		if !pack.HasEnergy || !pack.HasSOC {
			continue
		}
		total += pack.EnergyWh * (pack.SOC / 100.0)
	}
	return total
}

type batteryETAEstimates struct {
	ChargeValue     string
	DischargeValue  string
	ActiveValue     string
	PowerValue      string
	ConfidenceValue string
}

func (s *energySnapshot) estimateBatteryETAs(
	state systemStateKind,
	batteryInWatts float64,
	hasBatteryIn bool,
	batteryOutWatts float64,
	hasBatteryOut bool,
	effectiveIn float64,
	hasEffectiveIn bool,
	effectiveOut float64,
	hasEffectiveOut bool,
) batteryETAEstimates {
	estimates := batteryETAEstimates{
		ChargeValue:     "n/a",
		DischargeValue:  "n/a",
		ActiveValue:     "n/a",
		PowerValue:      "power: n/a",
		ConfidenceValue: "n/a",
	}

	energyToChargeWh, energyToDischargeWh, ok := s.energyToTargetsWh()
	if !ok {
		return estimates
	}

	// Prefer net system power when available; raw battery amp/vol telemetry can be scaled
	// inconsistently on some payloads and produce impossible ETA rates.
	chargePowerW := 0.0
	hasChargePower := false
	chargePowerSource := ""
	if hasEffectiveIn && hasEffectiveOut {
		netPowerW := effectiveIn - effectiveOut
		if netPowerW > systemStateNetThresholdWatts {
			chargePowerW = netPowerW
			hasChargePower = true
			chargePowerSource = "net"
		}
	}
	if packChargeW, _ := packPowerTotals(s.Packs); packChargeW > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(packChargeW); ok {
			if !hasChargePower || sanitized > chargePowerW {
				chargePowerW = sanitized
				hasChargePower = true
				chargePowerSource = "pack"
			}
		}
	}
	if !hasChargePower && hasBatteryIn && batteryInWatts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(batteryInWatts); ok {
			chargePowerW = sanitized
			hasChargePower = true
			chargePowerSource = "hint"
		}
	}

	dischargePowerW := 0.0
	hasDischargePower := false
	dischargePowerSource := ""
	if hasEffectiveIn && hasEffectiveOut {
		netPowerW := effectiveOut - effectiveIn
		if netPowerW > systemStateNetThresholdWatts {
			dischargePowerW = netPowerW
			hasDischargePower = true
			dischargePowerSource = "net"
		}
	}
	if _, packDischargeW := packPowerTotals(s.Packs); packDischargeW > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(packDischargeW); ok {
			if !hasDischargePower || sanitized > dischargePowerW {
				dischargePowerW = sanitized
				hasDischargePower = true
				dischargePowerSource = "pack"
			}
		}
	}
	if !hasDischargePower && hasBatteryOut && batteryOutWatts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(batteryOutWatts); ok {
			dischargePowerW = sanitized
			hasDischargePower = true
			dischargePowerSource = "hint"
		}
	}

	if hasChargePower {
		if energyToChargeWh <= 0 {
			estimates.ChargeValue = "0min (~0m)"
		} else {
			etaChargeMin := energyToChargeWh * 60.0 / chargePowerW
			estimates.ChargeValue = formatETAMinutes(etaChargeMin)
		}
	}
	if hasDischargePower {
		if energyToDischargeWh <= 0 {
			estimates.DischargeValue = "0min (~0m)"
		} else {
			etaDischargeMin := energyToDischargeWh * 60.0 / dischargePowerW
			estimates.DischargeValue = formatETAMinutes(etaDischargeMin)
		}
	}

	switch {
	case hasChargePower && hasDischargePower:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s dsg@%s", formatWatts(chargePowerW), formatWatts(dischargePowerW))
	case hasChargePower:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s", formatWatts(chargePowerW))
	case hasDischargePower:
		estimates.PowerValue = fmt.Sprintf("power: dsg@%s", formatWatts(dischargePowerW))
	}

	switch state {
	case systemStateCharging:
		estimates.ActiveValue = estimates.ChargeValue
	case systemStateDischarging:
		estimates.ActiveValue = estimates.DischargeValue
	default:
		estimates.ActiveValue = "n/a"
	}

	confidenceScore := 0.0
	hasConfidence := false
	switch state {
	case systemStateCharging:
		if hasChargePower {
			confidenceScore = etaConfidenceScoreForSource(chargePowerSource)
			hasConfidence = true
			if hasEffectiveIn && hasEffectiveOut {
				confidenceScore += 0.08
			}
			if chargePowerW <= 20 {
				confidenceScore -= 0.08
			}
		}
	case systemStateDischarging:
		if hasDischargePower {
			confidenceScore = etaConfidenceScoreForSource(dischargePowerSource)
			hasConfidence = true
			if hasEffectiveIn && hasEffectiveOut {
				confidenceScore += 0.08
			}
			if dischargePowerW <= 20 {
				confidenceScore -= 0.08
			}
		}
	default:
		// When state is unknown, confidence is inherently lower even if we have power.
		if hasChargePower || hasDischargePower {
			confidenceScore = 0.45
			hasConfidence = true
		}
	}
	estimates.ConfidenceValue = formatConfidenceValue(confidenceScore, hasConfidence)

	return estimates
}

func (s *energySnapshot) energyToTargetsWh() (energyToChargeWh, energyToDischargeWh float64, ok bool) {
	if s == nil {
		return 0, 0, false
	}
	capacityWh, hasCapacity := s.estimatedTotalCapacityWh()
	if !hasCapacity || capacityWh <= 0 {
		return 0, 0, false
	}
	remainingWh, hasRemaining := s.estimatedRemainingEnergyWh()
	if !hasRemaining || remainingWh < 0 {
		return 0, 0, false
	}

	minSOC, maxSOC := 0.0, 100.0
	if s.HasMinDischarge {
		minSOC = clampPercent(s.MinDischargeSOC)
	}
	if s.HasMaxChargeSOC {
		maxSOC = clampPercent(s.MaxChargeSOC)
	}
	if maxSOC < minSOC {
		minSOC, maxSOC = 0, 100
	}

	targetChargeWh := capacityWh * (maxSOC / 100.0)
	targetDischargeWh := capacityWh * (minSOC / 100.0)

	energyToChargeWh = targetChargeWh - remainingWh
	if energyToChargeWh < 0 {
		energyToChargeWh = 0
	}
	energyToDischargeWh = remainingWh - targetDischargeWh
	if energyToDischargeWh < 0 {
		energyToDischargeWh = 0
	}
	return energyToChargeWh, energyToDischargeWh, true
}

func estimateBatteryETAsML(snapshot *energySnapshot, history *minuteTelemetryHistory, state systemStateKind) batteryETAEstimates {
	estimates := batteryETAEstimates{
		ChargeValue:     "n/a",
		DischargeValue:  "n/a",
		ActiveValue:     "n/a",
		PowerValue:      "power: n/a",
		ConfidenceValue: "n/a",
	}
	if snapshot == nil {
		return estimates
	}
	energyToChargeWh, energyToDischargeWh, ok := snapshot.energyToTargetsWh()
	if !ok {
		return estimates
	}

	samples := netPowerSamplesFromMinuteHistory(history, 24)
	if len(samples) < 3 {
		return estimates
	}
	predNetW, meanNetW, stdNetW, ok := predictNetPowerEWMATrend(samples)
	if !ok {
		return estimates
	}

	chargePowerW := 0.0
	hasChargePower := false
	if predNetW > systemStateNetThresholdWatts {
		chargePowerW = predNetW
		hasChargePower = true
	} else if state == systemStateCharging && meanNetW > systemStateNetThresholdWatts {
		chargePowerW = meanNetW
		hasChargePower = true
	}
	if hasChargePower {
		if sanitized, ok := snapshot.sanitizeBatteryFlowHintWatts(chargePowerW); ok {
			chargePowerW = sanitized
		} else {
			hasChargePower = false
		}
	}

	dischargePowerW := 0.0
	hasDischargePower := false
	if predNetW < -systemStateNetThresholdWatts {
		dischargePowerW = -predNetW
		hasDischargePower = true
	} else if state == systemStateDischarging && meanNetW < -systemStateNetThresholdWatts {
		dischargePowerW = -meanNetW
		hasDischargePower = true
	}
	if hasDischargePower {
		if sanitized, ok := snapshot.sanitizeBatteryFlowHintWatts(dischargePowerW); ok {
			dischargePowerW = sanitized
		} else {
			hasDischargePower = false
		}
	}

	if hasChargePower {
		if energyToChargeWh <= 0 {
			estimates.ChargeValue = "0min (~0m)"
		} else {
			etaChargeMin := energyToChargeWh * 60.0 / chargePowerW
			estimates.ChargeValue = formatETAMinutes(etaChargeMin)
		}
	}
	if hasDischargePower {
		if energyToDischargeWh <= 0 {
			estimates.DischargeValue = "0min (~0m)"
		} else {
			etaDischargeMin := energyToDischargeWh * 60.0 / dischargePowerW
			estimates.DischargeValue = formatETAMinutes(etaDischargeMin)
		}
	}

	switch {
	case hasChargePower && hasDischargePower:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s dsg@%s (ewma+trend)", formatWatts(chargePowerW), formatWatts(dischargePowerW))
	case hasChargePower:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s (ewma+trend)", formatWatts(chargePowerW))
	case hasDischargePower:
		estimates.PowerValue = fmt.Sprintf("power: dsg@%s (ewma+trend)", formatWatts(dischargePowerW))
	}

	switch state {
	case systemStateCharging:
		estimates.ActiveValue = estimates.ChargeValue
	case systemStateDischarging:
		estimates.ActiveValue = estimates.DischargeValue
	default:
		estimates.ActiveValue = "n/a"
	}

	signMatchRatio := 0.0
	switch state {
	case systemStateCharging:
		signMatchRatio = signDirectionMatchRatio(samples, true)
	case systemStateDischarging:
		signMatchRatio = signDirectionMatchRatio(samples, false)
	default:
		signMatchRatio = 0.5
	}
	// Warm up 2x faster: reach full sample-confidence contribution in ~3 samples
	// instead of ~6 while keeping the same maximum weight.
	sampleScore := math.Min(float64(len(samples))/3.0, 1.0) * 0.42
	stabilityScore := 0.0
	if math.Abs(meanNetW) > systemStateNetThresholdWatts {
		cv := stdNetW / math.Abs(meanNetW)
		if cv < 0 {
			cv = 0
		}
		// Softer volatility penalty: tolerate transient swings without collapsing confidence.
		if cv > 1.8 {
			cv = 1.8
		}
		stabilityScore = (1 - (cv / 1.8)) * 0.24
	}
	confidenceScore := 0.25 + sampleScore + (signMatchRatio * 0.20) + stabilityScore
	if !hasChargePower && !hasDischargePower {
		confidenceScore -= 0.12
	}
	if state == systemStateUnknown {
		confidenceScore -= 0.05
	}
	estimates.ConfidenceValue = formatConfidenceValue(confidenceScore, true)

	return estimates
}

func netPowerSamplesFromMinuteHistory(history *minuteTelemetryHistory, limit int) []float64 {
	if history == nil {
		return nil
	}
	buckets := history.SortedBuckets(false, limit)
	if len(buckets) == 0 {
		return nil
	}
	out := make([]float64, 0, len(buckets))
	for _, bucket := range buckets {
		inW := 0.0
		hasIn := false
		if bucket.SolarSamples > 0 {
			inW += bucket.SolarSumWatts / float64(bucket.SolarSamples)
			hasIn = true
		}
		if bucket.ACInSamples > 0 {
			inW += bucket.ACInSumWatts / float64(bucket.ACInSamples)
			hasIn = true
		}

		outW := 0.0
		hasOut := false
		if bucket.ACOutSamples > 0 {
			outW += bucket.ACOutSumWatts / float64(bucket.ACOutSamples)
			hasOut = true
		}
		if bucket.DCOutSamples > 0 {
			outW += bucket.DCOutSumWatts / float64(bucket.DCOutSamples)
			hasOut = true
		}
		if !hasIn && !hasOut {
			continue
		}
		out = append(out, inW-outW)
	}
	return out
}

func predictNetPowerEWMATrend(samples []float64) (pred float64, mean float64, std float64, ok bool) {
	if len(samples) < 2 {
		return 0, 0, 0, false
	}
	const alpha = 0.35
	ewma := samples[0]
	for i := 1; i < len(samples); i++ {
		ewma = (alpha * samples[i]) + ((1 - alpha) * ewma)
	}

	n := float64(len(samples))
	xMean := (n - 1) / 2
	yMean := 0.0
	for _, sample := range samples {
		yMean += sample
	}
	yMean /= n

	num := 0.0
	den := 0.0
	for i, sample := range samples {
		x := float64(i) - xMean
		y := sample - yMean
		num += x * y
		den += x * x
	}
	slope := 0.0
	if den > 0 {
		slope = num / den
	}
	pred = ewma + slope
	if math.Abs(pred) < systemStateNetThresholdWatts && math.Abs(ewma) >= systemStateNetThresholdWatts {
		pred = ewma
	}

	variance := 0.0
	for _, sample := range samples {
		delta := sample - yMean
		variance += delta * delta
	}
	variance /= n
	std = math.Sqrt(variance)
	return pred, yMean, std, true
}

func signDirectionMatchRatio(samples []float64, charging bool) float64 {
	if len(samples) == 0 {
		return 0
	}
	match := 0
	for _, sample := range samples {
		if charging {
			if sample > systemStateNetThresholdWatts {
				match++
			}
		} else if sample < -systemStateNetThresholdWatts {
			match++
		}
	}
	return float64(match) / float64(len(samples))
}

func (s *energySnapshot) estimatedTotalCapacityWh() (float64, bool) {
	if s == nil {
		return 0, false
	}
	if s.HasFullEnergy && s.FullEnergyWh > 0 {
		return s.FullEnergyWh, true
	}
	total := 0.0
	count := 0
	for _, pack := range s.Packs {
		if packWh, ok := estimatedPackCapacityWh(pack); ok {
			total += packWh
			count++
		}
	}
	if count == 0 || total <= 0 {
		return 0, false
	}
	return total, true
}

func (s *energySnapshot) estimatedRemainingEnergyWh() (float64, bool) {
	if s == nil {
		return 0, false
	}
	total := 0.0
	count := 0
	for _, pack := range s.Packs {
		if packWh, ok := estimatedPackRemainingWh(pack); ok {
			total += packWh
			count++
		}
	}
	if count > 0 && total >= 0 {
		return total, true
	}

	capacityWh, hasCapacity := s.estimatedTotalCapacityWh()
	if !hasCapacity || capacityWh <= 0 {
		return 0, false
	}
	if soc, ok := s.displaySOC(); ok {
		return capacityWh * (clampPercent(soc) / 100.0), true
	}
	return 0, false
}

func estimatedPackCapacityWh(pack *packSnapshot) (float64, bool) {
	if pack == nil {
		return 0, false
	}
	if pack.HasEnergy && pack.EnergyWh > 0 {
		return pack.EnergyWh, true
	}
	if wh, ok := capacityToWh(pack.FullCap, pack.HasFullCap, pack.VoltageV, pack.HasVoltage); ok {
		return wh, true
	}
	if wh, ok := capacityToWh(pack.DesignCap, pack.HasDesignCap, pack.VoltageV, pack.HasVoltage); ok {
		return wh, true
	}
	return 0, false
}

func estimatedPackRemainingWh(pack *packSnapshot) (float64, bool) {
	if pack == nil {
		return 0, false
	}
	if wh, ok := capacityToWh(pack.RemainCap, pack.HasRemainCap, pack.VoltageV, pack.HasVoltage); ok {
		return wh, true
	}
	if capWh, ok := estimatedPackCapacityWh(pack); ok && pack.HasSOC {
		return capWh * (clampPercent(pack.SOC) / 100.0), true
	}
	return 0, false
}

func etaConfidenceScoreForSource(source string) float64 {
	switch source {
	case "net":
		return 0.88
	case "pack":
		return 0.8
	case "hint":
		return 0.65
	default:
		return 0
	}
}

func formatConfidenceValue(score float64, ok bool) string {
	if !ok || math.IsNaN(score) || math.IsInf(score, 0) {
		return "n/a"
	}
	if score < 0 {
		score = 0
	}
	if score > 0.99 {
		score = 0.99
	}
	return fmt.Sprintf("%.2f (%s)", score, confidenceTier(score))
}

func confidenceTier(score float64) string {
	switch {
	case score >= 0.8:
		return "high"
	case score >= 0.6:
		return "medium"
	default:
		return "low"
	}
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
