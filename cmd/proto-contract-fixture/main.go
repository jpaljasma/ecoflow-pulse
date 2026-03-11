package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"google.golang.org/protobuf/proto"
)

type contractEnvelope struct {
	EnvelopeID       string            `json:"envelopeId"`
	EnvelopeVersion  uint32            `json:"envelopeVersion"`
	DeviceID         string            `json:"deviceId"`
	EcoflowSN        string            `json:"ecoflowSn"`
	Shard            uint32            `json:"shard"`
	ShardCount       uint32            `json:"shardCount"`
	MessageID        string            `json:"messageId"`
	DeviceTimeUnixMs int64             `json:"deviceTimeUnixMs"`
	ObservedUnixMs   int64             `json:"observedTimeUnixMs"`
	IngestedUnixMs   int64             `json:"ingestedTimeUnixMs"`
	SourceKind       string            `json:"sourceKind"`
	Source           string            `json:"source"`
	TypeCode         string            `json:"typeCode"`
	PayloadType      string            `json:"payloadType"`
	PayloadVersion   uint32            `json:"payloadVersion"`
	PayloadEncoding  string            `json:"payloadEncoding"`
	PayloadBase64    string            `json:"payloadBase64"`
	PayloadUTF8      string            `json:"payloadUtf8"`
	Labels           map[string]string `json:"labels,omitempty"`
}

type fixtureOutput struct {
	EnvelopeBase64 string           `json:"envelopeBase64"`
	Expected       contractEnvelope `json:"expected"`
	NodeMessage    map[string]any   `json:"nodeMessage"`
}

func main() {
	decodeBase64 := flag.String("decode-base64", "", "decode base64 envelope bytes and print normalized JSON")
	flag.Parse()

	if strings.TrimSpace(*decodeBase64) != "" {
		decoded, err := decodeEnvelope(strings.TrimSpace(*decodeBase64))
		if err != nil {
			failf("decode envelope: %v", err)
		}
		writeJSON(decoded)
		return
	}

	envelope := sampleEnvelope()
	wire, err := proto.Marshal(envelope)
	if err != nil {
		failf("marshal sample envelope: %v", err)
	}
	normalized, err := normalizeEnvelope(envelope)
	if err != nil {
		failf("normalize sample envelope: %v", err)
	}
	out := fixtureOutput{
		EnvelopeBase64: base64.StdEncoding.EncodeToString(wire),
		Expected:       normalized,
		NodeMessage:    nodeMessageForEnvelope(envelope),
	}
	writeJSON(out)
}

func sampleEnvelope() *envelopev1.TelemetryEnvelope {
	payload := []byte(`{"params":{"wattsInSum":321.5,"f32ShowSoc":54.2,"temp":19.6},"pvW":88}`)
	return &envelopev1.TelemetryEnvelope{
		EnvelopeId:         "019caaf0-a123-7b3f-84c1-cf2a1337abcd",
		EnvelopeVersion:    1,
		DeviceId:           "019caaf0-b456-7e77-b356-3a2f12bcdeff",
		EcoflowSn:          "DEMOD2M00001057",
		Shard:              12,
		ShardCount:         128,
		MessageId:          "mqtt-msg-001",
		DeviceTimeUnixMs:   1730000000123,
		ObservedTimeUnixMs: 1730000000456,
		IngestedTimeUnixMs: 1730000000789,
		SourceKind:         envelopev1.SourceKind_SOURCE_KIND_MQTT_STATUS,
		Source:             "mqtt",
		TypeCode:           "pdStatus",
		PayloadType:        "ecoflow.mqtt.raw",
		PayloadVersion:     1,
		PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		Payload:            payload,
		Labels: map[string]string{
			"provider": "ecoflow",
			"region":   "us-east1",
		},
	}
}

func normalizeEnvelope(envelope *envelopev1.TelemetryEnvelope) (contractEnvelope, error) {
	payload := append([]byte(nil), envelope.GetPayload()...)
	return contractEnvelope{
		EnvelopeID:       strings.TrimSpace(envelope.GetEnvelopeId()),
		EnvelopeVersion:  envelope.GetEnvelopeVersion(),
		DeviceID:         strings.TrimSpace(envelope.GetDeviceId()),
		EcoflowSN:        strings.ToUpper(strings.TrimSpace(envelope.GetEcoflowSn())),
		Shard:            envelope.GetShard(),
		ShardCount:       envelope.GetShardCount(),
		MessageID:        strings.TrimSpace(envelope.GetMessageId()),
		DeviceTimeUnixMs: envelope.GetDeviceTimeUnixMs(),
		ObservedUnixMs:   envelope.GetObservedTimeUnixMs(),
		IngestedUnixMs:   envelope.GetIngestedTimeUnixMs(),
		SourceKind:       envelope.GetSourceKind().String(),
		Source:           strings.TrimSpace(envelope.GetSource()),
		TypeCode:         strings.TrimSpace(envelope.GetTypeCode()),
		PayloadType:      strings.TrimSpace(envelope.GetPayloadType()),
		PayloadVersion:   envelope.GetPayloadVersion(),
		PayloadEncoding:  envelope.GetPayloadEncoding().String(),
		PayloadBase64:    base64.StdEncoding.EncodeToString(payload),
		PayloadUTF8:      string(payload),
		Labels:           cloneLabels(envelope.GetLabels()),
	}, nil
}

func nodeMessageForEnvelope(envelope *envelopev1.TelemetryEnvelope) map[string]any {
	labels := make(map[string]any, len(envelope.GetLabels()))
	for k, v := range envelope.GetLabels() {
		labels[k] = v
	}
	return map[string]any{
		"envelopeId":         envelope.GetEnvelopeId(),
		"envelopeVersion":    envelope.GetEnvelopeVersion(),
		"deviceId":           envelope.GetDeviceId(),
		"ecoflowSn":          envelope.GetEcoflowSn(),
		"shard":              envelope.GetShard(),
		"shardCount":         envelope.GetShardCount(),
		"messageId":          envelope.GetMessageId(),
		"deviceTimeUnixMs":   envelope.GetDeviceTimeUnixMs(),
		"observedTimeUnixMs": envelope.GetObservedTimeUnixMs(),
		"ingestedTimeUnixMs": envelope.GetIngestedTimeUnixMs(),
		"sourceKind":         uint32(envelope.GetSourceKind()),
		"source":             envelope.GetSource(),
		"typeCode":           envelope.GetTypeCode(),
		"payloadType":        envelope.GetPayloadType(),
		"payloadVersion":     envelope.GetPayloadVersion(),
		"payloadEncoding":    uint32(envelope.GetPayloadEncoding()),
		"payload":            envelope.GetPayload(),
		"labels":             labels,
	}
}

func decodeEnvelope(rawBase64 string) (contractEnvelope, error) {
	wire, err := base64.StdEncoding.DecodeString(rawBase64)
	if err != nil {
		return contractEnvelope{}, fmt.Errorf("decode base64: %w", err)
	}
	var envelope envelopev1.TelemetryEnvelope
	if err := proto.Unmarshal(wire, &envelope); err != nil {
		return contractEnvelope{}, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return normalizeEnvelope(&envelope)
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		out[k] = v
	}
	return out
}

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		failf("encode json: %v", err)
	}
}

func failf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
