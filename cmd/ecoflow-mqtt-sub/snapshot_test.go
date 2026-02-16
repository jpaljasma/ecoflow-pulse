package main

import (
	"strings"
	"testing"
	"time"
)

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

func TestEnergySnapshotDerivesSolarChargingFromPackChargeStateAndPV(t *testing.T) {
	snapshot := newEnergySnapshot()

	bpPayload := []byte(`{"cmdId":4,"cmdFunc":2,"addr":"hs_yj751_pd_bp_addr","param":{"bpInfo":[{"bpNo":1,"bpSoc":14,"bpChgSta":1,"bpPwr":12}]},"params":{}}`)
	bpEnvelope, bpQuota, err := parseTelemetryPayload(bpPayload)
	if err != nil {
		t.Fatalf("parse bpInfo payload: %v", err)
	}
	snapshot.Update(bpEnvelope, bpQuota, nil, false, pdStatusSummary{}, false)

	mpptActivePayload := []byte(`{"moduleType":5,"needAck":0,"id":10759557,"time":139347912,"params":{"chgState":1,"inWatts":65},"version":"1.0","typeCode":"mpptStatus"}`)
	mpptActiveEnvelope, mpptActiveQuota, err := parseTelemetryPayload(mpptActivePayload)
	if err != nil {
		t.Fatalf("parse mppt active payload: %v", err)
	}
	snapshot.Update(mpptActiveEnvelope, mpptActiveQuota, nil, false, pdStatusSummary{}, false)

	derived := snapshot.derived()
	if derived.StatusSolarChargingValue != "[x]" {
		t.Fatalf("solar charging status mismatch when active: got=%s want=[x]", derived.StatusSolarChargingValue)
	}

	mpptIdlePayload := []byte(`{"moduleType":5,"needAck":0,"id":10759558,"time":139347913,"params":{"chgState":0,"inWatts":0},"version":"1.0","typeCode":"mpptStatus"}`)
	mpptIdleEnvelope, mpptIdleQuota, err := parseTelemetryPayload(mpptIdlePayload)
	if err != nil {
		t.Fatalf("parse mppt idle payload: %v", err)
	}
	snapshot.Update(mpptIdleEnvelope, mpptIdleQuota, nil, false, pdStatusSummary{}, false)

	derived = snapshot.derived()
	if derived.StatusSolarChargingValue != "[ ]" {
		t.Fatalf("solar charging status mismatch when mppt inactive: got=%s want=[ ]", derived.StatusSolarChargingValue)
	}
}

func TestEnergySnapshotDerivesSolarChargingFromBatteryNetFallback(t *testing.T) {
	snapshot := newEnergySnapshot()

	mpptPayload := []byte(`{"moduleType":5,"needAck":0,"id":10759557,"time":139347912,"params":{"chgState":1,"pv2ChgState":1,"inWatts":11,"pv2InWatts":24},"version":"1.0","typeCode":"mpptStatus"}`)
	mpptEnvelope, mpptQuota, err := parseTelemetryPayload(mpptPayload)
	if err != nil {
		t.Fatalf("parse mppt payload: %v", err)
	}
	snapshot.Update(mpptEnvelope, mpptQuota, nil, false, pdStatusSummary{}, false)

	// Delta 2 style fallback: no bpChgSta, but net battery direction is charging.
	chargingPDPayload := []byte(`{"moduleType":1,"needAck":0,"id":8220266,"time":16239852,"params":{"wattsInSum":35,"wattsOutSum":0,"invOutWatts":0},"version":"1.0","typeCode":"pdStatus"}`)
	chargingPDEnvelope, chargingPDQuota, err := parseTelemetryPayload(chargingPDPayload)
	if err != nil {
		t.Fatalf("parse charging pd payload: %v", err)
	}
	chargingPD, hasChargingPD := extractPDStatus(chargingPDQuota)
	snapshot.Update(chargingPDEnvelope, chargingPDQuota, nil, false, chargingPD, hasChargingPD)

	derived := snapshot.derived()
	if derived.StatusSolarChargingValue != "[x]" {
		t.Fatalf("solar charging fallback mismatch when charging: got=%s want=[x]", derived.StatusSolarChargingValue)
	}

	// If battery flow flips to discharge while PV remains active, solar charging should turn off.
	dischargingPDPayload := []byte(`{"moduleType":1,"needAck":0,"id":8220267,"time":16239853,"params":{"wattsInSum":11,"wattsOutSum":60,"invOutWatts":0},"version":"1.0","typeCode":"pdStatus"}`)
	dischargingPDEnvelope, dischargingPDQuota, err := parseTelemetryPayload(dischargingPDPayload)
	if err != nil {
		t.Fatalf("parse discharging pd payload: %v", err)
	}
	dischargingPD, hasDischargingPD := extractPDStatus(dischargingPDQuota)
	snapshot.Update(dischargingPDEnvelope, dischargingPDQuota, nil, false, dischargingPD, hasDischargingPD)

	derived = snapshot.derived()
	if derived.StatusSolarChargingValue != "[ ]" {
		t.Fatalf("solar charging fallback mismatch when discharging: got=%s want=[ ]", derived.StatusSolarChargingValue)
	}
}

