package grpcmw

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func LoggingUnary(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		code := status.Code(err)
		dur := time.Since(start)

		var attrs [6]slog.Attr
		n := 0
		attrs[n] = slog.String("grpc.method", info.FullMethod)
		n++
		attrs[n] = slog.String("grpc.code", code.String())
		n++
		attrs[n] = slog.Int64("duration_ms", dur.Milliseconds())
		n++
		if rid, ok := RequestIDFromContext(ctx); ok {
			attrs[n] = slog.String("request_id", rid)
			n++
		}
		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			attrs[n] = slog.String("peer", p.Addr.String())
			n++
		}
		if err != nil {
			attrs[n] = slog.String("error", err.Error())
			n++
			log.LogAttrs(ctx, slog.LevelWarn, "grpc unary", attrs[:n]...)
		} else {
			log.LogAttrs(ctx, slog.LevelInfo, "grpc unary", attrs[:n]...)
		}
		return resp, err
	}
}

func LoggingStream(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		code := status.Code(err)
		dur := time.Since(start)

		ctx := ss.Context()
		var attrs [8]slog.Attr
		n := 0
		attrs[n] = slog.String("grpc.method", info.FullMethod)
		n++
		attrs[n] = slog.String("grpc.code", code.String())
		n++
		attrs[n] = slog.Int64("duration_ms", dur.Milliseconds())
		n++
		attrs[n] = slog.Bool("grpc.is_client_stream", info.IsClientStream)
		n++
		attrs[n] = slog.Bool("grpc.is_server_stream", info.IsServerStream)
		n++
		if rid, ok := RequestIDFromContext(ctx); ok {
			attrs[n] = slog.String("request_id", rid)
			n++
		}
		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			attrs[n] = slog.String("peer", p.Addr.String())
			n++
		}
		if err != nil {
			attrs[n] = slog.String("error", err.Error())
			n++
			log.LogAttrs(ctx, slog.LevelWarn, "grpc stream", attrs[:n]...)
		} else {
			log.LogAttrs(ctx, slog.LevelInfo, "grpc stream", attrs[:n]...)
		}
		return err
	}
}
