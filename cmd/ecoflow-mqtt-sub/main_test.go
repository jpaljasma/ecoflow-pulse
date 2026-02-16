package main

import (
	"testing"
)

func TestParseTelemetryPayloadKitInfo(t *testing.T) {
	payload := []byte(`{"moduleType":2,"needAck":1,"id":38510683,"time":14084552,"params":{"watts":[{"appState":0,"curPower":0,"appVer":0,"f32Soc":0,"soc":0,"avaFlag":0,"sn":"","detail":0,"type":0,"loadVer":0},{"appState":1,"curPower":-97,"appVer":33620275,"f32Soc":29.38,"soc":29,"avaFlag":1,"sn":"R361Z1BAPH2K1398","detail":4,"type":81,"loadVer":33619974}]},"version":"1.0","typeCode":"kitInfo"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parseTelemetryPayload error: %v", err)
	}
	if envelope.TypeCode != "kitInfo" {
		t.Fatalf("typeCode mismatch: got=%q", envelope.TypeCode)
	}
	if envelope.ID != 38510683 {
		t.Fatalf("id mismatch: got=%d", envelope.ID)
	}
	entries, found := extractKitInfoWatts(quota)
	if !found {
		t.Fatal("expected kitInfo watts entries")
	}
	if len(entries) != 2 {
		t.Fatalf("entry count mismatch: got=%d want=2", len(entries))
	}
	if entries[1].SN != "R361Z1BAPH2K1398" {
		t.Fatalf("sn mismatch: got=%q", entries[1].SN)
	}
	if entries[1].CurPower != -97 {
		t.Fatalf("curPower mismatch: got=%v", entries[1].CurPower)
	}
}

func TestSummarizeKitInfo(t *testing.T) {
	entries := []kitInfoWattsEntry{
		{AvaFlag: 0, AppState: 0, CurPower: 0, F32Soc: 0, Soc: 0},
		{AvaFlag: 1, AppState: 1, CurPower: -97, F32Soc: 29.38, Soc: 29},
	}
	stats := summarizeKitInfo(entries)
	if stats.TotalSlots != 2 {
		t.Fatalf("total slots mismatch: got=%d", stats.TotalSlots)
	}
	if stats.AvailableSlots != 1 {
		t.Fatalf("available slots mismatch: got=%d", stats.AvailableSlots)
	}
	if stats.DischargingSlots != 1 {
		t.Fatalf("discharging slots mismatch: got=%d", stats.DischargingSlots)
	}
	if stats.TotalCurPower != -97 {
		t.Fatalf("total power mismatch: got=%f", stats.TotalCurPower)
	}
	if stats.AvgSOC != 29.38 {
		t.Fatalf("avg soc mismatch: got=%f", stats.AvgSOC)
	}
}

func TestParseTelemetryPayloadPDStatus(t *testing.T) {
	payload := []byte(`{"moduleType":1,"needAck":0,"id":8213779,"time":14083892,"params":{"XT150Watts2":-96,"wattsInSum":96,"icoBytes":[0,8,136,0,128,0,0,0,0,0,0,0,0,0]},"version":"1.0","typeCode":"pdStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parseTelemetryPayload error: %v", err)
	}
	if envelope.TypeCode != "pdStatus" {
		t.Fatalf("typeCode mismatch: got=%q", envelope.TypeCode)
	}
	if isPVInputKey("XT150Watts2") {
		t.Fatal("expected XT150Watts2 to not be recognized as PV input key")
	}
	if !isPVInputKey("pv1ChargeWatts") {
		t.Fatal("expected pv1ChargeWatts to be recognized as PV input key")
	}

	summary, found := extractPDStatus(quota)
	if !found {
		t.Fatal("expected pdStatus summary")
	}
	if summary.WattsInSum != 96 {
		t.Fatalf("wattsInSum mismatch: got=%f", summary.WattsInSum)
	}
	if summary.NetWatts != 96 {
		t.Fatalf("netWatts mismatch: got=%f want=96", summary.NetWatts)
	}
	if summary.XT150Watts["XT150Watts2"] != -96 {
		t.Fatalf("XT150Watts2 mismatch: got=%f", summary.XT150Watts["XT150Watts2"])
	}
	if summary.TotalXT150Watts != -96 {
		t.Fatalf("total XT150 watts mismatch: got=%f want=-96", summary.TotalXT150Watts)
	}
	if summary.TotalPVInputWatts != 0 {
		t.Fatalf("total pv input mismatch: got=%f want=0", summary.TotalPVInputWatts)
	}
	if len(summary.IcoBytes) != 14 {
		t.Fatalf("icoBytes size mismatch: got=%d want=14", len(summary.IcoBytes))
	}
	if summary.IcoBytes[2] != 136 {
		t.Fatalf("icoBytes[2] mismatch: got=%d want=136", summary.IcoBytes[2])
	}
}

