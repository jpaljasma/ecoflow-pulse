package grpcmw

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestLoggingUnary(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	log := slog.New(slog.NewTextHandler(&out, nil))
	interceptor := LoggingUnary(log)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "rid-log"))

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/pulse.test/Unary"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected log output")
	}
}

func TestLoggingUnaryWithError(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	log := slog.New(slog.NewTextHandler(&out, nil))
	interceptor := LoggingUnary(log)

	wantErr := errors.New("failed")
	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/pulse.test/Unary"}, func(ctx context.Context, req any) (any, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected log output")
	}
}

func TestLoggingStream(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	log := slog.New(slog.NewTextHandler(&out, nil))
	interceptor := LoggingStream(log)
	stream := &testServerStream{ctx: context.Background()}

	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pulse.test/Stream", IsServerStream: true}, func(srv any, ss grpc.ServerStream) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected log output")
	}
}
