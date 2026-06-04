package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	edgev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/edge/v1"
	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestEdgeIngestServiceDiscoveryApprovalAndTelemetryPublish(t *testing.T) {
	t.Parallel()

	store := controlplane.NewMemoryStore()
	store.EnsureUser("user-1")
	publisher := &testEnvelopePublisher{}
	svc := NewEdgeIngestService(EdgeIngestServiceDeps{
		Store:      store,
		Publisher:  publisher,
		SubjectCfg: telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 8},
	})

	created, err := svc.CreateCollector(context.Background(), &edgev1.CreateCollectorRequest{
		UserSubject: "user-1",
		DisplayName: "Pi 5",
	})
	if err != nil {
		t.Fatalf("CreateCollector failed: %v", err)
	}
	if created.GetSetupToken() == "" || created.GetCollector().GetIsActive() {
		t.Fatalf("unexpected create response: %+v", created)
	}

	enrolled, err := svc.EnrollCollector(context.Background(), &edgev1.EnrollCollectorRequest{
		SetupToken:       created.GetSetupToken(),
		CollectorVersion: "test",
		Hostname:         "pi",
	})
	if err != nil {
		t.Fatalf("EnrollCollector failed: %v", err)
	}
	if enrolled.GetCollectorSecret() == "" || !enrolled.GetCollector().GetIsActive() {
		t.Fatalf("unexpected enroll response: %+v", enrolled)
	}

	metadata, err := structpb.NewStruct(map[string]any{"model_prefix": "DEMO"})
	if err != nil {
		t.Fatalf("new metadata struct: %v", err)
	}
	_, err = svc.UploadDiscovery(context.Background(), &edgev1.UploadDiscoveryRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Discoveries: []*edgev1.EdgeDiscovery{{
			Provider:         controlplane.ProviderEcoFlow,
			Transport:        "ble",
			ProviderDeviceId: "DEMOEDGE0001",
			DisplayName:      "Demo edge device",
			Model:            "EcoFlow RIVER 3 Plus",
			Address:          "demo-ble-address",
			RssiDbm:          -59,
			ObservedAtUnixMs: time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC).UnixMilli(),
			Metadata:         metadata,
		}},
	})
	if err != nil {
		t.Fatalf("UploadDiscovery failed: %v", err)
	}

	sources, err := svc.ListDeviceSources(context.Background(), &edgev1.ListDeviceSourcesRequest{
		UserSubject: "user-1",
		Status:      "pending",
	})
	if err != nil {
		t.Fatalf("ListDeviceSources failed: %v", err)
	}
	if got := len(sources.GetSources()); got != 1 {
		t.Fatalf("source count=%d want 1", got)
	}

	approved, err := svc.ApproveDeviceSource(context.Background(), &edgev1.ApproveDeviceSourceRequest{
		UserSubject: "user-1",
		SourceId:    sources.GetSources()[0].GetId(),
	})
	if err != nil {
		t.Fatalf("ApproveDeviceSource failed: %v", err)
	}
	if approved.GetDeviceId() == "" || approved.GetSource().GetStatus() != "linked" {
		t.Fatalf("unexpected approval: %+v", approved)
	}

	metrics, err := structpb.NewStruct(map[string]any{
		"battery_soc_percent":             99,
		"output_power_w":                  118,
		"pv_input_power_w":                12,
		"battery_discharge_remaining_min": 88,
	})
	if err != nil {
		t.Fatalf("new metrics struct: %v", err)
	}
	resp, err := svc.UploadTelemetryBatch(context.Background(), &edgev1.UploadTelemetryBatchRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Samples: []*edgev1.EdgeTelemetrySample{{
			Provider:         controlplane.ProviderEcoFlow,
			Transport:        "ble",
			ProviderDeviceId: "DEMOEDGE0001",
			ObservedAtUnixMs: time.Date(2026, 5, 28, 12, 5, 0, 0, time.UTC).UnixMilli(),
			Metrics:          metrics,
			ClientSampleId:   "edge-sample-1",
		}},
	})
	if err != nil {
		t.Fatalf("UploadTelemetryBatch failed: %v", err)
	}
	if resp.GetAcceptedCount() != 1 || resp.GetDroppedCount() != 0 {
		t.Fatalf("unexpected telemetry response: %+v", resp)
	}
	if got := len(publisher.envelopes); got != 1 {
		t.Fatalf("published envelope count=%d want 1", got)
	}
	envelope := publisher.envelopes[0]
	if envelope.GetSourceKind() != envelopev1.SourceKind_SOURCE_KIND_EDGE_LOCAL {
		t.Fatalf("source kind=%v", envelope.GetSourceKind())
	}
	if envelope.GetDeviceId() != approved.GetDeviceId() || envelope.GetSource() != "ble" {
		t.Fatalf("envelope identity/source mismatch: %+v", envelope)
	}
	if envelope.GetMessageId() != "edge-sample-1" {
		t.Fatalf("message_id=%q want edge-sample-1", envelope.GetMessageId())
	}
	var payload struct {
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal(envelope.GetPayload(), &payload); err != nil {
		t.Fatalf("unmarshal envelope payload: %v", err)
	}
	if got := payload.Params["wattsOutSum"]; got != float64(118) {
		t.Fatalf("wattsOutSum=%v want 118", got)
	}
	if got := payload.Params["pv1ChargeWatts"]; got != float64(12) {
		t.Fatalf("pv1ChargeWatts=%v want 12", got)
	}
}

