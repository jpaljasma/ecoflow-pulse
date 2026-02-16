package main

import (
	"fmt"
	"math"
	"strings"
)

func extractPVInput(quota map[string]any) []telemetryMetric {
	keys := sortedMapKeys(quota)
	out := make([]telemetryMetric, 0, 6)
	for _, key := range keys {
		if !isPVInputKey(key) {
			continue
		}
		value, ok := numberFromAny(quota[key])
		if !ok {
			continue
		}
		out = append(out, telemetryMetric{Key: key, Value: value})
	}
	return out
}

func isPVInputKey(key string) bool {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "mppt.inwatts") {
		return true
	}
	if strings.Contains(lower, "inhvmpptpwr") || strings.Contains(lower, "inlvmpptpwr") {
		return true
	}
	if strings.Contains(lower, "pv") {
		if strings.Contains(lower, "chargewatts") || strings.Contains(lower, "inwatts") || strings.HasSuffix(lower, "pwr") {
			return true
		}
	}
	return false
}

func effectivePVInputWatts(
	hasInputWatts bool,
	inputWatts float64,
	hasVolts bool,
	volts float64,
	hasAmps bool,
	amps float64,
	hasActiveHint bool,
	activeHint bool,
) (float64, bool) {
	effectiveInputWatts := normalizeInputChannelWatts(inputWatts)
	effectiveHasInputWatts := hasInputWatts
	if hasActiveHint && !activeHint && (!effectiveHasInputWatts || math.Abs(effectiveInputWatts) < solarPowerEstimateMinWatts) {
		return 0, true
	}
	if hasVolts && hasAmps {
		estimatedInputWatts := math.Abs(volts * amps)
		if estimatedInputWatts > solarPowerEstimateMaxWatts {
			estimatedInputWatts = 0
		}
		// Guard against scale mismatches (for example raw mV/mA published as volts/amps).
		if effectiveHasInputWatts && effectiveInputWatts > 0 && estimatedInputWatts > effectiveInputWatts*5 {
			estimatedInputWatts = 0
		}
		// Prefer direct channel power (appshow/d_addr) when present.
		// Use V*I as a fallback when channel watts are missing/near-zero.
		// If both exist, only allow a small correction when they are close to avoid stale
		// backend V*I samples pinning PV high after clouds reduce real-time power.
		if estimatedInputWatts >= solarPowerEstimateMinWatts {
			switch {
			case !effectiveHasInputWatts || math.Abs(effectiveInputWatts) < solarPowerEstimateMinWatts:
				effectiveInputWatts = estimatedInputWatts
				effectiveHasInputWatts = true
			default:
				diff := math.Abs(estimatedInputWatts - effectiveInputWatts)
				tolerance := math.Max(3.0, effectiveInputWatts*0.10)
				if diff <= tolerance {
					effectiveInputWatts = estimatedInputWatts
				}
			}
		}
	}
	if effectiveHasInputWatts && math.Abs(effectiveInputWatts) < solarPowerEstimateMinWatts {
		effectiveInputWatts = 0
	}
	return effectiveInputWatts, effectiveHasInputWatts
}

func (s *energySnapshot) effectivePVInputChannels() (total float64, hasTotal bool, low float64, hasLow bool, high float64, hasHigh bool) {
	if s == nil {
		return 0, false, 0, false, 0, false
	}
	low, hasLow = effectivePVInputWatts(
		s.HasInPVLow,
		s.InPVLowWatts,
		s.HasSolarLVVolts,
		s.SolarLVVolts,
		s.HasSolarLVAmp,
		s.SolarLVAmp,
		s.HasPVLowChgState,
		isMPPTChargeStateActive(s.PVLowChgStateRaw),
	)
	high, hasHigh = effectivePVInputWatts(
		s.HasInPVHigh,
		s.InPVHighWatts,
		s.HasSolarHVVolts,
		s.SolarHVVolts,
		s.HasSolarHVAmp,
		s.SolarHVAmp,
		s.HasPVHighChgState,
		isMPPTChargeStateActive(s.PVHighChgStateRaw),
	)

	total, hasTotal = s.InPVWatts, s.HasInPV
	if hasLow || hasHigh {
		channelTotal := 0.0
		if hasLow {
			channelTotal += low
		}
		if hasHigh {
			channelTotal += high
		}
		if !hasTotal || total < channelTotal {
			total = channelTotal
			hasTotal = true
		}
	} else if !hasTotal && (s.HasInPVLow || s.HasInPVHigh) && s.refreshPVTotalFromChannels() {
		total, hasTotal = s.InPVWatts, s.HasInPV
	}
	return total, hasTotal, low, hasLow, high, hasHigh
}

