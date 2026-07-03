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

const (
	maxEdgeDiscoveryBatchRecords = 256
	maxEdgeTelemetryBatchSamples = 512
)

type edgeControlStore interface {
	CreateEdgeCollector(context.Context, controlplane.CreateEdgeCollectorInput) (controlplane.EdgeCollector, error)
	ListEdgeCollectors(context.Context, controlplane.ListEdgeCollectorsInput) ([]controlplane.EdgeCollector, error)
	GetEdgeCollectorBySetupTokenHash(context.Context, string) (controlplane.EdgeCollector, error)
	EnrollEdgeCollector(context.Context, controlplane.EnrollEdgeCollectorInput) (controlplane.EdgeCollector, error)
	AuthenticateEdgeCollector(context.Context, controlplane.AuthenticateEdgeCollectorInput) (controlplane.EdgeCollector, error)
	UpdateEdgeCollectorHeartbeat(context.Context, controlplane.UpdateEdgeCollectorHeartbeatInput) (controlplane.EdgeCollector, error)
	UpsertEdgeDeviceSource(context.Context, controlplane.UpsertEdgeDeviceSourceInput) (controlplane.EdgeDeviceSource, error)
	ListEdgeDeviceSources(context.Context, controlplane.ListEdgeDeviceSourcesInput) ([]controlplane.EdgeDeviceSource, error)
	ApproveEdgeDeviceSource(context.Context, controlplane.ApproveEdgeDeviceSourceInput) (controlplane.ApprovedEdgeDeviceSource, error)
	GetLinkedEdgeDeviceSource(context.Context, controlplane.GetLinkedEdgeDeviceSourceInput) (controlplane.EdgeDeviceSource, error)
}

type edgeCollectorEnvResolver interface {
	CollectorEnv(ctx context.Context, collector controlplane.EdgeCollector) (map[string]string, error)
}

type edgeCollectorEnvResolverFunc func(context.Context, controlplane.EdgeCollector) (map[string]string, error)

func (f edgeCollectorEnvResolverFunc) CollectorEnv(ctx context.Context, collector controlplane.EdgeCollector) (map[string]string, error) {
	return f(ctx, collector)
}

type edgeCollectorBLEAuthStore interface {
	GetActiveProviderCredentialByUserID(context.Context, string, string) (controlplane.ProviderCredential, error)
}

func newEdgeCollectorEnvResolver(store any) edgeCollectorEnvResolver {
	bleStore, ok := store.(edgeCollectorBLEAuthStore)
	if !ok {
		return nil
	}
	return edgeCollectorEnvResolverFunc(func(ctx context.Context, collector controlplane.EdgeCollector) (map[string]string, error) {
		credential, err := bleStore.GetActiveProviderCredentialByUserID(ctx, collector.UserID, controlplane.ProviderEcoFlowBLE)
		if err != nil {
			if errors.Is(err, controlplane.ErrCredentialNotFound) {
				return nil, nil
			}
			return nil, err
		}
		userID := strings.TrimSpace(credential.SecretKey)
		if userID == "" {
			return nil, nil
		}
		return map[string]string{
			"ECOFLOW_BLE_USER_ID": userID,
		}, nil
	})
}

type EdgeIngestService struct {
	edgev1.UnimplementedEdgeIngestServiceServer

	log         *slog.Logger
	store       edgeControlStore
	publisher   telemetrybus.EnvelopePublisher
	subjectCfg  telemetrybus.SubjectConfig
	envResolver edgeCollectorEnvResolver
}

type EdgeIngestServiceDeps struct {
	Log         *slog.Logger
	Store       edgeControlStore
	Publisher   telemetrybus.EnvelopePublisher
	SubjectCfg  telemetrybus.SubjectConfig
	EnvResolver edgeCollectorEnvResolver
}

type linkedEdgeSourceLookup struct {
	source controlplane.EdgeDeviceSource
	found  bool
}

type linkedEdgeSourceCacheKey struct {
	collectorID      string
	provider         string
	transport        string
	providerDeviceID string
}

