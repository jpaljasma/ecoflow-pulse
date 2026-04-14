package ingestworker

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

type ecoFlowQuotaNormalization struct {
	Params       map[string]any
	Capabilities map[string]any
	Metadata     map[string]any
}

var quotaMetadataGroupPrefixes = []string{
	"pd",
	"inv",
	"mppt",
	"bms_bmsStatus",
	"bms_emsStatus",
	"bms_kitInfo",
	"hs_yj751_pd_appshow_addr",
	"hs_yj751_pd_app_set_info_addr",
	"hs_yj751_pd_backend_addr",
	"hs_yj751_pd_bp_addr",
	"hs_yj751_bms_ems_status_addr",
	"hs_yj751_bms_kitInfo_addr",
	"cms",
	"bms",
	"pow",
	"d_addr",
	"plugInInfo",
	"flowInfo",
}

func normalizeEcoFlowQuota(quota map[string]string) ecoFlowQuotaNormalization {
	parsed := decodeQuotaMap(quota)
	params := make(map[string]any, 32)
	capabilities := make(map[string]any, 16)
	metadata := make(map[string]any, 8)

	copyNumberFromQuotaCandidates(params, "soc", parsed,
		"soc",
		"pd.soc",
		"hs_yj751_pd_appshow_addr.soc",
		"cmsBattSoc",
		"d_addr.cmsBattSoc",
	)
	if value, ok := params["soc"]; ok {
		params["f32ShowSoc"] = value
	}
	copyNumberFromQuotaCandidates(params, "remainTime", parsed,
		"remainTime",
		"pd.remainTime",
		"hs_yj751_pd_appshow_addr.remainTime",
		"dsgRemainTime",
		"chgRemainTime",
	)
	copyNumberFromQuotaCandidates(params, "chgRemainTime", parsed,
		"chgRemainTime",
		"bms_emsStatus.chgRemainTime",
		"hs_yj751_bms_ems_status_addr.chgRemainTime",
	)
	copyNumberFromQuotaCandidates(params, "dsgRemainTime", parsed,
		"dsgRemainTime",
		"bms_emsStatus.dsgRemainTime",
		"hs_yj751_bms_ems_status_addr.dsgRemainTime",
	)
	copyNumberFromQuotaCandidates(params, "chgDsgState", parsed,
		"chgDsgState",
		"pd.chgDsgState",
		"hs_yj751_pd_appshow_addr.chgDsgState",
		"bms_emsStatus.sysChgDsgState",
		"hs_yj751_bms_ems_status_addr.sysChgDsgState",
	)
	copyNumberFromQuotaCandidates(params, "wattsInSum", parsed,
		"wattsInSum",
		"pd.wattsInSum",
		"hs_yj751_pd_appshow_addr.wattsInSum",
		"powInSumW",
	)
	copyNumberFromQuotaCandidates(params, "wattsOutSum", parsed,
		"wattsOutSum",
		"pd.wattsOutSum",
		"hs_yj751_pd_appshow_addr.wattsOutSum",
		"powOutSumW",
	)
	copyNumberFromQuotaCandidates(params, "bmsInputWatts", parsed,
		"bmsInputWatts",
		"hs_yj751_pd_backend_addr.bmsInputWatts",
		"bms_bmsStatus.inputWatts",
	)
	copyNumberFromQuotaCandidates(params, "bmsOutputWatts", parsed,
		"bmsOutputWatts",
		"hs_yj751_pd_backend_addr.bmsOutputWatts",
		"bms_bmsStatus.outputWatts",
	)
	copyNumberFromQuotaCandidates(params, "batAmp", parsed,
		"batAmp",
		"hs_yj751_pd_backend_addr.batAmp",
		"bms_bmsStatus.amp",
	)
	copyNumberFromQuotaCandidates(params, "batVol", parsed,
		"batVol",
		"hs_yj751_pd_backend_addr.batVol",
		"bms_bmsStatus.vol",
	)
	copyNumberFromQuotaCandidates(params, "temp", parsed,
		"temp",
		"pd.temp",
		"bms_bmsStatus.temp",
	)
	copyNumberFromQuotaCandidates(params, "pdTemp", parsed,
		"pdTemp",
		"hs_yj751_pd_backend_addr.pdTemp",
	)
	copyNumberFromQuotaCandidates(params, "fanState", parsed,
		"fanState",
		"inv.fanState",
		"hs_yj751_inv_addr.fanState",
		"hs_yj751_pd_backend_addr.fanState",
	)
	copyNumberFromQuotaCandidates(params, "fanLevel", parsed,
		"fanLevel",
		"bms_emsStatus.fanLevel",
		"hs_yj751_bms_ems_status_addr.fanLevel",
	)
	copyNumberFromQuotaCandidates(params, "cfgAcEnabled", parsed,
		"cfgAcEnabled",
		"inv.cfgAcEnabled",
	)
	copyNumberFromQuotaCandidates(params, "dcOutState", parsed,
		"dcOutState",
		"pd.dcOutState",
		"hs_yj751_pd_appshow_addr.dcOutState",
	)
	copyNumberFromQuotaCandidates(params, "carState", parsed,
		"carState",
		"pd.carState",
		"mppt.carState",
		"hs_yj751_pd_appshow_addr.carState",
	)
	copyNumberFromQuotaCandidates(params, "showFlag", parsed,
		"showFlag",
		"pd.showFlag",
		"hs_yj751_pd_appshow_addr.showFlag",
	)
	copyNumberFromQuotaCandidates(params, "acOutFreq", parsed,
		"acOutFreq",
		"inv.acOutFreq",
		"hs_yj751_pd_backend_addr.acOutFreq",
		"hs_yj751_pd_app_set_info_addr.acOutFreq",
	)
	copyNumberFromQuotaCandidates(params, "chgMaxSoc", parsed,
		"chgMaxSoc",
		"maxChargeSoc",
		"cmsMaxChgSoc",
		"bms_emsStatus.maxChargeSoc",
		"hs_yj751_bms_ems_status_addr.maxChargeSoc",
		"hs_yj751_pd_app_set_info_addr.chgMaxSoc",
		"d_addr.cmsMaxChgSoc",
	)
	copyNumberFromQuotaCandidates(params, "dsgMinSoc", parsed,
		"dsgMinSoc",
		"minDsgSoc",
		"cmsMinDsgSoc",
		"bms_emsStatus.minDsgSoc",
		"hs_yj751_bms_ems_status_addr.minDsgSoc",
		"hs_yj751_pd_app_set_info_addr.dsgMinSoc",
		"d_addr.cmsMinDsgSoc",
	)
	copyNumberFromQuotaCandidates(params, "sysBackupSoc", parsed,
		"sysBackupSoc",
		"hs_yj751_pd_app_set_info_addr.sysBackupSoc",
		"d_addr.backupReverseSoc",
	)
	copyNumberFromQuotaCandidates(params, "bpPowerSoc", parsed,
		"bpPowerSoc",
		"pd.bpPowerSoc",
		"hs_yj751_pd_appshow_addr.bpPowerSoc",
	)
	copyNumberFromQuotaCandidates(params, "minAcSoc", parsed,
		"minAcSoc",
		"pd.minAcSoc",
		"hs_yj751_pd_appshow_addr.minAcSoc",
	)

	copyNumberFromQuotaCandidates(params, "inLvMpptVol", parsed,
		"inLvMpptVol",
		"hs_yj751_pd_backend_addr.inLvMpptVol",
		"mppt.inVol",
	)
	copyNumberFromQuotaCandidates(params, "inLvMpptAmp", parsed,
		"inLvMpptAmp",
		"hs_yj751_pd_backend_addr.inLvMpptAmp",
		"mppt.inAmp",
	)
	copyNumberFromQuotaCandidates(params, "inLvMpptPwr", parsed,
		"inLvMpptPwr",
		"hs_yj751_pd_appshow_addr.inLvMpptPwr",
		"powGetPvL",
	)
	copyNumberFromQuotaCandidates(params, "inHvMpptVol", parsed,
		"inHvMpptVol",
		"hs_yj751_pd_backend_addr.inHvMpptVol",
		"mppt.pv2InVol",
	)
	copyNumberFromQuotaCandidates(params, "inHvMpptAmp", parsed,
		"inHvMpptAmp",
		"hs_yj751_pd_backend_addr.inHvMpptAmp",
		"mppt.pv2InAmp",
	)
	copyNumberFromQuotaCandidates(params, "inHvMpptPwr", parsed,
		"inHvMpptPwr",
		"hs_yj751_pd_appshow_addr.inHvMpptPwr",
		"powGetPvH",
	)
	copyNumberFromQuotaCandidates(params, "pv1ChargeWatts", parsed,
		"pv1ChargeWatts",
		"pd.pv1ChargeWatts",
		"mppt.outWatts",
	)
	copyNumberFromQuotaCandidates(params, "pv2ChargeWatts", parsed,
		"pv2ChargeWatts",
		"pd.pv2ChargeWatts",
	)
	copyNumberFromQuotaCandidates(params, "pv1ChargeType", parsed,
		"pv1ChargeType",
		"pd.pv1ChargeType",
		"plugInInfoPvLType",
		"d_addr.plugInInfoPvLType",
	)
	copyNumberFromQuotaCandidates(params, "pv2ChargeType", parsed,
		"pv2ChargeType",
		"pd.pv2ChargeType",
		"plugInInfoPvHType",
		"d_addr.plugInInfoPvHType",
	)
	derivePVPower(params, "inLvMpptPwr", "inLvMpptVol", "inLvMpptAmp")
	derivePVPower(params, "inHvMpptPwr", "inHvMpptVol", "inHvMpptAmp")

	batteryPackCount := deriveBatteryPackCount(parsed)
	if batteryPackCount > 0 {
		capabilities["battery_pack_count"] = batteryPackCount
	}
	pvInputCount, pvInputs := derivePVInputs(parsed)
	if pvInputCount > 0 {
		capabilities["pv_input_count"] = pvInputCount
		capabilities["pv_inputs"] = pvInputs
	}
	if hasAnyKey(parsed,
		"cfgAcEnabled",
		"inv.cfgAcEnabled",
		"outAcTtPwr",
		"powGetAc",
		"outAcL11Pwr",
		"outAcL12Pwr",
		"outAcL21Pwr",
		"outAcL22Pwr",
	) {
		capabilities["supports_ac_output"] = true
	}
	if hasAnyKey(parsed,
		"dcOutState",
		"pd.dcOutState",
		"hs_yj751_pd_appshow_addr.dcOutState",
		"carState",
		"pd.carState",
		"mppt.carState",
		"hs_yj751_pd_appshow_addr.carState",
		"wireWatts",
		"outAdsPwr",
		"powGet12v",
		"powGet24v",
	) {
		capabilities["supports_dc_output"] = true
	}
	if hasAnyMatchingSubstring(parsed,
		"usb",
		"qcusb",
		"typec",
		"outusb",
		"outtypec",
		"powgettypec",
	) {
		capabilities["supports_usb_output"] = true
	}
	if hasAnyMatchingSubstring(parsed, "ev") {
		capabilities["supports_ev"] = true
	}
	if batteryPackCount > 1 || hasAnyMatchingSubstring(parsed, "kitinfo", "bpinfo", "slave_addr") {
		capabilities["supports_extra_battery"] = true
	}
	if hasAnyMatchingSubstring(parsed, "para") {
		capabilities["supports_parallel"] = true
	}

	if limits := pickQuotaSubset(parsed, []quotaCandidateSet{
		{Name: "chg_max_soc", Candidates: []string{"chgMaxSoc", "maxChargeSoc", "cmsMaxChgSoc", "bms_emsStatus.maxChargeSoc", "hs_yj751_bms_ems_status_addr.maxChargeSoc", "hs_yj751_pd_app_set_info_addr.chgMaxSoc"}},
		{Name: "dsg_min_soc", Candidates: []string{"dsgMinSoc", "minDsgSoc", "cmsMinDsgSoc", "bms_emsStatus.minDsgSoc", "hs_yj751_bms_ems_status_addr.minDsgSoc", "hs_yj751_pd_app_set_info_addr.dsgMinSoc"}},
		{Name: "backup_soc", Candidates: []string{"sysBackupSoc", "bpPowerSoc", "minAcSoc", "backupReverseSoc", "hs_yj751_pd_app_set_info_addr.sysBackupSoc", "pd.bpPowerSoc", "pd.minAcSoc"}},
		{Name: "ac_out_freq", Candidates: []string{"acOutFreq", "inv.acOutFreq", "hs_yj751_pd_backend_addr.acOutFreq", "hs_yj751_pd_app_set_info_addr.acOutFreq"}},
	}); len(limits) > 0 {
		metadata["limits"] = limits
	}
	if settings := pickQuotaSubset(parsed, []quotaCandidateSet{
		{Name: "cfg_ac_enabled", Candidates: []string{"cfgAcEnabled", "inv.cfgAcEnabled"}},
		{Name: "dc_out_state", Candidates: []string{"dcOutState", "pd.dcOutState", "hs_yj751_pd_appshow_addr.dcOutState"}},
		{Name: "car_state", Candidates: []string{"carState", "pd.carState", "mppt.carState", "hs_yj751_pd_appshow_addr.carState"}},
		{Name: "fan_state", Candidates: []string{"fanState", "inv.fanState", "hs_yj751_inv_addr.fanState", "hs_yj751_pd_backend_addr.fanState"}},
		{Name: "fan_level", Candidates: []string{"fanLevel", "bms_emsStatus.fanLevel", "hs_yj751_bms_ems_status_addr.fanLevel"}},
		{Name: "ev_charge_manual_ctrl", Candidates: []string{"evChgManualCtrl", "d_addr.evChgManualCtrl"}},
	}); len(settings) > 0 {
		metadata["settings"] = settings
	}

	if groups := groupQuotaMetadata(parsed); len(groups) > 0 {
		metadata["groups"] = groups
	}
	metadata["quota_key_count"] = len(parsed)

	return ecoFlowQuotaNormalization{
		Params:       collapseEmptyMap(params),
		Capabilities: collapseEmptyMap(capabilities),
		Metadata:     collapseEmptyMap(metadata),
	}
}

