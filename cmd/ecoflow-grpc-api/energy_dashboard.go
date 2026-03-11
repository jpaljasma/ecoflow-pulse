package main

import (
	"context"
	"sort"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	telemetryv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/telemetry/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/energydashboard"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func (s *TelemetryService) GetEnergyDashboard(ctx context.Context, req *telemetryv1.GetEnergyDashboardRequest) (*telemetryv1.GetEnergyDashboardResponse, error) {
	if s.queryReader == nil {
		return nil, status.Error(codes.Unavailable, "telemetry history unavailable")
	}

	preset, err := energydashboard.ParsePreset(req.GetPreset())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	loc := time.UTC
	if tz := strings.TrimSpace(req.GetTimezone()); tz != "" {
		loc, err = time.LoadLocation(tz)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid timezone: %v", err)
		}
	}

	visibleDeviceIDs, err := s.resolveVisibleDeviceIDs(ctx, req.GetDeviceId(), req.GetUseAllDevices())
	if err != nil {
		return nil, err
	}
	scope, err := energydashboard.ResolveScope(scopeRequestValue(req.GetDeviceId(), req.GetUseAllDevices()), visibleDeviceIDs)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	window, err := energydashboard.ResolveWindow(time.Now(), loc, preset)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	energyResolution := energyResolutionForPreset(preset)
	currentSeries, err := s.queryScopeSeries(ctx, scope.ResolvedDeviceIDs, energyResolution, window.From, window.To)
	if err != nil {
		return nil, err
	}
	previousSeries := telemetryquery.Series{
		DeviceID:   currentSeries.DeviceID,
		Resolution: currentSeries.Resolution,
		From:       window.PreviousFrom.UTC(),
		To:         window.PreviousTo.UTC(),
		Points:     []telemetryquery.Point{},
	}
	if req.GetIncludeComparison() {
		previousSeries, err = s.queryScopeSeries(ctx, scope.ResolvedDeviceIDs, energyResolution, window.PreviousFrom, window.PreviousTo)
		if err != nil {
			return nil, err
		}
	}

	powerResolution := powerResolutionForPreset(preset)
	currentPowerSeries, err := s.queryScopeSeries(ctx, scope.ResolvedDeviceIDs, powerResolution, window.From, window.To)
	if err != nil {
		return nil, err
	}
	previousPowerSeries := telemetryquery.Series{
		DeviceID:   currentPowerSeries.DeviceID,
		Resolution: currentPowerSeries.Resolution,
		From:       window.PreviousFrom.UTC(),
		To:         window.PreviousTo.UTC(),
		Points:     []telemetryquery.Point{},
	}
	if req.GetIncludeComparison() {
		previousPowerSeries, err = s.queryScopeSeries(ctx, scope.ResolvedDeviceIDs, powerResolution, window.PreviousFrom, window.PreviousTo)
		if err != nil {
			return nil, err
		}
	}
	pvPortHistory, err := s.queryScopePVPortHistory(ctx, scope.ResolvedDeviceIDs, window.From, window.To)
	if err != nil {
		return nil, err
	}

	resp := &telemetryv1.GetEnergyDashboardResponse{
		Scope: &telemetryv1.EnergyScope{
			Mode:              scope.Mode,
			DeviceId:          scope.DeviceID,
			ResolvedDeviceIds: append([]string(nil), scope.ResolvedDeviceIDs...),
		},
		Window: &telemetryv1.EnergyWindow{
			Preset:             string(preset),
			Timezone:           loc.String(),
			FromUnixMs:         window.From.UnixMilli(),
			ToUnixMs:           window.To.UnixMilli(),
			PreviousFromUnixMs: window.PreviousFrom.UnixMilli(),
			PreviousToUnixMs:   window.PreviousTo.UnixMilli(),
		},
		Summary:              summaryToProto(energydashboard.BuildSummary(currentSeries, previousSeries, req.GetGridPricePerKwh()), req.GetCurrency()),
		Battery:              batteryToProto(energydashboard.BuildBatterySummary(currentSeries)),
		CurrentEnergyPoints:  pointsToProto(currentSeries.Points),
		PreviousEnergyPoints: pointsToProto(previousSeries.Points),
		CurrentPowerPoints:   pointsToProto(currentPowerSeries.Points),
		PreviousPowerPoints:  pointsToProto(previousPowerSeries.Points),
		PvPortHistory:        pvPortHistoryToProto(pvPortHistory),
	}
	s.maybeEnableHistoryCompression(ctx, resp)
	return resp, nil
}

