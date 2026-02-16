package main

import (
	"fmt"
	"math"
	"time"
)

const sensorUpdateHistorySize = 5

type sensorStatusSample struct {
	key    string
	label  string
	on     bool
	known  bool
	status string
}

func (s *energySnapshot) recordSensorStatusTransitions(at time.Time) {
	if s == nil {
		return
	}
	if s.sensorStateLast == nil {
		s.sensorStateLast = make(map[string]bool)
	}
	if s.sensorStateSeen == nil {
		s.sensorStateSeen = make(map[string]bool)
	}

	for _, sample := range s.currentSensorStatusSamples() {
		if !sample.known {
			continue
		}
		prevSeen := s.sensorStateSeen[sample.key]
		prev := s.sensorStateLast[sample.key]
		if !prevSeen || prev != sample.on {
			s.appendSensorUpdate(sensorUpdateEvent{
				At:     at,
				Sensor: sample.label,
				Status: sample.status,
			})
		}
		s.sensorStateSeen[sample.key] = true
		s.sensorStateLast[sample.key] = sample.on
	}
}

func (s *energySnapshot) appendSensorUpdate(event sensorUpdateEvent) {
	if s == nil {
		return
	}
	s.sensorUpdates = append(s.sensorUpdates, event)
	if len(s.sensorUpdates) > sensorUpdateHistorySize {
		s.sensorUpdates = s.sensorUpdates[len(s.sensorUpdates)-sensorUpdateHistorySize:]
	}
}

func (s *energySnapshot) recentSensorUpdates(limit int) []sensorUpdateEvent {
	if s == nil || len(s.sensorUpdates) == 0 {
		return nil
	}
	if limit <= 0 || limit > len(s.sensorUpdates) {
		limit = len(s.sensorUpdates)
	}
	out := make([]sensorUpdateEvent, limit)
	copy(out, s.sensorUpdates[len(s.sensorUpdates)-limit:])
	return out
}

