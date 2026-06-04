package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	edgev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/edge/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/edgecollector"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type edgeControlStore interface {
	CreateEdgeCollector(context.Context, controlplane.CreateEdgeCollectorInput) (controlplane.EdgeCollector, error)
	ListEdgeCollectors(context.Context, controlplane.ListEdgeCollectorsInput) ([]controlplane.EdgeCollector, error)
	EnrollEdgeCollector(context.Context, controlplane.EnrollEdgeCollectorInput) (controlplane.EdgeCollector, error)
	AuthenticateEdgeCollector(context.Context, controlplane.AuthenticateEdgeCollectorInput) (controlplane.EdgeCollector, error)
	UpdateEdgeCollectorHeartbeat(context.Context, controlplane.UpdateEdgeCollectorHeartbeatInput) (controlplane.EdgeCollector, error)
	UpsertEdgeDeviceSource(context.Context, controlplane.UpsertEdgeDeviceSourceInput) (controlplane.EdgeDeviceSource, error)
	ListEdgeDeviceSources(context.Context, controlplane.ListEdgeDeviceSourcesInput) ([]controlplane.EdgeDeviceSource, error)
	ApproveEdgeDeviceSource(context.Context, controlplane.ApproveEdgeDeviceSourceInput) (controlplane.ApprovedEdgeDeviceSource, error)
	GetLinkedEdgeDeviceSource(context.Context, controlplane.GetLinkedEdgeDeviceSourceInput) (controlplane.EdgeDeviceSource, error)
}

type EdgeIngestService struct {
	edgev1.UnimplementedEdgeIngestServiceServer

	log        *slog.Logger
	store      edgeControlStore
	publisher  telemetrybus.EnvelopePublisher
	subjectCfg telemetrybus.SubjectConfig
}

type EdgeIngestServiceDeps struct {
	Log        *slog.Logger
	Store      edgeControlStore
	Publisher  telemetrybus.EnvelopePublisher
	SubjectCfg telemetrybus.SubjectConfig
}

func NewEdgeIngestService(deps EdgeIngestServiceDeps) *EdgeIngestService {
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	return &EdgeIngestService{
		log:        log,
		store:      deps.Store,
		publisher:  deps.Publisher,
		subjectCfg: deps.SubjectCfg.Normalized(),
	}
}

func (s *EdgeIngestService) CreateCollector(ctx context.Context, req *edgev1.CreateCollectorRequest) (*edgev1.CreateCollectorResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.FailedPrecondition, "edge store not configured")
	}
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	setupToken, err := edgecollector.GenerateSecret("pulse_edge_setup")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate edge setup token: %v", err)
	}
	row, err := s.store.CreateEdgeCollector(ctx, controlplane.CreateEdgeCollectorInput{
		UserSubject:    userSubject,
		DisplayName:    req.GetDisplayName(),
		SetupTokenHash: edgecollector.HashCollectorSecret(setupToken),
	})
	if err != nil {
		return nil, edgeStoreStatus("create edge collector", err)
	}
	return &edgev1.CreateCollectorResponse{
		Collector:  edgeCollectorToProto(row),
		SetupToken: setupToken,
	}, nil
}

func (s *EdgeIngestService) ListCollectors(ctx context.Context, req *edgev1.ListCollectorsRequest) (*edgev1.ListCollectorsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.FailedPrecondition, "edge store not configured")
	}
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListEdgeCollectors(ctx, controlplane.ListEdgeCollectorsInput{UserSubject: userSubject})
	if err != nil {
		return nil, edgeStoreStatus("list edge collectors", err)
	}
	out := make([]*edgev1.EdgeCollector, 0, len(rows))
	for _, row := range rows {
		out = append(out, edgeCollectorToProto(row))
	}
	return &edgev1.ListCollectorsResponse{Collectors: out}, nil
}

func (s *EdgeIngestService) EnrollCollector(ctx context.Context, req *edgev1.EnrollCollectorRequest) (*edgev1.EnrollCollectorResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.FailedPrecondition, "edge store not configured")
	}
	setupToken := strings.TrimSpace(req.GetSetupToken())
	if setupToken == "" {
		return nil, status.Error(codes.InvalidArgument, "setup_token required")
	}
	collectorSecret, err := edgecollector.GenerateSecret("pulse_edge_secret")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate edge collector secret: %v", err)
	}
	row, err := s.store.EnrollEdgeCollector(ctx, controlplane.EnrollEdgeCollectorInput{
		SetupTokenHash:      edgecollector.HashCollectorSecret(setupToken),
		CollectorSecretHash: edgecollector.HashCollectorSecret(collectorSecret),
		CollectorVersion:    req.GetCollectorVersion(),
		Hostname:            req.GetHostname(),
	})
	if err != nil {
		return nil, edgeStoreStatus("enroll edge collector", err)
	}
	return &edgev1.EnrollCollectorResponse{
		Collector:       edgeCollectorToProto(row),
		CollectorSecret: collectorSecret,
	}, nil
}

