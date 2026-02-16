package main

import "strings"

func (s *energySnapshot) applyMPPTStatusQuota(quota map[string]any) {
	if s == nil {
		return
	}
	if state, ok := firstNumberFromKeys(quota, "chgState"); ok {
		s.PVLowChgStateRaw = int64(state)
		s.HasPVLowChgState = true
	}
	if state, ok := firstNumberFromKeys(quota, "pv2ChgState"); ok {
		s.PVHighChgStateRaw = int64(state)
		s.HasPVHighChgState = true
	}
	if value, ok := firstNumberFromKeys(quota, "inWatts"); ok {
		s.InPVLowWatts = normalizeInputChannelWatts(value)
		s.HasInPVLow = true
	} else if s.HasPVLowChgState && !isMPPTChargeStateActive(s.PVLowChgStateRaw) {
		// Explicitly inactive MPPT channel; avoid stale/non-physical V*I fallback.
		s.InPVLowWatts = 0
		s.HasInPVLow = true
	}
	if value, ok := firstNumberFromKeys(quota, "pv2InWatts"); ok {
		s.InPVHighWatts = normalizeInputChannelWatts(value)
		s.HasInPVHigh = true
	} else if s.HasPVHighChgState && !isMPPTChargeStateActive(s.PVHighChgStateRaw) {
		// Explicitly inactive MPPT channel; avoid stale/non-physical V*I fallback.
		s.InPVHighWatts = 0
		s.HasInPVHigh = true
	}
	if value, ok := firstNumberFromKeys(quota, "inVol"); ok {
		s.SolarLVVolts = normalizeMPPTVoltageVolts(value)
		s.HasSolarLVVolts = true
	}
	if value, ok := firstNumberFromKeys(quota, "inAmp"); ok {
		s.SolarLVAmp = normalizeMPPTCurrentAmps(value)
		s.HasSolarLVAmp = true
	}
	if value, ok := firstNumberFromKeys(quota, "pv2InVol"); ok {
		s.SolarHVVolts = normalizeMPPTVoltageVolts(value)
		s.HasSolarHVVolts = true
	}
	if value, ok := firstNumberFromKeys(quota, "pv2InAmp"); ok {
		s.SolarHVAmp = normalizeMPPTCurrentAmps(value)
		s.HasSolarHVAmp = true
	}
	s.refreshPVTotalFromChannels()
}

func isMPPTStatusEnvelope(envelope telemetryEnvelope) bool {
	return strings.EqualFold(envelope.TypeCode, "mpptStatus")
}
