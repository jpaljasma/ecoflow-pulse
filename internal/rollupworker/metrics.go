package rollupworker

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/tidwall/gjson"
)

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

	if pv, ok := sumIfPresent(root,
		"params.inLvMpptPwr",
		"params.inHvMpptPwr",
		"param.powGetPvL",
		"param.powGetPvH",
	); ok {
		metrics.PV = optionalFloat{Value: pv, Valid: true}
	} else if pv, ok := sumIfPresent(root,
		"params.pv1ChargeWatts",
		"params.pv2ChargeWatts",
		"params.chgSunPower",
	); ok {
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

	if dc, ok := sumIfPresent(root,
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
		"params.outAdsPwr",
	); ok {
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

	if metrics.Load.Valid && (metrics.ACIn.Valid || metrics.PV.Valid) {
		metrics.Net = optionalFloat{Value: metrics.ACIn.Value + metrics.PV.Value - metrics.Load.Value, Valid: true}
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
