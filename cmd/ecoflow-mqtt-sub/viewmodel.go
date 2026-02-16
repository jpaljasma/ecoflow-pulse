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

	statusLines []string
}

func buildDashboardViewModel(
	device ecoflow.GeneralInfoDevice,
	topic string,
	envelope telemetryEnvelope,
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
	lastMessageValue := formatMQTTLastMessageAge(snapshot)
	pvLowLabel := formatPVInputRowLabel("low", device, snapshot)
	pvHighLabel := formatPVInputRowLabel("high", device, snapshot)

	updatedAt := formatSnapshotUpdatedRelative(snapshot)
	deviceHeaders := []string{"Icon", "Device Name", "SOC", "AC In", "Solar Generated", "Out", "Net", "State", "Updated"}
	summaryHeaders := []string{
		"Details",
		"In",
		"Out",
		"Net",
		"Remain",
	}
	lastTypeState := formatTypeWithState(firstNonEmpty(envelope.TypeCode, "n/a"), derived.SystemStateValue)
	lastMQTTMeta := formatLastMQTTMeta(envelope)
	mlEstimates := estimateBatteryETAsML(snapshot, minuteHistory, systemStateKind(derived.SystemStateValue))
	primaryEstimateCharge := firstNonEmpty(strings.TrimSpace(mlEstimates.ChargeValue), "n/a")
	primaryEstimateDischarge := firstNonEmpty(strings.TrimSpace(mlEstimates.DischargeValue), "n/a")
	primaryEstimateActive := firstNonEmpty(strings.TrimSpace(mlEstimates.ActiveValue), "n/a")
	primaryEstimatePower := firstNonEmpty(strings.TrimSpace(mlEstimates.PowerValue), "power: n/a")
	primaryEstimateConfidence := firstNonEmpty(strings.TrimSpace(mlEstimates.ConfidenceValue), "n/a")
	topStateValue := selectTopStateValue(
		snapshot,
		derived.RemainValue,
		systemStateKind(derived.SystemStateValue),
		mlEstimates,
	)
	topStateValue = sanitizeStateColumnValue(topStateValue)
	topStateIcon := topStateDisplayIcon(systemStateKind(derived.SystemStateValue), topStateValue)
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
		updatedAt,
	}}
	summaryRows := [][]string{
		{
			"channels",
			fmt.Sprintf("ac: %s pv_total: %s xt150_in: %s", derived.InACValue, pvTotalDisplay, derived.XT150InValue),
			fmt.Sprintf("ac: %s (l14: %s) dc: %s xt150_out: %s", derived.OutACValue, derived.OutACL14Value, derived.OutDCValue, derived.XT150OutValue),
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
			fmt.Sprintf("in: %s", pvLowDisplay),
			fmt.Sprintf("volts: %s amps: %s", derived.PVLowVoltsValue, derived.PVLowAmpsValue),
			fmt.Sprintf("state: %s", derived.PVLowStateValue),
			"-",
		},
		{
			pvHighLabel,
			fmt.Sprintf("in: %s", pvHighDisplay),
			fmt.Sprintf("volts: %s amps: %s", derived.PVHighVoltsValue, derived.PVHighAmpsValue),
			fmt.Sprintf("state: %s", derived.PVHighStateValue),
			"-",
		},
		{
			"battery",
			fmt.Sprintf("in: %s", derived.BatteryInValue),
			fmt.Sprintf("out: %s idle: %s", derived.BatteryOutValue, derived.IdleDrawValue),
			derived.BatteryNetValue,
			"-",
		},
		{
			"est ML",
			fmt.Sprintf("charge: %s", primaryEstimateCharge),
			fmt.Sprintf("discharge: %s", primaryEstimateDischarge),
			primaryEstimatePower,
			fmt.Sprintf(
				"active: %s conf: %s",
				primaryEstimateActive,
				primaryEstimateConfidence,
			),
		},
		{
			"mqtt",
			fmt.Sprintf("queue: %s", mqttQueueValue),
			mqttDropsValue,
			fmt.Sprintf("status: %s", mqttStatusValue),
			fmt.Sprintf("last: %s %s %s uptime: %s", lastTypeState, lastMQTTMeta, lastMessageValue, formatMQTTUptime(snapshot)),
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
			return mlValue
		}
	} else if mlReady && confidenceTierFromValue(ml.ConfidenceValue) == "high" {
		return mlValue
	}
	if deviceReported != "n/a" {
		return deviceReported
	}
	if mlReady {
		return mlValue
	}
	return "n/a"
}

func topStateDisplayIcon(state systemStateKind, value string) string {
	displayState := state
	if displayState == systemStateUnknown {
		lower := strings.ToLower(strings.TrimSpace(value))
		switch {
		case strings.HasPrefix(lower, "charging:"):
			displayState = systemStateCharging
		case strings.HasPrefix(lower, "discharging:"):
			displayState = systemStateDischarging
		case strings.HasPrefix(lower, "idle:"):
			displayState = systemStateIdle
		}
	}
	switch displayState {
	case systemStateCharging:
		return "⚡"
	case systemStateDischarging:
		return "↓"
	case systemStateIdle:
		return "⏸"
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
	return strings.Contains(strings.ToLower(strings.TrimSpace(ml.PowerValue)), "ewma+trend")
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
