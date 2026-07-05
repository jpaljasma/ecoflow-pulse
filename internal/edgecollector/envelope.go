package edgecollector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrypayload"
)

const (
	edgeEnvelopeVersion = 1
	edgePayloadVersion  = 1
	edgeTelemetryType   = "edge.ecoflow.ble.metrics"
)

type TelemetrySample struct {
	CollectorID      string
	DeviceID         string
	Provider         string
	ProviderDeviceID string
	Transport        string
	ObservedAt       time.Time
	Params           map[string]any
}

type edgeTelemetryPayload struct {
	TypeCode string         `json:"typeCode"`
	Params   map[string]any `json:"params"`
}

func BuildTelemetryEnvelope(sample TelemetrySample, subjectCfg telemetrybus.SubjectConfig) (*envelopev1.TelemetryEnvelope, error) {
	return buildTelemetryEnvelope(sample, subjectCfg, false)
}

// BuildTelemetryEnvelopeWithOwnedParams skips the defensive params clone when
// the caller created the params map only for this envelope.
func BuildTelemetryEnvelopeWithOwnedParams(sample TelemetrySample, subjectCfg telemetrybus.SubjectConfig) (*envelopev1.TelemetryEnvelope, error) {
	return buildTelemetryEnvelope(sample, subjectCfg, true)
}

func buildTelemetryEnvelope(sample TelemetrySample, subjectCfg telemetrybus.SubjectConfig, ownsParams bool) (*envelopev1.TelemetryEnvelope, error) {
	deviceID := strings.TrimSpace(sample.DeviceID)
	providerDeviceID := strings.ToUpper(strings.TrimSpace(sample.ProviderDeviceID))
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}
	if providerDeviceID == "" {
		return nil, errors.New("provider device id is required")
	}
	params := sample.Params
	if !ownsParams {
		params = cloneMap(sample.Params)
	}
	if len(params) == 0 {
		return nil, errors.New("params are required")
	}
	transport := strings.ToLower(strings.TrimSpace(sample.Transport))
	if transport == "" {
		transport = "ble"
	}
	provider := strings.ToLower(strings.TrimSpace(sample.Provider))
	if provider == "" {
		provider = "ecoflow"
	}
	observedAt := sample.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now()
	}
	observedAt = observedAt.UTC()
	payload, err := json.Marshal(edgeTelemetryPayload{
		TypeCode: edgeTelemetryType,
		Params:   params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal edge telemetry payload: %w", err)
	}
	messageID, envelopeID := stableTelemetryIDs(deviceID, providerDeviceID, provider, transport, observedAt.UnixMilli(), payload)
	normalizedCfg := subjectCfg.Normalized()
	labels := make(map[string]string, 3)
	if collectorID := strings.TrimSpace(sample.CollectorID); collectorID != "" {
		labels["collector_id"] = collectorID
	}
	labels["provider"] = provider
	labels["transport"] = transport

	return &envelopev1.TelemetryEnvelope{
		EnvelopeId:         envelopeID,
		EnvelopeVersion:    edgeEnvelopeVersion,
		DeviceId:           deviceID,
		EcoflowSn:          providerDeviceID,
		MessageId:          messageID,
		Shard:              telemetrybus.ShardForDevice(deviceID, normalizedCfg.ShardCount),
		ShardCount:         normalizedCfg.ShardCount,
		ObservedTimeUnixMs: observedAt.UnixMilli(),
		IngestedTimeUnixMs: time.Now().UTC().UnixMilli(),
		SourceKind:         envelopev1.SourceKind_SOURCE_KIND_EDGE_LOCAL,
		Source:             transport,
		TypeCode:           edgeTelemetryType,
		PayloadType:        telemetrypayload.ProviderNormalizedPayloadType,
		PayloadVersion:     edgePayloadVersion,
		PayloadEncoding:    envelopev1.PayloadEncoding_PAYLOAD_ENCODING_JSON_UTF8,
		Payload:            payload,
		Labels:             labels,
	}, nil
}

func stableTelemetryIDs(deviceID string, providerDeviceID string, provider string, transport string, observedAtUnixMS int64, payload []byte) (string, string) {
	buf := make([]byte, 0, stableTelemetryIDCapacity(deviceID, providerDeviceID, provider, transport, payload))
	buf = append(buf, edgeTelemetryType...)
	buf = append(buf, 0)
	buf = append(buf, deviceID...)
	buf = append(buf, 0)
	buf = append(buf, providerDeviceID...)
	buf = append(buf, 0)
	buf = append(buf, provider...)
	buf = append(buf, 0)
	buf = append(buf, transport...)
	buf = append(buf, 0)
	buf = strconv.AppendInt(buf, observedAtUnixMS, 10)
	buf = append(buf, 0)
	buf = append(buf, payload...)
	sum := sha256.Sum256(buf)
	var envelopeID [16]byte
	copy(envelopeID[:], sum[:16])
	envelopeID[6] = (envelopeID[6] & 0x0f) | 0x80
	envelopeID[8] = (envelopeID[8] & 0x3f) | 0x80
	return "edge-telemetry-" + hex.EncodeToString(sum[:]), uuid.UUID(envelopeID).String()
}

func stableTelemetryIDCapacity(deviceID string, providerDeviceID string, provider string, transport string, payload []byte) int {
	const maxInt = int(^uint(0) >> 1)

	capacity := 0
	for _, size := range [...]int{
		len(edgeTelemetryType),
		len(deviceID),
		len(providerDeviceID),
		len(provider),
		len(transport),
		len(payload),
		6,
		20,
	} {
		if size > maxInt-capacity {
			return 0
		}
		capacity += size
	}
	return capacity
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if strings.TrimSpace(key) == "" {
			continue
		}
		out[key] = value
	}
	return out
}
