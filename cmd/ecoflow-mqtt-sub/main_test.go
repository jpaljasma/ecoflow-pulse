package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jpaljasma/ecoflow-api-playground/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-api-playground/pkg/ecoflowmqtt"
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

func TestExtractBatterySOCFromKitInfo(t *testing.T) {
	quota := map[string]any{
		"watts": []any{
			map[string]any{
				"appState": 0, "curPower": 0, "f32Soc": 0, "soc": 0, "avaFlag": 0, "sn": "",
			},
			map[string]any{
				"appState": 1, "curPower": -53, "f32Soc": 27.42, "soc": 27, "avaFlag": 1, "sn": "R361Z1BAPH2K1398",
			},
		},
	}

	values := extractBatterySOC(quota)
	if len(values) != 1 {
		t.Fatalf("battery SOC count mismatch: got=%d want=1 values=%v", len(values), values)
	}
	if values[0].Label != "R361Z1BAPH2K1398" {
		t.Fatalf("label mismatch: got=%q", values[0].Label)
	}
	if values[0].SOC != 27.42 {
		t.Fatalf("soc mismatch: got=%f want=27.42", values[0].SOC)
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

func TestEnergySnapshotUsesDAddrSOCAndFullEnergy(t *testing.T) {
	snapshot := newEnergySnapshot()
	envelope := telemetryEnvelope{TypeCode: "dAddr", Addr: "d_addr"}
	quota := map[string]any{
		"cmsBattSoc":        16.0,
		"cmsBattFullEnergy": 12288.0,
	}

	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)
	if !snapshot.HasDeviceSOC || snapshot.DeviceSOC != 16 {
		t.Fatalf("expected device soc 16, got has=%v soc=%f", snapshot.HasDeviceSOC, snapshot.DeviceSOC)
	}
	if !snapshot.HasFullEnergy || snapshot.FullEnergyWh != 12288 {
		t.Fatalf("expected full energy 12288, got has=%v value=%f", snapshot.HasFullEnergy, snapshot.FullEnergyWh)
	}
	got := snapshot.String()
	if !strings.Contains(got, "soc=16.00%") {
		t.Fatalf("expected soc in summary, got=%q", got)
	}
	if !strings.Contains(got, "charge_left=1.97kWh") {
		t.Fatalf("expected charge_left fallback from full energy, got=%q", got)
	}
}

func TestEnergySnapshotAggregatesKitAndBMSSlavePack(t *testing.T) {
	snapshot := newEnergySnapshot()

	kitPayload := []byte(`{"moduleType":2,"needAck":1,"id":38540964,"time":16220822,"params":{"watts":[{"appState":0,"curPower":0,"appVer":0,"f32Soc":0,"soc":0,"avaFlag":0,"sn":"","detail":0,"type":0,"loadVer":0},{"appState":1,"curPower":-36,"appVer":33620275,"f32Soc":27.41,"soc":27,"avaFlag":1,"sn":"R361Z1BAPH2K1398","detail":4,"type":81,"loadVer":33619974}]},"version":"1.0","typeCode":"kitInfo"}`)
	kitEnvelope, kitQuota, err := parseTelemetryPayload(kitPayload)
	if err != nil {
		t.Fatalf("parse kit payload: %v", err)
	}
	kitEntries, hasKit := extractKitInfoWatts(kitQuota)
	snapshot.Update(kitEnvelope, kitQuota, kitEntries, hasKit, pdStatusSummary{}, false)

	pack2, ok := snapshot.Packs[2]
	if !ok {
		t.Fatalf("missing pack 2 after kit update: packs=%v", snapshot.Packs)
	}
	if !pack2.HasSOC || pack2.SOC != 27.41 {
		t.Fatalf("pack2 soc mismatch: has=%v soc=%f", pack2.HasSOC, pack2.SOC)
	}
	if !pack2.HasPower || pack2.PowerW != -36 {
		t.Fatalf("pack2 power mismatch: has=%v power=%f", pack2.HasPower, pack2.PowerW)
	}
	if !snapshot.HasDeviceSOC || snapshot.DeviceSOC != 27.41 {
		t.Fatalf("device soc fallback mismatch: has=%v soc=%f", snapshot.HasDeviceSOC, snapshot.DeviceSOC)
	}

	bmsSlavePayload := []byte(`{"moduleType":4,"needAck":0,"id":71398312,"time":16220752,"params":{"cellVol":[],"cellTemp":[],"remainTime":925,"outputWatts":36},"version":"1.0","typeCode":"bmsSlaveStatus_2"}`)
	bmsEnvelope, bmsQuota, err := parseTelemetryPayload(bmsSlavePayload)
	if err != nil {
		t.Fatalf("parse bms slave payload: %v", err)
	}
	snapshot.Update(bmsEnvelope, bmsQuota, nil, false, pdStatusSummary{}, false)

	pack2 = snapshot.Packs[2]
	if !pack2.HasPower || pack2.PowerW != -36 {
		t.Fatalf("pack2 output watts normalization mismatch: has=%v power=%f", pack2.HasPower, pack2.PowerW)
	}
	if pack2.RemainTimeRaw != 925 {
		t.Fatalf("pack2 remainTime mismatch: got=%d want=925", pack2.RemainTimeRaw)
	}

	summary := snapshot.String()
	if !strings.Contains(summary, "soc=27.41%") {
		t.Fatalf("summary missing device soc: %q", summary)
	}
	if !strings.Contains(summary, "bp2=27.4%") {
		t.Fatalf("summary missing pack2 soc: %q", summary)
	}
}

func TestEnergySnapshotIgnoresPlaceholderKitSlotForBP1(t *testing.T) {
	snapshot := newEnergySnapshot()

	kitPayload := []byte(`{"moduleType":2,"needAck":1,"id":38540964,"time":16220822,"params":{"watts":[{"appState":0,"curPower":0,"appVer":0,"f32Soc":0,"soc":0,"avaFlag":0,"sn":"","detail":0,"type":0,"loadVer":0},{"appState":1,"curPower":-36,"appVer":33620275,"f32Soc":27.41,"soc":27,"avaFlag":1,"sn":"R361Z1BAPH2K1398","detail":4,"type":81,"loadVer":33619974}]},"version":"1.0","typeCode":"kitInfo"}`)
	kitEnvelope, kitQuota, err := parseTelemetryPayload(kitPayload)
	if err != nil {
		t.Fatalf("parse kit payload: %v", err)
	}
	kitEntries, hasKit := extractKitInfoWatts(kitQuota)
	snapshot.Update(kitEnvelope, kitQuota, kitEntries, hasKit, pdStatusSummary{}, false)

	if _, ok := snapshot.Packs[1]; ok {
		t.Fatalf("expected bp1 to be absent for placeholder-only slot; packs=%v", snapshot.Packs)
	}
	if _, ok := snapshot.Packs[2]; !ok {
		t.Fatalf("expected bp2 slot to exist; packs=%v", snapshot.Packs)
	}
}

func TestEnergySnapshotMapsBMSStatusToBP1(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":2,"needAck":0,"id":77015949,"time":17086782,"params":{"soc":23,"f32ShowSoc":23.49,"temp":28,"remainTime":315,"outputWatts":0},"version":"1.0","typeCode":"bmsStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse bmsStatus payload: %v", err)
	}

	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	pack1, ok := snapshot.Packs[1]
	if !ok {
		t.Fatalf("expected bp1 from bmsStatus; packs=%v", snapshot.Packs)
	}
	if !pack1.HasSOC || pack1.SOC != 23.49 {
		t.Fatalf("bp1 soc mismatch: has=%v soc=%f", pack1.HasSOC, pack1.SOC)
	}
	if !pack1.HasTemp || pack1.TempC != 28 {
		t.Fatalf("bp1 temp mismatch: has=%v temp=%f", pack1.HasTemp, pack1.TempC)
	}
	if pack1.RemainTimeRaw != 315 {
		t.Fatalf("bp1 remain mismatch: got=%d want=315", pack1.RemainTimeRaw)
	}
}

func TestEnergySnapshotInfersBP1PowerFromAmpAndVolWhenWattsMissing(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":2,"needAck":0,"id":77015949,"time":17086782,"params":{"soc":23,"temp":28,"outputWatts":0,"inputWatts":0,"amp":-1612,"vol":52055},"version":"1.0","typeCode":"bmsStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse bmsStatus payload: %v", err)
	}

	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	pack1, ok := snapshot.Packs[1]
	if !ok {
		t.Fatalf("expected bp1 from bmsStatus; packs=%v", snapshot.Packs)
	}
	if !pack1.HasPower {
		t.Fatalf("expected bp1 inferred power from amp/vol, got hasPower=false pack=%+v", pack1)
	}
	if pack1.PowerW > -70 || pack1.PowerW < -100 {
		t.Fatalf("expected bp1 inferred discharge around -84W, got=%f", pack1.PowerW)
	}
}

func TestEnergySnapshotDisplaySOCUsesPackAverageForDPU(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 7
	snapshot.HasDeviceSOC = true

	pack1 := snapshot.ensurePack(1)
	pack1.SOC = 6
	pack1.HasSOC = true
	pack1.EnergyWh = 6144
	pack1.HasEnergy = true

	pack2 := snapshot.ensurePack(2)
	pack2.SOC = 7
	pack2.HasSOC = true
	pack2.EnergyWh = 6144
	pack2.HasEnergy = true

	derived := snapshot.derived()
	if derived.SOCValue != "6.50%" {
		t.Fatalf("display soc mismatch: got=%s want=6.50%%", derived.SOCValue)
	}
}

func TestEnergySnapshotMapsBMSSlaveStatusFiveToBP5(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":4,"needAck":0,"id":71499999,"time":17199999,"params":{"soc":61,"temp":23,"remainTime":812,"outputWatts":245},"version":"1.0","typeCode":"bmsSlaveStatus_5"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse bmsSlaveStatus_5 payload: %v", err)
	}

	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	pack5, ok := snapshot.Packs[5]
	if !ok {
		t.Fatalf("expected bp5 from bmsSlaveStatus_5; packs=%v", snapshot.Packs)
	}
	if !pack5.HasSOC || pack5.SOC != 61 {
		t.Fatalf("bp5 soc mismatch: has=%v soc=%f", pack5.HasSOC, pack5.SOC)
	}
	if !pack5.HasTemp || pack5.TempC != 23 {
		t.Fatalf("bp5 temp mismatch: has=%v temp=%f", pack5.HasTemp, pack5.TempC)
	}
	if !pack5.HasPower || pack5.PowerW != -245 {
		t.Fatalf("bp5 power mismatch: has=%v power=%f", pack5.HasPower, pack5.PowerW)
	}
	if pack5.RemainTimeRaw != 812 {
		t.Fatalf("bp5 remain mismatch: got=%d want=812", pack5.RemainTimeRaw)
	}
}