func (s *TelemetryService) queryScopePVPortHistory(ctx context.Context, deviceIDs []string, from, to time.Time) ([]energydashboard.PVPortHistory, error) {
	if s.archiveManifestStore == nil || s.archiveObjectReader == nil || s.controlPlaneStore == nil || len(deviceIDs) == 0 {
		return nil, nil
	}

	providerDeviceIDs := make([]string, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		row, err := s.controlPlaneStore.GetProviderDeviceByDeviceID(ctx, deviceID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "resolve provider device for energy pv history: %v", err)
		}
		if strings.EqualFold(strings.TrimSpace(row.Provider), controlplane.ProviderEcoFlow) && strings.TrimSpace(row.ProviderDeviceID) != "" {
			providerDeviceIDs = append(providerDeviceIDs, row.ProviderDeviceID)
		}
	}
	if len(providerDeviceIDs) == 0 {
		return nil, nil
	}

	objects, err := s.archiveManifestStore.ListByDevices(ctx, replaycli.DeviceQuery{
		Provider:           controlplane.ProviderEcoFlow,
		FromUnixMS:         from.UnixMilli(),
		ToUnixMS:           to.UnixMilli(),
		ProviderDeviceIDs:  providerDeviceIDs,
		MaxObjectsReturned: 256,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query archive manifest for energy pv history: %v", err)
	}

	providerFilter := make(map[string]struct{}, len(providerDeviceIDs))
	for _, id := range providerDeviceIDs {
		providerFilter[strings.ToUpper(strings.TrimSpace(id))] = struct{}{}
	}

	envelopes := make([]*envelopev1.TelemetryEnvelope, 0, 128)
	for _, object := range objects {
		body, err := s.archiveObjectReader.ReadObject(ctx, object.ObjectBucket, object.ObjectKey)
		if err != nil {
			if isMissingArchiveObjectError(err) {
				s.log.Warn(
					"skip missing archive object for energy pv history",
					"bucket", object.ObjectBucket,
					"key", object.ObjectKey,
					"error", err.Error(),
				)
				continue
			}
			return nil, status.Errorf(codes.Internal, "read archive object for energy pv history: %v", err)
		}
		frames, err := replaycli.DecodeEnvelopeFrames(body)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "decode archive object for energy pv history: %v", err)
		}
		for _, frame := range frames {
			var env envelopev1.TelemetryEnvelope
			if err := proto.Unmarshal(frame, &env); err != nil {
				return nil, status.Errorf(codes.Internal, "unmarshal archive envelope for energy pv history: %v", err)
			}
			if env.GetPayloadType() != "ecoflow.quota.normalized" {
				continue
			}
			providerDeviceID := strings.ToUpper(strings.TrimSpace(env.GetEcoflowSn()))
			if labels := env.GetLabels(); len(labels) > 0 {
				if candidate := strings.ToUpper(strings.TrimSpace(labels["provider_device_id"])); candidate != "" {
					providerDeviceID = candidate
				}
			}
			if _, ok := providerFilter[providerDeviceID]; !ok {
				continue
			}
			ts := envelopeTimestamp(&env)
			if ts.Before(from) || !ts.Before(to) {
				continue
			}
			envelopes = append(envelopes, proto.Clone(&env).(*envelopev1.TelemetryEnvelope))
		}
	}

	rows := energydashboard.SummarizePVPortHistory(envelopes)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].DeviceID == rows[j].DeviceID {
			return rows[i].PortID < rows[j].PortID
		}
		return rows[i].DeviceID < rows[j].DeviceID
	})
	return rows, nil
}

func isMissingArchiveObjectError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such key") || strings.Contains(msg, "specified key does not exist")
}

