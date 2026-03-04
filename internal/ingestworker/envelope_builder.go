package ingestworker

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
	"github.com/tidwall/gjson"
)

const (
	defaultEnvelopeVersion  = 1
	defaultPayloadVersion   = 1
	defaultRawPayloadType   = "ecoflow.mqtt.raw"
	defaultQuotaPayloadType = "ecoflow.quota.normalized"
)

func init() {
	// Reduce per-message UUIDv7 entropy syscall churn in high-throughput paths.
	uuid.EnableRandPool()
}

type mqttPayloadMeta struct {
	ID              gjson.Result
	Time            gjson.Result
	CmdID           gjson.Result
	CmdFunc         gjson.Result
	TypeCode        string
	Addr            string
	PayloadEncoding envelopev1.PayloadEncoding
}

type telemetryEnvelopeBuilder struct {
	deviceID           string
	normalizedSN       string
	normalizedProvider string
	credentialID       string
	shard              uint32
	shardCount         uint32
	disableLabels      bool
}

func buildTelemetryEnvelope(
	assignment controlplane.IngestAssignment,
	message ecoflowmqtt.Message,
	observedAt time.Time,
	cfg EcoFlowSessionConfig,
) (*envelopev1.TelemetryEnvelope, error) {
	builder := newTelemetryEnvelopeBuilder(assignment, cfg)
	return builder.Build(message, observedAt)
}

func newTelemetryEnvelopeBuilder(
	assignment controlplane.IngestAssignment,
	cfg EcoFlowSessionConfig,
) telemetryEnvelopeBuilder {
	normalizedSN := strings.ToUpper(strings.TrimSpace(assignment.ProviderDeviceID))
	shardCount := cfg.ShardCount
	if shardCount == 0 {
		shardCount = telemetrybus.DefaultShardCount
	}
	return telemetryEnvelopeBuilder{
		deviceID:           strings.TrimSpace(assignment.DeviceID),
		normalizedSN:       normalizedSN,
		normalizedProvider: sanitizeProvider(assignment.Provider),
		credentialID:       strings.TrimSpace(assignment.CredentialID),
		shard:              telemetrybus.ShardForDevice(normalizedSN, shardCount),
		shardCount:         shardCount,
		disableLabels:      cfg.DisableEnvelopeLabels,
	}
}

func (b telemetryEnvelopeBuilder) Build(
	message ecoflowmqtt.Message,
	observedAt time.Time,
) (*envelopev1.TelemetryEnvelope, error) {
	envelopeID, err := newEnvelopeID()
	if err != nil {
		return nil, err
	}

	meta := parseMQTTPayloadMeta(message.Payload)
	observed := observedAt.UTC()
	msgID := rawResultAsString(meta.ID)
	deviceTime := normalizeDeviceUnixMS(rawResultAsInt64(meta.Time), observed)
	var labels map[string]string
	if !b.disableLabels {
		labels = b.labels(meta)
	}

	payloadEncoding := meta.PayloadEncoding

	typeCode := strings.TrimSpace(meta.TypeCode)
	if typeCode == "" {
		typeCode = inferTypeCodeFromTopic(message.Topic)
	}

	return &envelopev1.TelemetryEnvelope{
		EnvelopeId:         envelopeID,
		EnvelopeVersion:    defaultEnvelopeVersion,
		DeviceId:           b.deviceID,
		EcoflowSn:          b.normalizedSN,
		Shard:              b.shard,
		ShardCount:         b.shardCount,
		MessageId:          msgID,
		DeviceTimeUnixMs:   deviceTime,
		ObservedTimeUnixMs: observed.UnixMilli(),
		IngestedTimeUnixMs: observed.UnixMilli(),
		SourceKind:         sourceKindFromTopic(message.Topic),
		Source:             "mqtt",
		TypeCode:           typeCode,
		PayloadType:        defaultRawPayloadType,
		PayloadVersion:     defaultPayloadVersion,
		PayloadEncoding:    payloadEncoding,
		Payload:            append([]byte(nil), message.Payload...),
		Labels:             labels,
	}, nil
}

