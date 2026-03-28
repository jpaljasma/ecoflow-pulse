package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"
)

func newPprofServerFromEnv(log *slog.Logger) (func(), string, error) {
	listenAddr := strings.TrimSpace(os.Getenv("GRPC_PPROF_LISTEN_ADDR"))
	if listenAddr == "" {
		log.Info("grpc pprof disabled", "reason", "GRPC_PPROF_LISTEN_ADDR not set")
		return func() {}, "disabled", nil
	}

	server, listener, err := newPprofServer(listenAddr)
	if err != nil {
		return nil, "", fmt.Errorf("create grpc pprof server: %w", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warn("grpc pprof server stopped", "error", err.Error(), "listen_addr", listener.Addr().String())
		}
	}()
	log.Info("grpc pprof enabled", "listen_addr", listener.Addr().String())
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warn("shutdown grpc pprof server failed", "error", err.Error())
		}
		<-done
	}, listener.Addr().String(), nil
}

func newPprofServer(listenAddr string) (*http.Server, net.Listener, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	for _, profile := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
		mux.Handle("/debug/pprof/"+profile, pprof.Handler(profile))
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, nil, err
	}
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server, listener, nil
}
