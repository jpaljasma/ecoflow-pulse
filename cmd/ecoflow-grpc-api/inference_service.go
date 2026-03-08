package main

import (
	"context"
	"log/slog"
	"strings"

	inferencev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/inference/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/inference"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultInferenceMaxItems = 8
	maxInferenceMaxItems     = 25
)

type InferenceService struct {
	inferencev1.UnimplementedInferenceServiceServer

	log               *slog.Logger
	reader            inference.Reader
	controlPlaneStore controlplane.Store
	defaultMaxItems   int
	maxItems          int
}

type InferenceServiceDeps struct {
	Log               *slog.Logger
	Reader            inference.Reader
	ControlPlaneStore controlplane.Store
	DefaultMaxItems   int
	MaxItems          int
}

func NewInferenceService(log *slog.Logger, controlPlaneStore controlplane.Store, reader inference.Reader) *InferenceService {
	return NewInferenceServiceWithDeps(InferenceServiceDeps{
		Log:               log,
		Reader:            reader,
		ControlPlaneStore: controlPlaneStore,
	})
}

func NewInferenceServiceWithDeps(deps InferenceServiceDeps) *InferenceService {
	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	defaultMaxItems := deps.DefaultMaxItems
	if defaultMaxItems <= 0 {
		defaultMaxItems = defaultInferenceMaxItems
	}
	maxItems := deps.MaxItems
	if maxItems <= 0 {
		maxItems = maxInferenceMaxItems
	}
	if defaultMaxItems > maxItems {
		defaultMaxItems = maxItems
	}
	return &InferenceService{
		log:               log,
		reader:            deps.Reader,
		controlPlaneStore: deps.ControlPlaneStore,
		defaultMaxItems:   defaultMaxItems,
		maxItems:          maxItems,
	}
}

func (s *InferenceService) GetDeviceInsights(ctx context.Context, req *inferencev1.GetDeviceInsightsRequest) (*inferencev1.GetDeviceInsightsResponse, error) {
	deviceID := strings.TrimSpace(req.GetDeviceId())
	if deviceID == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id required")
	}
	if err := s.authorizeDeviceAccess(ctx, deviceID); err != nil {
		return nil, err
	}
	filter, err := s.filterFromProto(req.GetFilter())
	if err != nil {
		return nil, err
	}

	var out inference.DeviceInsights
	if s.reader == nil {
		out = pendingDeviceInsights(deviceID)
	} else {
		out, err = s.reader.GetDeviceInsights(ctx, deviceID, filter)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get device insights: %v", err)
		}
		if strings.TrimSpace(out.DeviceID) == "" {
			out.DeviceID = deviceID
		}
	}

	return &inferencev1.GetDeviceInsightsResponse{
		Insights: deviceInsightsToProto(out),
	}, nil
}

func (s *InferenceService) ListFleetInsights(ctx context.Context, req *inferencev1.ListFleetInsightsRequest) (*inferencev1.ListFleetInsightsResponse, error) {
	filter, err := s.filterFromProto(req.GetFilter())
	if err != nil {
		return nil, err
	}
	deviceIDs, err := s.resolveFleetDeviceIDs(ctx, req.GetDeviceIds())
	if err != nil {
		return nil, err
	}
	if len(deviceIDs) == 0 {
		return &inferencev1.ListFleetInsightsResponse{Devices: []*inferencev1.DeviceInsights{}}, nil
	}

	var rows []inference.DeviceInsights
	if s.reader == nil {
		rows = make([]inference.DeviceInsights, 0, len(deviceIDs))
		for _, deviceID := range deviceIDs {
			rows = append(rows, pendingDeviceInsights(deviceID))
		}
	} else {
		rows, err = s.reader.ListFleetInsights(ctx, deviceIDs, filter)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list fleet insights: %v", err)
		}
	}

	devices := make([]*inferencev1.DeviceInsights, 0, len(deviceIDs))
	byID := make(map[string]inference.DeviceInsights, len(rows))
	for _, row := range rows {
		if id := strings.TrimSpace(row.DeviceID); id != "" {
			byID[id] = row
		}
	}
	for _, deviceID := range deviceIDs {
		row, ok := byID[deviceID]
		if !ok {
			row = pendingDeviceInsights(deviceID)
		}
		if strings.TrimSpace(row.DeviceID) == "" {
			row.DeviceID = deviceID
		}
		devices = append(devices, deviceInsightsToProto(row))
	}

	return &inferencev1.ListFleetInsightsResponse{Devices: devices}, nil
}

