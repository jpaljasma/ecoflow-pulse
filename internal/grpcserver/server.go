package grpcserver

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// Config is intentionally explicit so services can tune under load tests.
// Defaults are conservative and should be tuned empirically.
type Config struct {
	ListenAddr string
	Env        string // local|dev|staging|prod

	// Limits
	MaxRecvMsgSize    int
	MaxSendMsgSize    int
	MaxHeaderListSize uint32

	// Buffers
	ReadBufferSize  int
	WriteBufferSize int

	// HTTP/2 tuning
	MaxConcurrentStreams uint32
	InitConnWindowSize   int32
	InitStreamWindowSize int32

	KA        keepalive.ServerParameters
	KAEnforce keepalive.EnforcementPolicy
}

func DefaultConfig(env string) Config {
	return Config{
		ListenAddr: ":9090",
		Env:        env,

		// Safe baseline: unknown request sizes; allow override via env/config.
		MaxRecvMsgSize:    4 << 20,  // 4 MiB
		MaxSendMsgSize:    16 << 20, // 16 MiB (history queries may grow)
		MaxHeaderListSize: 16 << 10, // 16 KiB

		ReadBufferSize:  64 << 10, // 64 KiB
		WriteBufferSize: 64 << 10,

		// Start conservative; tune with ghz/k6.
		MaxConcurrentStreams: 10_000,
		InitConnWindowSize:   4 << 20, // 4 MiB
		InitStreamWindowSize: 1 << 20, // 1 MiB

		KA: keepalive.ServerParameters{
			Time:                  60 * time.Second,
			Timeout:               20 * time.Second,
			MaxConnectionIdle:     5 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 30 * time.Second,
		},
		KAEnforce: keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: false,
		},
	}
}

func New(cfg Config, unary []grpc.UnaryServerInterceptor, stream []grpc.StreamServerInterceptor, extraOpts ...grpc.ServerOption) (*grpc.Server, net.Listener, error) {
	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, nil, err
	}

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),

		grpc.KeepaliveParams(cfg.KA),
		grpc.KeepaliveEnforcementPolicy(cfg.KAEnforce),

		grpc.ReadBufferSize(cfg.ReadBufferSize),
		grpc.WriteBufferSize(cfg.WriteBufferSize),

		grpc.MaxConcurrentStreams(cfg.MaxConcurrentStreams),
		grpc.InitialConnWindowSize(cfg.InitConnWindowSize),
		grpc.InitialWindowSize(cfg.InitStreamWindowSize),

		grpc.MaxRecvMsgSize(cfg.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(cfg.MaxSendMsgSize),
		grpc.MaxHeaderListSize(cfg.MaxHeaderListSize),
	}

	opts = append(opts, extraOpts...)

	s := grpc.NewServer(opts...)

	// Reflection only for local/dev convenience.
	if cfg.Env == "local" || cfg.Env == "dev" {
		reflection.Register(s)
	}

	return s, lis, nil
}

// ServeWithSignal runs grpc.Server and blocks until:
// - server returns error
// - context is cancelled
// - SIGTERM/SIGINT is received
//
// It attempts graceful drain first, then hard stop after `grace`.
func ServeWithSignal(ctx context.Context, s *grpc.Server, lis net.Listener, grace time.Duration) error {
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(lis) }()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
		return stopWithGrace(s, grace)
	case <-sigCh:
		return stopWithGrace(s, grace)
	case err := <-errCh:
		return err
	}
}

func stopWithGrace(s *grpc.Server, grace time.Duration) error {
	stopped := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		return nil
	case <-time.After(grace):
		s.Stop()
		return nil
	}
}