func TestEnergySnapshotInfersSolarChargingForPackBasedSnapshotFromMetadata(t *testing.T) {
	snapshot := newEnergySnapshot()

	// DPU-style pack snapshot with SOC/power but missing bpChgSta.
	packPayload := []byte(`{"cmdId":28,"cmdFunc":3,"addr":"hs_yj751_bms_slave_addr_1","params":{"bpNo":1,"soc":14,"inputWatts":52,"outputWatts":0}}`)
	packEnvelope, packQuota, err := parseTelemetryPayload(packPayload)
	if err != nil {
		t.Fatalf("parse pack payload: %v", err)
	}
	snapshot.Update(packEnvelope, packQuota, nil, false, pdStatusSummary{}, false)

	appshowPayload := []byte(`{"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"inLvMpptPwr":59,"wattsInSum":59}}`)
	appshowEnvelope, appshowQuota, err := parseTelemetryPayload(appshowPayload)
	if err != nil {
		t.Fatalf("parse appshow payload: %v", err)
	}
	snapshot.Update(appshowEnvelope, appshowQuota, nil, false, pdStatusSummary{}, false)

	chargingPDPayload := []byte(`{"moduleType":1,"needAck":0,"id":8220266,"time":16239852,"params":{"wattsInSum":59,"wattsOutSum":0},"version":"1.0","typeCode":"pdStatus"}`)
	chargingPDEnvelope, chargingPDQuota, err := parseTelemetryPayload(chargingPDPayload)
	if err != nil {
		t.Fatalf("parse charging pd payload: %v", err)
	}
	chargingPD, hasChargingPD := extractPDStatus(chargingPDQuota)
	snapshot.Update(chargingPDEnvelope, chargingPDQuota, nil, false, chargingPD, hasChargingPD)

	derived := snapshot.derived()
	if derived.StatusSolarChargingValue != "[x]" {
		t.Fatalf("solar charging should be on with metadata gate and positive net charge: got=%s want=[x]", derived.StatusSolarChargingValue)
	}

	holdPDPayload := []byte(`{"moduleType":1,"needAck":0,"id":8220267,"time":16239853,"params":{"inLvMpptPwr":58,"wattsInSum":58,"wattsOutSum":0},"version":"1.0","typeCode":"pdStatus"}`)
	holdPDEnvelope, holdPDQuota, err := parseTelemetryPayload(holdPDPayload)
	if err != nil {
		t.Fatalf("parse hold pd payload: %v", err)
	}
	holdPD, hasHoldPD := extractPDStatus(holdPDQuota)
	snapshot.Update(holdPDEnvelope, holdPDQuota, nil, false, holdPD, hasHoldPD)

	derived = snapshot.derived()
	if derived.StatusSolarChargingValue != "[x]" {
		t.Fatalf("solar charging should remain on in hold band near 58W: got=%s want=[x]", derived.StatusSolarChargingValue)
	}

	offPDPayload := []byte(`{"moduleType":1,"needAck":0,"id":8220268,"time":16239854,"params":{"inLvMpptPwr":54,"wattsInSum":54,"wattsOutSum":0},"version":"1.0","typeCode":"pdStatus"}`)
	offPDEnvelope, offPDQuota, err := parseTelemetryPayload(offPDPayload)
	if err != nil {
		t.Fatalf("parse off pd payload: %v", err)
	}
	offPD, hasOffPD := extractPDStatus(offPDQuota)
	snapshot.Update(offPDEnvelope, offPDQuota, nil, false, offPD, hasOffPD)

	derived = snapshot.derived()
	if derived.StatusSolarChargingValue != "[ ]" {
		t.Fatalf("solar charging should turn off below hold threshold: got=%s want=[ ]", derived.StatusSolarChargingValue)
	}
}