func TestApplyDeviceQuotaToSnapshot_BootstrapsDelta2Style(t *testing.T) {
	snapshot := newEnergySnapshot()
	quota := map[string]string{
		"bms_kitInfo.kitNum":       "2",
		"bms_kitInfo.watts":        `[{"appState":0,"curPower":0,"f32Soc":0,"soc":0,"avaFlag":0,"sn":""},{"appState":1,"curPower":-100,"f32Soc":26.28,"soc":26,"avaFlag":1,"sn":"R361Z1BAPH2K1398"}]`,
		"bms_bmsStatus.f32ShowSoc": "23.49",
		"bms_bmsStatus.temp":       "28",
		"bms_bmsStatus.remainTime": "315",
		// Simulate stale slave#1 values observed in MQTT logs.
		"bms_slave_bmsSlaveStatus_1.f32ShowSoc":  "6.5",
		"bms_slave_bmsSlaveStatus_1.temp":        "12",
		"bms_slave_bmsSlaveStatus_1.outputWatts": "58",
		"pd.wattsInSum":                          "100",
		"pd.wattsOutSum":                         "152",
		"pd.invOutWatts":                         "141",
		"pd.XT150Watts2":                         "-100",
	}

	report := applyDeviceQuotaToSnapshot(snapshot, quota)
	if report.BatteryCount != 2 {
		t.Fatalf("battery count mismatch: got=%d want=2", report.BatteryCount)
	}
	if _, ok := snapshot.Packs[1]; !ok {
		t.Fatalf("missing bp1 after bootstrap: packs=%v", snapshot.Packs)
	}
	if _, ok := snapshot.Packs[2]; !ok {
		t.Fatalf("missing bp2 after bootstrap: packs=%v", snapshot.Packs)
	}
	if !snapshot.Packs[1].HasSOC || snapshot.Packs[1].SOC != 23.49 {
		t.Fatalf("bp1 soc mismatch: has=%v soc=%f", snapshot.Packs[1].HasSOC, snapshot.Packs[1].SOC)
	}
	if !snapshot.Packs[1].HasTemp || snapshot.Packs[1].TempC != 28 {
		t.Fatalf("bp1 temp mismatch: has=%v temp=%f", snapshot.Packs[1].HasTemp, snapshot.Packs[1].TempC)
	}
	if !snapshot.Packs[2].HasSOC || snapshot.Packs[2].SOC != 26.28 {
		t.Fatalf("bp2 soc mismatch: has=%v soc=%f", snapshot.Packs[2].HasSOC, snapshot.Packs[2].SOC)
	}
	if !snapshot.HasWattsIn || snapshot.WattsIn != 100 {
		t.Fatalf("watts in mismatch: has=%v value=%f", snapshot.HasWattsIn, snapshot.WattsIn)
	}
	if !snapshot.HasWattsOut || snapshot.WattsOut != 152 {
		t.Fatalf("watts out mismatch: has=%v value=%f", snapshot.HasWattsOut, snapshot.WattsOut)
	}
	if !snapshot.HasOutAC || snapshot.OutACWatts != 141 {
		t.Fatalf("out_ac mismatch: has=%v value=%f", snapshot.HasOutAC, snapshot.OutACWatts)
	}
	if !snapshot.HasOutDC || snapshot.OutDCWatts != 11 {
		t.Fatalf("out_dc mismatch: has=%v value=%f", snapshot.HasOutDC, snapshot.OutDCWatts)
	}
	if !snapshot.HasXT150 || snapshot.XT150Watts != -100 {
		t.Fatalf("xt150 mismatch: has=%v value=%f", snapshot.HasXT150, snapshot.XT150Watts)
	}
	if !report.MappedXT150 || report.XT150Watts != -100 {
		t.Fatalf("bootstrap report xt150 mismatch: mapped=%v value=%f", report.MappedXT150, report.XT150Watts)
	}
	derived := snapshot.derived()
	if derived.XT150InValue != "100.0W" || derived.XT150OutValue != "0.0W" {
		t.Fatalf("bootstrap xt150 direction mismatch: in=%s out=%s", derived.XT150InValue, derived.XT150OutValue)
	}
}

func TestApplyDeviceQuotaToSnapshot_BootstrapsXT150ChargeDirection(t *testing.T) {
	snapshot := newEnergySnapshot()
	quota := map[string]string{
		"pd.XT150Watts2": "270",
		"pd.wattsInSum":  "919",
		"pd.wattsOutSum": "405",
		"pd.invOutWatts": "119",
	}

	report := applyDeviceQuotaToSnapshot(snapshot, quota)
	if !snapshot.HasXT150 || snapshot.XT150Watts != 270 {
		t.Fatalf("xt150 mismatch: has=%v value=%f", snapshot.HasXT150, snapshot.XT150Watts)
	}
	if !report.MappedXT150 || report.XT150Watts != 270 {
		t.Fatalf("bootstrap report xt150 mismatch: mapped=%v value=%f", report.MappedXT150, report.XT150Watts)
	}
	derived := snapshot.derived()
	if derived.XT150InValue != "0.0W" || derived.XT150OutValue != "270.0W" {
		t.Fatalf("bootstrap xt150 direction mismatch: in=%s out=%s", derived.XT150InValue, derived.XT150OutValue)
	}
}

func TestApplyDeviceQuotaToSnapshot_BootstrapsDPUFivePacks(t *testing.T) {
	snapshot := newEnergySnapshot()
	quota := map[string]string{
		"hs_yj751_bms_slave_addr.5.soc":         "61",
		"hs_yj751_bms_slave_addr.5.temp":        "23",
		"hs_yj751_bms_slave_addr.5.remainTime":  "812",
		"hs_yj751_bms_slave_addr.5.outputWatts": "245",
		"bms_kitInfo.kitNum":                    "5",
	}

	report := applyDeviceQuotaToSnapshot(snapshot, quota)
	if report.BatteryCount != 5 {
		t.Fatalf("battery count mismatch: got=%d want=5", report.BatteryCount)
	}
	if _, ok := snapshot.Packs[5]; !ok {
		t.Fatalf("missing bp5 after bootstrap: packs=%v", snapshot.Packs)
	}
	pack5 := snapshot.Packs[5]
	if !pack5.HasSOC || pack5.SOC != 61 {
		t.Fatalf("bp5 soc mismatch: has=%v soc=%f", pack5.HasSOC, pack5.SOC)
	}
	if !pack5.HasPower || pack5.PowerW != -245 {
		t.Fatalf("bp5 power mismatch: has=%v power=%f", pack5.HasPower, pack5.PowerW)
	}
}

func TestRenderDashboardIncludesSummaryAndPackRows(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.DeviceSOC = 27.4
	snapshot.HasDeviceSOC = true
	snapshot.WattsIn = 36
	snapshot.HasWattsIn = true
	snapshot.WattsOut = 12
	snapshot.HasWattsOut = true
	snapshot.RemainTimeRaw = 960
	snapshot.HasRemainTime = true
	pack := snapshot.ensurePack(2)
	pack.SOC = 27.4
	pack.HasSOC = true
	pack.TempC = 24
	pack.HasTemp = true
	pack.PowerW = -36
	pack.HasPower = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "kitInfo"},
		snapshot,
		nil,
		minuteTableConfig{},
	)

	for _, expected := range []string{
		"EcoFlow Live Telemetry",
		"Kitchen Delta 2 Max",
		"channels",
		"ac: n/a pv_total: n/a xt150_in: n/a",
		"ac: n/a (l14: n/a) dc: n/a xt150_out: n/a",
		"solar #1 [500W]",
		"in: n/a",
		"state: n/a",
		"solar #2 [500W]",
		"[ ] AC On",
		"[ ] USB On",
		"[ ] 12V DC On",
		"[ ] UPS Passthrough",
		"[ ] Grounded (Estimated)",
		"[ ] Solar Passthrough",
		"estimates",
		"charge: n/a",
		"discharge: n/a",
		"| Pack",
		"bp2",
		"discharging",
		"~16h 0m",
		"Solar Generated (Wh)",
		"| n/a",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("dashboard missing %q; output=%q", expected, output)
		}
	}
}

func TestRenderDashboardShowsPassthroughAndGroundedWhenACInOutEquivalent(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasInAC = true
	snapshot.InACWatts = 900
	snapshot.HasOutAC = true
	snapshot.OutACWatts = 890

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra", SN: "Y711ZABA9H2P0294"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "[x] UPS Passthrough") {
		t.Fatalf("expected passthrough checkbox on, got=%q", output)
	}
	if !strings.Contains(output, "[x] Grounded (Estimated)") {
		t.Fatalf("expected grounded estimate checkbox on, got=%q", output)
	}
	if !strings.Contains(output, "[ ] Solar Passthrough") {
		t.Fatalf("expected solar passthrough checkbox off during AC passthrough, got=%q", output)
	}
}

func TestFormatPVInputRowLabelUsesCapacityHints(t *testing.T) {
	d2m := ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max"}
	if got := formatPVInputRowLabel("low", d2m, newEnergySnapshot()); got != "solar #1 [500W]" {
		t.Fatalf("d2m low label mismatch: got=%q want=%q", got, "solar #1 [500W]")
	}
	if got := formatPVInputRowLabel("high", d2m, newEnergySnapshot()); got != "solar #2 [500W]" {
		t.Fatalf("d2m high label mismatch: got=%q want=%q", got, "solar #2 [500W]")
	}

	dpu := ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra"}
	if got := formatPVInputRowLabel("low", dpu, newEnergySnapshot()); got != "solar [1.6kW]" {
		t.Fatalf("dpu low label mismatch: got=%q want=%q", got, "solar [1.6kW]")
	}
	if got := formatPVInputRowLabel("high", dpu, newEnergySnapshot()); got != "solar [4kW]" {
		t.Fatalf("dpu high label mismatch: got=%q want=%q", got, "solar [4kW]")
	}
}

func TestIsLikelyACPassthrough(t *testing.T) {
	if !isLikelyACPassthrough(true, 900, true, 890) {
		t.Fatalf("expected passthrough when ac in/out are equivalent")
	}
	if isLikelyACPassthrough(true, 900, true, 700) {
		t.Fatalf("did not expect passthrough when ac in/out diverge")
	}
	if isLikelyACPassthrough(true, 10, true, 10) {
		t.Fatalf("did not expect passthrough below minimum watts threshold")
	}
	if isLikelyACPassthrough(false, 900, true, 900) {
		t.Fatalf("did not expect passthrough when input signal is missing")
	}
}

func TestIsLikelySolarPassthrough(t *testing.T) {
	chargingFromSolar := newEnergySnapshot()
	chargingFromSolar.HasOutAC = true
	chargingFromSolar.OutACWatts = 130
	chargingFromSolar.HasInAC = true
	chargingFromSolar.InACWatts = 0
	chargingFromSolar.HasInPV = true
	chargingFromSolar.InPVWatts = 170
	if !isLikelySolarPassthrough(chargingFromSolar, 42, true, 0, true) {
		t.Fatalf("expected solar passthrough when AC output is served by PV and battery is charging")
	}
	if !isLikelySolarPassthrough(chargingFromSolar, 0, false, 0, false) {
		t.Fatalf("expected solar passthrough when PV covers AC output and batteries are full/not charging")
	}

	batteryDischarging := newEnergySnapshot()
	batteryDischarging.HasOutAC = true
	batteryDischarging.OutACWatts = 130
	batteryDischarging.HasInAC = true
	batteryDischarging.InACWatts = 0
	batteryDischarging.HasInPV = true
	batteryDischarging.InPVWatts = 70
	if isLikelySolarPassthrough(batteryDischarging, 0, true, 45, true) {
		t.Fatalf("did not expect solar passthrough when battery is discharging")
	}
}

func TestRenderDashboardShowsSmoothedPVButSummaryStaysRaw(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.configurePVSmoothing(3)

	applyPV := func(low, high float64) {
		snapshot.InPVLowWatts = low
		snapshot.HasInPVLow = true
		snapshot.InPVHighWatts = high
		snapshot.HasInPVHigh = true
		snapshot.refreshPVTotalFromChannels()
		snapshot.pushPVSmoothingSample()
	}
	applyPV(30, 60)
	applyPV(40, 70)
	applyPV(50, 80)

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)

	if !strings.Contains(output, "pv_total: 130.0W (~110.0W avg)") {
		t.Fatalf("expected smoothed pv total in dashboard, got=%q", output)
	}
	if !strings.Contains(output, "in: 50.0W (~40.0W avg)") {
		t.Fatalf("expected smoothed pv low in dashboard, got=%q", output)
	}
	if !strings.Contains(output, "in: 80.0W (~70.0W avg)") {
		t.Fatalf("expected smoothed pv high in dashboard, got=%q", output)
	}

	summary := snapshot.String()
	if !strings.Contains(summary, "in_pv_low=50.0W") || !strings.Contains(summary, "in_pv_high=80.0W") || !strings.Contains(summary, "in_pv=130.0W") {
		t.Fatalf("expected raw pv values in summary, got=%q", summary)
	}
	if strings.Contains(summary, "avg") {
		t.Fatalf("summary should not include smoothed marker, got=%q", summary)
	}
}