func (s *energySnapshot) effectivePVInputWatts() (float64, bool) {
	total, hasTotal, _, _, _, _ := s.effectivePVInputChannels()
	return total, hasTotal
}

func (s *energySnapshot) pushPVSmoothingSample() {
	if s == nil {
		return
	}
	total, hasTotal, low, hasLow, high, hasHigh := s.effectivePVInputChannels()
	if hasLow && s.pvLowSmoother != nil {
		s.pvLowSmoother.Add(low)
	}
	if hasHigh && s.pvHighSmoother != nil {
		s.pvHighSmoother.Add(high)
	}
	if hasTotal && s.pvTotalSmoother != nil {
		s.pvTotalSmoother.Add(total)
	}
}

func (s *energySnapshot) smoothedPVChannels() (total float64, hasTotal bool, low float64, hasLow bool, high float64, hasHigh bool) {
	if s == nil {
		return 0, false, 0, false, 0, false
	}
	if s.pvTotalSmoother != nil {
		if value, ok := s.pvTotalSmoother.Average(); ok {
			total, hasTotal = value, true
		}
	}
	if s.pvLowSmoother != nil {
		if value, ok := s.pvLowSmoother.Average(); ok {
			low, hasLow = value, true
		}
	}
	if s.pvHighSmoother != nil {
		if value, ok := s.pvHighSmoother.Average(); ok {
			high, hasHigh = value, true
		}
	}
	if !hasTotal && (hasLow || hasHigh) {
		total = 0
		if hasLow {
			total += low
		}
		if hasHigh {
			total += high
		}
		hasTotal = true
	}
	return total, hasTotal, low, hasLow, high, hasHigh
}

func derivePVInputState(
	hasInputWatts bool,
	inputWatts float64,
	hasVolts bool,
	volts float64,
	hasAmps bool,
	amps float64,
	lockedByFlag bool,
) string {
	effectiveInputWatts := inputWatts
	effectiveHasInputWatts := hasInputWatts
	if !effectiveHasInputWatts && hasVolts && hasAmps {
		effectiveInputWatts = math.Abs(volts * amps)
		effectiveHasInputWatts = true
	}

	hasPVSignals := lockedByFlag || hasVolts || hasAmps || effectiveHasInputWatts
	if !hasPVSignals {
		return "n/a"
	}

	lowCurrentWithVoltage := hasVolts && volts >= solarLockVoltageMinVolts
	if hasAmps {
		lowCurrentWithVoltage = lowCurrentWithVoltage && math.Abs(amps) <= solarLockCurrentMaxAmps
	}
	lowInput := !effectiveHasInputWatts || effectiveInputWatts <= solarLockInputMaxWatts
	switch {
	case lockedByFlag && lowInput:
		if hasVolts {
			return fmt.Sprintf("locked(%.1fV)", volts)
		}
		return "locked"
	case lowCurrentWithVoltage && lowInput:
		return fmt.Sprintf("locked(%.1fV)", volts)
	case effectiveHasInputWatts && effectiveInputWatts > solarLockInputMaxWatts:
		return fmt.Sprintf("active(%s)", formatWatts(effectiveInputWatts))
	default:
		return "idle"
	}
}

func normalizeVoltageVolts(value float64) float64 {
	if value > 1000 {
		return value / 1000.0
	}
	return value
}

func normalizeMPPTVoltageVolts(value float64) float64 {
	abs := math.Abs(value)
	// mpptStatus often reports PV voltages as integer millivolts (including sub-1000 values).
	// Keep explicit decimal values (e.g. 35.8) untouched.
	if abs >= 100 && value == math.Trunc(value) {
		return value / 1000.0
	}
	return normalizeVoltageVolts(value)
}

func normalizeMPPTCurrentAmps(value float64) float64 {
	abs := math.Abs(value)
	if abs >= 1 && value == math.Trunc(value) {
		// mpptStatus integer currents are typically raw mA.
		return value / 1000.0
	}
	return normalizeCurrentAmps(value)
}

