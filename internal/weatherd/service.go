package weatherd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/budget"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/cachekey"
)

var ErrUpstreamBudgetExceeded = errors.New("weather upstream budget exhausted")

type Upstream interface {
	FetchForecast(ctx context.Context, req Request) (*Bundle, error)
	FetchForecastBatch(ctx context.Context, reqs []Request) ([]Bundle, error)
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

func (s *Service) convertBundleForResponse(_ context.Context, bundle *Bundle, unitSystem UnitSystem) *Bundle {
	out := *bundle
	out.Hourly = make([]HourlyForecastPoint, len(bundle.Hourly))
	copy(out.Hourly, bundle.Hourly)
	for idx := range out.Hourly {
		out.Hourly[idx].Raw = ForecastValues(out.Hourly[idx].Raw, unitSystem)
		out.Hourly[idx].Corrected = out.Hourly[idx].Raw
	}
	out.Daily = make([]DailyForecastPoint, len(bundle.Daily))
	copy(out.Daily, bundle.Daily)
	for idx := range out.Daily {
		out.Daily[idx].Corrected = out.Daily[idx].Raw
	}
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