func NewEdgeIngestService(deps EdgeIngestServiceDeps) *EdgeIngestService {
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	return &EdgeIngestService{
		log:         log,
		store:       deps.Store,
		publisher:   deps.Publisher,
		subjectCfg:  deps.SubjectCfg.Normalized(),
		envResolver: deps.EnvResolver,
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
	setupTokenHash := edgecollector.HashCollectorSecret(setupToken)
	env, err := s.collectorEnvForSetupToken(ctx, setupTokenHash)
	if err != nil {
		return nil, err
	}
	row, err := s.store.EnrollEdgeCollector(ctx, controlplane.EnrollEdgeCollectorInput{
		SetupTokenHash:      setupTokenHash,
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
		CollectorEnv:    env,
	}, nil
}

func (s *EdgeIngestService) collectorEnvForSetupToken(ctx context.Context, setupTokenHash string) (map[string]string, error) {
	if s.envResolver == nil {
		return nil, nil
	}
	collector, err := s.store.GetEdgeCollectorBySetupTokenHash(ctx, setupTokenHash)
	if err != nil {
		return nil, edgeStoreStatus("resolve edge collector setup", err)
	}
	return s.collectorEnv(ctx, collector)
}

func (s *EdgeIngestService) collectorEnv(ctx context.Context, collector controlplane.EdgeCollector) (map[string]string, error) {
	if s.envResolver == nil {
		return nil, nil
	}
	env, err := s.envResolver.CollectorEnv(ctx, collector)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve edge collector config: %v", err)
	}
	out := make(map[string]string, len(env))
	for key, value := range env {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
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
	if len(req.GetDiscoveries()) > maxEdgeDiscoveryBatchRecords {
		return nil, status.Errorf(codes.InvalidArgument, "discovery batch exceeds max %d", maxEdgeDiscoveryBatchRecords)
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
	if len(req.GetSamples()) > maxEdgeTelemetryBatchSamples {
		return nil, status.Errorf(codes.InvalidArgument, "telemetry batch exceeds max %d", maxEdgeTelemetryBatchSamples)
	}
	var accepted, dropped uint32
	var linkedSourceCache map[linkedEdgeSourceCacheKey]linkedEdgeSourceLookup
	var seenMessageIDs map[string]struct{}
	if len(req.GetSamples()) > 1 {
		linkedSourceCache = make(map[linkedEdgeSourceCacheKey]linkedEdgeSourceLookup, len(req.GetSamples()))
		seenMessageIDs = make(map[string]struct{}, len(req.GetSamples()))
	}
	for _, sample := range req.GetSamples() {
		if strings.TrimSpace(sample.GetProviderDeviceId()) == "" {
			dropped++
			continue
		}
		if metrics := sample.GetMetrics(); metrics == nil || len(metrics.GetFields()) == 0 {
			dropped++
			continue
		}
		source, found, err := s.linkedEdgeSourceForTelemetry(ctx, collector.ID, sample, linkedSourceCache)
		if err != nil {
			return nil, edgeStoreStatus("resolve linked edge source", err)
		}
		if !found {
			dropped++
			continue
		}
		params := edgecollector.NormalizeEcoFlowBLEMetricStruct(sample.GetMetrics())
		if len(params) == 0 {
			dropped++
			continue
		}
		observedAt := time.UnixMilli(sample.GetObservedAtUnixMs()).UTC()
		if sample.GetObservedAtUnixMs() <= 0 {
			observedAt = time.Now().UTC()
		}
		envelope, err := edgecollector.BuildTelemetryEnvelopeWithOwnedParams(edgecollector.TelemetrySample{
			CollectorID:      collector.ID,
			DeviceID:         source.LinkedDeviceID,
			Provider:         source.Provider,
			ProviderDeviceID: source.ProviderDeviceID,
			Transport:        source.Transport,
			ObservedAt:       observedAt,
			Params:           params,
		}, s.subjectCfg)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "build edge telemetry envelope: %v", err)
		}
		if seenMessageIDs != nil {
			if _, ok := seenMessageIDs[envelope.GetMessageId()]; ok {
				dropped++
				continue
			}
			seenMessageIDs[envelope.GetMessageId()] = struct{}{}
		}
		if err := telemetrybus.PublishEnvelope(ctx, s.publisher, envelope); err != nil {
			return nil, status.Errorf(codes.Unavailable, "publish edge telemetry: %v", err)
		}
		accepted++
	}
	return &edgev1.UploadTelemetryBatchResponse{AcceptedCount: accepted, DroppedCount: dropped}, nil
}

func (s *EdgeIngestService) linkedEdgeSourceForTelemetry(
	ctx context.Context,
	collectorID string,
	sample *edgev1.EdgeTelemetrySample,
	cache map[linkedEdgeSourceCacheKey]linkedEdgeSourceLookup,
) (controlplane.EdgeDeviceSource, bool, error) {
	if cache == nil {
		return s.lookupLinkedEdgeSource(ctx, controlplane.GetLinkedEdgeDeviceSourceInput{
			CollectorID:      collectorID,
			Provider:         sample.GetProvider(),
			Transport:        sample.GetTransport(),
			ProviderDeviceID: sample.GetProviderDeviceId(),
		})
	}
	key := linkedEdgeSourceCacheKeyForTelemetry(collectorID, sample)
	if cached, ok := cache[key]; ok {
		return cached.source, cached.found, nil
	}
	source, found, err := s.lookupLinkedEdgeSource(ctx, controlplane.GetLinkedEdgeDeviceSourceInput{
		CollectorID:      collectorID,
		Provider:         key.provider,
		Transport:        key.transport,
		ProviderDeviceID: key.providerDeviceID,
	})
	if err != nil {
		return controlplane.EdgeDeviceSource{}, false, err
	}
	cache[key] = linkedEdgeSourceLookup{source: source, found: found}
	return source, found, nil
}

func (s *EdgeIngestService) lookupLinkedEdgeSource(ctx context.Context, in controlplane.GetLinkedEdgeDeviceSourceInput) (controlplane.EdgeDeviceSource, bool, error) {
	source, err := s.store.GetLinkedEdgeDeviceSource(ctx, in)
	if errors.Is(err, controlplane.ErrEdgeDeviceSourceNotFound) {
		return controlplane.EdgeDeviceSource{}, false, nil
	}
	if err != nil {
		return controlplane.EdgeDeviceSource{}, false, err
	}
	return source, true, nil
}

func linkedEdgeSourceCacheKeyForTelemetry(collectorID string, sample *edgev1.EdgeTelemetrySample) linkedEdgeSourceCacheKey {
	transport := strings.ToLower(strings.TrimSpace(sample.GetTransport()))
	if transport == "" {
		transport = "ble"
	}
	return linkedEdgeSourceCacheKey{
		collectorID:      strings.TrimSpace(collectorID),
		provider:         controlplane.NormalizeProvider(sample.GetProvider()),
		transport:        transport,
		providerDeviceID: strings.ToUpper(strings.TrimSpace(sample.GetProviderDeviceId())),
	}
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