func TestRenderDashboardShowsSmoothedTotalsButSummaryStaysRaw(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.configurePowerSmoothing(3)

	applyTotals := func(inWatts, outWatts float64) {
		snapshot.WattsIn = inWatts
		snapshot.HasWattsIn = true
		snapshot.WattsOut = outWatts
		snapshot.HasWattsOut = true
		snapshot.InACWatts = inWatts
		snapshot.HasInAC = true
		snapshot.pushPowerSmoothingSample()
	}
	applyTotals(300, 100)
	applyTotals(200, 100)
	applyTotals(100, 100)

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)

	if !strings.Contains(output, "100.0W (~200.0W avg)") {
		t.Fatalf("expected smoothed total input in dashboard, got=%q", output)
	}
	if !strings.Contains(output, "0.0W (~100.0W avg)") {
		t.Fatalf("expected smoothed total net in dashboard, got=%q", output)
	}

	summary := snapshot.String()
	if !strings.Contains(summary, "in=100.0W") || !strings.Contains(summary, "out=100.0W") || !strings.Contains(summary, "net=0.0W") {
		t.Fatalf("expected raw totals in summary, got=%q", summary)
	}
	if strings.Contains(summary, "avg") {
		t.Fatalf("summary should not include smoothed marker, got=%q", summary)
	}
}

func TestRenderDashboardHidesPreconditioningForD2M(t *testing.T) {
	snapshot := newEnergySnapshot()
	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if strings.Contains(output, "Battery Preconditioning On") {
		t.Fatalf("d2m dashboard should hide preconditioning line, got=%q", output)
	}
}

func TestRenderDashboardShowsPreconditioningForDPU(t *testing.T) {
	snapshot := newEnergySnapshot()
	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "DPU A 12 kWh", ProductName: "DELTA Pro Ultra", SN: "Y711ZABA9H2P0294"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "Battery Preconditioning On") {
		t.Fatalf("dpu dashboard should show preconditioning line, got=%q", output)
	}
}

func TestMinuteTelemetryHistoryAggregatesByMinute(t *testing.T) {
	history := newMinuteTelemetryHistory(32)
	snapshot := newEnergySnapshot()

	base := time.Date(2026, time.February, 15, 10, 1, 5, 0, time.Local)
	snapshot.HasInPV = true
	snapshot.InPVWatts = 50
	snapshot.HasInAC = true
	snapshot.InACWatts = 800
	snapshot.HasOutAC = true
	snapshot.OutACWatts = 100
	snapshot.HasOutDC = true
	snapshot.OutDCWatts = 20
	history.AddSample(base, snapshot)

	snapshot.InPVWatts = 70
	snapshot.InACWatts = 900
	snapshot.OutACWatts = 120
	snapshot.OutDCWatts = 40
	history.AddSample(base.Add(30*time.Second), snapshot)

	snapshot.InPVWatts = 30
	snapshot.InACWatts = 700
	snapshot.OutACWatts = 90
	snapshot.OutDCWatts = 10
	history.AddSample(base.Add(1*time.Minute), snapshot)

	rows := buildMinuteTelemetryRows(history, minuteTableConfig{Rows: 5, NewestFirst: true})
	if len(rows) != 2 {
		t.Fatalf("row count mismatch: got=%d want=2", len(rows))
	}
	if rows[0][0] != "2026-02-15 10:02" {
		t.Fatalf("newest row time mismatch: got=%q want=%q", rows[0][0], "2026-02-15 10:02")
	}
	if rows[0][1] != "0.5" || rows[0][2] != "11.7" || rows[0][3] != "1.5" || rows[0][4] != "0.2" || rows[0][5] != "10.5" || rows[0][6] != "12.2" || rows[0][7] != "1.7" || rows[0][8] != "10.5" {
		t.Fatalf("newest row metrics mismatch: got=%v", rows[0])
	}
	if rows[1][0] != "2026-02-15 10:01" {
		t.Fatalf("older row time mismatch: got=%q want=%q", rows[1][0], "2026-02-15 10:01")
	}
	if rows[1][1] != "1.0" || rows[1][2] != "14.2" || rows[1][3] != "1.8" || rows[1][4] != "0.5" || rows[1][5] != "12.8" || rows[1][6] != "15.2" || rows[1][7] != "2.3" || rows[1][8] != "12.8" {
		t.Fatalf("older row averages mismatch: got=%v", rows[1])
	}
}

func TestMinuteTelemetryHistorySortAndLimit(t *testing.T) {
	history := newMinuteTelemetryHistory(32)
	snapshot := newEnergySnapshot()
	snapshot.HasInPV = true
	snapshot.InPVWatts = 10

	base := time.Date(2026, time.February, 15, 10, 0, 5, 0, time.Local)
	for i := 0; i < 3; i++ {
		snapshot.InPVWatts = float64(10 + i)
		history.AddSample(base.Add(time.Duration(i)*time.Minute), snapshot)
	}

	rows := buildMinuteTelemetryRows(history, minuteTableConfig{Rows: 2, NewestFirst: false})
	if len(rows) != 2 {
		t.Fatalf("row count mismatch: got=%d want=2", len(rows))
	}
	if rows[0][0] != "2026-02-15 10:00" || rows[1][0] != "2026-02-15 10:01" {
		t.Fatalf("oldest-first ordering mismatch: got=%v", rows)
	}
}

func TestMinuteTelemetryStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry_history.jsonl")
	store, err := newMinuteTelemetryStore(path)
	if err != nil {
		t.Fatalf("new minute telemetry store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	minuteA := minuteTelemetryBucket{
		MinuteStartUnix:       time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local).Unix(),
		SolarSumWatts:         1000,
		SolarSamples:          4,
		ACInSumWatts:          3600,
		ACInSamples:           4,
		ACOutSumWatts:         480,
		ACOutSamples:          4,
		DCOutSumWatts:         120,
		DCOutSamples:          4,
		BatteryChargeSumWatts: 3000,
		BatteryChargeSamples:  4,
	}
	minuteAUpdated := minuteA
	minuteAUpdated.SolarSumWatts = 1200
	minuteAUpdated.SolarSamples = 5

	minuteB := minuteTelemetryBucket{
		MinuteStartUnix:       time.Date(2026, time.February, 15, 10, 1, 0, 0, time.Local).Unix(),
		SolarSumWatts:         800,
		SolarSamples:          4,
		ACInSumWatts:          2400,
		ACInSamples:           4,
		ACOutSumWatts:         360,
		ACOutSamples:          4,
		DCOutSumWatts:         60,
		DCOutSamples:          4,
		BatteryChargeSumWatts: 2000,
		BatteryChargeSamples:  4,
	}

	if err := store.AppendBucket("SN-1", minuteA); err != nil {
		t.Fatalf("append minuteA: %v", err)
	}
	if err := store.AppendBucket("SN-1", minuteAUpdated); err != nil {
		t.Fatalf("append minuteAUpdated: %v", err)
	}
	if err := store.AppendBucket("SN-1", minuteB); err != nil {
		t.Fatalf("append minuteB: %v", err)
	}
	if err := store.AppendBucket("SN-2", minuteB); err != nil {
		t.Fatalf("append minuteB SN-2: %v", err)
	}

	loaded := newMinuteTelemetryHistory(32)
	loadedCount, err := store.LoadInto("SN-1", loaded)
	if err != nil {
		t.Fatalf("load history SN-1: %v", err)
	}
	if loadedCount != 2 {
		t.Fatalf("loadedCount mismatch: got=%d want=2", loadedCount)
	}
	if len(loaded.buckets) != 2 {
		t.Fatalf("loaded bucket count mismatch: got=%d want=2", len(loaded.buckets))
	}

	gotMinuteA, ok := loaded.Bucket(minuteA.MinuteStartUnix)
	if !ok {
		t.Fatalf("missing minuteA bucket")
	}
	if gotMinuteA.SolarSumWatts != minuteAUpdated.SolarSumWatts || gotMinuteA.SolarSamples != minuteAUpdated.SolarSamples {
		t.Fatalf("minuteA upsert mismatch: got=%+v want=%+v", gotMinuteA, minuteAUpdated)
	}

	other := newMinuteTelemetryHistory(32)
	otherCount, err := store.LoadInto("SN-2", other)
	if err != nil {
		t.Fatalf("load history SN-2: %v", err)
	}
	if otherCount != 1 || len(other.buckets) != 1 {
		t.Fatalf("SN-2 filtering mismatch: count=%d buckets=%d", otherCount, len(other.buckets))
	}
}

func TestMinuteTelemetryStoreLoadWindowFiltersOlderBuckets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry_history_window.jsonl")
	store, err := newMinuteTelemetryStore(path)
	if err != nil {
		t.Fatalf("new minute telemetry store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	oldMinute := time.Date(2026, time.February, 15, 9, 0, 0, 0, time.Local).Unix()
	newMinute := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local).Unix()

	if err := store.AppendBucket("SN-1", minuteTelemetryBucket{
		MinuteStartUnix: oldMinute,
		SolarSumWatts:   100,
		SolarSamples:    2,
	}); err != nil {
		t.Fatalf("append old minute: %v", err)
	}
	if err := store.AppendBucket("SN-1", minuteTelemetryBucket{
		MinuteStartUnix: newMinute,
		SolarSumWatts:   200,
		SolarSamples:    2,
	}); err != nil {
		t.Fatalf("append new minute: %v", err)
	}

	loaded := newMinuteTelemetryHistory(32)
	loadedCount, err := store.LoadIntoWindow("SN-1", loaded, newMinute)
	if err != nil {
		t.Fatalf("load history with window: %v", err)
	}
	if loadedCount != 1 {
		t.Fatalf("loadedCount mismatch: got=%d want=1", loadedCount)
	}
	if len(loaded.buckets) != 1 {
		t.Fatalf("loaded bucket count mismatch: got=%d want=1", len(loaded.buckets))
	}
	if _, ok := loaded.Bucket(oldMinute); ok {
		t.Fatalf("old bucket should be filtered out")
	}
	if _, ok := loaded.Bucket(newMinute); !ok {
		t.Fatalf("new bucket should be loaded")
	}
}

func TestEnergySnapshotDerivesDashboardStatusFlags(t *testing.T) {
	snapshot := newEnergySnapshot()

	invPayload := []byte(`{"moduleType":3,"needAck":0,"id":32470802,"time":17087742,"params":{"cfgAcEnabled":1},"version":"1.0","typeCode":"invStatus"}`)
	invEnvelope, invQuota, err := parseTelemetryPayload(invPayload)
	if err != nil {
		t.Fatalf("parse inv payload: %v", err)
	}
	snapshot.Update(invEnvelope, invQuota, nil, false, pdStatusSummary{}, false)

	pdPayload := []byte(`{"moduleType":1,"needAck":0,"id":8222822,"time":17089392,"params":{"dcOutState":0},"version":"1.0","typeCode":"pdStatus"}`)
	pdEnvelope, pdQuota, err := parseTelemetryPayload(pdPayload)
	if err != nil {
		t.Fatalf("parse pd payload: %v", err)
	}
	pd, hasPD := extractPDStatus(pdQuota)
	snapshot.Update(pdEnvelope, pdQuota, nil, false, pd, hasPD)

	dAddrPayload := []byte(`{"cmdId":21,"cmdFunc":254,"addr":"d_addr","param":{"evChgManualCtrl":false,"plugInInfoAcpRunState":0},"params":{}}`)
	dAddrEnvelope, dAddrQuota, err := parseTelemetryPayload(dAddrPayload)
	if err != nil {
		t.Fatalf("parse d_addr payload: %v", err)
	}
	snapshot.Update(dAddrEnvelope, dAddrQuota, nil, false, pdStatusSummary{}, false)

	derived := snapshot.derived()
	if derived.StatusACValue != "[x]" {
		t.Fatalf("ac status mismatch: got=%s want=[x]", derived.StatusACValue)
	}
	if derived.StatusDCValue != "[ ]" {
		t.Fatalf("dc status mismatch: got=%s want=[ ]", derived.StatusDCValue)
	}
	if derived.StatusEVValue != "[ ]" {
		t.Fatalf("ev status mismatch: got=%s want=[ ]", derived.StatusEVValue)
	}
}