func envelopeTimestamp(env *envelopev1.TelemetryEnvelope) time.Time {
	if env == nil {
		return time.Time{}
	}
	switch {
	case env.GetObservedTimeUnixMs() > 0:
		return time.UnixMilli(env.GetObservedTimeUnixMs()).UTC()
	case env.GetIngestedTimeUnixMs() > 0:
		return time.UnixMilli(env.GetIngestedTimeUnixMs()).UTC()
	case env.GetDeviceTimeUnixMs() > 0:
		return time.UnixMilli(env.GetDeviceTimeUnixMs()).UTC()
	default:
		return time.Time{}
	}
}

func (s *TelemetryService) resolveVisibleDeviceIDs(ctx context.Context, requestedDeviceID string, useAllDevices bool) ([]string, error) {
	requestedDeviceID = strings.TrimSpace(requestedDeviceID)
	if !useAllDevices {
		if requestedDeviceID == "" {
			return nil, status.Error(codes.InvalidArgument, "device_id required when use_all_devices is false")
		}
		if err := s.authorizeDeviceAccess(ctx, requestedDeviceID); err != nil {
			return nil, err
		}
		return []string{requestedDeviceID}, nil
	}
	if s.controlPlaneStore == nil {
		return nil, status.Error(codes.InvalidArgument, "all-device energy scope unavailable without user device store")
	}
	claims, ok := grpcmw.ClaimsFromContext(ctx)
	if !ok || strings.TrimSpace(claims.Subject) == "" {
		return nil, status.Error(codes.InvalidArgument, "all-device energy scope requires user context")
	}
	rows, err := s.controlPlaneStore.ListUserDevices(ctx, controlplane.ListUserDevicesInput{UserSubject: claims.Subject})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list visible energy devices: %v", err)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.DeviceID) != "" {
			out = append(out, row.DeviceID)
		}
	}
	return out, nil
}

func scopeRequestValue(deviceID string, useAllDevices bool) string {
	if useAllDevices {
		return "all"
	}
	return strings.TrimSpace(deviceID)
}

func energyResolutionForPreset(preset energydashboard.Preset) telemetryquery.Resolution {
	switch preset {
	case energydashboard.PresetToday, energydashboard.PresetYesterday:
		return telemetryquery.ResolutionHour
	default:
		return telemetryquery.ResolutionDay
	}
}

func powerResolutionForPreset(preset energydashboard.Preset) telemetryquery.Resolution {
	switch preset {
	case energydashboard.PresetToday, energydashboard.PresetYesterday:
		return telemetryquery.ResolutionMinute
	case energydashboard.PresetLast7Days, energydashboard.PresetThisWeek, energydashboard.PresetPreviousWeek, energydashboard.PresetThisMonth:
		return telemetryquery.ResolutionHour
	default:
		return telemetryquery.ResolutionDay
	}
}

func (s *TelemetryService) queryScopeSeries(ctx context.Context, deviceIDs []string, resolution telemetryquery.Resolution, from, to time.Time) (telemetryquery.Series, error) {
	aggregated := telemetryquery.Series{
		DeviceID:   "all",
		Resolution: resolution,
		From:       from.UTC(),
		To:         to.UTC(),
		Points:     []telemetryquery.Point{},
	}
	for _, deviceID := range deviceIDs {
		series, err := s.queryReader.QueryRange(ctx, telemetryquery.RangeQuery{
			DeviceID:   deviceID,
			Resolution: resolution,
			From:       from.UTC(),
			To:         to.UTC(),
			Limit:      s.maxQueryBuckets,
		})
		if err != nil {
			return telemetryquery.Series{}, s.mapQueryError(err)
		}
		aggregated = mergeSeries(aggregated, series)
	}
	if len(deviceIDs) == 1 {
		aggregated.DeviceID = deviceIDs[0]
	}
	return aggregated, nil
}

func mergeSeries(base, incoming telemetryquery.Series) telemetryquery.Series {
	if len(base.Points) == 0 {
		base.Points = append([]telemetryquery.Point(nil), incoming.Points...)
		return base
	}
	index := make(map[int64]int, len(base.Points))
	for idx, point := range base.Points {
		index[point.BucketStart.UnixMilli()] = idx
	}
	for _, point := range incoming.Points {
		key := point.BucketStart.UnixMilli()
		if idx, ok := index[key]; ok {
			base.Points[idx] = mergePoint(base.Points[idx], point)
			continue
		}
		base.Points = append(base.Points, point)
		index[key] = len(base.Points) - 1
	}
	return base
}

