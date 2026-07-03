package edgecollector

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrypayload"
	"google.golang.org/protobuf/types/known/structpb"
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

func TestNormalizeEcoFlowBLEMetricsDropsUnsafeValues(t *testing.T) {
	t.Parallel()

	params := NormalizeEcoFlowBLEMetrics(map[string]any{
		"battery_soc_percent": math.NaN(),
		"input_power_w":       math.Inf(1),
		"ac_input_plugged":    "true",
		"oversized_string":    strings.Repeat("x", maxEcoFlowBLEMetricStringBytes+1),
		"nested":              map[string]any{"unexpected": true},
		"pv_input_state":      "charging",
	})

	for _, key := range []string{"soc", "f32ShowSoc", "wattsInSum", "acInputPlugged", "bleAcInputPlugged", "bleOversizedString", "bleNested"} {
		if _, ok := params[key]; ok {
			t.Fatalf("unsafe key %q should have been dropped: %#v", key, params)
		}
	}
	if got := params["blePvInputState"]; got != "charging" {
		t.Fatalf("blePvInputState=%v want charging", got)
	}
}

func TestNormalizeEcoFlowBLEMetricStructDropsUnsafeValues(t *testing.T) {
	t.Parallel()

	metrics := &structpb.Struct{Fields: map[string]*structpb.Value{
		"battery_soc_percent": structpb.NewNumberValue(math.NaN()),
		"input_power_w":       structpb.NewNumberValue(math.Inf(1)),
		"ac_input_plugged":    structpb.NewStringValue("true"),
		"oversized_string":    structpb.NewStringValue(strings.Repeat("x", maxEcoFlowBLEMetricStringBytes+1)),
		"nested":              structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{"unexpected": structpb.NewBoolValue(true)}}),
		"list":                structpb.NewListValue(&structpb.ListValue{Values: []*structpb.Value{structpb.NewStringValue("unexpected")}}),
		"pv_input_state":      structpb.NewStringValue("charging"),
	}}

	params := NormalizeEcoFlowBLEMetricStruct(metrics)
	for _, key := range []string{"soc", "f32ShowSoc", "wattsInSum", "acInputPlugged", "bleAcInputPlugged", "bleOversizedString", "bleNested", "bleList"} {
		if _, ok := params[key]; ok {
			t.Fatalf("unsafe key %q should have been dropped: %#v", key, params)
		}
	}
	if got := params["blePvInputState"]; got != "charging" {
		t.Fatalf("blePvInputState=%v want charging", got)
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

func TestBuildTelemetryEnvelopeDerivesStableServerSampleID(t *testing.T) {
	t.Parallel()

	sample := TelemetrySample{
		CollectorID:      "edge_collector_1",
		DeviceID:         "device-uuid-1",
		Provider:         "ecoflow",
		ProviderDeviceID: "DEMOEDGE0001",
		Transport:        "ble",
		ObservedAt:       time.Date(2026, 5, 28, 12, 30, 0, 0, time.UTC),
		Params: map[string]any{
			"wattsOutSum": float64(120),
		},
	}
	subjectCfg := telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 8}
	first, err := BuildTelemetryEnvelope(sample, subjectCfg)
	if err != nil {
		t.Fatalf("BuildTelemetryEnvelope first failed: %v", err)
	}
	second, err := BuildTelemetryEnvelope(sample, subjectCfg)
	if err != nil {
		t.Fatalf("BuildTelemetryEnvelope second failed: %v", err)
	}
	if first.GetMessageId() == "" || !strings.HasPrefix(first.GetMessageId(), "edge-telemetry-") {
		t.Fatalf("message_id=%q want server-derived edge telemetry id", first.GetMessageId())
	}
	if first.GetEnvelopeId() != second.GetEnvelopeId() {
		t.Fatalf("envelope_id should be stable for identical sample content: first=%q second=%q", first.GetEnvelopeId(), second.GetEnvelopeId())
	}
	third, err := BuildTelemetryEnvelope(sample, subjectCfg)
	if err != nil {
		t.Fatalf("BuildTelemetryEnvelope repeated sample failed: %v", err)
	}
	if third.GetEnvelopeId() != first.GetEnvelopeId() || third.GetMessageId() != first.GetMessageId() {
		t.Fatalf("server idempotency should be stable for repeated sample content: first=%q/%q third=%q/%q", first.GetEnvelopeId(), first.GetMessageId(), third.GetEnvelopeId(), third.GetMessageId())
	}
	changedSample := sample
	changedSample.ObservedAt = changedSample.ObservedAt.Add(time.Second)
	fourth, err := BuildTelemetryEnvelope(changedSample, subjectCfg)
	if err != nil {
		t.Fatalf("BuildTelemetryEnvelope changed sample failed: %v", err)
	}
	if fourth.GetEnvelopeId() == first.GetEnvelopeId() || fourth.GetMessageId() == first.GetMessageId() {
		t.Fatalf("different sample content should get a different server id")
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

func BenchmarkNormalizeEcoFlowBLEMetricStruct(b *testing.B) {
	metrics, err := structpb.NewStruct(map[string]any{
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
	})
	if err != nil {
		b.Fatalf("new metrics struct: %v", err)
	}

	b.ReportAllocs()
	for b.Loop() {
		params := NormalizeEcoFlowBLEMetricStruct(metrics)
		if params["wattsOutSum"] != float64(147) {
			b.Fatalf("wattsOutSum=%v", params["wattsOutSum"])
		}
	}
}

func BenchmarkNormalizeEcoFlowBLEMetricStructViaAsMap(b *testing.B) {
	metrics, err := structpb.NewStruct(map[string]any{
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
	})
	if err != nil {
		b.Fatalf("new metrics struct: %v", err)
	}

	b.ReportAllocs()
	for b.Loop() {
		params := NormalizeEcoFlowBLEMetrics(metrics.AsMap())
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

	b.Run("clone_params", func(b *testing.B) {
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
	})
	b.Run("owned_params", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			envelope, err := BuildTelemetryEnvelopeWithOwnedParams(sample, subjectCfg)
			if err != nil {
				b.Fatalf("BuildTelemetryEnvelopeWithOwnedParams failed: %v", err)
			}
			if envelope.GetShardCount() != 8 {
				b.Fatalf("shard count=%d", envelope.GetShardCount())
			}
		}
	})
}
