package main

import (
	"testing"
	"time"
)

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
		"inLvMpptPwr":                          190.0,
		"hs_yj751_pd_appshow_addr.inLvMpptPwr": 190.0,
		"powGetPvL":                            190.0,
		"d_addr.powGetPvL":                     190.0,
		"inHvMpptPwr":                          33.0,
		"hs_yj751_pd_appshow_addr.inHvMpptPwr": 33.0,
		"powGetPvH":                            33.0,
		"d_addr.powGetPvH":                     33.0,
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
	if rows[0][1] != "n/a" {
		t.Fatalf("soc minute metric mismatch: got=%s want=n/a", rows[0][1])
	}
	if rows[0][2] != "0.4" {
		t.Fatalf("solar minute metric mismatch: got=%s want=0.4", rows[0][2])
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
