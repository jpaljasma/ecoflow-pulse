package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"fmt"
	"github.com/jpaljasma/ecoflow-pulse/internal/ingestlease"
	"github.com/jpaljasma/ecoflow-pulse/internal/scheduler"
	solarstore "github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd/store"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/budget"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd/openmeteo"
	weatherstore "github.com/jpaljasma/ecoflow-pulse/internal/weatherd/store"
	"github.com/jpaljasma/ecoflow-pulse/internal/workermetrics"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultSchedulerPollInterval               = time.Minute
	defaultWeatherRefreshScanInterval          = 5 * time.Minute
	defaultWeatherHotDataPruneInterval         = 24 * time.Hour
	defaultSolarHotDataPruneInterval           = 24 * time.Hour
	defaultWeatherVerificationRetention        = 30 * 24 * time.Hour
	defaultWeatherRefreshCandidateRetention    = 14 * 24 * time.Hour
	defaultSolarForecastRunRetention           = 14 * 24 * time.Hour
	defaultSolarForecastDailyVerificationLimit = 60 * 24 * time.Hour
)

func main() {
	logCfg := pulselog.DefaultServiceConfig("scheduler")
	logCfg.Level = pulselog.ParseLevel(os.Getenv("LOG_LEVEL"), slog.LevelInfo)
	logCfg.AsyncEnabled = !runtimecfg.Bool("LOG_ASYNC_DISABLED", false)
	logCfg.AsyncQueueSize = runtimecfg.IntMin("LOG_ASYNC_QUEUE_SIZE", logCfg.AsyncQueueSize, 128)
	logCfg.AsyncBypassLevel = pulselog.ParseLevel(runtimecfg.EnvOrDefault("LOG_ASYNC_BYPASS_LEVEL", "warn"), slog.LevelWarn)

	log, asyncLogHandler, err := pulselog.BuildServiceLogger(logCfg)
	if err != nil {
		_, _ = os.Stderr.WriteString("init logger failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() {
		if asyncLogHandler != nil {
			asyncLogHandler.Close()
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logMetricsInterval := runtimecfg.DurationNonNegative("LOG_METRICS_INTERVAL", pulselog.DefaultLogMetricsInterval())
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, "scheduler", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()

	metricsRegistry := prometheus.NewRegistry()
	schedulerMetrics := newSchedulerMetrics(metricsRegistry)
	metricsListenAddr := runtimecfg.EnvOrDefault("SCHEDULER_METRICS_LISTEN_ADDR", "")
	stopMetrics := workermetrics.StartServer(ctx, log, metricsRegistry, metricsListenAddr)
	defer stopMetrics()

	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	if dsn == "" {
		log.Error("scheduler requires CONTROL_PLANE_DB_DSN")
		os.Exit(1)
	}

	store, err := scheduler.NewPostgresStore(dsn)
	if err != nil {
		log.Error("scheduler store init failed", "error", err.Error())
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	weatherSnapshots, cleanupWeatherSnapshots, err := newWeatherSnapshotStore(nowFn())
	if err != nil {
		log.Error("scheduler weather snapshot store init failed", "error", err.Error())
		os.Exit(1)
	}
	defer cleanupWeatherSnapshots()

	hotCache, cleanupHotCache, err := newWeatherHotCache(nowFn())
	if err != nil {
		log.Error("scheduler weather hot cache init failed", "error", err.Error())
		os.Exit(1)
	}
	defer cleanupHotCache()

	weatherSvc, err := newWeatherService(log, metricsRegistry, weatherSnapshots, hotCache)
	if err != nil {
		log.Error("scheduler weather service init failed", "error", err.Error())
		os.Exit(1)
	}

	solarTrainingStore, cleanupSolarTrainingStore, err := newSolarTrainingStore()
	if err != nil {
		log.Error("scheduler solar store init failed", "error", err.Error())
		os.Exit(1)
	}
	defer cleanupSolarTrainingStore()

	if err := ensureDefaultJobs(ctx, store, nowFn()()); err != nil {
		log.Error("scheduler job bootstrap failed", "error", err.Error())
		os.Exit(1)
	}

	pollInterval := runtimecfg.DurationNonNegative("SCHEDULER_POLL_INTERVAL", defaultSchedulerPollInterval)
	if pollInterval <= 0 {
		log.Info("scheduler disabled", "reason", "SCHEDULER_POLL_INTERVAL <= 0")
		<-ctx.Done()
		return
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	runOnce := func() {
		jobs, err := store.ClaimDueJobs(ctx, nowFn()(), runtimecfg.IntMin("SCHEDULER_CLAIM_LIMIT", 8, 1))
		if err != nil {
			if ctx.Err() == nil {
				log.Warn("scheduler claim failed", "error", err.Error())
			}
			return
		}
		for _, job := range jobs {
			startedAt := time.Now()
			err := runJob(ctx, log, job, weatherSvc, weatherSnapshots, solarTrainingStore, schedulerMetrics)
			schedulerMetrics.observeJobRun(job.JobType, err, time.Since(startedAt), time.Now())
			if err != nil && ctx.Err() == nil {
				log.Warn("scheduler job failed", "job_key", job.JobKey, "job_type", job.JobType, "error", err.Error())
			}
		}
	}

	runOnce()
	for {
		select {
		case <-ctx.Done():
			log.Info("scheduler stopped")
			return
		case <-ticker.C:
			runOnce()
		}
	}
}

func nowFn() func() time.Time {
	return time.Now
}

func newWeatherSnapshotStore(nowFn func() time.Time) (*weatherstore.PostgresStore, func(), error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	store, err := weatherstore.NewPostgresStore(dsn, nowFn)
	if err != nil {
		return nil, nil, err
	}
	return store, func() { _ = store.Close() }, nil
}

func newWeatherHotCache(nowFn func() time.Time) (weatherd.HotCache, func(), error) {
	valkeyAddrs := runtimecfg.SplitNonEmpty(strings.TrimSpace(os.Getenv("VALKEY_ADDRS")))
	if len(valkeyAddrs) == 0 {
		return weatherstore.NewMemoryHotCache(nowFn), func() {}, nil
	}
	cfg := ingestlease.DefaultValkeyClientConfig(valkeyAddrs)
	cfg.Username = strings.TrimSpace(os.Getenv("VALKEY_USERNAME"))
	cfg.Password = os.Getenv("VALKEY_PASSWORD")
	ingestlease.ConfigureSentinelFromEnv(&cfg)
	client, err := ingestlease.NewValkeyClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	cache, err := weatherstore.NewValkeyHotCache(client, strings.TrimSpace(runtimecfg.EnvOrDefault("WEATHER_KEY_PREFIX", "pulse:weather")), nowFn)
	if err != nil {
		client.Close()
		return nil, nil, err
	}
	return cache, func() { client.Close() }, nil
}

func newWeatherService(log *slog.Logger, registerer prometheus.Registerer, snapshots weatherd.SnapshotStore, hotCache weatherd.HotCache) (*weatherd.Service, error) {
	upstream := openmeteo.NewClient(openmeteo.Config{
		HTTPClient: &http.Client{
			Timeout: runtimecfg.DurationNonNegative("WEATHER_UPSTREAM_TIMEOUT", 20*time.Second),
		},
		ForecastBaseURL:           strings.TrimSpace(os.Getenv("WEATHER_FORECAST_BASE_URL")),
		PreviousRunsBaseURL:       strings.TrimSpace(os.Getenv("WEATHER_PREVIOUS_RUNS_BASE_URL")),
		HistoricalForecastBaseURL: strings.TrimSpace(os.Getenv("WEATHER_HISTORICAL_FORECAST_BASE_URL")),
	})
	return weatherd.NewService(
		upstream,
		hotCache,
		snapshots,
		budget.New(budget.Config{
			DailyLimit:     runtimecfg.IntMin("WEATHER_UPSTREAM_DAILY_BUDGET_UNITS", 8000, 1),
			PerMinuteLimit: runtimecfg.IntMin("WEATHER_UPSTREAM_PER_MINUTE_BUDGET_UNITS", 120, 1),
			NowFn:          time.Now,
		}),
		weatherd.Config{
			HotTTL:             runtimecfg.DurationNonNegative("WEATHER_HOT_CACHE_TTL", 4*time.Hour),
			RecentActiveWindow: runtimecfg.DurationNonNegative("WEATHER_RECENT_ACTIVE_WINDOW", 7*24*time.Hour),
			NowFn:              time.Now,
			Metrics:            weatherd.NewMetrics(registerer),
		},
	)
}

func newSolarTrainingStore() (*solarstore.PostgresStore, func(), error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	store, err := solarstore.NewPostgresStore(dsn)
	if err != nil {
		return nil, nil, err
	}
	return store, func() { _ = store.Close() }, nil
}

func ensureDefaultJobs(ctx context.Context, store *scheduler.PostgresStore, now time.Time) error {
	jobs := []scheduler.RecurringJob{
		{
			JobKey:      "weather.refresh_due_candidates",
			JobType:     "weather.refresh_due_candidates",
			Interval:    runtimecfg.DurationNonNegative("SCHEDULER_WEATHER_REFRESH_SCAN_INTERVAL", defaultWeatherRefreshScanInterval),
			Enabled:     true,
			NextRunAt:   now,
			PayloadJSON: []byte(`{}`),
		},
		{
			JobKey:      "weather.prune_hot_data",
			JobType:     "weather.prune_hot_data",
			Interval:    runtimecfg.DurationNonNegative("SCHEDULER_WEATHER_PRUNE_INTERVAL", defaultWeatherHotDataPruneInterval),
			Enabled:     true,
			NextRunAt:   now,
			PayloadJSON: []byte(`{}`),
		},
		{
			JobKey:      "solar.prune_hot_data",
			JobType:     "solar.prune_hot_data",
			Interval:    runtimecfg.DurationNonNegative("SCHEDULER_SOLAR_PRUNE_INTERVAL", defaultSolarHotDataPruneInterval),
			Enabled:     true,
			NextRunAt:   now,
			PayloadJSON: []byte(`{}`),
		},
	}
	for _, job := range jobs {
		if err := store.EnsureJob(ctx, job, now); err != nil {
			return err
		}
	}
	return nil
}

func runJob(
	ctx context.Context,
	log *slog.Logger,
	job scheduler.RecurringJob,
	weatherSvc *weatherd.Service,
	weatherSnapshots *weatherstore.PostgresStore,
	solarTrainingStore *solarstore.PostgresStore,
	metrics *schedulerMetrics,
) error {
	switch job.JobType {
	case "weather.refresh_due_candidates":
		if weatherSvc == nil {
			return errors.New("weather service is not configured")
		}
		return weatherSvc.RefreshRecentLocations(ctx)
	case "weather.prune_hot_data":
		if weatherSnapshots == nil {
			return errors.New("weather snapshot store is not configured")
		}
		stats, err := weatherSnapshots.PruneHotData(
			ctx,
			time.Now().UTC().Add(-runtimecfg.DurationNonNegative("WEATHER_VERIFICATION_RETENTION", defaultWeatherVerificationRetention)),
			time.Now().UTC().Add(-runtimecfg.DurationNonNegative("WEATHER_REFRESH_CANDIDATE_RETENTION", defaultWeatherRefreshCandidateRetention)),
		)
		if err != nil {
			return err
		}
		metrics.observeCleanupRows(job.JobType, "weather_forecast_snapshots_compacted", stats.CompactedSnapshots)
		metrics.observeCleanupRows(job.JobType, "weather_yesterday_verifications_pruned", stats.PrunedVerifications)
		metrics.observeCleanupRows(job.JobType, "weather_refresh_candidates_pruned", stats.PrunedCandidates)
		if counts, countErr := weatherSnapshots.CountHotData(ctx, time.Now().UTC()); countErr == nil {
			metrics.setRetainedRows("weather_forecast_snapshots", counts.Snapshots)
			metrics.setRetainedRows("weather_yesterday_verifications", counts.Verifications)
			metrics.setRetainedRows("weather_refresh_candidates", counts.RefreshCandidates)
			metrics.setRetainedRows("weather_refresh_candidates_due", counts.DueRefreshCandidates)
		}
		log.Info("weather hot data pruned",
			"compacted_snapshots", stats.CompactedSnapshots,
			"pruned_verifications", stats.PrunedVerifications,
			"pruned_candidates", stats.PrunedCandidates,
		)
		return nil
	case "solar.prune_hot_data":
		if solarTrainingStore == nil {
			return errors.New("solar training store is not configured")
		}
		runCutoff := time.Now().UTC().Add(-runtimecfg.DurationNonNegative("SOLAR_FORECAST_RUN_RETENTION", defaultSolarForecastRunRetention))
		dailyCutoff := time.Now().UTC().Add(-runtimecfg.DurationNonNegative("SOLAR_FORECAST_DAILY_VERIFICATION_RETENTION", defaultSolarForecastDailyVerificationLimit))
		var totalRuns int64
		for {
			pruned, err := solarTrainingStore.PruneRunsOlderThan(ctx, runCutoff, runtimecfg.IntMin("SOLAR_FORECAST_PRUNE_RUN_BATCH_LIMIT", 250, 1))
			if err != nil {
				return err
			}
			totalRuns += pruned
			if pruned == 0 {
				break
			}
		}
		var totalDaily int64
		for {
			pruned, err := solarTrainingStore.PruneDailyVerificationOlderThan(ctx, dailyCutoff, runtimecfg.IntMin("SOLAR_FORECAST_PRUNE_DAILY_BATCH_LIMIT", 1000, 1))
			if err != nil {
				return err
			}
			totalDaily += pruned
			if pruned == 0 {
				break
			}
		}
		var orphanedHourly int64
		for {
			pruned, err := solarTrainingStore.PruneOrphanedHourlyRecords(ctx, runtimecfg.IntMin("SOLAR_FORECAST_PRUNE_ORPHANED_HOURLY_BATCH_LIMIT", 10000, 1))
			if err != nil {
				return err
			}
			orphanedHourly += pruned
			if pruned == 0 {
				break
			}
		}
		var orphanedRunRollups int64
		for {
			pruned, err := solarTrainingStore.PruneOrphanedRunDailyRollups(ctx, runtimecfg.IntMin("SOLAR_FORECAST_PRUNE_ORPHANED_RUN_ROLLUP_BATCH_LIMIT", 5000, 1))
			if err != nil {
				return err
			}
			orphanedRunRollups += pruned
			if pruned == 0 {
				break
			}
		}
		metrics.observeCleanupRows(job.JobType, "solar_forecast_runs_pruned", totalRuns)
		metrics.observeCleanupRows(job.JobType, "solar_forecast_verification_daily_pruned", totalDaily)
		metrics.observeCleanupRows(job.JobType, "solar_forecast_hourly_training_records_orphaned_pruned", orphanedHourly)
		metrics.observeCleanupRows(job.JobType, "solar_forecast_run_daily_rollups_orphaned_pruned", orphanedRunRollups)
		if counts, countErr := solarTrainingStore.CountRetainedTrainingData(ctx); countErr == nil {
			metrics.setRetainedRows("solar_forecast_runs", counts.Runs)
			metrics.setRetainedRows("solar_forecast_hourly_training_records", counts.HourlyRows)
			metrics.setRetainedRows("solar_forecast_verification_daily", counts.DailyVerificationRows)
			metrics.setRetainedRows("solar_forecast_verification_daily_run_rollup", counts.RunDailyRollupRows)
			metrics.setRetainedRows("solar_forecast_hourly_training_records_orphaned", counts.OrphanedHourlyRows)
			metrics.setRetainedRows("solar_forecast_verification_daily_run_rollup_orphaned", counts.OrphanedRunDailyRollups)
		}
		log.Info("solar hot data pruned",
			"pruned_runs", totalRuns,
			"pruned_daily_rows", totalDaily,
			"pruned_orphaned_hourly_rows", orphanedHourly,
			"pruned_orphaned_run_rollups", orphanedRunRollups,
		)
		return nil
	default:
		return fmt.Errorf("unsupported scheduler job type %q", job.JobType)
	}
}
