package rollupworker

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/currenttelemetry"
	"github.com/tidwall/gjson"
)

const andersonPowerNoiseFloorWatts = 0.5
const solarPowerEstimateMinWatts = 0.5
const powerBalanceStateFloorWatts = 20
const batteryVoltageMaxCanonical = 1000
const batteryCurrentMaxCanonical = 200

var externalOutputLoadFields = []string{
	"params.outAcL11Pwr",
	"params.outAcL12Pwr",
	"params.outAcL14Pwr",
	"params.outAcL21Pwr",
	"params.outAcL22Pwr",
	"params.outAcTtPwr",
	"params.outAc5p8Pwr",
	"params.outUsb1Pwr",
	"params.outUsb2Pwr",
	"params.outTypec1Pwr",
	"params.outTypec2Pwr",
	"params.outPrPwr",
	"params.outAdsPwr",
	"params.invOutWatts",
	"params.carWatts",
	"params.wireWatts",
	"params.usb1Watts",
	"params.usb2Watts",
	"params.qcUsb1Watts",
	"params.qcUsb2Watts",
	"params.typec1Watts",
	"params.typec2Watts",
}

var extraBatteryTransferFields = []string{
	"params.XT150Watts1",
	"params.XT150Watts2",
	"param.XT150Watts1",
	"param.XT150Watts2",
}

func SampleFromEnvelope(env *envelopev1.TelemetryEnvelope) (*RollupSample, error) {
	if env == nil {
		return nil, ErrInvalidRollupEnvelope
	}

	provider := strings.TrimSpace(env.GetLabels()["provider"])
	providerDeviceID := strings.ToUpper(strings.TrimSpace(env.GetEcoflowSn()))
	deviceID := strings.TrimSpace(env.GetDeviceId())
	if provider == "" || providerDeviceID == "" || deviceID == "" {
		return nil, ErrInvalidRollupEnvelope
	}
	if _, err := uuid.Parse(deviceID); err != nil {
		return nil, ErrInvalidRollupEnvelope
	}

	eventUnixMs := firstPositiveInt64(env.GetObservedTimeUnixMs(), env.GetDeviceTimeUnixMs(), env.GetIngestedTimeUnixMs())
	if eventUnixMs <= 0 {
		return nil, ErrInvalidRollupEnvelope
	}
	ingestedUnixMs := env.GetIngestedTimeUnixMs()
	if ingestedUnixMs <= 0 {
		ingestedUnixMs = eventUnixMs
	}

	payload := strings.TrimSpace(string(env.GetPayload()))
	if payload == "" || !gjson.Valid(payload) {
		return nil, ErrInvalidRollupEnvelope
	}
	root := gjson.Parse(payload)
	metrics := extractMetrics(root)
	pvPorts := extractPVPortObservations(root)
	if currenttelemetry.IdleStale(currenttelemetry.ExtractNumericMetrics(env.GetPayload())) {
		suppressCurrentMetrics(&metrics)
		pvPorts = nil
	}
	if !metrics.HasAny() {
		return nil, ErrNoRollupMetrics
	}

	return &RollupSample{
		Provider:         provider,
		ProviderDeviceID: providerDeviceID,
		DeviceID:         deviceID,
		EventTime:        time.UnixMilli(eventUnixMs).UTC(),
		EventUnixMs:      eventUnixMs,
		IngestedUnixMs:   ingestedUnixMs,
		Metrics:          metrics,
		PVPorts:          pvPorts,
	}, nil
}

