package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"github.com/jpaljasma/ecoflow-pulse/internal/workermetrics"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
	"github.com/prometheus/client_golang/prometheus"
)

type noopWeatherForecaster struct{}

func (noopWeatherForecaster) Get7DayForecast(context.Context, weatherd.Request) (*weatherd.Bundle, error) {
	return nil, errors.New("weather forecast is not used by the solar verifier worker")
}

func main() {
	logCfg := pulselog.DefaultServiceConfig("solar-verifier")
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
	stopLogMetrics := pulselog.StartAsyncMetricsReporter(ctx, log, "solar-verifier", asyncLogHandler, logMetricsInterval)
	defer stopLogMetrics()
	metricsRegistry := prometheus.NewRegistry()
	metricsListenAddr := runtimecfg.EnvOrDefault("SOLAR_VERIFIER_METRICS_LISTEN_ADDR", "")
	stopMetrics := workermetrics.StartServer(ctx, log, metricsRegistry, metricsListenAddr)
	defer stopMetrics()

	queryReader, cleanupQueryReader, err := newSolarVerificationQueryReaderFromEnv(log)
	if err != nil {
		log.Error("solar verifier telemetry query reader init failed", "error", err.Error())
		os.Exit(1)
	}
	defer cleanupQueryReader()

	store, cleanupStore, err := newSolarVerificationTrainingStoreFromEnv(log)
	if err != nil {
		log.Error("solar verifier training store init failed", "error", err.Error())
		os.Exit(1)
	}
	defer cleanupStore()

	svc, err := solarforecastd.NewService(noopWeatherForecaster{}, queryReader, solarforecastd.Config{
		Log:     log,
		Store:   store,
		Metrics: solarforecastd.NewMetrics(metricsRegistry),
	})
	if err != nil {
		log.Error("solar verifier service init failed", "error", err.Error())
		os.Exit(1)
	}

	stopLoop := startSolarVerificationLoop(ctx, log, svc)
	defer stopLoop()

	log.Info("solar verifier starting",
		"log_level", logCfg.Level.String(),
		"log_async_enabled", logCfg.AsyncEnabled,
		"log_async_queue_size", logCfg.AsyncQueueSize,
		"log_async_bypass_level", logCfg.AsyncBypassLevel.String(),
		"log_metrics_interval", logMetricsInterval,
		"metrics_listen_addr", metricsListenAddr,
		"verification_interval", runtimecfg.DurationNonNegative("SOLAR_FORECAST_VERIFICATION_INTERVAL", 15*time.Minute),
		"batch_limit", runtimecfg.IntMin("SOLAR_FORECAST_VERIFICATION_BATCH_LIMIT", 24*64, 1),
	)

	<-ctx.Done()
	log.Info("solar verifier stopped")
}
