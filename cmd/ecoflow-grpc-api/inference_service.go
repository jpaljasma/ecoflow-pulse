package main

import (
	"context"
	"log/slog"
	"strings"
	"time"

	inferencev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/inference/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/energydashboard"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/inference"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	defaultInferenceMaxItems = 8
	maxInferenceMaxItems     = 25
)

type InferenceService struct {
	inferencev1.UnimplementedInferenceServiceServer

	log               *slog.Logger
	reader            inference.Reader
	comparisonCache   inference.EnergyComparisonCache
	queryReader       telemetryquery.Reader
	controlPlaneStore controlplane.Store
	defaultMaxItems   int
	maxItems          int
	maxQueryBuckets   int
	nowFn             func() time.Time
}

type InferenceServiceDeps struct {
	Log               *slog.Logger
	Reader            inference.Reader
	ComparisonCache   inference.EnergyComparisonCache
	QueryReader       telemetryquery.Reader
	ControlPlaneStore controlplane.Store
	DefaultMaxItems   int
	MaxItems          int
	MaxQueryBuckets   int
	NowFn             func() time.Time
}

func NewInferenceService(log *slog.Logger, controlPlaneStore controlplane.Store, reader inference.Reader) *InferenceService {
	var comparisonCache inference.EnergyComparisonCache
	if cache, ok := reader.(inference.EnergyComparisonCache); ok {
		comparisonCache = cache
	}
	return NewInferenceServiceWithDeps(InferenceServiceDeps{
		Log:               log,
		Reader:            reader,
		ComparisonCache:   comparisonCache,
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
	maxQueryBuckets := deps.MaxQueryBuckets
	if maxQueryBuckets <= 0 {
		maxQueryBuckets = defaultMaxQueryBuckets
	}
	nowFn := deps.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	return &InferenceService{
		log:               log,
		reader:            deps.Reader,
		comparisonCache:   deps.ComparisonCache,
		queryReader:       deps.QueryReader,
		controlPlaneStore: deps.ControlPlaneStore,
		defaultMaxItems:   defaultMaxItems,
		maxItems:          maxItems,
		maxQueryBuckets:   maxQueryBuckets,
		nowFn:             nowFn,
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
		if isEmptyDeviceInsights(out) {
			out = pendingDeviceInsights(deviceID)
		} else if strings.TrimSpace(out.DeviceID) == "" {
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
		if isEmptyDeviceInsights(row) {
			row = pendingDeviceInsights(deviceID)
		} else if strings.TrimSpace(row.DeviceID) == "" {
			row.DeviceID = deviceID
		}
		devices = append(devices, deviceInsightsToProto(row))
	}

	return &inferencev1.ListFleetInsightsResponse{Devices: devices}, nil
}

func (s *InferenceService) GetEnergyComparisonInsight(ctx context.Context, req *inferencev1.GetEnergyComparisonInsightRequest) (*inferencev1.GetEnergyComparisonInsightResponse, error) {
	if s.queryReader == nil {
		return nil, status.Error(codes.Unavailable, "energy comparison inference unavailable")
	}
	scope, window, loc, preset, err := s.resolveEnergyScopeWindow(ctx, req.GetDeviceId(), req.GetUseAllDevices(), req.GetPreset(), req.GetTimezone())
	if err != nil {
		return nil, err
	}
	now := s.nowFn().UTC()
	cacheKey := inference.EnergyComparisonCacheKey{
		ScopeMode:         scope.Mode,
		DeviceID:          scope.DeviceID,
		ResolvedDeviceIDs: append([]string(nil), scope.ResolvedDeviceIDs...),
		Preset:            string(preset),
		Timezone:          loc.String(),
		GridPricePerKwh:   req.GetGridPricePerKwh(),
		Currency:          strings.TrimSpace(req.GetCurrency()),
		RefreshSlotUnixMs: now.Truncate(time.Hour).UnixMilli(),
	}
	if s.comparisonCache != nil {
		cached, cacheErr := s.comparisonCache.GetEnergyComparison(ctx, cacheKey)
		if cacheErr != nil {
			s.log.Warn("energy comparison cache read failed", "error", cacheErr.Error())
		} else if cached != nil && cached.Insight != nil && cached.Insight.ExpiresAt.After(now) {
			return &inferencev1.GetEnergyComparisonInsightResponse{
				Status:       insightStatusToProto(cached.Status),
				StatusDetail: cached.StatusDetail,
				Insight:      energyComparisonInsightToProto(*cached.Insight),
			}, nil
		}
	}

	energyResolution := energyResolutionForPreset(preset)
	powerResolution := powerResolutionForPreset(preset)
	currentEnergy, err := s.queryScopeSeries(ctx, scope, energyResolution, window.From, window.To)
	if err != nil {
		return nil, err
	}
	previousEnergy, err := s.queryScopeSeries(ctx, scope, energyResolution, window.PreviousFrom, window.PreviousTo)
	if err != nil {
		return nil, err
	}
	currentPower, err := s.queryScopeSeries(ctx, scope, powerResolution, window.From, window.To)
	if err != nil {
		return nil, err
	}
	previousPower, err := s.queryScopeSeries(ctx, scope, powerResolution, window.PreviousFrom, window.PreviousTo)
	if err != nil {
		return nil, err
	}

	record := inference.BuildEnergyComparisonInsight(inference.EnergyComparisonInput{
		Now:             now,
		Scope:           inference.EnergyComparisonScope{Mode: scope.Mode, DeviceID: scope.DeviceID, ResolvedDeviceIDs: scope.ResolvedDeviceIDs},
		Preset:          string(preset),
		Timezone:        loc.String(),
		GridPricePerKwh: req.GetGridPricePerKwh(),
		Currency:        strings.TrimSpace(req.GetCurrency()),
		CurrentEnergy:   currentEnergy,
		PreviousEnergy:  previousEnergy,
		CurrentPower:    currentPower,
		PreviousPower:   previousPower,
	})
	if s.comparisonCache != nil {
		if cacheErr := s.comparisonCache.PutEnergyComparison(ctx, cacheKey, record); cacheErr != nil {
			s.log.Warn("energy comparison cache write failed", "error", cacheErr.Error())
		}
	}
	return &inferencev1.GetEnergyComparisonInsightResponse{
		Status:       insightStatusToProto(record.Status),
		StatusDetail: record.StatusDetail,
		Insight:      energyComparisonInsightToProto(*record.Insight),
	}, nil
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

func (s *InferenceService) resolveEnergyScopeWindow(ctx context.Context, deviceID string, useAllDevices bool, presetRaw, timezone string) (energydashboard.Scope, energydashboard.Window, *time.Location, energydashboard.Preset, error) {
	preset, err := energydashboard.ParsePreset(presetRaw)
	if err != nil {
		return energydashboard.Scope{}, energydashboard.Window{}, nil, "", status.Error(codes.InvalidArgument, err.Error())
	}
	loc := time.UTC
	if tz := strings.TrimSpace(timezone); tz != "" {
		loc, err = time.LoadLocation(tz)
		if err != nil {
			return energydashboard.Scope{}, energydashboard.Window{}, nil, "", status.Errorf(codes.InvalidArgument, "invalid timezone: %v", err)
		}
	}
	var visibleDeviceIDs []string
	if useAllDevices {
		visibleDeviceIDs, err = s.resolveFleetDeviceIDs(ctx, nil)
		if err != nil {
			return energydashboard.Scope{}, energydashboard.Window{}, nil, "", err
		}
	} else {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			return energydashboard.Scope{}, energydashboard.Window{}, nil, "", status.Error(codes.InvalidArgument, "device_id required when use_all_devices is false")
		}
		if err := s.authorizeDeviceAccess(ctx, deviceID); err != nil {
			return energydashboard.Scope{}, energydashboard.Window{}, nil, "", err
		}
		visibleDeviceIDs = []string{deviceID}
	}
	scope, err := energydashboard.ResolveScope(scopeRequestValue(deviceID, useAllDevices), visibleDeviceIDs)
	if err != nil {
		return energydashboard.Scope{}, energydashboard.Window{}, nil, "", status.Error(codes.PermissionDenied, err.Error())
	}
	window, err := energydashboard.ResolveWindow(s.nowFn(), loc, preset)
	if err != nil {
		return energydashboard.Scope{}, energydashboard.Window{}, nil, "", status.Error(codes.InvalidArgument, err.Error())
	}
	return scope, window, loc, preset, nil
}

func (s *InferenceService) queryScopeSeries(ctx context.Context, scope energydashboard.Scope, resolution telemetryquery.Resolution, from, to time.Time) (telemetryquery.Series, error) {
	if len(scope.ResolvedDeviceIDs) == 1 {
		series, err := s.queryReader.QueryRange(ctx, telemetryquery.RangeQuery{
			DeviceID:   scope.ResolvedDeviceIDs[0],
			Resolution: resolution,
			From:       from.UTC(),
			To:         to.UTC(),
			Limit:      s.maxQueryBuckets,
		})
		if err != nil {
			return telemetryquery.Series{}, status.Errorf(codes.Internal, "query energy comparison rollups: %v", err)
		}
		return series, nil
	}
	if aggregateReader, ok := s.queryReader.(telemetryquery.AggregateReader); ok {
		series, err := aggregateReader.QueryRangeMany(ctx, telemetryquery.AggregateRangeQuery{
			DeviceIDs:   scope.ResolvedDeviceIDs,
			Resolution:  resolution,
			From:        from.UTC(),
			To:          to.UTC(),
			Limit:       s.maxQueryBuckets,
			AggregateID: "all",
		})
		if err != nil {
			return telemetryquery.Series{}, status.Errorf(codes.Internal, "query energy comparison rollups: %v", err)
		}
		return series, nil
	}
	return telemetryquery.Series{}, status.Error(codes.Unimplemented, "fleet energy comparison aggregation unavailable")
}

func energyComparisonInsightToProto(in inference.EnergyComparisonInsight) *inferencev1.EnergyComparisonInsight {
	out := &inferencev1.EnergyComparisonInsight{
		Id:                in.ID,
		Scope:             &inferencev1.EnergyComparisonScope{Mode: in.Scope.Mode, DeviceId: in.Scope.DeviceID, ResolvedDeviceIds: append([]string(nil), in.Scope.ResolvedDeviceIDs...)},
		Preset:            in.Preset,
		Timezone:          in.Timezone,
		VerdictClass:      string(in.VerdictClass),
		Headline:          in.Headline,
		Summary:           in.Summary,
		Score:             in.Score,
		Confidence:        in.Confidence,
		ModelKey:          in.ModelKey,
		ModelVersion:      in.ModelVersion,
		GeneratedAtUnixMs: in.GeneratedAt.UnixMilli(),
		ExpiresAtUnixMs:   in.ExpiresAt.UnixMilli(),
		Tags:              append([]string(nil), in.Tags...),
		Evidence:          evidenceToProto(in.Evidence),
		Attributes:        structToProto(in.Attributes),
	}
	out.Cards = make([]*inferencev1.EnergyComparisonCard, 0, len(in.Cards))
	for _, card := range in.Cards {
		out.Cards = append(out.Cards, &inferencev1.EnergyComparisonCard{
			Category:       energyComparisonCardCategoryToProto(card.Category),
			Title:          card.Title,
			Summary:        card.Summary,
			Recommendation: card.Recommendation,
			Score:          card.Score,
			Confidence:     card.Confidence,
			Evidence:       evidenceToProto(card.Evidence),
			Attributes:     structToProto(card.Attributes),
		})
	}
	return out
}

func isEmptyDeviceInsights(in inference.DeviceInsights) bool {
	return strings.TrimSpace(in.DeviceID) == "" &&
		strings.TrimSpace(string(in.Status)) == "" &&
		strings.TrimSpace(in.StatusDetail) == "" &&
		in.RefreshedAt.IsZero() &&
		len(in.Insights) == 0
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

func evidenceToProto(in []inference.Evidence) []*inferencev1.InsightEvidence {
	out := make([]*inferencev1.InsightEvidence, 0, len(in))
	for _, evidence := range in {
		out = append(out, &inferencev1.InsightEvidence{
			Source:  insightEvidenceSourceToProto(evidence.Source),
			Summary: evidence.Summary,
			Metrics: mapToStructProto(evidence.Metrics),
		})
	}
	return out
}

func structToProto(value map[string]any) *structpb.Struct {
	return mapToStructProto(value)
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
	case inferencev1.InsightKind_INSIGHT_KIND_ENERGY_COMPARISON:
		return inference.KindEnergyComparison, nil
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
	case inference.KindEnergyComparison:
		return inferencev1.InsightKind_INSIGHT_KIND_ENERGY_COMPARISON
	default:
		return inferencev1.InsightKind_INSIGHT_KIND_UNSPECIFIED
	}
}

func energyComparisonCardCategoryToProto(category inference.EnergyComparisonCardCategory) inferencev1.EnergyComparisonCardCategory {
	switch category {
	case inference.EnergyComparisonCardSelfSufficiency:
		return inferencev1.EnergyComparisonCardCategory_ENERGY_COMPARISON_CARD_CATEGORY_SELF_SUFFICIENCY
	case inference.EnergyComparisonCardSolar:
		return inferencev1.EnergyComparisonCardCategory_ENERGY_COMPARISON_CARD_CATEGORY_SOLAR
	case inference.EnergyComparisonCardLoad:
		return inferencev1.EnergyComparisonCardCategory_ENERGY_COMPARISON_CARD_CATEGORY_LOAD
	case inference.EnergyComparisonCardBattery:
		return inferencev1.EnergyComparisonCardCategory_ENERGY_COMPARISON_CARD_CATEGORY_BATTERY
	case inference.EnergyComparisonCardGrid:
		return inferencev1.EnergyComparisonCardCategory_ENERGY_COMPARISON_CARD_CATEGORY_GRID
	case inference.EnergyComparisonCardValue:
		return inferencev1.EnergyComparisonCardCategory_ENERGY_COMPARISON_CARD_CATEGORY_VALUE
	default:
		return inferencev1.EnergyComparisonCardCategory_ENERGY_COMPARISON_CARD_CATEGORY_UNSPECIFIED
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
