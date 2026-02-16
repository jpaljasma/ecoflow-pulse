package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type mlEstimateProfile string

const (
	mlEstimateProfileDefault mlEstimateProfile = "default"
	mlEstimateProfileGeneric mlEstimateProfile = "generic"
	mlEstimateProfileD2M     mlEstimateProfile = "d2m"
	mlEstimateProfileDPU     mlEstimateProfile = "dpu"
)

type mlEstimateProfileConfig struct {
	name                 mlEstimateProfile
	fastWindow           int
	stableWindow         int
	recentWindow         int
	mediumWindow         int
	trendWindow          int
	latestWeight         float64
	recentWeight         float64
	mediumWeight         float64
	trendWeight          float64
	netThresholdScale    float64
	confidenceBase       float64
	confidenceSignalGain float64
	confidenceStateGain  float64
	remainBlendWeight    float64
	earlyHighSamples     int
	earlyHighDirection   float64
	earlyHighRecentCVMax float64
	earlyHighFloor       float64
}

var mlEstimateDefaultProfileConfig = mlEstimateProfileConfig{
	name:                 mlEstimateProfileGeneric,
	fastWindow:           14,
	stableWindow:         48,
	recentWindow:         3,
	mediumWindow:         16,
	trendWindow:          12,
	latestWeight:         0.805335181192606,
	recentWeight:         0.153512799544190,
	mediumWeight:         0.041152019263204,
	trendWeight:          0.221544188091400,
	netThresholdScale:    1.051656760004609,
	confidenceBase:       0.22,
	confidenceSignalGain: 0.14,
	confidenceStateGain:  0.09,
	remainBlendWeight:    0.34,
	earlyHighSamples:     10,
	earlyHighDirection:   0.90,
	earlyHighRecentCVMax: 0.50,
	earlyHighFloor:       0.90,
}

var mlEstimateProfileConfigs = map[mlEstimateProfile]mlEstimateProfileConfig{
	mlEstimateProfileDefault: mlEstimateDefaultProfileConfig,
	mlEstimateProfileGeneric: mlEstimateDefaultProfileConfig,
	mlEstimateProfileD2M: {
		name:                 mlEstimateProfileD2M,
		fastWindow:           27,
		stableWindow:         30,
		recentWindow:         3,
		mediumWindow:         10,
		trendWindow:          6,
		latestWeight:         0.786829119625947,
		recentWeight:         0.148453296278556,
		mediumWeight:         0.064717584095497,
		trendWeight:          0.113446462914328,
		netThresholdScale:    1.067483833161554,
		confidenceBase:       0.24,
		confidenceSignalGain: 0.18,
		confidenceStateGain:  0.12,
		remainBlendWeight:    0.72109583356051,
		earlyHighSamples:     8,
		earlyHighDirection:   0.88,
		earlyHighRecentCVMax: 0.58,
		earlyHighFloor:       0.90,
	},
	mlEstimateProfileDPU: {
		name:                 mlEstimateProfileDPU,
		fastWindow:           13,
		stableWindow:         29,
		recentWindow:         8,
		mediumWindow:         20,
		trendWindow:          3,
		latestWeight:         0.463261399215132,
		recentWeight:         0.305012377404102,
		mediumWeight:         0.231726223380766,
		trendWeight:          0.263488956398342,
		netThresholdScale:    0.813874424922922,
		confidenceBase:       0.26,
		confidenceSignalGain: 0.19,
		confidenceStateGain:  0.13,
		remainBlendWeight:    0.8688152910285984,
		earlyHighSamples:     6,
		earlyHighDirection:   0.84,
		earlyHighRecentCVMax: 0.65,
		earlyHighFloor:       0.90,
	},
}

func detectMLEstimateProfile(snapshot *energySnapshot) mlEstimateProfile {
	if snapshot == nil {
		return mlEstimateProfileGeneric
	}
	if snapshot.HasXT150 {
		return mlEstimateProfileD2M
	}
	if len(snapshot.Packs) > 0 {
		return mlEstimateProfileDPU
	}
	return mlEstimateProfileGeneric
}

func profileConfigForMLEstimate(profile mlEstimateProfile) mlEstimateProfileConfig {
	if cfg, ok := mlEstimateProfileConfigs[profile]; ok {
		return cfg
	}
	return mlEstimateDefaultProfileConfig
}

