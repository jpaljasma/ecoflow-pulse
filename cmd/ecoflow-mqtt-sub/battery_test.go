package main

import "testing"

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

func TestEnergySnapshotOverallPreconditioningDoesNotUsePTCEventAlone(t *testing.T) {
	snapshot := newEnergySnapshot()

	// Observed DPU case: ptcHeatingEvent can stay non-zero while mos state is 0.
	// The UI should not treat this as active heating.
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
	if !pack1.HasPreconditioning || pack1.PreconditioningOn {
		t.Fatalf("expected bp1 preconditioning off when only event is set, got has=%v on=%v", pack1.HasPreconditioning, pack1.PreconditioningOn)
	}
	if !pack1.HasPreconditioningEvent || pack1.PreconditioningEventRaw != 7 {
		t.Fatalf("expected ptc event=7, got has=%v value=%d", pack1.HasPreconditioningEvent, pack1.PreconditioningEventRaw)
	}
	derived := snapshot.derived()
	if derived.StatusPrecondValue != "[ ]" {
		t.Fatalf("overall preconditioning status mismatch: got=%s want=[ ]", derived.StatusPrecondValue)
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