func TestEdgeIngestServiceRejectsBadCollectorSecret(t *testing.T) {
	t.Parallel()

	store := controlplane.NewMemoryStore()
	svc := NewEdgeIngestService(EdgeIngestServiceDeps{Store: store})
	_, err := svc.Heartbeat(context.Background(), &edgev1.HeartbeatRequest{CollectorSecret: "bad"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("Heartbeat error=%v code=%v want unauthenticated", err, status.Code(err))
	}
}

func TestEdgeIngestServiceDropsEmptyTelemetrySamples(t *testing.T) {
	t.Parallel()

	store := controlplane.NewMemoryStore()
	store.EnsureUser("user-1")
	publisher := &testEnvelopePublisher{}
	svc := NewEdgeIngestService(EdgeIngestServiceDeps{
		Store:      store,
		Publisher:  publisher,
		SubjectCfg: telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 8},
	})
	created, err := svc.CreateCollector(context.Background(), &edgev1.CreateCollectorRequest{UserSubject: "user-1"})
	if err != nil {
		t.Fatalf("CreateCollector failed: %v", err)
	}
	enrolled, err := svc.EnrollCollector(context.Background(), &edgev1.EnrollCollectorRequest{SetupToken: created.GetSetupToken()})
	if err != nil {
		t.Fatalf("EnrollCollector failed: %v", err)
	}
	if _, err := svc.UploadDiscovery(context.Background(), &edgev1.UploadDiscoveryRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Discoveries: []*edgev1.EdgeDiscovery{{
			Provider:         controlplane.ProviderEcoFlow,
			Transport:        "ble",
			ProviderDeviceId: "DEMOEDGE0002",
		}},
	}); err != nil {
		t.Fatalf("UploadDiscovery failed: %v", err)
	}
	sources, err := svc.ListDeviceSources(context.Background(), &edgev1.ListDeviceSourcesRequest{UserSubject: "user-1"})
	if err != nil {
		t.Fatalf("ListDeviceSources failed: %v", err)
	}
	if _, err := svc.ApproveDeviceSource(context.Background(), &edgev1.ApproveDeviceSourceRequest{
		UserSubject: "user-1",
		SourceId:    sources.GetSources()[0].GetId(),
	}); err != nil {
		t.Fatalf("ApproveDeviceSource failed: %v", err)
	}
	emptyMetrics, err := structpb.NewStruct(map[string]any{})
	if err != nil {
		t.Fatalf("new empty metrics: %v", err)
	}

	resp, err := svc.UploadTelemetryBatch(context.Background(), &edgev1.UploadTelemetryBatchRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Samples: []*edgev1.EdgeTelemetrySample{{
			Provider:         controlplane.ProviderEcoFlow,
			Transport:        "ble",
			ProviderDeviceId: "DEMOEDGE0002",
			Metrics:          emptyMetrics,
		}},
	})
	if err != nil {
		t.Fatalf("UploadTelemetryBatch failed: %v", err)
	}
	if resp.GetAcceptedCount() != 0 || resp.GetDroppedCount() != 1 {
		t.Fatalf("unexpected telemetry response: %+v", resp)
	}
	if len(publisher.envelopes) != 0 {
		t.Fatalf("published envelopes=%d want 0", len(publisher.envelopes))
	}
}

type testEnvelopePublisher struct {
	envelopes []*envelopev1.TelemetryEnvelope
}

func (p *testEnvelopePublisher) PublishEnvelope(_ context.Context, envelope *envelopev1.TelemetryEnvelope) error {
	p.envelopes = append(p.envelopes, envelope)
	return nil
}

func (p *testEnvelopePublisher) Close() error {
	return nil
}
