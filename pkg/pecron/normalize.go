package pecron

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type MQTTBusMessage struct {
	DeviceKey string
	KV        map[string]any
}

func DecodeMQTTBusPayload(payload []byte) (MQTTBusMessage, error) {
	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		return MQTTBusMessage{}, fmt.Errorf("decode pecron mqtt payload: %w", err)
	}
	data := asMap(root["data"])
	kv := asMap(data["kv"])
	if len(kv) == 0 {
		return MQTTBusMessage{DeviceKey: strings.TrimSpace(asString(root["deviceKey"]))}, nil
	}
	return MQTTBusMessage{
		DeviceKey: strings.TrimSpace(asString(root["deviceKey"])),
		KV:        kv,
	}, nil
}

func MergeKV(base map[string]any, next map[string]any) map[string]any {
	return mergeKVAtPath(base, next, "")
}

func mergeKVAtPath(base map[string]any, next map[string]any, parentPath string) map[string]any {
	out := cloneMap(base)
	for key, value := range next {
		path := key
		if parentPath != "" {
			path = parentPath + "." + key
		}
		if nested := asMap(value); len(nested) > 0 {
			existing := asMap(out[key])
			out[key] = mergeKVAtPath(existing, nested, path)
			continue
		}
		if preserveExistingValue(path, out[key], value) {
			continue
		}
		out[key] = value
	}
	return out
}

func preserveExistingValue(path string, existing any, next any) bool {
	switch path {
	case "host_packet_data_jdb.host_packet_voltage":
		oldValue, oldOK := toFloat(existing)
		newValue, newOK := toFloat(next)
		return oldOK && oldValue > 0 && newOK && newValue <= 0
	default:
		return false
	}
}

func NormalizeTelemetry(device Device, kv map[string]any) NormalizedTelemetry {
	ref := DeviceRefFromDevice(device)
	params := map[string]any{}
	capabilities := StaticCapabilities(device)
	metadata := map[string]any{
		"provider":          "pecron",
		"product_key":       ref.ProductKey,
		"device_key_suffix": suffix(ref.DeviceKey, 6),
		"field_names":       sortedFieldPaths(kv),
	}

	copyNumber(params, "soc", firstNumber(kv, "host_packet_data_jdb.host_packet_electric_percentage", "battery_percentage"))
	if params["soc"] != nil {
		params["f32ShowSoc"] = params["soc"]
	}
	copyNumber(params, "wattsInSum", totalInputPower(kv))
	copyNumber(params, "wattsOutSum", totalOutputPower(kv))
	copyNumber(params, "remainTime", firstNumber(kv, "remain_time"))
	copyNumber(params, "dsgRemainTime", firstNumber(kv, "remain_time"))
	copyNumber(params, "chgRemainTime", firstNumber(kv, "remain_charging_time"))
	copyNumber(params, "batVol", firstNumber(kv, "host_packet_data_jdb.host_packet_voltage"))
	copyNumber(params, "batAmp", firstNumber(kv, "host_packet_data_jdb.host_packet_current"))
	copyNumber(params, "temp", firstNumber(kv, "host_packet_data_jdb.host_packet_temp", "battery_temp"))
	copyNumber(params, "inAcC20Pwr", firstNumber(kv, "ac_data_input_hm.ac_input_power", "ac_data_input_hm.ac_power"))
	copyNumber(params, "outAcTtPwr", firstNumber(kv, "ac_data_output_hm.ac_output_power"))
	copyNumber(params, "outAcVol", firstNumber(kv, "ac_data_output_hm.ac_output_voltage"))
	copyNumber(params, "outAcFreq", firstNumber(kv, "ac_data_output_hm.ac_output_hz"))
	copyNumber(params, "outAdsPwr", firstNumber(kv, "dc_data_output_hm.dc_output_power"))
	copyBool(params, "cfgAcEnabled", firstBool(kv, "ac_switch_hm", "host_packet_data_jdb.host_packet_ac_switch", "host_packet_data_jdb.ac_switch"))
	copyBool(params, "dcOutState", firstBool(kv, "dc_switch_hm", "host_packet_data_jdb.host_packet_dc_switch", "host_packet_data_jdb.dc_switch"))
	copyBool(params, "upsMode", firstBool(kv, "ups_status_hm", "host_packet_data_jdb.host_packet_ups_status", "host_packet_data_jdb.ups_status"))
	copyNumber(params, "chgDsgState", firstNumber(kv, "device_status_hm", "host_packet_data_jdb.host_packet_status"))

	pvCount := addPecronPVPorts(params, kv)
	if pvCount > 0 {
		capabilities["pv_input_count"] = pvCount
	}
	return NormalizedTelemetry{
		Params:       collapseEmptyMap(params),
		Capabilities: collapseEmptyMap(capabilities),
		Metadata:     collapseEmptyMap(metadata),
	}
}

