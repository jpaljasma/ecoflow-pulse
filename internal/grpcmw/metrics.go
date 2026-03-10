package grpcmw

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type Metrics struct {
	registry *prometheus.Registry
	inflight *prometheus.GaugeVec
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	m := &Metrics{
		registry: registry,
		inflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pulse_grpc_server_inflight_requests",
			Help: "Current in-flight gRPC requests by method and RPC type.",
		}, []string{"grpc_method", "grpc_type"}),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_grpc_server_requests_total",
			Help: "Total gRPC requests by method, RPC type, and gRPC status code.",
		}, []string{"grpc_method", "grpc_type", "grpc_code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_grpc_server_duration_seconds",
			Help:    "gRPC request duration by method, RPC type, and gRPC status code.",
			Buckets: prometheus.DefBuckets,
		}, []string{"grpc_method", "grpc_type", "grpc_code"}),
	}
	registry.MustRegister(m.inflight, m.requests, m.duration)
	return m
}

func (m *Metrics) Registry() *prometheus.Registry {
	if m == nil {
		return nil
	}
	return m.registry
}

func sanitizeMethod(method string) string {
	method = strings.TrimSpace(method)
	if method == "" {
		return "unknown"
	}
	return method
}

func (m *Metrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if m == nil {
			return handler(ctx, req)
		}
		method := sanitizeMethod(info.FullMethod)
		rpcType := "unary"
		start := time.Now()
		m.inflight.WithLabelValues(method, rpcType).Inc()
		defer m.inflight.WithLabelValues(method, rpcType).Dec()

		resp, err := handler(ctx, req)
		code := status.Code(err).String()
		m.requests.WithLabelValues(method, rpcType, code).Inc()
		m.duration.WithLabelValues(method, rpcType, code).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

func (m *Metrics) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if m == nil {
			return handler(srv, ss)
		}
		method := sanitizeMethod(info.FullMethod)
		rpcType := "bidi_stream"
		switch {
		case info.IsClientStream && !info.IsServerStream:
			rpcType = "client_stream"
		case !info.IsClientStream && info.IsServerStream:
			rpcType = "server_stream"
		case !info.IsClientStream && !info.IsServerStream:
			rpcType = "stream"
		}
		start := time.Now()
		m.inflight.WithLabelValues(method, rpcType).Inc()
		defer m.inflight.WithLabelValues(method, rpcType).Dec()

		err := handler(srv, ss)
		code := status.Code(err).String()
		m.requests.WithLabelValues(method, rpcType, code).Inc()
		m.duration.WithLabelValues(method, rpcType, code).Observe(time.Since(start).Seconds())
		return err
	}
}
