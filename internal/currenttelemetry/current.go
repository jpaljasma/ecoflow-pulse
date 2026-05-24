package currenttelemetry

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

func ExtractNumericMetrics(payload []byte) map[string]float64 {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return nil
	}
	root := gjson.ParseBytes(payload)
	out := make(map[string]float64, 32)
	walkMetricValue("", root, out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func IsCurrentMetricKey(key string) bool {
	clean := strings.ToLower(strings.TrimSpace(key))
	if clean == "" {
		return false
	}
	switch clean {
	case "pvw", "acw", "dcw", "loadw", "batteryw":
		return true
	}
	if strings.Contains(clean, "remaintime") {
		return true
	}
	if strings.Contains(clean, "watt") || strings.Contains(clean, "pwr") || strings.Contains(clean, "mppt") {
		return true
	}
	if strings.Contains(clean, ".pv") && (strings.Contains(clean, "invol") ||
		strings.Contains(clean, "inamp") ||
		strings.Contains(clean, "inwatt") ||
		strings.Contains(clean, "chargewatt") ||
		strings.Contains(clean, "chgstate")) {
		return true
	}
	if strings.HasSuffix(clean, ".invol") ||
		strings.HasSuffix(clean, ".inamp") ||
		strings.HasSuffix(clean, ".batvol") ||
		strings.HasSuffix(clean, ".batamp") {
		return true
	}
	return strings.Contains(clean, ".out") && (strings.Contains(clean, "vol") || strings.Contains(clean, "amp"))
}

func IdleStale(metrics map[string]float64) bool {
	inputW, hasInput := currentInputWatts(metrics)
	if !hasInput || inputW <= 5 {
		return false
	}
	loadW, hasLoad := firstMetric(metrics, "loadW", "params.wattsOutSum", "param.wattsOutSum", "params.invOutWatts")
	batterySinkW, hasBattery := currentBatterySinkWatts(metrics)
	if (hasLoad && math.Abs(loadW) > 1) || (hasBattery && batterySinkW > 1) {
		return false
	}
	if !hasLoad && !hasBattery {
		return false
	}
	return hasIdleOrPausedSignal(metrics) || hasSentinelRemainTime(metrics)
}

func walkMetricValue(path string, value gjson.Result, out map[string]float64) {
	switch value.Type {
	case gjson.Number:
		if path != "" {
			out[path] = value.Num
		}
	case gjson.True:
		if path != "" {
			out[path] = 1
		}
	case gjson.False:
		if path != "" {
			out[path] = 0
		}
	case gjson.JSON:
		if value.IsArray() {
			index := 0
			value.ForEach(func(_, child gjson.Result) bool {
				next := joinMetricPath(path, strconv.Itoa(index))
				walkMetricValue(next, child, out)
				index++
				return true
			})
			return
		}
		if value.IsObject() {
			value.ForEach(func(key, child gjson.Result) bool {
				next := joinMetricPath(path, key.String())
				walkMetricValue(next, child, out)
				return true
			})
		}
	}
}

func joinMetricPath(parent, child string) string {
	clean := sanitizeMetricPathSegment(child)
	if parent == "" {
		return clean
	}
	if clean == "" {
		return parent
	}
	return fmt.Sprintf("%s.%s", parent, clean)
}

func sanitizeMetricPathSegment(in string) string {
	clean := strings.TrimSpace(in)
	if clean == "" {
		return ""
	}
	clean = strings.ReplaceAll(clean, "{", "_")
	clean = strings.ReplaceAll(clean, "}", "_")
	clean = strings.ReplaceAll(clean, " ", "_")
	return clean
}

func currentInputWatts(metrics map[string]float64) (float64, bool) {
	if watts, ok := firstMetric(metrics, "pvW", "params.wattsInSum", "param.wattsInSum", "params.inWatts"); ok {
		return watts, true
	}
	return sumIfPresent(metrics,
		"params.inLvMpptPwr",
		"params.inHvMpptPwr",
		"params.pv1ChargeWatts",
		"params.pv2ChargeWatts",
		"params.pv1InWatts",
		"params.pv2InWatts",
	)
}

func currentBatterySinkWatts(metrics map[string]float64) (float64, bool) {
	if input, hasInput := firstMetric(metrics, "params.bmsInputWatts", "params.inputWatts"); hasInput {
		output, _ := firstMetric(metrics, "params.bmsOutputWatts", "params.outputWatts")
		return math.Abs(input) + math.Abs(output), true
	}
	if batteryW, ok := firstMetric(metrics, "batteryW"); ok {
		return math.Abs(batteryW), true
	}
	volts, hasVolts := firstMetric(metrics, "params.batVol")
	amps, hasAmps := firstMetric(metrics, "params.batAmp")
	if !hasVolts || !hasAmps {
		return 0, false
	}
	volts = normalizePotentialMilliUnit(volts, 1000)
	amps = normalizePotentialMilliUnit(amps, 200)
	return math.Abs(volts * amps), true
}

func hasIdleOrPausedSignal(metrics map[string]float64) bool {
	if value, ok := firstMetric(metrics, "params.chgPauseFlag"); ok && value >= 1 {
		return true
	}
	if value, ok := firstMetric(metrics, "params.chgDsgState"); ok && value == 2 {
		return true
	}
	if value, ok := firstMetric(metrics, "params.sysState"); ok && value == 2 {
		return true
	}
	return false
}

func hasSentinelRemainTime(metrics map[string]float64) bool {
	remain, hasRemain := firstMetric(metrics, "params.remainTime", "param.remainTime")
	charge, hasCharge := firstMetric(metrics, "params.chgRemainTime", "param.chgRemainTime")
	discharge, hasDischarge := firstMetric(metrics, "params.dsgRemainTime", "param.dsgRemainTime")
	if !hasRemain && !hasCharge && !hasDischarge {
		return false
	}
	return (!hasRemain || remain >= 5990) &&
		(!hasCharge || charge >= 5990) &&
		(!hasDischarge || discharge >= 5990)
}

func firstMetric(metrics map[string]float64, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := metrics[key]
		if ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
			return value, true
		}
	}
	return 0, false
}

func sumIfPresent(metrics map[string]float64, keys ...string) (float64, bool) {
	var total float64
	var found bool
	for _, key := range keys {
		value, ok := firstMetric(metrics, key)
		if !ok {
			continue
		}
		total += value
		found = true
	}
	return total, found
}

func normalizePotentialMilliUnit(value float64, maxAbsCanonical float64) float64 {
	if math.Abs(value) > maxAbsCanonical && math.Abs(value/1000) <= maxAbsCanonical {
		return value / 1000
	}
	return value
}