type quotaCandidateSet struct {
	Name       string
	Candidates []string
}

func decodeQuotaMap(quota map[string]string) map[string]any {
	decoded := make(map[string]any, len(quota))
	for key, raw := range quota {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		decoded[trimmedKey] = decodeQuotaValue(raw)
	}
	normalizeQuotaUnits(decoded)
	return decoded
}

func normalizeQuotaUnits(decoded map[string]any) {
	for key, value := range decoded {
		switch key {
		case "mppt.inVol", "mppt.pv2InVol":
			if scaled, ok := scaleMetricValue(value, 100, 1000); ok {
				decoded[key] = scaled
			}
		case "mppt.outVol", "inv.invOutVol", "inv.acInVol":
			if scaled, ok := scaleMetricValue(value, 1000, 1000); ok {
				decoded[key] = scaled
			}
		case "mppt.inAmp", "mppt.pv2InAmp":
			if scaled, ok := scaleMetricValue(value, 1, 1000); ok {
				decoded[key] = scaled
			}
		case "mppt.outAmp", "inv.invOutAmp", "inv.acInAmp":
			if scaled, ok := scaleMetricValue(value, 100, 1000); ok {
				decoded[key] = scaled
			}
		}
	}
}

func scaleMetricValue(value any, threshold float64, divisor float64) (any, bool) {
	number, ok := numberFromAny(value)
	if !ok || math.Abs(number) < threshold {
		return nil, false
	}
	scaled := number / divisor
	if math.Trunc(scaled) == scaled {
		return int64(scaled), true
	}
	return scaled, true
}