func extractMetrics(root gjson.Result) RollupMetrics {
	var metrics RollupMetrics

	metrics.SOC = firstNumber(root,
		"params.f32ShowSoc",
		"params.f32LcdShowSoc",
		"params.f32Soc",
		"param.cmsBattSoc",
		"params.cmsBattSoc",
		"params.soc",
		"param.soc",
	)

	if pv, ok := derivePV(root); ok {
		metrics.PV = optionalFloat{Value: pv, Valid: true}
	}

	if acIn, ok := sumIfPresent(root, "params.inAcC20Pwr", "params.inAc5p8Pwr"); ok {
		metrics.ACIn = optionalFloat{Value: acIn, Valid: true}
	} else if invIn := firstNumber(root, "params.invInWatts"); invIn.Valid {
		metrics.ACIn = invIn
	} else if wattsIn := firstNumber(root, "params.wattsInSum", "param.wattsInSum"); wattsIn.Valid {
		acIn := wattsIn.Value
		if metrics.PV.Valid {
			acIn -= metrics.PV.Value
		}
		if acIn < 0 {
			acIn = 0
		}
		metrics.ACIn = optionalFloat{Value: acIn, Valid: true}
	}

	if dc := firstNumber(root, "dcW"); dc.Valid {
		metrics.DC = dc
	} else if dc, ok := deriveDC(root); ok {
		metrics.DC = optionalFloat{Value: dc, Valid: true}
	}

	if load := deriveLoad(root); load.Valid {
		metrics.Load = load
	}

	if acOutput, ok := deriveACOutput(metrics.Load, metrics.DC); ok {
		metrics.ACOutput = optionalFloat{Value: acOutput, Valid: true}
	}

	if metrics.Load.Valid && (metrics.ACIn.Valid || metrics.PV.Valid) {
		metrics.Net = optionalFloat{Value: metrics.ACIn.Value + metrics.PV.Value - metrics.Load.Value, Valid: true}
	}

	if generated := firstNumber(root,
		"params.solarGeneratedWh",
		"param.solarGeneratedWh",
	); generated.Valid && generated.Value >= 0 {
		metrics.SolarGeneratedWh = generated
	}

	if battery, ok := batteryMetric(root, metrics.Net); ok {
		metrics.Battery = optionalFloat{Value: battery, Valid: true}
	} else if metrics.Net.Valid {
		metrics.Battery = metrics.Net
	}

	if temps := collectTemperatures(root); len(temps) > 0 {
		metrics.Temp = optionalFloat{Value: median(temps), Valid: true}
	}

	return metrics
}

func suppressCurrentMetrics(metrics *RollupMetrics) {
	if metrics == nil {
		return
	}
	zero := optionalFloat{Value: 0, Valid: true}
	metrics.ACIn = zero
	metrics.ACOutput = zero
	metrics.PV = zero
	metrics.DC = zero
	metrics.Load = zero
	metrics.Net = zero
	metrics.Battery = zero
	metrics.SolarGeneratedWh = optionalFloat{}
}

func derivePV(root gjson.Result) (float64, bool) {
	if hasAnyPVPortKey(root, "params.inLvMpptPwr", "param.powGetPvL", "params.inHvMpptPwr", "param.powGetPvH", "params.inLvMpptVol", "params.inLvMpptAmp", "params.inHvMpptVol", "params.inHvMpptAmp") {
		low, hasLow := derivePVChannel(root,
			[]string{"params.inLvMpptPwr", "param.powGetPvL"},
			[]string{"params.pv1ChargeWatts", "params.pv1InWatts"},
			[]string{"params.inVol", "params.inLvMpptVol"},
			[]string{"params.inAmp", "params.inLvMpptAmp"},
			[]string{"params.chgState"},
		)
		high, hasHigh := derivePVChannel(root,
			[]string{"params.inHvMpptPwr", "param.powGetPvH"},
			[]string{"params.pv2ChargeWatts", "params.pv2InWatts"},
			[]string{"params.pv2InVol", "params.inHvMpptVol"},
			[]string{"params.pv2InAmp", "params.inHvMpptAmp"},
			[]string{"params.pv2ChgState"},
		)
		if hasLow || hasHigh {
			return low + high, true
		}
	}
	if portSum, ok := sumPVPortObservationWatts(root); ok {
		return portSum, true
	}
	if pv := firstNumberCapped(root, 10_000, "pvW"); pv.Valid {
		return pv.Value, true
	}
	return 0, false
}

func derivePVChannel(root gjson.Result, primaryKeys, fallbackKeys, voltsKeys, ampsKeys, idleStateKeys []string) (float64, bool) {
	primaryWatts, hasPrimary := maxPresentCapped(root, 10_000, primaryKeys...)
	if hasPrimary && primaryWatts > solarPowerEstimateMinWatts {
		return primaryWatts, true
	}
	fallbackWatts, hasFallback := maxPresentCapped(root, 10_000, fallbackKeys...)
	if hasFallback && fallbackWatts > solarPowerEstimateMinWatts {
		return fallbackWatts, true
	}

	idle := false
	for _, key := range idleStateKeys {
		if state := firstNumber(root, key); state.Valid && int64(state.Value) <= 0 {
			idle = true
			break
		}
	}
	volts := firstPresentNumber(root, voltsKeys...)
	amps := firstPresentNumber(root, ampsKeys...)
	if !volts.Valid || !amps.Valid {
		if hasPrimary || hasFallback || idle {
			return 0, true
		}
		return 0, false
	}
	power := math.Abs(normalizeMPPTVoltageVolts(volts.Value) * normalizeMPPTCurrentAmps(amps.Value))
	if power > 10_000 {
		if hasPrimary || hasFallback || idle {
			return 0, true
		}
		return 0, false
	}
	if idle {
		return 0, true
	}
	if power <= solarPowerEstimateMinWatts {
		return 0, true
	}
	return power, true
}