func TestEnergySnapshotUpdatesStatusFromShowFlag(t *testing.T) {
	snapshot := newEnergySnapshot()

	offPayload := []byte(`{"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"showFlag":6428}}`)
	offEnvelope, offQuota, err := parseTelemetryPayload(offPayload)
	if err != nil {
		t.Fatalf("parse off payload: %v", err)
	}
	snapshot.Update(offEnvelope, offQuota, nil, false, pdStatusSummary{}, false)

	derived := snapshot.derived()
	if derived.StatusACValue != "[x]" {
		t.Fatalf("ac status mismatch at showFlag=6428: got=%s want=[x]", derived.StatusACValue)
	}
	if derived.StatusDCValue != "[ ]" {
		t.Fatalf("dc status mismatch at showFlag=6428: got=%s want=[ ]", derived.StatusDCValue)
	}

	onPayload := []byte(`{"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"showFlag":6430}}`)
	onEnvelope, onQuota, err := parseTelemetryPayload(onPayload)
	if err != nil {
		t.Fatalf("parse on payload: %v", err)
	}
	snapshot.Update(onEnvelope, onQuota, nil, false, pdStatusSummary{}, false)

	derived = snapshot.derived()
	if derived.StatusDCValue != "[x]" {
		t.Fatalf("dc status mismatch at showFlag=6430: got=%s want=[x]", derived.StatusDCValue)
	}
}

func TestEnergySnapshotMapsAppshowMetadata(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"bpNum":2,"showFlag":6428,"remainCombo":30,"fullCombo":100,"c20ChgMaxWatts":1800,"paraChgMaxWatts":7200}}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse appshow payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	derived := snapshot.derived()
	if derived.BatteryCount != "2" {
		t.Fatalf("battery count mismatch: got=%s want=2", derived.BatteryCount)
	}
	if derived.ShowFlagValue != "6428" {
		t.Fatalf("showFlag mismatch: got=%s want=6428", derived.ShowFlagValue)
	}
	if derived.ComboValue != "30/100" {
		t.Fatalf("combo mismatch: got=%s want=30/100", derived.ComboValue)
	}
	if derived.C20LimitValue != "1.80kW" {
		t.Fatalf("c20 limit mismatch: got=%s want=1.80kW", derived.C20LimitValue)
	}
	if derived.ParaLimitValue != "7.20kW" {
		t.Fatalf("para limit mismatch: got=%s want=7.20kW", derived.ParaLimitValue)
	}
}

func TestEnergySnapshotMapsSocGuardrailsFromDAddr(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"cmdId":21,"cmdFunc":254,"addr":"d_addr","params":{},"param":{"cmsMaxChgSoc":95,"cmsMinDsgSoc":5}}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse d_addr payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	derived := snapshot.derived()
	if derived.SocGuardrail != "5% .. 95%" {
		t.Fatalf("soc guardrail mismatch: got=%s want=5%% .. 95%%", derived.SocGuardrail)
	}
}

func TestEnergySnapshotMapsEMSVoltageWindowFromBackend(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"cmdId":2,"cmdFunc":2,"addr":"hs_yj751_pd_backend_addr","params":{"emsParaVolMin":101292,"emsParaVolMax":104292}}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse backend payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	derived := snapshot.derived()
	if derived.EMSWindowValue != "101.3V .. 104.3V" {
		t.Fatalf("ems window mismatch: got=%s want=101.3V .. 104.3V", derived.EMSWindowValue)
	}
}

func TestApplyDeviceQuotaToSnapshotSeedsStatusFromShowFlag(t *testing.T) {
	snapshot := newEnergySnapshot()
	quota := map[string]string{
		"hs_yj751_pd_appshow_addr.showFlag": "6428",
	}

	bootstrap := applyDeviceQuotaToSnapshot(snapshot, quota)
	if bootstrap.QuotaKeys != 1 {
		t.Fatalf("bootstrap quota keys mismatch: got=%d want=1", bootstrap.QuotaKeys)
	}
	derived := snapshot.derived()
	if derived.StatusACValue != "[x]" {
		t.Fatalf("bootstrap ac status mismatch: got=%s want=[x]", derived.StatusACValue)
	}
	if derived.StatusDCValue != "[ ]" {
		t.Fatalf("bootstrap dc status mismatch: got=%s want=[ ]", derived.StatusDCValue)
	}
}

func TestEnergySnapshotParsesACDCAndPVChannels(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":1,"needAck":0,"id":8229999,"time":17100000,"params":{"XT150Watts2":-100,"pv1ChargeWatts":60,"pv2ChargeWatts":40,"wattsInSum":100,"wattsOutSum":154,"invOutWatts":136,"usb1Watts":10,"typec1Watts":8},"version":"1.0","typeCode":"pdStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse pdStatus payload: %v", err)
	}
	pd, hasPD := extractPDStatus(quota)
	snapshot.Update(envelope, quota, nil, false, pd, hasPD)

	derived := snapshot.derived()
	if !snapshot.HasInPV || snapshot.InPVWatts != 100 {
		t.Fatalf("in_pv mismatch: has=%v value=%f", snapshot.HasInPV, snapshot.InPVWatts)
	}
	if !snapshot.HasInPVLow || snapshot.InPVLowWatts != 60 {
		t.Fatalf("in_pv_low mismatch: has=%v value=%f", snapshot.HasInPVLow, snapshot.InPVLowWatts)
	}
	if !snapshot.HasInPVHigh || snapshot.InPVHighWatts != 40 {
		t.Fatalf("in_pv_high mismatch: has=%v value=%f", snapshot.HasInPVHigh, snapshot.InPVHighWatts)
	}
	if !snapshot.HasXT150 || snapshot.XT150Watts != -100 {
		t.Fatalf("xt150 mismatch: has=%v value=%f", snapshot.HasXT150, snapshot.XT150Watts)
	}
	if !snapshot.HasOutAC || snapshot.OutACWatts != 136 {
		t.Fatalf("out_ac mismatch: has=%v value=%f", snapshot.HasOutAC, snapshot.OutACWatts)
	}
	if !snapshot.HasOutDC || snapshot.OutDCWatts != 18 {
		t.Fatalf("out_dc mismatch: has=%v value=%f", snapshot.HasOutDC, snapshot.OutDCWatts)
	}
	if derived.InPVValue != "100.0W" || derived.InPVLowValue != "60.0W" || derived.InPVHighValue != "40.0W" || derived.XT150InValue != "100.0W" || derived.XT150OutValue != "0.0W" || derived.OutACValue != "136.0W" || derived.OutDCValue != "18.0W" {
		t.Fatalf(
			"derived channel labels mismatch: in_pv=%s in_pv_low=%s in_pv_high=%s xt150_in=%s xt150_out=%s out_ac=%s out_dc=%s",
			derived.InPVValue,
			derived.InPVLowValue,
			derived.InPVHighValue,
			derived.XT150InValue,
			derived.XT150OutValue,
			derived.OutACValue,
			derived.OutDCValue,
		)
	}
}

func TestSumPVInputChannelsFromQuotaAvoidsAliasDoubleCount(t *testing.T) {
	quota := map[string]any{
		"inLvMpptPwr":                              190.0,
		"hs_yj751_pd_appshow_addr.inLvMpptPwr":    190.0,
		"powGetPvL":                                190.0,
		"d_addr.powGetPvL":                         190.0,
		"inHvMpptPwr":                              33.0,
		"hs_yj751_pd_appshow_addr.inHvMpptPwr":    33.0,
		"powGetPvH":                                33.0,
		"d_addr.powGetPvH":                         33.0,
	}

	low, hasLow, high, hasHigh := sumPVInputChannelsFromQuota(quota)
	if !hasLow || low != 190 {
		t.Fatalf("low pv mismatch: has=%v value=%f", hasLow, low)
	}
	if !hasHigh || high != 33 {
		t.Fatalf("high pv mismatch: has=%v value=%f", hasHigh, high)
	}
}

func TestEnergySnapshotDoesNotDoubleCountPVAliasKeys(t *testing.T) {
	snapshot := newEnergySnapshot()

	appshowPayload := []byte(`{"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"inLvMpptPwr":190,"wattsInSum":190}}`)
	appshowEnvelope, appshowQuota, err := parseTelemetryPayload(appshowPayload)
	if err != nil {
		t.Fatalf("parse appshow payload: %v", err)
	}
	snapshot.Update(appshowEnvelope, appshowQuota, nil, false, pdStatusSummary{}, false)
	derived := snapshot.derived()
	if derived.InPVLowValue != "190.0W" || derived.InPVValue != "190.0W" {
		t.Fatalf("appshow pv mismatch: low=%s total=%s", derived.InPVLowValue, derived.InPVValue)
	}

	dAddrPayload := []byte(`{"cmdId":21,"cmdFunc":254,"addr":"d_addr","param":{"powGetPvL":190},"params":{}}`)
	dAddrEnvelope, dAddrQuota, err := parseTelemetryPayload(dAddrPayload)
	if err != nil {
		t.Fatalf("parse d_addr payload: %v", err)
	}
	snapshot.Update(dAddrEnvelope, dAddrQuota, nil, false, pdStatusSummary{}, false)
	derived = snapshot.derived()
	if derived.InPVLowValue != "190.0W" || derived.InPVValue != "190.0W" {
		t.Fatalf("d_addr pv mismatch: low=%s total=%s", derived.InPVLowValue, derived.InPVValue)
	}
}

func TestEnergySnapshotParsesPVLowHighFromMPPTStatus(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":5,"needAck":0,"id":10297946,"time":17087042,"params":{"inWatts":23,"pv2InWatts":0,"inVol":35.8,"pv2InVol":0},"version":"1.0","typeCode":"mpptStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse mpptStatus payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	if !snapshot.HasInPVLow || snapshot.InPVLowWatts != 23 {
		t.Fatalf("in_pv_low mismatch: has=%v value=%f", snapshot.HasInPVLow, snapshot.InPVLowWatts)
	}
	if !snapshot.HasInPVHigh || snapshot.InPVHighWatts != 0 {
		t.Fatalf("in_pv_high mismatch: has=%v value=%f", snapshot.HasInPVHigh, snapshot.InPVHighWatts)
	}
	if !snapshot.HasInPV || snapshot.InPVWatts != 23 {
		t.Fatalf("in_pv total mismatch: has=%v value=%f", snapshot.HasInPV, snapshot.InPVWatts)
	}
}

