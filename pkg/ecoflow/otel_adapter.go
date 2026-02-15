//go:build otel

package ecoflow

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelmetric "go.opentelemetry.io/otel/metric"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// NewOpenTelemetryOptions adapts the global OpenTelemetry SDK to
// ObservabilityOptions used by this client.
func NewOpenTelemetryOptions(instrumentationScope string) ObservabilityOptions {
	return ObservabilityOptions{
		Tracer: otelTracer{tracer: otel.Tracer(instrumentationScope)},
		Meter:  otelMeter{meter: otel.Meter(instrumentationScope)},
	}
}

type otelTracer struct {
	tracer oteltrace.Tracer
}

func (t otelTracer) Start(ctx context.Context, name string) (context.Context, Span) {
	ctx, span := t.tracer.Start(ctx, name)
	return ctx, otelSpan{span: span}
}

type otelSpan struct {
	span oteltrace.Span
}

func (s otelSpan) End() {
	s.span.End()
}

func (s otelSpan) RecordError(err error) {
	s.span.RecordError(err)
}

func (s otelSpan) SetStatus(code SpanStatus, description string) {
	switch code {
	case SpanStatusOK:
		s.span.SetStatus(codes.Ok, description)
	default:
		s.span.SetStatus(codes.Error, description)
	}
}

type otelMeter struct {
	meter otelmetric.Meter
}

func (m otelMeter) Int64Counter(name string) (Int64Counter, error) {
	counter, err := m.meter.Int64Counter(name)
	if err != nil {
		return nil, err
	}
	return otelInt64Counter{counter: counter}, nil
}

func (m otelMeter) Float64Histogram(name string) (Float64Histogram, error) {
	histogram, err := m.meter.Float64Histogram(name)
	if err != nil {
		return nil, err
	}
	return otelFloat64Histogram{histogram: histogram}, nil
}

type otelInt64Counter struct {
	counter otelmetric.Int64Counter
}

func (c otelInt64Counter) Add(ctx context.Context, value int64, attrs RequestAttributes) {
	c.counter.Add(ctx, value, otelmetric.WithAttributes(
		attribute.String("http.request.method", attrs.Method),
		attribute.String("url.path", attrs.Path),
		attribute.Int("http.response.status_code", attrs.Status),
		attribute.Int("ecoflow.retry.attempts", attrs.Attempts),
	))
}

type otelFloat64Histogram struct {
	histogram otelmetric.Float64Histogram
}

func (h otelFloat64Histogram) Record(ctx context.Context, value float64, attrs RequestAttributes) {
	h.histogram.Record(ctx, value, otelmetric.WithAttributes(
		attribute.String("http.request.method", attrs.Method),
		attribute.String("url.path", attrs.Path),
		attribute.Int("http.response.status_code", attrs.Status),
		attribute.Int("ecoflow.retry.attempts", attrs.Attempts),
	))
}