func (s *EdgeIngestService) Heartbeat(ctx context.Context, req *edgev1.HeartbeatRequest) (*edgev1.HeartbeatResponse, error) {
	collector, err := s.authenticateCollector(ctx, req.GetCollectorSecret())
	if err != nil {
		return nil, err
	}
	row, err := s.store.UpdateEdgeCollectorHeartbeat(ctx, controlplane.UpdateEdgeCollectorHeartbeatInput{
		CollectorID:      collector.ID,
		CollectorVersion: req.GetCollectorVersion(),
		Hostname:         req.GetHostname(),
	})
	if err != nil {
		return nil, edgeStoreStatus("update edge collector heartbeat", err)
	}
	return &edgev1.HeartbeatResponse{Collector: edgeCollectorToProto(row)}, nil
}

func (s *EdgeIngestService) UploadDiscovery(ctx context.Context, req *edgev1.UploadDiscoveryRequest) (*edgev1.UploadDiscoveryResponse, error) {
	collector, err := s.authenticateCollector(ctx, req.GetCollectorSecret())
	if err != nil {
		return nil, err
	}
	var accepted uint32
	for _, discovery := range req.GetDiscoveries() {
		if strings.TrimSpace(discovery.GetProviderDeviceId()) == "" {
			continue
		}
		observedAt := time.UnixMilli(discovery.GetObservedAtUnixMs()).UTC()
		if discovery.GetObservedAtUnixMs() <= 0 {
			observedAt = time.Now().UTC()
		}
		_, err := s.store.UpsertEdgeDeviceSource(ctx, controlplane.UpsertEdgeDeviceSourceInput{
			CollectorID:      collector.ID,
			Provider:         discovery.GetProvider(),
			Transport:        discovery.GetTransport(),
			ProviderDeviceID: discovery.GetProviderDeviceId(),
			DisplayName:      discovery.GetDisplayName(),
			Model:            discovery.GetModel(),
			AddressHash:      edgecollector.HashCollectorSecret(discovery.GetAddress()),
			RSSIDBm:          discovery.GetRssiDbm(),
			Metadata:         structToMap(discovery.GetMetadata()),
			ObservedAt:       observedAt,
		})
		if err != nil {
			return nil, edgeStoreStatus("upsert edge discovery", err)
		}
		accepted++
	}
	return &edgev1.UploadDiscoveryResponse{AcceptedCount: accepted}, nil
}

func (s *EdgeIngestService) ListDeviceSources(ctx context.Context, req *edgev1.ListDeviceSourcesRequest) (*edgev1.ListDeviceSourcesResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.FailedPrecondition, "edge store not configured")
	}
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListEdgeDeviceSources(ctx, controlplane.ListEdgeDeviceSourcesInput{
		UserSubject: userSubject,
		CollectorID: req.GetCollectorId(),
		Status:      req.GetStatus(),
	})
	if err != nil {
		return nil, edgeStoreStatus("list edge device sources", err)
	}
	out := make([]*edgev1.EdgeDeviceSource, 0, len(rows))
	for _, row := range rows {
		out = append(out, edgeDeviceSourceToProto(row))
	}
	return &edgev1.ListDeviceSourcesResponse{Sources: out}, nil
}

func (s *EdgeIngestService) ApproveDeviceSource(ctx context.Context, req *edgev1.ApproveDeviceSourceRequest) (*edgev1.ApproveDeviceSourceResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.FailedPrecondition, "edge store not configured")
	}
	userSubject, err := resolveUserSubject(ctx, req.GetUserSubject())
	if err != nil {
		return nil, err
	}
	approved, err := s.store.ApproveEdgeDeviceSource(ctx, controlplane.ApproveEdgeDeviceSourceInput{
		UserSubject: userSubject,
		SourceID:    req.GetSourceId(),
		DeviceID:    req.GetDeviceId(),
		ProductName: req.GetProductName(),
		Model:       req.GetModel(),
	})
	if err != nil {
		return nil, edgeStoreStatus("approve edge device source", err)
	}
	return &edgev1.ApproveDeviceSourceResponse{
		Source:   edgeDeviceSourceToProto(approved.Source),
		DeviceId: approved.Device.DeviceID,
	}, nil
}

