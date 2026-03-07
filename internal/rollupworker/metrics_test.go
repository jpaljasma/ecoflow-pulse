package rollupworker

import (
	"testing"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
)

func TestSampleFromEnvelopeD2MPDStatus(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"params":{"wattsInSum":259,"pv1ChargeWatts":52,"pv2ChargeWatts":0,"wattsOutSum":217,"f32ShowSoc":25.5,"carWatts":0,"typec1Watts":0,"typec2Watts":0,"usb1Watts":0,"usb2Watts":0}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if got := sample.Metrics.SOC.Value; got != 25.5 {
		t.Fatalf("soc mismatch: got=%v want=25.5", got)
	}
	if got := sample.Metrics.PV.Value; got != 52 {
		t.Fatalf("pv mismatch: got=%v want=52", got)
	}
	if got := sample.Metrics.ACIn.Value; got != 207 {
		t.Fatalf("ac in mismatch: got=%v want=207", got)
	}
	if got := sample.Metrics.Load.Value; got != 217 {
		t.Fatalf("load mismatch: got=%v want=217", got)
	}
	if got := sample.Metrics.Net.Value; got != 42 {
		t.Fatalf("net mismatch: got=%v want=42", got)
	}
}

func TestSampleFromEnvelopeDPUAppShowAndBackendFields(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"params":{"inLvMpptPwr":53,"inHvMpptPwr":0,"wattsOutSum":140,"inAcC20Pwr":123,"outUsb1Pwr":10,"outTypec1Pwr":15,"bmsInputWatts":50,"bmsOutputWatts":10,"pdTemp":18,"mpptLvTemp":22,"pcsAcTemp":20,"cmsBattSoc":19.7}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if got := sample.Metrics.SOC.Value; got != 19.7 {
		t.Fatalf("soc mismatch: got=%v want=19.7", got)
	}
	if got := sample.Metrics.PV.Value; got != 53 {
		t.Fatalf("pv mismatch: got=%v want=53", got)
	}
	if got := sample.Metrics.ACIn.Value; got != 123 {
		t.Fatalf("ac in mismatch: got=%v want=123", got)
	}
	if got := sample.Metrics.Load.Value; got != 140 {
		t.Fatalf("load mismatch: got=%v want=140", got)
	}
	if got := sample.Metrics.DC.Value; got != 25 {
		t.Fatalf("dc mismatch: got=%v want=25", got)
	}
	if got := sample.Metrics.Battery.Value; got != 40 {
		t.Fatalf("battery mismatch: got=%v want=40", got)
	}
	if got := sample.Metrics.Temp.Value; got != 20 {
		t.Fatalf("temp median mismatch: got=%v want=20", got)
	}
}

func TestSampleFromEnvelopeD2MMPPTStatusDerivesPVFromNormalizedVoltageCurrent(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"typeCode":"mpptStatus","params":{"chgState":1,"pv2ChgState":1,"inVol":10499,"inAmp":133,"pv2InVol":15118,"pv2InAmp":330}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}

	want := (10.499 * 0.133) + (15.118 * 0.330)
	if !sample.Metrics.PV.Valid {
		t.Fatalf("expected pv metric to be valid")
	}
	if diff := sample.Metrics.PV.Value - want; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("pv mismatch: got=%v want=%v", sample.Metrics.PV.Value, want)
	}
}

func TestSampleFromEnvelopeDerivesDPUAndersonWattsFromBackendVoltageCurrent(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"params":{"outAdsPwr":0,"outAdsAmp":0.69193053,"outAdsVol":12.95,"outUsb1Pwr":3,"outTypec2Pwr":5,"cmsBattSoc":55}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if got := sample.Metrics.DC.Value; got < 16.95 || got > 16.97 {
		t.Fatalf("dc mismatch: got=%v want~=16.96", got)
	}
}

func TestSampleFromEnvelopePrefersCanonicalD2MQuotaPVFields(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"params":{"pv1ChargeWatts":158,"pv2ChargeWatts":333,"inLvMpptPwr":175288048,"inHvMpptPwr":443476824,"wattsInSum":491,"wattsOutSum":0,"f32ShowSoc":90}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if got := sample.Metrics.PV.Value; got != 491 {
		t.Fatalf("pv mismatch: got=%v want=491", got)
	}
	if got := sample.Metrics.ACIn.Value; got != 0 {
		t.Fatalf("ac in mismatch: got=%v want=0", got)
	}
}

func TestSampleFromEnvelopeIgnoresAbsurdTopLevelPV(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"pvW":1757819.6,"params":{"pv1ChargeWatts":57,"pv2ChargeWatts":45,"wattsInSum":102,"wattsOutSum":0,"f32ShowSoc":58}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if got := sample.Metrics.PV.Value; got != 102 {
		t.Fatalf("pv mismatch: got=%v want=102", got)
	}
	if got := sample.Metrics.ACIn.Value; got != 0 {
		t.Fatalf("ac in mismatch: got=%v want=0", got)
	}
}

func TestSampleFromEnvelopeDoesNotTreatChgSunPowerAsPVWatts(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"params":{"chgSunPower":260,"wattsInSum":0,"wattsOutSum":0,"f32ShowSoc":58}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if sample.Metrics.PV.Valid {
		t.Fatalf("expected PV to be invalid, got=%v", sample.Metrics.PV.Value)
	}
	if sample.Metrics.SolarGeneratedWh.Valid {
		t.Fatalf("expected SolarGeneratedWh to be invalid, got=%v", sample.Metrics.SolarGeneratedWh.Value)
	}
	if sample.Metrics.ACIn.Valid && sample.Metrics.ACIn.Value != 0 {
		t.Fatalf("expected ACIn to be zero/invalid, got=%v", sample.Metrics.ACIn.Value)
	}
}

