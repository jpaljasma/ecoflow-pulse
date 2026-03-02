package ingestworker

import (
	"encoding/json"
	"testing"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

func TestBuildTelemetryEnvelopeFromMQTTPayload(t *testing.T) {
	t.Parallel()

	assignment := controlplane.IngestAssignment{
		Provider:         "ecoflow",
		ProviderDeviceID: "r351zabaph331057",
		DeviceID:         "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:     "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
	}
	message := ecoflowmqtt.Message{
		Topic:   "/open/account/R351ZABAPH331057/quota",
		Payload: []byte(`{"id":38553260,"time":17089842,"typeCode":"kitInfo","addr":"d_addr","cmdId":1,"cmdFunc":2}`),
	}
	observedAt := time.UnixMilli(1771119926522)

	envelope, err := buildTelemetryEnvelope(assignment, message, observedAt, EcoFlowSessionConfig{
		ShardCount: 128,
	})
	if err != nil {
		t.Fatalf("buildTelemetryEnvelope() error = %v", err)
	}

	if envelope.GetEnvelopeId() == "" {
		t.Fatalf("envelope_id should be generated")
	}
	if envelope.GetEcoflowSn() != "R351ZABAPH331057" {
		t.Fatalf("ecoflow_sn mismatch: got=%q", envelope.GetEcoflowSn())
	}
	if envelope.GetSourceKind() != envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA {
		t.Fatalf("source kind mismatch: got=%s", envelope.GetSourceKind().String())
	}
	if envelope.GetSource() != "mqtt" {
		t.Fatalf("source mismatch: got=%q", envelope.GetSource())
	}
	if envelope.GetTypeCode() != "kitInfo" {
		t.Fatalf("type_code mismatch: got=%q", envelope.GetTypeCode())
	}
	if envelope.GetMessageId() != "38553260" {
		t.Fatalf("message_id mismatch: got=%q", envelope.GetMessageId())
	}
	if envelope.GetDeviceTimeUnixMs() != 0 {
		t.Fatalf("device_time_unix_ms should be 0 for non-unix ecoflow time, got=%d", envelope.GetDeviceTimeUnixMs())
	}
	if envelope.GetObservedTimeUnixMs() != observedAt.UnixMilli() {
		t.Fatalf("observed_time_unix_ms mismatch: got=%d", envelope.GetObservedTimeUnixMs())
	}
	if envelope.GetIngestedTimeUnixMs() != observedAt.UnixMilli() {
		t.Fatalf("ingested_time_unix_ms mismatch: got=%d", envelope.GetIngestedTimeUnixMs())
	}
	if envelope.GetPayloadEncoding() != envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8 {
		t.Fatalf("payload_encoding mismatch: got=%s", envelope.GetPayloadEncoding().String())
	}
	if envelope.GetLabels()["provider"] != "ecoflow" {
		t.Fatalf("provider label mismatch: got=%q", envelope.GetLabels()["provider"])
	}
	if envelope.GetLabels()["device_time_raw"] != "17089842" {
		t.Fatalf("device_time_raw label mismatch: got=%q", envelope.GetLabels()["device_time_raw"])
	}
	if envelope.GetLabels()["cmd_id"] != "1" || envelope.GetLabels()["cmd_func"] != "2" {
		t.Fatalf("cmd labels mismatch: labels=%v", envelope.GetLabels())
	}
}

func TestBuildTelemetryEnvelopeWithInvalidJSONPayload(t *testing.T) {
	t.Parallel()

	assignment := controlplane.IngestAssignment{
		Provider:         "ecoflow",
		ProviderDeviceID: "Y711ZABA9H2P0294",
	}
	message := ecoflowmqtt.Message{
		Topic:   "/open/account/Y711ZABA9H2P0294/quota",
		Payload: []byte("{not json"),
	}

	envelope, err := buildTelemetryEnvelope(assignment, message, time.UnixMilli(1771119926522), EcoFlowSessionConfig{})
	if err != nil {
		t.Fatalf("buildTelemetryEnvelope() error = %v", err)
	}
	if envelope.GetPayloadEncoding() != envelopev1.PayloadEncoding_PAYLOAD_ENCODING_UNSPECIFIED {
		t.Fatalf("payload_encoding mismatch: got=%s", envelope.GetPayloadEncoding().String())
	}
	if envelope.GetTypeCode() != "quota" {
		t.Fatalf("expected fallback topic type_code=quota, got=%q", envelope.GetTypeCode())
	}
}

func TestBuildTelemetryEnvelopeWithLabelsDisabled(t *testing.T) {
	t.Parallel()

	assignment := controlplane.IngestAssignment{
		Provider:         "ecoflow",
		ProviderDeviceID: "R351ZABAPH331057",
		DeviceID:         "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:     "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
	}
	message := ecoflowmqtt.Message{
		Topic:   "/open/account/R351ZABAPH331057/quota",
		Payload: []byte(`{"id":1,"time":17089842,"typeCode":"kitInfo","cmdId":1,"cmdFunc":2}`),
	}

	envelope, err := buildTelemetryEnvelope(assignment, message, time.UnixMilli(1771119926522), EcoFlowSessionConfig{
		DisableEnvelopeLabels: true,
	})
	if err != nil {
		t.Fatalf("buildTelemetryEnvelope() error = %v", err)
	}
	if got := envelope.GetLabels(); got != nil {
		t.Fatalf("expected labels to be nil when disabled, got=%v", got)
	}
}

func TestBuildQuotaTelemetryEnvelope(t *testing.T) {
	t.Parallel()

	assignment := controlplane.IngestAssignment{
		Provider:         "ecoflow",
		ProviderDeviceID: "R351ZABAPH331057",
		DeviceID:         "018f11c6-6b6e-7419-8a96-8e975db23659",
		CredentialID:     "018f11c6-6bd6-7e10-9f6f-1245fc66f52c",
	}
	builder := newTelemetryEnvelopeBuilder(assignment, EcoFlowSessionConfig{ShardCount: 128})
	observedAt := time.UnixMilli(1771119926522)

	envelope, err := builder.BuildQuota(map[string]any{
		"soc":         int64(33),
		"wattsInSum":  123.0,
		"chgDsgState": int64(1),
	}, observedAt)
	if err != nil {
		t.Fatalf("BuildQuota() error = %v", err)
	}
	if envelope.GetSource() != "quota" {
		t.Fatalf("quota source mismatch: got=%q", envelope.GetSource())
	}
	if envelope.GetTypeCode() != "quota" {
		t.Fatalf("quota type_code mismatch: got=%q", envelope.GetTypeCode())
	}
	if envelope.GetPayloadType() != "ecoflow.quota.normalized" {
		t.Fatalf("quota payload_type mismatch: got=%q", envelope.GetPayloadType())
	}
	var payload struct {
		TypeCode string         `json:"typeCode"`
		Params   map[string]any `json:"params"`
	}
	if err := json.Unmarshal(envelope.GetPayload(), &payload); err != nil {
		t.Fatalf("unmarshal quota payload failed: %v", err)
	}
	if payload.TypeCode != "quota" {
		t.Fatalf("quota payload typeCode mismatch: got=%q", payload.TypeCode)
	}
	if payload.Params["soc"] != float64(33) {
		t.Fatalf("quota payload soc mismatch: got=%v", payload.Params["soc"])
	}
}

func TestNormalizeDeviceUnixMS(t *testing.T) {
	t.Parallel()

	now := time.UnixMilli(1771119926522)
	if got := normalizeDeviceUnixMS(17089842, now); got != 0 {
		t.Fatalf("expected short ecoflow relative time to be rejected, got=%d", got)
	}
	if got := normalizeDeviceUnixMS(1771119926000, now); got != 1771119926000 {
		t.Fatalf("expected unix ms to pass through, got=%d", got)
	}
}
