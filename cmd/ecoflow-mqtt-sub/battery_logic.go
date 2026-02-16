package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func extractBatterySOC(quota map[string]any) []batterySOC {
	keys := sortedMapKeys(quota)
	out := make([]batterySOC, 0, 8)

	for _, key := range keys {
		if !strings.HasSuffix(strings.ToLower(key), ".bpinfo") {
			continue
		}
		entries, ok := quota[key].([]any)
		if !ok {
			continue
		}
		for i, raw := range entries {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			soc, ok := numberFromAny(entry["bpSoc"])
			if !ok {
				continue
			}
			bpNo, _ := numberFromAny(entry["bpNo"])
			label := fmt.Sprintf("%s[%d].bpSoc", key, i)
			if bpNo > 0 {
				label = fmt.Sprintf("%s.bpNo=%d", key, int(bpNo))
			}
			out = append(out, batterySOC{Label: label, SOC: soc})
		}
	}

	for _, key := range keys {
		if !strings.HasSuffix(strings.ToLower(key), ".soc") {
			continue
		}
		if !strings.Contains(strings.ToLower(key), "bms") {
			continue
		}
		soc, ok := numberFromAny(quota[key])
		if !ok {
			continue
		}
		out = append(out, batterySOC{Label: key, SOC: soc})
	}

	for _, key := range keys {
		lower := strings.ToLower(key)
		if lower != "watts" && !strings.HasSuffix(lower, "kitinfo.watts") {
			continue
		}
		entries, ok := parseKitInfoWattsEntries(quota[key])
		if !ok {
			continue
		}
		for i, entry := range entries {
			if entry.AvaFlag == 0 && entry.Soc == 0 && entry.F32Soc == 0 {
				continue
			}
			label := fmt.Sprintf("%s[%d]", key, i)
			if strings.TrimSpace(entry.SN) != "" {
				label = entry.SN
			}
			out = append(out, batterySOC{Label: label, SOC: entrySOC(entry)})
		}
	}

	return out
}

func (s *energySnapshot) effectiveBatteryChargeWatts() (float64, bool) {
	if s == nil {
		return 0, false
	}

	bestChargeWatts := 0.0
	hasSignal := false

	// Start with explicit system totals when both are present.
	if s.HasWattsIn && s.HasWattsOut {
		hasSignal = true
		netChargeWatts := s.WattsIn - s.WattsOut
		if netChargeWatts > idleDrawNoiseFloorWatts {
			bestChargeWatts = netChargeWatts
		}
	}

	// Fall back to channel totals (input sources minus external outputs).
	totalInputWatts := 0.0
	hasInput := false
	if s.HasInAC {
		totalInputWatts += s.InACWatts
		hasInput = true
	}
	if pvInputWatts, hasPVInput := s.effectivePVInputWatts(); hasPVInput {
		totalInputWatts += pvInputWatts
		hasInput = true
	}

	totalExternalOutputWatts := 0.0
	hasExternalOutput := false
	if s.HasOutAC {
		totalExternalOutputWatts += s.OutACWatts
		hasExternalOutput = true
	}
	if s.HasOutDC {
		totalExternalOutputWatts += s.OutDCWatts
		hasExternalOutput = true
	}

	if hasInput && hasExternalOutput {
		hasSignal = true
		netChargeWatts := totalInputWatts - totalExternalOutputWatts
		if netChargeWatts > idleDrawNoiseFloorWatts && netChargeWatts > bestChargeWatts {
			bestChargeWatts = netChargeWatts
		}
	}

	// Pack totals can be more complete than system net on Delta 2 style payloads,
	// where internal XT150 transfer may appear in wattsOutSum.
	packChargeWatts, _ := packPowerTotals(s.Packs)
	if packChargeWatts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(packChargeWatts); ok {
			hasSignal = true
			if sanitized > bestChargeWatts {
				bestChargeWatts = sanitized
			}
		}
	}

	// Last-resort fallback to direct battery hints.
	if s.HasBatteryIn && s.BatteryInWatts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(s.BatteryInWatts); ok {
			hasSignal = true
			if sanitized > bestChargeWatts {
				bestChargeWatts = sanitized
			}
		}
	}
	if s.HasXT150 && s.XT150Watts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(s.XT150Watts); ok {
			hasSignal = true
			if sanitized > bestChargeWatts {
				bestChargeWatts = sanitized
			}
		}
	}
	if !hasSignal {
		return 0, false
	}
	if bestChargeWatts <= idleDrawNoiseFloorWatts {
		return 0, true
	}
	return bestChargeWatts, true
}

