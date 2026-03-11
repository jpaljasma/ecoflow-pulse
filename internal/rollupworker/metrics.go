package rollupworker

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/tidwall/gjson"
)

const andersonPowerNoiseFloorWatts = 0.5
const solarPowerEstimateMinWatts = 0.5

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

	payload := strings.TrimSpace(string(env.GetPayload()))
	if payload == "" || !gjson.Valid(payload) {
		return nil, ErrInvalidRollupEnvelope
	}
	root := gjson.Parse(payload)
	metrics := extractMetrics(root)
	if !metrics.HasAny() {
		return nil, ErrNoRollupMetrics
	}

	return &RollupSample{
		Provider:         provider,
		ProviderDeviceID: providerDeviceID,
		DeviceID:         deviceID,
		EventTime:        time.UnixMilli(eventUnixMs).UTC(),
		EventUnixMs:      eventUnixMs,
		Metrics:          metrics,
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

	if load := firstNumber(root, "params.wattsOutSum", "param.wattsOutSum"); load.Valid {
		metrics.Load = load
	} else if load, ok := sumIfPresent(root,
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
	); ok {
		metrics.Load = optionalFloat{Value: load, Valid: true}
	} else if invOut := firstNumber(root, "params.invOutWatts"); invOut.Valid {
		metrics.Load = invOut
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

	if battery, ok := batteryMetric(root); ok {
		metrics.Battery = optionalFloat{Value: battery, Valid: true}
	} else if metrics.Net.Valid {
		metrics.Battery = metrics.Net
	}

	if temps := collectTemperatures(root); len(temps) > 0 {
		metrics.Temp = optionalFloat{Value: median(temps), Valid: true}
	}

	return metrics
}

func derivePV(root gjson.Result) (float64, bool) {
	low, hasLow := derivePVChannel(root,
		[]string{"params.inLvMpptPwr", "param.powGetPvL"},
		[]string{"params.pv1ChargeWatts", "params.inWatts"},
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

func deriveACOutput(load, dc optionalFloat) (float64, bool) {
	if !load.Valid || load.Value <= 0 {
		return 0, false
	}
	if dc.Valid && dc.Value > 0 {
		return math.Max(load.Value-dc.Value, 0), true
	}
	return load.Value, true
}

func batteryMetric(root gjson.Result) (float64, bool) {
	input := firstNumber(root, "params.bmsInputWatts", "params.inputWatts")
	output := firstNumber(root, "params.bmsOutputWatts", "params.outputWatts")
	if input.Valid || output.Valid {
		return input.Value - output.Value, true
	}
	batAmp := firstNumber(root, "params.batAmp")
	batVol := firstNumber(root, "params.batVol")
	if batAmp.Valid && batVol.Valid {
		return batAmp.Value * batVol.Value, true
	}
	return 0, false
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
