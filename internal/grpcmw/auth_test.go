package grpcmw

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type captureAuthorizer struct {
	method string
	claims Claims
	err    error
}

func (a *captureAuthorizer) Authorize(_ context.Context, fullMethod string, claims Claims) error {
	a.method = fullMethod
	a.claims = claims
	return a.err
}

func TestExtractBearerToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		md   metadata.MD
		want string
	}{
		{name: "missing", md: metadata.MD{}, want: ""},
		{name: "plain", md: metadata.Pairs("authorization", "abc"), want: ""},
		{name: "bearer", md: metadata.Pairs("authorization", "Bearer token-1"), want: "token-1"},
		{name: "bearer-lower", md: metadata.Pairs("authorization", "bearer token-2"), want: "token-2"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := extractBearerToken(tc.md); got != tc.want {
				t.Fatalf("unexpected token: got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestAuthUnaryAllowsAndPassesToken(t *testing.T) {
	t.Parallel()

	a := &captureAuthorizer{}
	interceptor := AuthUnary(a)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer jwt-token"))

	called := false
	resp, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/pulse.test/Unary"}, func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" || !called {
		t.Fatalf("handler was not executed as expected")
	}
	if a.method != "/pulse.test/Unary" {
		t.Fatalf("unexpected method: %q", a.method)
	}
	if a.claims.RawJWT != "jwt-token" {
		t.Fatalf("unexpected jwt passed to authorizer: %q", a.claims.RawJWT)
	}
}

func TestAuthUnaryDenied(t *testing.T) {
	t.Parallel()

	a := &captureAuthorizer{err: errors.New("denied")}
	interceptor := AuthUnary(a)

	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/pulse.test/Unary"}, func(ctx context.Context, req any) (any, error) {
		t.Fatalf("handler should not run when denied")
		return nil, nil
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestAuthStreamAllowsAndPassesToken(t *testing.T) {
	t.Parallel()

	a := &captureAuthorizer{}
	interceptor := AuthStream(a)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer stream-jwt"))
	stream := &testServerStream{ctx: ctx}

	called := false
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pulse.test/Stream"}, func(srv any, ss grpc.ServerStream) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("handler was not executed")
	}
	if a.method != "/pulse.test/Stream" {
		t.Fatalf("unexpected method: %q", a.method)
	}
	if a.claims.RawJWT != "stream-jwt" {
		t.Fatalf("unexpected jwt passed to authorizer: %q", a.claims.RawJWT)
	}
}

func TestAuthStreamDenied(t *testing.T) {
	t.Parallel()

	a := &captureAuthorizer{err: errors.New("nope")}
	interceptor := AuthStream(a)
	stream := &testServerStream{ctx: context.Background()}

	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pulse.test/Stream"}, func(srv any, ss grpc.ServerStream) error {
		t.Fatalf("handler should not run when denied")
		return nil
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}
