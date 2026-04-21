package weatherd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/budget"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/cachekey"
)

var ErrUpstreamBudgetExceeded = errors.New("weather upstream budget exhausted")

type Upstream interface {
	FetchForecast(ctx context.Context, req Request) (*Bundle, error)
	FetchForecastBatch(ctx context.Context, reqs []Request) ([]Bundle, error)
	FetchPreviousRuns(ctx context.Context, req Request) (*Bundle, error)
	FetchHistoricalForecast(ctx context.Context, req Request) (*Bundle, error)
}

type Config struct {
	HotTTL             time.Duration
	RecentActiveWindow time.Duration
	NowFn              func() time.Time
	Metrics            *Metrics
}

type Service struct {
	upstream     Upstream
	hotCache     HotCache
	snapshots    SnapshotStore
	budget       *budget.Manager
	hotTTL       time.Duration
	activeWindow time.Duration
	nowFn        func() time.Time
	metrics      *Metrics
}

func NewService(upstream Upstream, hotCache HotCache, snapshots SnapshotStore, budgetManager *budget.Manager, cfg Config) (*Service, error) {
	if upstream == nil {
		return nil, errors.New("weather upstream is required")
	}
	if hotCache == nil {
		return nil, errors.New("weather hot cache is required")
	}
	if snapshots == nil {
		return nil, errors.New("weather snapshot store is required")
	}
	if budgetManager == nil {
		return nil, errors.New("weather budget manager is required")
	}
	if cfg.HotTTL <= 0 {
		cfg.HotTTL = 4 * time.Hour
	}
	if cfg.RecentActiveWindow <= 0 {
		cfg.RecentActiveWindow = 7 * 24 * time.Hour
	}
	if cfg.NowFn == nil {
		cfg.NowFn = time.Now
	}
	return &Service{
		upstream:     upstream,
		hotCache:     hotCache,
		snapshots:    snapshots,
		budget:       budgetManager,
		hotTTL:       cfg.HotTTL,
		activeWindow: cfg.RecentActiveWindow,
		nowFn:        cfg.NowFn,
		metrics:      cfg.Metrics,
	}, nil
}