func (s *energySnapshot) effectiveBatteryNetWatts() (float64, bool) {
	if s == nil {
		return 0, false
	}

	// Pack-level power is the most direct signal for battery charge/discharge direction.
	packChargeWatts, packDischargeWatts := packPowerTotals(s.Packs)
	if packChargeWatts > idleDrawNoiseFloorWatts || packDischargeWatts > idleDrawNoiseFloorWatts {
		net := packChargeWatts - packDischargeWatts
		if math.Abs(net) <= idleDrawNoiseFloorWatts {
			return 0, true
		}
		return net, true
	}

	net := 0.0
	hasSignal := false

	if s.HasBatteryIn && s.BatteryInWatts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(s.BatteryInWatts); ok {
			net += sanitized
			hasSignal = true
		}
	}
	if s.HasBatteryOut && s.BatteryOutWatts > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(s.BatteryOutWatts); ok {
			net -= sanitized
			hasSignal = true
		}
	}
	if s.HasXT150 && math.Abs(s.XT150Watts) > idleDrawNoiseFloorWatts {
		if sanitized, ok := s.sanitizeBatteryFlowHintWatts(math.Abs(s.XT150Watts)); ok {
			// XT150 sign convention: positive inverter->battery (charge), negative battery->inverter (discharge).
			if s.XT150Watts > 0 {
				net += sanitized
			} else {
				net -= sanitized
			}
			hasSignal = true
		}
	}
	if hasSignal {
		if math.Abs(net) <= idleDrawNoiseFloorWatts {
			return 0, true
		}
		return net, true
	}

	// Last fallback: aggregate power channels.
	if s.HasWattsIn && s.HasWattsOut {
		net = s.WattsIn - s.WattsOut
		if math.Abs(net) <= idleDrawNoiseFloorWatts {
			return 0, true
		}
		return net, true
	}
	return 0, false
}

func (s *energySnapshot) aggregateBatteryNetWatts() (float64, bool) {
	if s == nil {
		return 0, false
	}

	// Prefer direct aggregate counters, which are more stable for pack-based
	// systems where bpChgSta/bpPwr can be sparse or delayed.
	if s.HasWattsIn && s.HasWattsOut {
		net := s.WattsIn - s.WattsOut
		if math.Abs(net) <= idleDrawNoiseFloorWatts {
			return 0, true
		}
		return net, true
	}

	if s.HasBatteryIn || s.HasBatteryOut {
		net := 0.0
		if s.HasBatteryIn {
			net += s.BatteryInWatts
		}
		if s.HasBatteryOut {
			net -= s.BatteryOutWatts
		}
		if math.Abs(net) <= idleDrawNoiseFloorWatts {
			return 0, true
		}
		return net, true
	}

	totalInputWatts := 0.0
	hasInput := false
	if s.HasInAC {
		totalInputWatts += s.InACWatts
		hasInput = true
	}
	if pvInputWatts, hasPVInput := s.effectivePVInputWatts(); hasPVInput {
		totalInputWatts += pvInputWatts
		hasInput = true
	}

	totalOutputWatts := 0.0
	hasOutput := false
	if s.HasOutAC {
		totalOutputWatts += s.OutACWatts
		hasOutput = true
	}
	if s.HasOutDC {
		totalOutputWatts += s.OutDCWatts
		hasOutput = true
	}
	if !hasInput && !hasOutput {
		return 0, false
	}
	net := totalInputWatts - totalOutputWatts
	if math.Abs(net) <= idleDrawNoiseFloorWatts {
		return 0, true
	}
	return net, true
}

