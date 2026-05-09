package workermetrics

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const drainHookDelay = 5 * time.Second

type ReadinessFunc func() (bool, string)

func StartServer(ctx context.Context, log *slog.Logger, registry *prometheus.Registry, listenAddr string) func() {
	return StartServerWithReadiness(ctx, log, registry, listenAddr, nil)
}

func StartServerWithReadiness(ctx context.Context, log *slog.Logger, registry *prometheus.Registry, listenAddr string, readiness ReadinessFunc) func() {
	if ctx == nil || log == nil || registry == nil || listenAddr == "" {
		return func() {}
	}
	var draining atomic.Bool
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if draining.Load() {
			http.Error(w, "draining", http.StatusServiceUnavailable)
			return
		}
		if readiness != nil {
			ok, reason := readiness()
			if !ok {
				if reason == "" {
					reason = "not_ready"
				}
				http.Error(w, reason, http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/drainz", func(w http.ResponseWriter, r *http.Request) {
		draining.Store(true)
		timer := time.NewTimer(drainHookDelay)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
		case <-timer.C:
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("draining"))
	})
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warn("metrics server stopped", slog.String("error", err.Error()))
		}
	}()
	go func() {
		<-ctx.Done()
		draining.Store(true)
	}()
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
			log.Warn("shutdown metrics server failed", slog.String("error", err.Error()))
		}
		<-done
	}
}