func (b telemetryEnvelopeBuilder) BuildQuota(
	params map[string]any,
	observedAt time.Time,
) (*envelopev1.TelemetryEnvelope, error) {
	envelopeID, err := newEnvelopeID()
	if err != nil {
		return nil, err
	}
	observed := observedAt.UTC()
	payload, err := json.Marshal(map[string]any{
		"typeCode": "quota",
		"params":   params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal quota payload: %w", err)
	}
	var labels map[string]string
	if !b.disableLabels {
		labels = b.labels(mqttPayloadMeta{})
	}
	return &envelopev1.TelemetryEnvelope{
		EnvelopeId:         envelopeID,
		EnvelopeVersion:    defaultEnvelopeVersion,
		DeviceId:           b.deviceID,
		EcoflowSn:          b.normalizedSN,
		Shard:              b.shard,
		ShardCount:         b.shardCount,
		ObservedTimeUnixMs: observed.UnixMilli(),
		IngestedTimeUnixMs: observed.UnixMilli(),
		SourceKind:         envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA,
		Source:             "quota",
		TypeCode:           "quota",
		PayloadType:        defaultQuotaPayloadType,
		PayloadVersion:     defaultPayloadVersion,
		PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		Payload:            payload,
		Labels:             labels,
	}, nil
}

func (b telemetryEnvelopeBuilder) labels(meta mqttPayloadMeta) map[string]string {
	addr := strings.TrimSpace(meta.Addr)
	deviceTimeRaw := rawResultAsString(meta.Time)
	cmdID := rawResultAsString(meta.CmdID)
	cmdFunc := rawResultAsString(meta.CmdFunc)

	count := 0
	if b.normalizedProvider != "" {
		count++
	}
	if b.credentialID != "" {
		count++
	}
	if addr != "" {
		count++
	}
	if deviceTimeRaw != "" {
		count++
	}
	if cmdID != "" {
		count++
	}
	if cmdFunc != "" {
		count++
	}
	if count == 0 {
		return nil
	}
	labels := make(map[string]string, count)
	if b.normalizedProvider != "" {
		labels["provider"] = b.normalizedProvider
	}
	if b.credentialID != "" {
		labels["credential_id"] = b.credentialID
	}
	if addr != "" {
		labels["addr"] = addr
	}
	if deviceTimeRaw != "" {
		labels["device_time_raw"] = deviceTimeRaw
	}
	if cmdID != "" {
		labels["cmd_id"] = cmdID
	}
	if cmdFunc != "" {
		labels["cmd_func"] = cmdFunc
	}
	return labels
}

func newEnvelopeID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate envelope uuidv7: %w", err)
	}
	return id.String(), nil
}

func parseMQTTPayloadMeta(payload []byte) mqttPayloadMeta {
	var meta mqttPayloadMeta
	if len(payload) == 0 {
		return meta
	}
	if !gjson.ValidBytes(payload) {
		return meta
	}
	root := gjson.ParseBytes(payload)
	meta.ID = root.Get("id")
	meta.Time = root.Get("time")
	meta.CmdID = root.Get("cmdId")
	meta.CmdFunc = root.Get("cmdFunc")
	meta.TypeCode = strings.TrimSpace(root.Get("typeCode").String())
	meta.Addr = strings.TrimSpace(root.Get("addr").String())
	meta.PayloadEncoding = envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8
	return meta
}

func rawResultAsString(raw gjson.Result) string {
	if !raw.Exists() {
		return ""
	}
	switch raw.Type {
	case gjson.String:
		return strings.TrimSpace(raw.String())
	case gjson.Number:
		return strings.TrimSpace(raw.Raw)
	case gjson.True:
		return "true"
	case gjson.False:
		return "false"
	default:
		value := strings.TrimSpace(raw.String())
		if value != "" {
			return value
		}
		return strings.TrimSpace(raw.Raw)
	}
}

func rawResultAsInt64(raw gjson.Result) int64 {
	if !raw.Exists() {
		return 0
	}
	switch raw.Type {
	case gjson.Number:
		return int64(raw.Num)
	case gjson.String:
		clean := strings.TrimSpace(raw.String())
		if clean == "" {
			return 0
		}
		asInt, err := strconv.ParseInt(clean, 10, 64)
		if err == nil {
			return asInt
		}
		asFloat, err := strconv.ParseFloat(clean, 64)
		if err == nil {
			return int64(math.Trunc(asFloat))
		}
		return 0
	default:
		return 0
	}
}

func normalizeDeviceUnixMS(raw int64, observed time.Time) int64 {
	if raw <= 0 {
		return 0
	}
	observedMS := observed.UnixMilli()
	const oneYearMS = int64(365 * 24 * time.Hour / time.Millisecond)
	const lowerBound = int64(946684800000) // 2000-01-01 UTC
	if raw < lowerBound {
		return 0
	}
	if raw > observedMS+oneYearMS {
		return 0
	}
	return raw
}

func sourceKindFromTopic(topic string) envelopev1.SourceKind {
	if strings.HasSuffix(strings.TrimSpace(topic), "/quota") {
		return envelopev1.SourceKind_SOURCE_KIND_MQTT_QUOTA
	}
	return envelopev1.SourceKind_SOURCE_KIND_MQTT_STATUS
}

func inferTypeCodeFromTopic(topic string) string {
	trimmed := strings.TrimSpace(topic)
	if trimmed == "" {
		return ""
	}
	if strings.HasSuffix(trimmed, "/quota") {
		return "quota"
	}
	return "status"
}
