package main

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	telemetryv1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/telemetry/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/energydashboard"
	"github.com/jpaljasma/ecoflow-pulse/internal/grpcmw"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrypayload"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
	"github.com/jpaljasma/ecoflow-pulse/internal/valkeycache"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func (s *EnergyService) GetEnergyDashboard(ctx context.Context, req *telemetryv1.GetEnergyDashboardRequest) (*telemetryv1.GetEnergyDashboardResponse, error) {
	if s.queryReader == nil {
		return nil, status.Error(codes.Unavailable, "telemetry history unavailable")
	}
	scope, window, loc, preset, err := s.resolveEnergyScopeWindow(ctx, req.GetDeviceId(), req.GetUseAllDevices(), req.GetPreset(), req.GetTimezone(), req.GetDate())
	if err != nil {
		return nil, err
	}

	energyResolution := energyResolutionForPreset(preset)
	powerResolution := powerResolutionForPreset(preset)
	previousSeries := telemetryquery.Series{
		DeviceID:   scope.SeriesDeviceID(),
		Resolution: energyResolution,
		From:       window.PreviousFrom.UTC(),
		To:         window.PreviousTo.UTC(),
		Points:     []telemetryquery.Point{},
	}
	previousPowerSeries := telemetryquery.Series{
		DeviceID:   scope.SeriesDeviceID(),
		Resolution: powerResolution,
		From:       window.PreviousFrom.UTC(),
		To:         window.PreviousTo.UTC(),
		Points:     []telemetryquery.Point{},
	}
	var (
		currentSeries      telemetryquery.Series
		currentPowerSeries telemetryquery.Series
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		series, queryErr := s.queryScopeSeries(groupCtx, scope, energyResolution, window.From, window.To)
		if queryErr != nil {
			return queryErr
		}
		currentSeries = series
		return nil
	})
	group.Go(func() error {
		series, queryErr := s.queryScopeSeries(groupCtx, scope, powerResolution, window.From, window.To)
		if queryErr != nil {
			return queryErr
		}
		currentPowerSeries = series
		return nil
	})
	if req.GetIncludeComparison() {
		group.Go(func() error {
			series, queryErr := s.queryScopeSeries(groupCtx, scope, energyResolution, window.PreviousFrom, window.PreviousTo)
			if queryErr != nil {
				return queryErr
			}
			previousSeries = series
			return nil
		})
		group.Go(func() error {
			series, queryErr := s.queryScopeSeries(groupCtx, scope, powerResolution, window.PreviousFrom, window.PreviousTo)
			if queryErr != nil {
				return queryErr
			}
			previousPowerSeries = series
			return nil
		})
	}
	if err := group.Wait(); err != nil {
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
		PvPortHistory:        []*telemetryv1.EnergyPVPortHistory{},
	}
	s.maybeEnableHistoryCompression(ctx, resp)
	return resp, nil
}

func (s *EnergyService) GetEnergyPvPortHistory(ctx context.Context, req *telemetryv1.GetEnergyPvPortHistoryRequest) (*telemetryv1.GetEnergyPvPortHistoryResponse, error) {
	scope, window, loc, preset, err := s.resolveEnergyScopeWindow(ctx, req.GetDeviceId(), req.GetUseAllDevices(), req.GetPreset(), req.GetTimezone(), req.GetDate())
	if err != nil {
		return nil, err
	}
	rows, err := s.getCachedPVPortHistory(ctx, scope.ResolvedDeviceIDs, pvPortHistoryResolutionForPreset(preset), window.From, window.To)
	if err != nil {
		return nil, err
	}
	resp := &telemetryv1.GetEnergyPvPortHistoryResponse{
		Scope: &telemetryv1.EnergyScope{
			Mode:              scope.Mode,
			DeviceId:          scope.DeviceID,
			ResolvedDeviceIds: append([]string(nil), scope.ResolvedDeviceIDs...),
		},
		Window: &telemetryv1.EnergyWindow{
			Preset:             string(req.GetPreset()),
			Timezone:           loc.String(),
			FromUnixMs:         window.From.UnixMilli(),
			ToUnixMs:           window.To.UnixMilli(),
			PreviousFromUnixMs: window.PreviousFrom.UnixMilli(),
			PreviousToUnixMs:   window.PreviousTo.UnixMilli(),
		},
		PvPortHistory: pvPortHistoryToProto(rows),
	}
	s.maybeEnableHistoryCompression(ctx, resp)
	return resp, nil
}