func StaticCapabilities(device Device) map[string]any {
	if !isE1000LFP(device) {
		return map[string]any{}
	}
	return map[string]any{
		"battery_capacity_wh":       1024,
		"battery_pack_count":        1,
		"pv_input_count":            1,
		"pv_input_connector":        "XT60",
		"pv_input_max_watts":        600,
		"pv_input_max_volts":        60,
		"pv_input_max_amps":         20,
		"car_input_max_watts":       100,
		"ac_input_max_watts":        1000,
		"ac_output_rated_watts":     1800,
		"ac_output_peak_watts":      3000,
		"dc_output_max_amps":        20,
		"dc_output_nominal_volts":   12,
		"usb_c_output_max_watts":    100,
		"expandable_battery_max_wh": 4864,
		"supports_ac_output":        true,
		"supports_dc_output":        true,
		"supports_usb_output":       true,
		"supports_ups_mode":         true,
		"supports_app_control":      true,
		"supports_wifi":             true,
		"supports_bluetooth":        true,
		"supports_battery_heating":  true,
		"supports_extra_battery":    true,
	}
}

func isE1000LFP(device Device) bool {
	ref := DeviceRefFromDevice(device)
	model := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(device.ProductName), " ", ""))
	if model == "" {
		model = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(device.DeviceName), " ", ""))
	}
	return strings.EqualFold(ref.ProductKey, ProductKeyE1000LFP) || strings.Contains(model, "E1000")
}

func sortedFieldPaths(values map[string]any) []any {
	if len(values) == 0 {
		return nil
	}
	fields := make([]string, 0, len(values))
	collectFieldPaths("", values, &fields)
	sort.Strings(fields)
	out := make([]any, 0, len(fields))
	for _, field := range fields {
		out = append(out, field)
	}
	return out
}

func collectFieldPaths(prefix string, values map[string]any, out *[]string) {
	for key, value := range values {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if nested := asMap(value); len(nested) > 0 {
			collectFieldPaths(path, nested, out)
			continue
		}
		*out = append(*out, path)
	}
}

func addPecronPVPorts(params map[string]any, kv map[string]any) int {
	if hasAnyPath(kv, "dc_data_input_hm.dc_input_power", "dc_data_input_hm.dc_input_voltage", "dc_data_input_hm.dc_input_current") {
		watts := firstNumber(kv, "dc_data_input_hm.dc_input_power")
		volts := firstNumber(kv, "dc_data_input_hm.dc_input_voltage", "dc_data_input_hm.gx16mf1_input_voltage", "dc_data_input_hm.dc5521_input_voltage")
		amps := firstNumber(kv, "dc_data_input_hm.dc_input_current", "dc_data_input_hm.gx16mf1_input_current", "dc_data_input_hm.dc5521_input_current")
		copyNumber(params, "pv1ChargeWatts", watts)
		copyNumber(params, "pv1InWatts", watts)
		copyNumber(params, "pv1InVol", volts)
		copyNumber(params, "pv1InAmp", amps)
		if asFloat(watts) > 1 {
			params["pv1ChgState"] = float64(2)
		} else {
			params["pv1ChgState"] = float64(0)
		}
		return 1
	}
	candidates := []struct {
		prefix string
		vol    string
		amp    string
		watts  string
	}{
		{prefix: "dc5521", vol: "dc_data_input_hm.dc5521_input_voltage", amp: "dc_data_input_hm.dc5521_input_current", watts: "dc_data_input_hm.dc5521_input_power"},
		{prefix: "gx16mf1", vol: "dc_data_input_hm.gx16mf1_input_voltage", amp: "dc_data_input_hm.gx16mf1_input_current", watts: "dc_data_input_hm.gx16mf1_input_power"},
		{prefix: "gx16mf2", vol: "dc_data_input_hm.gx16mf2_input_voltage", amp: "dc_data_input_hm.gx16mf2_input_current", watts: "dc_data_input_hm.gx16mf2_input_power"},
	}
	count := 0
	for _, candidate := range candidates {
		watts := firstNumber(kv, candidate.watts)
		volts := firstNumber(kv, candidate.vol)
		amps := firstNumber(kv, candidate.amp)
		if watts == nil && volts == nil && amps == nil {
			continue
		}
		count++
		prefix := fmt.Sprintf("pv%d", count)
		copyNumber(params, prefix+"ChargeWatts", watts)
		copyNumber(params, prefix+"InWatts", watts)
		copyNumber(params, prefix+"InVol", volts)
		copyNumber(params, prefix+"InAmp", amps)
		if asFloat(watts) > 1 {
			params[prefix+"ChgState"] = float64(2)
		} else {
			params[prefix+"ChgState"] = float64(0)
		}
	}
	return count
}