func TestParseTelemetryPayloadPDStatusRich(t *testing.T) {
	payload := []byte(`{"moduleType":1,"needAck":0,"id":8214440,"time":14303302,"params":{"chgSunPower":0,"chgPowerAC":58473,"usb2Watts":0,"pvChargePrioSet":0,"XT150Watts1":0,"XT150Watts2":-99,"soc":24,"sysVer":16975450,"pv1ChargeType":0,"typec2Temp":26,"dcOutState":1,"minAcSoc":10,"usb1Watts":0,"model":3,"wifiRssi":0,"typec1Temp":26,"wattsInSum":99,"invUsedTime":10335503,"relaySwitchCnt":16,"pv2ChargeType":0,"carWatts":0,"bpPowerSoc":15,"usbUsedTime":1873,"chgPowerDC":288051,"watchIsConfig":1,"bmsKitState":[0,65],"dsgPowerAC":224688,"dcInUsedTime":5070348,"acAutoPause":0,"usbqcUsedTime":154430,"typecUsedTime":166598,"carTemp":33,"reserved":[0,0],"acAutoOnCfg":0,"errCode":0,"invInWatts":0,"mpptUsedTime":0,"qcUsb1Watts":0,"carState":0,"beepMode":1,"otherKitState":0,"newAcAutoOnCfg":0,"invOutWatts":139,"dsgPowerDC":1563,"wifiVer":0,"qcUsb2Watts":0,"typec1Watts":0,"hysteresisAdd":5,"standbyMin":0,"wireWatts":0,"wifiAutoRcvy":0,"wattsOutSum":139,"remainTime":454,"chgDsgState":1,"brightLevel":100,"carUsedTime":193898,"pv1ChargeWatts":0,"typec2Watts":0,"lcdOffSec":1800,"pv2ChargeWatts":0,"icoBytes":[0,8,8,0,128,0,0,0,0,0,0,0,0,0]},"version":"1.0","typeCode":"pdStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parseTelemetryPayload error: %v", err)
	}
	if envelope.TypeCode != "pdStatus" {
		t.Fatalf("typeCode mismatch: got=%q", envelope.TypeCode)
	}

	summary, found := extractPDStatus(quota)
	if !found {
		t.Fatal("expected pdStatus summary")
	}
	if summary.Soc != 24 {
		t.Fatalf("soc mismatch: got=%d want=24", summary.Soc)
	}
	if summary.RemainTime != 454 {
		t.Fatalf("remainTime mismatch: got=%d want=454", summary.RemainTime)
	}
	if summary.WattsInSum != 99 {
		t.Fatalf("wattsInSum mismatch: got=%f want=99", summary.WattsInSum)
	}
	if summary.WattsOutSum != 139 {
		t.Fatalf("wattsOutSum mismatch: got=%f want=139", summary.WattsOutSum)
	}
	if summary.NetWatts != -40 {
		t.Fatalf("netWatts mismatch: got=%f want=-40", summary.NetWatts)
	}
	if summary.XT150Watts["XT150Watts2"] != -99 {
		t.Fatalf("XT150Watts2 mismatch: got=%f want=-99", summary.XT150Watts["XT150Watts2"])
	}
	if summary.TotalXT150Watts != -99 {
		t.Fatalf("total XT150 watts mismatch: got=%f want=-99", summary.TotalXT150Watts)
	}
	if summary.InvOutWatts != 139 {
		t.Fatalf("invOutWatts mismatch: got=%f want=139", summary.InvOutWatts)
	}
	if summary.ChargePowerCounters["chgPowerAC"] != 58473 {
		t.Fatalf("chgPowerAC mismatch: got=%f want=58473", summary.ChargePowerCounters["chgPowerAC"])
	}
	if summary.DischargePowerCounters["dsgPowerAC"] != 224688 {
		t.Fatalf("dsgPowerAC mismatch: got=%f want=224688", summary.DischargePowerCounters["dsgPowerAC"])
	}
	if len(summary.IcoBytes) != 14 || summary.IcoBytes[1] != 8 {
		t.Fatalf("icoBytes mismatch: got=%v", summary.IcoBytes)
	}
	if len(summary.BMSKitState) != 2 || summary.BMSKitState[1] != 65 {
		t.Fatalf("bmsKitState mismatch: got=%v", summary.BMSKitState)
	}
}

func TestParseTelemetryPayloadUsesParamFallback(t *testing.T) {
	payload := []byte(`{"cmdId":4,"cmdFunc":2,"addr":"hs_yj751_pd_bp_addr","param":{"bpInfo":[{"bpSoc":14,"bpNo":1},{"bpSoc":20,"bpNo":2}]},"params":{}}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parseTelemetryPayload error: %v", err)
	}
	if envelope.TypeCode != "bpInfo" {
		t.Fatalf("typeCode mismatch: got=%q want=bpInfo", envelope.TypeCode)
	}
	if envelope.CmdID != 4 || envelope.CmdFunc != 2 {
		t.Fatalf("cmd fields mismatch: cmdId=%d cmdFunc=%d", envelope.CmdID, envelope.CmdFunc)
	}
	if _, ok := quota["bpInfo"]; !ok {
		t.Fatalf("expected key bpInfo in merged quota, got keys=%v", keysFromMap(quota))
	}
	if _, ok := quota["hs_yj751_pd_bp_addr.bpInfo"]; !ok {
		t.Fatalf("expected prefixed key for bpInfo, got keys=%v", keysFromMap(quota))
	}
}

