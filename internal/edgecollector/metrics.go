package edgecollector

import (
	"math"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/structpb"
)

const (
	maxEcoFlowBLEMetricKeyBytes    = 128
	maxEcoFlowBLEMetricStringBytes = 4096
)

var ecoFlowBLENumericTargets = map[string][]string{
	"battery_soc_percent":             {"soc", "f32ShowSoc"},
	"input_power_w":                   {"wattsInSum"},
	"output_power_w":                  {"wattsOutSum"},
	"pv_input_power_w":                {"pv1ChargeWatts", "pv1InWatts"},
	"pv2_input_power_w":               {"pv2ChargeWatts", "pv2InWatts"},
	"battery_charge_remaining_min":    {"chgRemainTime"},
	"battery_discharge_remaining_min": {"dsgRemainTime"},
}

var ecoFlowBLEDirectTargets = map[string]string{
	"ac_input_plugged":     "acInputPlugged",
	"ac_charger_connected": "acChargerConnected",
	"ac_output_enabled":    "acOutEnable",
}

var ecoFlowBLEAliasKeys = map[string]string{
	"ac_charger_connected":            "bleAcChargerConnected",
	"ac_input_plugged":                "bleAcInputPlugged",
	"ac_input_power_w":                "bleAcInputPowerW",
	"ac_output_enabled":               "bleAcOutputEnabled",
	"ac_output_power_w":               "bleAcOutputPowerW",
	"battery_charge_remaining_min":    "bleBatteryChargeRemainingMin",
	"battery_discharge_remaining_min": "bleBatteryDischargeRemainingMin",
	"battery_power_w":                 "bleBatteryPowerW",
	"battery_soc_percent":             "bleBatterySocPercent",
	"error_code":                      "bleErrorCode",
	"input_power_w":                   "bleInputPowerW",
	"main_battery_soc_percent":        "bleMainBatterySocPercent",
	"output_power_w":                  "bleOutputPowerW",
	"pv2_input_power_w":               "blePv2InputPowerW",
	"pv2_input_state":                 "blePv2InputState",
	"pv_input_power_w":                "blePvInputPowerW",
	"pv_input_state":                  "blePvInputState",
	"usb_a_1_output_power_w":          "bleUsbA1OutputPowerW",
	"usb_a_2_output_power_w":          "bleUsbA2OutputPowerW",
	"usb_c_1_output_power_w":          "bleUsbC1OutputPowerW",
	"usb_c_2_output_power_w":          "bleUsbC2OutputPowerW",
}

// NormalizeEcoFlowBLEMetrics maps BLE display decoder fields into the same
// normalized params keys used by cloud MQTT quota/status ingestion.
func NormalizeEcoFlowBLEMetrics(metrics map[string]any) map[string]any {
	if len(metrics) == 0 {
		return nil
	}
	out := make(map[string]any, len(metrics)*2)
	for key, value := range metrics {
		setNormalizedEcoFlowBLEMetric(out, key, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func NormalizeEcoFlowBLEMetricStruct(metrics *structpb.Struct) map[string]any {
	if metrics == nil || len(metrics.GetFields()) == 0 {
		return nil
	}
	out := make(map[string]any, len(metrics.GetFields())*2)
	for key, value := range metrics.GetFields() {
		setNormalizedEcoFlowBLEStructMetric(out, key, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func setNormalizedEcoFlowBLEStructMetric(out map[string]any, key string, value *structpb.Value) {
	key, ok := normalizeEcoFlowBLEMetricKey(key)
	if !ok || value == nil {
		return
	}
	if targets := ecoFlowBLENumericTargets[key]; len(targets) > 0 {
		if numberValue, ok := numericStructValue(value); ok {
			setNumericTargets(out, targets, key, numberValue)
		}
		return
	}
	if target := ecoFlowBLEDirectTargets[key]; target != "" {
		if boolValue, ok := boolStructValue(value); ok {
			setDirectBoolTarget(out, target, key, boolValue)
		}
		return
	}
	scalar, ok := scalarStructValue(value)
	if !ok {
		return
	}
	setAliasOrDefault(out, key, scalar)
}

func setNormalizedEcoFlowBLEMetric(out map[string]any, key string, value any) {
	key, ok := normalizeEcoFlowBLEMetricKey(key)
	if !ok {
		return
	}
	if targets := ecoFlowBLENumericTargets[key]; len(targets) > 0 {
		if numberValue, ok := numericMetricValue(value); ok {
			setNumericTargets(out, targets, key, numberValue)
		}
		return
	}
	if target := ecoFlowBLEDirectTargets[key]; target != "" {
		if boolValue, ok := boolMetricValue(value); ok {
			setDirectBoolTarget(out, target, key, boolValue)
		}
		return
	}
	scalar, ok := scalarMetricValue(value)
	if !ok {
		return
	}
	setAliasOrDefault(out, key, scalar)
}

func setNumericTargets(out map[string]any, targets []string, key string, value float64) {
	for _, target := range targets {
		out[target] = value
	}
	setAlias(out, key, value)
}

func setDirectBoolTarget(out map[string]any, target string, key string, value bool) {
	out[target] = value
	setAlias(out, key, value)
}

func setAliasOrDefault(out map[string]any, key string, value any) {
	if alias := ecoFlowBLEAliasKeys[key]; alias != "" {
		out[alias] = value
		return
	}
	out["ble"+exportedCamelFromSnake(key)] = value
}

func setAlias(out map[string]any, key string, value any) {
	if alias := ecoFlowBLEAliasKeys[key]; alias != "" {
		out[alias] = value
	}
}

func normalizeEcoFlowBLEMetricKey(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxEcoFlowBLEMetricKeyBytes || !utf8.ValidString(value) {
		return "", false
	}
	return value, true
}

func numericStructValue(value *structpb.Value) (float64, bool) {
	number, ok := value.GetKind().(*structpb.Value_NumberValue)
	if !ok || !isFinite(number.NumberValue) {
		return 0, false
	}
	return number.NumberValue, true
}

func boolStructValue(value *structpb.Value) (bool, bool) {
	boolValue, ok := value.GetKind().(*structpb.Value_BoolValue)
	if !ok {
		return false, false
	}
	return boolValue.BoolValue, true
}

func scalarStructValue(value *structpb.Value) (any, bool) {
	switch kind := value.GetKind().(type) {
	case *structpb.Value_NumberValue:
		if !isFinite(kind.NumberValue) {
			return nil, false
		}
		return kind.NumberValue, true
	case *structpb.Value_BoolValue:
		return kind.BoolValue, true
	case *structpb.Value_StringValue:
		if !isBoundedMetricString(kind.StringValue) {
			return nil, false
		}
		return kind.StringValue, true
	default:
		return nil, false
	}
}

func numericMetricValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		if !isFinite(v) {
			return 0, false
		}
		return v, true
	case float32:
		out := float64(v)
		if !isFinite(out) {
			return 0, false
		}
		return out, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	default:
		return 0, false
	}
}

func boolMetricValue(value any) (bool, bool) {
	out, ok := value.(bool)
	return out, ok
}

func scalarMetricValue(value any) (any, bool) {
	if number, ok := numericMetricValue(value); ok {
		return number, true
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		if !isBoundedMetricString(v) {
			return nil, false
		}
		return v, true
	default:
		return nil, false
	}
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func isBoundedMetricString(value string) bool {
	return len(value) <= maxEcoFlowBLEMetricStringBytes && utf8.ValidString(value)
}

func exportedCamelFromSnake(value string) string {
	value = strings.Trim(value, "_")
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "_")
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			b.WriteString(part[1:])
		}
	}
	return b.String()
}