func hasAnyPath(root map[string]any, paths ...string) bool {
	for _, path := range paths {
		if _, ok := lookupPath(root, path); ok {
			return true
		}
	}
	return false
}

func copyNumber(dst map[string]any, key string, value any) {
	if number, ok := toFloat(value); ok {
		dst[key] = number
	}
}

func copyBool(dst map[string]any, key string, value any) {
	if value == nil {
		return
	}
	if b, ok := toBool(value); ok {
		if b {
			dst[key] = float64(1)
		} else {
			dst[key] = float64(0)
		}
	}
}

func firstNumber(root map[string]any, paths ...string) any {
	for _, path := range paths {
		value, ok := lookupPath(root, path)
		if !ok {
			continue
		}
		if _, ok := toFloat(value); ok {
			return value
		}
	}
	return nil
}

func totalInputPower(root map[string]any) any {
	if value := firstNumber(root, "total_input_power"); value != nil {
		return value
	}
	if sum, ok := sumFirstNumbers(
		root,
		[]string{"ac_data_input_hm.ac_input_power", "ac_data_input_hm.ac_power"},
		[]string{"dc_data_input_hm.dc_input_power"},
	); ok {
		return sum
	}
	return nil
}

func totalOutputPower(root map[string]any) any {
	if value := firstNumber(root, "total_output_power"); value != nil {
		return value
	}
	if sum, ok := sumFirstNumbers(
		root,
		[]string{"ac_data_output_hm.ac_output_power"},
		[]string{"dc_data_output_hm.dc_output_power"},
	); ok {
		return sum
	}
	return nil
}

func sumFirstNumbers(root map[string]any, groups ...[]string) (float64, bool) {
	var sum float64
	observed := false
	for _, group := range groups {
		if value := firstNumber(root, group...); value != nil {
			number, _ := toFloat(value)
			sum += number
			observed = true
		}
	}
	return sum, observed
}

func firstBool(root map[string]any, paths ...string) any {
	for _, path := range paths {
		value, ok := lookupPath(root, path)
		if !ok {
			continue
		}
		if _, ok := toBool(value); ok {
			return value
		}
	}
	return nil
}

func lookupPath(root map[string]any, path string) (any, bool) {
	if len(root) == 0 || path == "" {
		return nil, false
	}
	current := any(root)
	for {
		part := path
		if idx := strings.IndexByte(path, '.'); idx >= 0 {
			part = path[:idx]
			path = path[idx+1:]
		} else {
			path = ""
		}
		record := asMap(current)
		if len(record) == 0 {
			return nil, false
		}
		value, ok := record[part]
		if !ok {
			return nil, false
		}
		current = value
		if path == "" {
			return current, true
		}
	}
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		clean := strings.TrimSpace(v)
		if clean == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(clean, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func toBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "on", "yes", "enabled":
			return true, true
		case "false", "0", "off", "no", "disabled":
			return false, true
		default:
			return false, false
		}
	default:
		if f, ok := toFloat(value); ok {
			return f != 0, true
		}
		return false, false
	}
}

func asFloat(value any) float64 {
	f, _ := toFloat(value)
	return f
}

func asMap(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case string:
		clean := strings.TrimSpace(v)
		if len(clean) < 2 || clean[0] != '{' {
			return nil
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(clean), &parsed); err == nil {
			return parsed
		}
	}
	return nil
}

func asString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(value)
	}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if nested := asMap(value); len(nested) > 0 {
			out[key] = cloneMap(nested)
			continue
		}
		out[key] = value
	}
	return out
}

func collapseEmptyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	return in
}

func suffix(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}
