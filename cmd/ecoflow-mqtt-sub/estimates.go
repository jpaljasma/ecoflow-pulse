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

	samples := netPowerSamplesFromPowerHistory(snapshot.mlFastHistory, 90)
	if len(samples) < 2 {
		samples = netPowerSamplesFromMinuteHistory(history, 24)
	}
	if len(samples) < 2 {
		return estimates
	}
	samples = adaptMLPredictionSamples(samples)
	predNetW, meanNetW, stdNetW, ok := predictNetPowerEWMATrend(samples)
	if !ok {
		return estimates
	}

	chargePowerW := 0.0
	hasChargePower := false
	chargeEtaMinutes := 0.0
	hasChargeETA := false
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
	dischargeEtaMinutes := 0.0
	hasDischargeETA := false
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
			chargeEtaMinutes = 0
			hasChargeETA = true
		} else {
			etaChargeMin := energyToChargeWh * 60.0 / chargePowerW
			chargeEtaMinutes = etaChargeMin
			hasChargeETA = true
			estimates.ChargeValue = formatETAMinutes(etaChargeMin)
		}
	}
	if hasDischargePower {
		if energyToDischargeWh <= 0 {
			estimates.DischargeValue = "0min (~0m)"
			dischargeEtaMinutes = 0
			hasDischargeETA = true
		} else {
			etaDischargeMin := energyToDischargeWh * 60.0 / dischargePowerW
			dischargeEtaMinutes = etaDischargeMin
			hasDischargeETA = true
			estimates.DischargeValue = formatETAMinutes(etaDischargeMin)
		}
	}

	switch state {
	case systemStateCharging:
		estimates.ActiveValue = estimates.ChargeValue
	case systemStateDischarging:
		estimates.ActiveValue = estimates.DischargeValue
	default:
		estimates.ActiveValue = "n/a"
	}

	signMatchRatio := 0.5
	recentSignMatchRatio := 0.5
	directionDeadband := adaptiveDirectionDeadband(stdNetW)
	recentSamples := lastNSamples(samples, 6)
	switch state {
	case systemStateCharging:
		signMatchRatio = signDirectionMatchRatio(samples, true, directionDeadband)
		recentSignMatchRatio = signDirectionMatchRatio(recentSamples, true, directionDeadband)
	case systemStateDischarging:
		signMatchRatio = signDirectionMatchRatio(samples, false, directionDeadband)
		recentSignMatchRatio = signDirectionMatchRatio(recentSamples, false, directionDeadband)
	default:
		signMatchRatio = 0.5
		recentSignMatchRatio = 0.5
	}
	// Multi-feature confidence scoring:
	// - sample sufficiency
	// - directional agreement (global + recent)
	// - adaptive volatility normalized by an effective net baseline
	sampleScore := math.Min(float64(len(samples))/6.0, 1.0) * 0.34
	stabilityScore := 0.0
	effectiveMeanNet := math.Max(math.Abs(meanNetW), systemStateNetThresholdWatts*2.0)
	if effectiveMeanNet > 0 {
		cv := stdNetW / effectiveMeanNet
		if cv < 0 {
			cv = 0
		}
		// Normalize with a gentler cap so low-power operation does not get over-penalized.
		if cv > 2.0 {
			cv = 2.0
		}
		stabilityScore = (1 - (cv / 2.0)) * 0.18
	}
	recentDirectionScore := recentSignMatchRatio * 0.10
	confidenceScore := 0.22 + sampleScore + (signMatchRatio * 0.20) + stabilityScore + recentDirectionScore
	if predNetW*meanNetW < 0 && math.Abs(predNetW) > systemStateNetThresholdWatts && math.Abs(meanNetW) > systemStateNetThresholdWatts {
		confidenceScore -= 0.06
	}
	if !hasChargePower && !hasDischargePower {
		confidenceScore -= 0.16
	}
	if state == systemStateUnknown {
		confidenceScore -= 0.03
	}

	// Cross-check ML ETA against device-reported remain time and pull the ML ETA
	// closer if it diverges too far for the current state.
	if snapshot != nil && state != systemStateUnknown {
		if deviceRemainMin, _, hasDeviceRemain := snapshot.selectRemainForState(state); hasDeviceRemain && deviceRemainMin > 0 {
			deviceRemainFloat := float64(deviceRemainMin)
			mlActiveMinutes := 0.0
			hasMLActiveMinutes := false
			switch state {
			case systemStateCharging:
				if hasChargeETA {
					mlActiveMinutes = chargeEtaMinutes
					hasMLActiveMinutes = true
				}
			case systemStateDischarging:
				if hasDischargeETA {
					mlActiveMinutes = dischargeEtaMinutes
					hasMLActiveMinutes = true
				}
			}
			if hasMLActiveMinutes && mlActiveMinutes > 0 {
				ratio := mlActiveMinutes / deviceRemainFloat
				if ratio < 1 {
					ratio = 1 / ratio
				}

				// Correct severe divergence by blending toward device ETA.
				if ratio > 1.35 {
					blend := math.Min((ratio-1.35)/1.65, 0.75)
					corrected := (1.0-blend)*mlActiveMinutes + blend*deviceRemainFloat
					if corrected > 0 {
						switch state {
						case systemStateCharging:
							chargeEtaMinutes = corrected
							estimates.ChargeValue = formatETAMinutes(corrected)
							estimates.ActiveValue = estimates.ChargeValue
							if energyToChargeWh > 0 {
								chargePowerW = energyToChargeWh * 60.0 / corrected
								hasChargePower = chargePowerW > systemStateNetThresholdWatts
							}
						case systemStateDischarging:
							dischargeEtaMinutes = corrected
							estimates.DischargeValue = formatETAMinutes(corrected)
							estimates.ActiveValue = estimates.DischargeValue
							if energyToDischargeWh > 0 {
								dischargePowerW = energyToDischargeWh * 60.0 / corrected
								hasDischargePower = dischargePowerW > systemStateNetThresholdWatts
							}
						}
					}
				}

				switch {
				case ratio > 3.0:
					confidenceScore -= 0.30
				case ratio > 2.2:
					confidenceScore -= 0.20
				case ratio > 1.6:
					confidenceScore -= 0.11
				case ratio < 1.2:
					confidenceScore += 0.03
				}
			}
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
		if bucket.BatteryNetSamples > 0 {
			out = append(out, bucket.BatteryNetSumWatts/float64(bucket.BatteryNetSamples))
			continue
		}
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

func netPowerSamplesFromPowerHistory(history *powerTelemetryHistory, limit int) []float64 {
	if history == nil {
		return nil
	}
	buckets := history.SortedBuckets(false, limit)
	if len(buckets) == 0 {
		return nil
	}
	out := make([]float64, 0, len(buckets))
	for _, bucket := range buckets {
		if bucket.BatteryNetSamples > 0 {
			out = append(out, bucket.BatteryNetSumWatts/float64(bucket.BatteryNetSamples))
			continue
		}
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

func adaptMLPredictionSamples(samples []float64) []float64 {
	if len(samples) == 0 {
		return nil
	}

	if len(samples) >= 12 {
		recent := lastNSamples(samples, 6)
		prev := samples[len(samples)-12 : len(samples)-6]
		recentMean := average(recent)
		prevMean := average(prev)
		recentSpan := math.Abs(recent[len(recent)-1] - recent[0])
		jump := math.Abs(recentMean - prevMean)
		baseline := math.Max(
			systemStateNetThresholdWatts*2.0,
			math.Max(math.Abs(prevMean), averageAbs(samples)*0.45),
		)
		signFlip := recentMean*prevMean < 0 &&
			math.Abs(recentMean) > systemStateNetThresholdWatts &&
			math.Abs(prevMean) > (systemStateNetThresholdWatts*0.75)

		if signFlip || jump > baseline*0.45 || recentSpan > baseline*0.75 {
			const fastWindow = 24
			if len(samples) > fastWindow {
				return lastNSamples(samples, fastWindow)
			}
			return samples
		}
	}

	// Keep default history bounded so stale data does not dominate prediction.
	const stableWindow = 45
	if len(samples) > stableWindow {
		samples = lastNSamples(samples, stableWindow)
	}
	if len(samples) < 12 {
		return samples
	}

	recent := lastNSamples(samples, 6)
	prev := samples[len(samples)-12 : len(samples)-6]
	recentMean := average(recent)
	prevMean := average(prev)
	recentSpan := math.Abs(recent[len(recent)-1] - recent[0])
	jump := math.Abs(recentMean - prevMean)
	baseline := math.Max(
		systemStateNetThresholdWatts*2.0,
		math.Max(math.Abs(prevMean), averageAbs(samples)*0.45),
	)
	signFlip := recentMean*prevMean < 0 &&
		math.Abs(recentMean) > systemStateNetThresholdWatts &&
		math.Abs(prevMean) > (systemStateNetThresholdWatts*0.75)

	if signFlip || jump > baseline*0.45 || recentSpan > baseline*0.75 {
		const fastWindow = 24
		if len(samples) > fastWindow {
			return lastNSamples(samples, fastWindow)
		}
	}
	return samples
}

func predictNetPowerEWMATrend(samples []float64) (pred float64, mean float64, std float64, ok bool) {
	if len(samples) < 2 {
		return 0, 0, 0, false
	}
	alpha := 0.42
	if len(samples) >= 4 {
		recent := lastNSamples(samples, 4)
		recentSpan := math.Abs(recent[len(recent)-1] - recent[0])
		baseline := math.Max(systemStateNetThresholdWatts*2.0, averageAbs(samples)*0.45)
		switch {
		case recentSpan > baseline*1.0:
			alpha = 0.78
		case recentSpan > baseline*0.55:
			alpha = 0.62
		case recentSpan > baseline*0.30:
			alpha = 0.52
		}
	}
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
	if len(samples) >= 4 {
		recentMean := average(lastNSamples(samples, 4))
		if math.Abs(recentMean) >= systemStateNetThresholdWatts {
			// Fast reaction to abrupt ramps/sign flips: bias prediction toward recent short-window behavior.
			if pred*recentMean < 0 {
				pred = recentMean
			} else if math.Abs(recentMean) > math.Abs(pred)*1.10 {
				pred = (0.20 * pred) + (0.80 * recentMean)
			} else {
				pred = (0.45 * pred) + (0.55 * recentMean)
			}
		}
		lastDelta := math.Abs(samples[len(samples)-1] - samples[len(samples)-2])
		lastDeltaBaseline := math.Max(systemStateNetThresholdWatts*1.8, math.Abs(recentMean)*0.35)
		if lastDelta > lastDeltaBaseline {
			pred = (0.65 * pred) + (0.35 * samples[len(samples)-1])
		}
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

func signDirectionMatchRatio(samples []float64, charging bool, deadband float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	deadband = math.Abs(deadband)
	if deadband < systemStateNetThresholdWatts*0.35 {
		deadband = systemStateNetThresholdWatts * 0.35
	}
	maxDeadband := systemStateNetThresholdWatts * 1.2
	if deadband > maxDeadband {
		deadband = maxDeadband
	}
	score := 0.0
	for _, sample := range samples {
		if math.Abs(sample) <= deadband {
			// Near-zero/noise samples are neutral rather than direction mismatches.
			score += 0.5
			continue
		}
		if charging {
			if sample > deadband {
				score += 1.0
			}
		} else if sample < -deadband {
			score += 1.0
		}
	}
	return score / float64(len(samples))
}

func adaptiveDirectionDeadband(stdNetW float64) float64 {
	base := systemStateNetThresholdWatts * 0.45
	dynamic := math.Abs(stdNetW) * 0.25
	if dynamic > base {
		return dynamic
	}
	return base
}

func lastNSamples(samples []float64, n int) []float64 {
	if n <= 0 || len(samples) <= n {
		return samples
	}
	return samples[len(samples)-n:]
}

func average(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sum := 0.0
	for _, sample := range samples {
		sum += sample
	}
	return sum / float64(len(samples))
}

func averageAbs(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sum := 0.0
	for _, sample := range samples {
		sum += math.Abs(sample)
	}
	return sum / float64(len(samples))
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