func (s *EnergyService) GetEnergyCalendar(ctx context.Context, req *telemetryv1.GetEnergyCalendarRequest) (*telemetryv1.GetEnergyCalendarResponse, error) {
	if s.queryReader == nil {
		return nil, status.Error(codes.Unavailable, "telemetry history unavailable")
	}
	scope, loc, err := s.resolveEnergyCalendarScope(ctx, req.GetDeviceId(), req.GetUseAllDevices(), req.GetTimezone())
	if err != nil {
		return nil, err
	}
	now := s.now()
	cacheKey, cacheable := energyCalendarCacheKey(now, scope, loc, int(req.GetYear()), int(req.GetMonth()), req.GetGridPricePerKwh(), req.GetCurrency())
	if cacheable {
		if cached := s.readCachedEnergyCalendar(ctx, cacheKey); cached != nil {
			s.maybeEnableHistoryCompression(ctx, cached)
			return cached, nil
		}
		value, err, _ := s.energyCalendarGroup.Do(cacheKey, func() (any, error) {
			if cached := s.readCachedEnergyCalendar(ctx, cacheKey); cached != nil {
				return cached, nil
			}
			resp, buildErr := s.buildEnergyCalendarResponse(ctx, req, scope, loc, now)
			if buildErr != nil {
				return nil, buildErr
			}
			s.writeEnergyCalendarCache(cacheKey, resp, energyCalendarCacheExpiresAt(now, loc))
			return cloneEnergyCalendarResponse(resp), nil
		})
		if err != nil {
			return nil, err
		}
		resp, ok := value.(*telemetryv1.GetEnergyCalendarResponse)
		if !ok || resp == nil {
			return nil, status.Error(codes.Internal, "energy calendar cache returned invalid response")
		}
		s.maybeEnableHistoryCompression(ctx, resp)
		return resp, nil
	}
	resp, err := s.buildEnergyCalendarResponse(ctx, req, scope, loc, now)
	if err != nil {
		return nil, err
	}
	s.maybeEnableHistoryCompression(ctx, resp)
	return resp, nil
}

