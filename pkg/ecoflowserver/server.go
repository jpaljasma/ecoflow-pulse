package ecoflowserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

// Server wraps an http.Server with high-throughput defaults and graceful
// shutdown behavior.
type Server struct {
	config   Config
	http     *http.Server
	listener net.Listener
}

// New constructs a configured server instance.
func New(cfg Config) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	handler := cfg.Handler
	if handler == nil {
		handler = defaultHandler()
	}

	compressedHandler, err := compressionMiddleware(cfg.Compression, handler)
	if err != nil {
		return nil, err
	}

	srv := &Server{
		config: cfg,
	}
	srv.http = &http.Server{
		Addr:              cfg.Address,
		Handler:           compressedHandler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	return srv, nil
}

// ListenAndServe starts the server and blocks until shutdown or failure.
func (s *Server) ListenAndServe(ctx context.Context) error {
	applyProcessTuningFromEnvironment()

	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.config.Address, err)
	}
	s.listener = tcpKeepAliveListener{listener.(*net.TCPListener)}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- s.http.Serve(s.listener)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = s.http.Shutdown(shutdownCtx)
		return ctx.Err()
	case sig := <-stop:
		_ = sig
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	case err := <-serveErrCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func defaultHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	payload := []byte(`{"status":"ok","service":"ecoflow-server"}`)
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	})

	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, r.Body)
	})

	return mux
}

func applyProcessTuningFromEnvironment() {
	if raw := os.Getenv("ECOFLOW_SERVER_GOMAXPROCS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil && n > 0 {
			runtime.GOMAXPROCS(n)
		}
	}
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	conn, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	_ = conn.SetKeepAlive(true)
	_ = conn.SetKeepAlivePeriod(3 * time.Minute)
	_ = conn.SetNoDelay(true)
	return conn, nil
}