func (s *InferenceService) resolveFleetDeviceIDs(ctx context.Context, requested []string) ([]string, error) {
	seen := make(map[string]struct{}, len(requested))
	out := make([]string, 0, len(requested))
	for _, raw := range requested {
		deviceID := strings.TrimSpace(raw)
		if deviceID == "" {
			continue
		}
		if _, ok := seen[deviceID]; ok {
			continue
		}
		if err := s.authorizeDeviceAccess(ctx, deviceID); err != nil {
			return nil, err
		}
		seen[deviceID] = struct{}{}
		out = append(out, deviceID)
	}
	if len(out) > 0 {
		return out, nil
	}

	if s.controlPlaneStore == nil {
		return nil, status.Error(codes.InvalidArgument, "device_ids required when user context missing")
	}
	claims, ok := grpcmw.ClaimsFromContext(ctx)
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return nil, status.Error(codes.InvalidArgument, "device_ids required when user context missing")
	}
	rows, err := s.controlPlaneStore.ListUserDevices(ctx, controlplane.ListUserDevicesInput{
		UserSubject: claims.Subject,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list fleet devices: %v", err)
	}
	for _, row := range rows {
		if row.DeviceID == "" {
			continue
		}
		if _, ok := seen[row.DeviceID]; ok {
			continue
		}
		seen[row.DeviceID] = struct{}{}
		out = append(out, row.DeviceID)
	}
	return out, nil
}

func (s *InferenceService) authorizeDeviceAccess(ctx context.Context, deviceID string) error {
	if s.controlPlaneStore == nil {
		return nil
	}
	claims, ok := grpcmw.ClaimsFromContext(ctx)
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return nil
	}
	rows, err := s.controlPlaneStore.ListUserDevices(ctx, controlplane.ListUserDevicesInput{UserSubject: claims.Subject})
	if err != nil {
		return status.Errorf(codes.Internal, "authorize inference device access: %v", err)
	}
	for i := range rows {
		if rows[i].DeviceID == deviceID {
			return nil
		}
	}
	return status.Error(codes.PermissionDenied, "device access denied")
}

func (s *InferenceService) filterFromProto(in *inferencev1.InsightFilter) (inference.Filter, error) {
	filter := inference.Filter{
		MaxItems: s.defaultMaxItems,
	}
	if in == nil {
		return filter, nil
	}
	if maxItems := int(in.GetMaxItems()); maxItems > 0 {
		filter.MaxItems = maxItems
		if filter.MaxItems > s.maxItems {
			filter.MaxItems = s.maxItems
		}
	}
	if len(in.GetKinds()) == 0 {
		return filter, nil
	}
	seen := make(map[inference.Kind]struct{}, len(in.GetKinds()))
	filter.Kinds = make([]inference.Kind, 0, len(in.GetKinds()))
	for _, kindValue := range in.GetKinds() {
		kind, err := insightKindFromProto(kindValue)
		if err != nil {
			return inference.Filter{}, err
		}
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		filter.Kinds = append(filter.Kinds, kind)
	}
	return filter, nil
}

func pendingDeviceInsights(deviceID string) inference.DeviceInsights {
	return inference.DeviceInsights{
		DeviceID:     deviceID,
		Status:       inference.StatusPending,
		StatusDetail: "insight projection not initialized",
		Insights:     []inference.Insight{},
	}
}

func deviceInsightsToProto(in inference.DeviceInsights) *inferencev1.DeviceInsights {
	out := &inferencev1.DeviceInsights{
		DeviceId:     in.DeviceID,
		Status:       insightStatusToProto(in.Status),
		StatusDetail: in.StatusDetail,
		Insights:     make([]*inferencev1.DeviceInsight, 0, len(in.Insights)),
	}
	if !in.RefreshedAt.IsZero() {
		out.RefreshedAtUnixMs = in.RefreshedAt.UnixMilli()
	}
	for _, insight := range in.Insights {
		out.Insights = append(out.Insights, deviceInsightToProto(insight))
	}
	return out
}

func deviceInsightToProto(in inference.Insight) *inferencev1.DeviceInsight {
	out := &inferencev1.DeviceInsight{
		Id:           in.ID,
		DeviceId:     in.DeviceID,
		Kind:         insightKindToProto(in.Kind),
		Title:        in.Title,
		Summary:      in.Summary,
		Score:        in.Score,
		Rank:         in.Rank,
		ModelKey:     in.ModelKey,
		ModelVersion: in.ModelVersion,
		Tags:         append([]string(nil), in.Tags...),
		Attributes:   mapToStructProto(in.Attributes),
		Evidence:     make([]*inferencev1.InsightEvidence, 0, len(in.Evidence)),
		Actions:      make([]*inferencev1.InsightAction, 0, len(in.Actions)),
	}
	if !in.GeneratedAt.IsZero() {
		out.GeneratedAtUnixMs = in.GeneratedAt.UnixMilli()
	}
	if !in.ExpiresAt.IsZero() {
		out.ExpiresAtUnixMs = in.ExpiresAt.UnixMilli()
	}
	for _, evidence := range in.Evidence {
		out.Evidence = append(out.Evidence, &inferencev1.InsightEvidence{
			Source:  insightEvidenceSourceToProto(evidence.Source),
			Summary: evidence.Summary,
			Metrics: mapToStructProto(evidence.Metrics),
		})
	}
	for _, action := range in.Actions {
		out.Actions = append(out.Actions, &inferencev1.InsightAction{
			Kind:   insightActionKindToProto(action.Kind),
			Label:  action.Label,
			Target: action.Target,
			Params: mapToStructProto(action.Params),
		})
	}
	return out
}

