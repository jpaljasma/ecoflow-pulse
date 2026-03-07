package ingestworker

import (
	"math"
	"testing"
)

func TestNormalizeEcoFlowQuotaEmitsCanonicalParamsCapabilitiesAndMetadata(t *testing.T) {
	t.Parallel()

	normalized := normalizeEcoFlowQuota(map[string]string{
		"pd.soc":                                  "54",
		"pd.remainTime":                           "887",
		"pd.wattsInSum":                           "237.0",
		"pd.wattsOutSum":                          "98.0",
		"bms_bmsStatus.inputWatts":                "12.5",
		"bms_bmsStatus.outputWatts":               "45",
		"bms_bmsStatus.vol":                       "51.2",
		"bms_bmsStatus.amp":                       "-0.88",
		"bms_bmsStatus.temp":                      "13",
		"inv.cfgAcEnabled":                        "1",
		"pd.dcOutState":                           "1",
		"pd.carState":                             "1",
		"pd.typec1Watts":                          "22.4",
		"hs_yj751_pd_appshow_addr.bpNum":          "2",
		"hs_yj751_pd_backend_addr.inLvMpptVol":    "66.3",
		"hs_yj751_pd_backend_addr.inLvMpptAmp":    "0.79",
		"hs_yj751_pd_app_set_info_addr.chgMaxSoc": "85",
		"hs_yj751_pd_app_set_info_addr.dsgMinSoc": "5",
		"pd.pv1ChargeType":                        "2",
		"pd.pv2ChargeType":                        "1",
		"customJson":                              `{"nested":{"n":2},"arr":[1,2.5,true]}`,
	})

	if got := normalized.Params["soc"]; got != int64(54) {
		t.Fatalf("soc mismatch: got=%v", got)
	}
	if got := normalized.Params["f32ShowSoc"]; got != int64(54) {
		t.Fatalf("f32ShowSoc mismatch: got=%v", got)
	}
	if got, ok := numberFromAny(normalized.Params["wattsInSum"]); !ok || got != 237 {
		t.Fatalf("wattsInSum mismatch: got=%v ok=%v", got, ok)
	}
	if got := normalized.Params["bmsOutputWatts"]; got != int64(45) {
		t.Fatalf("bmsOutputWatts mismatch: got=%v", got)
	}
	if got, ok := numberFromAny(normalized.Params["inLvMpptPwr"]); !ok || got != 66.3*0.79 {
		t.Fatalf("derived low pv power mismatch: got=%v ok=%v", got, ok)
	}
	if got := normalized.Capabilities["battery_pack_count"]; got != int64(2) {
		t.Fatalf("battery_pack_count mismatch: got=%v", got)
	}
	if got := normalized.Capabilities["pv_input_count"]; got != int64(2) {
		t.Fatalf("pv_input_count mismatch: got=%v", got)
	}
	if got := normalized.Capabilities["supports_ac_output"]; got != true {
		t.Fatalf("supports_ac_output mismatch: got=%v", got)
	}
	if got := normalized.Capabilities["supports_dc_output"]; got != true {
		t.Fatalf("supports_dc_output mismatch: got=%v", got)
	}
	if got := normalized.Capabilities["supports_usb_output"]; got != true {
		t.Fatalf("supports_usb_output mismatch: got=%v", got)
	}
	if got := normalized.Capabilities["supports_extra_battery"]; got != true {
		t.Fatalf("supports_extra_battery mismatch: got=%v", got)
	}

	limits, ok := normalized.Metadata["limits"].(map[string]any)
	if !ok {
		t.Fatalf("expected limits metadata map, got=%T", normalized.Metadata["limits"])
	}
	if got := limits["chg_max_soc"]; got != int64(85) {
		t.Fatalf("chg_max_soc mismatch: got=%v", got)
	}
	settings, ok := normalized.Metadata["settings"].(map[string]any)
	if !ok {
		t.Fatalf("expected settings metadata map, got=%T", normalized.Metadata["settings"])
	}
	if got := settings["cfg_ac_enabled"]; got != int64(1) {
		t.Fatalf("cfg_ac_enabled mismatch: got=%v", got)
	}
	groups, ok := normalized.Metadata["groups"].(map[string]any)
	if !ok {
		t.Fatalf("expected grouped metadata map, got=%T", normalized.Metadata["groups"])
	}
	pdGroup, ok := groups["pd"].(map[string]any)
	if !ok {
		t.Fatalf("expected pd group map, got=%T", groups["pd"])
	}
	if got := pdGroup["soc"]; got != int64(54) {
		t.Fatalf("grouped pd.soc mismatch: got=%v", got)
	}
	if got := normalized.Metadata["quota_key_count"]; got != 21 {
		t.Fatalf("quota_key_count mismatch: got=%v", got)
	}
}