func extractBatteryPacks(quota map[string]any) map[int]packSnapshot {
	out := make(map[int]packSnapshot)
	for key, raw := range quota {
		lower := strings.ToLower(key)
		if lower != "bpinfo" && !strings.HasSuffix(lower, ".bpinfo") {
			continue
		}
		entries, ok := parseAnyArrayFromAny(raw)
		if !ok {
			continue
		}
		for i, entryRaw := range entries {
			entry, ok := entryRaw.(map[string]any)
			if !ok {
				continue
			}
			bpNo := int(toInt64(entry["bpNo"]))
			if bpNo <= 0 {
				bpNo = i + 1
			}
			pack := out[bpNo]
			if soc, ok := numberFromAny(entry["bpSoc"]); ok {
				pack.SOC = soc
				pack.HasSOC = true
			}
			if temp, ok := numberFromAny(entry["bpTemp"]); ok {
				pack.TempC = temp
				pack.HasTemp = true
			}
			if pwr, ok := numberFromAny(entry["bpPwr"]); ok {
				// Ignore zero-only bpInfo updates because they frequently represent sparse snapshots
				// and can overwrite useful non-zero power from richer telemetry frames.
				if pwr != 0 {
					setPackPower(&pack, pwr)
				}
			}
			if energy, ok := numberFromAny(entry["bpEnergy"]); ok {
				pack.EnergyWh = energy
				pack.HasEnergy = true
			}
			if maxSoc, ok := numberFromAny(entry["bpSocMax"]); ok && maxSoc >= 0 {
				pack.MaxSOC = maxSoc
				pack.HasMaxSOC = true
			}
			if minSoc, ok := numberFromAny(entry["bpSocMin"]); ok && minSoc >= 0 {
				pack.MinSOC = minSoc
				pack.HasMinSOC = true
			}
			if maxVolDiff, ok := numberFromAny(entry["maxVolDiff"]); ok && maxVolDiff >= 0 {
				pack.MaxVolDiff = maxVolDiff
				pack.HasMaxVolDiff = true
			}
			if chargeState, ok := numberFromAny(entry["bpChgSta"]); ok {
				pack.ChargeStateRaw = int64(chargeState)
				pack.HasChargeState = true
			}
			if heat := toInt64(entry["heatTime"]); heat >= 0 {
				pack.PreconditioningHeatTime = heat
				pack.HasPreconditioningHeat = true
				// Some bpInfo payloads only expose heatTime; use it as fallback ON/OFF state.
				if !pack.HasPreconditioningState {
					pack.PreconditioningOn = heat > 0
					pack.HasPreconditioning = true
				}
			}
			if stateRaw, ok := numberFromAny(entry["ptcMosState"]); ok {
				state := int64(stateRaw)
				pack.PreconditioningStateRaw = state
				pack.HasPreconditioningState = true
				pack.PreconditioningOn = state > 0
				pack.HasPreconditioning = true
			}
			if eventRaw, ok := numberFromAny(entry["ptcHeatingEvent"]); ok {
				event := int64(eventRaw)
				pack.PreconditioningEventRaw = event
				pack.HasPreconditioningEvent = true
			}
			if remain := toInt64(entry["remainTime"]); remain > 0 && !isLikelyRemainSentinel(remain) {
				pack.RemainTimeRaw = remain
			}
			out[bpNo] = pack
		}
	}
	return out
}

func formatPackSummary(packs map[int]*packSnapshot, kitSOC map[string]float64) string {
	if len(packs) > 0 {
		ids := make([]int, 0, len(packs))
		for id := range packs {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			pack := packs[id]
			label := fmt.Sprintf("bp%d", id)
			if pack.HasSOC {
				label += fmt.Sprintf("=%.1f%%", pack.SOC)
			}
			if pack.HasTemp {
				label += fmt.Sprintf("(%.1fC)", pack.TempC)
			}
			if isPackPowerFresh(pack) {
				label += fmt.Sprintf("[%s]", formatWatts(pack.PowerW))
			}
			parts = append(parts, label)
		}
		return strings.Join(parts, ",")
	}

	if len(kitSOC) > 0 {
		keys := make([]string, 0, len(kitSOC))
		for key := range kitSOC {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%.1f%%", key, kitSOC[key]))
		}
		return strings.Join(parts, ",")
	}

	return "n/a"
}

func (s *energySnapshot) maxReasonableBatteryFlowHintWatts() (float64, bool) {
	if s == nil {
		return 0, false
	}
	maxWatts := 0.0
	if s.HasParaChgMax && s.ParaChgMaxWatts > 0 {
		maxWatts = math.Max(maxWatts, s.ParaChgMaxWatts*1.15)
	}
	if s.HasC20ChgMax && s.C20ChgMaxWatts > 0 {
		// Dual charge on D2M can exceed C20 AC-only limit.
		maxWatts = math.Max(maxWatts, s.C20ChgMaxWatts*1.6)
	}
	if s.HasInAC || s.HasInPV {
		totalIn := 0.0
		if s.HasInAC {
			totalIn += s.InACWatts
		}
		if pvWatts, ok := s.effectivePVInputWatts(); ok {
			totalIn += pvWatts
		}
		if totalIn > 0 {
			maxWatts = math.Max(maxWatts, totalIn*1.6)
		}
	}
	if s.HasOutAC || s.HasOutDC {
		totalOut := 0.0
		if s.HasOutAC {
			totalOut += s.OutACWatts
		}
		if s.HasOutDC {
			totalOut += s.OutDCWatts
		}
		if totalOut > 0 {
			maxWatts = math.Max(maxWatts, totalOut*2.0)
		}
	}
	if maxWatts <= 0 {
		return 0, false
	}
	return maxWatts, true
}

func (s *energySnapshot) sanitizeBatteryFlowHintWatts(value float64) (float64, bool) {
	if value <= idleDrawNoiseFloorWatts || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	if maxWatts, ok := s.maxReasonableBatteryFlowHintWatts(); ok && maxWatts > 0 {
		if value > maxWatts {
			return 0, false
		}
	}
	return value, true
}

