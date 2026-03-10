package grpcmw

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
)

func TestMetricsUnaryInterceptorRecordsRequest(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics()
	interceptor := metrics.UnaryServerInterceptor()
	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{FullMethod: "/pulse.telemetry.v1.TelemetryService/GetSnapshot"}, func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	families, err := metrics.Registry().Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	found := false
	for _, family := range families {
		if family.GetName() != "pulse_grpc_server_requests_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["grpc_method"] == "/pulse.telemetry.v1.TelemetryService/GetSnapshot" &&
				labels["grpc_type"] == "unary" &&
				labels["grpc_code"] == "OK" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected unary grpc request metric")
	}
}

func TestMetricsStreamInterceptorRecordsError(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics()
	interceptor := metrics.StreamServerInterceptor()
	stream := &testServerStream{ctx: context.Background()}
	wantErr := errors.New("boom")
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/pulse.telemetry.v1.TelemetryService/Subscribe", IsServerStream: true}, func(srv any, ss grpc.ServerStream) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: %v", err)
	}

	families, err := metrics.Registry().Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	found := false
	for _, family := range families {
		if family.GetName() != "pulse_grpc_server_requests_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["grpc_method"] == "/pulse.telemetry.v1.TelemetryService/Subscribe" &&
				labels["grpc_type"] == "server_stream" &&
				labels["grpc_code"] == "Unknown" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected stream grpc request metric")
	}
}