func TestInferTypeAndPDStatusGatingForBMSSlave(t *testing.T) {
	payload := []byte(`{"cmdId":28,"cmdFunc":3,"addr":"hs_yj751_bms_slave_addr_1","params":{"soc":14,"remainTime":143999}}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parseTelemetryPayload error: %v", err)
	}
	if envelope.TypeCode != "bmsSlave" {
		t.Fatalf("typeCode mismatch: got=%q want=bmsSlave", envelope.TypeCode)
	}
	if isPDStatusEnvelope(envelope) {
		t.Fatal("bmsSlave payload must not be considered pdStatus")
	}
	if _, ok := quota["hs_yj751_bms_slave_addr_1.soc"]; !ok {
		t.Fatalf("expected prefixed SOC key for bms slave payload, got keys=%v", keysFromMap(quota))
	}
}

func TestParseTelemetryPayloadNoRecursiveAddrPrefix(t *testing.T) {
	payload := []byte(`{"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"remainTime":1615}}`)
	_, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parseTelemetryPayload error: %v", err)
	}
	if len(quota) != 2 {
		t.Fatalf("unexpected key count: got=%d want=2 keys=%v", len(quota), keysFromMap(quota))
	}
	if _, ok := quota["remainTime"]; !ok {
		t.Fatalf("missing raw key remainTime in keys=%v", keysFromMap(quota))
	}
	if _, ok := quota["hs_yj751_pd_appshow_addr.remainTime"]; !ok {
		t.Fatalf("missing prefixed key in keys=%v", keysFromMap(quota))
	}
}

func keysFromMap(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
