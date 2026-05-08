package ankersolix

import (
	"fmt"
	"strings"
)

func MergeValues(base map[string]any, next map[string]any) map[string]any {
	out := cloneMap(base)
	for key, value := range next {
		if nested := asMap(value); len(nested) > 0 {
			out[key] = MergeValues(asMap(out[key]), nested)
			continue
		}
		out[key] = value
	}
	return out
}

func NormalizeTelemetry(ref DeviceRef, values map[string]any) NormalizedTelemetry {
	ref = ref.Normalize()
	capability, ok := LookupCapability(ref.ProductCode)
	if !ok {
		capability = ModelCapability{ProductCode: ref.ProductCode, Status: SupportNeedsSample}
	}
	params := map[string]any{}
	capabilities := map[string]any{}
	metadata := map[string]any{
		"provider":         ProviderID,
		"product_code":     ref.ProductCode,
		"device_sn_suffix": suffix(ref.DeviceSN, 6),
		"family":           string(capability.Family),
		"support_status":   string(capability.Status),
		"raw_fields":       cloneMap(values),
	}
	if capability.DisplayName != "" {
		metadata["model"] = capability.DisplayName
	}
	if capability.BatteryCapacityWh > 0 {
		capabilities["battery_capacity_wh"] = float64(capability.BatteryCapacityWh)
	}
	if capability.Family != "" {
		capabilities["family"] = string(capability.Family)
	}
	copyNumber(params, "soc", firstNumber(values, "battery_soc", "main_battery_soc", "battery_soc_total"))
	if params["soc"] != nil {
		params["f32ShowSoc"] = params["soc"]
	}
	copyNumber(params, "temp", firstNumber(values, "temperature"))
	copyNumber(params, "wattsInSum", totalInputPower(capability.Family, values))
	copyNumber(params, "wattsOutSum", totalOutputPower(values))
	copyNumber(params, "inAcC20Pwr", firstNumber(values, "ac_input_power_total", "ac_input_power"))
	copyNumber(params, "outAcTtPwr", firstNumber(values, "ac_output_power_total", "ac_output_power", "home_demand"))
	copyNumber(params, "outAdsPwr", firstNumber(values, "dc_output_power_total", "dc_output_power"))
	copyNumber(params, "gridPower", firstNumber(values, "grid_power_signed"))
	copyNumber(params, "homeLoadWatts", firstNumber(values, "home_demand", "grid_to_home_power"))
	copyNumber(params, "batteryPower", firstNumber(values, "battery_power_signed"))
	copyBool(params, "cfgAcEnabled", firstBool(values, "ac_output_power_switch"))
	copyBool(params, "dcOutState", firstBool(values, "dc_output_power_switch"))
	pvCount := addPVPorts(params, values)
	if pvCount == 0 && capability.DefaultPVInputs > 0 && hasPVSource(values) {
		pvCount = capability.DefaultPVInputs
	}
	if pvCount > 0 {
		capabilities["pv_input_count"] = float64(pvCount)
	}
	if count := batteryModuleCount(values["battery_modules"]); count > 0 {
		capabilities["battery_module_count"] = float64(count)
	}
	addPortMetadata(metadata, values)
	return NormalizedTelemetry{
		Params:       collapseEmptyMap(params),
		Capabilities: collapseEmptyMap(capabilities),
		Metadata:     collapseEmptyMap(metadata),
	}
}

func addPVPorts(params map[string]any, values map[string]any) int {
	count := 0
	for i := 1; i <= 8; i++ {
		key := fmt.Sprintf("pv_%d_power", i)
		value, ok := values[key]
		if !ok {
			continue
		}
		if _, ok := toFloat(value); !ok {
			continue
		}
		count++
		prefix := fmt.Sprintf("pv%d", count)
		copyNumber(params, prefix+"ChargeWatts", value)
		copyNumber(params, prefix+"InWatts", value)
		setPVState(params, prefix, value)
	}
	if count > 0 {
		return count
	}
	for _, key := range []string{"photovoltaic_power", "pv_power_total", "dc_input_power_total", "dc_input_power"} {
		value, ok := values[key]
		if !ok {
			continue
		}
		if _, ok := toFloat(value); !ok {
			continue
		}
		copyNumber(params, "pv1ChargeWatts", value)
		copyNumber(params, "pv1InWatts", value)
		setPVState(params, "pv1", value)
		return 1
	}
	return 0
}

