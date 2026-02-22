package grpcserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func noopUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(ctx, req)
	}
}

func noopStream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, ss)
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("local")
	if cfg.ListenAddr != ":9090" {
		t.Fatalf("unexpected listen addr: %q", cfg.ListenAddr)
	}
	if cfg.MaxRecvMsgSize <= 0 || cfg.MaxSendMsgSize <= 0 || cfg.MaxHeaderListSize <= 0 {
		t.Fatalf("expected positive size limits, got recv=%d send=%d header=%d", cfg.MaxRecvMsgSize, cfg.MaxSendMsgSize, cfg.MaxHeaderListSize)
	}
	if cfg.MaxConcurrentStreams == 0 {
		t.Fatalf("expected max concurrent streams > 0")
	}
}

func TestNewInvalidListenAddr(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("local")
	cfg.ListenAddr = "invalid-address"
	s, lis, err := New(cfg, []grpc.UnaryServerInterceptor{noopUnary()}, []grpc.StreamServerInterceptor{noopStream()})
	if err == nil {
		if s != nil {
			s.Stop()
		}
		if lis != nil {
			_ = lis.Close()
		}
		t.Fatalf("expected listen error")
	}
}

func TestReflectionEnabledOnlyLocalDev(t *testing.T) {
	t.Parallel()

	check := func(t *testing.T, env string, want bool) {
		t.Helper()
		cfg := DefaultConfig(env)
		cfg.ListenAddr = "127.0.0.1:0"
		s, lis, err := New(cfg, []grpc.UnaryServerInterceptor{noopUnary()}, []grpc.StreamServerInterceptor{noopStream()})
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		defer s.Stop()
		defer lis.Close()

		info := s.GetServiceInfo()
		hasReflection := false
		for name := range info {
			if strings.Contains(name, "grpc.reflection") {
				hasReflection = true
				break
			}
		}
		if hasReflection != want {
			t.Fatalf("env=%s reflection=%v want=%v", env, hasReflection, want)
		}
	}

	check(t, "local", true)
	check(t, "dev", true)
	check(t, "prod", false)
}

func TestServeWithSignalStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("prod")
	cfg.ListenAddr = "127.0.0.1:0"
	s, lis, err := New(cfg, []grpc.UnaryServerInterceptor{noopUnary()}, []grpc.StreamServerInterceptor{noopStream()})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer lis.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ServeWithSignal(ctx, s, lis, 250*time.Millisecond)
	}()

	// Ensure Serve has started.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeWithSignal returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("ServeWithSignal did not stop in time")
	}
}
