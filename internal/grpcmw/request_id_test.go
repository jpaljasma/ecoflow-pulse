package grpcmw

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type testServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *testServerStream) Context() context.Context { return s.ctx }

func TestRequestIDUnaryFromMetadata(t *testing.T) {
	t.Parallel()

	interceptor := RequestIDUnary()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "rid-123"))

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/pulse.test/Unary"}, func(ctx context.Context, req any) (any, error) {
		got, ok := RequestIDFromContext(ctx)
		if !ok {
			t.Fatalf("expected request id in context")
		}
		if got != "rid-123" {
			t.Fatalf("unexpected request id: %q", got)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestIDUnaryGenerated(t *testing.T) {
	t.Parallel()

	interceptor := RequestIDUnary()

	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/pulse.test/Unary"}, func(ctx context.Context, req any) (any, error) {
		got, ok := RequestIDFromContext(ctx)
		if !ok {
			t.Fatalf("expected request id in context")
		}
		// 16 random bytes hex-encoded.
		if len(got) != 32 {
			t.Fatalf("unexpected request id length: %d (%q)", len(got), got)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestIDStreamFromMetadata(t *testing.T) {
	t.Parallel()

	interceptor := RequestIDStream()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "rid-stream"))
	stream := &testServerStream{ctx: ctx}

	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pulse.test/Stream"}, func(srv any, ss grpc.ServerStream) error {
		got, ok := RequestIDFromContext(ss.Context())
		if !ok {
			t.Fatalf("expected request id in stream context")
		}
		if got != "rid-stream" {
			t.Fatalf("unexpected request id: %q", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestIDStreamGenerated(t *testing.T) {
	t.Parallel()

	interceptor := RequestIDStream()
	stream := &testServerStream{ctx: context.Background()}

	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pulse.test/Stream"}, func(srv any, ss grpc.ServerStream) error {
		got, ok := RequestIDFromContext(ss.Context())
		if !ok {
			t.Fatalf("expected request id in stream context")
		}
		if len(got) != 32 {
			t.Fatalf("unexpected request id length: %d (%q)", len(got), got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