func mergePoint(left, right telemetryquery.Point) telemetryquery.Point {
	left.SampleCount += right.SampleCount
	if left.FirstTsUnixMs == 0 || (right.FirstTsUnixMs > 0 && right.FirstTsUnixMs < left.FirstTsUnixMs) {
		left.FirstTsUnixMs = right.FirstTsUnixMs
	}
	if right.LastTsUnixMs > left.LastTsUnixMs {
		left.LastTsUnixMs = right.LastTsUnixMs
	}
	left.Metrics = mergeMetrics(left.Metrics, right.Metrics)
	return left
}

func mergeMetrics(left, right telemetryquery.Metrics) telemetryquery.Metrics {
	return telemetryquery.Metrics{
		SOCAvgPct:                averageFloatPtrs(left.SOCAvgPct, right.SOCAvgPct),
		SOCMinPct:                minFloatPtrs(left.SOCMinPct, right.SOCMinPct),
		SOCMaxPct:                maxFloatPtrs(left.SOCMaxPct, right.SOCMaxPct),
		ACInAvgW:                 sumFloatPtrs(left.ACInAvgW, right.ACInAvgW),
		ACInMaxW:                 sumFloatPtrs(left.ACInMaxW, right.ACInMaxW),
		PVAvgW:                   sumFloatPtrs(left.PVAvgW, right.PVAvgW),
		PVMaxW:                   sumFloatPtrs(left.PVMaxW, right.PVMaxW),
		DCAvgW:                   sumFloatPtrs(left.DCAvgW, right.DCAvgW),
		DCMaxW:                   sumFloatPtrs(left.DCMaxW, right.DCMaxW),
		LoadAvgW:                 sumFloatPtrs(left.LoadAvgW, right.LoadAvgW),
		LoadMaxW:                 sumFloatPtrs(left.LoadMaxW, right.LoadMaxW),
		NetAvgW:                  sumFloatPtrs(left.NetAvgW, right.NetAvgW),
		NetMinW:                  sumFloatPtrs(left.NetMinW, right.NetMinW),
		NetMaxW:                  sumFloatPtrs(left.NetMaxW, right.NetMaxW),
		BatteryAvgW:              sumFloatPtrs(left.BatteryAvgW, right.BatteryAvgW),
		BatteryMinW:              sumFloatPtrs(left.BatteryMinW, right.BatteryMinW),
		BatteryMaxW:              sumFloatPtrs(left.BatteryMaxW, right.BatteryMaxW),
		TempAvgC:                 averageFloatPtrs(left.TempAvgC, right.TempAvgC),
		TempMinC:                 minFloatPtrs(left.TempMinC, right.TempMinC),
		TempMaxC:                 maxFloatPtrs(left.TempMaxC, right.TempMaxC),
		ACOutputAvgW:             sumFloatPtrs(left.ACOutputAvgW, right.ACOutputAvgW),
		ACOutputMaxW:             sumFloatPtrs(left.ACOutputMaxW, right.ACOutputMaxW),
		SolarGeneratedWh:         sumFloatPtrs(left.SolarGeneratedWh, right.SolarGeneratedWh),
		ACInputEnergyWh:          sumFloatPtrs(left.ACInputEnergyWh, right.ACInputEnergyWh),
		ACOutputEnergyWh:         sumFloatPtrs(left.ACOutputEnergyWh, right.ACOutputEnergyWh),
		DCOutputEnergyWh:         sumFloatPtrs(left.DCOutputEnergyWh, right.DCOutputEnergyWh),
		LoadEnergyWh:             sumFloatPtrs(left.LoadEnergyWh, right.LoadEnergyWh),
		BatteryChargeEnergyWh:    sumFloatPtrs(left.BatteryChargeEnergyWh, right.BatteryChargeEnergyWh),
		BatteryDischargeEnergyWh: sumFloatPtrs(left.BatteryDischargeEnergyWh, right.BatteryDischargeEnergyWh),
	}
}

func sumFloatPtrs(values ...*float64) *float64 {
	total := 0.0
	found := false
	for _, value := range values {
		if value == nil {
			continue
		}
		total += *value
		found = true
	}
	if !found {
		return nil
	}
	return floatPtr(total)
}