func (s *energySnapshot) currentSensorStatusSamples() []sensorStatusSample {
	if s == nil {
		return nil
	}

	samples := make([]sensorStatusSample, 0, 12)

	acOn := s.ACOn
	hasAC := s.HasACOn || s.HasOutAC || s.HasOutACL14
	if s.HasOutAC && s.OutACWatts > 0 {
		acOn = true
	}
	if s.HasOutACL14 && s.OutACL14Watts > 0 {
		acOn = true
	}
	samples = append(samples, sensorStatusSample{
		key:    "ac_charging",
		label:  "AC charging",
		on:     acOn,
		known:  hasAC,
		status: onOffTransitionText(acOn),
	})

	dcOn := s.DCOn
	hasDC := s.HasDCOn || s.HasOutDC
	if s.HasOutDC && s.OutDCWatts > 0 {
		dcOn = true
	}

	usbOn := s.USBOn
	hasUSB := s.HasUSBOn
	if !hasUSB && hasDC {
		usbOn = dcOn
		hasUSB = true
	}
	if hasUSB {
		samples = append(samples, sensorStatusSample{
			key:    "usb",
			label:  "USB",
			on:     usbOn,
			known:  true,
			status: onOffTransitionText(usbOn),
		})
	}
	if s.HasDC12VOn {
		samples = append(samples, sensorStatusSample{
			key:    "dc_12v",
			label:  "12V DC",
			on:     s.DC12VOn,
			known:  true,
			status: onOffTransitionText(s.DC12VOn),
		})
	}
	if !hasUSB && hasDC {
		samples = append(samples, sensorStatusSample{
			key:    "dc_usb",
			label:  "DC/USB",
			on:     dcOn,
			known:  true,
			status: onOffTransitionText(dcOn),
		})
	}

	if s.HasEVChargingOn {
		samples = append(samples, sensorStatusSample{
			key:    "ev_charging",
			label:  "EV charging",
			on:     s.EVChargingOn,
			known:  true,
			status: onOffTransitionText(s.EVChargingOn),
		})
	}

	fanKnown := s.HasFanOn || s.HasFanLevel
	fanOn := s.FanOn
	if !s.HasFanOn && s.HasFanLevel {
		fanOn = s.FanLevelRaw > 0
	}
	if fanKnown {
		samples = append(samples, sensorStatusSample{
			key:    "fan",
			label:  "Fan",
			on:     fanOn,
			known:  true,
			status: onOffTransitionText(fanOn),
		})
	}

	acOutWatts := 0.0
	hasOutAC := false
	if s.HasOutAC {
		acOutWatts = math.Max(acOutWatts, math.Abs(s.OutACWatts))
		hasOutAC = true
	}
	if s.HasOutACL14 {
		acOutWatts = math.Max(acOutWatts, math.Abs(s.OutACL14Watts))
		hasOutAC = true
	}
	passthroughKnown := s.HasInAC && hasOutAC
	passthroughOn := false
	if passthroughKnown {
		passthroughOn = isLikelyACPassthrough(true, s.InACWatts, true, acOutWatts)
	}
	if passthroughKnown {
		samples = append(samples, sensorStatusSample{
			key:    "ups_passthrough",
			label:  "UPS passthrough",
			on:     passthroughOn,
			known:  true,
			status: onOffTransitionText(passthroughOn),
		})
		samples = append(samples, sensorStatusSample{
			key:    "grounded_est",
			label:  "Grounded (Estimated)",
			on:     passthroughOn,
			known:  true,
			status: onOffTransitionText(passthroughOn),
		})
	}

	packChargeW, packDischargeW := packPowerTotals(s.Packs)
	effectiveIn, hasEffectiveIn, effectiveOut, hasEffectiveOut :=
		s.effectiveTotalsForDisplayWithPackTotals(packChargeW, packDischargeW)
	batteryFlow := s.batteryFlowForDisplay(
		effectiveIn,
		hasEffectiveIn,
		effectiveOut,
		hasEffectiveOut,
		packChargeW,
		packDischargeW,
	)
	solarPassKnown := s.HasInAC || s.HasInPV || s.HasInPVLow || s.HasInPVHigh || hasOutAC || batteryFlow.hasNet
	solarPassOn := isLikelySolarPassthrough(s, batteryFlow.inWatts, batteryFlow.hasIn, batteryFlow.outWatts, batteryFlow.hasOut)
	if solarPassKnown {
		samples = append(samples, sensorStatusSample{
			key:    "solar_passthrough",
			label:  "Solar passthrough",
			on:     solarPassOn,
			known:  true,
			status: onOffTransitionText(solarPassOn),
		})
	}

	solarChargingKnown, solarChargingOn := s.solarChargingStatus()
	if solarChargingKnown {
		samples = append(samples, sensorStatusSample{
			key:    "solar_charging",
			label:  "Solar charging",
			on:     solarChargingOn,
			known:  true,
			status: onOffTransitionText(solarChargingOn),
		})
	}

	preconditioningKnown, preconditioningOn := overallPreconditioningStatus(s.Packs)
	if preconditioningKnown {
		samples = append(samples, sensorStatusSample{
			key:    "battery_preconditioning",
			label:  "Battery preconditioning",
			on:     preconditioningOn,
			known:  true,
			status: onOffTransitionText(preconditioningOn),
		})
	}

	return samples
}

func onOffTransitionText(on bool) string {
	if on {
		return "turned On"
	}
	return "turned Off"
}

func buildSensorUpdateRows(snapshot *energySnapshot, limit int) [][]string {
	if snapshot == nil {
		return nil
	}
	updates := snapshot.recentSensorUpdates(limit)
	if len(updates) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(updates))
	for i := len(updates) - 1; i >= 0; i-- {
		update := updates[i]
		notification := buildNotificationText(update.Sensor, update.Status)
		statusIndicator := buildStatusIndicator(update.Status)
		rows = append(rows, []string{
			update.At.Local().Format("15:04:05"),
			notification,
			statusIndicator,
		})
	}
	return rows
}

func buildNotificationText(sensor, status string) string {
	if sensor == "" {
		return status
	}
	if status == "" {
		return sensor
	}
	return sensor + " " + status
}

func buildStatusIndicator(status string) string {
	switch status {
	case "turned On":
		return "🟢"
	case "turned Off":
		return "⚪"
	default:
		return status
	}
}

func (s sensorStatusSample) String() string {
	return fmt.Sprintf("%s=%t (%t)", s.key, s.on, s.known)
}