func TestEnergySnapshotParsesScaledMPPTStatusUnits(t *testing.T) {
	snapshot := newEnergySnapshot()
	// Delta 2 Max sample reports voltage/current in mV/mA while inWatts is already in watts.
	payload := []byte(`{"moduleType":5,"needAck":0,"id":10462462,"time":60658642,"params":{"pv2InWatts":61,"inVol":17480,"pv2MpptTemp":59,"inAmp":1823,"outVol":52104,"outAmp":1646,"mpptTemp":58,"outWatts":85,"carOutAmp":21,"inWatts":31,"pv2InVol":43084,"pv2InAmp":1434},"version":"1.0","typeCode":"mpptStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse mpptStatus payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	if !snapshot.HasSolarLVVolts || snapshot.SolarLVVolts < 17.0 || snapshot.SolarLVVolts > 18.0 {
		t.Fatalf("lv volts normalization mismatch: has=%v value=%f", snapshot.HasSolarLVVolts, snapshot.SolarLVVolts)
	}
	if !snapshot.HasSolarLVAmp || snapshot.SolarLVAmp < 1.7 || snapshot.SolarLVAmp > 1.9 {
		t.Fatalf("lv amps normalization mismatch: has=%v value=%f", snapshot.HasSolarLVAmp, snapshot.SolarLVAmp)
	}
	if !snapshot.HasSolarHVVolts || snapshot.SolarHVVolts < 43.0 || snapshot.SolarHVVolts > 43.2 {
		t.Fatalf("hv volts normalization mismatch: has=%v value=%f", snapshot.HasSolarHVVolts, snapshot.SolarHVVolts)
	}
	if !snapshot.HasSolarHVAmp || snapshot.SolarHVAmp < 1.3 || snapshot.SolarHVAmp > 1.5 {
		t.Fatalf("hv amps normalization mismatch: has=%v value=%f", snapshot.HasSolarHVAmp, snapshot.SolarHVAmp)
	}

	derived := snapshot.derived()
	if derived.InPVLowValue != "31.9W" {
		t.Fatalf("in_pv_low mismatch: got=%s want=31.9W", derived.InPVLowValue)
	}
	if derived.InPVHighValue != "61.8W" {
		t.Fatalf("in_pv_high mismatch: got=%s want=61.8W", derived.InPVHighValue)
	}
	if derived.InPVValue != "93.6W" {
		t.Fatalf("in_pv total mismatch: got=%s want=93.6W", derived.InPVValue)
	}
}

func TestEnergySnapshotParsesSubThousandMPPTMilliVolts(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":5,"needAck":0,"id":10588604,"time":94061552,"params":{"inVol":785,"pv2InVol":1070},"version":"1.0","typeCode":"mpptStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse mpptStatus payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	if !snapshot.HasSolarLVVolts || snapshot.SolarLVVolts < 0.78 || snapshot.SolarLVVolts > 0.79 {
		t.Fatalf("lv volts normalization mismatch: has=%v value=%f", snapshot.HasSolarLVVolts, snapshot.SolarLVVolts)
	}
	if !snapshot.HasSolarHVVolts || snapshot.SolarHVVolts < 1.06 || snapshot.SolarHVVolts > 1.08 {
		t.Fatalf("hv volts normalization mismatch: has=%v value=%f", snapshot.HasSolarHVVolts, snapshot.SolarHVVolts)
	}
}

func TestEnergySnapshotRejectsAbsurdPVEstimateFromScaledValues(t *testing.T) {
	// Simulate a bad scaled frame that slips through without normalization.
	gotWatts, got := effectivePVInputWatts(false, 0, true, 17480, true, 1823)
	if got {
		t.Fatalf("expected absurd estimate to be rejected, got hasInput=true watts=%f", gotWatts)
	}
	if gotWatts != 0 {
		t.Fatalf("expected absurd estimate clamp to zero watts, got=%f", gotWatts)
	}
}

func TestEnergySnapshotInfersPVLowWattsFromVoltsAndAmps(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"cmdId":2,"cmdFunc":2,"addr":"hs_yj751_pd_backend_addr","params":{"inLvMpptVol":33.8,"inLvMpptAmp":0.78,"inLvMpptPwr":0}}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse backend payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	derived := snapshot.derived()
	if derived.InPVLowValue != "26.4W" {
		t.Fatalf("pv low watts mismatch: got=%s want=26.4W", derived.InPVLowValue)
	}
	if derived.InPVValue != "26.4W" {
		t.Fatalf("pv total watts mismatch: got=%s want=26.4W", derived.InPVValue)
	}
	if derived.PVLowVoltsValue != "33.8V" {
		t.Fatalf("pv low volts mismatch: got=%s want=33.8V", derived.PVLowVoltsValue)
	}
	if derived.PVLowAmpsValue != "0.78A" {
		t.Fatalf("pv low amps mismatch: got=%s want=0.78A", derived.PVLowAmpsValue)
	}
}

func TestMinuteTelemetryUsesDerivedPVWattsFromVoltsAndAmps(t *testing.T) {
	history := newMinuteTelemetryHistory(16)
	snapshot := newEnergySnapshot()
	snapshot.SolarLVVolts = 33.8
	snapshot.HasSolarLVVolts = true
	snapshot.SolarLVAmp = 0.78
	snapshot.HasSolarLVAmp = true

	at := time.Date(2026, time.February, 15, 10, 45, 0, 0, time.Local)
	history.AddSample(at, snapshot)

	rows := buildMinuteTelemetryRows(history, minuteTableConfig{Rows: 1, NewestFirst: true})
	if len(rows) != 1 {
		t.Fatalf("row count mismatch: got=%d want=1", len(rows))
	}
	if rows[0][1] != "0.4" {
		t.Fatalf("solar minute metric mismatch: got=%s want=0.4", rows[0][1])
	}
}

func TestEnergySnapshotDerivesIdleDrawAndSolarLockState(t *testing.T) {
	snapshot := newEnergySnapshot()

	appshowPayload := []byte(`{"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"outAcTtPwr":0,"outUsb1Pwr":0,"outUsb2Pwr":0}}`)
	appshowEnvelope, appshowQuota, err := parseTelemetryPayload(appshowPayload)
	if err != nil {
		t.Fatalf("parse appshow payload: %v", err)
	}
	snapshot.Update(appshowEnvelope, appshowQuota, nil, false, pdStatusSummary{}, false)

	backendPayload := []byte(`{"cmdId":2,"cmdFunc":2,"addr":"hs_yj751_pd_backend_addr","params":{"bmsInputWatts":0,"bmsOutputWatts":23,"batAmp":-0.1,"batVol":102.8,"inLvMpptVol":35.9,"inLvMpptAmp":0}}`)
	backendEnvelope, backendQuota, err := parseTelemetryPayload(backendPayload)
	if err != nil {
		t.Fatalf("parse backend payload: %v", err)
	}
	snapshot.Update(backendEnvelope, backendQuota, nil, false, pdStatusSummary{}, false)

	dAddrPayload := []byte(`{"cmdId":21,"cmdFunc":254,"addr":"d_addr","param":{"plugInInfoPvWeakSourceFlag":1,"plugInInfoPvLFlag":1},"params":{}}`)
	dAddrEnvelope, dAddrQuota, err := parseTelemetryPayload(dAddrPayload)
	if err != nil {
		t.Fatalf("parse d_addr payload: %v", err)
	}
	snapshot.Update(dAddrEnvelope, dAddrQuota, nil, false, pdStatusSummary{}, false)

	derived := snapshot.derived()
	if derived.BatteryOutValue != "23.0W" {
		t.Fatalf("battery out mismatch: got=%s want=23.0W", derived.BatteryOutValue)
	}
	if derived.IdleDrawValue != "23.0W" {
		t.Fatalf("idle draw mismatch: got=%s want=23.0W", derived.IdleDrawValue)
	}
	if derived.PVStateValue != "locked(35.9V)" {
		t.Fatalf("pv state mismatch: got=%s want=locked(35.9V)", derived.PVStateValue)
	}
	if derived.PVLowStateValue != "locked(35.9V)" {
		t.Fatalf("pv low state mismatch: got=%s want=locked(35.9V)", derived.PVLowStateValue)
	}
	if derived.PVLowVoltsValue != "35.9V" {
		t.Fatalf("pv low volts mismatch: got=%s want=35.9V", derived.PVLowVoltsValue)
	}
	if derived.PVHighStateValue != "n/a" {
		t.Fatalf("pv high state mismatch: got=%s want=n/a", derived.PVHighStateValue)
	}
}

func TestEnergySnapshotDerivesBatteryFlowFromPackTotals(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.Packs[1] = &packSnapshot{HasPower: true, PowerW: 93}
	snapshot.Packs[2] = &packSnapshot{HasPower: true, PowerW: 79}

	derived := snapshot.derived()
	if derived.BatteryInValue != "172.0W" {
		t.Fatalf("battery in mismatch: got=%s want=172.0W", derived.BatteryInValue)
	}
	if derived.BatteryOutValue != "0.0W" {
		t.Fatalf("battery out mismatch: got=%s want=0.0W", derived.BatteryOutValue)
	}
	if derived.BatteryNetValue != "172.0W" {
		t.Fatalf("battery net mismatch: got=%s want=172.0W", derived.BatteryNetValue)
	}
}

func TestEnergySnapshotFormatsXT150DirectionalValues(t *testing.T) {
	snapshot := newEnergySnapshot()

	chargingPayload := []byte(`{"moduleType":1,"needAck":0,"id":8235001,"time":20830000,"params":{"XT150Watts2":270},"version":"1.0","typeCode":"pdStatus"}`)
	chargingEnvelope, chargingQuota, err := parseTelemetryPayload(chargingPayload)
	if err != nil {
		t.Fatalf("parse charging payload: %v", err)
	}
	chargingPD, hasChargingPD := extractPDStatus(chargingQuota)
	snapshot.Update(chargingEnvelope, chargingQuota, nil, false, chargingPD, hasChargingPD)

	derived := snapshot.derived()
	if derived.XT150InValue != "0.0W" || derived.XT150OutValue != "270.0W" {
		t.Fatalf("charging xt150 direction mismatch: in=%s out=%s", derived.XT150InValue, derived.XT150OutValue)
	}

	dischargingPayload := []byte(`{"moduleType":1,"needAck":0,"id":8235002,"time":20830100,"params":{"XT150Watts2":-37},"version":"1.0","typeCode":"pdStatus"}`)
	dischargingEnvelope, dischargingQuota, err := parseTelemetryPayload(dischargingPayload)
	if err != nil {
		t.Fatalf("parse discharging payload: %v", err)
	}
	dischargingPD, hasDischargingPD := extractPDStatus(dischargingQuota)
	snapshot.Update(dischargingEnvelope, dischargingQuota, nil, false, dischargingPD, hasDischargingPD)

	derived = snapshot.derived()
	if derived.XT150InValue != "37.0W" || derived.XT150OutValue != "0.0W" {
		t.Fatalf("discharging xt150 direction mismatch: in=%s out=%s", derived.XT150InValue, derived.XT150OutValue)
	}
}

func TestEnergySnapshotDoesNotTreatXT150AsPVInput(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":1,"needAck":0,"id":8229998,"time":17099900,"params":{"XT150Watts2":-100,"wattsInSum":100,"wattsOutSum":137,"invOutWatts":136},"version":"1.0","typeCode":"pdStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse pdStatus payload: %v", err)
	}
	pd, hasPD := extractPDStatus(quota)
	snapshot.Update(envelope, quota, nil, false, pd, hasPD)

	if snapshot.HasInPV {
		t.Fatalf("expected in_pv to be unset for XT150-only input, got=%f", snapshot.InPVWatts)
	}
	if !snapshot.HasXT150 || snapshot.XT150Watts != -100 {
		t.Fatalf("expected xt150 battery-link flow -100W, got has=%v value=%f", snapshot.HasXT150, snapshot.XT150Watts)
	}
	if !snapshot.HasOutAC || snapshot.OutACWatts != 136 {
		t.Fatalf("out_ac mismatch: has=%v value=%f", snapshot.HasOutAC, snapshot.OutACWatts)
	}
	if !snapshot.HasOutDC || snapshot.OutDCWatts != 0 {
		t.Fatalf("expected low residual (<%0.1fW) to normalize to zero out_dc, got has=%v value=%f", dcResidualInferenceMinWatts, snapshot.HasOutDC, snapshot.OutDCWatts)
	}
}

func TestEnergySnapshotInfersDCOutputFromTotalsResidual(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":1,"needAck":0,"id":8231287,"time":19902612,"params":{"wattsOutSum":152,"invOutWatts":135,"XT150Watts2":-104,"icoBytes":[0,8,136,128,128,0,0,0,0,0,0,0,0,0]},"version":"1.0","typeCode":"pdStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse pdStatus payload: %v", err)
	}
	pd, hasPD := extractPDStatus(quota)
	snapshot.Update(envelope, quota, nil, false, pd, hasPD)

	if !snapshot.HasOutAC || snapshot.OutACWatts != 135 {
		t.Fatalf("out_ac mismatch: has=%v value=%f", snapshot.HasOutAC, snapshot.OutACWatts)
	}
	if !snapshot.HasOutDC || snapshot.OutDCWatts != 17 {
		t.Fatalf("out_dc mismatch: has=%v value=%f", snapshot.HasOutDC, snapshot.OutDCWatts)
	}
}

func TestEnergySnapshotSetsZeroDCOutputWhenDCStateReported(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":1,"needAck":0,"id":8231111,"time":17200000,"params":{"dcOutState":1,"XT150Watts2":-96,"wattsInSum":96,"wattsOutSum":138,"invOutWatts":138},"version":"1.0","typeCode":"pdStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse pdStatus payload: %v", err)
	}
	pd, hasPD := extractPDStatus(quota)
	snapshot.Update(envelope, quota, nil, false, pd, hasPD)

	if !snapshot.HasOutDC || snapshot.OutDCWatts != 0 {
		t.Fatalf("out_dc mismatch: has=%v value=%f", snapshot.HasOutDC, snapshot.OutDCWatts)
	}
}

