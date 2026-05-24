package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/budget"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/openmeteo"
	weatherstore "github.com/jpaljasma/ecoflow-pulse/internal/weatherd/store"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
	"github.com/prometheus/client_golang/prometheus"
)

func newWeatherDomainFromEnv(log *slog.Logger, registerer prometheus.Registerer) (*weatherd.Service, func(), error) {
	nowFn := time.Now
	snapshotStore, cleanupSnapshots, snapshotSource, err := newWeatherSnapshotStoreFromEnv(log, nowFn)
	if err != nil {
		return nil, nil, err
	}
	hotCache, cleanupHotCache, hotCacheSource, err := newWeatherHotCacheFromEnv(log, nowFn)
	if err != nil {
		cleanupSnapshots()
		return nil, nil, err
	}
	upstream := openmeteo.NewClient(openmeteo.Config{
		HTTPClient: &http.Client{
			Timeout: runtimecfg.DurationNonNegative("WEATHER_UPSTREAM_TIMEOUT", 20*time.Second),
		},
		ForecastBaseURL:           strings.TrimSpace(os.Getenv("WEATHER_FORECAST_BASE_URL")),
		HistoricalForecastBaseURL: strings.TrimSpace(os.Getenv("WEATHER_HISTORICAL_FORECAST_BASE_URL")),
	})
	svc, err := weatherd.NewService(
		upstream,
		hotCache,
		snapshotStore,
		budget.New(budget.Config{
			DailyLimit:     runtimecfg.IntMin("WEATHER_UPSTREAM_DAILY_BUDGET_UNITS", 8000, 1),
			PerMinuteLimit: runtimecfg.IntMin("WEATHER_UPSTREAM_PER_MINUTE_BUDGET_UNITS", 120, 1),
			NowFn:          nowFn,
		}),
		weatherd.Config{
			HotTTL:             runtimecfg.DurationNonNegative("WEATHER_HOT_CACHE_TTL", 4*time.Hour),
			RecentActiveWindow: runtimecfg.DurationNonNegative("WEATHER_RECENT_ACTIVE_WINDOW", 7*24*time.Hour),
			NowFn:              nowFn,
			Metrics:            weatherd.NewMetrics(registerer),
		},
	)
	if err != nil {
		cleanupHotCache()
		cleanupSnapshots()
		return nil, nil, err
	}
	log.Info(
		"weather domain enabled",
		"snapshot_store", snapshotSource,
		"hot_cache", hotCacheSource,
		"daily_budget_units", runtimecfg.IntMin("WEATHER_UPSTREAM_DAILY_BUDGET_UNITS", 8000, 1),
		"per_minute_budget_units", runtimecfg.IntMin("WEATHER_UPSTREAM_PER_MINUTE_BUDGET_UNITS", 120, 1),
	)
	return svc, func() {
		cleanupHotCache()
		cleanupSnapshots()
	}, nil
}

func newWeatherSnapshotStoreFromEnv(log *slog.Logger, nowFn func() time.Time) (weatherd.SnapshotStore, func(), string, error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	if dsn == "" {
		log.Info("weather snapshot store using in-memory fallback", "reason", "CONTROL_PLANE_DB_DSN not set")
		store := weatherstore.NewMemorySnapshotStore(nowFn)
		return store, func() { _ = store.Close() }, "memory", nil
	}
	store, err := weatherstore.NewPostgresStore(dsn, nowFn)
	if err != nil {
		return nil, nil, "", err
	}
	return store, func() { _ = store.Close() }, "postgres", nil
}

func newWeatherHotCacheFromEnv(log *slog.Logger, nowFn func() time.Time) (weatherd.HotCache, func(), string, error) {
	valkeyAddrs := runtimecfg.SplitNonEmpty(strings.TrimSpace(os.Getenv("VALKEY_ADDRS")))
	if len(valkeyAddrs) == 0 {
		log.Info("weather hot cache using in-memory fallback", "reason", "VALKEY_ADDRS not set")
		return weatherstore.NewMemoryHotCache(nowFn), func() {}, "memory", nil
	}
	cfg := ingestlease.DefaultValkeyClientConfig(valkeyAddrs)
	cfg.Username = strings.TrimSpace(os.Getenv("VALKEY_USERNAME"))
	cfg.Password = os.Getenv("VALKEY_PASSWORD")
	ingestlease.ConfigureClientSideCacheFromEnv(&cfg)
	ingestlease.ConfigureSentinelFromEnv(&cfg)
	client, err := ingestlease.NewValkeyClient(cfg)
	if err != nil {
		return nil, nil, "", err
	}
	cache, err := weatherstore.NewValkeyHotCache(
		client,
		strings.TrimSpace(runtimecfg.EnvOrDefault("WEATHER_KEY_PREFIX", "pulse:weather")),
		nowFn,
	)
	if err != nil {
		client.Close()
		return nil, nil, "", err
	}
	return cache, func() { client.Close() }, "valkey", nil
}

func startWeatherRefreshLoop(ctx context.Context, log *slog.Logger, svc *weatherd.Service) func() {
	if svc == nil {
		return func() {}
	}
	interval := runtimecfg.DurationNonNegative("WEATHER_REFRESH_INTERVAL", 30*time.Minute)
	if interval <= 0 {
		log.Info("weather refresh loop disabled", "reason", "WEATHER_REFRESH_INTERVAL <= 0")
		return func() {}
	}
	childCtx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer ticker.Stop()
		if err := svc.RefreshRecentLocations(childCtx); err != nil && childCtx.Err() == nil {
			log.Warn("weather refresh loop failed", "error", err.Error())
		}
		for {
			select {
			case <-childCtx.Done():
				return
			case <-ticker.C:
				if err := svc.RefreshRecentLocations(childCtx); err != nil {
					log.Warn("weather refresh loop failed", "error", err.Error())
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func startWeatherMetricsLoop(ctx context.Context, log *slog.Logger, svc *weatherd.Service) func() {
	if svc == nil {
		return func() {}
	}
	interval := runtimecfg.DurationNonNegative("WEATHER_METRICS_REFRESH_INTERVAL", 30*time.Second)
	if interval <= 0 {
		log.Info("weather metrics loop disabled", "reason", "WEATHER_METRICS_REFRESH_INTERVAL <= 0")
		return func() {}
	}
	childCtx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer ticker.Stop()
		if err := svc.UpdateMetrics(childCtx); err != nil {
			log.Warn("weather metrics refresh failed", "error", err.Error())
		}
		for {
			select {
			case <-childCtx.Done():
				return
			case <-ticker.C:
				if err := svc.UpdateMetrics(childCtx); err != nil {
					log.Warn("weather metrics refresh failed", "error", err.Error())
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