func decodeQuotaValue(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.EqualFold(trimmed, "true") {
		return true
	}
	if strings.EqualFold(trimmed, "false") {
		return false
	}
	if i, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return f
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		dec := json.NewDecoder(bytes.NewBufferString(trimmed))
		dec.UseNumber()
		var value any
		if err := dec.Decode(&value); err == nil {
			return normalizeQuotaJSONValue(value)
		}
	}
	return trimmed
}

func normalizeQuotaJSONValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, entry := range v {
			out[key] = normalizeQuotaJSONValue(entry)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, entry := range v {
			out = append(out, normalizeQuotaJSONValue(entry))
		}
		return out
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
		if f, err := v.Float64(); err == nil {
			return f
		}
		return v.String()
	default:
		return value
	}
}

func findQuotaValueByCandidates(parsed map[string]any, candidates ...string) (any, bool) {
	for _, candidate := range candidates {
		if value, ok := parsed[candidate]; ok {
			return value, true
		}
	}
	return nil, false
}

func copyNumberFromQuotaCandidates(dst map[string]any, key string, parsed map[string]any, candidates ...string) bool {
	value, ok := findQuotaValueByCandidates(parsed, candidates...)
	if !ok {
		return false
	}
	number, ok := numberFromAny(value)
	if !ok {
		return false
	}
	if math.Trunc(number) == number {
		dst[key] = int64(number)
		return true
	}
	dst[key] = number
	return true
}

