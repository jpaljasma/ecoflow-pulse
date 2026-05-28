package edgecollector

import (
	"encoding/json"
	"errors"
	"fmt"
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

func BuildTelemetryEnvelope(sample TelemetrySample, subjectCfg telemetrybus.SubjectConfig) (*envelopev1.TelemetryEnvelope, error) {
	deviceID := strings.TrimSpace(sample.DeviceID)
	providerDeviceID := strings.ToUpper(strings.TrimSpace(sample.ProviderDeviceID))
	if deviceID == "" {
		return nil, errors.New("device id is required")
	}
	if providerDeviceID == "" {
		return nil, errors.New("provider device id is required")
	}
	params := cloneMap(sample.Params)
	if len(params) == 0 {
		return nil, errors.New("params are required")
	}
	observedAt := sample.ObservedAt
	if observedAt.IsZero() {
		observedAt = time.Now()
	}
	observedAt = observedAt.UTC()
	payload, err := json.Marshal(map[string]any{
		"typeCode": edgeTelemetryType,
		"params":   params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal edge telemetry payload: %w", err)
	}
	envelopeID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate envelope uuidv7: %w", err)
	}
	normalizedCfg := subjectCfg.Normalized()
	transport := strings.ToLower(strings.TrimSpace(sample.Transport))
	if transport == "" {
		transport = "ble"
	}
	provider := strings.ToLower(strings.TrimSpace(sample.Provider))
	if provider == "" {
		provider = "ecoflow"
	}
	labels := map[string]string{
		"collector_id": strings.TrimSpace(sample.CollectorID),
		"provider":     provider,
		"transport":    transport,
	}
	for key, value := range labels {
		if value == "" {
			delete(labels, key)
		}
	}

	return &envelopev1.TelemetryEnvelope{
		EnvelopeId:         envelopeID.String(),
		EnvelopeVersion:    edgeEnvelopeVersion,
		DeviceId:           deviceID,
		EcoflowSn:          providerDeviceID,
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