func (s *EnergyService) buildEnergyCalendarResponse(ctx context.Context, req *telemetryv1.GetEnergyCalendarRequest, scope energydashboard.Scope, loc *time.Location, now time.Time) (*telemetryv1.GetEnergyCalendarResponse, error) {
	calendar, err := energydashboard.BuildCalendarMonth(
		now,
		loc,
		int(req.GetYear()),
		int(req.GetMonth()),
		req.GetGridPricePerKwh(),
		req.GetCurrency(),
		func(from, to time.Time) (telemetryquery.Series, error) {
			return s.queryScopeSeries(ctx, scope, energyCalendarResolutionForRange(s.now(), loc, from, to), from, to)
		},
	)
	if err != nil {
		if code := status.Code(err); code != codes.Unknown {
			return nil, err
		}
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	resp := &telemetryv1.GetEnergyCalendarResponse{
		Scope: &telemetryv1.EnergyScope{
			Mode:              scope.Mode,
			DeviceId:          scope.DeviceID,
			ResolvedDeviceIds: append([]string(nil), scope.ResolvedDeviceIDs...),
		},
		Year:        int32(calendar.Year),
		Month:       int32(calendar.Month),
		Timezone:    loc.String(),
		VisibleDays: calendarDaysToProto(calendar.VisibleDays),
		SelectedMonthTotals: &telemetryv1.EnergyCalendarTotals{
			SolarGeneratedKwh: calendar.SelectedMonthTotals.SolarGeneratedKWh,
			EstimatedValue:    calendar.SelectedMonthTotals.EstimatedValue,
			Currency:          calendar.SelectedMonthTotals.Currency,
		},
	}
	return resp, nil
}

func energyCalendarCacheKey(now time.Time, scope energydashboard.Scope, loc *time.Location, year, month int, gridPricePerKWh float64, currency string) (string, bool) {
	if loc == nil {
		loc = time.UTC
	}
	localNow := now.In(loc)
	if localNow.Year() == year && int(localNow.Month()) == month {
		return "", false
	}
	parts := []string{
		"v1",
		loc.String(),
		strconv.Itoa(year),
		strconv.Itoa(month),
		scope.Mode,
		scope.DeviceID,
		strings.Join(scope.ResolvedDeviceIDs, ","),
		strconv.FormatFloat(gridPricePerKWh, 'g', -1, 64),
		currency,
	}
	return strings.Join(parts, "|"), true
}

func energyCalendarCacheExpiresAt(now time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	localNow := now.In(loc)
	nextLocalMidnight := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	midnightExpiry := nextLocalMidnight.UTC().Add(time.Second)
	ttlExpiry := now.UTC().Add(defaultEnergyCalendarCacheTTL)
	if midnightExpiry.Before(ttlExpiry) {
		return midnightExpiry
	}
	return ttlExpiry
}

func (s *EnergyService) readCachedEnergyCalendar(ctx context.Context, key string) *telemetryv1.GetEnergyCalendarResponse {
	if key == "" {
		return nil
	}
	if s.energyCalendarValkey != nil {
		cacheKey := s.energyCalendarValkey.Key("calendar", key)
		raw, ok, err := s.energyCalendarValkey.GetBytes(ctx, cacheKey, valkeycache.ReadOptions{})
		if err == nil && ok {
			var out telemetryv1.GetEnergyCalendarResponse
			unmarshalErr := proto.Unmarshal(raw, &out)
			if unmarshalErr == nil {
				return cloneEnergyCalendarResponse(&out)
			}
			s.log.Warn("energy calendar valkey cache decode failed", "error", unmarshalErr.Error())
		} else if err != nil {
			s.log.Warn("energy calendar valkey cache read failed", "error", err.Error())
		}
	}
	now := s.now().UTC()
	s.energyCalendarMu.Lock()
	defer s.energyCalendarMu.Unlock()
	entry, ok := s.energyCalendarCache[key]
	if !ok {
		return nil
	}
	if !now.Before(entry.expiresAt) {
		delete(s.energyCalendarCache, key)
		return nil
	}
	return cloneEnergyCalendarResponse(entry.resp)
}

func (s *EnergyService) writeEnergyCalendarCache(key string, resp *telemetryv1.GetEnergyCalendarResponse, expiresAt time.Time) {
	if key == "" || resp == nil {
		return
	}
	now := s.now().UTC()
	if s.energyCalendarValkey != nil {
		if ttl := expiresAt.UTC().Sub(now); ttl > 0 {
			if encoded, err := proto.Marshal(resp); err != nil {
				s.log.Warn("energy calendar valkey cache encode failed", "error", err.Error())
			} else if err := s.energyCalendarValkey.SetBytes(
				context.Background(),
				s.energyCalendarValkey.Key("calendar", key),
				encoded,
				valkeycache.SetOptions{TTL: ttl, ContentType: "application/protobuf"},
			); err != nil {
				s.log.Warn("energy calendar valkey cache write failed", "error", err.Error())
			}
		}
	}
	s.energyCalendarMu.Lock()
	defer s.energyCalendarMu.Unlock()
	for existingKey, entry := range s.energyCalendarCache {
		if !now.Before(entry.expiresAt) {
			delete(s.energyCalendarCache, existingKey)
		}
	}
	if len(s.energyCalendarCache) >= 128 {
		for existingKey := range s.energyCalendarCache {
			delete(s.energyCalendarCache, existingKey)
			break
		}
	}
	s.energyCalendarCache[key] = energyCalendarCacheEntry{
		resp:      cloneEnergyCalendarResponse(resp),
		expiresAt: expiresAt.UTC(),
	}
}

func cloneEnergyCalendarResponse(resp *telemetryv1.GetEnergyCalendarResponse) *telemetryv1.GetEnergyCalendarResponse {
	if resp == nil {
		return nil
	}
	cloned, ok := proto.Clone(resp).(*telemetryv1.GetEnergyCalendarResponse)
	if !ok {
		return nil
	}
	return cloned
}

func energyCalendarResolutionForRange(now time.Time, loc *time.Location, from, to time.Time) telemetryquery.Resolution {
	if loc == nil {
		loc = time.UTC
	}
	localFrom := from.In(loc)
	localTo := to.In(loc)
	localNow := now.In(loc)
	if !localNow.Before(localFrom) && localNow.Before(localTo) {
		return telemetryquery.ResolutionHour
	}
	utcFrom := from.UTC()
	utcTo := to.UTC()
	if utcFrom.Hour() != 0 || utcFrom.Minute() != 0 || utcFrom.Second() != 0 || utcFrom.Nanosecond() != 0 ||
		utcTo.Hour() != 0 || utcTo.Minute() != 0 || utcTo.Second() != 0 || utcTo.Nanosecond() != 0 {
		return telemetryquery.ResolutionHour
	}
	return telemetryquery.ResolutionDay
}

func (s *EnergyService) getCachedPVPortHistory(ctx context.Context, deviceIDs []string, resolution telemetryquery.Resolution, from, to time.Time) ([]energydashboard.PVPortHistory, error) {
	key := pvPortHistoryCacheKey(deviceIDs, resolution, from, to)
	now := s.now()
	if rows, ok := s.readPVPortHistoryCache(ctx, key, now); ok {
		return rows, nil
	}
	value, err, _ := s.pvPortHistoryGroup.Do(key, func() (any, error) {
		if rows, ok := s.readPVPortHistoryCache(ctx, key, s.now()); ok {
			return rows, nil
		}
		rows, err := s.queryScopePVPortHistory(ctx, deviceIDs, resolution, from, to)
		if err != nil {
			return nil, err
		}
		s.writePVPortHistoryCache(key, rows, s.now().Add(defaultPVPortHistoryCacheTTL))
		return rows, nil
	})
	if err != nil {
		return nil, err
	}
	rows, _ := value.([]energydashboard.PVPortHistory)
	return rows, nil
}

func (s *EnergyService) readPVPortHistoryCache(ctx context.Context, key string, now time.Time) ([]energydashboard.PVPortHistory, bool) {
	if s.pvPortHistoryValkey != nil {
		var rows []energydashboard.PVPortHistory
		ok, err := s.pvPortHistoryValkey.GetJSON(ctx, s.pvPortHistoryValkey.Key("pv-port-history", key), &rows, valkeycache.ReadOptions{})
		if err == nil && ok {
			return clonePVPortHistoryRows(rows), true
		}
		if err != nil {
			s.log.Warn("pv port history valkey cache read failed", "error", err.Error())
		}
	}
	s.pvPortHistoryMu.Lock()
	defer s.pvPortHistoryMu.Unlock()
	entry, ok := s.pvPortHistoryCache[key]
	if !ok {
		return nil, false
	}
	if !now.Before(entry.expiresAt) {
		delete(s.pvPortHistoryCache, key)
		return nil, false
	}
	return clonePVPortHistoryRows(entry.rows), true
}

func (s *EnergyService) writePVPortHistoryCache(key string, rows []energydashboard.PVPortHistory, expiresAt time.Time) {
	if s.pvPortHistoryValkey != nil {
		if ttl := expiresAt.UTC().Sub(s.now().UTC()); ttl > 0 {
			if err := s.pvPortHistoryValkey.SetJSON(
				context.Background(),
				s.pvPortHistoryValkey.Key("pv-port-history", key),
				rows,
				valkeycache.SetOptions{TTL: ttl},
			); err != nil {
				s.log.Warn("pv port history valkey cache write failed", "error", err.Error())
			}
		}
	}
	s.pvPortHistoryMu.Lock()
	defer s.pvPortHistoryMu.Unlock()
	now := s.now()
	for existingKey, entry := range s.pvPortHistoryCache {
		if !now.Before(entry.expiresAt) {
			delete(s.pvPortHistoryCache, existingKey)
		}
	}
	if len(s.pvPortHistoryCache) >= 64 {
		for existingKey := range s.pvPortHistoryCache {
			delete(s.pvPortHistoryCache, existingKey)
			break
		}
	}
	cachedRows := make([]energydashboard.PVPortHistory, len(rows))
	copy(cachedRows, rows)
	s.pvPortHistoryCache[key] = pvPortHistoryCacheEntry{
		rows:      cachedRows,
		expiresAt: expiresAt,
	}
}

func clonePVPortHistoryRows(rows []energydashboard.PVPortHistory) []energydashboard.PVPortHistory {
	cloned := make([]energydashboard.PVPortHistory, len(rows))
	copy(cloned, rows)
	return cloned
}

func pvPortHistoryCacheKey(deviceIDs []string, resolution telemetryquery.Resolution, from, to time.Time) string {
	parts := append([]string(nil), deviceIDs...)
	sort.Strings(parts)
	return strings.Join(parts, ",") + "|" + resolution.String() + "|" + from.UTC().Format(time.RFC3339Nano) + "|" + to.UTC().Format(time.RFC3339Nano)
}

func (s *EnergyService) resolveEnergyScopeWindow(ctx context.Context, deviceID string, useAllDevices bool, presetRaw, timezone, selectedDate string) (energydashboard.Scope, energydashboard.Window, *time.Location, energydashboard.Preset, error) {
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
	visibleDeviceIDs, err := s.resolveVisibleDeviceIDs(ctx, deviceID, useAllDevices)
	if err != nil {
		return energydashboard.Scope{}, energydashboard.Window{}, nil, "", err
	}
	scope, err := energydashboard.ResolveScope(scopeRequestValue(deviceID, useAllDevices), visibleDeviceIDs)
	if err != nil {
		return energydashboard.Scope{}, energydashboard.Window{}, nil, "", status.Error(codes.PermissionDenied, err.Error())
	}
	window, err := energydashboard.ResolveWindowForDate(s.now(), loc, preset, selectedDate)
	if err != nil {
		return energydashboard.Scope{}, energydashboard.Window{}, nil, "", status.Error(codes.InvalidArgument, err.Error())
	}
	return scope, window, loc, preset, nil
}

func (s *EnergyService) resolveEnergyCalendarScope(ctx context.Context, deviceID string, useAllDevices bool, timezone string) (energydashboard.Scope, *time.Location, error) {
	loc := time.UTC
	var err error
	if tz := strings.TrimSpace(timezone); tz != "" {
		loc, err = time.LoadLocation(tz)
		if err != nil {
			return energydashboard.Scope{}, nil, status.Errorf(codes.InvalidArgument, "invalid timezone: %v", err)
		}
	}
	visibleDeviceIDs, err := s.resolveVisibleDeviceIDs(ctx, deviceID, useAllDevices)
	if err != nil {
		return energydashboard.Scope{}, nil, err
	}
	scope, err := energydashboard.ResolveScope(scopeRequestValue(deviceID, useAllDevices), visibleDeviceIDs)
	if err != nil {
		return energydashboard.Scope{}, nil, status.Error(codes.PermissionDenied, err.Error())
	}
	return scope, loc, nil
}

func (s *EnergyService) queryScopePVPortHistory(ctx context.Context, deviceIDs []string, resolution telemetryquery.Resolution, from, to time.Time) ([]energydashboard.PVPortHistory, error) {
	if len(deviceIDs) == 0 {
		return nil, nil
	}
	if reader, ok := s.queryReader.(telemetryquery.PVPortHistoryReader); ok {
		rows, err := reader.QueryPVPortHistory(ctx, telemetryquery.PVPortHistoryQuery{
			DeviceIDs:  deviceIDs,
			Resolution: resolution,
			From:       from.UTC(),
			To:         to.UTC(),
			Limit:      s.maxQueryBuckets,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "query pv-port history rollups: %v", err)
		}
		if len(rows) > 0 {
			return pvPortHistoryRows(rows), nil
		}
	}
	if s.archiveManifestStore == nil || s.archiveObjectReader == nil || s.controlPlaneStore == nil {
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

	summaries := make([][]energydashboard.PVPortHistory, len(objects))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(8)
	for i, object := range objects {
		i := i
		object := object
		group.Go(func() error {
			body, err := s.archiveObjectReader.ReadObject(groupCtx, object.ObjectBucket, object.ObjectKey)
			if err != nil {
				if isMissingArchiveObjectError(err) {
					s.log.Warn(
						"skip missing archive object for energy pv history",
						"bucket", object.ObjectBucket,
						"key", object.ObjectKey,
						"error", err.Error(),
					)
					return nil
				}
				return status.Errorf(codes.Internal, "read archive object for energy pv history: %v", err)
			}
			frames, err := replaycli.DecodeEnvelopeFrames(body)
			if err != nil {
				return status.Errorf(codes.Internal, "decode archive object for energy pv history: %v", err)
			}
			rows, err := energydashboard.SummarizePVPortHistoryFrames(frames, func(env *envelopev1.TelemetryEnvelope) bool {
				if !telemetrypayload.IsNormalizedParamsPayloadType(env.GetPayloadType()) {
					return false
				}
				providerDeviceID := strings.ToUpper(strings.TrimSpace(env.GetEcoflowSn()))
				if labels := env.GetLabels(); len(labels) > 0 {
					if candidate := strings.ToUpper(strings.TrimSpace(labels["provider_device_id"])); candidate != "" {
						providerDeviceID = candidate
					}
				}
				if _, ok := providerFilter[providerDeviceID]; !ok {
					return false
				}
				ts := envelopeTimestamp(env)
				return !ts.Before(from) && ts.Before(to)
			})
			if err != nil {
				return status.Errorf(codes.Internal, "unmarshal archive envelope for energy pv history: %v", err)
			}
			summaries[i] = rows
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	rows := energydashboard.MergePVPortHistorySets(summaries...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].DeviceID == rows[j].DeviceID {
			return rows[i].PortID < rows[j].PortID
		}
		return rows[i].DeviceID < rows[j].DeviceID
	})
	return rows, nil
}

func pvPortHistoryResolutionForPreset(preset energydashboard.Preset) telemetryquery.Resolution {
	switch preset {
	case energydashboard.PresetToday, energydashboard.PresetPast24Hours, energydashboard.PresetYesterday:
		return telemetryquery.ResolutionMinute
	case energydashboard.PresetThisWeek, energydashboard.PresetPreviousWeek, energydashboard.PresetThisMonth, energydashboard.PresetLast7Days:
		return telemetryquery.ResolutionHour
	default:
		return telemetryquery.ResolutionDay
	}
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

func (s *EnergyService) resolveVisibleDeviceIDs(ctx context.Context, requestedDeviceID string, useAllDevices bool) ([]string, error) {
	requestedDeviceID = strings.TrimSpace(requestedDeviceID)
	if !useAllDevices {
		if requestedDeviceID == "" {
			return nil, status.Error(codes.InvalidArgument, "device_id required when use_all_devices is false")
		}
		if err := authorizeDeviceAccess(ctx, s.controlPlaneStore, requestedDeviceID); err != nil {
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
	case energydashboard.PresetToday, energydashboard.PresetPast24Hours, energydashboard.PresetYesterday:
		return telemetryquery.ResolutionHour
	case energydashboard.PresetLast30Days, energydashboard.PresetLastMonth:
		return telemetryquery.ResolutionDay
	default:
		return telemetryquery.ResolutionDay
	}
}

func powerResolutionForPreset(preset energydashboard.Preset) telemetryquery.Resolution {
	switch preset {
	case energydashboard.PresetToday, energydashboard.PresetPast24Hours, energydashboard.PresetYesterday:
		return telemetryquery.ResolutionFiveMinutes
	case energydashboard.PresetThisWeek, energydashboard.PresetPreviousWeek, energydashboard.PresetThisMonth, energydashboard.PresetLast7Days:
		return telemetryquery.ResolutionHour
	case energydashboard.PresetLast30Days, energydashboard.PresetLastMonth:
		return telemetryquery.ResolutionDay
	default:
		return telemetryquery.ResolutionDay
	}
}

func (s *EnergyService) queryScopeSeries(ctx context.Context, scope energydashboard.Scope, resolution telemetryquery.Resolution, from, to time.Time) (telemetryquery.Series, error) {
	if len(scope.ResolvedDeviceIDs) == 1 {
		series, err := s.queryReader.QueryRange(ctx, telemetryquery.RangeQuery{
			DeviceID:   scope.ResolvedDeviceIDs[0],
			Resolution: resolution,
			From:       from.UTC(),
			To:         to.UTC(),
			Limit:      s.maxQueryBuckets,
		})
		if err != nil {
			return telemetryquery.Series{}, s.mapQueryError(err)
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
			AggregateID: scope.SeriesDeviceID(),
		})
		if err != nil {
			return telemetryquery.Series{}, s.mapQueryError(err)
		}
		return series, nil
	}
	aggregated := telemetryquery.Series{
		DeviceID:   scope.SeriesDeviceID(),
		Resolution: resolution,
		From:       from.UTC(),
		To:         to.UTC(),
		Points:     []telemetryquery.Point{},
	}
	for _, deviceID := range scope.ResolvedDeviceIDs {
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

func pvPortHistoryRows(rows []telemetryquery.PVPortHistory) []energydashboard.PVPortHistory {
	out := make([]energydashboard.PVPortHistory, 0, len(rows))
	for _, row := range rows {
		out = append(out, energydashboard.PVPortHistory{
			DeviceID:          row.DeviceID,
			PortID:            row.PortID,
			PortLabel:         row.PortLabel,
			MaxObservedVolts:  row.MaxObservedVolts,
			MaxObservedAmps:   row.MaxObservedAmps,
			MaxObservedWatts:  row.MaxObservedWatts,
			LastObservedVolts: row.LastObservedVolts,
			LastObservedAmps:  row.LastObservedAmps,
			LastObservedWatts: row.LastObservedWatts,
			LastObservedAt:    row.LastObservedAt,
			SampleCount:       row.SampleCount,
		})
	}
	return out
}

func calendarDaysToProto(days []energydashboard.CalendarDay) []*telemetryv1.EnergyCalendarDay {
	out := make([]*telemetryv1.EnergyCalendarDay, 0, len(days))
	for _, day := range days {
		out = append(out, &telemetryv1.EnergyCalendarDay{
			Date:              day.Date,
			Year:              int32(day.Year),
			Month:             int32(day.Month),
			Day:               int32(day.Day),
			InSelectedMonth:   day.InSelectedMonth,
			HasData:           day.HasData,
			IsFuture:          day.IsFuture,
			SolarGeneratedKwh: day.SolarGeneratedKWh,
			EstimatedValue:    day.EstimatedValue,
			Currency:          day.Currency,
		})
	}
	return out
}

func floatPtr(value float64) *float64 {
	v := value
	return &v
}
