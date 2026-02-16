package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

type dashboardViewModel struct {
	topic string

	deviceHeaders []string
	deviceRows    [][]string

	summaryHeaders []string
	summaryRows    [][]string

	packHeaders []string
	packRows    [][]string

	packDiagHeaders []string
	packDiagRows    [][]string

	minuteHeaders []string
	minuteRows    [][]string

	estimateHeaders []string
	estimateRows    [][]string

	sensorHeaders []string
	sensorRows    [][]string

	statusLines []string
}

func buildDashboardViewModel(
	device ecoflow.GeneralInfoDevice,
	topic string,
	_ telemetryEnvelope,
	snapshot *energySnapshot,
	minuteHistory *minuteTelemetryHistory,
	minuteCfg minuteTableConfig,
) dashboardViewModel {
	derived := snapshot.derived()
	pvTotalRaw, hasPVTotalRaw, pvLowRaw, hasPVLowRaw, pvHighRaw, hasPVHighRaw := snapshot.effectivePVInputChannels()
	pvTotalSmooth, hasPVTotalSmooth, pvLowSmooth, hasPVLowSmooth, pvHighSmooth, hasPVHighSmooth := snapshot.smoothedPVChannels()
	acInSmooth, hasACInSmooth := snapshot.smoothedACInput()
	totalInSmooth, hasTotalInSmooth, totalOutSmooth, hasTotalOutSmooth := snapshot.smoothedTotalChannels()
	pvTotalDisplay := formatSmoothedWattsValue(derived.InPVValue, hasPVTotalRaw, pvTotalRaw, hasPVTotalSmooth, pvTotalSmooth)
	pvLowDisplay := formatSmoothedWattsValue(derived.InPVLowValue, hasPVLowRaw, pvLowRaw, hasPVLowSmooth, pvLowSmooth)
	pvHighDisplay := formatSmoothedWattsValue(derived.InPVHighValue, hasPVHighRaw, pvHighRaw, hasPVHighSmooth, pvHighSmooth)
	acInDisplay := formatSmoothedWattsValue(derived.InACValue, snapshot.HasInAC, snapshot.InACWatts, hasACInSmooth, acInSmooth)
	totalInDisplay := formatSmoothedWattsValue(derived.InputValue, derived.HasEffectiveIn, derived.EffectiveIn, hasTotalInSmooth, totalInSmooth)
	totalOutDisplay := formatSmoothedWattsValue(derived.OutputValue, derived.HasEffectiveOut, derived.EffectiveOut, hasTotalOutSmooth, totalOutSmooth)
	totalNetDisplay := derived.NetValue
	hasRawNet := derived.HasEffectiveIn && derived.HasEffectiveOut
	rawNet := derived.EffectiveIn - derived.EffectiveOut
	hasSmoothNet := hasTotalInSmooth && hasTotalOutSmooth
	smoothNet := totalInSmooth - totalOutSmooth
	if hasRawNet {
		totalNetDisplay = formatSmoothedWattsValue(derived.NetValue, true, rawNet, hasSmoothNet, smoothNet)
	}
	mqttQueueValue := "n/a"
	mqttDropsValue := "n/a"
	if snapshot.MQTTQueueCapacity > 0 {
		mqttQueueValue = fmt.Sprintf("%d/%d", snapshot.MQTTQueueDepth, snapshot.MQTTQueueCapacity)
		mqttDropsValue = fmt.Sprintf("drop-oldest: %d", snapshot.MQTTQueueDroppedOldest)
	}
	mqttStatusValue := formatMQTTStatus(snapshot)
	pvLowLabel := formatPVInputRowLabel("low", device, snapshot)
	pvHighLabel := formatPVInputRowLabel("high", device, snapshot)
	pvLowCapability := formatPVInputCapability("low", device, snapshot)
	pvHighCapability := formatPVInputCapability("high", device, snapshot)
	pvLowUtilization := formatPVUtilizationGauge("low", device, snapshot, hasPVLowRaw, pvLowRaw, hasPVLowSmooth, pvLowSmooth)
	pvHighUtilization := formatPVUtilizationGauge("high", device, snapshot, hasPVHighRaw, pvHighRaw, hasPVHighSmooth, pvHighSmooth)
	showXT150 := shouldShowXT150Channels(device, snapshot, derived)

	channelsInValue := fmt.Sprintf("ac: %s pv_total: %s", derived.InACValue, pvTotalDisplay)
	channelsOutValue := fmt.Sprintf("ac: %s (l14: %s) dc: %s", derived.OutACValue, derived.OutACL14Value, derived.OutDCValue)
	if showXT150 {
		channelsInValue = fmt.Sprintf("%s xt150_in: %s", channelsInValue, derived.XT150InValue)
		channelsOutValue = fmt.Sprintf("%s xt150_out: %s", channelsOutValue, derived.XT150OutValue)
	}

	updatedAt := formatSnapshotUpdatedRelative(snapshot)
	deviceHeaders := []string{"Icon", "Device Name", "SOC", "AC In", "Solar Generated", "Out", "Net", "State", "Model", "Updated"}
	summaryHeaders := []string{
		"Details",
		"In",
		"Out",
		"Net",
		"Details",
	}
	stateKind := systemStateKind(derived.SystemStateValue)
	originalMLEstimates := estimateBatteryETAsML(snapshot, minuteHistory, stateKind)
	genericMLEstimates := estimateBatteryETAsMLGeneric(snapshot, minuteHistory, stateKind)
	newMLEstimates, newMLProfile := estimateBatteryETAsMLProfiled(snapshot, minuteHistory, stateKind)
	unitEstimates := estimateBatteryETAsUnitSpecific(snapshot, stateKind)

	topStateEstimate, topStateEstimateModel := pickPreferredMLForTopState(
		unitEstimates,
		genericMLEstimates,
		newMLEstimates,
		stateKind,
	)
	batterySource := inferBatteryChargeSource(snapshot, derived)
	topStateValue, topStateUsedML := selectTopStateValueWithSource(
		snapshot,
		derived.RemainValue,
		stateKind,
		topStateEstimate,
	)
	topStateModel := "MPPT"
	if topStateUsedML {
		topStateModel = topStateEstimateModel
	}
	topStateValue = sanitizeStateColumnValue(topStateValue)
	topStateValue = annotateIdleStateWithIncoming(topStateValue, stateKind, totalInDisplay)
	topStateIcon := topStateDisplayIcon(stateKind, topStateValue, batterySource)
	iconCell := firstNonEmpty(topStateIcon, "·")
	deviceLabel := chooseDeviceLabel(device)
	topSOCDisplay := formatSOCUnavailableWithGauge(10)
	if soc, ok := snapshot.displaySOC(); ok {
		topSOCDisplay = formatSOCWithGauge(soc, 10)
	}
	deviceRows := [][]string{{
		iconCell,
		deviceLabel,
		topSOCDisplay,
		acInDisplay,
		pvTotalDisplay,
		totalOutDisplay,
		totalNetDisplay,
		topStateValue,
		topStateModel,
		updatedAt,
	}}
	summaryRows := [][]string{
		{
			"channels",
			channelsInValue,
			channelsOutValue,
			derived.ChannelsNetValue,
			"-",
		},
		{
			"meta",
			fmt.Sprintf("packs: %s showFlag: %s", derived.BatteryCount, derived.ShowFlagValue),
			fmt.Sprintf("combo: %s c20: %s para: %s", derived.ComboValue, derived.C20LimitValue, derived.ParaLimitValue),
			fmt.Sprintf("socWindow: %s", derived.SocGuardrail),
			"-",
		},
		{
			pvLowLabel,
			pvLowCapability,
			fmt.Sprintf("volts: %s amps: %s watts: %s", derived.PVLowVoltsValue, derived.PVLowAmpsValue, pvLowDisplay),
			formatSolarNetSummary(derived.PVLowStateValue, pvLowDisplay),
			pvLowUtilization,
		},
		{
			pvHighLabel,
			pvHighCapability,
			fmt.Sprintf("volts: %s amps: %s watts: %s", derived.PVHighVoltsValue, derived.PVHighAmpsValue, pvHighDisplay),
			formatSolarNetSummary(derived.PVHighStateValue, pvHighDisplay),
			pvHighUtilization,
		},
		{
			"battery",
			fmt.Sprintf("in: %s", derived.BatteryInValue),
			fmt.Sprintf("out: %s idle: %s", derived.BatteryOutValue, derived.IdleDrawValue),
			fmt.Sprintf("%s source: %s", derived.BatteryNetValue, batterySource),
			"-",
		},
		{
			"mqtt",
			fmt.Sprintf("queue: %s", mqttQueueValue),
			mqttDropsValue,
			fmt.Sprintf("status: %s", mqttStatusValue),
			"-",
		},
	}

	estimateHeaders := []string{"Model", "Charge", "Discharge", "Power", "Confidence", "Δ ETA vs Unit", "Δ Power vs Unit"}
	estimateRows := [][]string{
		{
			"MPPT",
			firstNonEmpty(strings.TrimSpace(unitEstimates.ChargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(unitEstimates.DischargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(unitEstimates.PowerValue), "power: n/a"),
			firstNonEmpty(strings.TrimSpace(unitEstimates.ConfidenceValue), "n/a"),
			"-",
			"-",
		},
		{
			"Old",
			firstNonEmpty(strings.TrimSpace(originalMLEstimates.ChargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(originalMLEstimates.DischargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(originalMLEstimates.PowerValue), "power: n/a"),
			firstNonEmpty(strings.TrimSpace(originalMLEstimates.ConfidenceValue), "n/a"),
			estimateDeltaMinutesDisplay(unitEstimates, originalMLEstimates, stateKind),
			estimateDeltaPowerDisplay(unitEstimates, originalMLEstimates, stateKind),
		},
		{
			"Generic",
			firstNonEmpty(strings.TrimSpace(genericMLEstimates.ChargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(genericMLEstimates.DischargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(genericMLEstimates.PowerValue), "power: n/a"),
			firstNonEmpty(strings.TrimSpace(genericMLEstimates.ConfidenceValue), "n/a"),
			estimateDeltaMinutesDisplay(unitEstimates, genericMLEstimates, stateKind),
			estimateDeltaPowerDisplay(unitEstimates, genericMLEstimates, stateKind),
		},
		{
			fmt.Sprintf("New (%s)", strings.ToUpper(string(newMLProfile))),
			firstNonEmpty(strings.TrimSpace(newMLEstimates.ChargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(newMLEstimates.DischargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(newMLEstimates.PowerValue), "power: n/a"),
			firstNonEmpty(strings.TrimSpace(newMLEstimates.ConfidenceValue), "n/a"),
			estimateDeltaMinutesDisplay(unitEstimates, newMLEstimates, stateKind),
			estimateDeltaPowerDisplay(unitEstimates, newMLEstimates, stateKind),
		},
	}

	packHeaders := []string{"Pack", "SOC", "Temp", "Power", "Remain", "MaxΔmV", "State"}
	packRows := make([][]string, 0, len(snapshot.Packs))
	ids := make([]int, 0, len(snapshot.Packs))
	for id := range snapshot.Packs {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		pack := snapshot.Packs[id]
		soc := formatSOCUnavailableWithGauge(8)
		if pack.HasSOC {
			soc = formatSOCWithGauge(pack.SOC, 8)
		}
		temp := "n/a"
		if pack.HasTemp {
			temp = fmt.Sprintf("%.1fC", pack.TempC)
		}
		power := "n/a"
		if isPackPowerFresh(pack) {
			power = formatWatts(pack.PowerW)
		}
		remain := "n/a"
		if pack.RemainTimeRaw > 0 && !isLikelyRemainSentinel(pack.RemainTimeRaw) {
			remain = fmt.Sprintf("%dmin (~%s)", pack.RemainTimeRaw, formatMinutesHuman(pack.RemainTimeRaw))
		}
		maxVolDiff := "n/a"
		if pack.HasMaxVolDiff {
			maxVolDiff = fmt.Sprintf("%.0f", pack.MaxVolDiff)
		}
		packRows = append(packRows, []string{
			fmt.Sprintf("bp%d", id),
			soc,
			temp,
			power,
			remain,
			maxVolDiff,
			packStateLabel(pack),
		})
	}
	if len(packRows) == 0 {
		packRows = append(packRows, []string{"n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "waiting"})
	}

	packDiagHeaders := []string{"Pack", "Serial", "Energy", "SOH", "Voltage", "SOC(target)", "Limits", "ΔSOC", "Cap(rem/full)", "Board"}
	packDiagRows := make([][]string, 0, len(snapshot.Packs))
	for _, id := range ids {
		pack := snapshot.Packs[id]
		packDiagRows = append(packDiagRows, []string{
			fmt.Sprintf("bp%d", id),
			formatPackSerial(pack),
			formatPackEnergy(pack),
			formatPackSoh(pack),
			formatPackVoltage(pack),
			formatPackTarget(pack),
			formatPackLimits(pack),
			formatPackDiffSOC(pack),
			formatPackCapacity(pack),
			formatPackBoardTemp(pack),
		})
	}
	if len(packDiagRows) == 0 {
		packDiagRows = append(packDiagRows, []string{"n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a"})
	}

	minuteHeaders := []string{
		"Time",
		"Battery SOC (%)",
		"Solar Generated (Wh)",
		"AC Input (Wh)",
		"AC Output (Wh)",
		"DC Output (Wh)",
		"Battery Charge (Wh)",
		"Total Input (Wh)",
		"Total Output (Wh)",
		"Net (Wh)",
	}
	minuteRows := buildMinuteTelemetryRows(minuteHistory, minuteCfg)
	if len(minuteRows) == 0 {
		minuteRows = append(minuteRows, []string{"n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a"})
	}
	sensorHeaders := []string{"Time", "Notification", "Status"}
	sensorRows := buildSensorUpdateRows(snapshot, sensorUpdateHistorySize)
	if len(sensorRows) == 0 {
		sensorRows = append(sensorRows, []string{"n/a", "n/a", "n/a"})
	}

	statusLines := make([]string, 0, 8)
	showSeparateUSBAndDC := shouldShowSeparateUSBAndDC(device, snapshot)
	showEVStatus := shouldShowEVStatus(device, snapshot)
	showFanStatus := shouldShowFanStatus(device, snapshot)
	showPreconditioningStatus := shouldShowPreconditioningStatus(device, snapshot)
	statusLines = append(statusLines, fmt.Sprintf("%s AC On", derived.StatusACValue))
	if showSeparateUSBAndDC {
		statusLines = append(statusLines, fmt.Sprintf("%s USB On", derived.StatusUSBValue))
		statusLines = append(statusLines, fmt.Sprintf("%s 12V DC On", derived.StatusDC12VValue))
	} else {
		statusLines = append(statusLines, fmt.Sprintf("%s DC/USB On", derived.StatusDCValue))
	}
	if showEVStatus {
		statusLines = append(statusLines, fmt.Sprintf("%s EV Charging On", derived.StatusEVValue))
	}
	if showFanStatus {
		statusLines = append(statusLines, fmt.Sprintf("%s Fan On", derived.StatusFanValue))
	}
	statusLines = append(statusLines, fmt.Sprintf("%s UPS Passthrough", derived.StatusPassthroughValue))
	statusLines = append(statusLines, fmt.Sprintf("%s Solar Passthrough", derived.StatusSolarPassValue))
	statusLines = append(statusLines, fmt.Sprintf("%s Solar Charging", derived.StatusSolarChargingValue))
	statusLines = append(statusLines, fmt.Sprintf("%s Grounded (Estimated)", derived.StatusGroundedValue))
	if showPreconditioningStatus {
		statusLines = append(statusLines, fmt.Sprintf("%s Battery Preconditioning On", derived.StatusPrecondValue))
	}

	return dashboardViewModel{
		topic: topic,

		deviceHeaders: deviceHeaders,
		deviceRows:    deviceRows,

		summaryHeaders: summaryHeaders,
		summaryRows:    summaryRows,

		packHeaders: packHeaders,
		packRows:    packRows,

		packDiagHeaders: packDiagHeaders,
		packDiagRows:    packDiagRows,

		minuteHeaders: minuteHeaders,
		minuteRows:    minuteRows,

		estimateHeaders: estimateHeaders,
		estimateRows:    estimateRows,

		sensorHeaders: sensorHeaders,
		sensorRows:    sensorRows,

		statusLines: statusLines,
	}
}
func chooseDeviceLabel(device ecoflow.GeneralInfoDevice) string {
	name := strings.TrimSpace(device.DeviceName)
	if name == "" {
		return device.SN
	}
	return name
}

func firstNonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func selectTopStateValue(
	snapshot *energySnapshot,
	deviceReported string,
	state systemStateKind,
	ml batteryETAEstimates,
) string {
	value, _ := selectTopStateValueWithSource(snapshot, deviceReported, state, ml)
	return value
}

func selectTopStateValueWithSource(
	snapshot *energySnapshot,
	deviceReported string,
	state systemStateKind,
	ml batteryETAEstimates,
) (string, bool) {
	deviceReported = strings.TrimSpace(deviceReported)
	if deviceReported == "" {
		deviceReported = "n/a"
	}

	mlValue := formatStateETAForDisplay(state, ml.ActiveValue)
	mlReady := isMLEstimateReady(ml) && mlValue != "n/a"
	score, hasScore := parseConfidenceScore(ml.ConfidenceValue)
	effectiveScore := score
	if snapshot != nil && hasScore {
		if !snapshot.hasMLConfEWMA {
			snapshot.mlConfidenceEWMA = score
			snapshot.hasMLConfEWMA = true
		} else {
			const alpha = 0.35
			snapshot.mlConfidenceEWMA = alpha*score + (1-alpha)*snapshot.mlConfidenceEWMA
		}
		effectiveScore = snapshot.mlConfidenceEWMA
	}
	if snapshot != nil {
		const enableThreshold = 0.84
		const disableThreshold = 0.72
		if !mlReady {
			snapshot.mlTopStateUse = false
		} else if snapshot.mlTopStateUse {
			if hasScore && effectiveScore < disableThreshold {
				snapshot.mlTopStateUse = false
			}
		} else if hasScore && effectiveScore >= enableThreshold {
			snapshot.mlTopStateUse = true
		}
		if snapshot.mlTopStateUse {
			return mlValue, true
		}
	} else if mlReady && confidenceTierFromValue(ml.ConfidenceValue) == "high" {
		return mlValue, true
	}
	if deviceReported != "n/a" {
		return deviceReported, false
	}
	if mlReady {
		return mlValue, true
	}
	return "n/a", false
}

func topStateDisplayIcon(state systemStateKind, value string, source string) string {
	displayState := state
	lower := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.HasPrefix(lower, "charging:"):
		displayState = systemStateCharging
	case strings.HasPrefix(lower, "discharging:"):
		displayState = systemStateDischarging
	case strings.HasPrefix(lower, "idle:"):
		displayState = systemStateIdle
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if displayState == systemStateCharging {
		switch source {
		case "solar":
			return "🌞"
		case "hybrid(ac+solar)":
			return "🔆"
		case "ac":
			return "🌩"
		default:
			return "🌩"
		}
	}
	switch displayState {
	case systemStateDischarging:
		return "🔻"
	case systemStateIdle:
		return "🟢"
	default:
		return ""
	}
}

func sanitizeStateColumnValue(value string) string {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"⚡ ", "↓ ", "⏸ "} {
		if strings.HasPrefix(value, prefix) {
			value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
		}
	}
	return value
}

func annotateIdleStateWithIncoming(value string, state systemStateKind, incomingWatts string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "n/a" {
		return value
	}
	lower := strings.ToLower(value)
	if state != systemStateIdle && !strings.HasPrefix(lower, "idle") {
		return value
	}
	if incomingWatts == "" || incomingWatts == "n/a" {
		return value
	}
	if strings.Contains(lower, "in:") {
		return value
	}
	return fmt.Sprintf("%s (in: %s)", value, incomingWatts)
}

func formatSolarNetSummary(stateValue string, wattsDisplay string) string {
	trimmedState := strings.TrimSpace(stateValue)
	lowerState := strings.ToLower(trimmedState)
	switch {
	case strings.HasPrefix(lowerState, "active"):
		watts := extractDisplayWattsWithAvg(wattsDisplay)
		if watts == "" {
			watts = extractPVWattsFromState(trimmedState)
		}
		if watts == "" {
			return "active"
		}
		return fmt.Sprintf("active: %s", watts)
	case strings.HasPrefix(lowerState, "locked"):
		return trimmedState
	case strings.HasPrefix(lowerState, "idle"):
		return "idle"
	case trimmedState == "" || strings.EqualFold(trimmedState, "n/a"):
		return "idle"
	default:
		return trimmedState
	}
}

func extractDisplayWattsWithAvg(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "n/a") {
		return ""
	}
	return value
}

func extractPVWattsFromState(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "active(") || !strings.HasSuffix(value, ")") {
		return ""
	}
	openIdx := strings.Index(value, "(")
	closeIdx := strings.LastIndex(value, ")")
	if openIdx < 0 || closeIdx <= openIdx+1 {
		return ""
	}
	return strings.TrimSpace(value[openIdx+1 : closeIdx])
}

func inferBatteryChargeSource(snapshot *energySnapshot, derived snapshotDerived) string {
	if snapshot == nil {
		return "n/a"
	}

	const sourceMinWatts = systemStateNetThresholdWatts
	state := systemStateKind(strings.TrimSpace(derived.SystemStateValue))

	acInWatts := 0.0
	hasACIn := false
	if snapshot.HasInAC {
		acInWatts = math.Abs(snapshot.InACWatts)
		hasACIn = true
	}
	acOutWatts := 0.0
	hasACOut := false
	if snapshot.HasOutAC {
		acOutWatts = math.Abs(snapshot.OutACWatts)
		hasACOut = true
	}
	if snapshot.HasOutACL14 {
		l14 := math.Abs(snapshot.OutACL14Watts)
		if !hasACOut || l14 > acOutWatts {
			acOutWatts = l14
			hasACOut = true
		}
	}

	pvInWatts := 0.0
	hasPVIn := false
	if watts, ok := snapshot.effectivePVInputWatts(); ok {
		pvInWatts = math.Abs(watts)
		hasPVIn = true
	}
	// Some D2M streams may intermittently omit direct AC-in while totals remain valid.
	// In that case, infer AC input from total in minus PV so source icon can reflect hybrid charging.
	if !hasACIn && derived.HasEffectiveIn && hasPVIn {
		inferredACIn := derived.EffectiveIn - pvInWatts
		if inferredACIn > sourceMinWatts {
			acInWatts = inferredACIn
			hasACIn = true
		}
	}

	// Do not count AC passthrough load as charging source; only residual AC input
	// above load should be attributed to battery charging.
	acChargeWatts := acInWatts
	if hasACIn && hasACOut {
		residual := acInWatts - acOutWatts
		switch {
		case residual > sourceMinWatts:
			acChargeWatts = residual
		case isLikelyACPassthrough(true, acInWatts, true, acOutWatts):
			acChargeWatts = 0
		}
	}
	acActive := hasACIn && acChargeWatts > sourceMinWatts
	pvActive := hasPVIn && pvInWatts > sourceMinWatts

	switch state {
	case systemStateCharging:
		switch {
		case acActive && pvActive:
			return "hybrid(ac+solar)"
		case acActive:
			return "ac"
		case pvActive:
			return "solar"
		case hasACIn || hasPVIn:
			return "unknown"
		default:
			return "n/a"
		}
	case systemStateDischarging:
		if pvActive && !acActive {
			return "battery+solar"
		}
		return "battery"
	case systemStateIdle:
		switch {
		case acActive && pvActive:
			return "hybrid(ac+solar)"
		case acActive:
			return "ac"
		case pvActive:
			return "solar"
		default:
			return "none"
		}
	default:
		switch {
		case acActive && pvActive:
			return "hybrid(ac+solar)"
		case acActive:
			return "ac"
		case pvActive:
			return "solar"
		default:
			return "n/a"
		}
	}
}

func extractPrimaryWattsDisplay(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "n/a" {
		return ""
	}
	if idx := strings.Index(value, " (~"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	value = strings.TrimSpace(value)
	if !strings.HasSuffix(value, "W") || strings.HasSuffix(value, "kW") {
		return value
	}
	if n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, "W")), 64); err == nil {
		if math.Abs(n) <= 0.05 {
			return ""
		}
	}
	return value
}

func formatStateETAForDisplay(state systemStateKind, activeETA string) string {
	activeETA = strings.TrimSpace(activeETA)
	if activeETA == "" || activeETA == "n/a" {
		return "n/a"
	}
	prefix := ""
	switch state {
	case systemStateCharging:
		prefix = "charging"
	case systemStateDischarging:
		prefix = "discharging"
	case systemStateIdle:
		prefix = "idle"
	}
	if prefix == "" {
		return activeETA
	}
	return fmt.Sprintf("%s: %s", prefix, activeETA)
}

func isMLEstimateReady(ml batteryETAEstimates) bool {
	active := strings.TrimSpace(ml.ActiveValue)
	if active == "" || active == "n/a" {
		return false
	}
	powerValue := strings.ToLower(strings.TrimSpace(ml.PowerValue))
	return strings.Contains(powerValue, "ewma+trend") || strings.Contains(powerValue, "profile:")
}

func pickPreferredMLForTopState(
	unit batteryETAEstimates,
	generic batteryETAEstimates,
	new batteryETAEstimates,
	state systemStateKind,
) (batteryETAEstimates, string) {
	genericReady := isMLEstimateReady(generic)
	newReady := isMLEstimateReady(new)
	genericTier := confidenceTierFromValue(generic.ConfidenceValue)
	newTier := confidenceTierFromValue(new.ConfidenceValue)
	genericConf, _ := parseConfidenceScore(generic.ConfidenceValue)
	newConf, _ := parseConfidenceScore(new.ConfidenceValue)

	const modelSelectConfidenceFloor = 0.70
	switch {
	case newReady && newTier == "high":
		return new, "New"
	case genericReady && genericTier == "high":
		return generic, "Generic"
	case newReady && newConf >= modelSelectConfidenceFloor:
		return new, "New"
	case genericReady && genericConf >= modelSelectConfidenceFloor:
		return generic, "Generic"
	}

	newCloseness, newHasCloseness := estimateClosenessToUnit(unit, new, state)
	genericCloseness, genericHasCloseness := estimateClosenessToUnit(unit, generic, state)
	switch {
	case newHasCloseness && !genericHasCloseness:
		return new, "New"
	case genericHasCloseness && !newHasCloseness:
		return generic, "Generic"
	case newHasCloseness && genericHasCloseness && newCloseness+1e-6 < genericCloseness:
		return new, "New"
	case genericHasCloseness && newHasCloseness && genericCloseness+1e-6 < newCloseness:
		return generic, "Generic"
	case newReady:
		return new, "New"
	case genericReady:
		return generic, "Generic"
	}

	return unit, "MPPT"
}

func estimateClosenessToUnit(
	unit batteryETAEstimates,
	candidate batteryETAEstimates,
	state systemStateKind,
) (float64, bool) {
	score := 0.0
	hasSignal := false

	if unitMinutes, ok := estimateActiveMinutes(unit, state); ok {
		if candMinutes, ok := estimateActiveMinutes(candidate, state); ok {
			score += math.Abs(candMinutes - unitMinutes)
		} else {
			score += 1e6
		}
		hasSignal = true
	}
	if unitPower, ok := estimateSignedPower(unit, state); ok {
		if candPower, ok := estimateSignedPower(candidate, state); ok {
			// Keep power contribution smaller than ETA contribution.
			score += math.Abs(candPower-unitPower) * 0.2
		} else {
			score += 1e6
		}
		hasSignal = true
	}
	return score, hasSignal
}

func confidenceTierFromValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "n/a" {
		return "low"
	}
	switch {
	case strings.Contains(value, "(high)"):
		return "high"
	case strings.Contains(value, "(medium)"):
		return "medium"
	case strings.Contains(value, "(low)"):
		return "low"
	default:
		return "low"
	}
}

func parseConfidenceScore(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "n/a" {
		return 0, false
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return 0, false
	}
	score, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || math.IsNaN(score) || math.IsInf(score, 0) {
		return 0, false
	}
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score, true
}

func formatTypeWithState(typeCode string, systemState string) string {
	typeCode = strings.TrimSpace(typeCode)
	systemState = strings.TrimSpace(systemState)
	if typeCode == "" {
		typeCode = "n/a"
	}
	if systemState == "" || systemState == string(systemStateUnknown) {
		return typeCode
	}
	return typeCode + "/" + systemState
}

func formatPVInputRowLabel(channel string, device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel != "high" {
		channel = "low"
	}
	maxWatts, ok := estimatePVInputMaxWatts(channel, device, snapshot)
	if !ok {
		if channel == "high" {
			return "pv high"
		}
		return "pv low"
	}
	capLabel := formatPVCapacityWatts(maxWatts)
	otherChannel := "high"
	if channel == "high" {
		otherChannel = "low"
	}
	if otherWatts, otherOK := estimatePVInputMaxWatts(otherChannel, device, snapshot); otherOK && math.Abs(otherWatts-maxWatts) <= 0.5 {
		if channel == "high" {
			return fmt.Sprintf("solar #2 [%s]", capLabel)
		}
		return fmt.Sprintf("solar #1 [%s]", capLabel)
	}
	return fmt.Sprintf("solar [%s]", capLabel)
}

type pvInputCapability struct {
	minVolts float64
	maxVolts float64
	maxAmps  float64
	maxWatts float64
}

func formatPVInputCapability(channel string, device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) string {
	capability, ok := estimatePVInputCapability(channel, device, snapshot)
	if !ok {
		if watts, wattsOK := estimatePVInputMaxWatts(channel, device, snapshot); wattsOK {
			return fmt.Sprintf("max: n/aV n/aA %s", formatPVCapacityWatts(watts))
		}
		return "max: n/a"
	}
	return fmt.Sprintf(
		"max: %s %s %s",
		formatVoltageRange(capability.minVolts, capability.maxVolts),
		formatAmpsLimit(capability.maxAmps),
		formatPVCapacityWatts(capability.maxWatts),
	)
}

func formatPVUtilizationGauge(
	channel string,
	device ecoflow.GeneralInfoDevice,
	snapshot *energySnapshot,
	hasRaw bool,
	rawWatts float64,
	hasSmooth bool,
	smoothWatts float64,
) string {
	maxWatts, ok := estimatePVInputMaxWatts(channel, device, snapshot)
	if !ok || maxWatts <= 0 {
		return "n/a"
	}

	watts := 0.0
	hasWatts := false
	switch {
	case hasRaw:
		watts = math.Abs(rawWatts)
		hasWatts = true
	case hasSmooth:
		watts = math.Abs(smoothWatts)
		hasWatts = true
	}
	if !hasWatts {
		return "n/a"
	}

	percent := (watts / maxWatts) * 100.0
	if percent < 0 {
		percent = 0
	}
	return fmt.Sprintf("%7.1f%% %s", percent, formatUtilizationGauge(percent, 10))
}

func formatUtilizationGauge(percent float64, width int) string {
	if width <= 0 {
		width = 10
	}
	filled := int(math.Round((math.Max(0, percent) / 100.0) * float64(width)))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

func estimatePVInputCapability(channel string, device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) (pvInputCapability, bool) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel != "high" {
		channel = "low"
	}
	deviceName := strings.ToLower(strings.TrimSpace(device.DeviceName + " " + device.ProductName))
	isD2M := strings.Contains(deviceName, "delta 2 max")
	isDPU := strings.Contains(deviceName, "delta pro ultra") || strings.Contains(deviceName, "dpu")

	if isD2M {
		return pvInputCapability{minVolts: 11, maxVolts: 60, maxAmps: 15, maxWatts: 500}, true
	}
	if isDPU {
		if channel == "high" {
			return pvInputCapability{minVolts: 80, maxVolts: 450, maxAmps: 15, maxWatts: 4000}, true
		}
		return pvInputCapability{minVolts: 30, maxVolts: 150, maxAmps: 15, maxWatts: 1600}, true
	}

	if snapshot != nil && snapshot.HasPVLowType && snapshot.HasPVHighType {
		// Delta 2 Max style dual-LV MPPT.
		if snapshot.PVLowType == 2 && snapshot.PVHighType == 2 {
			return pvInputCapability{minVolts: 11, maxVolts: 60, maxAmps: 15, maxWatts: 500}, true
		}
		// Delta Pro Ultra style mixed PV ports.
		if snapshot.PVLowType == 2 && snapshot.PVHighType == 0 {
			if channel == "high" {
				return pvInputCapability{minVolts: 80, maxVolts: 450, maxAmps: 15, maxWatts: 4000}, true
			}
			return pvInputCapability{minVolts: 30, maxVolts: 150, maxAmps: 15, maxWatts: 1600}, true
		}
	}

	if snapshot != nil {
		var typeCode int64
		var hasType bool
		if channel == "high" {
			typeCode, hasType = snapshot.PVHighType, snapshot.HasPVHighType
		} else {
			typeCode, hasType = snapshot.PVLowType, snapshot.HasPVLowType
		}
		if hasType {
			if typeCode == 2 {
				// Generic LV MPPT profile.
				return pvInputCapability{minVolts: 11, maxVolts: 60, maxAmps: 15, maxWatts: 500}, true
			}
			if channel == "high" && typeCode == 0 {
				// Generic HV MPPT profile.
				return pvInputCapability{minVolts: 80, maxVolts: 450, maxAmps: 15, maxWatts: 4000}, true
			}
		}
	}
	return pvInputCapability{}, false
}

func estimatePVInputMaxWatts(channel string, device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) (float64, bool) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel != "high" {
		channel = "low"
	}
	deviceName := strings.ToLower(strings.TrimSpace(device.DeviceName + " " + device.ProductName))
	isD2M := strings.Contains(deviceName, "delta 2 max")
	isDPU := strings.Contains(deviceName, "delta pro ultra") || strings.Contains(deviceName, "dpu")

	if isD2M {
		return 500, true
	}
	if isDPU {
		if channel == "high" {
			return 4000, true
		}
		return 1600, true
	}

	if snapshot != nil && snapshot.HasPVLowType && snapshot.HasPVHighType {
		if snapshot.PVLowType == 2 && snapshot.PVHighType == 2 {
			return 500, true
		}
		if snapshot.PVLowType == 2 && snapshot.PVHighType == 0 {
			if channel == "high" {
				return 4000, true
			}
			return 1600, true
		}
	}
	if snapshot != nil {
		var typeCode int64
		var hasType bool
		if channel == "high" {
			typeCode, hasType = snapshot.PVHighType, snapshot.HasPVHighType
		} else {
			typeCode, hasType = snapshot.PVLowType, snapshot.HasPVLowType
		}
		if hasType {
			if typeCode == 2 {
				return 500, true
			}
			if channel == "high" && typeCode == 0 {
				return 4000, true
			}
		}
	}
	return 0, false
}

func formatPVCapacityWatts(watts float64) string {
	if watts >= 1000 {
		kw := watts / 1000.0
		if math.Abs(kw-math.Round(kw)) <= 0.05 {
			return fmt.Sprintf("%.0fkW", math.Round(kw))
		}
		return fmt.Sprintf("%.1fkW", kw)
	}
	return fmt.Sprintf("%.0fW", watts)
}

func formatVoltageRange(minVolts, maxVolts float64) string {
	return fmt.Sprintf("%s-%sV", trimFloatForRange(minVolts), trimFloatForRange(maxVolts))
}

func formatAmpsLimit(amps float64) string {
	return fmt.Sprintf("%sA", trimFloatForRange(amps))
}

func trimFloatForRange(value float64) string {
	if math.Abs(value-math.Round(value)) <= 0.05 {
		return fmt.Sprintf("%.0f", math.Round(value))
	}
	return fmt.Sprintf("%.1f", value)
}

func formatLastMQTTMeta(envelope telemetryEnvelope) string {
	parts := make([]string, 0, 4)
	if envelope.ID != 0 {
		parts = append(parts, fmt.Sprintf("id: %d", envelope.ID))
	}
	if envelope.CmdID != 0 || envelope.CmdFunc != 0 {
		parts = append(parts, fmt.Sprintf("cmd: %d/%d", envelope.CmdID, envelope.CmdFunc))
	}
	if addr := strings.TrimSpace(envelope.Addr); addr != "" {
		parts = append(parts, "addr: "+addr)
	}
	if envelope.Time != 0 {
		parts = append(parts, fmt.Sprintf("t: %d", envelope.Time))
	}
	if len(parts) == 0 {
		return "meta: n/a"
	}
	return strings.Join(parts, " ")
}

func formatMQTTStatus(snapshot *energySnapshot) string {
	if snapshot == nil {
		return "n/a"
	}
	if snapshot.MQTTDegraded {
		base := snapshot.MQTTDegradedReason
		if strings.TrimSpace(base) == "" {
			base = "MQTT degraded"
		}
		if snapshot.MQTTFallbackActive {
			return base + " + REST fallback"
		}
		return base
	}
	if snapshot.MQTTConnected {
		return "MQTT live"
	}
	if snapshot.MQTTFallbackActive {
		return "MQTT reconnecting + REST fallback"
	}
	return "MQTT reconnecting"
}

func formatMQTTUptime(snapshot *energySnapshot) string {
	if snapshot == nil || !snapshot.MQTTConnected || !snapshot.HasMQTTConnectedSince {
		return "n/a"
	}
	uptime := time.Since(snapshot.MQTTConnectedSince)
	if uptime < 0 {
		return "n/a"
	}
	seconds := int64(uptime.Round(time.Second) / time.Second)
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	remSeconds := seconds % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm%02ds", minutes, remSeconds)
	}
	hours := minutes / 60
	remMinutes := minutes % 60
	if hours < 24 {
		return fmt.Sprintf("%dh%02dm", hours, remMinutes)
	}
	days := hours / 24
	remHours := hours % 24
	return fmt.Sprintf("%dd%02dh", days, remHours)
}

func formatSOCWithGauge(soc float64, gaugeWidth int) string {
	return fmt.Sprintf("%s %s", formatSOCPercentFixed(soc), formatSOCGauge(soc, gaugeWidth))
}

func formatSOCUnavailableWithGauge(gaugeWidth int) string {
	return fmt.Sprintf("%-7s %s", "n/a", formatSOCGauge(0, gaugeWidth))
}

func formatSOCPercentFixed(soc float64) string {
	if soc < 0 {
		soc = 0
	}
	if soc > 100 {
		soc = 100
	}
	// Fixed-width percent field so SOC table column width stays stable.
	return fmt.Sprintf("%6.2f%%", soc)
}

func formatSOCGauge(soc float64, gaugeWidth int) string {
	if gaugeWidth <= 0 {
		gaugeWidth = 8
	}
	if soc < 0 {
		soc = 0
	}
	if soc > 100 {
		soc = 100
	}
	filled := int(math.Round((soc / 100.0) * float64(gaugeWidth)))
	if filled < 0 {
		filled = 0
	}
	if filled > gaugeWidth {
		filled = gaugeWidth
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", gaugeWidth-filled) + "]"
}

func formatMQTTLastMessageAge(snapshot *energySnapshot) string {
	if snapshot == nil || !snapshot.HasMQTTLastMessage || snapshot.MQTTLastMessageAt.IsZero() {
		return "last_msg: n/a"
	}
	age := time.Since(snapshot.MQTTLastMessageAt)
	if age < 0 {
		age = 0
	}
	if age < time.Second {
		return "last_msg: now"
	}
	return fmt.Sprintf("last_msg: %s ago", formatShortDuration(age))
}

func formatSnapshotUpdatedRelative(snapshot *energySnapshot) string {
	if snapshot == nil {
		return "n/a"
	}
	if snapshot.HasDataUpdatedAt && !snapshot.DataUpdatedAt.IsZero() {
		return formatRelativeTimeAgo(snapshot.DataUpdatedAt)
	}
	if snapshot.HasMQTTLastMessage && !snapshot.MQTTLastMessageAt.IsZero() {
		return formatRelativeTimeAgo(snapshot.MQTTLastMessageAt)
	}
	return "n/a"
}

func formatRelativeTimeAgo(ts time.Time) string {
	if ts.IsZero() {
		return "n/a"
	}
	age := time.Since(ts)
	if age < 0 {
		age = 0
	}
	if age < time.Second {
		return "< 1 second ago"
	}
	seconds := int(age / time.Second)
	if seconds < 60 {
		if seconds == 1 {
			return "1 second ago"
		}
		return fmt.Sprintf("%d seconds ago", seconds)
	}
	minutes := int(age / time.Minute)
	if minutes < 60 {
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	hours := int(age / time.Hour)
	if hours < 24 {
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := hours / 24
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}

func formatShortDuration(value time.Duration) string {
	if value < 0 {
		value = -value
	}
	if value < time.Second {
		return "0s"
	}
	if value < time.Minute {
		return fmt.Sprintf("%ds", int(value/time.Second))
	}
	if value < time.Hour {
		minutes := int(value / time.Minute)
		seconds := int((value % time.Minute) / time.Second)
		if seconds == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	hours := int(value / time.Hour)
	minutes := int((value % time.Hour) / time.Minute)
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, minutes)
}

func packStateLabel(pack *packSnapshot) string {
	if !isPackPowerFresh(pack) {
		return "unknown"
	}
	if pack.PowerW > 0 {
		return "charging"
	}
	if pack.PowerW < 0 {
		return "discharging"
	}
	return "idle"
}

func formatPackPreconditioning(pack *packSnapshot) string {
	stateKnown := pack.HasPreconditioning
	on := pack.PreconditioningOn
	if !stateKnown && pack.HasPreconditioningState {
		stateKnown = true
		on = pack.PreconditioningStateRaw > 0
	}
	if !stateKnown && pack.HasPreconditioningHeat && pack.PreconditioningHeatTime > 0 {
		stateKnown = true
		on = true
	}
	if !stateKnown {
		return "n/a"
	}
	return checkboxStatus(on)
}

func overallPreconditioningStatus(packs map[int]*packSnapshot) (known bool, on bool) {
	if len(packs) == 0 {
		return false, false
	}
	for _, pack := range packs {
		if pack == nil {
			continue
		}
		packKnown := pack.HasPreconditioning || pack.HasPreconditioningState || pack.HasPreconditioningHeat
		if !packKnown {
			continue
		}
		known = true
		packOn := pack.PreconditioningOn
		if !pack.HasPreconditioning && pack.HasPreconditioningState {
			packOn = pack.PreconditioningStateRaw > 0
		}
		if !pack.HasPreconditioning && !pack.HasPreconditioningState && pack.HasPreconditioningHeat {
			packOn = pack.PreconditioningHeatTime > 0
		}
		if packOn {
			return true, true
		}
	}
	return known, false
}

func formatPackSerial(pack *packSnapshot) string {
	if pack == nil {
		return "n/a"
	}
	serial := strings.TrimSpace(pack.Serial)
	if serial == "" {
		return "n/a"
	}
	return serial
}

func formatPackEnergy(pack *packSnapshot) string {
	if pack == nil || !pack.HasEnergy {
		return "n/a"
	}
	return formatEnergyWh(pack.EnergyWh)
}

func formatPackSoh(pack *packSnapshot) string {
	if pack == nil {
		return "n/a"
	}
	if pack.HasActSOH && pack.HasSOH {
		return fmt.Sprintf("%.2f/%.1f%%", pack.ActSOH, pack.SOH)
	}
	if pack.HasActSOH {
		return fmt.Sprintf("%.2f%%", pack.ActSOH)
	}
	if pack.HasSOH {
		return fmt.Sprintf("%.1f%%", pack.SOH)
	}
	return "n/a"
}

func formatPackVoltage(pack *packSnapshot) string {
	if pack == nil || !pack.HasVoltage {
		return "n/a"
	}
	return fmt.Sprintf("%.3fV", pack.VoltageV)
}

func formatPackTarget(pack *packSnapshot) string {
	if pack == nil {
		return "n/a"
	}
	if !pack.HasTargetSOC {
		if pack.HasSOC {
			return fmt.Sprintf("%.2f%%", pack.SOC)
		}
		return "n/a"
	}
	return fmt.Sprintf("%.2f%%", pack.TargetSOC)
}

func formatPackLimits(pack *packSnapshot) string {
	if pack == nil {
		return "n/a"
	}
	if pack.HasMinSOC && pack.HasMaxSOC {
		return fmt.Sprintf("%.0f-%.0f%%", pack.MinSOC, pack.MaxSOC)
	}
	if pack.HasMinSOC {
		return fmt.Sprintf("min %.0f%%", pack.MinSOC)
	}
	if pack.HasMaxSOC {
		return fmt.Sprintf("max %.0f%%", pack.MaxSOC)
	}
	return "n/a"
}

func formatPackDiffSOC(pack *packSnapshot) string {
	if pack == nil || !pack.HasDiffSOC {
		return "n/a"
	}
	return fmt.Sprintf("%.2f%%", pack.DiffSOC)
}

func formatPackCapacity(pack *packSnapshot) string {
	if pack == nil {
		return "n/a"
	}
	switch {
	case pack.HasRemainCap && pack.HasFullCap:
		return fmt.Sprintf("%.0f/%.0f", pack.RemainCap, pack.FullCap)
	case pack.HasRemainCap:
		return fmt.Sprintf("%.0f", pack.RemainCap)
	case pack.HasFullCap:
		return fmt.Sprintf("n/a/%.0f", pack.FullCap)
	default:
		return "n/a"
	}
}

func formatPackBoardTemp(pack *packSnapshot) string {
	if pack == nil || !pack.HasBoardTemp {
		return "n/a"
	}
	return fmt.Sprintf("%.1fC", pack.BoardTempC)
}