func setPackPower(pack *packSnapshot, watts float64) {
	if pack == nil {
		return
	}
	pack.PowerW = watts
	pack.HasPower = true
	pack.PowerUpdatedAt = time.Now()
}

func isPackPowerFresh(pack *packSnapshot) bool {
	if pack == nil || !pack.HasPower {
		return false
	}
	if pack.PowerUpdatedAt.IsZero() {
		return true
	}
	return time.Since(pack.PowerUpdatedAt) <= packPowerStaleAfter
}

func packPowerTotals(packs map[int]*packSnapshot) (chargeW float64, dischargeW float64) {
	for _, pack := range packs {
		if !isPackPowerFresh(pack) {
			continue
		}
		if pack.PowerW > 0 {
			chargeW += pack.PowerW
			continue
		}
		if pack.PowerW < 0 {
			dischargeW += -pack.PowerW
		}
	}
	return chargeW, dischargeW
}

func (s *energySnapshot) ensurePack(packNo int) *packSnapshot {
	pack, ok := s.Packs[packNo]
	if ok {
		return pack
	}
	pack = &packSnapshot{}
	s.Packs[packNo] = pack
	return pack
}

func (s *energySnapshot) applyGlobalRemain(remain int64, source string) {
	if remain <= 0 || isLikelyRemainSentinel(remain) {
		return
	}
	if s.HasRemainTime && !isLikelyRemainSentinel(s.RemainTimeRaw) {
		// Guard against known transient remainTime spikes in live appshow traffic
		// (for example 447 -> 45 -> 23 -> 186 -> 447).
		if strings.EqualFold(source, "pdStatus") && s.RemainTimeRaw >= 240 && remain < s.RemainTimeRaw/2 {
			return
		}
	}
	s.RemainTimeRaw = remain
	s.HasRemainTime = true
}

func (s *energySnapshot) applyChargeRemain(remain int64) {
	if remain <= 0 || isLikelyRemainSentinel(remain) {
		return
	}
	s.ChargeRemainTimeRaw = remain
	s.HasChargeRemainTime = true
}

func (s *energySnapshot) applyDischargeRemain(remain int64) {
	if remain <= 0 || isLikelyRemainSentinel(remain) {
		return
	}
	s.DischargeRemainTimeRaw = remain
	s.HasDischargeRemainTime = true
}

func (s *energySnapshot) applyRemainForCurrentState(remain int64) {
	if remain <= 0 || isLikelyRemainSentinel(remain) {
		return
	}
	packChargeW, packDischargeW := packPowerTotals(s.Packs)
	state := s.detectSystemState(s.WattsIn, s.HasWattsIn, s.WattsOut, s.HasWattsOut, packChargeW, packDischargeW)
	switch state {
	case systemStateCharging:
		s.applyChargeRemain(remain)
	case systemStateDischarging:
		s.applyDischargeRemain(remain)
	}
}

func isLikelyRemainSentinel(remain int64) bool {
	return remain >= 120000
}

func averagePackSOC(packs map[int]*packSnapshot) (float64, bool) {
	if len(packs) == 0 {
		return 0, false
	}
	total := 0.0
	count := 0
	for _, pack := range packs {
		if !pack.HasSOC {
			continue
		}
		total += pack.SOC
		count++
	}
	if count == 0 {
		return 0, false
	}
	return total / float64(count), true
}

func (s *energySnapshot) displaySOC() (float64, bool) {
	if weightedSOC, ok := weightedAveragePackSOC(s.Packs); ok {
		return weightedSOC, true
	}
	if avgSOC, ok := averagePackSOC(s.Packs); ok {
		return avgSOC, true
	}
	if s.HasDeviceSOC {
		return s.DeviceSOC, true
	}
	return 0, false
}

func weightedAveragePackSOC(packs map[int]*packSnapshot) (float64, bool) {
	if len(packs) == 0 {
		return 0, false
	}
	weightedTotal := 0.0
	weightSum := 0.0
	for _, pack := range packs {
		if pack == nil || !pack.HasSOC {
			continue
		}
		weight, ok := packSOCWeight(pack)
		if !ok {
			continue
		}
		weightedTotal += pack.SOC * weight
		weightSum += weight
	}
	if weightSum <= 0 {
		return 0, false
	}
	return weightedTotal / weightSum, true
}

func packSOCWeight(pack *packSnapshot) (float64, bool) {
	if pack == nil {
		return 0, false
	}
	switch {
	case pack.HasDesignCap && pack.DesignCap > 0:
		return pack.DesignCap, true
	case pack.HasFullCap && pack.FullCap > 0:
		return pack.FullCap, true
	case pack.HasEnergy && pack.EnergyWh > 0:
		return pack.EnergyWh, true
	default:
		return 0, false
	}
}
