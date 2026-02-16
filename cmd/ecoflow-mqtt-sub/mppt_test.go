package main

import "testing"

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

func TestEnergySnapshotMPPTInactiveStateSuppressesVIFallback(t *testing.T) {
	snapshot := newEnergySnapshot()
	payload := []byte(`{"moduleType":5,"needAck":0,"id":10759557,"time":139347912,"params":{"inVol":10500,"inAmp":145,"chgState":0,"pv2InVol":16662,"pv2InAmp":36,"pv2ChgState":0},"version":"1.0","typeCode":"mpptStatus"}`)
	envelope, quota, err := parseTelemetryPayload(payload)
	if err != nil {
		t.Fatalf("parse mpptStatus payload: %v", err)
	}
	snapshot.Update(envelope, quota, nil, false, pdStatusSummary{}, false)

	if !snapshot.HasSolarLVAmp || snapshot.SolarLVAmp < 0.14 || snapshot.SolarLVAmp > 0.15 {
		t.Fatalf("lv amps normalization mismatch: has=%v value=%f", snapshot.HasSolarLVAmp, snapshot.SolarLVAmp)
	}
	if !snapshot.HasSolarHVAmp || snapshot.SolarHVAmp < 0.03 || snapshot.SolarHVAmp > 0.04 {
		t.Fatalf("hv amps normalization mismatch: has=%v value=%f", snapshot.HasSolarHVAmp, snapshot.SolarHVAmp)
	}
	if !snapshot.HasInPVLow || snapshot.InPVLowWatts != 0 {
		t.Fatalf("in_pv_low should be explicit 0 when mppt low is inactive, has=%v value=%f", snapshot.HasInPVLow, snapshot.InPVLowWatts)
	}
	if !snapshot.HasInPVHigh || snapshot.InPVHighWatts != 0 {
		t.Fatalf("in_pv_high should be explicit 0 when mppt high is inactive, has=%v value=%f", snapshot.HasInPVHigh, snapshot.InPVHighWatts)
	}

	derived := snapshot.derived()
	if derived.InPVValue != "0.0W" {
		t.Fatalf("pv total watts mismatch: got=%s want=0.0W", derived.InPVValue)
	}
}

func TestEnergySnapshotRejectsAbsurdPVEstimateFromScaledValues(t *testing.T) {
	// Simulate a bad scaled frame that slips through without normalization.
	gotWatts, got := effectivePVInputWatts(false, 0, true, 17480, true, 1823, false, false)
	if got {
		t.Fatalf("expected absurd estimate to be rejected, got hasInput=true watts=%f", gotWatts)
	}
	if gotWatts != 0 {
		t.Fatalf("expected absurd estimate clamp to zero watts, got=%f", gotWatts)
	}
}

func TestEffectivePVInputWattsExplicitZeroSuppressesVIFallback(t *testing.T) {
	gotWatts, got := effectivePVInputWatts(true, 0, true, 65.0, true, 1.0, false, false)
	if !got {
		t.Fatalf("expected explicit 0W PV reading to be considered known")
	}
	if gotWatts != 0 {
		t.Fatalf("expected explicit 0W to suppress V*I fallback, got=%f", gotWatts)
	}
}
