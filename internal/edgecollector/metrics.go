package edgecollector

import (
	"strings"
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
	out := make(map[string]any, len(metrics)*2)
	for key, value := range metrics {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		for _, target := range ecoFlowBLENumericTargets[key] {
			setIfNumeric(out, target, value)
		}
		if target := ecoFlowBLEDirectTargets[key]; target != "" {
			out[target] = value
		}
		if alias := ecoFlowBLEAliasKeys[key]; alias != "" {
			out[alias] = value
			continue
		}
		out["ble"+exportedCamelFromSnake(key)] = value
	}
	return out
}

func setIfNumeric(out map[string]any, key string, value any) {
	switch v := value.(type) {
	case float64:
		out[key] = v
	case float32:
		out[key] = float64(v)
	case int:
		out[key] = float64(v)
	case int64:
		out[key] = float64(v)
	case int32:
		out[key] = float64(v)
	case uint:
		out[key] = float64(v)
	case uint64:
		out[key] = float64(v)
	case uint32:
		out[key] = float64(v)
	}
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
