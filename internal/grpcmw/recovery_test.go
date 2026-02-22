package grpcmw

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecoveryUnaryHandlesPanic(t *testing.T) {
	t.Parallel()

	interceptor := RecoveryUnary()
	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/pulse.test/Unary"}, func(ctx context.Context, req any) (any, error) {
		panic("boom")
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected internal error, got %v", err)
	}
}

func TestRecoveryUnaryPassThrough(t *testing.T) {
	t.Parallel()

	interceptor := RecoveryUnary()
	resp, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/pulse.test/Unary"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
}

func TestRecoveryStreamHandlesPanic(t *testing.T) {
	t.Parallel()

	interceptor := RecoveryStream()
	stream := &testServerStream{ctx: context.Background()}
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pulse.test/Stream"}, func(srv any, ss grpc.ServerStream) error {
		panic("boom")
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected internal error, got %v", err)
	}
}

func TestRecoveryStreamPassThrough(t *testing.T) {
	t.Parallel()

	interceptor := RecoveryStream()
	stream := &testServerStream{ctx: context.Background()}
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pulse.test/Stream"}, func(srv any, ss grpc.ServerStream) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
