package main

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
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
	if envelope.GetMessageId() == "" || !strings.HasPrefix(envelope.GetMessageId(), "edge-telemetry-") {
		t.Fatalf("message_id=%q want server-derived edge telemetry id", envelope.GetMessageId())
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

func TestEdgeIngestServiceRejectsOversizedEdgeBatches(t *testing.T) {
	t.Parallel()

	store := controlplane.NewMemoryStore()
	store.EnsureUser("user-1")
	svc := NewEdgeIngestService(EdgeIngestServiceDeps{
		Store:     store,
		Publisher: &testEnvelopePublisher{},
	})
	created, err := svc.CreateCollector(context.Background(), &edgev1.CreateCollectorRequest{UserSubject: "user-1"})
	if err != nil {
		t.Fatalf("CreateCollector failed: %v", err)
	}
	enrolled, err := svc.EnrollCollector(context.Background(), &edgev1.EnrollCollectorRequest{SetupToken: created.GetSetupToken()})
	if err != nil {
		t.Fatalf("EnrollCollector failed: %v", err)
	}

	_, err = svc.UploadDiscovery(context.Background(), &edgev1.UploadDiscoveryRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Discoveries:     make([]*edgev1.EdgeDiscovery, maxEdgeDiscoveryBatchRecords+1),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("UploadDiscovery code=%v want InvalidArgument", status.Code(err))
	}

	_, err = svc.UploadTelemetryBatch(context.Background(), &edgev1.UploadTelemetryBatchRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Samples:         make([]*edgev1.EdgeTelemetrySample, maxEdgeTelemetryBatchSamples+1),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("UploadTelemetryBatch code=%v want InvalidArgument", status.Code(err))
	}
}

func TestEdgeIngestServiceEnrollReturnsCollectorEnv(t *testing.T) {
	t.Parallel()

	store := controlplane.NewMemoryStore()
	store.EnsureUser("user-1")
	svc := NewEdgeIngestService(EdgeIngestServiceDeps{
		Store: store,
		EnvResolver: edgeCollectorEnvResolverFunc(func(_ context.Context, collector controlplane.EdgeCollector) (map[string]string, error) {
			if collector.UserID == "" {
				t.Fatal("collector passed to env resolver did not include user id")
			}
			return map[string]string{
				"ECOFLOW_BLE_USER_ID": "  ecoflow-user-1  ",
				"EMPTY":               "",
				"":                    "ignored",
			}, nil
		}),
	})
	created, err := svc.CreateCollector(context.Background(), &edgev1.CreateCollectorRequest{
		UserSubject: "user-1",
		DisplayName: "Pi 5",
	})
	if err != nil {
		t.Fatalf("CreateCollector failed: %v", err)
	}

	enrolled, err := svc.EnrollCollector(context.Background(), &edgev1.EnrollCollectorRequest{
		SetupToken: created.GetSetupToken(),
	})
	if err != nil {
		t.Fatalf("EnrollCollector failed: %v", err)
	}
	if got := enrolled.GetCollectorEnv()["ECOFLOW_BLE_USER_ID"]; got != "ecoflow-user-1" {
		t.Fatalf("collector env ECOFLOW_BLE_USER_ID=%q", got)
	}
	if _, ok := enrolled.GetCollectorEnv()["EMPTY"]; ok {
		t.Fatalf("collector env should omit empty values: %#v", enrolled.GetCollectorEnv())
	}

	listed, err := svc.ListCollectors(context.Background(), &edgev1.ListCollectorsRequest{UserSubject: "user-1"})
	if err != nil {
		t.Fatalf("ListCollectors failed: %v", err)
	}
	if len(listed.GetCollectors()) != 1 {
		t.Fatalf("collector count=%d want 1", len(listed.GetCollectors()))
	}
}

func TestEdgeIngestServiceEnrollDoesNotConsumeSetupTokenWhenEnvFails(t *testing.T) {
	t.Parallel()

	store := controlplane.NewMemoryStore()
	store.EnsureUser("user-1")
	var calls atomic.Int32
	svc := NewEdgeIngestService(EdgeIngestServiceDeps{
		Store: store,
		EnvResolver: edgeCollectorEnvResolverFunc(func(context.Context, controlplane.EdgeCollector) (map[string]string, error) {
			if calls.Add(1) == 1 {
				return nil, errors.New("temporary auth lookup failure")
			}
			return map[string]string{"ECOFLOW_BLE_USER_ID": "ble-user-123"}, nil
		}),
	})
	created, err := svc.CreateCollector(context.Background(), &edgev1.CreateCollectorRequest{
		UserSubject: "user-1",
		DisplayName: "Pi 5",
	})
	if err != nil {
		t.Fatalf("CreateCollector failed: %v", err)
	}
	_, err = svc.EnrollCollector(context.Background(), &edgev1.EnrollCollectorRequest{
		SetupToken: created.GetSetupToken(),
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("EnrollCollector error=%v code=%v want Internal", err, status.Code(err))
	}

	enrolled, err := svc.EnrollCollector(context.Background(), &edgev1.EnrollCollectorRequest{
		SetupToken: created.GetSetupToken(),
	})
	if err != nil {
		t.Fatalf("EnrollCollector retry failed: %v", err)
	}
	if enrolled.GetCollectorSecret() == "" {
		t.Fatalf("expected collector secret")
	}
	if got := enrolled.GetCollectorEnv()["ECOFLOW_BLE_USER_ID"]; got != "ble-user-123" {
		t.Fatalf("collector env ECOFLOW_BLE_USER_ID=%q", got)
	}
	if _, err := svc.Heartbeat(context.Background(), &edgev1.HeartbeatRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
	}); err != nil {
		t.Fatalf("Heartbeat with returned collector secret failed: %v", err)
	}
}

func TestEdgeCollectorEnvResolverUsesActiveEcoFlowBLECredential(t *testing.T) {
	t.Parallel()

	store := controlplane.NewMemoryStore()
	userID := store.EnsureUser("dev-user")
	if _, err := store.CreateProviderCredential(context.Background(), controlplane.CreateProviderCredentialInput{
		UserSubject: "dev-user",
		Provider:    controlplane.ProviderEcoFlowBLE,
		AccessKey:   "owner@example.test",
		SecretKey:   "ble-user-123",
		IsActive:    true,
	}); err != nil {
		t.Fatalf("CreateProviderCredential failed: %v", err)
	}
	resolver := newEdgeCollectorEnvResolver(store)
	if resolver == nil {
		t.Fatalf("resolver is nil")
	}
	env, err := resolver.CollectorEnv(context.Background(), controlplane.EdgeCollector{UserID: userID})
	if err != nil {
		t.Fatalf("CollectorEnv failed: %v", err)
	}
	if got := env["ECOFLOW_BLE_USER_ID"]; got != "ble-user-123" {
		t.Fatalf("ECOFLOW_BLE_USER_ID=%q", got)
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

func TestEdgeIngestServiceCachesLinkedSourceLookupWithinTelemetryBatch(t *testing.T) {
	t.Parallel()

	store := &linkedLookupCountingStore{MemoryStore: controlplane.NewMemoryStore()}
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
			ProviderDeviceId: "DEMOEDGE0003",
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
	metrics, err := structpb.NewStruct(map[string]any{"battery_soc_percent": 88})
	if err != nil {
		t.Fatalf("new metrics: %v", err)
	}
	emptyMetrics, err := structpb.NewStruct(map[string]any{})
	if err != nil {
		t.Fatalf("new empty metrics: %v", err)
	}

	resp, err := svc.UploadTelemetryBatch(context.Background(), &edgev1.UploadTelemetryBatchRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Samples: []*edgev1.EdgeTelemetrySample{
			{
				Provider:         controlplane.ProviderEcoFlow,
				Transport:        "ble",
				ProviderDeviceId: "DEMOEDGE0003",
				ObservedAtUnixMs: time.Date(2026, 5, 28, 12, 5, 0, 0, time.UTC).UnixMilli(),
				Metrics:          metrics,
				ClientSampleId:   "edge-sample-1",
			},
			{
				Provider:         controlplane.ProviderEcoFlow,
				Transport:        "ble",
				ProviderDeviceId: "DEMOEDGE0003",
				ObservedAtUnixMs: time.Date(2026, 5, 28, 12, 5, 1, 0, time.UTC).UnixMilli(),
				Metrics:          metrics,
				ClientSampleId:   "edge-sample-2",
			},
			{
				Provider:         controlplane.ProviderEcoFlow,
				Transport:        "ble",
				ProviderDeviceId: "DEMOEDGE0003",
				ObservedAtUnixMs: time.Date(2026, 5, 28, 12, 5, 2, 0, time.UTC).UnixMilli(),
				Metrics:          emptyMetrics,
				ClientSampleId:   "edge-sample-empty",
			},
		},
	})
	if err != nil {
		t.Fatalf("UploadTelemetryBatch failed: %v", err)
	}
	if resp.GetAcceptedCount() != 2 || resp.GetDroppedCount() != 1 {
		t.Fatalf("unexpected telemetry response: %+v", resp)
	}
	if got := store.linkedLookups.Load(); got != 1 {
		t.Fatalf("linked source lookups=%d want 1", got)
	}
	if got := len(publisher.envelopes); got != 2 {
		t.Fatalf("published envelopes=%d want 2", got)
	}
}

func TestEdgeIngestServiceDropsDuplicateTelemetrySamplesWithinBatch(t *testing.T) {
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
			ProviderDeviceId: "DEMOEDGE0005",
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
	metrics, err := structpb.NewStruct(map[string]any{"battery_soc_percent": 88})
	if err != nil {
		t.Fatalf("new metrics: %v", err)
	}
	observedAt := time.Date(2026, 5, 28, 12, 5, 0, 0, time.UTC).UnixMilli()

	resp, err := svc.UploadTelemetryBatch(context.Background(), &edgev1.UploadTelemetryBatchRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Samples: []*edgev1.EdgeTelemetrySample{
			{
				Provider:         controlplane.ProviderEcoFlow,
				Transport:        "ble",
				ProviderDeviceId: "DEMOEDGE0005",
				ObservedAtUnixMs: observedAt,
				Metrics:          metrics,
				ClientSampleId:   "client-sample-a",
			},
			{
				Provider:         controlplane.ProviderEcoFlow,
				Transport:        "ble",
				ProviderDeviceId: "DEMOEDGE0005",
				ObservedAtUnixMs: observedAt,
				Metrics:          metrics,
				ClientSampleId:   "client-sample-b",
			},
		},
	})
	if err != nil {
		t.Fatalf("UploadTelemetryBatch failed: %v", err)
	}
	if resp.GetAcceptedCount() != 1 || resp.GetDroppedCount() != 1 {
		t.Fatalf("unexpected telemetry response: %+v", resp)
	}
	if got := len(publisher.envelopes); got != 1 {
		t.Fatalf("published envelopes=%d want 1", got)
	}
}

type linkedLookupCountingStore struct {
	*controlplane.MemoryStore
	linkedLookups atomic.Int32
}

func (s *linkedLookupCountingStore) GetLinkedEdgeDeviceSource(ctx context.Context, in controlplane.GetLinkedEdgeDeviceSourceInput) (controlplane.EdgeDeviceSource, error) {
	s.linkedLookups.Add(1)
	return s.MemoryStore.GetLinkedEdgeDeviceSource(ctx, in)
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

type discardEnvelopePublisher struct{}

func (discardEnvelopePublisher) PublishEnvelope(context.Context, *envelopev1.TelemetryEnvelope) error {
	return nil
}

func (discardEnvelopePublisher) Close() error {
	return nil
}

func BenchmarkEdgeIngestUploadTelemetryBatch(b *testing.B) {
	store := controlplane.NewMemoryStore()
	store.EnsureUser("user-1")
	svc := NewEdgeIngestService(EdgeIngestServiceDeps{
		Store:      store,
		Publisher:  discardEnvelopePublisher{},
		SubjectCfg: telemetrybus.SubjectConfig{Prefix: "pulse", ShardCount: 8},
	})
	created, err := svc.CreateCollector(context.Background(), &edgev1.CreateCollectorRequest{UserSubject: "user-1"})
	if err != nil {
		b.Fatalf("CreateCollector failed: %v", err)
	}
	enrolled, err := svc.EnrollCollector(context.Background(), &edgev1.EnrollCollectorRequest{SetupToken: created.GetSetupToken()})
	if err != nil {
		b.Fatalf("EnrollCollector failed: %v", err)
	}
	if _, err := svc.UploadDiscovery(context.Background(), &edgev1.UploadDiscoveryRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Discoveries: []*edgev1.EdgeDiscovery{{
			Provider:         controlplane.ProviderEcoFlow,
			Transport:        "ble",
			ProviderDeviceId: "DEMOEDGE0004",
			DisplayName:      "Demo edge device",
			ObservedAtUnixMs: time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC).UnixMilli(),
		}},
	}); err != nil {
		b.Fatalf("UploadDiscovery failed: %v", err)
	}
	sources, err := svc.ListDeviceSources(context.Background(), &edgev1.ListDeviceSourcesRequest{UserSubject: "user-1"})
	if err != nil {
		b.Fatalf("ListDeviceSources failed: %v", err)
	}
	if _, err := svc.ApproveDeviceSource(context.Background(), &edgev1.ApproveDeviceSourceRequest{
		UserSubject: "user-1",
		SourceId:    sources.GetSources()[0].GetId(),
	}); err != nil {
		b.Fatalf("ApproveDeviceSource failed: %v", err)
	}
	metrics, err := structpb.NewStruct(map[string]any{
		"battery_soc_percent":             99,
		"input_power_w":                   149,
		"output_power_w":                  118,
		"pv_input_power_w":                12,
		"battery_discharge_remaining_min": 88,
	})
	if err != nil {
		b.Fatalf("new metrics struct: %v", err)
	}
	req := &edgev1.UploadTelemetryBatchRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Samples:         make([]*edgev1.EdgeTelemetrySample, 32),
	}
	for i := range req.Samples {
		req.Samples[i] = &edgev1.EdgeTelemetrySample{
			Provider:         controlplane.ProviderEcoFlow,
			Transport:        "ble",
			ProviderDeviceId: "DEMOEDGE0004",
			ObservedAtUnixMs: time.Date(2026, 5, 28, 12, 5, 0, 0, time.UTC).UnixMilli() + int64(i),
			Metrics:          metrics,
			ClientSampleId:   "edge-sample-bench-" + strconv.Itoa(i),
		}
	}
	singleReq := &edgev1.UploadTelemetryBatchRequest{
		CollectorSecret: enrolled.GetCollectorSecret(),
		Samples:         req.Samples[:1],
	}

	b.Run("single", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			resp, err := svc.UploadTelemetryBatch(context.Background(), singleReq)
			if err != nil {
				b.Fatalf("UploadTelemetryBatch single failed: %v", err)
			}
			if resp.GetAcceptedCount() != 1 {
				b.Fatalf("accepted=%d want 1", resp.GetAcceptedCount())
			}
		}
	})
	b.Run("batch32", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			resp, err := svc.UploadTelemetryBatch(context.Background(), req)
			if err != nil {
				b.Fatalf("UploadTelemetryBatch batch failed: %v", err)
			}
			if resp.GetAcceptedCount() != uint32(len(req.GetSamples())) {
				b.Fatalf("accepted=%d want %d", resp.GetAcceptedCount(), len(req.GetSamples()))
			}
		}
	})
}
