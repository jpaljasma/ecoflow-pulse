package workermetrics

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func StartServer(ctx context.Context, log *slog.Logger, registry *prometheus.Registry, listenAddr string) func() {
	if ctx == nil || log == nil || registry == nil || listenAddr == "" {
		return func() {}
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
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
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
			log.Warn("shutdown metrics server failed", slog.String("error", err.Error()))
		}
		<-done
	}
}