func normalizeCurrentAmps(value float64) float64 {
	abs := math.Abs(value)
	if abs > 200 {
		return value / 1000.0
	}
	return value
}

func isMPPTChargeStateActive(raw int64) bool {
	return raw > 0
}

func (s *energySnapshot) refreshPVTotalFromChannels() bool {
	if s == nil {
		return false
	}
	if !s.HasInPVLow && !s.HasInPVHigh {
		return false
	}
	total := 0.0
	if s.HasInPVLow {
		total += s.InPVLowWatts
	}
	if s.HasInPVHigh {
		total += s.InPVHighWatts
	}
	s.InPVWatts = total
	s.HasInPV = true
	return true
}

func splitPVChannels(values map[string]float64) (low float64, hasLow bool, high float64, hasHigh bool) {
	seen := make(map[string]struct{}, len(values))
	for key, value := range values {
		canonical := canonicalQuotaMetricKey(key)
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		normalized := normalizeInputChannelWatts(value)
		switch classifyPVInputChannelKey(key) {
		case "low":
			if !hasLow || normalized > low {
				low = normalized
			}
			hasLow = true
		case "high":
			if !hasHigh || normalized > high {
				high = normalized
			}
			hasHigh = true
		}
	}
	return low, hasLow, high, hasHigh
}

func canonicalQuotaMetricKey(key string) string {
	lower := strings.ToLower(strings.TrimSpace(key))
	if lower == "" {
		return ""
	}
	if idx := strings.LastIndex(lower, "."); idx >= 0 && idx+1 < len(lower) {
		return lower[idx+1:]
	}
	return lower
}

func classifyPVInputChannelKey(key string) string {
	lower := strings.ToLower(strings.TrimSpace(key))
	switch {
	case lower == "":
		return ""
	case strings.Contains(lower, "xt150watts"):
		return ""
	case strings.Contains(lower, "inhvmppt"), strings.Contains(lower, "pv2chargewatts"), strings.Contains(lower, "pv2inwatts"), strings.Contains(lower, "powgetpvh"), strings.Contains(lower, "pv2chargetype"), strings.Contains(lower, "pluginfopvhtype"):
		return "high"
	case strings.Contains(lower, "inlvmppt"), strings.Contains(lower, "pv1chargewatts"), strings.Contains(lower, "powgetpvl"), lower == "inwatts", strings.Contains(lower, "mppt.inwatts"), strings.Contains(lower, "pv1chargetype"), strings.Contains(lower, "pluginfopvltype"):
		return "low"
	default:
		return ""
	}
}

func splitPVChannelTypes(values map[string]int64) (low int64, hasLow bool, high int64, hasHigh bool) {
	for key, value := range values {
		switch classifyPVInputChannelKey(key) {
		case "low":
			low = value
			hasLow = true
		case "high":
			high = value
			hasHigh = true
		}
	}
	return low, hasLow, high, hasHigh
}

func sumPVInputChannelsFromQuota(quota map[string]any) (low float64, hasLow bool, high float64, hasHigh bool) {
	seen := make(map[string]struct{}, len(quota))
	for key, raw := range quota {
		if !isPVInputKey(key) {
			continue
		}
		canonical := canonicalQuotaMetricKey(key)
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		value, ok := numberFromAny(raw)
		if !ok {
			continue
		}
		normalized := normalizeInputChannelWatts(value)
		switch classifyPVInputChannelKey(key) {
		case "low":
			if !hasLow || normalized > low {
				low = normalized
			}
			hasLow = true
		case "high":
			if !hasHigh || normalized > high {
				high = normalized
			}
			hasHigh = true
		}
	}
	return low, hasLow, high, hasHigh
}

func sumPVInputFromQuota(quota map[string]any) (float64, bool) {
	total := 0.0
	found := false
	seen := make(map[string]struct{}, len(quota))
	for key, raw := range quota {
		if !isPVInputKey(key) {
			continue
		}
		canonical := canonicalQuotaMetricKey(key)
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		value, ok := numberFromAny(raw)
		if !ok {
			continue
		}
		total += normalizeInputChannelWatts(value)
		found = true
	}
	return total, found
}