func setPVState(params map[string]any, prefix string, value any) {
	if watts, ok := toFloat(value); ok && watts > 1 {
		params[prefix+"ChgState"] = float64(2)
	} else {
		params[prefix+"ChgState"] = float64(0)
	}
}

func totalInputPower(family ModelFamily, values map[string]any) any {
	if value := firstNumber(values, "input_power_total", "watts_in_sum"); value != nil {
		return value
	}
	if family == FamilyHES {
		if sum, ok := sumFirstNumbers(values,
			[]string{"photovoltaic_power", "pv_power_total"},
			[]string{"grid_power_signed"},
		); ok {
			return sum
		}
	}
	if sum, ok := sumFirstNumbers(values,
		[]string{"ac_input_power_total", "ac_input_power"},
		[]string{"dc_input_power_total", "dc_input_power"},
		[]string{"photovoltaic_power", "pv_power_total"},
		[]string{"pv_1_power"},
		[]string{"pv_2_power"},
		[]string{"pv_3_power"},
		[]string{"pv_4_power"},
	); ok {
		return sum
	}
	if value := firstNumber(values, "bat_charge_power", "grid_to_battery_power"); value != nil {
		return value
	}
	return nil
}

func totalOutputPower(values map[string]any) any {
	if value := firstNumber(values, "output_power_total", "ac_output_power_total", "dc_output_power_total", "home_demand", "grid_to_home_power", "ac_output_power"); value != nil {
		return value
	}
	if sum, ok := sumFirstNumbers(values,
		[]string{"ac_output_power"},
		[]string{"dc_output_power"},
	); ok {
		return sum
	}
	return nil
}

func firstNumber(root map[string]any, keys ...string) any {
	for _, key := range keys {
		value, ok := root[key]
		if !ok {
			continue
		}
		if _, ok := toFloat(value); ok {
			return value
		}
	}
	return nil
}

func firstBool(root map[string]any, keys ...string) any {
	for _, key := range keys {
		value, ok := root[key]
		if !ok {
			continue
		}
		if _, ok := toBool(value); ok {
			return value
		}
	}
	return nil
}

func sumFirstNumbers(root map[string]any, groups ...[]string) (float64, bool) {
	var sum float64
	observed := false
	for _, group := range groups {
		value := firstNumber(root, group...)
		if value == nil {
			continue
		}
		number, _ := toFloat(value)
		sum += number
		observed = true
	}
	return sum, observed
}

func copyNumber(dst map[string]any, key string, value any) {
	if number, ok := toFloat(value); ok {
		dst[key] = number
	}
}

func copyBool(dst map[string]any, key string, value any) {
	if b, ok := toBool(value); ok {
		if b {
			dst[key] = float64(1)
		} else {
			dst[key] = float64(0)
		}
	}
}

func batteryModuleCount(value any) int {
	switch v := value.(type) {
	case []any:
		return len(v)
	case []map[string]any:
		return len(v)
	default:
		return 0
	}
}

func hasPVSource(values map[string]any) bool {
	for key := range values {
		if strings.HasPrefix(key, "pv_") || key == "photovoltaic_power" || key == "pv_power_total" || strings.Contains(key, "dc_input_power") {
			return true
		}
	}
	return false
}

func addPortMetadata(metadata map[string]any, values map[string]any) {
	ports := map[string]any{}
	for key, value := range values {
		if strings.HasPrefix(key, "usbc_") || strings.HasPrefix(key, "usba_") {
			ports[key] = value
		}
	}
	if len(ports) > 0 {
		metadata["ports"] = ports
	}
}

func suffix(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}
