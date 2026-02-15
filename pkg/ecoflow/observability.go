package ecoflow

import (
	"context"
	"sync/atomic"
	"time"
)

// ObservabilityOptions provides tracer and meter dependencies for the client.
// When omitted, lightweight no-op/in-memory defaults are used.
type ObservabilityOptions struct {
	Tracer Tracer
	Meter  Meter
}

func defaultObservability() ObservabilityOptions {
	return ObservabilityOptions{
		Tracer: noopTracer{},
		Meter:  &inMemoryMeter{},
	}
}

type clientMetrics struct {
	requestsTotal Int64Counter
	retriesTotal  Int64Counter
	failuresTotal Int64Counter
	latency       Float64Histogram
}

func newClientMetrics(meter Meter) (*clientMetrics, error) {
	requestsTotal, err := meter.Int64Counter("ecoflow_client_requests_total")
	if err != nil {
		return nil, err
	}

	retriesTotal, err := meter.Int64Counter("ecoflow_client_retries_total")
	if err != nil {
		return nil, err
	}

	failuresTotal, err := meter.Int64Counter("ecoflow_client_failures_total")
	if err != nil {
		return nil, err
	}

	latency, err := meter.Float64Histogram("ecoflow_client_request_latency_ms")
	if err != nil {
		return nil, err
	}

	return &clientMetrics{
		requestsTotal: requestsTotal,
		retriesTotal:  retriesTotal,
		failuresTotal: failuresTotal,
		latency:       latency,
	}, nil
}

func (m *clientMetrics) recordRequest(ctx context.Context, attrs RequestAttributes) {
	m.requestsTotal.Add(ctx, 1, attrs)
}

func (m *clientMetrics) recordRetry(ctx context.Context, attrs RequestAttributes) {
	m.retriesTotal.Add(ctx, 1, attrs)
}

func (m *clientMetrics) recordFailure(ctx context.Context, attrs RequestAttributes) {
	m.failuresTotal.Add(ctx, 1, attrs)
}

func (m *clientMetrics) recordLatency(ctx context.Context, elapsed time.Duration, attrs RequestAttributes) {
	m.latency.Record(ctx, float64(elapsed.Milliseconds()), attrs)
}

// RequestAttributes is the common dimension set recorded for request telemetry.
type RequestAttributes struct {
	Method   string
	Path     string
	Status   int
	Attempts int
}

func requestAttributes(method, path string, status int, attempts int) RequestAttributes {
	return RequestAttributes{
		Method:   method,
		Path:     path,
		Status:   status,
		Attempts: attempts,
	}
}

func annotateSpanResult(span Span, statusCode int, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(SpanStatusError, err.Error())
		return
	}
	if statusCode >= 400 {
		span.SetStatus(SpanStatusError, "http error response")
		return
	}
	span.SetStatus(SpanStatusOK, "")
}

// SpanStatus represents a coarse success/failure outcome for a span.
type SpanStatus string

const (
	// SpanStatusOK indicates successful request completion.
	SpanStatusOK SpanStatus = "ok"
	// SpanStatusError indicates a failed request lifecycle.
	SpanStatusError SpanStatus = "error"
)

// Tracer represents the minimal tracing API required by this client.
type Tracer interface {
	Start(ctx context.Context, name string) (context.Context, Span)
}

// Span is the minimal span interface used by the client runtime.
type Span interface {
	End()
	RecordError(err error)
	SetStatus(code SpanStatus, description string)
}

// Meter represents the minimal metrics API required by this client.
type Meter interface {
	Int64Counter(name string) (Int64Counter, error)
	Float64Histogram(name string) (Float64Histogram, error)
}

// Int64Counter records monotonic integer metrics.
type Int64Counter interface {
	Add(ctx context.Context, value int64, attrs RequestAttributes)
}

// Float64Histogram records floating-point distribution metrics.
type Float64Histogram interface {
	Record(ctx context.Context, value float64, attrs RequestAttributes)
}

type noopTracer struct{}

func (noopTracer) Start(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}

type noopSpan struct{}

func (noopSpan) End()                         {}
func (noopSpan) RecordError(error)            {}
func (noopSpan) SetStatus(SpanStatus, string) {}

type inMemoryMeter struct {
	requests atomic.Int64
	retries  atomic.Int64
	failures atomic.Int64
}

func (m *inMemoryMeter) Int64Counter(name string) (Int64Counter, error) {
	switch name {
	case "ecoflow_client_requests_total":
		return &atomicCounter{target: &m.requests}, nil
	case "ecoflow_client_retries_total":
		return &atomicCounter{target: &m.retries}, nil
	case "ecoflow_client_failures_total":
		return &atomicCounter{target: &m.failures}, nil
	default:
		return &atomicCounter{}, nil
	}
}

func (m *inMemoryMeter) Float64Histogram(_ string) (Float64Histogram, error) {
	return noopHistogram{}, nil
}

type atomicCounter struct {
	target *atomic.Int64
}

func (c *atomicCounter) Add(_ context.Context, value int64, _ RequestAttributes) {
	if c.target != nil {
		c.target.Add(value)
	}
}

type noopHistogram struct{}

func (noopHistogram) Record(context.Context, float64, RequestAttributes) {}