func TestDecodeQuotaMapNormalizesJSONValues(t *testing.T) {
	t.Parallel()

	decoded := decodeQuotaMap(map[string]string{
		"json":  `{"enabled":true,"limit":85,"nested":[1,2.5,"x"]}`,
		"float": "12.5",
		"int":   "42",
		"bool":  "false",
		"text":  "delta2max",
	})

	if got := decoded["float"]; got != 12.5 {
		t.Fatalf("float mismatch: got=%v", got)
	}
	if got := decoded["int"]; got != int64(42) {
		t.Fatalf("int mismatch: got=%v", got)
	}
	if got := decoded["bool"]; got != false {
		t.Fatalf("bool mismatch: got=%v", got)
	}
	jsonValue, ok := decoded["json"].(map[string]any)
	if !ok {
		t.Fatalf("expected json map, got=%T", decoded["json"])
	}
	if got := jsonValue["limit"]; got != int64(85) {
		t.Fatalf("json limit mismatch: got=%v", got)
	}
	nested, ok := jsonValue["nested"].([]any)
	if !ok || len(nested) != 3 {
		t.Fatalf("nested array mismatch: got=%T len=%d", jsonValue["nested"], len(nested))
	}
	if got := nested[1]; got != 2.5 {
		t.Fatalf("nested float mismatch: got=%v", got)
	}
	if got := decoded["text"]; got != "delta2max" {
		t.Fatalf("text mismatch: got=%v", got)
	}
}

func TestNormalizeEcoFlowQuotaReturnsNilMapsWhenEmpty(t *testing.T) {
	t.Parallel()

	normalized := normalizeEcoFlowQuota(nil)
	if normalized.Params != nil {
		t.Fatalf("expected nil params for empty quota, got=%v", normalized.Params)
	}
	if normalized.Capabilities != nil {
		t.Fatalf("expected nil capabilities for empty quota, got=%v", normalized.Capabilities)
	}
	metadata, ok := normalized.Metadata["quota_key_count"]
	if !ok || metadata != 0 {
		t.Fatalf("expected only quota_key_count=0 metadata, got=%v", normalized.Metadata)
	}
}

func TestNormalizeEcoFlowQuotaScalesD2MMilliUnits(t *testing.T) {
	t.Parallel()

	normalized := normalizeEcoFlowQuota(map[string]string{
		"mppt.inVol":    "10499",
		"mppt.inAmp":    "133",
		"mppt.pv2InVol": "15118",
		"mppt.pv2InAmp": "33",
		"inv.acInVol":   "119260",
		"inv.acInAmp":   "1251",
	})

	if got, ok := numberFromAny(normalized.Params["inLvMpptVol"]); !ok || got != 10.499 {
		t.Fatalf("scaled inLvMpptVol mismatch: got=%v ok=%v", got, ok)
	}
	if got, ok := numberFromAny(normalized.Params["inLvMpptAmp"]); !ok || got != 0.133 {
		t.Fatalf("scaled inLvMpptAmp mismatch: got=%v ok=%v", got, ok)
	}
	if got, ok := numberFromAny(normalized.Params["inLvMpptPwr"]); !ok || math.Abs(got-(10.499*0.133)) > 1e-9 {
		t.Fatalf("scaled inLvMpptPwr mismatch: got=%v ok=%v", got, ok)
	}
	if got, ok := numberFromAny(normalized.Params["inHvMpptAmp"]); !ok || got != 0.033 {
		t.Fatalf("scaled inHvMpptAmp mismatch: got=%v ok=%v", got, ok)
	}
	if got, ok := numberFromAny(normalized.Params["inHvMpptPwr"]); !ok || math.Abs(got-(15.118*0.033)) > 1e-9 {
		t.Fatalf("scaled inHvMpptPwr mismatch: got=%v ok=%v", got, ok)
	}
	groups, _ := normalized.Metadata["groups"].(map[string]any)
	mppt, _ := groups["mppt"].(map[string]any)
	if got := mppt["inVol"]; got != 10.499 {
		t.Fatalf("grouped scaled mppt.inVol mismatch: got=%v", got)
	}
	if got := mppt["pv2InAmp"]; got != 0.033 {
		t.Fatalf("grouped scaled mppt.pv2InAmp mismatch: got=%v", got)
	}
	inv, _ := groups["inv"].(map[string]any)
	if got := inv["acInVol"]; got != 119.26 {
		t.Fatalf("grouped scaled inv.acInVol mismatch: got=%v", got)
	}
}
