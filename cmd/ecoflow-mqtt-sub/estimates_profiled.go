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
	historySampleLimit   int
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
	ioSignalBlend        float64
	packSignalBlend      float64
	earlyHighSamples     int
	earlyHighDirection   float64
	earlyHighRecentCVMax float64
	earlyHighFloor       float64
}

var mlEstimateDefaultProfileConfig = mlEstimateProfileConfig{
	name:                 mlEstimateProfileGeneric,
	historySampleLimit:   180,
	fastWindow:           25,
	stableWindow:         52,
	recentWindow:         3,
	mediumWindow:         13,
	trendWindow:          7,
	latestWeight:         0.887823187849583,
	recentWeight:         0.069210837718650,
	mediumWeight:         0.042965974431767,
	trendWeight:          0.147042937396007,
	netThresholdScale:    0.808012840677693,
	confidenceBase:       0.22,
	confidenceSignalGain: 0.14,
	confidenceStateGain:  0.09,
	remainBlendWeight:    0.34,
	ioSignalBlend:        0.18,
	packSignalBlend:      0.32,
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
		historySampleLimit:   180,
		fastWindow:           13,
		stableWindow:         56,
		recentWindow:         5,
		mediumWindow:         10,
		trendWindow:          4,
		latestWeight:         0.807097623579115,
		recentWeight:         0.060103464051422,
		mediumWeight:         0.132798912369463,
		trendWeight:          0.131669747457649,
		netThresholdScale:    0.865958841104081,
		confidenceBase:       0.24,
		confidenceSignalGain: 0.18,
		confidenceStateGain:  0.12,
		remainBlendWeight:    0.72109583356051,
		ioSignalBlend:        0.26,
		packSignalBlend:      0.40,
		earlyHighSamples:     8,
		earlyHighDirection:   0.88,
		earlyHighRecentCVMax: 0.58,
		earlyHighFloor:       0.90,
	},
	mlEstimateProfileDPU: {
		name:                 mlEstimateProfileDPU,
		historySampleLimit:   210,
		fastWindow:           23,
		stableWindow:         42,
		recentWindow:         8,
		mediumWindow:         10,
		trendWindow:          6,
		latestWeight:         0.659730061423868,
		recentWeight:         0.279151762947370,
		mediumWeight:         0.061118175628762,
		trendWeight:          0.267736099158261,
		netThresholdScale:    1.163887790391481,
		confidenceBase:       0.26,
		confidenceSignalGain: 0.19,
		confidenceStateGain:  0.13,
		remainBlendWeight:    0.8688152910285984,
		ioSignalBlend:        0.14,
		packSignalBlend:      0.34,
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

	sampleLimit := cfg.historySampleLimit
	if sampleLimit < 120 {
		sampleLimit = 120
	}
	samples := netPowerSamplesFromPowerHistory(snapshot.mlFastHistory, sampleLimit)
	if len(samples) < 2 {
		samples = netPowerSamplesFromMinuteHistory(history, 36)
	}
	if len(samples) < 2 {
		return estimates
	}

	samples = adaptProfilePredictionSamples(samples, cfg)
	predNetW, meanNetW, stdNetW, ok := predictNetPowerProfiled(samples, cfg)
	if !ok {
		return estimates
	}
	if ioSamples := ioNetPowerSamplesFromPowerHistory(snapshot.mlFastHistory, sampleLimit); len(ioSamples) >= 2 {
		ioSamples = adaptProfilePredictionSamples(ioSamples, cfg)
		if ioPredW, ioMeanW, _, ioOK := predictNetPowerProfiled(ioSamples, cfg); ioOK {
			predNetW = blendAuxNetPrediction(predNetW, ioPredW, cfg.ioSignalBlend)
			meanNetW = blendAuxNetPrediction(meanNetW, ioMeanW, cfg.ioSignalBlend*0.85)
		}
	}
	if packSamples := packNetPowerSamplesFromPowerHistory(snapshot.mlFastHistory, sampleLimit); len(packSamples) >= 2 {
		packSamples = adaptProfilePredictionSamples(packSamples, cfg)
		if packPredW, packMeanW, _, packOK := predictNetPowerProfiled(packSamples, cfg); packOK {
			predNetW = blendAuxNetPrediction(predNetW, packPredW, cfg.packSignalBlend)
			meanNetW = blendAuxNetPrediction(meanNetW, packMeanW, cfg.packSignalBlend*0.85)
		}
	}
	threshold := systemStateNetThresholdWatts * cfg.netThresholdScale
	if threshold < 3 {
		threshold = 3
	}
	directionThreshold := threshold
	if profile == mlEstimateProfileD2M {
		// D2M low-PV operation can be noisy around zero net; widen directional deadband
		// using short-term volatility so we don't flip state on tiny oscillations.
		noisePad := stdNetW * 0.45
		maxPad := threshold * 2.0
		if noisePad > maxPad {
			noisePad = maxPad
		}
		directionThreshold += noisePad
		minDirection := math.Max(threshold*1.8, 14.0)
		if directionThreshold < minDirection {
			directionThreshold = minDirection
		}
	}

	chargePowerW := 0.0
	hasChargePower := false
	if predNetW > directionThreshold {
		chargePowerW = predNetW
		hasChargePower = true
	} else {
		recent := average(lastNSamples(samples, cfg.recentWindow))
		if state == systemStateCharging && recent > directionThreshold*0.9 {
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
	if predNetW < -directionThreshold {
		dischargePowerW = -predNetW
		hasDischargePower = true
	} else {
		recent := average(lastNSamples(samples, cfg.recentWindow))
		if state == systemStateDischarging && recent < -directionThreshold*0.9 {
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

	// Low-power steady-state fallback: when prediction hovers near threshold but
	// samples are directionally consistent, still emit a usable direction.
	if !hasChargePower && !hasDischargePower && state != systemStateIdle {
		recentSamples := lastNSamples(samples, cfg.recentWindow)
		if len(recentSamples) < 4 {
			recentSamples = lastNSamples(samples, 4)
		}
		if len(recentSamples) > 0 {
			recentMean := average(recentSamples)
			signDeadband := adaptiveDirectionDeadband(stdNetW)
			minDeadband := directionThreshold * 0.55
			if signDeadband < minDeadband {
				signDeadband = minDeadband
			}
			const lowPowerDirectionFloor = 0.55
			const lowPowerAgreementMin = 0.62
			switch state {
			case systemStateCharging:
				agree := signDirectionMatchRatio(recentSamples, true, signDeadband)
				if agree >= lowPowerAgreementMin &&
					(predNetW > directionThreshold*lowPowerDirectionFloor ||
						meanNetW > directionThreshold*lowPowerDirectionFloor ||
						recentMean > directionThreshold*lowPowerDirectionFloor) {
					chargePowerW = math.Max(math.Max(predNetW, meanNetW), recentMean)
					hasChargePower = chargePowerW > 0
				}
			case systemStateDischarging:
				agree := signDirectionMatchRatio(recentSamples, false, signDeadband)
				if agree >= lowPowerAgreementMin &&
					(predNetW < -directionThreshold*lowPowerDirectionFloor ||
						meanNetW < -directionThreshold*lowPowerDirectionFloor ||
						recentMean < -directionThreshold*lowPowerDirectionFloor) {
					dischargePowerW = math.Max(math.Max(-predNetW, -meanNetW), -recentMean)
					hasDischargePower = dischargePowerW > 0
				}
			}
		}
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
	flowNetW := 0.0
	hasFlowNet := flow.hasNet
	if hasFlowNet {
		flowNetW = flow.netWatts
	}

	// Favor pack/battery flow direction when net model is ambiguous. Keep D2M
	// conservative near zero; let DPU recover direction a bit faster.
	if hasFlowNet {
		flowStrongMul := 1.1
		modelAmbiguousMul := 2.0
		if profile == mlEstimateProfileD2M {
			flowStrongMul = 1.4
			modelAmbiguousMul = 2.4
		}
		flowStrong := math.Abs(flowNetW) > directionThreshold*flowStrongMul
		modelAmbiguous := math.Abs(predNetW) <= directionThreshold*modelAmbiguousMul
		noModelDirection := !hasChargePower && !hasDischargePower
		if flowStrong && (modelAmbiguous || noModelDirection) {
			if flowNetW > 0 {
				chargePowerW = math.Abs(flowNetW)
				hasChargePower = true
				dischargePowerW = 0
				hasDischargePower = false
			} else if flowNetW < 0 {
				dischargePowerW = math.Abs(flowNetW)
				hasDischargePower = true
				chargePowerW = 0
				hasChargePower = false
			}
		}
	}

	if hasChargePower {
		if sanitized, ok := snapshot.sanitizeBatteryFlowHintWatts(chargePowerW); ok {
			chargePowerW = sanitized
		} else {
			hasChargePower = false
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
	idleAgreement := 0.0
	if stateForModel == systemStateIdle {
		idleAgreement = directionAgreement
	}
	if hasFlowNet && math.Abs(flowNetW) > directionThreshold*1.2 {
		flowAgreeBonus := 0.0
		flowDisagreePenalty := 0.0
		switch profile {
		case mlEstimateProfileD2M:
			flowAgreeBonus = 0.07
			flowDisagreePenalty = 0.08
		case mlEstimateProfileDPU:
			flowAgreeBonus = 0.05
			flowDisagreePenalty = 0.06
		}
		if flowAgreeBonus > 0 && flowDisagreePenalty > 0 {
			switch stateForModel {
			case systemStateCharging:
				if flowNetW > 0 {
					stateScore += flowAgreeBonus
				} else {
					stateScore -= flowDisagreePenalty
				}
			case systemStateDischarging:
				if flowNetW < 0 {
					stateScore += flowAgreeBonus
				} else {
					stateScore -= flowDisagreePenalty
				}
			}
		}
	}
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
		// DPU trickle-state warmup: steady low-power direction should converge
		// faster even when absolute signal is small.
		if profile == mlEstimateProfileDPU &&
			len(samples) >= cfg.recentWindow+2 &&
			signalPower >= directionThreshold*0.65 &&
			signalPower <= directionThreshold*3.5 &&
			directionAgreement >= 0.78 &&
			recentSignMatch >= 0.78 &&
			recentCV <= 0.55 &&
			confidenceScore < 0.86 {
			confidenceScore = 0.86
		}
		// D2M low-PV steady windows can have tiny sign jitter around net-zero;
		// promote confidence earlier when recent behavior is still directionally stable.
		if profile == mlEstimateProfileD2M &&
			len(samples) >= 6 &&
			directionAgreement >= 0.74 &&
			recentSignMatch >= 0.76 &&
			recentCV <= 0.95 &&
			signalPower >= directionThreshold*1.6 &&
			signalPower <= 170 &&
			confidenceScore < 0.92 {
			confidenceScore = 0.92
		}
	} else if stateForModel == systemStateIdle {
		idleWindow := cfg.recentWindow + 2
		if idleWindow < 6 {
			idleWindow = 6
		}
		if len(samples) >= idleWindow && idleAgreement >= 0.80 && recentCV <= 0.75 {
			confidenceScore += 0.05
		}
		if profile == mlEstimateProfileD2M &&
			len(samples) >= 6 &&
			idleAgreement >= 0.86 &&
			recentCV <= 0.62 &&
			confidenceScore < 0.90 {
			confidenceScore = 0.90
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
	case stateForModel == systemStateIdle:
		estimates.PowerValue = fmt.Sprintf("power: idle@0.0W (profile:%s)", cfg.name)
	default:
		estimates.PowerValue = fmt.Sprintf("power: warmup (profile:%s)", cfg.name)
	}

	if strings.TrimSpace(estimates.ActiveValue) == "n/a" {
		if remainMin, _, ok := snapshot.selectRemainForState(stateForModel); ok && remainMin > 0 {
			estimates.ActiveValue = formatETAMinutes(float64(remainMin))
		} else if remainMin, _, ok := snapshot.selectRemainForState(state); ok && remainMin > 0 {
			estimates.ActiveValue = formatETAMinutes(float64(remainMin))
		}
	}
	if stateForModel == systemStateCharging && strings.TrimSpace(estimates.ChargeValue) == "n/a" {
		estimates.ChargeValue = estimates.ActiveValue
	}
	if stateForModel == systemStateDischarging && strings.TrimSpace(estimates.DischargeValue) == "n/a" {
		estimates.DischargeValue = estimates.ActiveValue
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

func blendAuxNetPrediction(base float64, aux float64, weight float64) float64 {
	if weight <= 0 {
		return base
	}
	if weight > 0.90 {
		weight = 0.90
	}
	baseAbs := math.Abs(base)
	auxAbs := math.Abs(aux)
	if base*aux < 0 {
		switch {
		case auxAbs < baseAbs*0.8:
			weight *= 0.15
		case auxAbs < baseAbs*1.2:
			weight *= 0.30
		default:
			weight *= 0.45
		}
	} else if auxAbs < 4.0 {
		weight *= 0.25
	}
	return ((1.0 - weight) * base) + (weight * aux)
}

func ioNetPowerSamplesFromPowerHistory(history *powerTelemetryHistory, limit int) []float64 {
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

func packNetPowerSamplesFromPowerHistory(history *powerTelemetryHistory, limit int) []float64 {
	if history == nil {
		return nil
	}
	buckets := history.SortedBuckets(false, limit)
	if len(buckets) == 0 {
		return nil
	}
	out := make([]float64, 0, len(buckets))
	for _, bucket := range buckets {
		hasCharge := bucket.PackChargeSamples > 0
		hasDischarge := bucket.PackDischargeSamples > 0
		if !hasCharge && !hasDischarge {
			continue
		}
		charge := 0.0
		discharge := 0.0
		if hasCharge {
			charge = bucket.PackChargeSumWatts / float64(bucket.PackChargeSamples)
		}
		if hasDischarge {
			discharge = bucket.PackDischargeSumWatts / float64(bucket.PackDischargeSamples)
		}
		out = append(out, charge-discharge)
	}
	return out
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