func TestEnergySnapshotSolarChargingTurnsOffOnExplicitZeroPVForDPU(t *testing.T) {
	snapshot := newEnergySnapshot()

	packPayload := []byte(`{"cmdId":28,"cmdFunc":3,"addr":"hs_yj751_bms_slave_addr_1","params":{"bpNo":1,"soc":14,"inputWatts":45,"outputWatts":0}}`)
	packEnvelope, packQuota, err := parseTelemetryPayload(packPayload)
	if err != nil {
		t.Fatalf("parse pack payload: %v", err)
	}
	snapshot.Update(packEnvelope, packQuota, nil, false, pdStatusSummary{}, false)

	onPayload := []byte(`{"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"inLvMpptPwr":57,"wattsInSum":57}}`)
	onEnvelope, onQuota, err := parseTelemetryPayload(onPayload)
	if err != nil {
		t.Fatalf("parse on payload: %v", err)
	}
	snapshot.Update(onEnvelope, onQuota, nil, false, pdStatusSummary{}, false)

	chargingPDPayload := []byte(`{"moduleType":1,"needAck":0,"id":8220266,"time":16239852,"params":{"wattsInSum":57,"wattsOutSum":0},"version":"1.0","typeCode":"pdStatus"}`)
	chargingPDEnvelope, chargingPDQuota, err := parseTelemetryPayload(chargingPDPayload)
	if err != nil {
		t.Fatalf("parse charging pd payload: %v", err)
	}
	chargingPD, hasChargingPD := extractPDStatus(chargingPDQuota)
	snapshot.Update(chargingPDEnvelope, chargingPDQuota, nil, false, chargingPD, hasChargingPD)

	derived := snapshot.derived()
	if derived.StatusSolarChargingValue != "[x]" {
		t.Fatalf("solar charging should be on around 57W with positive net charge: got=%s want=[x]", derived.StatusSolarChargingValue)
	}

	offPayload := []byte(`{"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"inLvMpptPwr":0,"wattsInSum":0}}`)
	offEnvelope, offQuota, err := parseTelemetryPayload(offPayload)
	if err != nil {
		t.Fatalf("parse off payload: %v", err)
	}
	snapshot.Update(offEnvelope, offQuota, nil, false, pdStatusSummary{}, false)

	offDAddrPayload := []byte(`{"cmdId":21,"cmdFunc":254,"addr":"d_addr","param":{"powGetPvL":0},"params":{}}`)
	offDAddrEnvelope, offDAddrQuota, err := parseTelemetryPayload(offDAddrPayload)
	if err != nil {
		t.Fatalf("parse off d_addr payload: %v", err)
	}
	snapshot.Update(offDAddrEnvelope, offDAddrQuota, nil, false, pdStatusSummary{}, false)

	derived = snapshot.derived()
	if derived.StatusSolarChargingValue != "[ ]" {
		t.Fatalf("solar charging should turn off on explicit 0W PV: got=%s want=[ ]", derived.StatusSolarChargingValue)
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

func TestEnergySnapshotClearsStaleACInputFromTotalsWithoutACKeys(t *testing.T) {
	snapshot := newEnergySnapshot()

	// Seed a non-zero AC input from invStatus.
	invPayload := []byte(`{"moduleType":3,"needAck":0,"id":32520001,"time":20830001,"params":{"inputWatts":56,"outputWatts":15},"version":"1.0","typeCode":"invStatus"}`)
	invEnvelope, invQuota, err := parseTelemetryPayload(invPayload)
	if err != nil {
		t.Fatalf("parse invStatus payload: %v", err)
	}
	snapshot.Update(invEnvelope, invQuota, nil, false, pdStatusSummary{}, false)
	if !snapshot.HasInAC || snapshot.InACWatts != 56 {
		t.Fatalf("seed in_ac mismatch: has=%v value=%f", snapshot.HasInAC, snapshot.InACWatts)
	}

	// Next sparse pdStatus has no AC keys; wattsIn is fully explained by XT150 input.
	// Stale AC input should be cleared to 0.
	pdPayload := []byte(`{"moduleType":1,"needAck":0,"id":82360001,"time":20830002,"params":{"XT150Watts2":-99,"wattsInSum":99,"wattsOutSum":140,"invOutWatts":138},"version":"1.0","typeCode":"pdStatus"}`)
	pdEnvelope, pdQuota, err := parseTelemetryPayload(pdPayload)
	if err != nil {
		t.Fatalf("parse pdStatus payload: %v", err)
	}
	pd, hasPD := extractPDStatus(pdQuota)
	snapshot.Update(pdEnvelope, pdQuota, nil, false, pd, hasPD)

	if !snapshot.HasInAC || snapshot.InACWatts != 0 {
		t.Fatalf("stale in_ac should clear to zero, got has=%v value=%f", snapshot.HasInAC, snapshot.InACWatts)
	}
}

func TestEnergySnapshotClearsStaleACInputWhenSparseFramesPersist(t *testing.T) {
	snapshot := newEnergySnapshot()
	snapshot.HasInAC = true
	snapshot.InACWatts = 922
	snapshot.HasInACAt = true
	snapshot.InACAt = time.Now().Add(-30 * time.Second)
	snapshot.HasWattsIn = true
	snapshot.WattsIn = 1490
	snapshot.HasWattsInAt = true
	snapshot.WattsInAt = time.Now().Add(-30 * time.Second)

	// Sparse frame with no power keys should trigger stale clear fallback.
	payload := []byte(`{"moduleType":1,"needAck":0,"id":82360010,"time":20830010,"params":{"icoBytes":[8,8,8,0,128,0,0,0,0,0,0,0,0,0]},"version":"1.0","typeCode":"pdStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse sparse pdStatus payload: %v", err)
	}
	pd, hasPD := extractPDStatus(quota)
	snapshot.Update(envelope, quota, nil, false, pd, hasPD)

	if !snapshot.HasInAC || snapshot.InACWatts != 0 {
		t.Fatalf("stale in_ac should clear on sparse frames, got has=%v value=%f", snapshot.HasInAC, snapshot.InACWatts)
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

func TestEnergySnapshotFanStateOverridesFanLevelWhenBothPresent(t *testing.T) {
	snapshot := newEnergySnapshot()
	envelope := telemetryEnvelope{TypeCode: "quotaBootstrap"}
	quota := map[string]any{
		"fanState": 1,
		"fanLevel": 0,
	}

	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	if !snapshot.HasFanOn {
		t.Fatalf("expected fan on signal to be present")
	}
	if !snapshot.FanOn {
		t.Fatalf("expected fan to be on when fanState=1 even with fanLevel=0")
	}
	if !snapshot.HasFanLevel || snapshot.FanLevelRaw != 0 {
		t.Fatalf("expected fan level tracked as zero, got has=%v level=%d", snapshot.HasFanLevel, snapshot.FanLevelRaw)
	}
}

func TestEnergySnapshotRecordsSensorStatusTransitions(t *testing.T) {
	snapshot := newEnergySnapshot()

	onQuota := map[string]any{
		"cfgAcEnabled": 1,
		"fanState":     1,
	}
	snapshot.Update(telemetryEnvelope{TypeCode: "pdStatus"}, onQuota, nil, false, pdStatusSummary{}, false)

	offQuota := map[string]any{
		"cfgAcEnabled": 0,
		"fanState":     0,
	}
	snapshot.Update(telemetryEnvelope{TypeCode: "pdStatus"}, offQuota, nil, false, pdStatusSummary{}, false)

	updates := snapshot.recentSensorUpdates(10)
	if len(updates) < 4 {
		t.Fatalf("expected at least 4 sensor updates, got=%d", len(updates))
	}
	joined := ""
	for _, update := range updates {
		joined += update.Sensor + " " + update.Status + "\n"
	}
	for _, expected := range []string{
		"AC charging turned On",
		"Fan turned On",
		"AC charging turned Off",
		"Fan turned Off",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing expected sensor transition %q in updates:\n%s", expected, joined)
		}
	}
}