func derivePVPower(dst map[string]any, powerKey, voltsKey, ampsKey string) {
	if _, exists := dst[powerKey]; exists {
		return
	}
	volts, okVolts := numberFromAny(dst[voltsKey])
	amps, okAmps := numberFromAny(dst[ampsKey])
	if !okVolts || !okAmps {
		return
	}
	dst[powerKey] = volts * amps
}

func deriveBatteryPackCount(parsed map[string]any) int64 {
	for _, candidates := range [][]string{
		{"bpNum", "pd.bpNum", "hs_yj751_pd_appshow_addr.bpNum"},
		{"bms_kitInfo.watts", "hs_yj751_bms_kitInfo_addr.watts", "kitInfo.watts"},
		{"bpInfo", "hs_yj751_pd_bp_addr.bpInfo"},
	} {
		if value, ok := findQuotaValueByCandidates(parsed, candidates...); ok {
			switch typed := value.(type) {
			case int64:
				if typed > 0 {
					return typed
				}
			case float64:
				if typed > 0 {
					return int64(typed)
				}
			case []any:
				if len(typed) > 0 {
					return int64(len(typed))
				}
			}
		}
	}
	return 0
}

func derivePVInputs(parsed map[string]any) (int64, map[string]any) {
	lowPresent := hasAnyMatchingSubstring(parsed,
		"inlvmppt",
		"pluginfopvl",
		"flowinfopvl",
		"pv1",
	)
	highPresent := hasAnyMatchingSubstring(parsed,
		"inhvmppt",
		"pluginfopvh",
		"flowinfopvh",
		"pv2",
	)
	if !lowPresent && !highPresent {
		return 0, nil
	}
	inputs := map[string]any{}
	if lowPresent {
		inputs["low"] = true
	}
	if highPresent {
		inputs["high"] = true
	}
	count := int64(0)
	if lowPresent {
		count++
	}
	if highPresent {
		count++
	}
	return count, inputs
}

