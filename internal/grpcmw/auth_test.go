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

func (a *captureAuthorizer) Authorize(_ context.Context, fullMethod string, claims *Claims) error {
	a.method = fullMethod
	if claims != nil {
		a.claims = *claims
	}
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

func TestAuthUnaryStoresClaimsInContext(t *testing.T) {
	t.Parallel()

	a := &captureAuthorizer{}
	interceptor := AuthUnary(a)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer jwt-token"))

	_, err := interceptor(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/pulse.test/Unary"}, func(ctx context.Context, req any) (any, error) {
		claims, ok := ClaimsFromContext(ctx)
		if !ok {
			t.Fatalf("expected claims in context")
		}
		if claims.RawJWT != "jwt-token" {
			t.Fatalf("unexpected raw jwt in context: %q", claims.RawJWT)
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestAuthStreamStoresClaimsInContext(t *testing.T) {
	t.Parallel()

	a := &captureAuthorizer{}
	interceptor := AuthStream(a)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer stream-jwt"))
	stream := &testServerStream{ctx: ctx}

	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pulse.test/Stream"}, func(srv any, ss grpc.ServerStream) error {
		claims, ok := ClaimsFromContext(ss.Context())
		if !ok {
			t.Fatalf("expected claims in stream context")
		}
		if claims.RawJWT != "stream-jwt" {
			t.Fatalf("unexpected raw jwt in stream context: %q", claims.RawJWT)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestNoopAuthorizerCopiesUserSubjectFromMetadata(t *testing.T) {
	t.Parallel()

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-subject", "dev-user-1"))
	claims := &Claims{}

	if err := (NoopAuthorizer{}).Authorize(ctx, "/pulse.test/Unary", claims); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Subject != "dev-user-1" {
		t.Fatalf("unexpected subject: got=%q want=%q", claims.Subject, "dev-user-1")
	}
}