func TestEnergySnapshotKeepsExplicitDCAndZeroPVAcrossSparseUpdates(t *testing.T) {
	snapshot := newEnergySnapshot()

	// Full pdStatus frame with explicit DC channel and explicit zero PV channels.
	fullPayload := []byte(`{"moduleType":1,"needAck":0,"id":8234001,"time":20821872,"params":{"XT150Watts2":270,"wattsInSum":919,"invOutWatts":119,"wattsOutSum":405,"typec2Watts":11,"pv1ChargeWatts":0,"pv2ChargeWatts":0},"version":"1.0","typeCode":"pdStatus"}`)
	fullEnvelope, fullQuota, err := parseTelemetryPayload(fullPayload)
	if err != nil {
		t.Fatalf("parse full pdStatus payload: %v", err)
	}
	fullPD, hasFullPD := extractPDStatus(fullQuota)
	snapshot.Update(fullEnvelope, fullQuota, nil, false, fullPD, hasFullPD)

	if !snapshot.HasOutDC || snapshot.OutDCWatts != 11 {
		t.Fatalf("explicit out_dc mismatch: has=%v value=%f", snapshot.HasOutDC, snapshot.OutDCWatts)
	}
	if !snapshot.HasInPV || snapshot.InPVWatts != 0 {
		t.Fatalf("explicit zero in_pv mismatch: has=%v value=%f", snapshot.HasInPV, snapshot.InPVWatts)
	}

	// Sparse pdStatus with totals only must not overwrite explicit DC value.
	sparsePayload := []byte(`{"moduleType":1,"needAck":0,"id":8234002,"time":20822192,"params":{"wattsOutSum":409,"invOutWatts":122},"version":"1.0","typeCode":"pdStatus"}`)
	sparseEnvelope, sparseQuota, err := parseTelemetryPayload(sparsePayload)
	if err != nil {
		t.Fatalf("parse sparse pdStatus payload: %v", err)
	}
	sparsePD, hasSparsePD := extractPDStatus(sparseQuota)
	snapshot.Update(sparseEnvelope, sparseQuota, nil, false, sparsePD, hasSparsePD)

	if !snapshot.HasOutDC || snapshot.OutDCWatts != 11 {
		t.Fatalf("sparse update must keep explicit out_dc=11W, got has=%v value=%f", snapshot.HasOutDC, snapshot.OutDCWatts)
	}

	// AC input update must not backfill fake PV residual when PV keys were explicitly 0.
	invPayload := []byte(`{"moduleType":3,"needAck":0,"id":32515312,"time":20822812,"params":{"inputWatts":900,"outputWatts":112},"version":"1.0","typeCode":"invStatus"}`)
	invEnvelope, invQuota, err := parseTelemetryPayload(invPayload)
	if err != nil {
		t.Fatalf("parse invStatus payload: %v", err)
	}
	snapshot.Update(invEnvelope, invQuota, nil, false, pdStatusSummary{}, false)

	if !snapshot.HasInAC || snapshot.InACWatts != 900 {
		t.Fatalf("in_ac mismatch: has=%v value=%f", snapshot.HasInAC, snapshot.InACWatts)
	}
	if !snapshot.HasInPV || snapshot.InPVWatts != 0 {
		t.Fatalf("in_pv should remain explicit 0W, got has=%v value=%f", snapshot.HasInPV, snapshot.InPVWatts)
	}

	// Explicit zero AC input should clear previous non-zero value (no stale lag).
	invZeroPayload := []byte(`{"moduleType":3,"needAck":0,"id":32515313,"time":20822912,"params":{"inputWatts":0,"outputWatts":0},"version":"1.0","typeCode":"invStatus"}`)
	invZeroEnvelope, invZeroQuota, err := parseTelemetryPayload(invZeroPayload)
	if err != nil {
		t.Fatalf("parse invStatus zero payload: %v", err)
	}
	snapshot.Update(invZeroEnvelope, invZeroQuota, nil, false, pdStatusSummary{}, false)
	if !snapshot.HasInAC || snapshot.InACWatts != 0 {
		t.Fatalf("in_ac should be explicitly zero after zero inputWatts update, got has=%v value=%f", snapshot.HasInAC, snapshot.InACWatts)
	}
}

func TestEnergySnapshotInfersResidualDCAfterXT150Deduction(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":1,"needAck":0,"id":8234003,"time":20825182,"params":{"XT150Watts2":270,"wattsOutSum":400,"invOutWatts":112},"version":"1.0","typeCode":"pdStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse pdStatus payload: %v", err)
	}
	pd, hasPD := extractPDStatus(quota)
	snapshot.Update(envelope, quota, nil, false, pd, hasPD)

	if !snapshot.HasOutDC || snapshot.OutDCWatts != 18 {
		t.Fatalf("out_dc mismatch after xt150 deduction: has=%v value=%f", snapshot.HasOutDC, snapshot.OutDCWatts)
	}
}

func TestEnergySnapshotBpInfoZeroPowerDoesNotOverwriteActivePackPower(t *testing.T) {
	snapshot := newEnergySnapshot()

	bmsPayload := []byte(`{"moduleType":4,"needAck":0,"id":71300001,"time":17100000,"params":{"outputWatts":81,"soc":7},"version":"1.0","typeCode":"bmsSlaveStatus_1"}`)
	bmsEnvelope, bmsQuota, err := parseTelemetryPayload(bmsPayload)
	if err != nil {
		t.Fatalf("parse bms payload: %v", err)
	}
	snapshot.Update(bmsEnvelope, bmsQuota, nil, false, pdStatusSummary{}, false)

	pack1 := snapshot.Packs[1]
	if pack1 == nil || !pack1.HasPower || pack1.PowerW != -81 {
		t.Fatalf("expected bp1=-81W after bms update, got pack=%+v", pack1)
	}

	bpInfoPayload := []byte(`{"cmdId":4,"cmdFunc":2,"addr":"hs_yj751_pd_bp_addr","param":{"bpInfo":[{"bpNo":1,"bpSoc":7,"bpPwr":0,"remainTime":1759}]},"params":{}}`)
	bpEnvelope, bpQuota, err := parseTelemetryPayload(bpInfoPayload)
	if err != nil {
		t.Fatalf("parse bpInfo payload: %v", err)
	}
	snapshot.Update(bpEnvelope, bpQuota, nil, false, pdStatusSummary{}, false)

	pack1 = snapshot.Packs[1]
	if pack1 == nil || !pack1.HasPower || pack1.PowerW != -81 {
		t.Fatalf("bpInfo zero-power should not overwrite non-zero pack power, got pack=%+v", pack1)
	}
}

func TestEnergySnapshotDoesNotOverwriteGlobalRemainWithBmsSentinel(t *testing.T) {
	snapshot := newEnergySnapshot()

	pdPayload := []byte(`{"moduleType":1,"needAck":0,"id":8225000,"time":17100000,"params":{"remainTime":447,"soc":8},"version":"1.0","typeCode":"pdStatus"}`)
	pdEnvelope, pdQuota, err := parseTelemetryPayload(pdPayload)
	if err != nil {
		t.Fatalf("parse pd payload: %v", err)
	}
	pd, hasPD := extractPDStatus(pdQuota)
	snapshot.Update(pdEnvelope, pdQuota, nil, false, pd, hasPD)
	if !snapshot.HasRemainTime || snapshot.RemainTimeRaw != 447 {
		t.Fatalf("expected global remain=447 after pdStatus, got has=%v remain=%d", snapshot.HasRemainTime, snapshot.RemainTimeRaw)
	}

	bmsPayload := []byte(`{"cmdId":28,"cmdFunc":3,"addr":"hs_yj751_bms_slave_addr_1","params":{"remainTime":143999,"outputWatts":11,"soc":7}}`)
	bmsEnvelope, bmsQuota, err := parseTelemetryPayload(bmsPayload)
	if err != nil {
		t.Fatalf("parse bms payload: %v", err)
	}
	snapshot.Update(bmsEnvelope, bmsQuota, nil, false, pdStatusSummary{}, false)

	if snapshot.RemainTimeRaw != 447 {
		t.Fatalf("bms sentinel remain must not overwrite global remain, got=%d", snapshot.RemainTimeRaw)
	}
	if pack1 := snapshot.Packs[1]; pack1 != nil && pack1.RemainTimeRaw >= 120000 {
		t.Fatalf("pack remain sentinel must be ignored, got=%d", pack1.RemainTimeRaw)
	}
}

func TestEnergySnapshotIgnoresBpInfoSentinelRemain(t *testing.T) {
	snapshot := newEnergySnapshot()

	validPayload := []byte(`{"moduleType":4,"needAck":0,"id":71300001,"time":17100000,"params":{"outputWatts":81,"remainTime":315,"soc":7},"version":"1.0","typeCode":"bmsSlaveStatus_1"}`)
	validEnvelope, validQuota, err := parseTelemetryPayload(validPayload)
	if err != nil {
		t.Fatalf("parse valid payload: %v", err)
	}
	snapshot.Update(validEnvelope, validQuota, nil, false, pdStatusSummary{}, false)

	pack1 := snapshot.Packs[1]
	if pack1 == nil || pack1.RemainTimeRaw != 315 {
		t.Fatalf("expected initial pack remain=315, got pack=%+v", pack1)
	}

	bpInfoPayload := []byte(`{"cmdId":4,"cmdFunc":2,"addr":"hs_yj751_pd_bp_addr","param":{"bpInfo":[{"bpNo":1,"bpSoc":7,"bpPwr":-12,"remainTime":143999}]},"params":{}}`)
	bpEnvelope, bpQuota, err := parseTelemetryPayload(bpInfoPayload)
	if err != nil {
		t.Fatalf("parse bpInfo payload: %v", err)
	}
	snapshot.Update(bpEnvelope, bpQuota, nil, false, pdStatusSummary{}, false)

	pack1 = snapshot.Packs[1]
	if pack1 == nil {
		t.Fatal("expected bp1 pack")
	}
	if pack1.RemainTimeRaw != 315 {
		t.Fatalf("bpInfo sentinel remain must not overwrite pack remain, got=%d", pack1.RemainTimeRaw)
	}
}