func (s *Service) Get7DayForecast(ctx context.Context, req Request) (*Bundle, error) {
	start := time.Now()
	metricReq := metricRequest(req)
	source := "upstream"
	key, err := s.snapshots.FindCanonicalLocationKeyByRequest(ctx, metricReq)
	if err != nil {
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_7_day_forecast", "canonical_lookup", err, time.Since(start))
		}
		return nil, err
	}
	if key != "" {
		if cached, err := s.hotCache.GetForecast(ctx, key); err != nil {
			if s.metrics != nil {
				s.metrics.ObserveRequest("get_7_day_forecast", "hot_cache_lookup", err, time.Since(start))
			}
			return nil, err
		} else if cached != nil {
			if cached.StaleAfter.After(s.nowFn().UTC()) {
				source = "hot_cache_fresh"
				out := cached.Bundle
				if s.metrics != nil {
					s.metrics.ObserveRequest("get_7_day_forecast", source, nil, time.Since(start))
				}
				return s.convertBundleForResponse(ctx, &out, req.UnitSystem), nil
			}
		}
		if latest, err := s.snapshots.LatestBundle(ctx, key); err != nil {
			if s.metrics != nil {
				s.metrics.ObserveRequest("get_7_day_forecast", "snapshot_lookup", err, time.Since(start))
			}
			return nil, err
		} else if latest != nil && latest.Provenance.IssuedAt.Add(s.hotTTL).After(s.nowFn().UTC()) {
			source = "snapshot_fresh"
			latestCopy := *latest
			_ = s.hotCache.PutForecast(ctx, key, latestCopy, s.hotTTL)
			if s.metrics != nil {
				s.metrics.ObserveRequest("get_7_day_forecast", source, nil, time.Since(start))
			}
			return s.convertBundleForResponse(ctx, &latestCopy, req.UnitSystem), nil
		}
	}
	if !s.budget.Allow(2) {
		source = "budget_rejected"
		if s.metrics != nil {
			s.metrics.ObserveBudgetDenied(s.budgetDeniedWindow(2))
			s.metrics.ObserveBudgetSnapshot(s.budget.Snapshot())
		}
		if key != "" {
			if cached, err := s.hotCache.GetForecast(ctx, key); err == nil && cached != nil {
				source = "hot_cache_stale"
				out := cached.Bundle
				if s.metrics != nil {
					s.metrics.ObserveRequest("get_7_day_forecast", source, nil, time.Since(start))
				}
				return s.convertBundleForResponse(ctx, &out, req.UnitSystem), nil
			}
			if latest, err := s.snapshots.LatestBundle(ctx, key); err == nil && latest != nil {
				source = "snapshot_stale"
				if s.metrics != nil {
					s.metrics.ObserveRequest("get_7_day_forecast", source, nil, time.Since(start))
				}
				return s.convertBundleForResponse(ctx, latest, req.UnitSystem), nil
			}
		}
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_7_day_forecast", source, ErrUpstreamBudgetExceeded, time.Since(start))
		}
		return nil, ErrUpstreamBudgetExceeded
	}
	if s.metrics != nil {
		s.metrics.ObserveBudgetUnitsConsumed("forecast", 2)
		s.metrics.ObserveBudgetSnapshot(s.budget.Snapshot())
	}
	upstreamStart := time.Now()
	bundle, err := s.upstream.FetchForecast(ctx, metricReq)
	if s.metrics != nil {
		s.metrics.ObserveUpstreamCall("forecast", err, time.Since(upstreamStart))
	}
	if err != nil {
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_7_day_forecast", source, err, time.Since(start))
		}
		return nil, err
	}
	source = "upstream"
	bundle.Provenance.IssuedAt = s.nowFn().UTC()
	bundle.Provenance.CanonicalLocationKey = canonicalKeyFromBundle(bundle, metricReq)
	if err := s.snapshots.SaveForecastBundle(ctx, metricReq, *bundle); err != nil {
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_7_day_forecast", "snapshot_save", err, time.Since(start))
		}
		return nil, err
	}
	if err := s.hotCache.PutForecast(ctx, bundle.Provenance.CanonicalLocationKey, *bundle, s.hotTTL); err != nil {
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_7_day_forecast", "hot_cache_write", err, time.Since(start))
		}
		return nil, err
	}
	if err := s.snapshots.TouchRefreshCandidate(ctx, bundle.Provenance.CanonicalLocationKey, metricReq, s.nowFn().UTC()); err != nil {
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_7_day_forecast", "refresh_candidate_touch", err, time.Since(start))
		}
		return nil, err
	}
	if s.metrics != nil {
		s.metrics.ObserveRequest("get_7_day_forecast", source, nil, time.Since(start))
	}
	return s.convertBundleForResponse(ctx, bundle, req.UnitSystem), nil
}