func estimateBatteryETAsMLProfiled(
	snapshot *energySnapshot,
	history *minuteTelemetryHistory,
	state systemStateKind,
) (batteryETAEstimates, mlEstimateProfile) {
	profile := detectMLEstimateProfile(snapshot)
	estimates := estimateBatteryETAsMLWithProfile(snapshot, history, state, profile)
	return estimates, profile
}

func estimateBatteryETAsMLGeneric(
	snapshot *energySnapshot,
	history *minuteTelemetryHistory,
	state systemStateKind,
) batteryETAEstimates {
	return estimateBatteryETAsMLWithProfile(snapshot, history, state, mlEstimateProfileGeneric)
}

func estimateBatteryETAsMLWithProfile(
	snapshot *energySnapshot,
	history *minuteTelemetryHistory,
	state systemStateKind,
	profile mlEstimateProfile,
) batteryETAEstimates {
	cfg := profileConfigForMLEstimate(profile)

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

	samples := netPowerSamplesFromPowerHistory(snapshot.mlFastHistory, 120)
	if len(samples) < 2 {
		samples = netPowerSamplesFromMinuteHistory(history, 24)
	}
	if len(samples) < 2 {
		return estimates
	}

	samples = adaptProfilePredictionSamples(samples, cfg)
	predNetW, meanNetW, stdNetW, ok := predictNetPowerProfiled(samples, cfg)
	if !ok {
		return estimates
	}
	threshold := systemStateNetThresholdWatts * cfg.netThresholdScale
	if threshold < 3 {
		threshold = 3
	}

	chargePowerW := 0.0
	hasChargePower := false
	if predNetW > threshold {
		chargePowerW = predNetW
		hasChargePower = true
	} else {
		recent := average(lastNSamples(samples, cfg.recentWindow))
		if state == systemStateCharging && recent > threshold*0.9 {
			chargePowerW = recent
			hasChargePower = true
		}
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
	if predNetW < -threshold {
		dischargePowerW = -predNetW
		hasDischargePower = true
	} else {
		recent := average(lastNSamples(samples, cfg.recentWindow))
		if state == systemStateDischarging && recent < -threshold*0.9 {
			dischargePowerW = -recent
			hasDischargePower = true
		}
	}
	if hasDischargePower {
		if sanitized, ok := snapshot.sanitizeBatteryFlowHintWatts(dischargePowerW); ok {
			dischargePowerW = sanitized
		} else {
			hasDischargePower = false
		}
	}

	stateForModel := resolveMLScoringState(state, hasChargePower, hasDischargePower, predNetW, meanNetW)

	chargeEtaMin := 0.0
	hasChargeETA := false
	if hasChargePower {
		estimates.chargeWatts = chargePowerW
		estimates.hasChargeWatts = true
		if energyToChargeWh <= 0 {
			chargeEtaMin = 0
			hasChargeETA = true
			estimates.ChargeValue = "0min (~0m)"
		} else {
			chargeEtaMin = energyToChargeWh * 60.0 / chargePowerW
			hasChargeETA = true
			estimates.ChargeValue = formatETAMinutes(chargeEtaMin)
		}
	}

	dischargeEtaMin := 0.0
	hasDischargeETA := false
	if hasDischargePower {
		estimates.dischargeWatts = dischargePowerW
		estimates.hasDischargeWatts = true
		if energyToDischargeWh <= 0 {
			dischargeEtaMin = 0
			hasDischargeETA = true
			estimates.DischargeValue = "0min (~0m)"
		} else {
			dischargeEtaMin = energyToDischargeWh * 60.0 / dischargePowerW
			hasDischargeETA = true
			estimates.DischargeValue = formatETAMinutes(dischargeEtaMin)
		}
	}

	deviceEtaResidualRatio := 0.0
	hasDeviceEtaResidual := false

	// Blend toward directional device remain when it is available.
	if deviceRemainMin, remainSource, ok := snapshot.selectRemainForState(stateForModel); ok && deviceRemainMin > 0 {
		deviceRemain := float64(deviceRemainMin)
		switch {
		case stateForModel == systemStateCharging && hasChargeETA && (remainSource == "charge" || remainSource == "global"):
			blend := adaptiveProfileRemainBlend(cfg.remainBlendWeight, profile, chargeEtaMin, deviceRemain, remainSource)
			chargeEtaMin = ((1.0 - blend) * chargeEtaMin) + (blend * deviceRemain)
			chargeEtaMin = applyProfileEtaBias(snapshot, profile, systemStateCharging, chargeEtaMin, deviceRemain, remainSource)
			estimates.ChargeValue = formatETAMinutes(chargeEtaMin)
			estimates.ActiveValue = estimates.ChargeValue
			deviceEtaResidualRatio = etaResidualRatio(chargeEtaMin, deviceRemain)
			hasDeviceEtaResidual = true
			if energyToChargeWh > 0 && chargeEtaMin > 0 {
				chargePowerW = energyToChargeWh * 60.0 / chargeEtaMin
				if sanitized, ok := snapshot.sanitizeBatteryFlowHintWatts(chargePowerW); ok {
					chargePowerW = sanitized
					estimates.chargeWatts = chargePowerW
					estimates.hasChargeWatts = true
				}
			}
		case stateForModel == systemStateDischarging && hasDischargeETA && (remainSource == "discharge" || remainSource == "global"):
			blend := adaptiveProfileRemainBlend(cfg.remainBlendWeight, profile, dischargeEtaMin, deviceRemain, remainSource)
			dischargeEtaMin = ((1.0 - blend) * dischargeEtaMin) + (blend * deviceRemain)
			dischargeEtaMin = applyProfileEtaBias(snapshot, profile, systemStateDischarging, dischargeEtaMin, deviceRemain, remainSource)
			estimates.DischargeValue = formatETAMinutes(dischargeEtaMin)
			estimates.ActiveValue = estimates.DischargeValue
			deviceEtaResidualRatio = etaResidualRatio(dischargeEtaMin, deviceRemain)
			hasDeviceEtaResidual = true
			if energyToDischargeWh > 0 && dischargeEtaMin > 0 {
				dischargePowerW = energyToDischargeWh * 60.0 / dischargeEtaMin
				if sanitized, ok := snapshot.sanitizeBatteryFlowHintWatts(dischargePowerW); ok {
					dischargePowerW = sanitized
					estimates.dischargeWatts = dischargePowerW
					estimates.hasDischargeWatts = true
				}
			}
		}
	}

	switch stateForModel {
	case systemStateCharging:
		estimates.ActiveValue = estimates.ChargeValue
	case systemStateDischarging:
		estimates.ActiveValue = estimates.DischargeValue
	default:
		estimates.ActiveValue = selectDominantETAValue(
			estimates.ChargeValue,
			hasChargePower,
			chargePowerW,
			estimates.DischargeValue,
			hasDischargePower,
			dischargePowerW,
		)
	}

	signMatch := 0.5
	recent := lastNSamples(samples, cfg.recentWindow)
	recentSignMatch := 0.5
	deadband := adaptiveDirectionDeadband(stdNetW)
	if deadband < threshold*0.7 {
		deadband = threshold * 0.7
	}
	switch stateForModel {
	case systemStateCharging:
		signMatch = signDirectionMatchRatio(samples, true, deadband)
		recentSignMatch = signDirectionMatchRatio(recent, true, deadband)
	case systemStateDischarging:
		signMatch = signDirectionMatchRatio(samples, false, deadband)
		recentSignMatch = signDirectionMatchRatio(recent, false, deadband)
	default:
		signMatch = nearZeroSampleRatio(samples, deadband*1.25)
		recentSignMatch = nearZeroSampleRatio(recent, deadband*1.25)
	}
	warmupSamples := cfg.recentWindow * 2
	if warmupSamples < 8 {
		warmupSamples = 8
	}
	sampleWarmup := 1.0 - math.Exp(-float64(len(samples))/float64(warmupSamples))
	sampleScore := sampleWarmup * 0.34
	sampleScore += math.Min(float64(len(samples))/float64(cfg.stableWindow), 1.0) * 0.08
	signalPower := math.Max(math.Abs(predNetW), math.Abs(meanNetW))
	signalScore := math.Min(signalPower/(threshold*6.5), 1.0) * cfg.confidenceSignalGain
	stabilityDenom := math.Max(math.Abs(meanNetW), threshold*1.8)
	stabilityCV := stdNetW / stabilityDenom
	if stabilityCV < 0 {
		stabilityCV = 0
	}
	if stabilityCV > 1.6 {
		stabilityCV = 1.6
	}
	stabilityScore := (1 - (stabilityCV / 1.6)) * 0.18
	recentStd := sampleStdDev(recent)
	recentMeanAbs := math.Abs(average(recent))
	recentStabilityDenom := math.Max(recentMeanAbs, threshold*1.4)
	recentCV := 0.0
	if recentStabilityDenom > 0 {
		recentCV = recentStd / recentStabilityDenom
	}
	if recentCV < 0 {
		recentCV = 0
	}
	if recentCV > 2 {
		recentCV = 2
	}
	recentStabilityScore := (1 - (recentCV / 2.0)) * 0.12
	directionAgreement := (signMatch * 0.45) + (recentSignMatch * 0.55)
	stateScore := directionAgreement * cfg.confidenceStateGain
	confidenceScore := cfg.confidenceBase + sampleScore + signalScore + stabilityScore + recentStabilityScore + stateScore
	if !hasChargePower && !hasDischargePower {
		if stateForModel == systemStateIdle {
			confidenceScore += 0.06
		} else {
			confidenceScore -= 0.12
		}
	}
	if predNetW*meanNetW < 0 && math.Abs(predNetW) > threshold && math.Abs(meanNetW) > threshold {
		confidenceScore -= 0.08
	}
	if hasDeviceEtaResidual {
		switch {
		case deviceEtaResidualRatio > 0.18:
			confidenceScore -= math.Min(0.18, (deviceEtaResidualRatio-0.18)*0.45)
		case deviceEtaResidualRatio < 0.08:
			confidenceScore += 0.03
		}
	}
	if stateForModel == systemStateCharging || stateForModel == systemStateDischarging {
		steadyWindow := cfg.recentWindow + 2
		if steadyWindow < 8 {
			steadyWindow = 8
		}
		if directionAgreement >= 0.88 && len(samples) >= steadyWindow {
			confidenceScore += 0.11
		} else if directionAgreement >= 0.80 && len(samples) >= cfg.recentWindow {
			confidenceScore += 0.06
		}
		if signalPower >= threshold*4.0 {
			confidenceScore += 0.04
		}
		highFloorWindow := cfg.recentWindow * 2
		if highFloorWindow < 10 {
			highFloorWindow = 10
		}
		if directionAgreement >= 0.9 && recentCV <= 0.5 && len(samples) >= highFloorWindow {
			if confidenceScore < 0.90 {
				confidenceScore = 0.90
			}
		}
		// Fast warmup path: for stable direction and low recent volatility,
		// promote confidence early with profile-specific thresholds.
		if len(samples) >= cfg.earlyHighSamples &&
			directionAgreement >= cfg.earlyHighDirection &&
			recentCV <= cfg.earlyHighRecentCVMax &&
			confidenceScore < cfg.earlyHighFloor {
			confidenceScore = cfg.earlyHighFloor
		}
	}
	estimates.ConfidenceValue = formatConfidenceValue(confidenceScore, true)

	switch {
	case estimates.hasChargeWatts && estimates.hasDischargeWatts:
		estimates.PowerValue = fmt.Sprintf(
			"power: chg@%s dsg@%s (profile:%s)",
			formatWatts(estimates.chargeWatts),
			formatWatts(estimates.dischargeWatts),
			cfg.name,
		)
	case estimates.hasChargeWatts:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s (profile:%s)", formatWatts(estimates.chargeWatts), cfg.name)
	case estimates.hasDischargeWatts:
		estimates.PowerValue = fmt.Sprintf("power: dsg@%s (profile:%s)", formatWatts(estimates.dischargeWatts), cfg.name)
	}

	return estimates
}

func adaptProfilePredictionSamples(samples []float64, cfg mlEstimateProfileConfig) []float64 {
	if len(samples) == 0 {
		return nil
	}
	if len(samples) > cfg.stableWindow {
		samples = lastNSamples(samples, cfg.stableWindow)
	}
	if len(samples) < 12 {
		return samples
	}
	recent := lastNSamples(samples, 6)
	prev := samples[len(samples)-12 : len(samples)-6]
	jump := math.Abs(average(recent) - average(prev))
	span := math.Abs(recent[len(recent)-1] - recent[0])
	base := math.Max(systemStateNetThresholdWatts*2.0, averageAbs(samples)*0.4)
	if jump > base*0.45 || span > base*0.65 {
		if len(samples) > cfg.fastWindow {
			return lastNSamples(samples, cfg.fastWindow)
		}
	}
	return samples
}

func predictNetPowerProfiled(samples []float64, cfg mlEstimateProfileConfig) (pred float64, mean float64, std float64, ok bool) {
	if len(samples) < 2 {
		return 0, 0, 0, false
	}
	latest := samples[len(samples)-1]
	recent := average(lastNSamples(samples, cfg.recentWindow))
	medium := average(lastNSamples(samples, cfg.mediumWindow))
	mean = average(samples)

	trend := 0.0
	if len(samples) > cfg.trendWindow {
		prev := samples[len(samples)-1-cfg.trendWindow]
		trend = (latest - prev) / float64(cfg.trendWindow)
	}

	pred = (cfg.latestWeight * latest) + (cfg.recentWeight * recent) + (cfg.mediumWeight * medium) + (cfg.trendWeight * trend)
	if math.Abs(pred) < systemStateNetThresholdWatts*0.85 && math.Abs(recent) >= systemStateNetThresholdWatts {
		pred = recent
	}

	variance := 0.0
	for _, sample := range samples {
		delta := sample - mean
		variance += delta * delta
	}
	variance /= float64(len(samples))
	std = math.Sqrt(variance)
	return pred, mean, std, true
}

func adaptiveProfileRemainBlend(
	base float64,
	profile mlEstimateProfile,
	modelMinutes float64,
	deviceMinutes float64,
	source string,
) float64 {
	blend := base
	if modelMinutes > 0 && deviceMinutes > 0 {
		divergence := math.Abs(modelMinutes-deviceMinutes) / math.Max(deviceMinutes, 1.0)
		switch profile {
		case mlEstimateProfileD2M:
			blend += math.Min(0.24, divergence*0.45)
		case mlEstimateProfileDPU:
			blend += math.Min(0.18, divergence*0.32)
		default:
			blend += math.Min(0.14, divergence*0.28)
		}
	}
	if strings.EqualFold(source, "global") {
		switch profile {
		case mlEstimateProfileD2M:
			// D2M global remain is useful but can be noisier than directional.
			blend -= 0.04
		case mlEstimateProfileDPU:
			blend -= 0.06
		default:
			blend -= 0.07
		}
	}
	if blend < 0.30 {
		blend = 0.30
	}
	if blend > 0.96 {
		blend = 0.96
	}
	return blend
}

func applyProfileEtaBias(
	snapshot *energySnapshot,
	profile mlEstimateProfile,
	state systemStateKind,
	modelMinutes float64,
	deviceMinutes float64,
	remainSource string,
) float64 {
	if snapshot == nil || modelMinutes <= 0 || deviceMinutes <= 0 {
		return modelMinutes
	}
	if profile != mlEstimateProfileD2M {
		return modelMinutes
	}

	alpha := 0.28
	if strings.EqualFold(remainSource, "global") {
		alpha = 0.18
	}
	targetRatio := deviceMinutes / modelMinutes
	if targetRatio < 0.55 || targetRatio > 1.80 {
		// Ignore extreme outliers for bias updates.
		return modelMinutes
	}

	switch state {
	case systemStateCharging:
		snapshot.mlProfileChargeBias, snapshot.hasMLProfileChargeBias = updateProfileBiasEMA(
			snapshot.mlProfileChargeBias,
			snapshot.hasMLProfileChargeBias,
			targetRatio,
			alpha,
		)
		if snapshot.hasMLProfileChargeBias {
			return modelMinutes * snapshot.mlProfileChargeBias
		}
	case systemStateDischarging:
		snapshot.mlProfileDischargeBias, snapshot.hasMLProfileDischargeBias = updateProfileBiasEMA(
			snapshot.mlProfileDischargeBias,
			snapshot.hasMLProfileDischargeBias,
			targetRatio,
			alpha,
		)
		if snapshot.hasMLProfileDischargeBias {
			return modelMinutes * snapshot.mlProfileDischargeBias
		}
	}
	return modelMinutes
}

func updateProfileBiasEMA(prev float64, hasPrev bool, target float64, alpha float64) (float64, bool) {
	if alpha <= 0 {
		alpha = 0.2
	}
	if alpha > 1 {
		alpha = 1
	}
	bias := target
	if hasPrev {
		bias = (alpha * target) + ((1 - alpha) * prev)
	}
	if bias < 0.70 {
		bias = 0.70
	}
	if bias > 1.30 {
		bias = 1.30
	}
	return bias, true
}

func etaResidualRatio(modelMinutes float64, deviceMinutes float64) float64 {
	if modelMinutes <= 0 || deviceMinutes <= 0 {
		return 0
	}
	return math.Abs(modelMinutes-deviceMinutes) / math.Max(deviceMinutes, 1.0)
}

func sampleStdDev(samples []float64) float64 {
	if len(samples) <= 1 {
		return 0
	}
	m := average(samples)
	variance := 0.0
	for _, sample := range samples {
		delta := sample - m
		variance += delta * delta
	}
	variance /= float64(len(samples))
	return math.Sqrt(variance)
}

func estimateBatteryETAsUnitSpecific(snapshot *energySnapshot, state systemStateKind) batteryETAEstimates {
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

	packChargeW, packDischargeW := packPowerTotals(snapshot.Packs)
	effectiveIn, hasEffectiveIn, effectiveOut, hasEffectiveOut := snapshot.effectiveTotalsForDisplayWithPackTotals(packChargeW, packDischargeW)
	flow := snapshot.batteryFlowForDisplay(
		effectiveIn,
		hasEffectiveIn,
		effectiveOut,
		hasEffectiveOut,
		packChargeW,
		packDischargeW,
	)
	if flow.hasIn && flow.inWatts > systemStateNetThresholdWatts {
		estimates.chargeWatts = flow.inWatts
		estimates.hasChargeWatts = true
	}
	if flow.hasOut && flow.outWatts > systemStateNetThresholdWatts {
		estimates.dischargeWatts = flow.outWatts
		estimates.hasDischargeWatts = true
	}

	confidenceScore := 0.0
	hasConfidence := false
	if remainMin, source, ok := snapshot.selectRemainForState(state); ok && remainMin > 0 {
		remainValue := formatETAMinutes(float64(remainMin))
		switch source {
		case "charge":
			estimates.ChargeValue = remainValue
			confidenceScore = 0.95
			hasConfidence = true
		case "discharge":
			estimates.DischargeValue = remainValue
			confidenceScore = 0.95
			hasConfidence = true
		default:
			switch state {
			case systemStateCharging:
				estimates.ChargeValue = remainValue
			case systemStateDischarging:
				estimates.DischargeValue = remainValue
			default:
				estimates.ActiveValue = remainValue
			}
			confidenceScore = 0.78
			hasConfidence = true
		}
	}

	if (estimates.ChargeValue == "n/a" || estimates.DischargeValue == "n/a") && snapshot != nil {
		energyToChargeWh, energyToDischargeWh, ok := snapshot.energyToTargetsWh()
		if ok {
			if estimates.ChargeValue == "n/a" && estimates.hasChargeWatts && estimates.chargeWatts > 0 {
				estimates.ChargeValue = formatETAMinutes((energyToChargeWh * 60.0) / estimates.chargeWatts)
			}
			if estimates.DischargeValue == "n/a" && estimates.hasDischargeWatts && estimates.dischargeWatts > 0 {
				estimates.DischargeValue = formatETAMinutes((energyToDischargeWh * 60.0) / estimates.dischargeWatts)
			}
			if !hasConfidence && (estimates.hasChargeWatts || estimates.hasDischargeWatts) {
				confidenceScore = 0.68
				hasConfidence = true
			}
		}
	}

	switch state {
	case systemStateCharging:
		estimates.ActiveValue = estimates.ChargeValue
	case systemStateDischarging:
		estimates.ActiveValue = estimates.DischargeValue
	default:
		estimates.ActiveValue = selectDominantETAValue(
			estimates.ChargeValue,
			estimates.hasChargeWatts,
			estimates.chargeWatts,
			estimates.DischargeValue,
			estimates.hasDischargeWatts,
			estimates.dischargeWatts,
		)
	}

	switch {
	case estimates.hasChargeWatts && estimates.hasDischargeWatts:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s dsg@%s", formatWatts(estimates.chargeWatts), formatWatts(estimates.dischargeWatts))
	case estimates.hasChargeWatts:
		estimates.PowerValue = fmt.Sprintf("power: chg@%s", formatWatts(estimates.chargeWatts))
	case estimates.hasDischargeWatts:
		estimates.PowerValue = fmt.Sprintf("power: dsg@%s", formatWatts(estimates.dischargeWatts))
	}
	estimates.ConfidenceValue = formatConfidenceValue(confidenceScore, hasConfidence)
	return estimates
}

func estimateDeltaMinutesDisplay(reference batteryETAEstimates, candidate batteryETAEstimates, state systemStateKind) string {
	refMinutes, okRef := estimateActiveMinutes(reference, state)
	candMinutes, okCand := estimateActiveMinutes(candidate, state)
	if !okRef || !okCand {
		return "n/a"
	}
	delta := candMinutes - refMinutes
	if math.Abs(delta) < 0.5 {
		return "~0min"
	}
	return fmt.Sprintf("%+.0fmin", delta)
}

func estimateDeltaPowerDisplay(reference batteryETAEstimates, candidate batteryETAEstimates, state systemStateKind) string {
	refPower, okRef := estimateSignedPower(reference, state)
	candPower, okCand := estimateSignedPower(candidate, state)
	if !okRef || !okCand {
		return "n/a"
	}
	delta := candPower - refPower
	if math.Abs(delta) < 0.5 {
		return "~0.0W"
	}
	return fmt.Sprintf("%+.1fW", delta)
}

func estimateActiveMinutes(est batteryETAEstimates, state systemStateKind) (float64, bool) {
	switch state {
	case systemStateCharging:
		return parseETAMinutes(est.ChargeValue)
	case systemStateDischarging:
		return parseETAMinutes(est.DischargeValue)
	default:
		if minutes, ok := parseETAMinutes(est.ActiveValue); ok {
			return minutes, true
		}
		if minutes, ok := parseETAMinutes(est.ChargeValue); ok {
			return minutes, true
		}
		return parseETAMinutes(est.DischargeValue)
	}
}

func estimateSignedPower(est batteryETAEstimates, state systemStateKind) (float64, bool) {
	switch state {
	case systemStateCharging:
		if est.hasChargeWatts {
			return est.chargeWatts, true
		}
	case systemStateDischarging:
		if est.hasDischargeWatts {
			return -est.dischargeWatts, true
		}
	default:
		switch {
		case est.hasChargeWatts && est.hasDischargeWatts:
			if est.chargeWatts >= est.dischargeWatts {
				return est.chargeWatts, true
			}
			return -est.dischargeWatts, true
		case est.hasChargeWatts:
			return est.chargeWatts, true
		case est.hasDischargeWatts:
			return -est.dischargeWatts, true
		}
	}
	return parseSignedPowerFromDisplay(est.PowerValue)
}

func parseETAMinutes(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "n/a" {
		return 0, false
	}
	fields := strings.Fields(value)
	for _, field := range fields {
		field = strings.TrimSpace(strings.TrimSuffix(field, ","))
		if !strings.HasSuffix(field, "min") {
			continue
		}
		num := strings.TrimSpace(strings.TrimSuffix(field, "min"))
		if num == "" {
			continue
		}
		parsed, err := strconv.ParseFloat(num, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			continue
		}
		return parsed, true
	}
	return 0, false
}

func parseSignedPowerFromDisplay(value string) (float64, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "n/a" {
		return 0, false
	}
	if idx := strings.Index(value, "chg@"); idx >= 0 {
		if watts, ok := parseWattsToken(value[idx+4:]); ok {
			return watts, true
		}
	}
	if idx := strings.Index(value, "dsg@"); idx >= 0 {
		if watts, ok := parseWattsToken(value[idx+4:]); ok {
			return -watts, true
		}
	}
	return 0, false
}

func parseWattsToken(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0, false
	}
	token := strings.TrimSpace(fields[0])
	switch {
	case strings.HasSuffix(token, "kw"):
		raw := strings.TrimSuffix(token, "kw")
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return 0, false
		}
		return parsed * 1000.0, true
	case strings.HasSuffix(token, "w"):
		raw := strings.TrimSuffix(token, "w")
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		parsed, err := strconv.ParseFloat(token, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	}
}
