package grpcmw

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Authorizer enforces authn/authz for internal gRPC.
// In EcoFlow Pulse, Node REST forwards the user JWT in metadata and Go enforces device-level authz.
// This interface lets you swap implementations without changing server wiring.
type Authorizer interface {
	Authorize(ctx context.Context, fullMethod string, claims *Claims) error
}

// Claims is a minimal placeholder. Expand once you wire Keycloak/JWKS validation.
type Claims struct {
	Subject string
	Email   string
	Roles   []string
	RawJWT  string
}

// NoopAuthorizer allows all calls (use only for local/dev scaffolding).
type NoopAuthorizer struct{}

func (NoopAuthorizer) Authorize(ctx context.Context, fullMethod string, claims *Claims) error {
	return nil
}

type claimsContextKey struct{}

func ContextWithClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	v, ok := ctx.Value(claimsContextKey{}).(Claims)
	return v, ok
}

func extractBearerToken(md metadata.MD) string {
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return ""
	}
	v := vals[0]
	if strings.HasPrefix(strings.ToLower(v), "bearer ") {
		return strings.TrimSpace(v[7:])
	}
	return ""
}

func AuthUnary(a Authorizer) grpc.UnaryServerInterceptor {
	if a == nil {
		a = NoopAuthorizer{}
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		jwt := extractBearerToken(md)

		claims := &Claims{RawJWT: jwt}

		if err := a.Authorize(ctx, info.FullMethod, claims); err != nil {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return handler(ContextWithClaims(ctx, *claims), req)
	}
}

func AuthStream(a Authorizer) grpc.StreamServerInterceptor {
	if a == nil {
		a = NoopAuthorizer{}
	}
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		md, _ := metadata.FromIncomingContext(ctx)
		jwt := extractBearerToken(md)

		claims := &Claims{RawJWT: jwt}

		if err := a.Authorize(ctx, info.FullMethod, claims); err != nil {
			return status.Error(codes.PermissionDenied, err.Error())
		}
		return handler(srv, &claimsServerStream{
			ServerStream: ss,
			ctx:          ContextWithClaims(ctx, *claims),
		})
	}
}

type claimsServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *claimsServerStream) Context() context.Context {
	return s.ctx
}