func (s *Service) GetYesterdayVerification(ctx context.Context, req Request) (*VerificationResult, error) {
	start := time.Now()
	metricReq := metricRequest(req)
	var source string
	key, err := s.snapshots.FindCanonicalLocationKeyByRequest(ctx, metricReq)
	if err != nil {
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_yesterday_verification", "canonical_lookup", err, time.Since(start))
		}
		return nil, err
	}
	if key != "" {
		loc := loadLocation(metricReq.Timezone)
		yesterdayStart, _ := yesterdayBounds(s.nowFn, loc)
		cachedVerification, err := s.snapshots.LoadVerification(ctx, key, yesterdayStart)
		if err != nil {
			if s.metrics != nil {
				s.metrics.ObserveRequest("get_yesterday_verification", "verification_cache_lookup", err, time.Since(start))
			}
			return nil, err
		}
		if cachedVerification != nil {
			source = "stored_verification"
			if req.UnitSystem == UnitSystemImperial {
				converted := *cachedVerification
				converted.UnitSystem = UnitSystemImperial
				for idx := range converted.Hourly {
					converted.Hourly[idx].ForecastRaw = ForecastValues(converted.Hourly[idx].ForecastRaw, UnitSystemImperial)
					converted.Hourly[idx].ForecastCorrected = ForecastValues(converted.Hourly[idx].ForecastCorrected, UnitSystemImperial)
					converted.Hourly[idx].Actual = ForecastValues(converted.Hourly[idx].Actual, UnitSystemImperial)
				}
				if s.metrics != nil {
					s.metrics.ObserveRequest("get_yesterday_verification", source, nil, time.Since(start))
				}
				return &converted, nil
			}
			if s.metrics != nil {
				s.metrics.ObserveRequest("get_yesterday_verification", source, nil, time.Since(start))
			}
			return cachedVerification, nil
		}
	}
	latest, err := s.ensureLatestBundle(ctx, metricReq, key)
	if err != nil {
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_yesterday_verification", "ensure_latest_bundle", err, time.Since(start))
		}
		return nil, err
	}
	key = latest.Provenance.CanonicalLocationKey
	loc := loadLocation(latest.Provenance.Timezone)
	yesterdayStart, todayStart := yesterdayBounds(s.nowFn, loc)
	actualByTime := hourlyByTime(filterHourly(latest.Hourly, yesterdayStart, todayStart))

	var (
		forecastBundle      *Bundle
		verificationSource  string
		biasStates          []BiasState
		forecastLoadErr     error
		biasLoadErr         error
		refreshCandidateErr error
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		forecastBundle, verificationSource, forecastLoadErr = s.loadVerificationForecast(ctx, metricReq, key, yesterdayStart)
	}()
	go func() {
		defer wg.Done()
		biasStates, biasLoadErr = s.snapshots.LoadBiasStates(ctx, key)
	}()
	go func() {
		defer wg.Done()
		refreshCandidateErr = s.snapshots.TouchRefreshCandidate(ctx, key, metricReq, s.nowFn().UTC())
	}()
	wg.Wait()
	if refreshCandidateErr != nil {
		return nil, refreshCandidateErr
	}
	if forecastLoadErr != nil {
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_yesterday_verification", "verification_forecast_load", forecastLoadErr, time.Since(start))
		}
		return nil, forecastLoadErr
	}
	if biasLoadErr != nil {
		return nil, biasLoadErr
	}
	source = verificationSource
	forecastByTime := hourlyByTime(filterHourly(forecastBundle.Hourly, yesterdayStart, todayStart))
	biasIndex := BuildBiasIndex(biasStates)
	out := &VerificationResult{
		Provenance: Provenance{
			Source:               "open_meteo",
			ModelSelection:       "best_match",
			ActualSource:         "past_days",
			VerificationSource:   verificationSource,
			Timezone:             latest.Provenance.Timezone,
			CanonicalLocationKey: key,
			IssuedAt:             s.nowFn().UTC(),
		},
		UnitSystem:       UnitSystemMetric,
		VerificationDate: yesterdayStart.UTC(),
		Hourly:           make([]VerificationHour, 0, len(actualByTime)),
	}
	var windDirectionDiffs []float64
	var updatedStates []BiasState
	for ts, actualPoint := range actualByTime {
		forecastPoint, ok := forecastByTime[ts]
		if !ok {
			continue
		}
		correctedPoint := ApplyForecastBias(forecastPoint, loc, biasIndex)
		row := VerificationHour{
			Time:              actualPoint.Time.UTC(),
			ForecastCondition: forecastPoint.Condition,
			ActualCondition:   actualPoint.Condition,
			ForecastRaw:       forecastPoint.Raw,
			ForecastCorrected: correctedPoint.Corrected,
			Actual:            actualPoint.Raw,
		}
		out.Hourly = append(out.Hourly, row)
		if forecastPoint.Raw.WindDirectionDegrees != nil && actualPoint.Raw.WindDirectionDegrees != nil {
			windDirectionDiffs = append(windDirectionDiffs, CircularErrorDegrees(*forecastPoint.Raw.WindDirectionDegrees, *actualPoint.Raw.WindDirectionDegrees))
		}
		hourOfDay := actualPoint.Time.In(loc).Hour()
		updatedStates = append(updatedStates, UpdateBiasStates(s.nowFn().UTC(), key, forecastPoint.Raw, actualPoint.Raw, hourOfDay, biasStates)...)
	}
	sort.Slice(out.Hourly, func(i, j int) bool { return out.Hourly[i].Time.Before(out.Hourly[j].Time) })
	out.Summary = buildVerificationSummary(out.Hourly, windDirectionDiffs)
	out.Summary.CircularWindDirectionMeanAbsoluteError = CircularWindDirectionMAE(windDirectionDiffs)
	if len(updatedStates) > 0 {
		if err := s.snapshots.UpsertBiasStates(ctx, dedupeBiasStates(updatedStates)); err != nil {
			if s.metrics != nil {
				s.metrics.ObserveRequest("get_yesterday_verification", "bias_state_upsert", err, time.Since(start))
			}
			return nil, err
		}
	}
	if err := s.snapshots.SaveVerification(ctx, *out); err != nil {
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_yesterday_verification", "verification_save", err, time.Since(start))
		}
		return nil, err
	}
	if req.UnitSystem == UnitSystemImperial {
		converted := *out
		converted.UnitSystem = UnitSystemImperial
		for idx := range converted.Hourly {
			converted.Hourly[idx].ForecastRaw = ForecastValues(converted.Hourly[idx].ForecastRaw, UnitSystemImperial)
			converted.Hourly[idx].ForecastCorrected = ForecastValues(converted.Hourly[idx].ForecastCorrected, UnitSystemImperial)
			converted.Hourly[idx].Actual = ForecastValues(converted.Hourly[idx].Actual, UnitSystemImperial)
		}
		if s.metrics != nil {
			s.metrics.ObserveRequest("get_yesterday_verification", source, nil, time.Since(start))
		}
		return &converted, nil
	}
	if s.metrics != nil {
		s.metrics.ObserveRequest("get_yesterday_verification", source, nil, time.Since(start))
	}
	return out, nil
}