func (s *EdgeIngestService) UploadTelemetryBatch(ctx context.Context, req *edgev1.UploadTelemetryBatchRequest) (*edgev1.UploadTelemetryBatchResponse, error) {
	collector, err := s.authenticateCollector(ctx, req.GetCollectorSecret())
	if err != nil {
		return nil, err
	}
	if s.publisher == nil {
		return nil, status.Error(codes.FailedPrecondition, "edge ingest publisher not configured")
	}
	var accepted, dropped uint32
	for _, sample := range req.GetSamples() {
		source, err := s.store.GetLinkedEdgeDeviceSource(ctx, controlplane.GetLinkedEdgeDeviceSourceInput{
			CollectorID:      collector.ID,
			Provider:         sample.GetProvider(),
			Transport:        sample.GetTransport(),
			ProviderDeviceID: sample.GetProviderDeviceId(),
		})
		if err != nil {
			if errors.Is(err, controlplane.ErrEdgeDeviceSourceNotFound) {
				dropped++
				continue
			}
			return nil, edgeStoreStatus("resolve linked edge source", err)
		}
		observedAt := time.UnixMilli(sample.GetObservedAtUnixMs()).UTC()
		if sample.GetObservedAtUnixMs() <= 0 {
			observedAt = time.Now().UTC()
		}
		params := edgecollector.NormalizeEcoFlowBLEMetrics(structToMap(sample.GetMetrics()))
		if len(params) == 0 {
			dropped++
			continue
		}
		envelope, err := edgecollector.BuildTelemetryEnvelope(edgecollector.TelemetrySample{
			CollectorID:      collector.ID,
			DeviceID:         source.LinkedDeviceID,
			Provider:         source.Provider,
			ProviderDeviceID: source.ProviderDeviceID,
			Transport:        source.Transport,
			ObservedAt:       observedAt,
			Params:           params,
			ClientSampleID:   sample.GetClientSampleId(),
		}, s.subjectCfg)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "build edge telemetry envelope: %v", err)
		}
		if err := telemetrybus.PublishEnvelope(ctx, s.publisher, envelope); err != nil {
			return nil, status.Errorf(codes.Unavailable, "publish edge telemetry: %v", err)
		}
		accepted++
	}
	return &edgev1.UploadTelemetryBatchResponse{AcceptedCount: accepted, DroppedCount: dropped}, nil
}

func (s *EdgeIngestService) authenticateCollector(ctx context.Context, collectorSecret string) (controlplane.EdgeCollector, error) {
	if s.store == nil {
		return controlplane.EdgeCollector{}, status.Error(codes.FailedPrecondition, "edge store not configured")
	}
	if strings.TrimSpace(collectorSecret) == "" {
		return controlplane.EdgeCollector{}, status.Error(codes.Unauthenticated, "collector secret required")
	}
	collector, err := s.store.AuthenticateEdgeCollector(ctx, controlplane.AuthenticateEdgeCollectorInput{
		CollectorSecretHash: edgecollector.HashCollectorSecret(collectorSecret),
	})
	if err != nil {
		if errors.Is(err, controlplane.ErrEdgeCollectorNotFound) {
			return controlplane.EdgeCollector{}, status.Error(codes.Unauthenticated, "collector secret rejected")
		}
		return controlplane.EdgeCollector{}, edgeStoreStatus("authenticate edge collector", err)
	}
	return collector, nil
}

func edgeStoreStatus(operation string, err error) error {
	switch {
	case errors.Is(err, controlplane.ErrUserNotFound),
		errors.Is(err, controlplane.ErrDeviceNotFound),
		errors.Is(err, controlplane.ErrEdgeCollectorNotFound),
		errors.Is(err, controlplane.ErrEdgeDeviceSourceNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, controlplane.ErrPermissionDenied):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return controlPlaneStoreError(operation, err)
	}
}

func edgeCollectorToProto(in controlplane.EdgeCollector) *edgev1.EdgeCollector {
	out := &edgev1.EdgeCollector{
		Id:               in.ID,
		DisplayName:      in.DisplayName,
		IsActive:         in.IsActive,
		CollectorVersion: in.CollectorVersion,
		Hostname:         in.Hostname,
		CreatedAtUnixMs:  in.CreatedAt.UnixMilli(),
		UpdatedAtUnixMs:  in.UpdatedAt.UnixMilli(),
	}
	if !in.LastHeartbeatAt.IsZero() {
		out.LastHeartbeatAtUnixMs = in.LastHeartbeatAt.UnixMilli()
	}
	return out
}

func edgeDeviceSourceToProto(in controlplane.EdgeDeviceSource) *edgev1.EdgeDeviceSource {
	return &edgev1.EdgeDeviceSource{
		Id:               in.ID,
		CollectorId:      in.CollectorID,
		Provider:         in.Provider,
		Transport:        in.Transport,
		ProviderDeviceId: in.ProviderDeviceID,
		DisplayName:      in.DisplayName,
		Model:            in.Model,
		Status:           in.Status,
		LinkedDeviceId:   in.LinkedDeviceID,
		RssiDbm:          in.RSSIDBm,
		LastSeenAtUnixMs: in.LastSeenAt.UnixMilli(),
		CreatedAtUnixMs:  in.CreatedAt.UnixMilli(),
		UpdatedAtUnixMs:  in.UpdatedAt.UnixMilli(),
		Metadata:         mapToStructProto(in.Metadata),
	}
}