func averageFloatPtrs(values ...*float64) *float64 {
	total := 0.0
	count := 0.0
	for _, value := range values {
		if value == nil {
			continue
		}
		total += *value
		count++
	}
	if count == 0 {
		return nil
	}
	return floatPtr(total / count)
}

func minFloatPtrs(values ...*float64) *float64 {
	var (
		best  float64
		found bool
	)
	for _, value := range values {
		if value == nil {
			continue
		}
		if !found || *value < best {
			best = *value
			found = true
		}
	}
	if !found {
		return nil
	}
	return floatPtr(best)
}

func maxFloatPtrs(values ...*float64) *float64 {
	var (
		best  float64
		found bool
	)
	for _, value := range values {
		if value == nil {
			continue
		}
		if !found || *value > best {
			best = *value
			found = true
		}
	}
	if !found {
		return nil
	}
	return floatPtr(best)
}

func summaryToProto(summary energydashboard.Summary, currency string) *telemetryv1.EnergySummary {
	return &telemetryv1.EnergySummary{
		SolarGeneratedKwh:    comparisonToProto(summary.SolarGeneratedKWh),
		LoadConsumedKwh:      comparisonToProto(summary.LoadConsumedKWh),
		SelfSufficiencyPct:   comparisonToProto(summary.SelfSufficiencyPct),
		BatteryNetKwh:        comparisonToProto(summary.BatteryNetKWh),
		EstimatedValue:       comparisonToProto(summary.EstimatedValue),
		EstimatedAcInputCost: comparisonToProto(summary.EstimatedACInputCost),
		Currency:             currency,
	}
}

func batteryToProto(summary energydashboard.BatterySummary) *telemetryv1.BatterySummary {
	return &telemetryv1.BatterySummary{
		ChargeKwh:    summary.ChargeKWh,
		DischargeKwh: summary.DischargeKWh,
		NetKwh:       summary.NetKWh,
		SocStartPct:  summary.StartSOCPct,
		SocEndPct:    summary.EndSOCPct,
		SocMinPct:    summary.MinSOCPct,
		SocMaxPct:    summary.MaxSOCPct,
	}
}

func comparisonToProto(value energydashboard.Comparison) *telemetryv1.EnergyValueComparison {
	out := &telemetryv1.EnergyValueComparison{
		Current:  value.Current,
		Previous: value.Previous,
		Delta:    value.Delta,
	}
	if value.DeltaPct != nil {
		out.DeltaPct = floatPtr(*value.DeltaPct)
	}
	return out
}

func pointsToProto(points []telemetryquery.Point) []*telemetryv1.RollupPoint {
	out := make([]*telemetryv1.RollupPoint, 0, len(points))
	for i := range points {
		out = append(out, pointToProto(points[i]))
	}
	return out
}

func pvPortHistoryToProto(rows []energydashboard.PVPortHistory) []*telemetryv1.EnergyPVPortHistory {
	out := make([]*telemetryv1.EnergyPVPortHistory, 0, len(rows))
	for _, row := range rows {
		var lastObservedUnixMs int64
		sampleCount := uint32(0)
		if !row.LastObservedAt.IsZero() {
			lastObservedUnixMs = row.LastObservedAt.UnixMilli()
		}
		switch {
		case row.SampleCount <= 0:
			sampleCount = 0
		case row.SampleCount > int(^uint32(0)):
			sampleCount = ^uint32(0)
		default:
			sampleCount = uint32(row.SampleCount)
		}
		out = append(out, &telemetryv1.EnergyPVPortHistory{
			DeviceId:           row.DeviceID,
			PortId:             row.PortID,
			PortLabel:          row.PortLabel,
			MaxObservedVolts:   row.MaxObservedVolts,
			MaxObservedAmps:    row.MaxObservedAmps,
			MaxObservedWatts:   row.MaxObservedWatts,
			LastObservedVolts:  row.LastObservedVolts,
			LastObservedAmps:   row.LastObservedAmps,
			LastObservedWatts:  row.LastObservedWatts,
			LastObservedUnixMs: lastObservedUnixMs,
			SampleCount:        sampleCount,
		})
	}
	return out
}

func floatPtr(value float64) *float64 {
	v := value
	return &v
}