func extractPVPortObservations(root gjson.Result) []PVPortObservation {
	specs := buildPVPortSpecs(root)
	observations := make([]PVPortObservation, 0, len(specs))
	for _, spec := range specs {
		if observation, ok := derivePVPortObservation(root, spec.PortID, spec.PortLabel, spec.VoltsKeys, spec.AmpsKeys, spec.WattsKeys, spec.IdleStateKeys); ok {
			observations = append(observations, observation)
		}
	}
	return observations
}

type pvPortSpec struct {
	PortID        string
	PortLabel     string
	VoltsKeys     []string
	AmpsKeys      []string
	WattsKeys     []string
	IdleStateKeys []string
}

var numberedPVPortPattern = regexp.MustCompile(`^pv(\d+)(InVol|InAmp|ChargeWatts|InWatts|ChgState)$`)

func buildPVPortSpecs(root gjson.Result) []pvPortSpec {
	if hasAnyPVPortKey(root, "params.inLvMpptVol", "params.inLvMpptAmp", "params.inLvMpptPwr", "params.inHvMpptVol", "params.inHvMpptAmp", "params.inHvMpptPwr") {
		return []pvPortSpec{
			{
				PortID:        "pv-low",
				PortLabel:     "PV Low",
				VoltsKeys:     []string{"params.inLvMpptVol", "params.inVol"},
				AmpsKeys:      []string{"params.inLvMpptAmp", "params.inAmp"},
				WattsKeys:     []string{"params.pv1ChargeWatts", "params.inLvMpptPwr", "params.pv1InWatts"},
				IdleStateKeys: []string{"params.chgState"},
			},
			{
				PortID:        "pv-high",
				PortLabel:     "PV High",
				VoltsKeys:     []string{"params.inHvMpptVol", "params.pv2InVol"},
				AmpsKeys:      []string{"params.inHvMpptAmp", "params.pv2InAmp"},
				WattsKeys:     []string{"params.pv2ChargeWatts", "params.inHvMpptPwr", "params.pv2InWatts"},
				IdleStateKeys: []string{"params.pv2ChgState"},
			},
		}
	}

	params := root.Get("params").Map()
	numberedPorts := map[int]struct{}{1: {}}
	for key := range params {
		matches := numberedPVPortPattern.FindStringSubmatch(strings.TrimSpace(key))
		if len(matches) != 3 {
			continue
		}
		index, err := strconv.Atoi(matches[1])
		if err != nil || index <= 0 {
			continue
		}
		numberedPorts[index] = struct{}{}
	}
	indexes := make([]int, 0, len(numberedPorts))
	for index := range numberedPorts {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	specs := make([]pvPortSpec, 0, len(indexes))
	for _, index := range indexes {
		spec := pvPortSpec{
			PortID:    "pv-" + strconv.Itoa(index),
			PortLabel: "PV " + strconv.Itoa(index),
		}
		if index == 1 {
			spec.VoltsKeys = []string{"params.inVol", "params.pv1InVol"}
			spec.AmpsKeys = []string{"params.inAmp", "params.pv1InAmp"}
			spec.WattsKeys = []string{"params.pv1ChargeWatts", "params.pv1InWatts"}
			spec.IdleStateKeys = []string{"params.chgState", "params.pv1ChgState"}
		} else {
			prefix := "params.pv" + strconv.Itoa(index)
			spec.VoltsKeys = []string{prefix + "InVol"}
			spec.AmpsKeys = []string{prefix + "InAmp"}
			spec.WattsKeys = []string{prefix + "ChargeWatts", prefix + "InWatts"}
			spec.IdleStateKeys = []string{prefix + "ChgState"}
		}
		specs = append(specs, spec)
	}
	return specs
}

func hasAnyPVPortKey(root gjson.Result, paths ...string) bool {
	for _, path := range paths {
		if value := root.Get(path); value.Exists() {
			return true
		}
	}
	return false
}

func sumPVPortObservationWatts(root gjson.Result) (float64, bool) {
	observations := extractPVPortObservations(root)
	if len(observations) == 0 {
		return 0, false
	}
	sum := 0.0
	for _, observation := range observations {
		sum += observation.Watts
	}
	return sum, true
}

func derivePVPortObservation(root gjson.Result, portID, portLabel string, voltsKeys, ampsKeys, wattsKeys, idleStateKeys []string) (PVPortObservation, bool) {
	volts := firstPresentNumber(root, voltsKeys...)
	amps := firstPresentNumber(root, ampsKeys...)
	watts := firstPresentNumber(root, wattsKeys...)
	idle := false
	for _, key := range idleStateKeys {
		if state := firstNumber(root, key); state.Valid && int64(state.Value) <= 0 {
			idle = true
			break
		}
	}
	normalizedVolts := 0.0
	hasVolts := false
	if volts.Valid {
		normalizedVolts = clampNonNegative(normalizeMPPTVoltageVolts(volts.Value))
		hasVolts = true
	}
	normalizedAmps := 0.0
	hasAmps := false
	if amps.Valid {
		normalizedAmps = clampNonNegative(normalizeMPPTCurrentAmps(amps.Value))
		hasAmps = true
	}
	normalizedWatts := 0.0
	hasWatts := false
	if watts.Valid && watts.Value >= 0 && watts.Value <= 10_000 {
		normalizedWatts = watts.Value
		hasWatts = true
	}
	if (!hasWatts || normalizedWatts <= 0) && hasVolts && hasAmps && !idle {
		normalizedWatts = normalizedVolts * normalizedAmps
		hasWatts = true
	}
	if idle {
		normalizedWatts = 0
		hasWatts = true
	}
	if !hasVolts && !hasAmps && !hasWatts {
		return PVPortObservation{}, false
	}
	return PVPortObservation{
		PortID:    portID,
		PortLabel: portLabel,
		Volts:     normalizedVolts,
		Amps:      normalizedAmps,
		Watts:     clampNonNegative(normalizedWatts),
	}, true
}

func maxPresentCapped(root gjson.Result, maxAbs float64, paths ...string) (float64, bool) {
	found := false
	maxValue := 0.0
	for _, path := range paths {
		value := firstNumber(root, path)
		if !value.Valid {
			continue
		}
		if value.Value < -maxAbs || value.Value > maxAbs {
			continue
		}
		normalized := math.Max(0, value.Value)
		if !found || normalized > maxValue {
			maxValue = normalized
		}
		found = true
	}
	return maxValue, found
}

func firstPresentNumber(root gjson.Result, paths ...string) optionalFloat {
	for _, path := range paths {
		value := firstNumber(root, path)
		if value.Valid {
			return value
		}
	}
	return optionalFloat{}
}

func normalizeMPPTVoltageVolts(value float64) float64 {
	abs := math.Abs(value)
	if abs >= 100 && value == math.Trunc(value) {
		return value / 1000.0
	}
	if abs > 1000 {
		return value / 1000.0
	}
	return value
}

func normalizeMPPTCurrentAmps(value float64) float64 {
	abs := math.Abs(value)
	if abs >= 1 && value == math.Trunc(value) {
		return value / 1000.0
	}
	if abs > 200 {
		return value / 1000.0
	}
	return value
}

func clampNonNegative(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	return value
}

func deriveDC(root gjson.Result) (float64, bool) {
	base, found := sumIfPresent(root,
		"params.carWatts",
		"params.wireWatts",
		"params.usb1Watts",
		"params.usb2Watts",
		"params.qcUsb1Watts",
		"params.qcUsb2Watts",
		"params.typec1Watts",
		"params.typec2Watts",
		"params.outUsb1Pwr",
		"params.outUsb2Pwr",
		"params.outTypec1Pwr",
		"params.outTypec2Pwr",
		"params.outPrPwr",
	)
	if anderson, ok := deriveAndersonPower(root); ok {
		base += anderson
		found = true
	}
	return base, found
}

func deriveLoad(root gjson.Result) optionalFloat {
	if explicit := firstNumber(root, "loadW"); explicit.Valid {
		return explicit
	}

	aggregate := firstNumber(root, "params.wattsOutSum", "param.wattsOutSum")
	explicitOutputs, hasExplicitOutputs := sumNonNegativeIfPresent(root, externalOutputLoadFields...)
	extraBatteryCharge := extraBatteryChargeTransfer(root)
	if aggregate.Valid {
		if extraBatteryCharge <= 0 {
			return aggregate
		}
		adjustedAggregate := math.Max(0, aggregate.Value-extraBatteryCharge)
		return optionalFloat{Value: math.Max(explicitOutputs, adjustedAggregate), Valid: true}
	}
	if hasExplicitOutputs {
		return optionalFloat{Value: explicitOutputs, Valid: true}
	}
	return optionalFloat{}
}

func deriveAndersonPower(root gjson.Result) (float64, bool) {
	explicit := firstNumber(root, "params.outAdsPwr")
	amp := firstNumber(root, "params.outAdsAmp")
	vol := firstNumber(root, "params.outAdsVol")
	if amp.Valid && vol.Valid {
		watts := amp.Value * vol.Value
		if watts < 0 {
			watts = 0
		}
		if watts > andersonPowerNoiseFloorWatts || !explicit.Valid || explicit.Value <= andersonPowerNoiseFloorWatts {
			return watts, true
		}
	}
	if explicit.Valid {
		return explicit.Value, true
	}
	return 0, false
}

func firstNumberCapped(root gjson.Result, maxAbs float64, paths ...string) optionalFloat {
	for _, path := range paths {
		result := root.Get(path)
		if !result.Exists() {
			continue
		}
		if !isNumericResult(result) {
			continue
		}
		value := result.Float()
		if value < -maxAbs || value > maxAbs {
			continue
		}
		return optionalFloat{Value: value, Valid: true}
	}
	return optionalFloat{}
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNumber(root gjson.Result, paths ...string) optionalFloat {
	for _, path := range paths {
		result := root.Get(path)
		if !result.Exists() {
			continue
		}
		if !isNumericResult(result) {
			continue
		}
		return optionalFloat{Value: result.Float(), Valid: true}
	}
	return optionalFloat{}
}

func sumIfPresent(root gjson.Result, paths ...string) (float64, bool) {
	var sum float64
	var found bool
	for _, path := range paths {
		result := root.Get(path)
		if !result.Exists() || !isNumericResult(result) {
			continue
		}
		sum += result.Float()
		found = true
	}
	return sum, found
}

func sumNonNegativeIfPresent(root gjson.Result, paths ...string) (float64, bool) {
	var sum float64
	var found bool
	for _, path := range paths {
		result := root.Get(path)
		if !result.Exists() || !isNumericResult(result) {
			continue
		}
		sum += math.Max(0, result.Float())
		found = true
	}
	return sum, found
}

func sumPositiveIfPresent(root gjson.Result, paths ...string) (float64, bool) {
	var sum float64
	var found bool
	for _, path := range paths {
		value := firstNumber(root, path)
		if !value.Valid || value.Value <= 0 {
			continue
		}
		sum += value.Value
		found = true
	}
	return sum, found
}

func deriveACOutput(load, dc optionalFloat) (float64, bool) {
	if !load.Valid || load.Value <= 0 {
		return 0, false
	}
	if dc.Valid && dc.Value > 0 {
		return math.Max(load.Value-dc.Value, 0), true
	}
	return load.Value, true
}

func batteryMetric(root gjson.Result, powerBalance optionalFloat) (float64, bool) {
	if explicit := firstNumber(root, "batteryW"); explicit.Valid {
		return reconcileBatteryPower(explicit.Value, powerBalance), true
	}
	bmsInput := firstNumber(root, "params.bmsInputWatts")
	bmsOutput := firstNumber(root, "params.bmsOutputWatts")
	extraBatteryCharge := extraBatteryChargeTransfer(root)
	if extraBatteryCharge > 0 && !hasNonZero(bmsInput) && !hasNonZero(bmsOutput) {
		output := firstNumber(root, "params.outputWatts", "param.outputWatts")
		outputValue := 0.0
		if output.Valid && output.Value > 0 {
			outputValue = output.Value
		}
		return extraBatteryCharge - outputValue, true
	}
	if bmsInput.Valid || bmsOutput.Valid {
		return reconcileBatteryPower(bmsInput.Value-bmsOutput.Value, powerBalance), true
	}
	input := firstNumber(root, "params.inputWatts", "param.inputWatts")
	output := firstNumber(root, "params.outputWatts", "param.outputWatts")
	if input.Valid || output.Valid {
		return reconcileBatteryPower(input.Value-output.Value, powerBalance), true
	}
	batAmp := firstNumber(root, "params.batAmp")
	batVol := firstNumber(root, "params.batVol")
	if batAmp.Valid && batVol.Valid {
		battery := normalizePotentialMilliUnit(batAmp.Value, batteryCurrentMaxCanonical) *
			normalizePotentialMilliUnit(batVol.Value, batteryVoltageMaxCanonical)
		return reconcileBatteryPower(battery, powerBalance), true
	}
	return 0, false
}

func reconcileBatteryPower(candidate float64, powerBalance optionalFloat) float64 {
	if !powerBalance.Valid || math.Abs(powerBalance.Value) <= powerBalanceStateFloorWatts {
		return candidate
	}
	if math.Abs(candidate) <= powerBalanceStateFloorWatts || sign(candidate) != sign(powerBalance.Value) {
		return powerBalance.Value
	}
	return candidate
}

func sign(value float64) int {
	switch {
	case value > 0:
		return 1
	case value < 0:
		return -1
	default:
		return 0
	}
}

func extraBatteryChargeTransfer(root gjson.Result) float64 {
	xt150Charge, hasXT150Charge := sumPositiveIfPresent(root, extraBatteryTransferFields...)
	kitCharge := sumExtraBatteryPackCharge(root)
	if !hasXT150Charge && kitCharge <= 0 {
		return 0
	}

	input := firstNumber(root, "params.inputWatts", "param.inputWatts")
	output := firstNumber(root, "params.outputWatts", "param.outputWatts")
	inputCharge := 0.0
	if input.Valid && input.Value > 0 && (!output.Valid || output.Value <= 0) {
		inputCharge = input.Value
	}
	return math.Max(math.Max(xt150Charge, kitCharge), inputCharge)
}

func sumExtraBatteryPackCharge(root gjson.Result) float64 {
	var sum float64
	addRows := func(rows gjson.Result) {
		if !rows.Exists() {
			return
		}
		rows.ForEach(func(_, row gjson.Result) bool {
			power := row.Get("curPower")
			if !isNumericResult(power) || power.Float() <= 0 {
				return true
			}
			available := row.Get("avaFlag")
			if isNumericResult(available) && available.Float() <= 0 {
				return true
			}
			sum += power.Float()
			return true
		})
	}
	addRows(root.Get("params.watts"))
	addRows(root.Get("params.watts.values"))
	return sum
}

func hasNonZero(value optionalFloat) bool {
	return value.Valid && math.Abs(value.Value) > 0.5
}

func normalizePotentialMilliUnit(value float64, maxAbsCanonical float64) float64 {
	if math.Abs(value) > maxAbsCanonical && math.Abs(value/1000) <= maxAbsCanonical {
		return value / 1000
	}
	return value
}

func collectTemperatures(root gjson.Result) []float64 {
	paths := []string{
		"params.temp",
		"params.pdTemp",
		"params.outTemp",
		"params.mpptLvTemp",
		"params.mpptHvTemp",
		"params.pcsAcTemp",
		"params.pcsDcTemp",
		"params.carTemp",
		"params.dcInTemp",
		"params.typec1Temp",
		"params.typec2Temp",
	}
	values := make([]float64, 0, len(paths)+8)
	for _, path := range paths {
		if value := firstNumber(root, path); value.Valid && saneTemperature(value.Value) {
			values = append(values, value.Value)
		}
	}
	for _, path := range []string{"params.cellTemp", "param.cellTemp"} {
		result := root.Get(path)
		if !result.Exists() || !result.IsArray() {
			continue
		}
		result.ForEach(func(_, value gjson.Result) bool {
			if isNumericResult(value) && saneTemperature(value.Float()) {
				values = append(values, value.Float())
			}
			return true
		})
	}
	return values
}

func saneTemperature(value float64) bool {
	return value >= -80 && value <= 120
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func isNumericResult(result gjson.Result) bool {
	return result.Type == gjson.Number || result.Type == gjson.True || result.Type == gjson.False
}
