package edgecollector

import (
	"encoding/json"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrypayload"
)

func TestNormalizeEcoFlowBLEMetricsMapsDisplayFields(t *testing.T) {
	t.Parallel()

	params := NormalizeEcoFlowBLEMetrics(map[string]any{
		"battery_soc_percent":             float64(100),
		"input_power_w":                   float64(149),
		"output_power_w":                  float64(147),
		"pv_input_power_w":                float64(42),
		"pv2_input_power_w":               float64(17),
		"battery_charge_remaining_min":    float64(5939),
		"battery_discharge_remaining_min": float64(88),
		"ac_input_plugged":                true,
		"error_code":                      float64(0),
	})

	want := map[string]any{
		"soc":               float64(100),
		"f32ShowSoc":        float64(100),
		"wattsInSum":        float64(149),
		"wattsOutSum":       float64(147),
		"pv1ChargeWatts":    float64(42),
		"pv1InWatts":        float64(42),
		"pv2ChargeWatts":    float64(17),
		"pv2InWatts":        float64(17),
		"chgRemainTime":     float64(5939),
		"dsgRemainTime":     float64(88),
		"acInputPlugged":    true,
		"bleErrorCode":      float64(0),
		"bleAcInputPlugged": true,
	}

	for key, wantValue := range want {
		if got := params[key]; got != wantValue {
			t.Fatalf("%s=%v want %v", key, got, wantValue)
		}
	}
}

func TestBuildTelemetryEnvelopeUsesEdgeLocalSource(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, 5, 28, 12, 30, 0, 0, time.UTC)
	subjectCfg := telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 8}
	envelope, err := BuildTelemetryEnvelope(TelemetrySample{
		CollectorID:      "edge_collector_1",
		DeviceID:         "device-uuid-1",
		Provider:         "ecoflow",
		ProviderDeviceID: "DEMOEDGE0001",
		Transport:        "ble",
		ObservedAt:       observedAt,
		Params: map[string]any{
			"wattsOutSum": float64(120),
		},
	}, subjectCfg)
	if err != nil {
		t.Fatalf("BuildTelemetryEnvelope failed: %v", err)
	}

	if envelope.GetSourceKind() != envelopev1.SourceKind_SOURCE_KIND_EDGE_LOCAL {
		t.Fatalf("source kind=%v want edge local", envelope.GetSourceKind())
	}
	if envelope.GetSource() != "ble" {
		t.Fatalf("source=%q want ble", envelope.GetSource())
	}
	if envelope.GetTypeCode() != "edge.ecoflow.ble.metrics" {
		t.Fatalf("type code=%q", envelope.GetTypeCode())
	}
	if envelope.GetPayloadType() != telemetrypayload.ProviderNormalizedPayloadType {
		t.Fatalf("payload type=%q", envelope.GetPayloadType())
	}
	if envelope.GetDeviceId() != "device-uuid-1" || envelope.GetEcoflowSn() != "DEMOEDGE0001" {
		t.Fatalf("identity mismatch: device=%q sn=%q", envelope.GetDeviceId(), envelope.GetEcoflowSn())
	}
	if envelope.GetShard() != telemetrybus.ShardForDevice("device-uuid-1", subjectCfg.ShardCount) {
		t.Fatalf("unexpected shard=%d", envelope.GetShard())
	}

	var payload struct {
		TypeCode string         `json:"typeCode"`
		Params   map[string]any `json:"params"`
	}
	if err := json.Unmarshal(envelope.GetPayload(), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.TypeCode != "edge.ecoflow.ble.metrics" {
		t.Fatalf("payload typeCode=%q", payload.TypeCode)
	}
	if got := payload.Params["wattsOutSum"]; got != float64(120) {
		t.Fatalf("wattsOutSum=%v", got)
	}
}

func TestCollectorSecretHashValidationRejectsWrongSecret(t *testing.T) {
	t.Parallel()

	secret := "pulse_edge_secret_test_value"
	hash := HashCollectorSecret(secret)
	if !ValidateCollectorSecret(secret, hash) {
		t.Fatalf("expected secret to validate")
	}
	if ValidateCollectorSecret("wrong-secret", hash) {
		t.Fatalf("wrong secret validated")
	}
	if ValidateCollectorSecret(secret, "") {
		t.Fatalf("blank hash validated")
	}
}

func BenchmarkNormalizeEcoFlowBLEMetrics(b *testing.B) {
	metrics := map[string]any{
		"battery_soc_percent":             float64(100),
		"input_power_w":                   float64(149),
		"output_power_w":                  float64(147),
		"pv_input_power_w":                float64(42),
		"pv2_input_power_w":               float64(17),
		"battery_charge_remaining_min":    float64(5939),
		"battery_discharge_remaining_min": float64(88),
		"ac_input_plugged":                true,
		"ac_charger_connected":            true,
		"ac_output_enabled":               true,
		"error_code":                      float64(0),
	}

	b.ReportAllocs()
	for b.Loop() {
		params := NormalizeEcoFlowBLEMetrics(metrics)
		if params["wattsOutSum"] != float64(147) {
			b.Fatalf("wattsOutSum=%v", params["wattsOutSum"])
		}
	}
}

func BenchmarkBuildTelemetryEnvelope(b *testing.B) {
	sample := TelemetrySample{
		CollectorID:      "edge_collector_1",
		DeviceID:         "device-uuid-1",
		Provider:         "ecoflow",
		ProviderDeviceID: "DEMOEDGE0001",
		Transport:        "ble",
		ObservedAt:       time.Date(2026, 5, 28, 12, 30, 0, 0, time.UTC),
		Params: map[string]any{
			"soc":            float64(100),
			"wattsInSum":     float64(149),
			"wattsOutSum":    float64(147),
			"pv1ChargeWatts": float64(42),
			"pv2ChargeWatts": float64(17),
		},
	}
	subjectCfg := telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 8}

	b.ReportAllocs()
	for b.Loop() {
		envelope, err := BuildTelemetryEnvelope(sample, subjectCfg)
		if err != nil {
			b.Fatalf("BuildTelemetryEnvelope failed: %v", err)
		}
		if envelope.GetShardCount() != 8 {
			b.Fatalf("shard count=%d", envelope.GetShardCount())
		}
	}
}
