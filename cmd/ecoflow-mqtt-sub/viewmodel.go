package main

import (
	"fmt"
	"math"
	"regexp"
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

	solarRecHeaders  []string
	solarRecRows     [][]string
	solarCandHeaders []string
	solarCandRows    [][]string

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

	updatedAt := formatSnapshotUpdatedRelative(snapshot)
	deviceHeaders := []string{"Icon", "Device Name", "SOC", "AC In", "Solar Generated", "Out", "Net", "State", "Model", "Updated"}
	summaryHeaders := []string{
		"Details",
		"In",
		"Out",
		"Net",
		"Panel Model",
		"Details",
	}
	stateKind := systemStateKind(derived.SystemStateValue)
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
			"meta",
			fmt.Sprintf("packs: %s showFlag: %s", derived.BatteryCount, derived.ShowFlagValue),
			fmt.Sprintf("combo: %s c20: %s para: %s", derived.ComboValue, derived.C20LimitValue, derived.ParaLimitValue),
			fmt.Sprintf("socWindow: %s backup: %s", derived.SocGuardrail, derived.BackupReserveValue),
			"-",
			"-",
		},
		{
			pvLowLabel,
			pvLowCapability,
			fmt.Sprintf("volts: %s amps: %s watts: %s", derived.PVLowVoltsValue, derived.PVLowAmpsValue, pvLowDisplay),
			formatSolarNetSummary(derived.PVLowStateValue, pvLowDisplay),
			formatPanelPrediction(snapshot.HasPVLowPanelPrediction, snapshot.PVLowPanelSetup, snapshot.PVLowPanelConfidence, snapshot.PVLowPanelSamples, snapshot.PVLowPanelStatus),
			pvLowUtilization,
		},
		{
			pvHighLabel,
			pvHighCapability,
			fmt.Sprintf("volts: %s amps: %s watts: %s", derived.PVHighVoltsValue, derived.PVHighAmpsValue, pvHighDisplay),
			formatSolarNetSummary(derived.PVHighStateValue, pvHighDisplay),
			formatPanelPrediction(snapshot.HasPVHighPanelPrediction, snapshot.PVHighPanelSetup, snapshot.PVHighPanelConfidence, snapshot.PVHighPanelSamples, snapshot.PVHighPanelStatus),
			pvHighUtilization,
		},
		{
			"battery",
			fmt.Sprintf("in: %s", derived.BatteryInValue),
			fmt.Sprintf("out: %s idle: %s", derived.BatteryOutValue, derived.IdleDrawValue),
			fmt.Sprintf("%s source: %s", derived.BatteryNetValue, batterySource),
			"-",
			"-",
		},
		{
			"mqtt",
			fmt.Sprintf("queue: %s", mqttQueueValue),
			mqttDropsValue,
			fmt.Sprintf("status: %s", mqttStatusValue),
			"-",
			"-",
		},
	}

	solarRecHeaders, solarRecRows := buildSolarRecommendationRows(device, snapshot, hasPVLowRaw, pvLowRaw, hasPVLowSmooth, pvLowSmooth, hasPVHighRaw, pvHighRaw, hasPVHighSmooth, pvHighSmooth)
	if len(solarRecRows) == 0 {
		if len(solarRecHeaders) == 0 {
			solarRecHeaders = []string{"Metric", "PV"}
		}
		fallback := make([]string, 0, len(solarRecHeaders))
		for i := range solarRecHeaders {
			if i == 0 {
				fallback = append(fallback, "n/a")
				continue
			}
			fallback = append(fallback, "n/a")
		}
		solarRecRows = append(solarRecRows, fallback)
	}
	var solarCandHeaders []string
	var solarCandRows [][]string
	if minuteCfg.ShowSolarCandidates {
		solarCandHeaders, solarCandRows = buildSolarCandidateRows(device, snapshot)
	}

	estimateHeaders := []string{"Model", "Charge", "Discharge", "Active", "Power", "Confidence", "Δ ETA vs Unit", "Δ Power vs Unit"}
	estimateRows := [][]string{
		{
			"MPPT",
			firstNonEmpty(strings.TrimSpace(unitEstimates.ChargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(unitEstimates.DischargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(unitEstimates.ActiveValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(unitEstimates.PowerValue), "power: n/a"),
			firstNonEmpty(strings.TrimSpace(unitEstimates.ConfidenceValue), "n/a"),
			"-",
			"-",
		},
		{
			fmt.Sprintf("New (%s)", strings.ToUpper(string(newMLProfile))),
			firstNonEmpty(strings.TrimSpace(newMLEstimates.ChargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(newMLEstimates.DischargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(newMLEstimates.ActiveValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(newMLEstimates.PowerValue), "power: n/a"),
			firstNonEmpty(strings.TrimSpace(newMLEstimates.ConfidenceValue), "n/a"),
			estimateDeltaMinutesDisplay(unitEstimates, newMLEstimates, stateKind),
			estimateDeltaPowerDisplay(unitEstimates, newMLEstimates, stateKind),
		},
		{
			"Generic",
			firstNonEmpty(strings.TrimSpace(genericMLEstimates.ChargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(genericMLEstimates.DischargeValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(genericMLEstimates.ActiveValue), "n/a"),
			firstNonEmpty(strings.TrimSpace(genericMLEstimates.PowerValue), "power: n/a"),
			firstNonEmpty(strings.TrimSpace(genericMLEstimates.ConfidenceValue), "n/a"),
			estimateDeltaMinutesDisplay(unitEstimates, genericMLEstimates, stateKind),
			estimateDeltaPowerDisplay(unitEstimates, genericMLEstimates, stateKind),
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

	statusItems := make([]string, 0, 10)
	statusItems = append(statusItems, fmt.Sprintf("%s AC On", derived.StatusACValue))
	if showSeparateUSBAndDC {
		statusItems = append(statusItems, fmt.Sprintf("%s USB On", derived.StatusUSBValue))
		statusItems = append(statusItems, fmt.Sprintf("%s 12V DC On", derived.StatusDC12VValue))
	} else {
		statusItems = append(statusItems, fmt.Sprintf("%s DC/USB On", derived.StatusDCValue))
	}
	if showFanStatus {
		statusItems = append(statusItems, fmt.Sprintf("%s Fan On", derived.StatusFanValue))
	}
	if showEVStatus {
		statusItems = append(statusItems, fmt.Sprintf("%s EV Charging On", derived.StatusEVValue))
	}
	statusItems = append(statusItems, fmt.Sprintf("%s UPS Passthrough", derived.StatusPassthroughValue))
	statusItems = append(statusItems, fmt.Sprintf("%s Solar Passthrough", derived.StatusSolarPassValue))
	statusItems = append(statusItems, fmt.Sprintf("%s Solar Charging", derived.StatusSolarChargingValue))
	statusItems = append(statusItems, fmt.Sprintf("%s Grounded (Estimated)", derived.StatusGroundedValue))
	if showPreconditioningStatus {
		statusItems = append(statusItems, fmt.Sprintf("%s Battery Preconditioning On", derived.StatusPrecondValue))
	}

	if len(statusItems) > 0 {
		statusLines = append(statusLines, strings.Join(statusItems, "  "))
	}

	return dashboardViewModel{
		topic: topic,

		deviceHeaders: deviceHeaders,
		deviceRows:    deviceRows,

		summaryHeaders: summaryHeaders,
		summaryRows:    summaryRows,

		solarRecHeaders:  solarRecHeaders,
		solarRecRows:     solarRecRows,
		solarCandHeaders: solarCandHeaders,
		solarCandRows:    solarCandRows,

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

func formatPanelPrediction(has bool, setup string, confidence float64, samples int, status string) string {
	status = strings.TrimSpace(status)
	if !has || strings.TrimSpace(setup) == "" {
		if status != "" {
			return "panel: " + status
		}
		return "panel: n/a"
	}
	if status != "" {
		return fmt.Sprintf("panel: %s (%s)", setup, status)
	}
	if samples > 0 {
		return fmt.Sprintf("panel: %s (%.2f, n=%d)", setup, confidence, samples)
	}
	return fmt.Sprintf("panel: %s (%.2f)", setup, confidence)
}

type detectedPanelSetup struct {
	has         bool
	setup       string
	status      string
	bifacial    bool
	confidence  float64
	samples     int
	panelCount  int
	hasCount    bool
	nominalW    float64
	hasNominalW bool
}

type upgradePanelTarget struct {
	label        string
	hasLabel     bool
	purchaseLink string
	hasLink      bool
	status       string
	sourceKind   string
	panelWatts   float64
	hasPanelW    bool
	panelVocV    float64
	hasPanelVoc  bool
	panelVmpV    float64
	hasPanelVmp  bool
	panelImpA    float64
	hasPanelImp  bool
	panelIscA    float64
	hasPanelIsc  bool
	panelEffPct  float64
	hasPanelEff  bool
	panelEffSrc  string
	minSeries    int
	hasMinSeries bool
	maxSeries    int
	hasMaxSeries bool
	bifacial     bool
}

type solarRecommendationOption struct {
	text         string
	nominalW     float64
	potentialW   float64
	hasPotential bool
	clipped      bool
	bifacial     bool
	effPct       float64
	hasEffPct    bool
	effSrc       string
	sourceLabel  string
	sourceLink   string
	sourceKind   string
	series       int
	parallel     int
	units        int
	complexity   float64
}

type solarRecommendationPort struct {
	channel      string
	label        string
	maxWatts     float64
	hasMaxWatts  bool
	capability   pvInputCapability
	hasCap       bool
	detected     detectedPanelSetup
	upgrade      upgradePanelTarget
	upgradeAlt   upgradePanelTarget
	dbCandidates []upgradePanelTarget
}

type solarRecommendationPortPlanCache struct {
	channel         string
	label           string
	addText         string
	upgradeText     string
	upgrade2Text    string
	addOption       solarRecommendationOption
	upgradeOption   solarRecommendationOption
	upgradeOption2  solarRecommendationOption
	portMaxWatts    float64
	hasPortMaxWatts bool
}

type solarPortRecommendationData struct {
	channel               string
	label                 string
	detected              string
	add                   string
	upgrade               string
	upgrade2              string
	eta                   string
	eta2                  string
	basePotentialETAW     float64
	addPotentialETAW      float64
	hasAddPotential       bool
	upgradePotentialETAW  float64
	hasUpgradePotential   bool
	upgrade2PotentialETAW float64
	hasUpgrade2Potential  bool
	addOption             solarRecommendationOption
	upgradeOption         solarRecommendationOption
	upgradeOption2        solarRecommendationOption
	portMaxWatts          float64
	hasPortMaxWatts       bool
}

var (
	panelSetupCountPattern = regexp.MustCompile(`(?i)^\s*(\d+)\s*x\s*([0-9]+(?:\.[0-9]+)?)\s*w\b`)
	panelSetupWattPattern  = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*w\b`)
)

const bifacialETAConservativeGain = 0.15
const panelShoulderHoursGain = 0.24
const panelComplexityPenaltyFactor = 0.012
const ecoflow125ComplexityFactor = 0.5
const panelEfficiencyBoostFactor = 0.025
const panelEstimatedEfficiencyWeight = 0.6

func buildSolarRecommendationRows(
	device ecoflow.GeneralInfoDevice,
	snapshot *energySnapshot,
	hasPVLowRaw bool,
	pvLowRaw float64,
	_ bool,
	_ float64,
	hasPVHighRaw bool,
	pvHighRaw float64,
	_ bool,
	_ float64,
) ([]string, [][]string) {
	if snapshot == nil {
		return nil, nil
	}

	lowPort := solarRecommendationPort{
		channel:      "low",
		label:        formatPVInputRowLabel("low", device, snapshot),
		detected:     detectedPanelForChannel(snapshot, "low"),
		upgrade:      upgradePanelForChannel(snapshot, "low", false),
		upgradeAlt:   upgradePanelForChannel(snapshot, "low", true),
		dbCandidates: upgradePanelCandidatesForChannel(snapshot, "low"),
	}
	highPort := solarRecommendationPort{
		channel:      "high",
		label:        formatPVInputRowLabel("high", device, snapshot),
		detected:     detectedPanelForChannel(snapshot, "high"),
		upgrade:      upgradePanelForChannel(snapshot, "high", false),
		upgradeAlt:   upgradePanelForChannel(snapshot, "high", true),
		dbCandidates: upgradePanelCandidatesForChannel(snapshot, "high"),
	}
	if maxW, ok := estimatePVInputMaxWatts("low", device, snapshot); ok {
		lowPort.maxWatts = maxW
		lowPort.hasMaxWatts = true
	}
	if maxW, ok := estimatePVInputMaxWatts("high", device, snapshot); ok {
		highPort.maxWatts = maxW
		highPort.hasMaxWatts = true
	}
	if capLow, ok := estimatePVInputCapability("low", device, snapshot); ok {
		lowPort.capability = capLow
		lowPort.hasCap = true
	}
	if capHigh, ok := estimatePVInputCapability("high", device, snapshot); ok {
		highPort.capability = capHigh
		highPort.hasCap = true
	}

	baseLow, baseLowNominal, hasBaseLow := baselineSolarPortPotential(lowPort, hasPVLowRaw, pvLowRaw)
	baseHigh, baseHighNominal, hasBaseHigh := baselineSolarPortPotential(highPort, hasPVHighRaw, pvHighRaw)
	baseLowETAW := solarRecommendationETAW(baseLow, baseLowNominal, lowPort.maxWatts, lowPort.hasMaxWatts, portDetectedBifacial(lowPort.detected))
	baseHighETAW := solarRecommendationETAW(baseHigh, baseHighNominal, highPort.maxWatts, highPort.hasMaxWatts, portDetectedBifacial(highPort.detected))
	baseTotalETAW := 0.0
	if hasBaseLow {
		baseTotalETAW += baseLowETAW
	}
	if hasBaseHigh {
		baseTotalETAW += baseHighETAW
	}

	remainingToChargeWh, hasRemainingCharge := estimatedEnergyToChargeTargetWh(snapshot)

	ports := []solarRecommendationPort{lowPort, highPort}
	portByChannel := map[string]solarRecommendationPort{
		"low":  lowPort,
		"high": highPort,
	}
	plans := loadOrBuildSolarRecommendationPlans(snapshot, ports, hasPVLowRaw, hasPVHighRaw)
	portRows := make([]solarPortRecommendationData, 0, 2)
	for _, plan := range plans {
		port, ok := portByChannel[plan.channel]
		if !ok {
			continue
		}
		basePortPotential, _, _ := baselineSolarPortPotential(port, false, 0)
		basePortETAW := basePortPotential
		if port.channel == "low" {
			basePortETAW = baseLowETAW
		} else if port.channel == "high" {
			basePortETAW = baseHighETAW
		}
		otherPortsETAW := baseTotalETAW - basePortETAW
		addTotalETAW := otherPortsETAW + solarRecommendationOptionETAW(plan.addOption, port.maxWatts, port.hasMaxWatts, portDetectedBifacial(port.detected))
		upgradeTotalETAW := otherPortsETAW + solarRecommendationOptionETAW(plan.upgradeOption, port.maxWatts, port.hasMaxWatts, plan.upgradeOption.bifacial)
		upgradeTotalETAW2 := otherPortsETAW + solarRecommendationOptionETAW(plan.upgradeOption2, port.maxWatts, port.hasMaxWatts, plan.upgradeOption2.bifacial)
		etaImpact := buildSolarETAImpact(
			remainingToChargeWh,
			hasRemainingCharge,
			baseTotalETAW,
			addTotalETAW,
			plan.addOption.hasPotential,
			upgradeTotalETAW,
			plan.upgradeOption.hasPotential,
		)
		etaImpact2 := buildSolarETAImpact(
			remainingToChargeWh,
			hasRemainingCharge,
			baseTotalETAW,
			addTotalETAW,
			plan.addOption.hasPotential,
			upgradeTotalETAW2,
			plan.upgradeOption2.hasPotential,
		)
		portRows = append(portRows, solarPortRecommendationData{
			channel:               port.channel,
			label:                 port.label,
			detected:              formatDetectedPanelRecommendation(port.detected, port.maxWatts, port.hasMaxWatts),
			add:                   plan.addText,
			upgrade:               plan.upgradeText,
			upgrade2:              plan.upgrade2Text,
			eta:                   etaImpact,
			eta2:                  etaImpact2,
			basePotentialETAW:     basePortETAW,
			addPotentialETAW:      solarRecommendationOptionETAW(plan.addOption, port.maxWatts, port.hasMaxWatts, portDetectedBifacial(port.detected)),
			hasAddPotential:       plan.addOption.hasPotential,
			upgradePotentialETAW:  solarRecommendationOptionETAW(plan.upgradeOption, port.maxWatts, port.hasMaxWatts, plan.upgradeOption.bifacial),
			hasUpgradePotential:   plan.upgradeOption.hasPotential,
			upgrade2PotentialETAW: solarRecommendationOptionETAW(plan.upgradeOption2, port.maxWatts, port.hasMaxWatts, plan.upgradeOption2.bifacial),
			hasUpgrade2Potential:  plan.upgradeOption2.hasPotential,
			addOption:             plan.addOption,
			upgradeOption:         plan.upgradeOption,
			upgradeOption2:        plan.upgradeOption2,
			portMaxWatts:          port.maxWatts,
			hasPortMaxWatts:       port.hasMaxWatts,
		})
	}

	if len(portRows) == 0 {
		return nil, nil
	}

	headers := make([]string, 0, len(portRows)+1)
	headers = append(headers, "Metric")
	for _, port := range portRows {
		headers = append(headers, port.label)
	}
	includeAllPortsSummary := len(portRows) > 1
	allPortsChargeETAImpact := ""
	allPortsChargeETAImpact2 := ""
	if includeAllPortsSummary {
		allAddTotalW := 0.0
		allUpgradeTotalW := 0.0
		allUpgradeTotalW2 := 0.0
		hasAllAdd := false
		hasAllUpgrade := false
		hasAllUpgrade2 := false
		for _, port := range portRows {
			addW := port.basePotentialETAW
			if port.hasAddPotential {
				addW = port.addPotentialETAW
				hasAllAdd = true
			}
			allAddTotalW += addW

			upgradeW := port.basePotentialETAW
			if port.hasUpgradePotential {
				upgradeW = port.upgradePotentialETAW
				hasAllUpgrade = true
			}
			allUpgradeTotalW += upgradeW

			upgradeW2 := port.basePotentialETAW
			if port.hasUpgrade2Potential {
				upgradeW2 = port.upgrade2PotentialETAW
				hasAllUpgrade2 = true
			}
			allUpgradeTotalW2 += upgradeW2
		}
		allPortsChargeETAImpact = buildSolarETAImpact(
			remainingToChargeWh,
			hasRemainingCharge,
			baseTotalETAW,
			allAddTotalW,
			hasAllAdd,
			allUpgradeTotalW,
			hasAllUpgrade,
		)
		allPortsChargeETAImpact2 = buildSolarETAImpact(
			remainingToChargeWh,
			hasRemainingCharge,
			baseTotalETAW,
			allAddTotalW,
			hasAllAdd,
			allUpgradeTotalW2,
			hasAllUpgrade2,
		)
	}

	rows := make([][]string, 0, 7)
	appendMetricRow := func(metric string, valueSelector func(port solarPortRecommendationData) string) {
		row := make([]string, 0, len(portRows)+1)
		row = append(row, metric)
		for _, port := range portRows {
			row = append(row, valueSelector(port))
		}
		rows = append(rows, row)
	}

	appendMetricRow("Detected", func(port solarPortRecommendationData) string { return port.detected })
	appendMetricRow("Add Panels", func(port solarPortRecommendationData) string { return port.add })
	appendMetricRow("Upgrade Panels", func(port solarPortRecommendationData) string { return port.upgrade })
	appendMetricRow("Upgrade Panels #2", func(port solarPortRecommendationData) string { return port.upgrade2 })
	appendMetricRow("Charge ETA Impact", func(port solarPortRecommendationData) string { return port.eta })
	appendMetricRow("Charge ETA Impact #2", func(port solarPortRecommendationData) string { return port.eta2 })
	if includeAllPortsSummary {
		summaryRow := []string{
			"All Ports ETA Impact",
			makeColspanCell(allPortsChargeETAImpact),
		}
		rows = append(rows, summaryRow)
		summaryRow2 := []string{
			"All Ports ETA Impact #2",
			makeColspanCell(allPortsChargeETAImpact2),
		}
		rows = append(rows, summaryRow2)
	}
	bestUpgradePath := buildBestUpgradePathSummary(portRows, remainingToChargeWh, hasRemainingCharge, baseTotalETAW)
	if strings.TrimSpace(bestUpgradePath) != "" {
		rows = append(rows, []string{
			"Best Upgrade Path",
			makeColspanCell(bestUpgradePath),
		})
	}
	return headers, rows
}

func loadOrBuildSolarRecommendationPlans(
	snapshot *energySnapshot,
	ports []solarRecommendationPort,
	hasPVLowRaw bool,
	hasPVHighRaw bool,
) []solarRecommendationPortPlanCache {
	cacheKey := solarRecommendationPlanCacheKey(ports)
	if snapshot != nil && snapshot.hasSolarRecPlanCache && snapshot.solarRecPlanCacheKey == cacheKey {
		return cloneSolarRecommendationPlanCache(snapshot.solarRecPlanCache)
	}

	portByChannel := make(map[string]solarRecommendationPort, len(ports))
	for _, port := range ports {
		portByChannel[port.channel] = port
	}

	plans := make([]solarRecommendationPortPlanCache, 0, len(ports))
	for _, port := range ports {
		includePort := port.hasMaxWatts || port.detected.has || port.upgrade.hasLabel || len(port.dbCandidates) > 0
		if port.channel == "low" && hasPVLowRaw {
			includePort = true
		}
		if port.channel == "high" && hasPVHighRaw {
			includePort = true
		}
		if !includePort {
			continue
		}
		peerPort := portByChannel["low"]
		if strings.EqualFold(port.channel, "low") {
			peerPort = portByChannel["high"]
		}
		addOption := buildAddPanelsOption(port, peerPort)
		upgradeOption, upgradeOption2 := selectUpgradeRecommendationPair(port, peerPort)
		plans = append(plans, solarRecommendationPortPlanCache{
			channel:         port.channel,
			label:           port.label,
			addText:         addOption.text,
			upgradeText:     upgradeOption.text,
			upgrade2Text:    upgradeOption2.text,
			addOption:       addOption,
			upgradeOption:   upgradeOption,
			upgradeOption2:  upgradeOption2,
			portMaxWatts:    port.maxWatts,
			hasPortMaxWatts: port.hasMaxWatts,
		})
	}

	if snapshot != nil {
		snapshot.solarRecPlanCacheKey = cacheKey
		snapshot.solarRecPlanCache = cloneSolarRecommendationPlanCache(plans)
		snapshot.hasSolarRecPlanCache = true
	}
	return plans
}

func solarRecommendationPlanCacheKey(ports []solarRecommendationPort) string {
	var builder strings.Builder
	for _, port := range ports {
		builder.WriteString(strings.ToLower(strings.TrimSpace(port.channel)))
		builder.WriteString(":")
		builder.WriteString(detectedPanelStableSignature(port.detected))
		builder.WriteString(";")
	}
	return builder.String()
}

func detectedPanelStableSignature(d detectedPanelSetup) string {
	if !d.has || strings.TrimSpace(d.setup) == "" {
		return "none"
	}
	builder := strings.Builder{}
	builder.WriteString(strings.ToLower(strings.TrimSpace(d.setup)))
	builder.WriteString("|")
	if d.hasCount {
		builder.WriteString(fmt.Sprintf("c=%d", d.panelCount))
	} else {
		builder.WriteString("c=n/a")
	}
	builder.WriteString("|")
	if d.hasNominalW {
		builder.WriteString(fmt.Sprintf("w=%.1f", d.nominalW))
	} else {
		builder.WriteString("w=n/a")
	}
	return builder.String()
}

func cloneSolarRecommendationPlanCache(src []solarRecommendationPortPlanCache) []solarRecommendationPortPlanCache {
	if len(src) == 0 {
		return nil
	}
	out := make([]solarRecommendationPortPlanCache, len(src))
	copy(out, src)
	return out
}

func buildSolarCandidateRows(device ecoflow.GeneralInfoDevice, snapshot *energySnapshot) ([]string, [][]string) {
	if snapshot == nil {
		return nil, nil
	}
	lowPort := solarRecommendationPort{
		channel:      "low",
		label:        formatPVInputRowLabel("low", device, snapshot),
		dbCandidates: upgradePanelCandidatesForChannel(snapshot, "low"),
	}
	highPort := solarRecommendationPort{
		channel:      "high",
		label:        formatPVInputRowLabel("high", device, snapshot),
		dbCandidates: upgradePanelCandidatesForChannel(snapshot, "high"),
	}
	if maxW, ok := estimatePVInputMaxWatts("low", device, snapshot); ok {
		lowPort.maxWatts = maxW
		lowPort.hasMaxWatts = true
	}
	if maxW, ok := estimatePVInputMaxWatts("high", device, snapshot); ok {
		highPort.maxWatts = maxW
		highPort.hasMaxWatts = true
	}
	if capLow, ok := estimatePVInputCapability("low", device, snapshot); ok {
		lowPort.capability = capLow
		lowPort.hasCap = true
	}
	if capHigh, ok := estimatePVInputCapability("high", device, snapshot); ok {
		highPort.capability = capHigh
		highPort.hasCap = true
	}

	headers := []string{"Port", "Panel", "Status", "STC", "Voc/Vmp", "Imp/Isc", "Series(cold)", "Best Layout", "Complexity", "Potential"}
	type candidateDisplayRow struct {
		cells      []string
		port       string
		complexity float64
		potential  float64
	}
	displayRows := make([]candidateDisplayRow, 0, len(lowPort.dbCandidates)+len(highPort.dbCandidates))
	appendPortCandidates := func(port solarRecommendationPort) {
		if !port.hasMaxWatts || !port.hasCap || len(port.dbCandidates) == 0 {
			return
		}
		for _, target := range port.dbCandidates {
			layout, ok := selectSafePanelLayout(port, target, false, panelLayout{})
			if !ok {
				continue
			}
			status := strings.TrimSpace(target.status)
			if status == "" {
				status = "compatible"
			}
			potential := formatPVCapacityWatts(layout.potentialW)
			if layout.clipped {
				potential = fmt.Sprintf("%s (from %s)", formatPVCapacityWatts(layout.potentialW), formatPVCapacityWatts(layout.nominalW))
			}
			complexity := adjustedPanelLayoutComplexity(panelLayoutComplexityScore(layout), target.label, layout.parallel)
			displayRows = append(displayRows, candidateDisplayRow{
				port:       port.label,
				complexity: complexity,
				potential:  layout.potentialW,
				cells: []string{
					port.label,
					target.label,
					status,
					formatPVCapacityWatts(target.panelWatts),
					fmt.Sprintf("%.1f/%.1fV", target.panelVocV, target.panelVmpV),
					fmt.Sprintf("%.2f/%.2fA", target.panelImpA, target.panelIscA),
					formatPanelSeriesRange(target),
					fmt.Sprintf("%s (%dx)", formatSeriesParallel(layout.series, layout.parallel), layout.units),
					fmt.Sprintf("%.1f", complexity),
					potential,
				},
			})
		}
	}
	appendPortCandidates(lowPort)
	appendPortCandidates(highPort)
	if len(displayRows) == 0 {
		return nil, nil
	}
	sort.SliceStable(displayRows, func(i, j int) bool {
		if displayRows[i].port != displayRows[j].port {
			return displayRows[i].port < displayRows[j].port
		}
		if math.Abs(displayRows[i].complexity-displayRows[j].complexity) > 0.05 {
			return displayRows[i].complexity < displayRows[j].complexity
		}
		if math.Abs(displayRows[i].potential-displayRows[j].potential) > 1 {
			return displayRows[i].potential > displayRows[j].potential
		}
		return displayRows[i].cells[1] < displayRows[j].cells[1]
	})
	rows := make([][]string, 0, len(displayRows))
	for _, row := range displayRows {
		rows = append(rows, row.cells)
	}
	return headers, rows
}

func formatPanelSeriesRange(target upgradePanelTarget) string {
	minSeries := 1
	maxSeries := 0
	if target.hasMinSeries && target.minSeries > 0 {
		minSeries = target.minSeries
	}
	if target.hasMaxSeries && target.maxSeries > 0 {
		maxSeries = target.maxSeries
	}
	switch {
	case maxSeries > 0 && minSeries > maxSeries:
		return fmt.Sprintf("%d-%d", maxSeries, minSeries)
	case maxSeries > 0:
		return fmt.Sprintf("%d-%d", minSeries, maxSeries)
	default:
		return fmt.Sprintf("%d+", minSeries)
	}
}

func detectedPanelForChannel(snapshot *energySnapshot, channel string) detectedPanelSetup {
	out := detectedPanelSetup{}
	if snapshot == nil {
		return out
	}
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "high":
		out.setup = strings.TrimSpace(snapshot.PVHighPanelSetup)
		out.status = strings.TrimSpace(snapshot.PVHighPanelStatus)
		out.bifacial = panelTextIsBifacial(out.setup)
		out.confidence = snapshot.PVHighPanelConfidence
		out.samples = snapshot.PVHighPanelSamples
		out.panelCount = snapshot.PVHighPanelCount
		out.hasCount = snapshot.HasPVHighPanelCount && snapshot.PVHighPanelCount > 0
		out.nominalW = snapshot.PVHighPanelNominalWatts
		out.hasNominalW = snapshot.HasPVHighPanelNominal && snapshot.PVHighPanelNominalWatts > 0
		out.has = snapshot.HasPVHighPanelPrediction && out.setup != ""
	default:
		out.setup = strings.TrimSpace(snapshot.PVLowPanelSetup)
		out.status = strings.TrimSpace(snapshot.PVLowPanelStatus)
		out.bifacial = panelTextIsBifacial(out.setup)
		out.confidence = snapshot.PVLowPanelConfidence
		out.samples = snapshot.PVLowPanelSamples
		out.panelCount = snapshot.PVLowPanelCount
		out.hasCount = snapshot.HasPVLowPanelCount && snapshot.PVLowPanelCount > 0
		out.nominalW = snapshot.PVLowPanelNominalWatts
		out.hasNominalW = snapshot.HasPVLowPanelNominal && snapshot.PVLowPanelNominalWatts > 0
		out.has = snapshot.HasPVLowPanelPrediction && out.setup != ""
	}
	if !out.has {
		return out
	}
	if !out.hasNominalW || out.nominalW <= 0 || !out.hasCount || out.panelCount <= 0 {
		if parsedCount, parsedPerPanelW, ok := parsePanelSetupCountAndWatts(out.setup); ok {
			if !out.hasCount || out.panelCount <= 0 {
				out.panelCount = parsedCount
				out.hasCount = parsedCount > 0
			}
			if !out.hasNominalW || out.nominalW <= 0 {
				out.nominalW = float64(parsedCount) * parsedPerPanelW
				out.hasNominalW = out.nominalW > 0
			}
		}
	}
	return out
}

func upgradePanelForChannel(snapshot *energySnapshot, channel string, alternate bool) upgradePanelTarget {
	out := upgradePanelTarget{}
	if snapshot == nil {
		return out
	}
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "high":
		if alternate {
			out.sourceKind = "metadata_alt"
			out.label = strings.TrimSpace(snapshot.PVHighAltPanelLabel)
			out.hasLabel = snapshot.HasPVHighAltPanelLabel && out.label != ""
			out.purchaseLink = strings.TrimSpace(snapshot.PVHighAltPanelLink)
			out.hasLink = snapshot.HasPVHighAltPanelLink && out.purchaseLink != ""
			out.panelWatts = snapshot.PVHighAltPanelWatts
			out.hasPanelW = snapshot.HasPVHighAltPanelWatts && snapshot.PVHighAltPanelWatts > 0
			out.panelVocV = snapshot.PVHighAltPanelVocV
			out.hasPanelVoc = snapshot.HasPVHighAltPanelVocV && snapshot.PVHighAltPanelVocV > 0
			out.panelVmpV = snapshot.PVHighAltPanelVmpV
			out.hasPanelVmp = snapshot.HasPVHighAltPanelVmpV && snapshot.PVHighAltPanelVmpV > 0
			out.panelImpA = snapshot.PVHighAltPanelImpA
			out.hasPanelImp = snapshot.HasPVHighAltPanelImpA && snapshot.PVHighAltPanelImpA > 0
			out.panelIscA = snapshot.PVHighAltPanelIscA
			out.hasPanelIsc = snapshot.HasPVHighAltPanelIscA && snapshot.PVHighAltPanelIscA > 0
			out.panelEffPct = snapshot.PVHighAltPanelEffPct
			out.hasPanelEff = snapshot.HasPVHighAltPanelEffPct && snapshot.PVHighAltPanelEffPct > 0
			out.panelEffSrc = strings.TrimSpace(snapshot.PVHighAltPanelEffSource)
			out.maxSeries = snapshot.PVHighAltPanelMaxSeries
			out.hasMaxSeries = snapshot.HasPVHighAltPanelSeries && snapshot.PVHighAltPanelMaxSeries > 0
			out.bifacial = snapshot.HasPVHighAltPanelType && snapshot.PVHighAltPanelBifacial
			if !snapshot.HasPVHighAltPanelType {
				out.bifacial = panelTextIsBifacial(out.label)
			}
			return out
		}
		out.sourceKind = "metadata_best"
		out.label = strings.TrimSpace(snapshot.PVHighBestPanelLabel)
		out.hasLabel = snapshot.HasPVHighBestPanelLabel && out.label != ""
		out.purchaseLink = strings.TrimSpace(snapshot.PVHighBestPanelLink)
		out.hasLink = snapshot.HasPVHighBestPanelLink && out.purchaseLink != ""
		out.panelWatts = snapshot.PVHighBestPanelWatts
		out.hasPanelW = snapshot.HasPVHighBestPanelWatts && snapshot.PVHighBestPanelWatts > 0
		out.panelVocV = snapshot.PVHighBestPanelVocV
		out.hasPanelVoc = snapshot.HasPVHighBestPanelVocV && snapshot.PVHighBestPanelVocV > 0
		out.panelVmpV = snapshot.PVHighBestPanelVmpV
		out.hasPanelVmp = snapshot.HasPVHighBestPanelVmpV && snapshot.PVHighBestPanelVmpV > 0
		out.panelImpA = snapshot.PVHighBestPanelImpA
		out.hasPanelImp = snapshot.HasPVHighBestPanelImpA && snapshot.PVHighBestPanelImpA > 0
		out.panelIscA = snapshot.PVHighBestPanelIscA
		out.hasPanelIsc = snapshot.HasPVHighBestPanelIscA && snapshot.PVHighBestPanelIscA > 0
		out.panelEffPct = snapshot.PVHighBestPanelEffPct
		out.hasPanelEff = snapshot.HasPVHighBestPanelEffPct && snapshot.PVHighBestPanelEffPct > 0
		out.panelEffSrc = strings.TrimSpace(snapshot.PVHighBestPanelEffSource)
		out.maxSeries = snapshot.PVHighBestPanelMaxSeries
		out.hasMaxSeries = snapshot.HasPVHighBestPanelSeries && snapshot.PVHighBestPanelMaxSeries > 0
		out.bifacial = snapshot.HasPVHighBestPanelType && snapshot.PVHighBestPanelBifacial
		if !snapshot.HasPVHighBestPanelType {
			out.bifacial = panelTextIsBifacial(out.label)
		}
	default:
		if alternate {
			out.sourceKind = "metadata_alt"
			out.label = strings.TrimSpace(snapshot.PVLowAltPanelLabel)
			out.hasLabel = snapshot.HasPVLowAltPanelLabel && out.label != ""
			out.purchaseLink = strings.TrimSpace(snapshot.PVLowAltPanelLink)
			out.hasLink = snapshot.HasPVLowAltPanelLink && out.purchaseLink != ""
			out.panelWatts = snapshot.PVLowAltPanelWatts
			out.hasPanelW = snapshot.HasPVLowAltPanelWatts && snapshot.PVLowAltPanelWatts > 0
			out.panelVocV = snapshot.PVLowAltPanelVocV
			out.hasPanelVoc = snapshot.HasPVLowAltPanelVocV && snapshot.PVLowAltPanelVocV > 0
			out.panelVmpV = snapshot.PVLowAltPanelVmpV
			out.hasPanelVmp = snapshot.HasPVLowAltPanelVmpV && snapshot.PVLowAltPanelVmpV > 0
			out.panelImpA = snapshot.PVLowAltPanelImpA
			out.hasPanelImp = snapshot.HasPVLowAltPanelImpA && snapshot.PVLowAltPanelImpA > 0
			out.panelIscA = snapshot.PVLowAltPanelIscA
			out.hasPanelIsc = snapshot.HasPVLowAltPanelIscA && snapshot.PVLowAltPanelIscA > 0
			out.panelEffPct = snapshot.PVLowAltPanelEffPct
			out.hasPanelEff = snapshot.HasPVLowAltPanelEffPct && snapshot.PVLowAltPanelEffPct > 0
			out.panelEffSrc = strings.TrimSpace(snapshot.PVLowAltPanelEffSource)
			out.maxSeries = snapshot.PVLowAltPanelMaxSeries
			out.hasMaxSeries = snapshot.HasPVLowAltPanelSeries && snapshot.PVLowAltPanelMaxSeries > 0
			out.bifacial = snapshot.HasPVLowAltPanelType && snapshot.PVLowAltPanelBifacial
			if !snapshot.HasPVLowAltPanelType {
				out.bifacial = panelTextIsBifacial(out.label)
			}
			return out
		}
		out.sourceKind = "metadata_best"
		out.label = strings.TrimSpace(snapshot.PVLowBestPanelLabel)
		out.hasLabel = snapshot.HasPVLowBestPanelLabel && out.label != ""
		out.purchaseLink = strings.TrimSpace(snapshot.PVLowBestPanelLink)
		out.hasLink = snapshot.HasPVLowBestPanelLink && out.purchaseLink != ""
		out.panelWatts = snapshot.PVLowBestPanelWatts
		out.hasPanelW = snapshot.HasPVLowBestPanelWatts && snapshot.PVLowBestPanelWatts > 0
		out.panelVocV = snapshot.PVLowBestPanelVocV
		out.hasPanelVoc = snapshot.HasPVLowBestPanelVocV && snapshot.PVLowBestPanelVocV > 0
		out.panelVmpV = snapshot.PVLowBestPanelVmpV
		out.hasPanelVmp = snapshot.HasPVLowBestPanelVmpV && snapshot.PVLowBestPanelVmpV > 0
		out.panelImpA = snapshot.PVLowBestPanelImpA
		out.hasPanelImp = snapshot.HasPVLowBestPanelImpA && snapshot.PVLowBestPanelImpA > 0
		out.panelIscA = snapshot.PVLowBestPanelIscA
		out.hasPanelIsc = snapshot.HasPVLowBestPanelIscA && snapshot.PVLowBestPanelIscA > 0
		out.panelEffPct = snapshot.PVLowBestPanelEffPct
		out.hasPanelEff = snapshot.HasPVLowBestPanelEffPct && snapshot.PVLowBestPanelEffPct > 0
		out.panelEffSrc = strings.TrimSpace(snapshot.PVLowBestPanelEffSource)
		out.maxSeries = snapshot.PVLowBestPanelMaxSeries
		out.hasMaxSeries = snapshot.HasPVLowBestPanelSeries && snapshot.PVLowBestPanelMaxSeries > 0
		out.bifacial = snapshot.HasPVLowBestPanelType && snapshot.PVLowBestPanelBifacial
		if !snapshot.HasPVLowBestPanelType {
			out.bifacial = panelTextIsBifacial(out.label)
		}
	}
	return out
}

func upgradePanelCandidatesForChannel(snapshot *energySnapshot, channel string) []upgradePanelTarget {
	if snapshot == nil {
		return nil
	}
	var source []panelDBCandidate
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "high":
		if !snapshot.HasPVHighDBCandidates || len(snapshot.PVHighDBCandidates) == 0 {
			return nil
		}
		source = snapshot.PVHighDBCandidates
	default:
		if !snapshot.HasPVLowDBCandidates || len(snapshot.PVLowDBCandidates) == 0 {
			return nil
		}
		source = snapshot.PVLowDBCandidates
	}
	out := make([]upgradePanelTarget, 0, len(source))
	seen := make(map[string]struct{}, len(source))
	for _, candidate := range source {
		label := strings.TrimSpace(candidate.Label)
		if label == "" {
			continue
		}
		key := strings.ToLower(label)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		target := upgradePanelTarget{
			label:        label,
			hasLabel:     true,
			purchaseLink: strings.TrimSpace(candidate.PurchaseLink),
			hasLink:      strings.TrimSpace(candidate.PurchaseLink) != "",
			status:       strings.TrimSpace(candidate.Status),
			sourceKind:   "db",
			panelWatts:   candidate.PanelWatts,
			hasPanelW:    candidate.PanelWatts > 0,
			panelVocV:    candidate.VocV,
			hasPanelVoc:  candidate.VocV > 0,
			panelVmpV:    candidate.VmpV,
			hasPanelVmp:  candidate.VmpV > 0,
			panelImpA:    candidate.ImpA,
			hasPanelImp:  candidate.ImpA > 0,
			panelIscA:    candidate.IscA,
			hasPanelIsc:  candidate.IscA > 0,
			panelEffPct:  candidate.ModuleEfficiencyPct,
			hasPanelEff:  candidate.ModuleEfficiencyPct > 0,
			panelEffSrc:  strings.TrimSpace(candidate.ModuleEfficiencySrc),
			minSeries:    candidate.MinSeries,
			hasMinSeries: candidate.MinSeries > 0,
			maxSeries:    candidate.MaxSeries,
			hasMaxSeries: candidate.MaxSeries > 0,
			bifacial:     candidate.Bifacial,
		}
		if target.hasPanelW {
			out = append(out, target)
		}
	}
	return out
}

func baselineSolarPortPotential(port solarRecommendationPort, hasObserved bool, observedWatts float64) (float64, float64, bool) {
	if port.detected.has && port.detected.hasNominalW && port.detected.nominalW > 0 {
		nominal := port.detected.nominalW
		potential := nominal
		if port.hasMaxWatts && port.maxWatts > 0 {
			potential = math.Min(nominal, port.maxWatts)
		}
		return potential, nominal, true
	}
	if hasObserved {
		observed := math.Abs(observedWatts)
		if observed > 0 {
			return observed, observed, true
		}
	}
	return 0, 0, false
}

func buildAddPanelsOption(port solarRecommendationPort, peer solarRecommendationPort) solarRecommendationOption {
	if !port.hasMaxWatts || port.maxWatts <= 0 {
		return solarRecommendationOption{text: "n/a", potentialW: 0, hasPotential: false}
	}

	addSource := port.detected
	usingPeerFallback := false
	currentW := 0.0
	if port.detected.has && port.detected.hasNominalW && port.detected.nominalW > 0 {
		currentW = math.Min(port.detected.nominalW, port.maxWatts)
		if currentW >= port.maxWatts*0.96 {
			return solarRecommendationOption{
				text:         "already near port max",
				potentialW:   currentW,
				hasPotential: true,
				sourceLabel:  normalizePeerSetupLabel(addSource.setup),
				units:        0,
			}
		}
	} else if peer.detected.has && peer.detected.hasNominalW && peer.detected.nominalW > 0 {
		addSource = peer.detected
		usingPeerFallback = true
		currentW = 0
	} else {
		return solarRecommendationOption{text: "waiting for panel detection", potentialW: 0, hasPotential: false}
	}

	perPanelW := 0.0
	if addSource.hasCount && addSource.panelCount > 0 {
		perPanelW = addSource.nominalW / float64(addSource.panelCount)
	}
	if perPanelW <= 0 {
		if _, parsedPerPanelW, ok := parsePanelSetupCountAndWatts(addSource.setup); ok {
			perPanelW = parsedPerPanelW
		}
	}
	if perPanelW <= 0 {
		headroom := math.Max(port.maxWatts-currentW, 0)
		recommendationPrefix := "add"
		if usingPeerFallback {
			recommendationPrefix = "mirror peer setup: add"
		}
		return solarRecommendationOption{
			text:         fmt.Sprintf("%s ~%s panels (target %s)", recommendationPrefix, formatPVCapacityWatts(headroom), formatPVCapacityWatts(port.maxWatts)),
			nominalW:     port.maxWatts,
			potentialW:   port.maxWatts,
			hasPotential: true,
			sourceLabel:  normalizePeerSetupLabel(addSource.setup),
		}
	}

	headroomW := math.Max(port.maxWatts-currentW, 0)
	units := int(math.Ceil(headroomW / perPanelW))
	if units < 1 {
		units = 1
	}
	projectedNominalW := addSource.nominalW + (float64(units) * perPanelW)
	if usingPeerFallback {
		projectedNominalW = float64(units) * perPanelW
	}
	projectedPotentialW := math.Min(projectedNominalW, port.maxWatts)
	recommendationPrefix := "add"
	if usingPeerFallback {
		recommendationPrefix = "mirror peer setup: add"
	}
	return solarRecommendationOption{
		text: fmt.Sprintf(
			"%s %dx ~%s panel (%s -> %s)",
			recommendationPrefix,
			units,
			formatPVCapacityWatts(perPanelW),
			formatPVCapacityWatts(currentW),
			formatPVCapacityWatts(projectedPotentialW),
		),
		nominalW:     projectedNominalW,
		potentialW:   projectedPotentialW,
		hasPotential: true,
		sourceLabel:  normalizePeerSetupLabel(addSource.setup),
		units:        units,
	}
}

func buildUpgradePanelsOption(port solarRecommendationPort, target upgradePanelTarget, preferNoClip bool) solarRecommendationOption {
	return buildUpgradePanelsOptionWithExclude(port, target, preferNoClip, panelLayout{})
}

func selectUpgradeRecommendationPair(port solarRecommendationPort, peer solarRecommendationPort) (solarRecommendationOption, solarRecommendationOption) {
	targets := collectUpgradeTargets(port, peer)
	if len(targets) == 0 {
		if primary, ok := fallbackUpgradeFromPeerDetection(port, peer); ok {
			return primary, solarRecommendationOption{text: "n/a", potentialW: 0, hasPotential: false}
		}
		return solarRecommendationOption{text: "n/a", potentialW: 0, hasPotential: false},
			solarRecommendationOption{text: "n/a", potentialW: 0, hasPotential: false}
	}

	options := make([]solarRecommendationOption, 0, len(targets))
	targetByLabel := make(map[string]upgradePanelTarget, len(targets))
	for _, target := range targets {
		key := normalizeUpgradeTargetLabel(target.label)
		if _, ok := targetByLabel[key]; !ok {
			targetByLabel[key] = target
		}
		option := buildUpgradePanelsOption(port, target, false)
		if option.hasPotential {
			options = append(options, option)
		}
	}
	if len(options) == 0 {
		if primary, ok := fallbackUpgradeFromPeerDetection(port, peer); ok {
			return primary, solarRecommendationOption{text: "n/a", potentialW: 0, hasPotential: false}
		}
		return solarRecommendationOption{text: "n/a (no safe layout within MPPT limits)", potentialW: 0, hasPotential: false},
			solarRecommendationOption{text: "n/a (no safe layout within MPPT limits)", potentialW: 0, hasPotential: false}
	}

	sort.SliceStable(options, func(i, j int) bool {
		return shouldPreferUpgradeOption(options[i], options[j], port.maxWatts)
	})
	primary := options[0]

	secondary := solarRecommendationOption{}
	sameModelFallback := solarRecommendationOption{}
	requireNoClip := primary.clipped
	for idx := 1; idx < len(options); idx++ {
		candidate := options[idx]
		if requireNoClip && candidate.clipped {
			continue
		}
		if !isMeaningfulAlternateLayout(primary, candidate, port.maxWatts) {
			continue
		}
		if sameUpgradeModel(primary, candidate) {
			if !sameModelFallback.hasPotential || shouldPreferUpgradeOption(candidate, sameModelFallback, port.maxWatts) {
				sameModelFallback = candidate
			}
			continue
		}
		secondary = candidate
		break
	}

	if !secondary.hasPotential && requireNoClip {
		noClipCandidates := make([]solarRecommendationOption, 0, len(targets))
		for _, target := range targets {
			candidate := buildUpgradePanelsOption(port, target, true)
			if !candidate.hasPotential || candidate.clipped {
				continue
			}
			noClipCandidates = append(noClipCandidates, candidate)
		}
		sort.SliceStable(noClipCandidates, func(i, j int) bool {
			return shouldPreferUpgradeOption(noClipCandidates[i], noClipCandidates[j], port.maxWatts)
		})
		for _, candidate := range noClipCandidates {
			if !isMeaningfulAlternateLayout(primary, candidate, port.maxWatts) {
				continue
			}
			if sameUpgradeModel(primary, candidate) {
				if !sameModelFallback.hasPotential || shouldPreferUpgradeOption(candidate, sameModelFallback, port.maxWatts) {
					sameModelFallback = candidate
				}
				continue
			}
			secondary = candidate
			break
		}
	}

	if !secondary.hasPotential && primary.hasPotential {
		if target, ok := targetByLabel[normalizeUpgradeTargetLabel(primary.sourceLabel)]; ok {
			exclude := panelLayout{
				series:   primary.series,
				parallel: primary.parallel,
				units:    primary.units,
			}
			altLayout := buildUpgradePanelsOptionWithExclude(port, target, primary.clipped, exclude)
			if !altLayout.hasPotential {
				altLayout = buildUpgradePanelsOptionWithExclude(port, target, !primary.clipped, exclude)
			}
			if altLayout.hasPotential && isMeaningfulAlternateLayout(primary, altLayout, port.maxWatts) {
				if altLayout.text != "" {
					altLayout.text += " (alt layout)"
				}
				secondary = altLayout
			}
		}
	}

	if !secondary.hasPotential && sameModelFallback.hasPotential {
		secondary = sameModelFallback
	}

	if !secondary.hasPotential {
		if primary.clipped {
			secondary = solarRecommendationOption{
				text:         "n/a (no non-clipping option)",
				potentialW:   0,
				hasPotential: false,
				clipped:      false,
			}
		} else {
			secondary = solarRecommendationOption{text: "n/a", potentialW: 0, hasPotential: false}
		}
	}

	return primary, secondary
}

func fallbackUpgradeFromPeerDetection(port solarRecommendationPort, peer solarRecommendationPort) (solarRecommendationOption, bool) {
	target, ok := peerDetectedUpgradeTargetAnyCapability(port, peer)
	if !ok {
		return solarRecommendationOption{}, false
	}
	primary := buildUpgradePanelsOption(port, target, false)
	if !primary.hasPotential {
		return solarRecommendationOption{}, false
	}
	return primary, true
}

func collectUpgradeTargets(port solarRecommendationPort, peer solarRecommendationPort) []upgradePanelTarget {
	targets := make([]upgradePanelTarget, 0, len(port.dbCandidates)+4)
	targets = appendUniqueUpgradeTarget(targets, port.upgrade)
	targets = appendUniqueUpgradeTarget(targets, port.upgradeAlt)
	for _, candidate := range port.dbCandidates {
		targets = appendUniqueUpgradeTarget(targets, candidate)
	}
	if detectedTarget, ok := detectedSetupUpgradeTarget(port.detected); ok {
		targets = appendUniqueUpgradeTarget(targets, detectedTarget)
	}
	if peerTarget, ok := peerDetectedUpgradeTarget(port, peer); ok {
		targets = appendUniqueUpgradeTarget(targets, peerTarget)
	}
	return targets
}

func appendUniqueUpgradeTarget(targets []upgradePanelTarget, target upgradePanelTarget) []upgradePanelTarget {
	if !target.hasLabel || strings.TrimSpace(target.label) == "" || !target.hasPanelW || target.panelWatts <= 0 {
		return targets
	}
	newKey := normalizeUpgradeTargetKey(target)
	for _, existing := range targets {
		if normalizeUpgradeTargetKey(existing) == newKey {
			return targets
		}
	}
	return append(targets, target)
}

func normalizeUpgradeTargetLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

func normalizeUpgradeTargetKey(target upgradePanelTarget) string {
	label := normalizeUpgradeTargetLabel(target.label)
	return fmt.Sprintf(
		"%s|%.2f|%.2f|%.2f|%.2f|%.2f|%d|%d",
		label,
		target.panelWatts,
		target.panelVocV,
		target.panelVmpV,
		target.panelImpA,
		target.panelIscA,
		target.minSeries,
		target.maxSeries,
	)
}

func sameUpgradeModel(a solarRecommendationOption, b solarRecommendationOption) bool {
	return normalizeUpgradeTargetLabel(a.sourceLabel) == normalizeUpgradeTargetLabel(b.sourceLabel)
}

func peerDetectedUpgradeTarget(port solarRecommendationPort, peer solarRecommendationPort) (upgradePanelTarget, bool) {
	target := upgradePanelTarget{}
	if !port.hasMaxWatts || port.maxWatts <= 0 {
		return target, false
	}
	if !portsSharePVCapability(port, peer) {
		return target, false
	}
	if !peer.detected.has || !peer.detected.hasNominalW || peer.detected.nominalW <= 0 {
		return target, false
	}
	if !peer.detected.hasCount || peer.detected.panelCount <= 0 {
		return target, false
	}
	perPanelW := peer.detected.nominalW / float64(peer.detected.panelCount)
	if perPanelW <= 0 {
		return target, false
	}
	target.label = normalizePeerSetupLabel(peer.detected.setup)
	target.hasLabel = target.label != ""
	target.sourceKind = "peer"
	target.panelWatts = perPanelW
	target.hasPanelW = perPanelW > 0
	target.bifacial = peer.detected.bifacial
	if !target.hasLabel || !target.hasPanelW {
		return target, false
	}
	return target, true
}

func peerDetectedUpgradeTargetAnyCapability(port solarRecommendationPort, peer solarRecommendationPort) (upgradePanelTarget, bool) {
	target := upgradePanelTarget{}
	if !port.hasMaxWatts || port.maxWatts <= 0 {
		return target, false
	}
	if !peer.detected.has || !peer.detected.hasNominalW || peer.detected.nominalW <= 0 {
		return target, false
	}
	panelCount := peer.detected.panelCount
	if !peer.detected.hasCount || panelCount <= 0 {
		if parsedCount, _, ok := parsePanelSetupCountAndWatts(peer.detected.setup); ok {
			panelCount = parsedCount
		}
	}
	if panelCount <= 0 {
		return target, false
	}
	perPanelW := peer.detected.nominalW / float64(panelCount)
	if perPanelW <= 0 {
		return target, false
	}
	target.label = normalizePeerSetupLabel(peer.detected.setup)
	target.hasLabel = target.label != ""
	target.sourceKind = "peer_fallback"
	target.panelWatts = perPanelW
	target.hasPanelW = perPanelW > 0
	target.bifacial = peer.detected.bifacial
	if !target.hasLabel || !target.hasPanelW {
		return target, false
	}
	return target, true
}

func detectedSetupUpgradeTarget(detected detectedPanelSetup) (upgradePanelTarget, bool) {
	target := upgradePanelTarget{}
	if !detected.has || !detected.hasNominalW || detected.nominalW <= 0 {
		return target, false
	}
	panelCount := detected.panelCount
	if !detected.hasCount || panelCount <= 0 {
		if parsedCount, _, ok := parsePanelSetupCountAndWatts(detected.setup); ok {
			panelCount = parsedCount
		}
	}
	if panelCount <= 0 {
		return target, false
	}
	perPanelW := detected.nominalW / float64(panelCount)
	if perPanelW <= 0 {
		if _, parsedPerPanelW, ok := parsePanelSetupCountAndWatts(detected.setup); ok && parsedPerPanelW > 0 {
			perPanelW = parsedPerPanelW
		}
	}
	if perPanelW <= 0 {
		return target, false
	}
	target.label = normalizePeerSetupLabel(detected.setup)
	target.hasLabel = target.label != ""
	target.sourceKind = "detected"
	target.panelWatts = perPanelW
	target.hasPanelW = perPanelW > 0
	target.bifacial = detected.bifacial
	if !target.hasLabel || !target.hasPanelW {
		return target, false
	}
	return target, true
}

func buildUpgradePanelsOptionWithExclude(port solarRecommendationPort, target upgradePanelTarget, preferNoClip bool, exclude panelLayout) solarRecommendationOption {
	if !target.hasLabel || !target.hasPanelW || target.panelWatts <= 0 {
		return solarRecommendationOption{text: "n/a", potentialW: 0, hasPotential: false}
	}
	if !port.hasMaxWatts || port.maxWatts <= 0 {
		return solarRecommendationOption{text: "n/a", potentialW: 0, hasPotential: false}
	}
	layout, ok := selectSafePanelLayout(port, target, preferNoClip, exclude)
	if !ok {
		if preferNoClip {
			return solarRecommendationOption{text: "n/a (no safe non-clipping layout)", potentialW: 0, hasPotential: false}
		}
		return solarRecommendationOption{text: "n/a (no safe layout within MPPT limits)", potentialW: 0, hasPotential: false}
	}

	text := fmt.Sprintf(
		"replace with %dx %s (%s, ~%s STC",
		layout.units,
		target.label,
		formatSeriesParallel(layout.series, layout.parallel),
		formatPVCapacityWatts(layout.nominalW),
	)
	if layout.clipped {
		text += fmt.Sprintf(", clipped to %s", formatPVCapacityWatts(port.maxWatts))
	}
	text += ")"
	return solarRecommendationOption{
		text:         text,
		nominalW:     layout.nominalW,
		potentialW:   layout.potentialW,
		hasPotential: true,
		clipped:      layout.clipped,
		bifacial:     target.bifacial,
		effPct:       target.panelEffPct,
		hasEffPct:    target.hasPanelEff && target.panelEffPct > 0,
		effSrc:       target.panelEffSrc,
		sourceLabel:  target.label,
		sourceLink:   target.purchaseLink,
		sourceKind:   target.sourceKind,
		series:       layout.series,
		parallel:     layout.parallel,
		units:        layout.units,
		complexity:   panelLayoutComplexityScore(layout),
	}
}

func portsSharePVCapability(a, b solarRecommendationPort) bool {
	if !a.hasMaxWatts || !b.hasMaxWatts {
		return false
	}
	if math.Abs(a.maxWatts-b.maxWatts) > 0.5 {
		return false
	}
	if a.hasCap && b.hasCap {
		if math.Abs(a.capability.minVolts-b.capability.minVolts) > 0.5 {
			return false
		}
		if math.Abs(a.capability.maxVolts-b.capability.maxVolts) > 0.5 {
			return false
		}
		if math.Abs(a.capability.maxAmps-b.capability.maxAmps) > 0.2 {
			return false
		}
	}
	return true
}

func normalizePeerSetupLabel(setup string) string {
	setup = strings.TrimSpace(setup)
	if setup == "" {
		return ""
	}
	if _, perPanelW, ok := parsePanelSetupCountAndWatts(setup); ok && perPanelW > 0 {
		lower := strings.ToLower(setup)
		if idx := strings.Index(lower, "w"); idx >= 0 && idx+1 < len(setup) {
			candidate := strings.TrimSpace(setup[idx+1:])
			if candidate != "" {
				return candidate
			}
		}
	}
	return setup
}

func shouldPreferUpgradeOption(candidate solarRecommendationOption, current solarRecommendationOption, maxWatts float64) bool {
	if !candidate.hasPotential {
		return false
	}
	if !current.hasPotential {
		return true
	}
	candidateEffective := upgradeOptionEffectiveWatts(candidate, maxWatts)
	currentEffective := upgradeOptionEffectiveWatts(current, maxWatts)
	candidateEffective += recommendationEfficiencyBoostWatts(candidate, maxWatts)
	currentEffective += recommendationEfficiencyBoostWatts(current, maxWatts)
	nearMaxCandidate := maxWatts > 0 && candidate.potentialW >= maxWatts*0.95
	nearMaxCurrent := maxWatts > 0 && current.potentialW >= maxWatts*0.95
	candidateFewerPanelsNearMax := candidate.units > 0 && current.units > 0 &&
		candidate.units+2 <= current.units && nearMaxCandidate && nearMaxCurrent
	candidateComplexity := recommendationOptionComplexity(candidate)
	currentComplexity := recommendationOptionComplexity(current)
	complexityPenaltyPerPoint := math.Max(8, maxWatts*panelComplexityPenaltyFactor)
	adjustedCandidate := candidateEffective - (candidateComplexity * complexityPenaltyPerPoint)
	adjustedCurrent := currentEffective - (currentComplexity * complexityPenaltyPerPoint)
	if math.Abs(adjustedCandidate-adjustedCurrent) > math.Max(10, maxWatts*0.03) {
		return adjustedCandidate > adjustedCurrent
	}
	if candidateEffective > currentEffective+1 {
		return true
	}
	if currentEffective > candidateEffective+1 && !candidateFewerPanelsNearMax {
		return false
	}
	closeBand := math.Max(10, maxWatts*0.03)
	if math.Abs(candidateEffective-currentEffective) <= closeBand {
		candidateSourceRank := recommendationSourceRank(candidate.sourceKind)
		currentSourceRank := recommendationSourceRank(current.sourceKind)
		if candidateSourceRank != currentSourceRank {
			return candidateSourceRank > currentSourceRank
		}
	}
	nearEqualBand := math.Max(closeBand, maxWatts*0.04)
	if candidate.units > 0 && current.units > 0 && candidate.units != current.units {
		if math.Abs(candidateEffective-currentEffective) <= nearEqualBand {
			return candidate.units < current.units
		}
	}
	if candidateFewerPanelsNearMax {
		unitReduction := current.units - candidate.units
		if unitReduction >= 2 {
			effectiveGap := currentEffective - candidateEffective
			// Near MPPT saturation, prefer materially simpler wiring if energy is close.
			allowedGap := math.Max(20, maxWatts*0.05)
			if effectiveGap <= allowedGap {
				return true
			}
		}
	}
	if candidate.clipped != current.clipped {
		// When both options are effectively near max and close in energy outcome,
		// prefer materially simpler wiring even if it clips.
		if nearMaxCandidate && nearMaxCurrent && math.Abs(candidateComplexity-currentComplexity) >= 0.75 {
			return candidateComplexity < currentComplexity
		}
		// Product preference: favor overpaneling/clipping setups for better shoulder-hours capture.
		return candidate.clipped
	}
	if candidate.units > 0 && current.units > 0 && candidate.units != current.units {
		return candidate.units < current.units
	}
	nearMax := maxWatts > 0 && math.Abs(candidate.potentialW-maxWatts) <= math.Max(1, maxWatts*0.03)
	currentNearMax := maxWatts > 0 && math.Abs(current.potentialW-maxWatts) <= math.Max(1, maxWatts*0.03)
	if nearMax != currentNearMax {
		return nearMax
	}
	return false
}

func recommendationOptionComplexity(option solarRecommendationOption) float64 {
	if option.complexity > 0 {
		return adjustedPanelLayoutComplexity(option.complexity, option.sourceLabel, option.parallel)
	}
	layout := panelLayout{
		series:   option.series,
		parallel: option.parallel,
		units:    option.units,
	}
	base := panelLayoutComplexityScore(layout)
	return adjustedPanelLayoutComplexity(base, option.sourceLabel, option.parallel)
}

func recommendationSourceRank(sourceKind string) int {
	switch strings.ToLower(strings.TrimSpace(sourceKind)) {
	case "detected", "peer", "peer_fallback":
		return 3
	case "db", "metadata_best", "metadata_alt":
		return 2
	default:
		return 1
	}
}

func recommendationEfficiencyBoostWatts(option solarRecommendationOption, maxWatts float64) float64 {
	if maxWatts <= 0 || !option.hasEffPct || option.effPct <= 0 {
		return 0
	}
	score := normalizedPanelEfficiencyScore(option.effPct, option.bifacial)
	if score <= 0 {
		return 0
	}
	src := strings.ToLower(strings.TrimSpace(option.effSrc))
	if strings.HasPrefix(src, "estimated_") {
		score *= panelEstimatedEfficiencyWeight
	}
	return maxWatts * panelEfficiencyBoostFactor * score
}

func normalizedPanelEfficiencyScore(effPct float64, bifacial bool) float64 {
	if effPct <= 0 {
		return 0
	}
	baseline := 18.0
	upper := 22.0
	if bifacial {
		baseline = 20.0
		upper = 25.0
	}
	if upper <= baseline {
		return 0
	}
	score := (effPct - baseline) / (upper - baseline)
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func upgradeOptionEffectiveWatts(option solarRecommendationOption, maxWatts float64) float64 {
	base := option.potentialW
	if maxWatts <= 0 {
		return base
	}
	nominal := option.nominalW
	if nominal <= maxWatts || nominal <= 0 {
		return base
	}
	oversizeRatio := (nominal / maxWatts) - 1.0
	if oversizeRatio <= 0 {
		return base
	}
	// Use a logarithmic shoulder-hours gain so oversizing helps, but with diminishing returns.
	bonus := maxWatts * panelShoulderHoursGain * math.Log1p(oversizeRatio)
	maxBonus := maxWatts * 0.25
	if bonus > maxBonus {
		bonus = maxBonus
	}
	return base + bonus
}

func solarRecommendationETAW(potentialW float64, nominalW float64, maxWatts float64, hasMaxWatts bool, bifacial bool) float64 {
	option := solarRecommendationOption{
		potentialW: potentialW,
		nominalW:   nominalW,
	}
	if option.nominalW <= 0 {
		option.nominalW = option.potentialW
	}
	watts := option.potentialW
	if hasMaxWatts && maxWatts > 0 {
		watts = upgradeOptionEffectiveWatts(option, maxWatts)
	}
	return applyBifacialETAAdjustment(watts, bifacial)
}

func solarRecommendationOptionETAW(option solarRecommendationOption, maxWatts float64, hasMaxWatts bool, bifacial bool) float64 {
	return solarRecommendationETAW(option.potentialW, option.nominalW, maxWatts, hasMaxWatts, bifacial)
}

func adjustedPanelLayoutComplexity(base float64, label string, parallel int) float64 {
	if base <= 0 {
		return base
	}
	if !isEcoFlow125BifacialModularPanel(label) {
		return base
	}
	if parallel < 1 || parallel > 4 {
		return base
	}
	return base * ecoflow125ComplexityFactor
}

func isEcoFlow125BifacialModularPanel(label string) bool {
	v := strings.ToLower(strings.TrimSpace(label))
	if v == "" {
		return false
	}
	return strings.Contains(v, "ecoflow") &&
		strings.Contains(v, "125w") &&
		strings.Contains(v, "bifacial") &&
		strings.Contains(v, "modular")
}

func isMeaningfulAlternateLayout(primary solarRecommendationOption, alt solarRecommendationOption, maxWatts float64) bool {
	if !primary.hasPotential || !alt.hasPotential {
		return false
	}
	sameModel := strings.EqualFold(strings.TrimSpace(primary.sourceLabel), strings.TrimSpace(alt.sourceLabel))
	// Prefer diversity for alternate recommendations. Same-model alternates are only useful
	// when they materially change clipping behavior.
	if sameModel {
		return primary.clipped != alt.clipped
	}
	// If this isn't the same panel/model recommendation, keep it.
	if !sameModel {
		return true
	}
	// Reject tiny same-model tweaks that only reduce panel count a little (e.g. 6S1P -> 5S1P).
	if primary.parallel > 0 && alt.parallel > 0 && primary.parallel == alt.parallel &&
		primary.series > 0 && alt.series > 0 && absInt(primary.series-alt.series) <= 1 {
		return false
	}
	// Reject near-identical power outcomes for same model.
	diff := math.Abs(primary.potentialW - alt.potentialW)
	minDelta := math.Max(15, maxWatts*0.08)
	if diff < minDelta {
		return false
	}
	return true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type panelLayout struct {
	series     int
	parallel   int
	units      int
	nominalW   float64
	potentialW float64
	clipped    bool
}

func selectSafePanelLayout(port solarRecommendationPort, target upgradePanelTarget, preferNoClip bool, exclude panelLayout) (panelLayout, bool) {
	if !port.hasMaxWatts || port.maxWatts <= 0 || !target.hasPanelW || target.panelWatts <= 0 {
		return panelLayout{}, false
	}

	sMin := 1
	sMax := 12
	if target.hasMinSeries && target.minSeries > sMin {
		sMin = target.minSeries
	}
	if target.hasMaxSeries && target.maxSeries > 0 && target.maxSeries < sMax {
		sMax = target.maxSeries
	}
	if target.hasPanelVoc && target.panelVocV > 0 && port.hasCap && port.capability.maxVolts > 0 {
		coldVoc := target.panelVocV * coldVocRiseFactor
		if coldVoc > 0 {
			vocMaxSeries := int(math.Floor(port.capability.maxVolts / coldVoc))
			if vocMaxSeries < sMax {
				sMax = vocMaxSeries
			}
		}
	}
	if target.hasPanelVmp && target.panelVmpV > 0 && port.hasCap {
		if port.capability.minVolts > 0 {
			minByVmp := int(math.Ceil(port.capability.minVolts / target.panelVmpV))
			if minByVmp > sMin {
				sMin = minByVmp
			}
		}
		if port.capability.maxVolts > 0 {
			maxByVmp := int(math.Floor(port.capability.maxVolts / target.panelVmpV))
			if maxByVmp < sMax {
				sMax = maxByVmp
			}
		}
	}
	if sMin < 1 {
		sMin = 1
	}
	if sMax < sMin || sMax < 1 {
		return panelLayout{}, false
	}

	pMax := 16
	if port.hasCap && port.capability.maxAmps > 0 {
		perPanelCurrent := panelCurrentLimitValue(target)
		if perPanelCurrent > 0 {
			maxParallel := int(math.Floor(port.capability.maxAmps / perPanelCurrent))
			if maxParallel < pMax {
				pMax = maxParallel
			}
		}
	}
	if pMax < 1 {
		return panelLayout{}, false
	}

	best := panelLayout{}
	hasBest := false
	for s := sMin; s <= sMax; s++ {
		for p := 1; p <= pMax; p++ {
			units := s * p
			if units < 1 {
				continue
			}
			nominal := float64(units) * target.panelWatts
			if nominal <= 0 {
				continue
			}
			clipped := nominal > port.maxWatts*1.05
			if preferNoClip && clipped {
				continue
			}
			potential := math.Min(nominal, port.maxWatts)
			if potential <= 0 {
				continue
			}

			candidate := panelLayout{
				series:     s,
				parallel:   p,
				units:      units,
				nominalW:   nominal,
				potentialW: potential,
				clipped:    clipped,
			}
			if exclude.units > 0 && candidate.series == exclude.series && candidate.parallel == exclude.parallel {
				continue
			}
			if !hasBest || panelLayoutBetter(candidate, best) {
				best = candidate
				hasBest = true
			}
		}
	}
	return best, hasBest
}

func panelLayoutBetter(a, b panelLayout) bool {
	aComplexity := panelLayoutComplexityScore(a)
	bComplexity := panelLayoutComplexityScore(b)
	if math.Abs(a.potentialW-b.potentialW) <= math.Max(15, a.potentialW*0.05) {
		if math.Abs(aComplexity-bComplexity) > 0.5 {
			return aComplexity < bComplexity
		}
	}
	if a.potentialW > b.potentialW+1 {
		return true
	}
	if b.potentialW > a.potentialW+1 {
		return false
	}
	if a.units != b.units {
		return a.units < b.units
	}
	if math.Abs(aComplexity-bComplexity) > 0.5 {
		return aComplexity < bComplexity
	}
	if a.clipped != b.clipped {
		// For equal potential and equal panel count, prefer non-clipping.
		return !a.clipped
	}
	if a.series != b.series {
		// Prefer higher series / lower parallel for lower current.
		return a.series > b.series
	}
	return a.parallel < b.parallel
}

func panelLayoutComplexityScore(layout panelLayout) float64 {
	if layout.units <= 0 {
		return 0
	}
	series := float64(layout.series)
	if series < 1 {
		series = 1
	}
	parallel := float64(layout.parallel)
	if parallel < 1 {
		parallel = 1
	}
	units := float64(layout.units)

	score := units
	if parallel > 1 {
		score += (parallel - 1) * 4.0
	}
	if series > 1 {
		score += (series - 1) * 0.3
	}
	if series > 1 && parallel > 1 {
		score += 10.0
	}
	if units > 12 {
		score += (units - 12) * 1.8
	}
	return score
}

func panelCurrentLimitValue(target upgradePanelTarget) float64 {
	if target.hasPanelImp && target.panelImpA > 0 {
		return target.panelImpA
	}
	if target.hasPanelIsc && target.panelIscA > 0 {
		return target.panelIscA
	}
	return 0
}

func formatSeriesParallel(series int, parallel int) string {
	if series < 1 {
		series = 1
	}
	if parallel < 1 {
		parallel = 1
	}
	return fmt.Sprintf("%dS%dP", series, parallel)
}

func formatDetectedPanelRecommendation(detected detectedPanelSetup, maxWatts float64, hasMaxWatts bool) string {
	if !detected.has || strings.TrimSpace(detected.setup) == "" {
		if strings.TrimSpace(detected.status) != "" {
			return detected.status
		}
		return "panel: n/a"
	}
	parts := []string{detected.setup}
	if strings.TrimSpace(detected.status) != "" {
		parts = append(parts, "("+detected.status+")")
		return strings.Join(parts, " ")
	}
	if detected.hasNominalW && detected.nominalW > 0 {
		if hasMaxWatts && maxWatts > 0 {
			parts = append(parts, fmt.Sprintf("~%s/%s", formatPVCapacityWatts(math.Min(detected.nominalW, maxWatts)), formatPVCapacityWatts(maxWatts)))
		} else {
			parts = append(parts, fmt.Sprintf("~%s", formatPVCapacityWatts(detected.nominalW)))
		}
	}
	parts = append(parts, fmt.Sprintf("c=%.2f", detected.confidence))
	if detected.samples > 0 {
		parts = append(parts, fmt.Sprintf("n=%d", detected.samples))
	}
	return strings.Join(parts, " ")
}

func buildSolarETAImpact(
	energyToTargetWh float64,
	hasEnergyToTarget bool,
	baseTotalW float64,
	addTotalW float64,
	hasAdd bool,
	upgradeTotalW float64,
	hasUpgrade bool,
) string {
	if !hasEnergyToTarget {
		return "n/a"
	}
	if energyToTargetWh <= 0 {
		return "at target SOC"
	}

	baseMinutes, hasBase := estimateSolarChargeMinutes(energyToTargetWh, baseTotalW)
	addMinutes, hasAddETA := estimateSolarChargeMinutes(energyToTargetWh, addTotalW)
	upgradeMinutes, hasUpgradeETA := estimateSolarChargeMinutes(energyToTargetWh, upgradeTotalW)

	baseText := "n/a"
	if hasBase {
		baseText = formatETAMinutes(baseMinutes)
	}
	addText := "n/a"
	if hasAdd && hasAddETA {
		addText = formatETAMinutes(addMinutes)
	}
	upgradeText := "n/a"
	if hasUpgrade && hasUpgradeETA {
		upgradeText = formatETAMinutes(upgradeMinutes)
	}
	return fmt.Sprintf(
		"sunny base: %s | add: %s (%s) | upg: %s (%s)",
		baseText,
		addText,
		formatMinutesDelta(baseMinutes, hasBase, addMinutes, hasAddETA && hasAdd),
		upgradeText,
		formatMinutesDelta(baseMinutes, hasBase, upgradeMinutes, hasUpgradeETA && hasUpgrade),
	)
}

type upgradePathSelection struct {
	channel      string
	portLabel    string
	option       solarRecommendationOption
	hasOption    bool
	portMaxWatts float64
	hasPortMaxW  bool
}

type upgradePathScenario struct {
	name       string
	totalWatts float64
	hasAny     bool
	minutes    float64
	hasMinutes bool
	steps      []upgradePathSelection
	stepCount  int
	complexity float64
}

type upgradePathChoice struct {
	name          string
	selection     upgradePathSelection
	hasOption     bool
	potentialETAW float64
	complexity    float64
}

func buildBestUpgradePathSummary(
	portRows []solarPortRecommendationData,
	energyToTargetWh float64,
	hasEnergyToTarget bool,
	baseTotalW float64,
) string {
	if len(portRows) == 0 || !hasEnergyToTarget || energyToTargetWh <= 0 {
		return ""
	}
	baseMinutes, hasBaseMinutes := estimateSolarChargeMinutes(energyToTargetWh, baseTotalW)

	choicesByPort := make([][]upgradePathChoice, 0, len(portRows))
	for _, port := range portRows {
		baseSelection := upgradePathSelection{
			channel:      port.channel,
			portLabel:    port.label,
			hasOption:    false,
			portMaxWatts: port.portMaxWatts,
			hasPortMaxW:  port.hasPortMaxWatts,
		}
		portChoices := []upgradePathChoice{{
			name:          "base",
			selection:     baseSelection,
			hasOption:     false,
			potentialETAW: port.basePotentialETAW,
			complexity:    0,
		}}
		appendChoice := func(name string, option solarRecommendationOption) {
			if !option.hasPotential {
				return
			}
			selection := upgradePathSelection{
				channel:      port.channel,
				portLabel:    port.label,
				option:       option,
				hasOption:    true,
				portMaxWatts: port.portMaxWatts,
				hasPortMaxW:  port.hasPortMaxWatts,
			}
			choice := upgradePathChoice{
				name:          name,
				selection:     selection,
				hasOption:     true,
				potentialETAW: solarRecommendationOptionETAW(option, port.portMaxWatts, port.hasPortMaxWatts, option.bifacial),
				complexity:    recommendationOptionComplexity(option),
			}
			for _, existing := range portChoices {
				if !existing.hasOption {
					continue
				}
				if sameUpgradePathOption(existing.selection.option, option) {
					return
				}
			}
			portChoices = append(portChoices, choice)
		}
		appendChoice("add", port.addOption)
		appendChoice("upg1", port.upgradeOption)
		appendChoice("upg2", port.upgradeOption2)
		choicesByPort = append(choicesByPort, portChoices)
	}

	var (
		bestScenario upgradePathScenario
		hasBest      bool
		activeSteps  = make([]upgradePathSelection, len(portRows))
		activeKinds  = make([]string, len(portRows))
	)
	var visit func(
		index int,
		totalWatts float64,
		hasAny bool,
		stepCount int,
		complexity float64,
	)
	visit = func(
		index int,
		totalWatts float64,
		hasAny bool,
		stepCount int,
		complexity float64,
	) {
		if index >= len(portRows) {
			if !hasAny {
				return
			}
			minutes, hasMinutes := estimateSolarChargeMinutes(energyToTargetWh, totalWatts)
			if !hasMinutes {
				return
			}
			candidate := upgradePathScenario{
				name:       strings.Join(activeKinds, "+"),
				totalWatts: totalWatts,
				hasAny:     true,
				minutes:    minutes,
				hasMinutes: true,
				steps:      append([]upgradePathSelection(nil), activeSteps...),
				stepCount:  stepCount,
				complexity: complexity,
			}
			if !hasBest || shouldPreferUpgradePathScenario(candidate, bestScenario) {
				bestScenario = candidate
				hasBest = true
			}
			return
		}
		for _, choice := range choicesByPort[index] {
			activeSteps[index] = choice.selection
			activeKinds[index] = choice.name
			nextHasAny := hasAny || choice.hasOption
			nextStepCount := stepCount
			nextComplexity := complexity
			if choice.hasOption {
				nextStepCount++
				nextComplexity += choice.complexity
			}
			visit(index+1, totalWatts+choice.potentialETAW, nextHasAny, nextStepCount, nextComplexity)
		}
	}
	visit(0, 0, false, 0, 0)
	if !hasBest {
		return ""
	}
	best := bestScenario

	lines := make([]string, 0, 8)
	type panelDemand struct {
		label string
		units int
	}
	demandOrder := make([]string, 0, 4)
	demandMap := map[string]panelDemand{}
	for _, step := range best.steps {
		if !step.hasOption {
			continue
		}
		label := strings.TrimSpace(step.option.sourceLabel)
		if label == "" {
			continue
		}
		units := step.option.units
		if units <= 0 {
			units = 1
		}
		key := strings.ToLower(label)
		existing, exists := demandMap[key]
		if !exists {
			demandOrder = append(demandOrder, key)
			existing = panelDemand{label: label}
		}
		existing.units += units
		demandMap[key] = existing
	}
	for _, key := range demandOrder {
		item := demandMap[key]
		if item.units <= 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("* Get %dx %s", item.units, item.label))
	}

	for _, step := range best.steps {
		if !step.hasOption {
			continue
		}
		installLine := formatUpgradePathInstallLine(step)
		if installLine == "" {
			continue
		}
		lines = append(lines, "* "+installLine)
	}

	resultETA := "n/a"
	if best.hasMinutes {
		resultETA = formatETAMinutes(best.minutes)
	}
	resultDelta := "n/a"
	if hasBaseMinutes && best.hasMinutes {
		resultDelta = formatSignedMinutesDelta(best.minutes - baseMinutes)
	}
	lines = append(lines, fmt.Sprintf("* Results: Charge time %s (%s)", resultETA, resultDelta))
	return strings.Join(lines, "\n")
}

func shouldPreferUpgradePathScenario(candidate, current upgradePathScenario) bool {
	if !candidate.hasMinutes {
		return false
	}
	if !current.hasMinutes {
		return true
	}
	if math.Abs(candidate.minutes-current.minutes) > 0.5 {
		return candidate.minutes < current.minutes
	}
	if candidate.stepCount != current.stepCount {
		return candidate.stepCount < current.stepCount
	}
	if math.Abs(candidate.complexity-current.complexity) > 0.05 {
		return candidate.complexity < current.complexity
	}
	if math.Abs(candidate.totalWatts-current.totalWatts) > 1 {
		return candidate.totalWatts > current.totalWatts
	}
	return candidate.name < current.name
}

func sameUpgradePathOption(a, b solarRecommendationOption) bool {
	if !a.hasPotential || !b.hasPotential {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(a.sourceLabel), strings.TrimSpace(b.sourceLabel)) &&
		a.series == b.series &&
		a.parallel == b.parallel &&
		a.units == b.units {
		return true
	}
	return math.Abs(a.potentialW-b.potentialW) <= 0.5 &&
		math.Abs(a.nominalW-b.nominalW) <= 0.5 &&
		a.clipped == b.clipped
}

func formatUpgradePathInstallLine(step upgradePathSelection) string {
	panelLabel := strings.TrimSpace(step.option.sourceLabel)
	if panelLabel == "" {
		if strings.TrimSpace(step.option.text) == "" {
			return ""
		}
		return step.option.text
	}
	target := strings.TrimSpace(step.portLabel)
	if step.hasPortMaxW && step.portMaxWatts > 0 {
		target = fmt.Sprintf("solar [%s]", formatPVCapacityWatts(step.portMaxWatts))
	}
	if step.option.series > 0 && step.option.parallel > 0 && step.option.nominalW > 0 {
		details := fmt.Sprintf("%s, ~%s STC", formatSeriesParallel(step.option.series, step.option.parallel), formatPVCapacityWatts(step.option.nominalW))
		if step.option.clipped && step.hasPortMaxW && step.portMaxWatts > 0 {
			details += fmt.Sprintf(", clipped to %s", formatPVCapacityWatts(step.portMaxWatts))
		}
		return fmt.Sprintf("Install %s (%s) into %s", panelLabel, details, target)
	}
	return fmt.Sprintf("Install %s into %s", panelLabel, target)
}

func formatSignedMinutesDelta(deltaMinutes float64) string {
	if math.IsNaN(deltaMinutes) || math.IsInf(deltaMinutes, 0) {
		return "n/a"
	}
	rounded := int64(math.Round(deltaMinutes))
	if rounded == 0 {
		return "≈0"
	}
	sign := "+"
	if rounded < 0 {
		sign = "-"
		rounded = -rounded
	}
	if rounded < 1 {
		rounded = 1
	}
	return sign + formatMinutesHumanETA(rounded)
}

func formatMinutesDelta(baseMinutes float64, hasBase bool, candidateMinutes float64, hasCandidate bool) string {
	if !hasBase || !hasCandidate {
		return "n/a"
	}
	delta := int64(math.Round(candidateMinutes - baseMinutes))
	if delta == 0 {
		return "≈0"
	}
	sign := "+"
	if delta < 0 {
		sign = "-"
		delta = -delta
	}
	if delta < 1 {
		delta = 1
	}
	return sign + formatMinutesHumanETA(delta)
}

func estimateSolarChargeMinutes(energyWh float64, solarWatts float64) (float64, bool) {
	const solarChargeEfficiency = 0.85
	if energyWh <= 0 || solarWatts <= solarPowerEstimateMinWatts {
		return 0, false
	}
	effectiveWatts := solarWatts * solarChargeEfficiency
	if effectiveWatts <= solarPowerEstimateMinWatts {
		return 0, false
	}
	return (energyWh / effectiveWatts) * 60.0, true
}

func estimatedEnergyToChargeTargetWh(snapshot *energySnapshot) (float64, bool) {
	if snapshot == nil {
		return 0, false
	}
	totalCapacityWh, hasTotalCapacity := snapshot.estimatedTotalCapacityWh()
	remainingWh, hasRemainingWh := snapshot.estimatedRemainingEnergyWh()
	if !hasTotalCapacity || !hasRemainingWh || totalCapacityWh <= 0 {
		return 0, false
	}
	targetSOC := 100.0
	if snapshot.HasMaxChargeSOC && snapshot.MaxChargeSOC > 0 {
		targetSOC = clampPercent(snapshot.MaxChargeSOC)
	}
	targetWh := totalCapacityWh * (targetSOC / 100.0)
	if targetWh <= remainingWh {
		return 0, true
	}
	return targetWh - remainingWh, true
}

func parsePanelSetupCountAndWatts(setup string) (count int, perPanelW float64, ok bool) {
	setup = strings.TrimSpace(setup)
	if setup == "" {
		return 0, 0, false
	}
	if matches := panelSetupCountPattern.FindStringSubmatch(setup); len(matches) == 3 {
		parsedCount, errCount := strconv.Atoi(strings.TrimSpace(matches[1]))
		parsedWatts, errWatts := strconv.ParseFloat(strings.TrimSpace(matches[2]), 64)
		if errCount == nil && errWatts == nil && parsedCount > 0 && parsedWatts > 0 {
			return parsedCount, parsedWatts, true
		}
	}
	if matches := panelSetupWattPattern.FindStringSubmatch(setup); len(matches) >= 2 {
		parsedWatts, errWatts := strconv.ParseFloat(strings.TrimSpace(matches[1]), 64)
		if errWatts == nil && parsedWatts > 0 {
			return 1, parsedWatts, true
		}
	}
	return 0, 0, false
}

func panelTextIsBifacial(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	return strings.Contains(value, "bifacial") || strings.Contains(value, "bi-facial")
}

func portDetectedBifacial(detected detectedPanelSetup) bool {
	if detected.bifacial {
		return true
	}
	return panelTextIsBifacial(detected.setup)
}

func applyBifacialETAAdjustment(watts float64, bifacial bool) float64 {
	if watts <= 0 {
		return watts
	}
	if !bifacial {
		return watts
	}
	return watts * (1.0 + bifacialETAConservativeGain)
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
	// Source is fresher than ETA/state text during transitional windows.
	switch source {
	case "solar":
		return "🌞"
	case "hybrid(ac+solar)":
		return "🔆"
	case "ac":
		return "🌩"
	}
	if displayState == systemStateCharging {
		return "🌩"
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
	if !hasACIn {
		if smoothedACIn, ok := snapshot.smoothedACInput(); ok && smoothedACIn > sourceMinWatts {
			acInWatts = math.Abs(smoothedACIn)
			hasACIn = true
		}
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
	if !hasPVIn {
		if smoothedPVTotal, ok, _, _, _, _ := snapshot.smoothedPVChannels(); ok && smoothedPVTotal > sourceMinWatts {
			pvInWatts = math.Abs(smoothedPVTotal)
			hasPVIn = true
		}
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

	batteryChargeWatts := 0.0
	hasBatteryCharge := false
	if snapshot.HasBatteryIn && snapshot.BatteryInWatts > sourceMinWatts {
		batteryChargeWatts = math.Abs(snapshot.BatteryInWatts)
		hasBatteryCharge = true
	}

	netChargeWatts := 0.0
	hasNetCharge := false
	if derived.HasEffectiveIn && derived.HasEffectiveOut {
		netChargeWatts = derived.EffectiveIn - derived.EffectiveOut
		if netChargeWatts > sourceMinWatts {
			hasNetCharge = true
		}
	}

	// If both AC and PV are present while charging, AC can still contribute even when
	// AC in/out appears as passthrough. Use charging-flow evidence to classify hybrid.
	if state == systemStateCharging && hasACIn && acInWatts > sourceMinWatts && hasPVIn && !acActive {
		switch {
		case hasBatteryCharge && batteryChargeWatts > pvInWatts+sourceMinWatts:
			acActive = true
		case hasNetCharge && netChargeWatts > pvInWatts+sourceMinWatts:
			acActive = true
		}
	}

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
	_ = generic
	newReady := isMLEstimateReady(new)
	newTier := confidenceTierFromValue(new.ConfidenceValue)
	newConf, _ := parseConfidenceScore(new.ConfidenceValue)

	const modelSelectConfidenceFloor = 0.70
	switch {
	case newReady && newTier == "high":
		return new, "New"
	case newReady && newConf >= modelSelectConfidenceFloor:
		return new, "New"
	}

	if newReady {
		return new, "New"
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
		if isMQTTStale(snapshot) {
			return "MQTT stale (no data)"
		}
		return "MQTT live"
	}
	if snapshot.MQTTFallbackActive {
		return "MQTT reconnecting + REST fallback"
	}
	return "MQTT reconnecting"
}

func isMQTTStale(snapshot *energySnapshot) bool {
	if snapshot == nil || !snapshot.MQTTConnected || !snapshot.HasMQTTLastMessage || snapshot.MQTTLastMessageAt.IsZero() {
		return false
	}
	threshold := snapshot.MQTTStaleAfter
	if threshold <= 0 {
		threshold = defaultMQTTStaleAfter
	}
	return time.Since(snapshot.MQTTLastMessageAt) >= threshold
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