func (s *Service) RefreshRecentLocations(ctx context.Context) error {
	now := s.nowFn().UTC()
	candidates, err := s.snapshots.ListDueRefreshCandidates(ctx, now.Add(-s.activeWindow), now)
	if err != nil {
		return err
	}
	groups := groupCandidates(candidates)
	for _, group := range groups {
		cost := 2 * len(group)
		if !s.budget.Allow(cost) {
			if s.metrics != nil {
				s.metrics.ObserveBudgetDenied(s.budgetDeniedWindow(cost))
				s.metrics.ObserveBudgetSnapshot(s.budget.Snapshot())
			}
			continue
		}
		if s.metrics != nil {
			s.metrics.ObserveBudgetUnitsConsumed("forecast_batch", cost)
			s.metrics.ObserveBudgetSnapshot(s.budget.Snapshot())
		}
		reqs := make([]Request, 0, len(group))
		for _, candidate := range group {
			reqs = append(reqs, metricRequest(candidate.Request))
		}
		upstreamStart := time.Now()
		bundles, err := s.upstream.FetchForecastBatch(ctx, reqs)
		if s.metrics != nil {
			s.metrics.ObserveUpstreamCall("forecast_batch", err, time.Since(upstreamStart))
		}
		if err != nil {
			return err
		}
		for idx, bundle := range bundles {
			req := reqs[idx]
			bundle.Provenance.IssuedAt = s.nowFn().UTC()
			bundle.Provenance.CanonicalLocationKey = canonicalKeyFromBundle(&bundle, req)
			if err := s.snapshots.SaveForecastBundle(ctx, req, bundle); err != nil {
				return err
			}
			if err := s.hotCache.PutForecast(ctx, bundle.Provenance.CanonicalLocationKey, bundle, s.hotTTL); err != nil {
				return err
			}
			refreshedAt := s.nowFn().UTC()
			if err := s.snapshots.MarkRefreshCandidateRefreshed(ctx, bundle.Provenance.CanonicalLocationKey, refreshedAt, refreshedAt.Add(s.hotTTL)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) ensureLatestBundle(ctx context.Context, req Request, key string) (*Bundle, error) {
	if key != "" {
		if cached, err := s.hotCache.GetForecast(ctx, key); err != nil {
			return nil, err
		} else if cached != nil {
			out := cached.Bundle
			return &out, nil
		}
		if latest, err := s.snapshots.LatestBundle(ctx, key); err != nil {
			return nil, err
		} else if latest != nil {
			return latest, nil
		}
	}
	if !s.budget.Allow(2) {
		return nil, ErrUpstreamBudgetExceeded
	}
	if s.metrics != nil {
		s.metrics.ObserveBudgetUnitsConsumed("forecast", 2)
		s.metrics.ObserveBudgetSnapshot(s.budget.Snapshot())
	}
	upstreamStart := time.Now()
	bundle, err := s.upstream.FetchForecast(ctx, req)
	if s.metrics != nil {
		s.metrics.ObserveUpstreamCall("forecast", err, time.Since(upstreamStart))
	}
	if err != nil {
		return nil, err
	}
	bundle.Provenance.IssuedAt = s.nowFn().UTC()
	bundle.Provenance.CanonicalLocationKey = canonicalKeyFromBundle(bundle, req)
	if err := s.snapshots.SaveForecastBundle(ctx, req, *bundle); err != nil {
		return nil, err
	}
	if err := s.hotCache.PutForecast(ctx, bundle.Provenance.CanonicalLocationKey, *bundle, s.hotTTL); err != nil {
		return nil, err
	}
	return bundle, nil
}

func (s *Service) loadVerificationForecast(ctx context.Context, req Request, key string, yesterdayStart time.Time) (*Bundle, string, error) {
	prior, err := s.snapshots.LatestBundleBefore(ctx, key, yesterdayStart)
	if err != nil {
		return nil, "", err
	}
	if prior != nil {
		return prior, "snapshot", nil
	}
	if !s.budget.Allow(2) {
		if s.metrics != nil {
			s.metrics.ObserveBudgetDenied(s.budgetDeniedWindow(2))
			s.metrics.ObserveBudgetSnapshot(s.budget.Snapshot())
		}
		return nil, "", ErrUpstreamBudgetExceeded
	}
	if s.metrics != nil {
		s.metrics.ObserveBudgetUnitsConsumed("previous_runs", 2)
		s.metrics.ObserveBudgetSnapshot(s.budget.Snapshot())
	}
	upstreamStart := time.Now()
	fallback, err := s.upstream.FetchPreviousRuns(ctx, req)
	if s.metrics != nil {
		s.metrics.ObserveUpstreamCall("previous_runs", err, time.Since(upstreamStart))
	}
	if err != nil {
		return nil, "", err
	}
	if fallback == nil {
		return nil, "", errors.New("weather previous runs unavailable")
	}
	fallback.Provenance.CanonicalLocationKey = key
	fallback.Provenance.IssuedAt = s.nowFn().UTC()
	return fallback, "previous_runs", nil
}

func (s *Service) convertBundleForResponse(ctx context.Context, bundle *Bundle, unitSystem UnitSystem) *Bundle {
	out := *bundle
	states, err := s.snapshots.LoadBiasStates(ctx, bundle.Provenance.CanonicalLocationKey)
	if err == nil {
		loc := loadLocation(bundle.Provenance.Timezone)
		index := BuildBiasIndex(states)
		out.Hourly = make([]HourlyForecastPoint, len(bundle.Hourly))
		for idx, point := range bundle.Hourly {
			out.Hourly[idx] = ApplyForecastBias(point, loc, index)
			out.Hourly[idx].Raw = ForecastValues(out.Hourly[idx].Raw, unitSystem)
			out.Hourly[idx].Corrected = ForecastValues(out.Hourly[idx].Corrected, unitSystem)
		}
	} else {
		out.Hourly = make([]HourlyForecastPoint, len(bundle.Hourly))
		copy(out.Hourly, bundle.Hourly)
		for idx := range out.Hourly {
			out.Hourly[idx].Raw = ForecastValues(out.Hourly[idx].Raw, unitSystem)
			out.Hourly[idx].Corrected = ForecastValues(out.Hourly[idx].Corrected, unitSystem)
		}
	}
	out.Daily = make([]DailyForecastPoint, len(bundle.Daily))
	copy(out.Daily, bundle.Daily)
	out.Provenance.VerificationSource = ""
	return &out
}

func metricRequest(req Request) Request {
	req = req.Normalized()
	req.UnitSystem = UnitSystemMetric
	req.Timezone = normalizedTimezone(req.Timezone)
	return req
}

func normalizedTimezone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "auto"
	}
	return value
}

func canonicalKeyFromBundle(bundle *Bundle, req Request) string {
	return cachekey.Build(cachekey.CanonicalLocation{
		Latitude:            bundle.Provenance.Latitude,
		Longitude:           bundle.Provenance.Longitude,
		Elevation:           bundle.Provenance.Elevation,
		PanelTiltDegrees:    cachekey.TiltBucket(req.PanelTiltDegrees),
		PanelAzimuthDegrees: cachekey.AzimuthBucket(req.PanelAzimuthDegrees),
	})
}

func loadLocation(name string) *time.Location {
	if strings.TrimSpace(name) == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

func yesterdayBounds(nowFn func() time.Time, loc *time.Location) (time.Time, time.Time) {
	now := nowFn().In(loc)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return todayStart.AddDate(0, 0, -1), todayStart
}

func filterHourly(points []HourlyForecastPoint, from, to time.Time) []HourlyForecastPoint {
	out := make([]HourlyForecastPoint, 0, len(points))
	for _, point := range points {
		if !point.Time.Before(from.UTC()) && point.Time.Before(to.UTC()) {
			out = append(out, point)
		}
	}
	return out
}

func hourlyByTime(points []HourlyForecastPoint) map[int64]HourlyForecastPoint {
	out := make(map[int64]HourlyForecastPoint, len(points))
	for _, point := range points {
		out[point.Time.UTC().UnixMilli()] = point
	}
	return out
}

func buildVerificationSummary(rows []VerificationHour, windDirectionDiffs []float64) VerificationSummary {
	return VerificationSummary{
		Temperature: metricError(rows, func(row VerificationHour) (*float64, *float64) {
			return row.ForecastRaw.Temperature, row.Actual.Temperature
		}),
		WindSpeed: metricError(rows, func(row VerificationHour) (*float64, *float64) {
			return row.ForecastRaw.WindSpeed, row.Actual.WindSpeed
		}),
		CloudCover: metricError(rows, func(row VerificationHour) (*float64, *float64) {
			return row.ForecastRaw.CloudCover, row.Actual.CloudCover
		}),
		Visibility: metricError(rows, func(row VerificationHour) (*float64, *float64) {
			return row.ForecastRaw.Visibility, row.Actual.Visibility
		}),
		UVIndex: metricError(rows, func(row VerificationHour) (*float64, *float64) { return row.ForecastRaw.UVIndex, row.Actual.UVIndex }),
		ShortwaveRadiation: metricError(rows, func(row VerificationHour) (*float64, *float64) {
			return row.ForecastRaw.ShortwaveRadiation, row.Actual.ShortwaveRadiation
		}),
		GlobalTiltedIrradiance: metricError(rows, func(row VerificationHour) (*float64, *float64) {
			return row.ForecastRaw.GlobalTiltedIrradiance, row.Actual.GlobalTiltedIrradiance
		}),
		Precipitation: metricError(rows, func(row VerificationHour) (*float64, *float64) {
			return row.ForecastRaw.Precipitation, row.Actual.Precipitation
		}),
	}
}

func metricError(rows []VerificationHour, accessor func(VerificationHour) (*float64, *float64)) MetricError {
	sumAbs := 0.0
	sumBias := 0.0
	count := 0.0
	for _, row := range rows {
		forecast, actual := accessor(row)
		if forecast == nil || actual == nil {
			continue
		}
		diff := *actual - *forecast
		if diff < 0 {
			sumAbs -= diff
		} else {
			sumAbs += diff
		}
		sumBias += diff
		count++
	}
	if count == 0 {
		return MetricError{}
	}
	mae := sumAbs / count
	bias := sumBias / count
	return MetricError{
		MeanAbsoluteError: &mae,
		Bias:              &bias,
	}
}

func dedupeBiasStates(states []BiasState) []BiasState {
	latest := make(map[string]BiasState, len(states))
	for _, state := range states {
		key := fmt.Sprintf("%s|%s|%d", state.CanonicalLocationKey, state.Metric, state.HourOfDay)
		latest[key] = state
	}
	out := make([]BiasState, 0, len(latest))
	for _, state := range latest {
		out = append(out, state)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Metric == out[j].Metric {
			return out[i].HourOfDay < out[j].HourOfDay
		}
		return out[i].Metric < out[j].Metric
	})
	return out
}

func groupCandidates(candidates []RefreshCandidate) [][]RefreshCandidate {
	groups := map[string][]RefreshCandidate{}
	for _, candidate := range candidates {
		normalized := metricRequest(candidate.Request)
		groupKey := fmt.Sprintf("tz=%s|tilt=%v|az=%v", normalized.Timezone, valueString(normalized.PanelTiltDegrees), valueString(normalized.PanelAzimuthDegrees))
		groups[groupKey] = append(groups[groupKey], candidate)
	}
	out := make([][]RefreshCandidate, 0, len(groups))
	for _, group := range groups {
		out = append(out, group)
	}
	return out
}

func valueString(v *float64) string {
	if v == nil {
		return "none"
	}
	return fmt.Sprintf("%.3f", *v)
}

func (s *Service) budgetDeniedWindow(cost int) string {
	snapshot := s.budget.Snapshot()
	switch {
	case snapshot.DailyLimit > 0 && snapshot.DailyUsed+cost > snapshot.DailyLimit:
		return "daily"
	case snapshot.PerMinuteLimit > 0 && snapshot.MinuteUsed+cost > snapshot.PerMinuteLimit:
		return "per_minute"
	default:
		return "unknown"
	}
}