func TestEnergySnapshotRemainPrefersDischargeRemainWhenDischarging(t *testing.T) {
	snapshot := newEnergySnapshot()

	payload := []byte(`{"moduleType":1,"needAck":0,"id":900001,"time":17100000,"params":{"wattsInSum":0,"wattsOutSum":112,"remainTime":999,"chgRemainTime":5999,"dsgRemainTime":455},"version":"1.0","typeCode":"pdStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	pd, hasPD := extractPDStatus(quota)
	snapshot.Update(envelope, quota, nil, false, pd, hasPD)

	derived := snapshot.derived()
	if derived.SystemStateValue != "discharging" {
		t.Fatalf("system state mismatch: got=%s want=discharging", derived.SystemStateValue)
	}
	if derived.RemainValue != "discharging: 455min (~7h 35m)" {
		t.Fatalf("remain mismatch: got=%s want=%s", derived.RemainValue, "discharging: 455min (~7h 35m)")
	}
}

func TestEnergySnapshotRemainPrefersChargeRemainWhenCharging(t *testing.T) {
	snapshot := newEnergySnapshot()

	payload := []byte(`{"moduleType":1,"needAck":0,"id":900002,"time":17100001,"params":{"wattsInSum":902,"wattsOutSum":102,"remainTime":888,"chgRemainTime":320,"dsgRemainTime":5100},"version":"1.0","typeCode":"pdStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	pd, hasPD := extractPDStatus(quota)
	snapshot.Update(envelope, quota, nil, false, pd, hasPD)

	derived := snapshot.derived()
	if derived.SystemStateValue != "charging" {
		t.Fatalf("system state mismatch: got=%s want=charging", derived.SystemStateValue)
	}
	if derived.RemainValue != "charging: 320min (~5h 20m)" {
		t.Fatalf("remain mismatch: got=%s want=%s", derived.RemainValue, "charging: 320min (~5h 20m)")
	}
}

func TestEnergySnapshotStateSmoothingPrefersPackDischargeDirection(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.configureStateSmoothing(6)
	snapshot.Packs[1] = &packSnapshot{PowerW: -22.1, HasPower: true}
	snapshot.Packs[2] = &packSnapshot{PowerW: -29.0, HasPower: true}
	for i := 0; i < 6; i++ {
		snapshot.pushStateSmoothingSample()
	}

	// Simulate a stale frame where aggregate in/out briefly looks like charging.
	state := snapshot.detectSystemState(58, true, 29, true, 0, 0)
	if state != systemStateDischarging {
		t.Fatalf("state mismatch with discharge smoothing: got=%s want=%s", state, systemStateDischarging)
	}

	// Strong explicit pack charging should still override smoothed discharge trend.
	state = snapshot.detectSystemState(10, true, 90, true, 70, 0)
	if state != systemStateCharging {
		t.Fatalf("state mismatch with explicit pack charge: got=%s want=%s", state, systemStateCharging)
	}
}

func TestEnergySnapshotETAEstimatesRow(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 50
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5
	snapshot.HasBatteryIn = true
	snapshot.BatteryInWatts = 200
	snapshot.HasBatteryOut = true
	snapshot.BatteryOutWatts = 0
	snapshot.HasWattsIn = true
	snapshot.WattsIn = 320
	snapshot.HasWattsOut = true
	snapshot.WattsOut = 120

	derived := snapshot.derived()
	if derived.SystemStateValue != "charging" {
		t.Fatalf("system state mismatch: got=%s want=charging", derived.SystemStateValue)
	}
	if derived.EstimateChargeValue != "540min (~9h 0m)" {
		t.Fatalf("charge eta mismatch: got=%s want=%s", derived.EstimateChargeValue, "540min (~9h 0m)")
	}
	if derived.EstimateActiveValue != "540min (~9h 0m)" {
		t.Fatalf("active eta mismatch: got=%s want=%s", derived.EstimateActiveValue, "540min (~9h 0m)")
	}
	if derived.EstimatePowerValue != "power: chg@200.0W" {
		t.Fatalf("estimate power mismatch: got=%s want=%s", derived.EstimatePowerValue, "power: chg@200.0W")
	}
	if derived.EstimateConfidenceValue != "0.96 (high)" {
		t.Fatalf("estimate confidence mismatch: got=%s want=%s", derived.EstimateConfidenceValue, "0.96 (high)")
	}

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	for _, expected := range []string{"estimates", "charge: 540min (~9h 0m)", "active: 540min (~9h 0m)", "conf: 0.96 (high)", "heuristic"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("dashboard missing %q in estimates row; output=%q", expected, output)
		}
	}
}

func TestEstimateBatteryETAsML(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 50
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5

	history := newMinuteTelemetryHistory(16)
	base := time.Date(2026, time.February, 15, 10, 0, 0, 0, time.Local)
	for i := 0; i < 6; i++ {
		snapshot.HasInPV = true
		snapshot.InPVWatts = 200 + float64(i)
		snapshot.HasInAC = false
		snapshot.HasOutAC = true
		snapshot.OutACWatts = 45 + float64(i%2)
		snapshot.HasOutDC = true
		snapshot.OutDCWatts = 8
		history.AddSample(base.Add(time.Duration(i)*time.Minute), snapshot)
	}

	estimates := estimateBatteryETAsML(snapshot, history, systemStateCharging)
	if estimates.ChargeValue == "n/a" {
		t.Fatalf("ml charge eta should be available, got=%s", estimates.ChargeValue)
	}
	if !strings.Contains(estimates.PowerValue, "ewma+trend") {
		t.Fatalf("ml power label mismatch: got=%s", estimates.PowerValue)
	}
	if estimates.ConfidenceValue == "n/a" {
		t.Fatalf("ml confidence should be available, got=%s", estimates.ConfidenceValue)
	}
}

func TestSelectTopStateValueUsesDeviceUntilMLReady(t *testing.T) {
	deviceState := "charging: 320min (~5h 20m)"
	ml := batteryETAEstimates{
		ActiveValue:     "120min (~2h 0m)",
		PowerValue:      "power: chg@180.0W",
		ConfidenceValue: "0.95 (high)",
	}
	heuristic := batteryETAEstimates{
		ActiveValue:     "540min (~9h 0m)",
		ConfidenceValue: "0.96 (high)",
	}

	got := selectTopStateValue(deviceState, systemStateCharging, ml, heuristic)
	if got != deviceState {
		t.Fatalf("top state should use device until ML is ready: got=%q want=%q", got, deviceState)
	}
}

func TestSelectTopStateValueUsesMLWhenReadyAndHighConfidence(t *testing.T) {
	deviceState := "charging: 320min (~5h 20m)"
	ml := batteryETAEstimates{
		ActiveValue:     "120min (~2h 0m)",
		PowerValue:      "power: chg@180.0W (ewma+trend)",
		ConfidenceValue: "0.92 (high)",
	}
	heuristic := batteryETAEstimates{
		ActiveValue:     "540min (~9h 0m)",
		ConfidenceValue: "0.96 (high)",
	}

	got := selectTopStateValue(deviceState, systemStateCharging, ml, heuristic)
	want := "charging: 120min (~2h 0m)"
	if got != want {
		t.Fatalf("top state should use ML when ready and high confidence: got=%q want=%q", got, want)
	}
}

func TestSelectTopStateValueUsesHeuristicWhenMLNotHigh(t *testing.T) {
	deviceState := "discharging: 455min (~7h 35m)"
	ml := batteryETAEstimates{
		ActiveValue:     "300min (~5h 0m)",
		PowerValue:      "power: dsg@250.0W (ewma+trend)",
		ConfidenceValue: "0.68 (medium)",
	}
	heuristic := batteryETAEstimates{
		ActiveValue:     "360min (~6h 0m)",
		ConfidenceValue: "0.85 (high)",
	}

	got := selectTopStateValue(deviceState, systemStateDischarging, ml, heuristic)
	want := "discharging: 360min (~6h 0m)"
	if got != want {
		t.Fatalf("top state should use heuristic when ML is not high confidence: got=%q want=%q", got, want)
	}
}

func TestSelectTopStateValueUsesDeviceWhenBothLowConfidence(t *testing.T) {
	deviceState := "discharging: 455min (~7h 35m)"
	ml := batteryETAEstimates{
		ActiveValue:     "300min (~5h 0m)",
		PowerValue:      "power: dsg@250.0W (ewma+trend)",
		ConfidenceValue: "0.40 (low)",
	}
	heuristic := batteryETAEstimates{
		ActiveValue:     "360min (~6h 0m)",
		ConfidenceValue: "0.55 (low)",
	}

	got := selectTopStateValue(deviceState, systemStateDischarging, ml, heuristic)
	if got != deviceState {
		t.Fatalf("top state should use device when both ML and heuristic confidence are low: got=%q want=%q", got, deviceState)
	}
}

func TestTopStateDisplayIcon(t *testing.T) {
	tests := []struct {
		name  string
		state systemStateKind
		value string
		want  string
	}{
		{name: "charging", state: systemStateCharging, value: "charging: 1", want: "⚡"},
		{name: "discharging", state: systemStateDischarging, value: "discharging: 1", want: "↓"},
		{name: "idle", state: systemStateIdle, value: "idle: 1", want: "⏸"},
		{name: "infer charging", state: systemStateUnknown, value: "charging: 1", want: "⚡"},
		{name: "infer discharging", state: systemStateUnknown, value: "discharging: 1", want: "↓"},
		{name: "infer idle", state: systemStateUnknown, value: "idle: 1", want: "⏸"},
		{name: "unknown", state: systemStateUnknown, value: "n/a", want: ""},
	}

	for _, tc := range tests {
		got := topStateDisplayIcon(tc.state, tc.value)
		if got != tc.want {
			t.Fatalf("%s: icon mismatch got=%q want=%q", tc.name, got, tc.want)
		}
	}
}

func TestSanitizeStateColumnValue(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "⚡ charging: 320min (~5h 20m)", want: "charging: 320min (~5h 20m)"},
		{in: "↓ discharging: 455min (~7h 35m)", want: "discharging: 455min (~7h 35m)"},
		{in: "⏸ idle: 30min (~30m)", want: "idle: 30min (~30m)"},
		{in: "charging: 120min (~2h 0m)", want: "charging: 120min (~2h 0m)"},
	}

	for _, tc := range tests {
		got := sanitizeStateColumnValue(tc.in)
		if got != tc.want {
			t.Fatalf("sanitize state mismatch: in=%q got=%q want=%q", tc.in, got, tc.want)
		}
	}
}

func TestEnergySnapshotIgnoresUnrealisticBatteryHintWhenNetKnown(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasWattsIn = true
	snapshot.WattsIn = 300
	snapshot.HasWattsOut = true
	snapshot.WattsOut = 100
	snapshot.HasBatteryIn = true
	snapshot.BatteryInWatts = 9210.9
	snapshot.HasBatteryOut = true
	snapshot.BatteryOutWatts = 0
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 50
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5

	derived := snapshot.derived()
	if derived.BatteryInValue != "200.0W" {
		t.Fatalf("battery in should follow net power, got=%s want=%s", derived.BatteryInValue, "200.0W")
	}
	if derived.EstimatePowerValue != "power: chg@200.0W" {
		t.Fatalf("estimate power should follow net power, got=%s want=%s", derived.EstimatePowerValue, "power: chg@200.0W")
	}
}

func TestEnergySnapshotRejectsUnrealisticBatteryHintWithoutTotals(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasBatteryIn = true
	snapshot.BatteryInWatts = 9210.9
	snapshot.HasBatteryOut = true
	snapshot.BatteryOutWatts = 0
	snapshot.HasC20ChgMax = true
	snapshot.C20ChgMaxWatts = 1800
	snapshot.HasFullEnergy = true
	snapshot.FullEnergyWh = 4000
	snapshot.HasDeviceSOC = true
	snapshot.DeviceSOC = 50
	snapshot.HasMaxChargeSOC = true
	snapshot.MaxChargeSOC = 95
	snapshot.HasMinDischarge = true
	snapshot.MinDischargeSOC = 5

	derived := snapshot.derived()
	if derived.BatteryInValue != "n/a" {
		t.Fatalf("battery in should be rejected as unrealistic hint, got=%s want=n/a", derived.BatteryInValue)
	}
	if derived.EstimatePowerValue != "power: n/a" {
		t.Fatalf("estimate power should reject unrealistic hint, got=%s want=power: n/a", derived.EstimatePowerValue)
	}
}