func TestSampleFromEnvelopePrefersExplicitZeroCanonicalPVOverStaleTopLevelPV(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"pvW":260,"params":{"pv1ChargeWatts":0,"pv2ChargeWatts":0,"inLvMpptPwr":0,"inHvMpptPwr":0,"wattsInSum":0,"wattsOutSum":0,"f32ShowSoc":58}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if !sample.Metrics.PV.Valid || sample.Metrics.PV.Value != 0 {
		t.Fatalf("pv mismatch: got valid=%v value=%v want=0", sample.Metrics.PV.Valid, sample.Metrics.PV.Value)
	}
	if sample.Metrics.ACIn.Valid && sample.Metrics.ACIn.Value != 0 {
		t.Fatalf("ac in mismatch: got=%v want=0", sample.Metrics.ACIn.Value)
	}
}

func TestSampleFromEnvelopeFallsBackToPerPortWhenDpuMpptIsExplicitZero(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"params":{"pv1ChargeWatts":260,"inLvMpptPwr":0,"inHvMpptPwr":0,"wattsInSum":0,"wattsOutSum":0,"f32ShowSoc":58}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if !sample.Metrics.PV.Valid || sample.Metrics.PV.Value != 260 {
		t.Fatalf("pv mismatch: got valid=%v value=%v want=260", sample.Metrics.PV.Valid, sample.Metrics.PV.Value)
	}
	if sample.Metrics.ACIn.Valid && sample.Metrics.ACIn.Value != 0 {
		t.Fatalf("ac in mismatch: got=%v want=0", sample.Metrics.ACIn.Value)
	}
}

func TestSampleFromEnvelopeFallsBackToPerPortWhenMpptZero(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"params":{"inLvMpptPwr":0,"inHvMpptPwr":0,"pv1ChargeWatts":180,"pv2ChargeWatts":55,"wattsInSum":235,"wattsOutSum":0,"f32ShowSoc":58}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if !sample.Metrics.PV.Valid || sample.Metrics.PV.Value != 235 {
		t.Fatalf("pv mismatch: got valid=%v value=%v want=235", sample.Metrics.PV.Valid, sample.Metrics.PV.Value)
	}
}

func TestSampleFromEnvelopeFallsBackToVoltsTimesAmpsWhenPerPortZeroAndNotIdle(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"typeCode":"mpptStatus","params":{"inWatts":0,"chgState":1,"inVol":68000,"inAmp":1200}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if !sample.Metrics.PV.Valid {
		t.Fatalf("expected pv metric to be valid")
	}
	want := 68.0 * 1.2
	if diff := sample.Metrics.PV.Value - want; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("pv mismatch: got=%v want=%v", sample.Metrics.PV.Value, want)
	}
}

func TestSampleFromEnvelopeDoesNotUseVoltsTimesAmpsWhenMpptIdle(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"typeCode":"mpptStatus","params":{"inWatts":0,"chgState":0,"inVol":68000,"inAmp":1200}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if !sample.Metrics.PV.Valid || sample.Metrics.PV.Value != 0 {
		t.Fatalf("pv mismatch: got valid=%v value=%v want=0", sample.Metrics.PV.Valid, sample.Metrics.PV.Value)
	}
}

func TestSampleFromEnvelopeUsesCellTempMedian(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"params":{"outputWatts":99,"cellTemp":[13,19,17,11,80]}}`)

	sample, err := SampleFromEnvelope(env)
	if err != nil {
		t.Fatalf("SampleFromEnvelope failed: %v", err)
	}
	if got := sample.Metrics.Temp.Value; got != 17 {
		t.Fatalf("temp median mismatch: got=%v want=17", got)
	}
}

func TestSampleFromEnvelopeReturnsNoMetrics(t *testing.T) {
	t.Parallel()
	env := testEnvelope(`{"params":{"icoBytes":[0,1,2]}}`)

	_, err := SampleFromEnvelope(env)
	if err != ErrNoRollupMetrics {
		t.Fatalf("expected ErrNoRollupMetrics, got=%v", err)
	}
}

func TestSampleFromEnvelopeRejectsInvalidIdentity(t *testing.T) {
	t.Parallel()
	env := &envelopev1.TelemetryEnvelope{
		DeviceId:           "not-a-uuid",
		EcoflowSn:          "R351ZABAPH331057",
		ObservedTimeUnixMs: 1,
		Payload:            []byte(`{"params":{"wattsOutSum":1}}`),
		Labels:             map[string]string{"provider": "ecoflow"},
	}

	_, err := SampleFromEnvelope(env)
	if err != ErrInvalidRollupEnvelope {
		t.Fatalf("expected ErrInvalidRollupEnvelope, got=%v", err)
	}
}

func testEnvelope(payload string) *envelopev1.TelemetryEnvelope {
	return &envelopev1.TelemetryEnvelope{
		DeviceId:           "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52",
		EcoflowSn:          "R351ZABAPH331057",
		ObservedTimeUnixMs: 1770000000000,
		Payload:            []byte(payload),
		Labels:             map[string]string{"provider": "ecoflow"},
	}
}