func pickQuotaSubset(parsed map[string]any, sets []quotaCandidateSet) map[string]any {
	out := make(map[string]any, len(sets))
	for _, set := range sets {
		if value, ok := findQuotaValueByCandidates(parsed, set.Candidates...); ok {
			out[set.Name] = value
		}
	}
	return collapseEmptyMap(out)
}

func groupQuotaMetadata(parsed map[string]any) map[string]any {
	groups := make(map[string]any, len(quotaMetadataGroupPrefixes))
	for _, prefix := range quotaMetadataGroupPrefixes {
		group := make(map[string]any)
		dotPrefix := prefix + "."
		for key, value := range parsed {
			if strings.HasPrefix(key, dotPrefix) {
				group[strings.TrimPrefix(key, dotPrefix)] = value
			}
		}
		if len(group) > 0 {
			groups[prefix] = group
		}
	}
	return collapseEmptyMap(groups)
}

func hasAnyKey(parsed map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := parsed[key]; ok {
			return true
		}
	}
	return false
}

func hasAnyMatchingSubstring(parsed map[string]any, fragments ...string) bool {
	for key := range parsed {
		lowerKey := strings.ToLower(key)
		for _, fragment := range fragments {
			if strings.Contains(lowerKey, strings.ToLower(fragment)) {
				return true
			}
		}
	}
	return false
}

func collapseEmptyMap[T any](values map[string]T) map[string]T {
	if len(values) == 0 {
		return nil
	}
	return values
}

func numberFromAny(value any) (float64, bool) {
	switch v := value.(type) {
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
	case int16:
		return float64(v), true
	case int8:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint8:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
