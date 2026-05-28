package edgecollector

import (
	"strings"
)

// NormalizeEcoFlowBLEMetrics maps BLE display decoder fields into the same
// normalized params keys used by cloud MQTT quota/status ingestion.
func NormalizeEcoFlowBLEMetrics(metrics map[string]any) map[string]any {
	out := make(map[string]any, len(metrics)*2)
	for key, value := range metrics {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		switch key {
		case "battery_soc_percent":
			setIfNumeric(out, "soc", value)
			setIfNumeric(out, "f32ShowSoc", value)
		case "input_power_w":
			setIfNumeric(out, "wattsInSum", value)
		case "output_power_w":
			setIfNumeric(out, "wattsOutSum", value)
		case "pv_input_power_w":
			setIfNumeric(out, "pv1ChargeWatts", value)
			setIfNumeric(out, "pv1InWatts", value)
		case "pv2_input_power_w":
			setIfNumeric(out, "pv2ChargeWatts", value)
			setIfNumeric(out, "pv2InWatts", value)
		case "battery_charge_remaining_min":
			setIfNumeric(out, "chgRemainTime", value)
		case "battery_discharge_remaining_min":
			setIfNumeric(out, "dsgRemainTime", value)
		case "ac_input_plugged":
			out["acInputPlugged"] = value
		case "ac_charger_connected":
			out["acChargerConnected"] = value
		case "ac_output_enabled":
			out["acOutEnable"] = value
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