func TestEnergySnapshotMapsPackPreconditioningFromPTCState(t *testing.T) {
	snapshot := newEnergySnapshot()

	payload := []byte(`{"moduleType":4,"needAck":0,"id":71311111,"time":17100000,"params":{"ptcMosState":1,"ptcHeatingEvent":6,"packSn":"PACK_B"},"version":"1.0","typeCode":"bmsSlaveStatus_2"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse preconditioning payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	pack := snapshot.Packs[2]
	if pack == nil {
		t.Fatal("expected bp2 pack")
	}
	if !pack.HasPreconditioning || !pack.PreconditioningOn {
		t.Fatalf("expected bp2 preconditioning on, got has=%v on=%v", pack.HasPreconditioning, pack.PreconditioningOn)
	}
	if !pack.HasPreconditioningState || pack.PreconditioningStateRaw != 1 {
		t.Fatalf("expected bp2 ptc state=1, got has=%v value=%d", pack.HasPreconditioningState, pack.PreconditioningStateRaw)
	}
	if !pack.HasPreconditioningEvent || pack.PreconditioningEventRaw != 6 {
		t.Fatalf("expected bp2 ptc event=6, got has=%v value=%d", pack.HasPreconditioningEvent, pack.PreconditioningEventRaw)
	}
	if got := formatPackPreconditioning(pack); got != "[x]" {
		t.Fatalf("preconditioning format mismatch: got=%q want=%q", got, "[x]")
	}
}

func TestEnergySnapshotMapsPackPreconditioningFallbackFromHeatTime(t *testing.T) {
	snapshot := newEnergySnapshot()

	payload := []byte(`{"cmdId":4,"cmdFunc":2,"addr":"hs_yj751_pd_bp_addr","param":{"bpInfo":[{"bpNo":1,"bpSoc":7,"heatTime":12,"bpPwr":0}]},"params":{}}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse bpInfo payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	pack := snapshot.Packs[1]
	if pack == nil {
		t.Fatal("expected bp1 pack")
	}
	if !pack.HasPreconditioningHeat || pack.PreconditioningHeatTime != 12 {
		t.Fatalf("expected heatTime=12, got has=%v value=%d", pack.HasPreconditioningHeat, pack.PreconditioningHeatTime)
	}
	if !pack.HasPreconditioning || !pack.PreconditioningOn {
		t.Fatalf("expected preconditioning on from heatTime, got has=%v on=%v", pack.HasPreconditioning, pack.PreconditioningOn)
	}
	if got := formatPackPreconditioning(pack); got != "[x]" {
		t.Fatalf("preconditioning format mismatch: got=%q want=%q", got, "[x]")
	}
}

func TestEnergySnapshotTurnsPreconditioningOffWhenHeatTimeReturnsZero(t *testing.T) {
	snapshot := newEnergySnapshot()

	onPayload := []byte(`{"cmdId":4,"cmdFunc":2,"addr":"hs_yj751_pd_bp_addr","param":{"bpInfo":[{"bpNo":1,"heatTime":9,"bpPwr":0}]},"params":{}}`)
	onEnvelope, onQuota, err := parseTelemetryPayload(onPayload)
	if err != nil {
		t.Fatalf("parse preconditioning on payload: %v", err)
	}
	snapshot.Update(onEnvelope, onQuota, nil, false, pdStatusSummary{}, false)

	pack := snapshot.Packs[1]
	if pack == nil || !pack.HasPreconditioning || !pack.PreconditioningOn {
		t.Fatalf("expected preconditioning on after heatTime=9, got pack=%+v", pack)
	}

	offPayload := []byte(`{"cmdId":4,"cmdFunc":2,"addr":"hs_yj751_pd_bp_addr","param":{"bpInfo":[{"bpNo":1,"heatTime":0,"bpPwr":0}]},"params":{}}`)
	offEnvelope, offQuota, err := parseTelemetryPayload(offPayload)
	if err != nil {
		t.Fatalf("parse preconditioning off payload: %v", err)
	}
	snapshot.Update(offEnvelope, offQuota, nil, false, pdStatusSummary{}, false)

	pack = snapshot.Packs[1]
	if pack == nil || !pack.HasPreconditioning || pack.PreconditioningOn {
		t.Fatalf("expected preconditioning off after heatTime=0, got pack=%+v", pack)
	}
	if got := formatPackPreconditioning(pack); got != "[ ]" {
		t.Fatalf("preconditioning format mismatch after off: got=%q want=%q", got, "[ ]")
	}
}

func TestEnergySnapshotOverallPreconditioningUsesPTCEvent(t *testing.T) {
	snapshot := newEnergySnapshot()

	// Observed DPU case: event is active while mos state remains 0.
	payload := []byte(`{"cmdId":28,"cmdFunc":3,"addr":"hs_yj751_bms_slave_addr_1","params":{"ptcMosState":0,"ptcHeatingEvent":7,"packSn":"Y716ZA1BBHBN0478"}}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	pack1 := snapshot.Packs[1]
	if pack1 == nil {
		t.Fatal("expected bp1 pack")
	}
	if !pack1.HasPreconditioning || !pack1.PreconditioningOn {
		t.Fatalf("expected bp1 preconditioning on via event, got has=%v on=%v", pack1.HasPreconditioning, pack1.PreconditioningOn)
	}
	if !pack1.HasPreconditioningEvent || pack1.PreconditioningEventRaw != 7 {
		t.Fatalf("expected ptc event=7, got has=%v value=%d", pack1.HasPreconditioningEvent, pack1.PreconditioningEventRaw)
	}
	derived := snapshot.derived()
	if derived.StatusPrecondValue != "[x]" {
		t.Fatalf("overall preconditioning status mismatch: got=%s want=[x]", derived.StatusPrecondValue)
	}
}

func TestEnergySnapshotMapsBmsSlaveFramesByPackSerial(t *testing.T) {
	snapshot := newEnergySnapshot()

	bootstrapPack1 := telemetryEnvelope{TypeCode: "bmsSlaveStatus_1"}
	bootstrapPack1Quota := map[string]any{"packSn": "PACK_A", "soc": 7, "outputWatts": 50}
	snapshot.Update(bootstrapPack1, bootstrapPack1Quota, nil, false, pdStatusSummary{}, false)

	bootstrapPack2 := telemetryEnvelope{TypeCode: "bmsSlaveStatus_2"}
	bootstrapPack2Quota := map[string]any{"packSn": "PACK_B", "soc": 8, "outputWatts": 60}
	snapshot.Update(bootstrapPack2, bootstrapPack2Quota, nil, false, pdStatusSummary{}, false)

	if p := snapshot.Packs[1]; p == nil || p.PowerW != -50 {
		t.Fatalf("bootstrap bp1 mismatch: %+v", p)
	}
	if p := snapshot.Packs[2]; p == nil || p.PowerW != -60 {
		t.Fatalf("bootstrap bp2 mismatch: %+v", p)
	}

	// Live stream can report addr "..._1" for either physical battery; serial identifies the actual pack.
	runtimePayload := []byte(`{"cmdId":28,"cmdFunc":3,"addr":"hs_yj751_bms_slave_addr_1","params":{"packSn":"PACK_B","soc":9,"outputWatts":13,"maxVolDiff":7}}`)
	runtimeEnvelope, runtimeQuota, err := parseTelemetryPayload(runtimePayload)
	if err != nil {
		t.Fatalf("parse runtime payload: %v", err)
	}
	snapshot.Update(runtimeEnvelope, runtimeQuota, nil, false, pdStatusSummary{}, false)

	if p := snapshot.Packs[1]; p == nil || p.PowerW != -50 {
		t.Fatalf("bp1 should remain unchanged, got=%+v", p)
	}
	pack2 := snapshot.Packs[2]
	if pack2 == nil || pack2.PowerW != -13 || !pack2.HasMaxVolDiff || pack2.MaxVolDiff != 7 {
		t.Fatalf("runtime update should target bp2 by serial, got=%+v", pack2)
	}
}

func TestReconnectAttemptStateBackoffAndReset(t *testing.T) {
	state := newReconnectAttemptState(500*time.Millisecond, 2*time.Second)
	if state.currentAttempt() != 1 {
		t.Fatalf("initial attempt mismatch: got=%d want=1", state.currentAttempt())
	}

	attempt1, wait1 := state.registerFailure(0.25)
	if attempt1 != 1 {
		t.Fatalf("attempt1 mismatch: got=%d want=1", attempt1)
	}
	if wait1 <= 0 {
		t.Fatalf("wait1 must be positive: got=%s", wait1)
	}
	if state.failureCount != 1 {
		t.Fatalf("failure count mismatch after first failure: got=%d want=1", state.failureCount)
	}
	if state.currentBackoff != 1*time.Second {
		t.Fatalf("backoff mismatch after first failure: got=%s want=1s", state.currentBackoff)
	}

	attempt2, wait2 := state.registerFailure(0.25)
	if attempt2 != 2 {
		t.Fatalf("attempt2 mismatch: got=%d want=2", attempt2)
	}
	if wait2 <= 0 {
		t.Fatalf("wait2 must be positive: got=%s", wait2)
	}
	if state.failureCount != 2 {
		t.Fatalf("failure count mismatch after second failure: got=%d want=2", state.failureCount)
	}
	if state.currentBackoff != 2*time.Second {
		t.Fatalf("backoff mismatch after second failure: got=%s want=2s", state.currentBackoff)
	}

	attempt3, _ := state.registerFailure(0.25)
	if attempt3 != 3 {
		t.Fatalf("attempt3 mismatch: got=%d want=3", attempt3)
	}
	if state.currentBackoff != 2*time.Second {
		t.Fatalf("backoff should clamp at max=2s: got=%s", state.currentBackoff)
	}

	state.reset()
	if state.failureCount != 0 {
		t.Fatalf("failure count should reset to zero: got=%d", state.failureCount)
	}
	if state.currentBackoff != 500*time.Millisecond {
		t.Fatalf("backoff should reset to initial: got=%s want=500ms", state.currentBackoff)
	}
	if state.currentAttempt() != 1 {
		t.Fatalf("attempt should reset to one after success: got=%d", state.currentAttempt())
	}
}

func TestEnqueueMQTTMessageDropOldest(t *testing.T) {
	ctx := context.Background()
	queue := make(chan ecoflowmqtt.Message, 2)
	stats := &mqttQueueStats{}

	msg1 := ecoflowmqtt.Message{Topic: "t", Payload: []byte("1")}
	msg2 := ecoflowmqtt.Message{Topic: "t", Payload: []byte("2")}
	msg3 := ecoflowmqtt.Message{Topic: "t", Payload: []byte("3")}

	if ok, dropped := enqueueMQTTMessageDropOldest(ctx, queue, msg1, stats); !ok || dropped {
		t.Fatalf("enqueue msg1 failed: ok=%v dropped=%v", ok, dropped)
	}
	if ok, dropped := enqueueMQTTMessageDropOldest(ctx, queue, msg2, stats); !ok || dropped {
		t.Fatalf("enqueue msg2 failed: ok=%v dropped=%v", ok, dropped)
	}
	if ok, dropped := enqueueMQTTMessageDropOldest(ctx, queue, msg3, stats); !ok || !dropped {
		t.Fatalf("enqueue msg3 should drop oldest: ok=%v dropped=%v", ok, dropped)
	}

	if got := stats.droppedOldest.Load(); got != 1 {
		t.Fatalf("drop count mismatch: got=%d want=1", got)
	}
	if got := len(queue); got != 2 {
		t.Fatalf("queue depth mismatch: got=%d want=2", got)
	}

	out1 := <-queue
	out2 := <-queue
	if string(out1.Payload) != "2" || string(out2.Payload) != "3" {
		t.Fatalf("queue order mismatch after drop-oldest: got=[%s,%s] want=[2,3]", string(out1.Payload), string(out2.Payload))
	}
}

func TestRenderDashboardShowsMQTTQueueRow(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.MQTTQueueDepth = 3
	snapshot.MQTTQueueCapacity = 128
	snapshot.MQTTQueueDroppedOldest = 2

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "pdStatus"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "mqtt") || !strings.Contains(output, "queue: 3/128") || !strings.Contains(output, "drop-oldest: 2") || !strings.Contains(output, "last: pdStatus") {
		t.Fatalf("dashboard missing mqtt queue row, got=%q", output)
	}
}

func TestFormatMQTTStatusDegradedFallback(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.MQTTDegraded = true
	snapshot.MQTTDegradedReason = "MQTT auth degraded (broker reject code 5)"
	snapshot.MQTTFallbackActive = true

	got := formatMQTTStatus(snapshot)
	want := "MQTT auth degraded (broker reject code 5) + REST fallback"
	if got != want {
		t.Fatalf("mqtt degraded status mismatch: got=%q want=%q", got, want)
	}
}

func TestRenderDashboardShowsMQTTDegradedStatus(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.MQTTQueueDepth = 0
	snapshot.MQTTQueueCapacity = 128
	snapshot.MQTTQueueDroppedOldest = 0
	snapshot.MQTTDegraded = true
	snapshot.MQTTDegradedReason = "MQTT auth degraded (broker reject code 5)"
	snapshot.MQTTFallbackActive = true

	output := renderDashboard(
		ecoflow.GeneralInfoDevice{DeviceName: "Kitchen Delta 2 Max", ProductName: "DELTA 2 Max", SN: "R351ZABAPH331057"},
		"/open/a/b/quota",
		telemetryEnvelope{TypeCode: "n/a"},
		snapshot,
		nil,
		minuteTableConfig{},
	)
	if !strings.Contains(output, "status: MQTT auth degraded (broker reject code 5) + REST fallback") {
		t.Fatalf("dashboard missing mqtt degraded status row, got=%q", output)
	}
}

func keysFromMap(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