func insightKindFromProto(kind inferencev1.InsightKind) (inference.Kind, error) {
	switch kind {
	case inferencev1.InsightKind_INSIGHT_KIND_BATTERY_EXPANSION:
		return inference.KindBatteryExpansion, nil
	case inferencev1.InsightKind_INSIGHT_KIND_SOLAR_ADD_ON:
		return inference.KindSolarAddOn, nil
	case inferencev1.InsightKind_INSIGHT_KIND_SOLAR_UPGRADE:
		return inference.KindSolarUpgrade, nil
	case inferencev1.InsightKind_INSIGHT_KIND_ENERGY_SHIFT:
		return inference.KindEnergyShift, nil
	case inferencev1.InsightKind_INSIGHT_KIND_MAINTENANCE:
		return inference.KindMaintenance, nil
	default:
		return "", status.Error(codes.InvalidArgument, "unsupported insight kind")
	}
}

func insightKindToProto(kind inference.Kind) inferencev1.InsightKind {
	switch kind {
	case inference.KindBatteryExpansion:
		return inferencev1.InsightKind_INSIGHT_KIND_BATTERY_EXPANSION
	case inference.KindSolarAddOn:
		return inferencev1.InsightKind_INSIGHT_KIND_SOLAR_ADD_ON
	case inference.KindSolarUpgrade:
		return inferencev1.InsightKind_INSIGHT_KIND_SOLAR_UPGRADE
	case inference.KindEnergyShift:
		return inferencev1.InsightKind_INSIGHT_KIND_ENERGY_SHIFT
	case inference.KindMaintenance:
		return inferencev1.InsightKind_INSIGHT_KIND_MAINTENANCE
	default:
		return inferencev1.InsightKind_INSIGHT_KIND_UNSPECIFIED
	}
}

func insightStatusToProto(statusValue inference.Status) inferencev1.InsightStatus {
	switch statusValue {
	case inference.StatusPending:
		return inferencev1.InsightStatus_INSIGHT_STATUS_PENDING
	case inference.StatusReady:
		return inferencev1.InsightStatus_INSIGHT_STATUS_READY
	case inference.StatusStale:
		return inferencev1.InsightStatus_INSIGHT_STATUS_STALE
	case inference.StatusUnavailable:
		return inferencev1.InsightStatus_INSIGHT_STATUS_UNAVAILABLE
	default:
		return inferencev1.InsightStatus_INSIGHT_STATUS_UNSPECIFIED
	}
}

func insightActionKindToProto(kind inference.ActionKind) inferencev1.InsightActionKind {
	switch kind {
	case inference.ActionKindInternalRoute:
		return inferencev1.InsightActionKind_INSIGHT_ACTION_KIND_INTERNAL_ROUTE
	case inference.ActionKindExternalURL:
		return inferencev1.InsightActionKind_INSIGHT_ACTION_KIND_EXTERNAL_URL
	case inference.ActionKindLearnMore:
		return inferencev1.InsightActionKind_INSIGHT_ACTION_KIND_LEARN_MORE
	case inference.ActionKindDismiss:
		return inferencev1.InsightActionKind_INSIGHT_ACTION_KIND_DISMISS
	default:
		return inferencev1.InsightActionKind_INSIGHT_ACTION_KIND_UNSPECIFIED
	}
}

func insightEvidenceSourceToProto(source inference.EvidenceSource) inferencev1.InsightEvidenceSource {
	switch source {
	case inference.EvidenceSourceLiveSnapshot:
		return inferencev1.InsightEvidenceSource_INSIGHT_EVIDENCE_SOURCE_LIVE_SNAPSHOT
	case inference.EvidenceSourceRollupHistory:
		return inferencev1.InsightEvidenceSource_INSIGHT_EVIDENCE_SOURCE_ROLLUP_HISTORY
	case inference.EvidenceSourceDeviceCapabilities:
		return inferencev1.InsightEvidenceSource_INSIGHT_EVIDENCE_SOURCE_DEVICE_CAPABILITIES
	case inference.EvidenceSourceProviderMetadata:
		return inferencev1.InsightEvidenceSource_INSIGHT_EVIDENCE_SOURCE_PROVIDER_METADATA
	case inference.EvidenceSourceModelOutput:
		return inferencev1.InsightEvidenceSource_INSIGHT_EVIDENCE_SOURCE_MODEL_OUTPUT
	case inference.EvidenceSourceRuleEngine:
		return inferencev1.InsightEvidenceSource_INSIGHT_EVIDENCE_SOURCE_RULE_ENGINE
	case inference.EvidenceSourceUserContext:
		return inferencev1.InsightEvidenceSource_INSIGHT_EVIDENCE_SOURCE_USER_CONTEXT
	default:
		return inferencev1.InsightEvidenceSource_INSIGHT_EVIDENCE_SOURCE_UNSPECIFIED
	}
}
