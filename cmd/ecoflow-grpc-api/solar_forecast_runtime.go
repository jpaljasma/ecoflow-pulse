package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd"
	solarstore "github.com/jpaljasma/ecoflow-pulse/internal/solarforecastd/store"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetryquery"
	"github.com/jpaljasma/ecoflow-pulse/internal/weatherd"
	"github.com/jpaljasma/ecoflow-pulse/pkg/runtimecfg"
	"github.com/prometheus/client_golang/prometheus"
)

func newSolarForecastDomainFromEnv(
	log *slog.Logger,
	registerer prometheus.Registerer,
	weatherDomain *weatherd.Service,
	queryReader telemetryquery.Reader,
) (*solarforecastd.Service, func(), error) {
	store, cleanupStore, source, err := newSolarForecastTrainingStoreFromEnv(log)
	if err != nil {
		return nil, nil, err
	}
	svc, err := solarforecastd.NewService(weatherDomain, queryReader, solarforecastd.Config{
		Log:     log,
		Store:   store,
		Metrics: solarforecastd.NewMetrics(registerer),
	})
	if err != nil {
		cleanupStore()
		return nil, nil, err
	}
	log.Info("solar forecast domain enabled", "training_store", source)
	return svc, cleanupStore, nil
}

func newSolarForecastTrainingStoreFromEnv(log *slog.Logger) (solarforecastd.TrainingStore, func(), string, error) {
	dsn := strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN"))
	if dsn == "" {
		log.Info("solar forecast training store disabled", "reason", "CONTROL_PLANE_DB_DSN not set")
		return nil, func() {}, "disabled", nil
	}
	store, err := solarstore.NewPostgresStore(dsn)
	if err != nil {
		return nil, nil, "", err
	}
	return store, func() { _ = store.Close() }, "postgres", nil
}

func startSolarVerificationLoop(ctx context.Context, log *slog.Logger, svc *solarforecastd.Service) func() {
	if svc == nil {
		return func() {}
	}
	interval := runtimecfg.DurationNonNegative("SOLAR_FORECAST_VERIFICATION_INTERVAL", 15*time.Minute)
	if interval <= 0 {
		log.Info("solar forecast verification loop disabled", "reason", "SOLAR_FORECAST_VERIFICATION_INTERVAL <= 0")
		return func() {}
	}
	limit := runtimecfg.IntMin("SOLAR_FORECAST_VERIFICATION_BATCH_LIMIT", 24*64, 1)
	childCtx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer ticker.Stop()
		if err := svc.VerifyIssuedForecasts(childCtx, time.Now().UTC(), limit); err != nil {
			log.Warn("solar forecast verification loop failed", "error", err.Error())
		}
		for {
			select {
			case <-childCtx.Done():
				return
			case <-ticker.C:
				if err := svc.VerifyIssuedForecasts(childCtx, time.Now().UTC(), limit); err != nil {
					log.Warn("solar forecast verification loop failed", "error", err.Error())
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
